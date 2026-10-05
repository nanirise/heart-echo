package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/internal/repository"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// 前端能收到的三种事件名（契约 §6），只此三种。
//
// 与 ai_client 的 EventDelta/EventEnd 刻意不同名：Python 的 end 是"我生成完了"，
// 语义上还没有 messageId；Go 收到 end 后要先落库再发 done，两者不是同一个事件。
// 复用同一个常量串会让"漏发 done"和"漏转 end"看起来像同一个 bug。
const (
	ChatEventDelta = "delta"
	ChatEventDone  = "done"
	ChatEventError = "error"
)

// historyLimit 一次带进 prompt 的历史消息**条数**上限（不是轮数）。
//
// 不设成"越多越好"：这些内容每一轮都要重新发一遍，条数直接乘进 token 成本，
// 超过模型的上下文窗口还会让整次请求直接报错。20 条约等于 10 轮往返，
// 够模型"接上上一句"，又不至于让第 50 轮的开销变成第 1 轮的几十倍。
//
// 不放进 config：现在没有任何地方需要调它。没人用的开关只会变成噪音，
// 等真要按模型调窗口的时候再提上去。
const historyLimit = 20

// ChatEvent 是 Go 发给前端的一条事件，字段按 type 择一使用。
type ChatEvent struct {
	Type      string // delta | done | error
	Text      string // 仅 delta
	MessageID uint64 // 仅 done
	Code      int    // 仅 error
	Message   string // 仅 error
}

// NudgeInput 一次注入式生成的全部输入。
//
// UserID 只有两个合法来源：手动端点从 Token 取、定时任务从扫描行取；
// 永不出自请求体（AGENTS §4.3：任何查询必须同时带 user_id 与 persona_id）。
type NudgeInput struct {
	UserID    uint64
	PersonaID uint64
	// Content 是被注入的 user 消息正文。主动消息固定为 "[nudge]"；
	// 日程提醒（P1）复用本方法时带上事件描述。
	Content string
}

// NudgeResult 是生成出来的 assistant 回复，**不是**被注入的那行 [nudge]。
// 三个字段与 API_CONTRACT §9 的 {messageId, content, createdAt} 一一对应。
type NudgeResult struct {
	MessageID uint64
	Content   string
	CreatedAt time.Time
}

// ChatService 是对话链路的唯一实现：SSE 对话、主动消息、日程提醒三条链路共用。
//
// 三条链路共用同一份落库逻辑是刻意的：主动消息的 [nudge] 与用户手打的字
// 在库里是同一张表的同一类行，各写一份 INSERT 迟早会在 created_at /
// last_message_at 上分叉。
type ChatService struct {
	db          *gorm.DB
	msgRepo     *repository.MessageRepo
	personaRepo *repository.PersonaRepo
	ai          AIClient
}

// NewChatService 构造 ChatService。
//
// ai 从外面传而不是在这里 New：它要读 config（AI_SERVICE_URL / TOKEN），
// 而 service 层不碰配置。副作用是测试能塞一个假的 AIClient，
// 不用真起 Python 服务就能跑整条链路。
func NewChatService(db *gorm.DB, ai AIClient) *ChatService {
	return &ChatService{
		db:          db,
		msgRepo:     repository.NewMessageRepo(db),
		personaRepo: repository.NewPersonaRepo(db),
		ai:          ai,
	}
}

// persistMessage 落一条消息，并把会话的 last_message_at 推到当前时刻。
//
// user 消息与 assistant 消息共用这一份：两者的落库动作完全相同，
// 拆成两个函数迟早会在 created_at / last_message_at 的同步上分叉。
// 调用方负责填 Role / Content / IsNudge，本函数只管"插一条 + 推时间戳"。
//
// 两处写入放在同一个事务里（chat-message §4 的不变量）：消息 INSERT 成功、
// 时间戳更新失败会留下"消息进去了、会话列表排序没动"的半截状态，
// 而列表排序错乱是查不出原因的——看上去一切正常，只是顺序不对。
//
// 成功时 m.ID 与 m.CreatedAt 已被数据库回填。
func (s *ChatService) persistMessage(
	ctx context.Context, m *model.ChatMessage,
) (*model.ChatMessage, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.msgRepo.Create(ctx, tx, m); err != nil {
			return err
		}
		return s.personaRepo.TouchLastMessageAt(ctx, tx, m.UserID, m.PersonaID)
	})
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	return m, nil
}

// buildAIRequest 组装发给 ai-service 的请求体：人格 + 最近 historyLimit 条历史 + 当前消息。
//
// 两个调用方（StreamChat / InjectNudge）都必须走这里。人格与历史一旦各取各的，
// 手动对话和主动消息会长出两套上下文，而"主动消息说话不像人设"是最难查的一类 bug：
// 现象是"偶尔不像"，没有任何一处报错。
//
// ⚠️ 调用方必须在**落 user 消息之前**调用它。落库之后再取历史，当前这句会同时出现在
// History 和 Message 里，模型会看到自己的问题被问了两遍。
func (s *ChatService) buildAIRequest(
	ctx context.Context, userID, personaID uint64, message string,
) (ChatRequest, error) {
	// FindOwned 一次拿到"是不是你的"与"三行设定"，比 ExistsOwnedByUser 之后再去查一遍少一次往返。
	// 「别人的 persona」与「不存在的 persona」都返回 not found，不可区分（契约 §12 2026-09-13 条）。
	persona, err := s.personaRepo.FindOwned(ctx, userID, personaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ChatRequest{}, errcode.New(errcode.ErrPersonaNotFound)
		}
		return ChatRequest{}, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	history, err := s.recentHistory(ctx, userID, personaID)
	if err != nil {
		return ChatRequest{}, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	return ChatRequest{
		UserID:    userID,
		PersonaID: personaID,
		Message:   message,
		Persona: &PersonaBrief{
			Name:            persona.Name,
			PersonalityDesc: persona.PersonalityDesc,
			SpeakingStyle:   persona.SpeakingStyle,
		},
		History: history,
	}, nil
}

// recentHistory 取最近 historyLimit 条历史，转成**时间正序**（旧→新）。
//
// 仓储返回的是倒序（最新在前）—— 那是给前端翻页用的顺序，不是给模型的。
// messages 数组必须按发生顺序排，所以在这里反转。反转放在这一层而不是改仓储：
// 倒序是它对所有调用方的既定语义，为一个调用方改掉会让另一个踩坑。
//
// 已知代价：ListByPersona 会顺带跑一条 COUNT(*)，本函数用不上。多一次查询换
// "不新增仓储方法、不动成员 3 的文件"，现阶段划算；等它成为热点再说。
func (s *ChatService) recentHistory(
	ctx context.Context, userID, personaID uint64,
) ([]HistoryTurn, error) {
	list, _, err := s.msgRepo.ListByPersona(ctx, userID, personaID, 0, historyLimit)
	if err != nil {
		return nil, err
	}

	turns := make([]HistoryTurn, 0, len(list))
	for i := len(list) - 1; i >= 0; i-- {
		turns = append(turns, HistoryTurn{
			Role:    string(list[i].Role),
			Content: list[i].Content,
		})
	}
	return turns, nil
}

// StreamChat 是 SSE 对话端点的主干：判归属 → 落 user → 调 AI → 转发 → 落 assistant → 发 done。
//
// 返回的 error 只表示"还没进 SSE 就失败了"（4001 / 4043 / 落库失败）：这时调用方
// 还没写响应头，可以正常回一个 JSON 错误。函数一旦返回 channel，响应头就已经
// 写出去了，之后所有的错（5001 / 5002 / 5003）只能走 error 事件——这是 spec §1.1
// "归属校验必须在写响应头之前"的直接后果。
func (s *ChatService) StreamChat(
	ctx context.Context, userID uint64, req dto.StreamChatRequest,
) (<-chan ChatEvent, error) {
	content := strings.TrimSpace(req.Content)
	if content == "" {
		return nil, errcode.New(errcode.ErrInvalidParams)
	}

	// 取人格 + 历史，顺带完成归属闸门（4043 从这里出来）。
	// 必须在落 user 消息**之前**：理由见 buildAIRequest 的说明。
	aiReq, err := s.buildAIRequest(ctx, userID, req.PersonaID, content)
	if err != nil {
		return nil, err
	}

	// 事务 ①：用户说的话先落库，再去找 AI 要回复。
	// 反过来的话，AI 不可用时这句话就从库里凭空消失，下一轮 prompt 也少一问。
	//
	// [假设] 这一步失败按普通 JSON 错误返回（5003，HTTP 500）。spec §1.1 只把
	// 4001 / 4010 / 4043 归为"进 SSE 之前"，没给落库失败的位置；此时流还没开，
	// 走 JSON 是真话。见下方 §待确认。
	if _, err := s.persistMessage(ctx, &model.ChatMessage{
		UserID:    userID,
		PersonaID: req.PersonaID,
		Role:      model.RoleUser,
		Content:   content,
	}); err != nil {
		return nil, err
	}

	// 到这里为止都没写响应头。channel 一交出去，调用方就会写 SSE 头并开始 range。
	// channel 无缓冲：本 goroutine 往里推事件时会阻塞到调用方来读，
	// 所以不存在"事件比响应头先到"的竞态。
	ch := make(chan ChatEvent)
	go s.runStream(ctx, aiReq, ch)
	return ch, nil
}

// runStream 在独立 goroutine 里推进"要回复 → 边收边转 → 收尾落库 → 发 done"。
//
// 它拿到 channel 时响应头已经在路上了，所以任何失败都只能作为 error 事件发出，
// 没有别的出口。
//
// 形参收组装好的 ChatRequest，而不是 (userID, personaID, content) 三件散装：
// 落 assistant 消息要用的两个 ID 本来就在里面，不必再传一遍，也就不可能传错顺序。
func (s *ChatService) runStream(ctx context.Context, aiReq ChatRequest, ch chan<- ChatEvent) {
	defer close(ch)

	upstream, err := s.ai.StreamChat(ctx, aiReq)
	if err != nil {
		// 这里是 5002 而不是 4043：响应头已经写出去了，回不去 HTTP 错误（spec §1.1）。
		emitChatEvent(ctx, ch, ChatEvent{
			Type:    ChatEventError,
			Code:    int(errcode.ErrAIUnavailable),
			Message: errcode.ErrAIUnavailable.Message(),
		})
		return
	}

	var reply strings.Builder
	for ev := range upstream {
		switch ev.Type {
		case EventDelta:
			reply.WriteString(ev.Text)
			// 逐段转发，不攒批：攒批会把打字机效果变成一段一段地蹦（spec §1.3）。
			if !emitChatEvent(ctx, ch, ChatEvent{Type: ChatEventDelta, Text: ev.Text}) {
				// 浏览器断开。两个后果都要认：ctx 已作废，用它做 DB 写必然失败；
				// 半截回复也不进历史。
				// [假设] spec 未定义这条路径。见下方 §待确认。
				return
			}
		case EventError:
			// Python 侧判定生成失败（5001）。半截回复同样不落库：
			// 截断的内容进了历史会污染后面每一轮的 prompt。
			emitChatEvent(ctx, ch, ChatEvent{
				Type:    ChatEventError,
				Code:    ev.Code,
				Message: ev.Message,
			})
			return
		case EventEnd:
			// 内部事件，绝不透传（契约 §6 只认 delta/done/error）
		}
	}

	// 一段都没攒到（上游异常关闭，且没走上面的 error 分支）：空回复不入库。
	// 空 content 在历史里看起来像"AI 没说话"，比直接报错更难查。
	if reply.Len() == 0 {
		emitChatEvent(ctx, ch, ChatEvent{
			Type:    ChatEventError,
			Code:    int(errcode.ErrLLMFailed),
			Message: errcode.ErrLLMFailed.Message(),
		})
		return
	}

	// 事务 ②。与事务 ① 之间隔着整个生成过程，两段不能合并成一个长事务（spec §1.5）。
	assistant, err := s.persistMessage(ctx, &model.ChatMessage{
		UserID:    aiReq.UserID,
		PersonaID: aiReq.PersonaID,
		Role:      model.RoleAssistant,
		Content:   reply.String(),
	})
	if err != nil {
		// 落库失败发 5003、不发 done：done 的语义是"这条已经在库里了"（spec §1.3）
		emitChatEvent(ctx, ch, ChatEvent{
			Type:    ChatEventError,
			Code:    int(errcode.ErrDBFailed),
			Message: errcode.ErrDBFailed.Message(),
		})
		return
	}

	emitChatEvent(ctx, ch, ChatEvent{Type: ChatEventDone, MessageID: assistant.ID})
}

// emitChatEvent 往 channel 推一个事件，返回 false 表示调用方已经不听了。
//
// 必须带 ctx.Done() 分支：调用方断开后没人再来读，直接写 channel 会永久阻塞，
// 这条 goroutine 就再也回收不了。
func emitChatEvent(ctx context.Context, ch chan<- ChatEvent, ev ChatEvent) bool {
	select {
	case ch <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

// ListMessages 分页读某个会话的历史消息（契约 §5，chat-message spec §3.2）。
//
// 顺序不能反：**先判人设归属，再查消息**。只靠 repo 里的 WHERE user_id 是不够的——
// 查出来是空集时，"这不是你的人设"与"是你的但还没聊过"长得一模一样，
// 而后者的正确答案是 200 + 空列表，前者必须是 4043。
//
// 返回倒序（最新在前），前端自己反转成"旧→新"。倒序是 TECH_DESIGN §10.3 定的。
func (s *ChatService) ListMessages(
	ctx context.Context, userID, personaID uint64, page, pageSize int,
) (*dto.PageResult[dto.ChatMessageItem], error) {
	owned, err := s.personaRepo.ExistsOwnedByUser(ctx, userID, personaID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if !owned {
		return nil, errcode.New(errcode.ErrPersonaNotFound)
	}

	// 钳制必须在算 offset 之前：否则 pageSize=100000 会先算出一个巨大的偏移量。
	page, pageSize = dto.ClampPage(page, pageSize)

	list, total, err := s.msgRepo.ListByPersona(ctx, userID, personaID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	// page / pageSize 回填的是**钳制后**的值，否则前端按自己传的 100000 去算页数会算错。
	return &dto.PageResult[dto.ChatMessageItem]{
		List:     dto.NewChatMessageItems(list),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// InjectNudge 注入一条 role=user、is_nudge=true 的消息，
// 走完整聊天链路生成 assistant 回复并落库，同步返回生成的回复。
//
// 与 StreamChat 是同一件事的两种包装：StreamChat 边收边转，因为前端正盯着字出来；
// 本方法先收完再交，因为调用方是定时任务或手动按钮，没有人要看着它一个字一个字出。
// 唯一的区别就是"要不要转发"，所以落库部分共用 persistMessage。
//
// 触发判定（该不该发、有没有超限额）由调用方负责（TriggerNow 侧）；本方法做
// 「取人格与历史 → 注入 [nudge] → 生成 → 落库」。归属校验在这里会**再做一次** ——
// 它是 buildAIRequest 取人格的副产品，等于白拿：多一次 WHERE 换"人格一定取得到"，
// 比单纯信任调用方更划算，何况 persona 是生成必需的，本来就躲不开。
//
// 生成失败原样上抛，调用方按 5001 出口；本方法不做任何降级（spec §3.a：
// 基础设施错误不能被吞成「未触发」，否则演示时「点了没反应」会变成常态）。
func (s *ChatService) InjectNudge(ctx context.Context, in NudgeInput) (*NudgeResult, error) {
	if strings.TrimSpace(in.Content) == "" {
		return nil, errcode.New(errcode.ErrInvalidParams)
	}

	// 同样在落 [nudge] 消息**之前**组装：否则这条注入消息会进入它自己的 History，
	// 模型在"历史里"和"当前消息里"各看到一次 [nudge]。
	aiReq, err := s.buildAIRequest(ctx, in.UserID, in.PersonaID, in.Content)
	if err != nil {
		return nil, err
	}

	// 事务 ①：注入的 [nudge] 也是一条 role=user 的消息，与用户手打的字同一张表。
	// is_nudge=true 是它唯一的区别，用来回答"这一轮是谁发起的"。
	// Role 保持 user 不变——主动消息仍然是"这一轮由用户侧发起"。
	if _, err := s.persistMessage(ctx, &model.ChatMessage{
		UserID:    in.UserID,
		PersonaID: in.PersonaID,
		Role:      model.RoleUser,
		Content:   in.Content,
		IsNudge:   true,
	}); err != nil {
		return nil, err
	}

	upstream, err := s.ai.StreamChat(ctx, aiReq)
	if err != nil {
		// AIClient 已包成 5002，原样上抛
		return nil, err
	}

	var reply strings.Builder
	for ev := range upstream {
		switch ev.Type {
		case EventDelta:
			// 不转发，先攒全文：这里没有 SSE 通道可发
			reply.WriteString(ev.Text)
		case EventError:
			// 生成失败原样上抛。绝不降级成"未触发"——那会让主动消息
			// 在演示时表现成"点了没反应"，而日志里一条错都没有（spec §3.a）。
			return nil, errcode.New(errcode.ErrLLMFailed)
		case EventEnd:
			// 内部事件，与 StreamChat 同样吞掉
		}
	}

	// 空回复不入库，理由与 runStream 相同：空 content 在历史里比报错更难查。
	if reply.Len() == 0 {
		return nil, errcode.New(errcode.ErrLLMFailed)
	}

	// 事务 ②
	assistant, err := s.persistMessage(ctx, &model.ChatMessage{
		UserID:    in.UserID,
		PersonaID: in.PersonaID,
		Role:      model.RoleAssistant,
		Content:   reply.String(),
	})
	if err != nil {
		return nil, err
	}

	return &NudgeResult{
		MessageID: assistant.ID,
		Content:   assistant.Content,
		CreatedAt: assistant.CreatedAt,
	}, nil
}

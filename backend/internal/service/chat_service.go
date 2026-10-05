package service

import (
	"context"
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

	// 归属闸门：对"别人的 persona"与"不存在的 persona"返回同一个 4043，不泄漏存在性。
	// 用 ExistsOwnedByUser 而不是 FindOwned：本轮不拼 prompt，不需要把那行取回来。
	owned, err := s.personaRepo.ExistsOwnedByUser(ctx, userID, req.PersonaID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if !owned {
		return nil, errcode.New(errcode.ErrPersonaNotFound)
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
	go s.runStream(ctx, userID, req.PersonaID, content, ch)
	return ch, nil
}

// runStream 在独立 goroutine 里推进"要回复 → 边收边转 → 收尾落库 → 发 done"。
//
// 它拿到 channel 时响应头已经在路上了，所以任何失败都只能作为 error 事件发出，
// 没有别的出口。
func (s *ChatService) runStream(
	ctx context.Context, userID, personaID uint64, content string, ch chan<- ChatEvent,
) {
	defer close(ch)

	upstream, err := s.ai.StreamChat(ctx, ChatRequest{
		UserID:    userID,
		PersonaID: personaID,
		Message:   content,
	})
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
		UserID:    userID,
		PersonaID: personaID,
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

// InjectNudge 注入一条 role=user、is_nudge=true 的消息，
// 走完整聊天链路生成 assistant 回复并落库，同步返回生成的回复。
//
// 判定与归属校验由调用方负责（TriggerNow 已判过），本方法只做「注入 → 生成 → 落库」。
// 生成失败原样上抛，调用方按 5001 出口；本方法不做任何降级（spec §3.a：
// 基础设施错误不能被吞成「未触发」，否则演示时「点了没反应」会变成常态）。
// InjectNudge 注入一条 role=user、is_nudge=true 的消息，
// 走完整聊天链路生成 assistant 回复并落库，同步返回生成的回复。
//
// 与 StreamChat 是同一件事的两种包装：StreamChat 边收边转，因为前端正盯着字出来；
// 本方法先收完再交，因为调用方是定时任务或手动按钮，没有人要看着它一个字一个字出。
// 唯一的区别就是"要不要转发"，所以落库部分共用 persistMessage。
//
// 判定与归属校验由调用方负责（TriggerNow 已判过），本方法只做「注入 → 生成 → 落库」。
// 生成失败原样上抛，调用方按 5001 出口；本方法不做任何降级（spec §3.a：
// 基础设施错误不能被吞成「未触发」，否则演示时「点了没反应」会变成常态）。
func (s *ChatService) InjectNudge(ctx context.Context, in NudgeInput) (*NudgeResult, error) {
	if strings.TrimSpace(in.Content) == "" {
		return nil, errcode.New(errcode.ErrInvalidParams)
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

	upstream, err := s.ai.StreamChat(ctx, ChatRequest{
		UserID:    in.UserID,
		PersonaID: in.PersonaID,
		Message:   in.Content,
	})
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

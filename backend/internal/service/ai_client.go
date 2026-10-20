package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// ChatRequest 是 Go → Python 的请求体（chat-stream spec §2.2）。
// 刻意保持 snake_case：它跟前端那个 {personaId, content} 长得不一样，
// 两边混起来会立刻解析报错，而不是悄悄串味。
//
// Persona / History 是 2026-10-05 新增的**可选**字段（spec §2.2，纯加法）。
// 两者都带 omitempty：为零值时字段整个不出现在 JSON 里，Python 侧拿到自己的默认值，
// 于是「没传」和「传了个空的」在协议上长得一模一样，不必让 Python 去分辨。
type ChatRequest struct {
	UserID    uint64 `json:"user_id"`
	PersonaID uint64 `json:"persona_id"`
	Message   string `json:"message"`

	// Persona 是这个人设的三行文字设定，Python 拿它拼 system prompt。
	// 为 nil 时 Python 不发 system message —— 这是 Week 2 的旧行为，保留它是为了
	// 让旧调用方（以及 spec §4 那条只发三个字段的验收 curl）继续能跑。
	Persona *PersonaBrief `json:"persona,omitempty"`
	// History 是最近若干轮对话，**必须按时间正序**（旧→新）。
	// 顺序错了模型会以为"结果"发生在"原因"之前，而它不会报错，只会答得莫名其妙。
	History []HistoryTurn `json:"history,omitempty"`
}

// PersonaBrief 是 Persona 里参与生成的那三列。
//
// 不复用 model.Persona：那个结构带着 ID / State / LastMessageAt 等一整行，
// 全发过去等于把数据库表结构钉进跨服务的协议里，加一列就要两边一起改。
type PersonaBrief struct {
	Name            string `json:"name"`
	PersonalityDesc string `json:"personality_desc"`
	SpeakingStyle   string `json:"speaking_style"`
}

// HistoryTurn 是一条历史消息。Role 的取值与 chat_messages 表的 CHECK 约束一致，
// 直接用 model.MessageRole 转过来，不在这里另立一套字符串。
type HistoryTurn struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamEvent 是 ai-service 下发的一条事件，Type 只有三种，见下面的常量。
type StreamEvent struct {
	Type    string // delta | end | error
	Text    string // 仅 delta
	Code    int    // 仅 error
	Message string // 仅 error
}

const (
	EventDelta = "delta"
	EventEnd   = "end"
	EventError = "error"
)

// ScheduleParseResult 是 POST /schedule/parse 的结果（TECH_DESIGN §5.7）。
//
// HasIntent=false 表示这句话不是提醒委托（陈述句），调用方不应建日程也不应回问；
// NeedClarify=true 表示有意图但时间/事件没解析出来，调用方应让 AI 回问。
// RemindAt 用指针：Python 在非成功出口不返回该字段，nil 与"零值时间"必须能区分。
type ScheduleParseResult struct {
	HasIntent   bool
	NeedClarify bool
	Content     string
	RemindAt    *time.Time
}

// MemoryCandidate 是 POST /memory/extract 返回的一条候选记忆。
//
// 没有 user_id / persona_id：归属字段只能由 Go 侧从本轮对话补上，
// 绝不能采信 AI 服务的返回体（user-memory spec §3.3 约定 1）。
type MemoryCandidate struct {
	MemoryType      string  `json:"memory_type"`
	Content         string  `json:"content"`
	ImportanceScore float64 `json:"importance_score"`
}

// MemoryExtractResult 是 POST /memory/extract 的完整返回。
//
// ProfileUpdates 保留原始 JSON：画像键由提取链路决定，Go 侧不定义固定 struct，
// 否则未知键会在读写往返中静默丢失（user-profile spec §3.4）。
type MemoryExtractResult struct {
	Memories       []MemoryCandidate
	ProfileUpdates json.RawMessage
}

// MemoryStoreItem 是 POST /memory/store 的一条记忆。
//
// ID 是 PostgreSQL 主键，Python 侧拿它当 Chroma 文档 ID —— 重试同一条记忆时
// 是 upsert 而不是再插一份，所以这个字段不能省。
type MemoryStoreItem struct {
	ID              uint64  `json:"id"`
	MemoryType      string  `json:"memory_type"`
	Content         string  `json:"content"`
	ImportanceScore float64 `json:"importance_score"`
}

// AIClient 是 Go 侧调用 Python AI 服务的唯一出入口。
// 只放已经落地的内部端点：流式对话、日程解析、记忆提取、向量写入。
type AIClient interface {
	// StreamChat 发起一次流式生成。
	//
	// 返回的 error 只表示"没能开始"（连不上 / 非 200）：函数把 channel 交出去
	// 就返回了，之后再出的错没有机会走 error，只能从 channel 里发 EventError。
	StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)

	// ParseSchedule 调 Python 规则解析器，把用户原话解析成日程。
	// 只负责一次内部 HTTP 调用，不落库、不判断归属（那些在 ChatService 里做）。
	ParseSchedule(ctx context.Context, message string) (*ScheduleParseResult, error)

	// ExtractMemories 调 Python 提取候选记忆与画像更新。
	// 只负责一次内部 HTTP 调用，不落库、不判断归属（那些在 ChatService 里做）。
	ExtractMemories(ctx context.Context, message string) (*MemoryExtractResult, error)

	// StoreMemories 把已落 PostgreSQL 的记忆同步进 ChromaDB。
	// 归属字段由 Go 侧赋值，Python 只用它们写 metadata 与过滤条件。
	//
	// 返回实际写入条数：MEMORY_BACKEND=recent 时 Python 返回 0 且不报错，
	// 调用方不能把这种 200 当成"索引已建好"。
	StoreMemories(
		ctx context.Context, userID, personaID uint64, memories []MemoryStoreItem,
	) (int, error)
}

type aiHTTPClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewAIClient 构造 HTTP 实现。
func NewAIClient(cfg config.AIConfig) AIClient {
	// 克隆默认 Transport，只改"等响应头"的超时。
	// 不设 http.Client.Timeout：它算的是"从发出到读完 body"的总时间，
	// 而流式回复本来就要持续几十秒，设了会把正在流的回复直接掐断。
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = 10 * time.Second

	return &aiHTTPClient{
		baseURL: strings.TrimRight(cfg.ServiceURL, "/"),
		token:   cfg.ServiceToken,
		http:    &http.Client{Transport: tr},
	}
}

func (c *aiHTTPClient) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/internal/chat/stream", bytes.NewReader(body),
	)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// 必须与 ai-service 的 AI_SERVICE_TOKEN 逐字相同。
	// 不一致的表现是恒 401，且日志里看不出是配置问题（AGENTS §4.3）。
	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		// 还没开始流就失败：属于"AI 服务不可用"，不是"生成失败"
		return nil, errcode.Wrap(errcode.ErrAIUnavailable, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, errcode.Wrap(
			errcode.ErrAIUnavailable, fmt.Errorf("ai-service status %d", resp.StatusCode),
		)
	}

	ch := make(chan StreamEvent)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		// 契约 §6 把事件块分隔符写死成 \n\n。不能用默认的"按行切"：
		// 一个事件块横跨三行（event: / data: / 空行），按行切会被切碎。
		scanner.Split(splitSSEBlock)

		for scanner.Scan() {
			ev, ok := parseSSEBlock(scanner.Text())
			if !ok {
				continue
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
				// 调用方已经不要了（浏览器断开）。继续推会永远卡在这里，
				// 这条 goroutine 就再也回收不了。
				return
			}
		}

		// 读到一半断了。ctx 没被取消才算真出错 —— 用户主动断开不算错。
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			select {
			case ch <- StreamEvent{
				Type:    EventError,
				Code:    int(errcode.ErrLLMFailed),
				Message: errcode.ErrLLMFailed.Message(),
			}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

// ParseSchedule 调用 ai-service 的规则解析器（TECH_DESIGN §5.7）。
//
// 返回值可能是三种互斥形态，由调用方按 HasIntent / NeedClarify 分流；
// 本方法不把"解析不出"当错误——陈述句与需要澄清都是正常答案，不是故障。
func (c *aiHTTPClient) ParseSchedule(
	ctx context.Context, message string,
) (*ScheduleParseResult, error) {
	body, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/schedule/parse", bytes.NewReader(body),
	)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrAIUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errcode.Wrap(
			errcode.ErrAIUnavailable, fmt.Errorf("ai-service status %d", resp.StatusCode),
		)
	}

	// remind_at 用 *string 而不是 *time.Time：三个出口里只有成功那一个带该字段，
	// 且空字符串必须按"没给"处理，不能因为一个空值把整次解析判成失败。
	var payload struct {
		HasIntent   bool    `json:"has_intent"`
		NeedClarify bool    `json:"need_clarify"`
		Content     string  `json:"content"`
		RemindAt    *string `json:"remind_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}

	result := &ScheduleParseResult{
		HasIntent:   payload.HasIntent,
		NeedClarify: payload.NeedClarify,
		Content:     payload.Content,
	}
	if payload.RemindAt != nil && *payload.RemindAt != "" {
		// RFC3339Nano 同时接受带小数秒和不带小数秒的形态；
		// 规则解析器当前不带小数秒，换成 LLM 解析后可能带。
		remindAt, err := time.Parse(time.RFC3339Nano, *payload.RemindAt)
		if err != nil {
			return nil, errcode.Wrap(errcode.ErrInternal, err)
		}
		result.RemindAt = &remindAt
	}
	return result, nil
}

// ExtractMemories 调用 ai-service 的记忆提取端点（TECH_DESIGN §5.3）。
//
// 规则档与 LLM 档返回同一份结构，调用方不需要知道 MEMORY_EXTRACT_BACKEND 的值。
// 失败按 ErrAIUnavailable 返回：异步调用方只记 warn，不影响主对话流程。
func (c *aiHTTPClient) ExtractMemories(
	ctx context.Context, message string,
) (*MemoryExtractResult, error) {
	body, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/memory/extract", bytes.NewReader(body),
	)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrAIUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errcode.Wrap(
			errcode.ErrAIUnavailable, fmt.Errorf("ai-service status %d", resp.StatusCode),
		)
	}

	var payload struct {
		Memories       []MemoryCandidate `json:"memories"`
		ProfileUpdates json.RawMessage   `json:"profile_updates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}
	return &MemoryExtractResult{
		Memories:       payload.Memories,
		ProfileUpdates: payload.ProfileUpdates,
	}, nil
}

// StoreMemories 调用 ai-service 的向量写入端点（TECH_DESIGN §5.3 双写）。
//
// 只在 PostgreSQL 事务提交成功后调用：PG 是唯一事实来源，向量库只是索引。
// 失败按 ErrAIUnavailable 返回，异步调用方只记 warn，不回滚已落库的记忆。
func (c *aiHTTPClient) StoreMemories(
	ctx context.Context, userID, personaID uint64, memories []MemoryStoreItem,
) (int, error) {
	if len(memories) == 0 {
		return 0, nil
	}

	body, err := json.Marshal(struct {
		UserID    uint64            `json:"user_id"`
		PersonaID uint64            `json:"persona_id"`
		Memories  []MemoryStoreItem `json:"memories"`
	}{
		UserID:    userID,
		PersonaID: personaID,
		Memories:  memories,
	})
	if err != nil {
		return 0, errcode.Wrap(errcode.ErrInternal, err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/memory/store", bytes.NewReader(body),
	)
	if err != nil {
		return 0, errcode.Wrap(errcode.ErrInternal, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return 0, errcode.Wrap(errcode.ErrAIUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, errcode.Wrap(
			errcode.ErrAIUnavailable, fmt.Errorf("ai-service status %d", resp.StatusCode),
		)
	}

	var payload struct {
		Stored int `json:"stored"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, errcode.Wrap(errcode.ErrInternal, err)
	}
	return payload.Stored, nil
}

// splitSSEBlock 按 \n\n 切块，切出来的内容不含结尾那两个换行。
func splitSSEBlock(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		return i + 2, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// parseSSEBlock 把 "event: delta\ndata: {...}" 变成 StreamEvent。
func parseSSEBlock(block string) (StreamEvent, bool) {
	var ev StreamEvent
	var data string

	for _, line := range strings.Split(block, "\n") {
		switch {
		case strings.HasPrefix(line, "event:"):
			ev.Type = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	// 空块（两个换行之间什么都没有）跳过
	if ev.Type == "" {
		return StreamEvent{}, false
	}

	switch ev.Type {
	case EventDelta:
		var payload struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal([]byte(data), &payload)
		ev.Text = payload.Text
	case EventError:
		var payload struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal([]byte(data), &payload)
		ev.Code, ev.Message = payload.Code, payload.Message
	}
	// end 事件的 data 是 {}，不需要解析任何东西
	return ev, true
}

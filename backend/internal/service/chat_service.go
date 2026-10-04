package service

import (
	"context"
	"time"

	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

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
type ChatService struct {
	// [TODO PR 3] db / messageRepo / personaRepo / aiClient 随 PR 3 补齐。
	// 本次先留空结构体：目的是把 InjectNudge 的签名先交出去，
	// 让 proactive 的 TriggerNow 能接线（接口先行，实现后补）。
}

// NewChatService 构造 ChatService。
//
// [TODO PR 3] 将接收 db 与 aiClient。届时只影响 router.go / main.go 两个调用点，
// 调用方 ProactiveService 持有的字段类型不变，它那边不用动。
func NewChatService() *ChatService { return &ChatService{} }

// InjectNudge 注入一条 role=user、is_nudge=true 的消息，
// 走完整聊天链路生成 assistant 回复并落库，同步返回生成的回复。
//
// 判定与归属校验由调用方负责（TriggerNow 已判过），本方法只做「注入 → 生成 → 落库」。
// 生成失败原样上抛，调用方按 5001 出口；本方法不做任何降级（spec §3.a：
// 基础设施错误不能被吞成「未触发」，否则演示时「点了没反应」会变成常态）。
func (s *ChatService) InjectNudge(ctx context.Context, in NudgeInput) (*NudgeResult, error) {
	// [TODO PR 3] 落 user 消息 → 调 ai-service 流式 → 攒全文 → 落 assistant 消息
	// → 同一事务内推进 personas.last_message_at。
	//
	// 返回 ErrInternal 而不是 nil, nil：nil 会让调用方把「没实现」当成「触发成功」，
	// 手动端点会吐一条假消息（契约 §9 的响应体是生成的回复，不是空壳）。
	return nil, errcode.New(errcode.ErrInternal)
}

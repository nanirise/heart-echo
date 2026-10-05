package dto

import (
	"time"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// StreamChatRequest 是 POST /chat/stream 的请求体（契约 §6）。
// 只有两个字段，没有 userId —— 它从 Token 取，前端永远不传（AGENTS §4.3）。
type StreamChatRequest struct {
	PersonaID uint64 `json:"personaId"`
	Content   string `json:"content"`
}

// ChatMessageItem 是一条对话消息的响应形状（契约 §5）。
//
// 单独定义、不直接返回 model.ChatMessage：model 是**数据库**的形状。
// 将来给表加一列、顺手配上 json tag，那一列就会自动漏进接口；
// 响应形状钉在这里，改表不会悄悄改接口。
type ChatMessageItem struct {
	ID        uint64 `json:"id"`
	PersonaID uint64 `json:"personaId"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	// 内部信号：随历史消息回读供聚合，但界面一律不得渲染，也不得加 SSE 事件（AGENTS §4.4）
	EmotionLabel *string   `json:"emotionLabel"`
	EmotionScore *float64  `json:"emotionScore"`
	IsNudge      bool      `json:"isNudge"`
	CreatedAt    time.Time `json:"createdAt"`
}

// NewChatMessageItem 把数据库行转成响应形状。
func NewChatMessageItem(m model.ChatMessage) ChatMessageItem {
	return ChatMessageItem{
		ID:           m.ID,
		PersonaID:    m.PersonaID,
		Role:         string(m.Role),
		Content:      m.Content,
		EmotionLabel: m.EmotionLabel,
		EmotionScore: m.EmotionScore,
		IsNudge:      m.IsNudge,
		CreatedAt:    m.CreatedAt,
	}
}

// NewChatMessageItems 批量转换，供列表端点用。
func NewChatMessageItems(ms []model.ChatMessage) []ChatMessageItem {
	// 从空切片起步而不是 nil：Go 的 nil slice 会序列化成 null，
	// 前端 v-for 到 null 直接崩（user_memory spec 已定这条口径）。
	items := make([]ChatMessageItem, 0, len(ms))
	for _, m := range ms {
		items = append(items, NewChatMessageItem(m))
	}
	return items
}

package dto

import (
	"time"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// MemoryListQuery 是 GET /memory 的 query 参数（契约 §7）。
//
// form tag 必须逐字 personaId：ShouldBindQuery 只读 form tag 且区分大小写，
// 写成 persona_id 会静默绑不上，PersonaID 恒为 0 → 4001。
type MemoryListQuery struct {
	PersonaID uint64 `form:"personaId" binding:"required"`
}

// MemoryItemResponse 是契约 §7 的 MemoryItem，恰好 6 个字段。
// 没有 userId / embeddingId / embeddingStatus / sourceMessageId：归属与内部状态不进响应体。
type MemoryItemResponse struct {
	ID              uint64           `json:"id"`
	PersonaID       uint64           `json:"personaId"`
	MemoryType      model.MemoryType `json:"memoryType"`
	Content         string           `json:"content"`
	ImportanceScore float64          `json:"importanceScore"`
	CreatedAt       time.Time        `json:"createdAt"`
}

// NewMemoryItemResponse 把实体转成响应结构。转换函数放 DTO 层，model 不依赖 dto。
func NewMemoryItemResponse(m *model.UserMemory) MemoryItemResponse {
	return MemoryItemResponse{
		ID:              m.ID,
		PersonaID:       m.PersonaID,
		MemoryType:      m.MemoryType,
		Content:         m.Content,
		ImportanceScore: m.ImportanceScore,
		CreatedAt:       m.CreatedAt,
	}
}

// PortraitQuery 是 GET /profile/portrait 的 query 参数（契约 §7）。
// 与 MemoryListQuery 同一条 camelCase 陷阱。
type PortraitQuery struct {
	PersonaID uint64 `form:"personaId" binding:"required"`
}

// PortraitResponse 是契约 §7 的 Portrait，恰好 3 个字段；id / userId 不进响应体。
// updatedAt 是指针：没有画像行时必须是 null，值类型会序列化成 "0001-01-01T00:00:00Z"。
type PortraitResponse struct {
	PersonaID   uint64      `json:"personaId"`
	ProfileData model.JSONB `json:"profileData"`
	UpdatedAt   *time.Time  `json:"updatedAt"`
}

// NewPortraitResponse 把实体转成响应结构；p 为 nil 表示该人设还没有画像行，是合法空态。
// 空态必须显式给 "{}"：零值 JSONB 的 MarshalJSON 返回 null，前端 Object.keys(null) 直接崩。
func NewPortraitResponse(p *model.UserProfile, personaID uint64) PortraitResponse {
	if p == nil {
		return PortraitResponse{
			PersonaID:   personaID,
			ProfileData: model.JSONB(`{}`),
		}
	}
	return PortraitResponse{
		PersonaID:   p.PersonaID,
		ProfileData: p.ProfileData,
		UpdatedAt:   &p.UpdatedAt,
	}
}

package dto

import (
	"time"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// ScheduleResponse 是契约 §10 的 Schedule，GET /schedules 的列表项。
// 恰好 6 个字段：userId 是越权防线、sourceMessageId 是溯源列，二者都不进响应体。
type ScheduleResponse struct {
	ID        uint64               `json:"id"`
	PersonaID uint64               `json:"personaId"`
	Content   string               `json:"content"`
	RemindAt  time.Time            `json:"remindAt"`
	Status    model.ScheduleStatus `json:"status"`
	CreatedAt time.Time            `json:"createdAt"`
}

// NewScheduleResponse 把实体转成响应结构。
func NewScheduleResponse(s *model.Schedule) ScheduleResponse {
	return ScheduleResponse{
		ID:        s.ID,
		PersonaID: s.PersonaID,
		Content:   s.Content,
		RemindAt:  s.RemindAt,
		Status:    s.Status,
		CreatedAt: s.CreatedAt,
	}
}

// ListSchedulesQuery 是 GET /schedules 的 query 参数（契约 §10）。
//
// form tag 必须逐字 personaId：ShouldBindQuery 只读 form tag 且区分大小写，
// 写成 persona_id 会静默绑不上，PersonaID 恒为 0 → 4001（与 proactive 同款陷阱）。
type ListSchedulesQuery struct {
	PersonaID uint64 `form:"personaId" binding:"required"`
}

package dto

import (
	"time"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// SettingsResponse 是契约 §9 的 Settings，GET / PUT 两个端点共用的响应 data。
// 恰好 6 个字段；id / user_id 不进响应体（model 上是 json:"-"）。
type SettingsResponse struct {
	PersonaID   uint64 `json:"personaId"`
	Enabled     bool   `json:"enabled"`
	IntervalMin int    `json:"intervalMin"`
	IntervalMax int    `json:"intervalMax"`
	DailyLimit  int    `json:"dailyLimit"`
	// 必须指针：NULL = 从未触发过；值类型会序列化成 0001-01-01，
	// 消费方会以为它刚被触发过（proactive-setting §1.5）。
	LastNudgeAt *time.Time `json:"lastNudgeAt"`
}

// NewSettingsResponse 把实体转成响应结构。
func NewSettingsResponse(s *model.ProactiveSetting) SettingsResponse {
	return SettingsResponse{
		PersonaID:   s.PersonaID,
		Enabled:     s.Enabled,
		IntervalMin: s.IntervalMin,
		IntervalMax: s.IntervalMax,
		DailyLimit:  s.DailyLimit,
		LastNudgeAt: s.LastNudgeAt,
	}
}

// GetSettingsQuery 是 GET /proactive/settings 的 query 参数（proactive-setting §2.1）。
type GetSettingsQuery struct {
	// form tag 必须逐字 personaId：ShouldBindQuery 只读 form tag 且区分大小写，
	// 写成 persona_id 会静默绑不上，PersonaID 恒为 0。
	PersonaID uint64 `form:"personaId" binding:"required"`
}

// UpdateSettingsRequest 是 PUT /proactive/settings 的请求体（proactive-setting §2.2）。
// 五个字段一律必传：personaId + 四个可改字段。
//
// 四个可改字段必须是指针：bool / int 的零值（enabled=false）用值类型会被
// binding:"required" 判成"没传" → 4001，开关永远关不掉（§1.1 第一层陷阱）。
// 范围与跨字段规则不写进 tag：它们在 service 层校验（§2.2 要求 2）。
type UpdateSettingsRequest struct {
	PersonaID   uint64 `json:"personaId"   binding:"required"`
	Enabled     *bool  `json:"enabled"     binding:"required"`
	IntervalMin *int   `json:"intervalMin" binding:"required"`
	IntervalMax *int   `json:"intervalMax" binding:"required"`
	DailyLimit  *int   `json:"dailyLimit"  binding:"required"`
}

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

// TriggerRequest 是 POST /proactive/trigger 的请求体（契约 §9）。
// personaId 必传且非 0：缺失在参数层就是 4001——此刻还没有任何归属可供校验，
// 不能退化成 personaId=0 查库（会命中空集，把调用方 bug 伪装成"未触发"）。
type TriggerRequest struct {
	PersonaID uint64 `json:"personaId" binding:"required"`
}

// TriggerResponse 是 POST /proactive/trigger 的响应 data（契约 §9）：
// 生成出来的 assistant 回复，**不是**被注入的那行 [nudge]。
//
// 判定未通过时返回零值（messageId=0 / content="" / createdAt=null）：契约只给了成功形态，
// 没有"被拦下"的表达，复用零值形态是群内对齐的口径（spec §4 的未定义行为按此落地），
// 前端按 messageId === 0 区分。
//
// CreatedAt 必须是指针：零值要序列化成 null，不是 0001-01-01。
type TriggerResponse struct {
	MessageID uint64     `json:"messageId"`
	Content   string     `json:"content"`
	CreatedAt *time.Time `json:"createdAt"`
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

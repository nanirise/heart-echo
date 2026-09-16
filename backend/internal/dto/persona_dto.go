package dto

import (
	"encoding/json"
	"time"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// CreatePersonaRequest 是 POST /personas 的请求体。
// 刻意不声明 userId：字段不存在，就没有被前端传进来的可能（user_id 只能来自 Token）。
type CreatePersonaRequest struct {
	Name            string `json:"name"            binding:"required,max=50"`
	PersonalityDesc string `json:"personalityDesc" binding:"required,max=2000"`
	SpeakingStyle   string `json:"speakingStyle"   binding:"required,max=255"`
}

// UpdatePersonaRequest 是 PUT /personas/:id 的请求体。
// 字段与 Create 相同，但 PUT 是整体替换不是部分更新：三个字段一律必填。
// 刻意不声明 state / familiarity：客户端多传这些应被静默忽略（Gin 默认忽略未知字段）。
type UpdatePersonaRequest struct {
	Name            string `json:"name"            binding:"required,max=50"`
	PersonalityDesc string `json:"personalityDesc" binding:"required,max=2000"`
	SpeakingStyle   string `json:"speakingStyle"   binding:"required,max=255"`
}

// PersonaResponse 是契约 §4 冻结的人设响应结构，字段名逐字对齐。
// 没有 userId —— 归属信息不进响应体。
type PersonaResponse struct {
	ID              uint64      `json:"id"`
	Name            string      `json:"name"`
	PersonalityDesc string      `json:"personalityDesc"`
	SpeakingStyle   string      `json:"speakingStyle"`
	State           model.JSONB `json:"state"`
	Familiarity     int         `json:"familiarity"`
	LastMessageAt   *time.Time  `json:"lastMessageAt"`
	CreatedAt       time.Time   `json:"createdAt"`
}

// NewPersonaResponse 把实体转成响应结构，familiarity 从 state 里拍平出来。
// state 原样透传，未知键不丢。
func NewPersonaResponse(p *model.Persona) PersonaResponse {
	var state struct {
		Familiarity int `json:"familiarity"`
	}
	// 解析失败与键不存在都取 0（语义 = 初识）。DDL 默认值是 '{}'，
	// 历史数据或别的模块写入的 state 都可能没有这个键，不能让列表接口因此 500。
	_ = json.Unmarshal(p.State, &state)

	return PersonaResponse{
		ID:              p.ID,
		Name:            p.Name,
		PersonalityDesc: p.PersonalityDesc,
		SpeakingStyle:   p.SpeakingStyle,
		State:           p.State,
		Familiarity:     state.Familiarity,
		LastMessageAt:   p.LastMessageAt,
		CreatedAt:       p.CreatedAt,
	}
}

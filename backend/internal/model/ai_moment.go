package model

import "time"

// AIMoment AI 朋友圈动态，对应数据库表 ai_moments。
// 动态只由定时任务产生，没有手动触发接口（AGENTS §4.6）。
type AIMoment struct {
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 外键列：不带 autoIncrement，否则动态会挂到不存在的人设上（spec §1.1）
	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_moments_persona_time,priority:1" json:"personaId"`

	Content string `gorm:"column:content;type:text;not null" json:"content"`

	// 内部信号：只决定生成语气，不进任何响应体（AGENTS §4.4）。前端物理拿不到，从结构上杜绝误渲染。
	// 指针 = 可空：分析不出情绪时是 NULL，不是空串。
	EmotionLabel *string `gorm:"column:emotion_label;type:varchar(20)" json:"-"`

	LikeCount int `gorm:"column:like_count;type:integer;not null;default:0" json:"likeCount"`

	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime;index:idx_moments_persona_time,priority:2,sort:desc" json:"createdAt"`

	// 仅供 GORM 生成外键约束用，不参与序列化。只写标量 PersonaID 时不会建出 ON DELETE CASCADE。
	Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName 显式指定表名。
func (AIMoment) TableName() string { return "ai_moments" }

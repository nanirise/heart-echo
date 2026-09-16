package model

import "time"

// UserProfile 画像表：一人设一份（persona_id 唯一），全项目唯一有 UPSERT 语义的表。
type UserProfile struct {
	ID          uint64    `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"-"`                                   // bigint 而非 bigserial：避免关联复制污染下游外键
	UserID      uint64    `gorm:"column:user_id;type:bigint;not null" json:"-"`                                              // 越权防线载体，不进响应体
	PersonaID   uint64    `gorm:"column:persona_id;type:bigint;not null;uniqueIndex" json:"personaId"`                       // uniqueIndex 是 UPSERT 冲突推断依据，删则退化
	ProfileData JSONB     `gorm:"column:profile_data;type:jsonb;not null;default:'{}'" json:"profileData"`                   // default 单引号不能省，否则建表失败
	UpdatedAt   time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now();autoUpdateTime" json:"updatedAt"` // autoUpdateTime 而非 autoCreateTime
}

// TableName 显式指定，GORM 复数化会变 user_profiles。
func (UserProfile) TableName() string {
	return "user_profile"
}

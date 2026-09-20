package model

import "time"

// Persona 人设表，一个人设 = 一个 AI 伴侣 = 一个对话（无独立会话表，本表同时就是对话列表）。
type Persona struct {
	// type:bigint 不能省：GORM 建关联时会把被引用主键的 DataType 复制到外键列，写成 bigserial
	// 会让下游外键长出 nextval 默认值（同 user.go）。
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 越权防线的载体，不进响应体（契约 §4 的 Persona 实体无 userId）。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null;index:idx_personas_user_last_msg,priority:1" json:"-"`

	// 无 UNIQUE：人设允许重名、数量不限（总纲 §0.3）。
	Name string `gorm:"column:name;type:varchar(50);not null" json:"name"`

	PersonalityDesc string `gorm:"column:personality_desc;type:text;not null" json:"personalityDesc"`

	SpeakingStyle string `gorm:"column:speaking_style;type:varchar(255);not null" json:"speakingStyle"`

	// 必须原样保留：改成固定字段的 struct，读写一次就会静默丢掉未知键（如 self_note 演化字段）。
	State JSONB `gorm:"column:state;type:jsonb;not null;default:'{}'" json:"state"`

	// 必须是指针：契约要求「从未聊过为 null」。查询排序需显式补 NULLS LAST，GORM 的 index tag 表达不了。
	LastMessageAt *time.Time `gorm:"column:last_message_at;type:timestamptz;index:idx_personas_user_last_msg,priority:2,sort:desc" json:"lastMessageAt"`

	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime" json:"createdAt"`

	// 仅供 GORM 生成 ON DELETE CASCADE 外键；Create 时保持零值，否则会连带写 users 表。
	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (Persona) TableName() string {
	return "personas"
}

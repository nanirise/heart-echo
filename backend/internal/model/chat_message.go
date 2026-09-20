package model

import "time"

// MessageRole 消息角色。用自定义类型而非裸 string：写错在编译期暴露，DB 的 CHECK 是第二道闸门。
type MessageRole string

const (
	// RoleUser 用户侧消息。主动消息 / 日程提醒注入的 [nudge] 也是它——标的是"这一轮由谁发起"。
	RoleUser MessageRole = "user"

	RoleAssistant MessageRole = "assistant"
)

// ChatMessage 消息表。一个人设 = 一个对话，消息直接挂 persona_id；本表是 SSE 对话 / 主动消息 /
// 日程提醒三条链路的共同落点，三条链路共用 message_repo.Create，不要各自写一份 INSERT。
type ChatMessage struct {
	// type:bigint 不能省：user_memory.source_message_id 引用本列，关联复制会让它长出 nextval 默认值。
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// ⛔ 绝不能带 autoIncrement：本列是外键不是主键，带上会让每条消息拿到与用户无关的 id，越权防线当场失效。
	// 不进响应体（契约 §5 无 userId）。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null;index:idx_messages_user_id" json:"-"`

	// ⛔ 同样不带 autoIncrement（外键列）。与 CreatedAt 共用组合索引，两列的顺序不能颠倒。
	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_messages_persona_time,priority:1" json:"personaId"`

	// check: 的约束名由 GORM 派生（chk_chat_messages_role）；写坏不会报错，只是整条约束静默丢失。
	Role MessageRole `gorm:"column:role;type:varchar(10);not null;check:role IN ('user','assistant')" json:"role"`

	Content string `gorm:"column:content;type:text;not null" json:"content"`

	// 两个情绪字段都必须是指针（契约允许 null：没分析出 ≠ 标签为空串）。⚠️ 内部信号：随历史消息回读
	// 供聚合，但界面一律不得渲染，也不得加 SSE 事件（AGENTS §4.4）。
	EmotionLabel *string `gorm:"column:emotion_label;type:varchar(20)" json:"emotionLabel"`

	EmotionScore *float64 `gorm:"column:emotion_score;type:numeric(4,3)" json:"emotionScore"`

	// 标记该 user 消息是系统注入的 [nudge]；不改变 Role（主动消息的 role 仍是 user）。
	IsNudge bool `gorm:"column:is_nudge;type:boolean;not null;default:false" json:"isNudge"`

	// ⚠️ 排序必须补 id DESC 兜底：NOW() 是事务开始时间，同事务多行时间戳相同，只按它排会翻页重复或漏项。
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime;index:idx_messages_persona_time,priority:2" json:"createdAt"`

	// 仅供 GORM 生成 ON DELETE CASCADE 外键；Create 时保持零值，否则会连带写 users / personas 表。
	User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (ChatMessage) TableName() string {
	return "chat_messages"
}

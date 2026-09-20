package model

import "time"

// MemoryType 记忆类型（fact / preference / event）。没有 emotion——情绪是内部信号，不入长期记忆（AGENTS §4.4）。
type MemoryType string

const (
	MemoryTypeFact       MemoryType = "fact"
	MemoryTypePreference MemoryType = "preference"
	MemoryTypeEvent      MemoryType = "event"
)

// UserMemory 记忆表，一个人设一份（与 chat_messages 同粒度）。只写不改：无更新 / 删除接口。
type UserMemory struct {
	// type:bigint 不能省：GORM 关联复制会让下游外键列长出 nextval 默认值（全项目已踩三次）。
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// ⛔ 外键列，绝不能带 autoIncrement（带上会让每条记忆拿到与用户无关的 id，越权防线失效）；不进响应体。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null;index:idx_memory_user_id" json:"-"`

	// ⛔ 同样不带 autoIncrement（外键列）。与 MemoryType 共用组合索引，persona_id 必须是首列。
	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_memory_persona_type,priority:1" json:"personaId"`

	// ⛔ 不加 check: tag——权威 DDL 上**没有**这条约束（chat_messages.role 才有），加了就是偏离 DDL。
	MemoryType MemoryType `gorm:"column:memory_type;type:varchar(20);not null;index:idx_memory_persona_type,priority:2" json:"memoryType"`

	Content string `gorm:"column:content;type:text;not null" json:"content"`

	// 必须是指针：本列可空，阶段一恒为 NULL。json:"-" 让前端物理上拿不到，杜绝误渲染。
	EmbeddingID *string `gorm:"column:embedding_id;type:varchar(64)" json:"-"`

	// default: 后的单引号不能省（不带引号建表失败）；GORM 会把它显式写进 INSERT，不走数据库默认值。
	EmbeddingStatus string `gorm:"column:embedding_status;type:varchar(10);not null;default:'pending';index:idx_memory_embedding_status" json:"-"`

	// 值类型是刻意的：DDL 上是 NOT NULL 的必写字段，不需要"零值 = 未设置"的语义。
	ImportanceScore float64 `gorm:"column:importance_score;type:numeric(4,3);not null" json:"importanceScore"`

	// 必须是指针（SET NULL 的前提是列可空）；值类型会被建成 NOT NULL，删消息时直接报错。
	SourceMessageID *uint64 `gorm:"column:source_message_id;type:bigint" json:"-"`

	// ⚠️ 排序必须补 id DESC 兜底：记忆批量写入，同事务多行时间戳相同，只按它排会翻页重复或漏项。
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime" json:"createdAt"`

	// 仅供 GORM 生成外键约束；Create 时保持零值，否则会连带写 users / personas / chat_messages。
	User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	// ⚠️ 唯一一个 SET NULL（也是唯一一个指针关联）：删除消息只断溯源、不删记忆，写成 CASCADE 就错了。
	SourceMessage *ChatMessage `gorm:"foreignKey:SourceMessageID;references:ID;constraint:OnDelete:SET NULL" json:"-"`
}

// TableName 这里**必需**：GORM 默认把 UserMemory 复数化成 user_memories，与 DDL 表名不一致。
func (UserMemory) TableName() string {
	return "user_memory"
}

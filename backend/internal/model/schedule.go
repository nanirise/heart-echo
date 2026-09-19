package model

import "time"

// ScheduleStatus 日程状态：pending 待提醒 ｜ sent 已提醒 ｜ cancelled 已取消。
// 与 MemoryType 同一手法；本列数据库侧没有 CHECK 约束（spec §1.4），常量是它唯一的防线。
type ScheduleStatus string

const (
	ScheduleStatusPending   ScheduleStatus = "pending"
	ScheduleStatusSent      ScheduleStatus = "sent"
	ScheduleStatusCancelled ScheduleStatus = "cancelled"
)

// Schedule 日程提醒，对应数据库表 schedules。一个人设一份，删账号 / 删人设时级联删除。
type Schedule struct {
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 越权防线载体，契约 §10 的 Schedule 没有 userId，不进响应体。
	// ⛔ 外键列，绝不能带 autoIncrement：带上会让本列长出数据库自增默认值（spec §1.1）。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null" json:"-"`

	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_schedules_persona" json:"personaId"`

	Content string `gorm:"column:content;type:text;not null" json:"content"`

	// 到期扫描是 WHERE status='pending' AND remind_at <= NOW()，故组合索引 status 在前。
	// ⚠️ 取日程必须写成 ORDER BY remind_at ASC, id DESC：兜底列钉死同值顺序（spec §1.3）。
	RemindAt time.Time `gorm:"column:remind_at;type:timestamptz;not null;index:idx_schedules_due,priority:2" json:"remindAt"`

	Status ScheduleStatus `gorm:"column:status;type:varchar(10);not null;default:'pending';index:idx_schedules_due,priority:1" json:"status"`

	// 指针是 SET NULL 的前提（值类型会建出 NOT NULL，删消息时直接报错）；溯源断链不删日程本身。
	SourceMessageID *uint64 `gorm:"column:source_message_id;type:bigint" json:"-"`

	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime" json:"createdAt"`

	// 仅供 GORM 生成外键约束用，不参与序列化，Create 时不要赋值（否则会连带写 users / personas / chat_messages）。
	// 三个关联字段缺一不可，缺哪个就没有哪个 ON DELETE（spec §1.0 / §1.5）。
	User          User         `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Persona       Persona      `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	SourceMessage *ChatMessage `gorm:"foreignKey:SourceMessageID;references:ID;constraint:OnDelete:SET NULL" json:"-"`
}

// TableName 显式指定表名。
func (Schedule) TableName() string { return "schedules" }

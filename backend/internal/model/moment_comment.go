package model

import "time"

// MomentComment 动态评论，对应数据库表 moment_comments。
// 作者二选一：AI 评论挂 persona_id，用户评论挂 user_id，恰好一个非空（DDL 的 chk_comment_author 兜底）。
type MomentComment struct {
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 外键列，不带 autoIncrement：带上会让本列长出数据库自增默认值，漏传 moment_id 的 INSERT 会静默挂到别的动态下（spec §1.1）
	MomentID uint64 `gorm:"column:moment_id;type:bigint;not null;index:idx_comments_moment,priority:1" json:"momentId"`

	// 条件里含逗号是安全的：GORM 只在逗号前半段是纯标识符（^[\w-]+$）时才把它当约束名，chk_comment_author 正是（spec §2）
	PersonaID *uint64 `gorm:"column:persona_id;type:bigint;check:chk_comment_author,(persona_id IS NOT NULL AND user_id IS NULL) OR (persona_id IS NULL AND user_id IS NOT NULL)" json:"personaId"`

	// 必须是 *uint64：契约 §8 要求非作者的一侧是 null，值类型会序列化成 0，且该列恒非空会撑破 CHECK（spec §1.2）
	// ⚠️ 本列是「评论作者」，不是归属者：查询的越权防线必须走 moment_id → ai_moments → personas.user_id，不要拿本列当闸门（spec §1.3）
	UserID *uint64 `gorm:"column:user_id;type:bigint" json:"userId"`

	Content string `gorm:"column:content;type:text;not null" json:"content"`

	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime;index:idx_comments_moment,priority:2" json:"createdAt"`

	// 仅供 GORM 生成外键约束用，不参与序列化。三条都必需，缺一条就没有那个 ON DELETE CASCADE。
	Moment  AIMoment `gorm:"foreignKey:MomentID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Persona Persona  `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	User    User     `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName 显式指定表名。
func (MomentComment) TableName() string { return "moment_comments" }

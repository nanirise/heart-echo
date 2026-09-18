package model

import "time"

// MomentLike 动态点赞，对应数据库表 moment_likes。
// 幂等由 uq_moment_like(user_id, moment_id) 保证：同一用户对同一动态只能有一行。
type MomentLike struct {
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 外键列，不带 autoIncrement：带上会让本列长出数据库自增默认值，漏传的 INSERT 会静默落到别的行上（spec §1.1）
	UserID uint64 `gorm:"column:user_id;type:bigint;not null;uniqueIndex:uq_moment_like,priority:1;index:idx_likes_user" json:"userId"`

	// 幂等约束的另一半。列顺序必须与 DDL 一致（user_id 在前），下游按这个顺序匹配。
	// 下游只能写 ON CONFLICT (user_id, moment_id) DO NOTHING；写成 ON CONFLICT ON CONSTRAINT uq_moment_like 会直接报错（spec §1.2）
	MomentID uint64 `gorm:"column:moment_id;type:bigint;not null;uniqueIndex:uq_moment_like,priority:2;index:idx_likes_moment" json:"momentId"`

	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime" json:"createdAt"`

	// ⚠️ 计数列不在本表，在 ai_moments：真值以本表行数为准，两者必须同事务维护（spec §1.3）
	// 仅供 GORM 生成外键约束用，不参与序列化。两条都必需，缺一条就没有那个 ON DELETE CASCADE。
	User   User     `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Moment AIMoment `gorm:"foreignKey:MomentID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName 显式指定表名。
func (MomentLike) TableName() string { return "moment_likes" }

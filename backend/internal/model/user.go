package model

import "time"

// User 用户表结构体，对应数据库表 users
type User struct {
	ID       uint64 `gorm:"column:id;type:bigserial;primaryKey;autoIncrement" json:"id"`
	Username string `gorm:"column:username;type:varchar(20);not null;uniqueIndex" json:"username"`
	Email    string `gorm:"column:email;type:varchar(100);not null;uniqueIndex" json:"email"`
	//   json:"-"              JSON 字段名；忽略该字段，彻底不出现在响应里（安全防线！）
	PasswordHash string    `gorm:"column:password_hash;type:varchar(100);not null" json:"-"`
	AvatarURL    *string   `gorm:"column:avatar_url;type:varchar(255)" json:"avatarUrl"`
	CreatedAt    time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now();autoUpdateTime" json:"updatedAt"`
}

// TableName 显式指定表名。
// GORM 默认把结构体名 User 复数化为 users，恰好一致；显式实现可避免将来改名或配置变化导致映射错误。
func (User) TableName() string {
	return "users"
}

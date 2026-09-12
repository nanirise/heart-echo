package model

import "time"

// User 用户表结构体，对应数据库表 users
type User struct {
	// 对应 SQL：id BIGSERIAL PRIMARY KEY
	//   column:id          指定数据库列名为 id（不写时 GORM 自动按字段名转 snake_case）
	//   type:bigserial     PostgreSQL 列类型：8 字节自增序列
	//   primaryKey         声明为主键
	//   autoIncrement      插入时值为 0 则由数据库序列生成（BIGSERIAL 的自增语义）
	//   json:"id"          JSON 序列化/反序列化时的字段名
	ID uint64 `gorm:"column:id;type:bigserial;primaryKey;autoIncrement" json:"id"`

	// 对应 SQL：username VARCHAR(20) NOT NULL UNIQUE
	//   column:username    列名 username
	//   type:varchar(20)   列类型 varchar(20)，最长 20 字符
	//   not null           非空约束
	//   uniqueIndex        创建唯一索引（对应 UNIQUE 约束；比 unique 更推荐——unique 对多列会生成重名索引，uniqueIndex 行为可预期
	//   json:"username"    JSON 字段名
	Username string `gorm:"column:username;type:varchar(20);not null;uniqueIndex" json:"username"`

	// 对应 SQL：email VARCHAR(100) NOT NULL UNIQUE
	//   （各 tag 含义与 Username 相同：列名 / 类型 varchar(100) / 非空 / 唯一索引 / JSON 字段名）
	Email string `gorm:"column:email;type:varchar(100);not null;uniqueIndex" json:"email"`

	// 对应 SQL：password_hash VARCHAR(100) NOT NULL（bcrypt）
	//   column:password_hash  列名 password_hash
	//   type:varchar(100)     列类型 varchar(100)（bcrypt 哈希固定 60 字符，100 留有余量）
	//   not null              非空约束
	//   json:"-"              JSON 字段名；忽略该字段，彻底不出现在响应里（安全防线！）
	PasswordHash string `gorm:"column:password_hash;type:varchar(100);not null" json:"-"`

	// 对应 SQL：avatar_url VARCHAR(255)（可空，无 NOT NULL）
	//   column:avatar_url    列名 avatar_url
	//   type:varchar(255)    列类型 varchar(255)
	//   （没有 not null）     列允许 NULL，所以 Go 侧用指针 *string 区分「NULL」与「空字符串」
	//   json:"avatarUrl"     JSON 字段名；指针为 nil 时序列化为 null
	AvatarURL *string `gorm:"column:avatar_url;type:varchar(255)" json:"avatarUrl"`

	// 对应 SQL：created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	//   column:created_at    列名 created_at
	//   type:timestamptz     列类型 timestamptz（带时区时间戳）
	//   not null             非空约束
	//   default:now()        数据库列默认值为 SQL 函数 now()（AutoMigrate 时生成 DEFAULT now()）
	//   autoCreateTime       插入时 GORM 在 Go 层自动写入当前时间——字段名 CreatedAt 本身已触发该约定，显式声明更清晰
	//   json:"createdAt"     JSON 字段名
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime" json:"createdAt"`

	// 对应 SQL：updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	//   autoUpdateTime       每次 Update 记录时 GORM 自动刷新为当前时间——PG 没有 MySQL 的 ON UPDATE 语法，全靠它维持 updated_at
	//   json:"updatedAt"     JSON 字段名
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now();autoUpdateTime" json:"updatedAt"`
}

// TableName 显式指定表名。
// GORM 默认把结构体名 User 复数化为 users，恰好一致；显式实现可避免将来改名或配置变化导致映射错误。
func (User) TableName() string {
	return "users"
}

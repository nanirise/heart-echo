package model

import (
	"database/sql/driver"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// JSONB 是 PostgreSQL jsonb 列的自定义类型。
// 目前被 personas.state 与 user_profile.profile_data 使用（两张表都是字段不固定、
// 演进频繁的结构，见技术文档 §6 的说明），后续同类列请一并复用它。
//
// 为什么自己写而不用 gorm.io/datatypes：为一个 jsonb 列引入该包会连带拖进
// gorm.io/driver/mysql、go-sql-driver/mysql 等一串与本项目（纯 PostgreSQL）无关的依赖，
// 还会与成员 1 后续接入 PGX 驱动时的 go.mod 改动互相冲突。本类型零新增依赖。
//
// 为什么不直接用 json.RawMessage：它经 GORM + pgx 写 jsonb 列时会被当作 bytea 发送，
// 报 `column "state" is of type jsonb but expression is of type bytea`。
// 关键在 Value() 必须返回 string 而不是 []byte —— 见下面的实现。
type JSONB []byte

// Value 实现 driver.Valuer（写入路径）。
//
// 必须返回 string(j)：返回 []byte 会被 pgx 当成 bytea 发送，上面那个报错就是这么来的。
// 空值返回 nil（SQL NULL）；state 列是 NOT NULL DEFAULT '{}'，而本字段带 default 标签，
// GORM 对零值字段会直接省略该列、由数据库默认值兜底，所以正常路径不会走到 NULL。
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

// Scan 实现 sql.Scanner（读取路径）。
// pgx 对 jsonb 返回 []byte，为稳妥同时兼容 string 与 NULL。
func (j *JSONB) Scan(value any) error {
	if j == nil {
		return errors.New("model.JSONB: Scan 的目标是 nil 指针")
	}
	switch v := value.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append((*j)[0:0], v...)
	case string:
		*j = append((*j)[0:0], v...)
	default:
		return fmt.Errorf("model.JSONB: 无法 Scan 类型 %T", value)
	}
	return nil
}

// GormDBDataType 让 AutoMigrate 在字段没写 type:jsonb 标签时也能建出 jsonb 列。
// 没有它，GORM 会按底层类型 []byte 映射成 bytea —— 正是本类型要避免的那个坑。
func (JSONB) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	if db.Dialector.Name() == "postgres" {
		return "jsonb"
	}
	return "json"
}

// MarshalJSON 定制 JSON 序列化（响应体路径）。
//
// ⚠️ 这个方法不能省：JSONB 的底层类型是 []byte，而 encoding/json 对 []byte 的默认行为是
// **base64 编码**。少了它，`"state": {"familiarity": 0}` 会变成 `"state": "eyJmYW1pbGlh..."`，
// 前端解析直接崩，而且它照样能编译、能跑、能"演示"，只在看响应体时才暴露。
// （json.RawMessage 之所以没这个问题，正是因为它自己实现了这两个方法。）
func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}

// UnmarshalJSON 直接保存原始字节，保证未知键（如技术文档 §5.6 预留的 state.self_note）原样保留。
func (j *JSONB) UnmarshalJSON(data []byte) error {
	if j == nil {
		return errors.New("model.JSONB: UnmarshalJSON 的目标是 nil 指针")
	}
	*j = append((*j)[0:0], data...)
	return nil
}

package model

import "time"

// ProactiveSetting 主动消息配置表结构体，对应数据库表 proactive_settings。
// 一个人设一份配置，由 POST /personas 创建人设时同事务播种一行（persona-model §4.3.1）。
type ProactiveSetting struct {
	// json:"-"：契约 §9 的 Settings 里没有 id，不进响应体。
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"-"`

	// 越权防线载体，不进响应体（契约 §9 的 Settings 无 userId）。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null;uniqueIndex:uq_proactive_settings_user_persona,priority:1" json:"-"`

	// 组合唯一索引的另一半。两个字段的索引名必须逐字相同，写错会静默变成两个单列唯一索引。
	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;uniqueIndex:uq_proactive_settings_user_persona,priority:2" json:"personaId"`

	// ⛔ 带 default: 的这几列不能用 struct 写零值 —— GORM 会把 tag 解析出的 Go 值
	//    替换进去（实测）：Create 时 Enabled:false 入库是 true，IntervalMin:0 入库是 30；
	//    Updates(model.ProactiveSetting{Enabled:false}) 更是 Error=nil、RowsAffected=0
	//    的静默空操作，只查 err 的调用方会以为关掉了。要写 false/0 一律用 map。
	//    播种处的 true/30/120/3 是显式可读，不是必须 —— 留零值入库的也是这组值。
	//
	// ⛔ 三列写 type:integer，不能写 type:int：tag 写成 int 时 field.DataType 等于
	//    schema.Int("int")，而 Postgres dialector 对这个类型不看 tag、改按 field.Size
	//    决定列类型（getSchemaIntType：≤16 smallint / ≤32 integer / 否则 bigint），
	//    Go 的 int 在 64 位平台 Size=64 → 建出 bigint，与 DDL 的 INT 不符。实测过。
	Enabled     bool `gorm:"column:enabled;type:boolean;not null;default:true" json:"enabled"`
	IntervalMin int  `gorm:"column:interval_min;type:integer;not null;default:30" json:"intervalMin"`
	IntervalMax int  `gorm:"column:interval_max;type:integer;not null;default:120" json:"intervalMax"`
	DailyLimit  int  `gorm:"column:daily_limit;type:integer;not null;default:3" json:"dailyLimit"`

	// 必须指针：本列可空，"从未触发过"是 NULL，值类型会变成 0001-01-01。
	LastNudgeAt *time.Time `gorm:"column:last_nudge_at;type:timestamptz" json:"lastNudgeAt"`

	// 仅供 GORM 生成 ON DELETE CASCADE 外键用，不参与序列化。
	// 两个都要声明，否则一条外键都不会建，"删账号/删人设级联清配置"就不成立。
	// Create 时不要赋值（保持零值），否则 GORM 会尝试连带写入 users / personas。
	User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName 显式指定表名。
// GORM 默认复数化恰好是 proactive_settings；显式实现避免将来改名或配置变化导致静默建错表。
func (ProactiveSetting) TableName() string {
	return "proactive_settings"
}

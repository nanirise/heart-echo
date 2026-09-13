package model

import "time"

// Persona 人设表结构体，对应数据库表 personas。
// 一个人设 = 一个 AI 伴侣 = 一个对话：本项目的 personas 表同时就是对话列表（无独立会话表）。
type Persona struct {
	// 对应 SQL：id BIGSERIAL PRIMARY KEY
	//   type:bigint（不是 bigserial）+ autoIncrement → 渲染出来仍是 bigserial，但 DataType 是 bigint。
	//   原因同 user.go：关联复制会把本字段的 DataType 传给外键列，写成 bigserial 会让
	//   proactive_settings.persona_id 等下游外键长出多余的 nextval 默认值。见 UserID 的说明。
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 对应 SQL：user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE
	//   json:"-"  契约 §4 的 Persona 实体里没有 userId。这一列是越权防线的载体，不是给前端看的字段；
	//             不要为了"方便调试"改成 json:"userId"——那等于把归属信息暴露出去。
	//   index     与 LastMessageAt 共用 idx_personas_user_last_msg（priority:1 = 组合索引首列）
	//
	//   type:bigint 不能省。GORM 建 belongs-to 关联时会把 User.ID 的 DataType 复制到本字段
	//   （schema/relationship.go 的 copyableDataType 只看类型串里有没有 auto_increment /
	//    primary key，而 "bigserial" 两者都没有，于是照抄）。若 user.go 把 ID 写成 bigserial，
	//   本列就会变成「bigint NOT NULL DEFAULT nextval('personas_user_id_seq')」——Postgres 里
	//   bigserial 的语义就是建序列加默认值——既偏离 DDL，又让漏传 user_id 的 INSERT 静默
	//   拿到一个可能撞上真实用户 id 的序列值。源头已在 user.go 切断，这里保持 bigint 即可。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null;index:idx_personas_user_last_msg,priority:1" json:"-"`

	// 对应 SQL：name VARCHAR(50) NOT NULL
	//   DDL 上没有 UNIQUE：人设允许重名、数量不限制（总纲 §0.3），不要自行加唯一校验
	Name string `gorm:"column:name;type:varchar(50);not null" json:"name"`

	// 对应 SQL：personality_desc TEXT NOT NULL
	PersonalityDesc string `gorm:"column:personality_desc;type:text;not null" json:"personalityDesc"`

	// 对应 SQL：speaking_style VARCHAR(255) NOT NULL
	SpeakingStyle string `gorm:"column:speaking_style;type:varchar(255);not null" json:"speakingStyle"`

	// 对应 SQL：state JSONB NOT NULL DEFAULT '{}'
	//   人格状态（形如 {"familiarity": 0}），不是情绪，别往情绪上靠。
	//   用 model.JSONB（见 jsonb.go）原样保留：技术文档 §5.6 预留了 state.self_note 等演化字段，
	//   一旦改成固定字段的 struct，读写一次就会静默丢掉所有未知键。
	//   读取时不要依赖本列一定含有 familiarity 键（DDL 默认值就是 '{}'），见 spec §4.6 的容错取值。
	State JSONB `gorm:"column:state;type:jsonb;not null;default:'{}'" json:"state"`

	// 对应 SQL：last_message_at TIMESTAMPTZ（可空）
	//   必须是指针：契约要求"从未聊过为 null"，值类型会序列化成 "0001-01-01T00:00:00Z"。
	//   排序时注意：Postgres 的 DESC 默认是 NULLS FIRST，查询里必须显式写 NULLS LAST，
	//   否则从未聊过的人设会排在最前面，与需求正好相反。
	//   GORM 的 index tag 表达不了 NULLS LAST，索引这里只能落到 DESC（查询侧补 NULLS LAST）。
	LastMessageAt *time.Time `gorm:"column:last_message_at;type:timestamptz;index:idx_personas_user_last_msg,priority:2,sort:desc" json:"lastMessageAt"`

	// 对应 SQL：created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime" json:"createdAt"`

	// 仅供 GORM 生成外键约束用；不参与序列化。
	// 只写标量 UserID 时 GORM 不会创建外键，"删人设级联删消息"这条验收项就不成立——
	// 必须声明这个关联字段，AutoMigrate 才会建出 ON DELETE CASCADE。
	// Create 时不要给它赋值（保持零值），否则 GORM 会尝试连带写入 users 表。
	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName 显式指定表名。
// GORM 默认把结构体名 Persona 复数化为 personas，恰好一致；显式实现可避免将来改名或配置变化导致映射错误。
func (Persona) TableName() string {
	return "personas"
}

package model

import "time"

// MessageRole 消息角色。
//
// 用自定义类型 + 常量而不是裸 string：写入方只能从这两个常量里取值，拼错单词在编译期就暴露。
// 数据库侧的 CHECK (role IN ('user','assistant')) 是第二道闸门（见 ChatMessage.Role 的说明）：
// Go 层管得住本项目自己的代码，CHECK 管得住任何绕过 Go 的写入（手工 INSERT、脚本、将来的补偿任务）。
type MessageRole string

const (
	// RoleUser 用户侧消息。
	//   注意：主动消息与日程提醒注入的 [nudge] 也是 user——它标的是"这一轮由谁发起"，
	//   不是"这句话是不是用户本人打的字"，后者看 IsNudge 字段。
	RoleUser MessageRole = "user"

	// RoleAssistant AI 回复。
	RoleAssistant MessageRole = "assistant"
)

// ChatMessage 消息表结构体，对应数据库表 chat_messages。
// 一个人设 = 一个对话（无独立会话表）：消息直接挂 persona_id，删人设时由外键级联删除本表全部行。
//
// 本表是三条写入链路的共同落点：SSE 流式对话（用户消息 + AI 回复）、主动消息（[nudge]）、
// 日程提醒（[nudge]）。三条链路共用 message_repo.Create，不要各自写一份 INSERT。
type ChatMessage struct {
	// 对应 SQL：id BIGSERIAL PRIMARY KEY
	//   type:bigint（不是 bigserial）+ autoIncrement → 渲染出来仍是 bigserial，但 DataType 是 bigint。
	//   原因同 user.go / persona.go：user_memory.source_message_id 引用本列，GORM 建关联时会把这里的
	//   DataType 复制到那一列上（schema/relationship.go 只抄 DataType / GORMDataType / Size，
	//   不抄 autoIncrement）。写成 bigserial 会让下游那一列长出 DEFAULT nextval(...)，漏传
	//   source_message_id 的 INSERT 会静默拿到一个不存在的消息 id——今天看不出来，等写记忆表那天才炸。
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 对应 SQL：user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE
	//   json:"-"        契约 §5 的 ChatMessage 实体里没有 userId（8 个字段逐个数一遍）。这一列是
	//                   越权防线的载体，不是给前端看的字段；不要为了"方便调试"改成 json:"userId"——
	//                   那等于把归属信息暴露出去。
	//   type:bigint     不能省。被引用的 users.id 已写成 bigint，关联复制到此为止（persona 那轮实测：
	//                   序列里不会多出 <表名>_user_id_seq）。
	//   ⛔ 绝不能带 autoIncrement：本列是外键、不是主键。带上自增会让每条消息自动拿到一个与用户
	//      无关的 id，越权防线当场失效——这是本表最危险的一处 tag。
	//   index           idx_messages_user_id 单列索引（DDL）。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null;index:idx_messages_user_id" json:"-"`

	// 对应 SQL：persona_id BIGINT NOT NULL REFERENCES personas(id) ON DELETE CASCADE
	//   json:"personaId" 契约 §5 有这个字段，且它是消息与对话的唯一关联（没有 sessionId）。
	//   ⛔ 同样不带 autoIncrement：外键列。带上自增会让消息挂到一个不存在的 persona 上，
	//      还会顺带把"这个人设属于谁"判错。
	//   index            与 CreatedAt 共用 idx_messages_persona_time（priority:1 = 组合索引首列）。
	//                    历史消息分页只有这一条读路径，索引列顺序不能颠倒。
	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_messages_persona_time,priority:1" json:"personaId"`

	// 对应 SQL：role VARCHAR(10) NOT NULL CHECK (role IN ('user', 'assistant'))
	//   check:          条件里含逗号是安全的：GORM 只在逗号前半段是纯标识符（^[\w-]+$）时才把它当
	//                   约束名，"role IN ('user'" 含空格与括号，不匹配，于是整串保留为条件，
	//                   约束名自动派生为 chk_chat_messages_role。建表后必须在真库用
	//                   \d chat_messages 亲眼确认这条约束在——CHECK 是"翻译 Struct"时最容易整行
	//                   丢掉的东西，丢了不影响编译、不影响跑通，只在数据脏了以后才暴露。
	Role MessageRole `gorm:"column:role;type:varchar(10);not null;check:role IN ('user','assistant')" json:"role"`

	// 对应 SQL：content TEXT NOT NULL
	//   原文存取：本表不解析、不裁剪、不转义。主动消息注入的 [nudge] 标记也原样存在这里，
	//   格式由写入方（主动消息模块 / SSE 链路）约定，本字段不参与。长度也不在这里限制
	//   （DDL 是 TEXT），上限由写入方的请求体校验决定。
	Content string `gorm:"column:content;type:text;not null" json:"content"`

	// 对应 SQL：emotion_label VARCHAR(20)（可空）
	//   必须是**指针**：契约要求该字段可以为 null（词典档分析不出情绪时就是空）。值类型会把 null
	//   序列化成 ""，前端拿到 "" 会以为"有情绪但标签是空串"，与"没分析出"是两回事。
	//   ⚠️ 内部信号：随历史消息返回（供画像聚合与记忆打分），但**界面一律不得渲染**（AGENTS §4.4）。
	//      不要给它加展示标签，也不要为它加 SSE 事件——SSE 只有 delta / done / error 三种。
	EmotionLabel *string `gorm:"column:emotion_label;type:varchar(20)" json:"emotionLabel"`

	// 对应 SQL：emotion_score NUMERIC(4,3)（可空，0.000 ~ 1.000）
	//   同上是**指针**，同样只回读不展示。用 *float64 而不是引入 decimal 依赖：本项目只有这一列
	//   需要小数，为它加一个第三方包不划算。写入方（SSE 落库）若实测 pgx 往 numeric 编码 float64
	//   报类型不匹配，退到自写 driver.Valuer 返回字符串即可，仍然零新增依赖。
	EmotionScore *float64 `gorm:"column:emotion_score;type:numeric(4,3)" json:"emotionScore"`

	// 对应 SQL：is_nudge BOOLEAN NOT NULL DEFAULT FALSE
	//   标记"这条 user 消息是系统注入的 [nudge]，不是用户本人打的字"（技术文档 §5.4）。
	//   它**不改变 Role**：主动消息的 role 仍然是 user。不要把它理解成"这条是不是 AI 发的"。
	IsNudge bool `gorm:"column:is_nudge;type:boolean;not null;default:false" json:"isNudge"`

	// 对应 SQL：created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	//   autoCreateTime 让 GORM 插入时填 Go 侧的 time.Now()（不依赖 DB 默认值）。
	//   ⚠️ 排序必须补 id 兜底：Postgres 的 NOW() 是**事务开始时间**，同一事务内插入的多行会拿到
	//      完全相同的时间戳，只按 created_at 排序时同值行顺序不确定，翻页会重复或漏项。
	//      查询写法固定为 ORDER BY created_at DESC, id DESC（技术文档 §10.3：历史消息倒序分页）。
	//   index 不带 sort:desc：DDL 的索引就是普通 btree，PG 可以反向扫描，DESC 查询照样用得上。
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime;index:idx_messages_persona_time,priority:2" json:"createdAt"`

	// 以下两个关联字段仅供 GORM 生成外键约束用，不参与序列化。
	// 只写标量 UserID / PersonaID 时 GORM **不会创建任何外键**——"删人设级联删消息"与
	// "删账号清空其数据"两条验收项都不成立。必须声明它们，AutoMigrate 才会建出 ON DELETE CASCADE。
	// Create 时不要给它们赋值（保持零值），否则 GORM 会尝试连带写入 users / personas 表。
	User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName 显式指定表名。
// GORM 默认把结构体名 ChatMessage 复数化为 chat_messages，恰好一致；显式实现可避免将来改名或配置变化导致映射错误。
func (ChatMessage) TableName() string {
	return "chat_messages"
}

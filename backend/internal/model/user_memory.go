package model

import "time"

// MemoryType 记忆类型。
//
// 用自定义类型 + 常量而不是裸 string：写入方只能从这三个常量里取值，拼错单词在编译期就暴露。
// 与 MessageRole 同一手法。但**本类型与 MessageRole 有一处关键差别**：数据库侧没有 CHECK
// 约束（见 UserMemory.MemoryType 的说明）——所以这里的常量是本表**唯一**的编译期防线，
// 绕过 Go 的写入（手工 INSERT、脚本、将来的补偿任务）能塞进任意字符串。
type MemoryType string

const (
	// MemoryTypeFact 事实类记忆（"用户养了一只叫豆豆的猫"）。
	MemoryTypeFact MemoryType = "fact"

	// MemoryTypePreference 偏好类记忆（"用户喜欢跑步"）。
	MemoryTypePreference MemoryType = "preference"

	// MemoryTypeEvent 事件类记忆（"用户上周去了杭州"）。
	MemoryTypeEvent MemoryType = "event"

	// ⛔ 没有 MemoryTypeEmotion：情绪是内部信号（AGENTS §4.4 / 总纲 §0），
	//    随消息与画像参与计算，但不作为长期记忆存储、也不给展示标签。
	//    阶段二的 LLM 若返回 "emotion"，落库前必须归一或丢弃（spec §3.3）。
)

// UserMemory 记忆表结构体，对应数据库表 user_memory。
// 一个人设一份记忆（与 chat_messages 同一粒度，没有独立的会话表），删账号 / 删人设时由外键级联删除。
//
// 本表阶段一**只写不读**：写入方只有记忆提取链路（memory_repo.CreateBatch），读路径只有
// GET /memory 一个端点；没有更新与删除路径——记忆是长期事实，不做用户可见的编辑（总纲 §0），
// 删除只随账号 / 人设级联发生，或在删消息时断开溯源（见 SourceMessage）。
type UserMemory struct {
	// 对应 SQL：id BIGSERIAL PRIMARY KEY
	//   type:bigint（不是 bigserial）+ autoIncrement → 渲染出来仍是 bigserial，但 DataType 是 bigint。
	//   原因同 user.go / persona.go / chat_message.go：GORM 建关联时会把这里的 DataType 复制到
	//   外键列上（schema/relationship.go 只抄 DataType / GORMDataType / Size，不抄 autoIncrement）。
	//   写成 bigserial 会让下游那一列长出 DEFAULT nextval(...)，是**全项目已踩过三次**的坑
	//   （见 git log 的 fix(model): fix user_id auto-increment pollution for foreign keys）。
	//   本表虽然处在引用链末端，但阶段二 embedding_id = memory_id 会把主键语义抬到 ChromaDB，
	//   将来仍可能有人引用 user_memory.id——照抄正确写法，不要开倒车。
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 对应 SQL：user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE
	//   json:"-"        契约 §7 的 MemoryItem 里**没有** userId（6 个字段逐个数一遍）。这一列是
	//                   越权防线的载体，不是给前端看的字段；不要为了"方便调试"改成 json:"userId"——
	//                   那等于把归属信息暴露出去。
	//   type:bigint      不能省：被引用的 users.id 已写成 bigint，关联复制到此为止。
	//   ⛔ 绝不能带 autoIncrement：本列是外键、不是主键。带上自增会让每条记忆自动拿到一个与用户
	//      无关的 id，越权防线当场失效——与 chat_messages.user_id 同一处陷阱。
	//   index            idx_memory_user_id 单列索引（DDL）。越权防线的第二条查询条件靠它。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null;index:idx_memory_user_id" json:"-"`

	// 对应 SQL：persona_id BIGINT NOT NULL REFERENCES personas(id) ON DELETE CASCADE
	//   json:"personaId" 契约 §7 有这个字段，且它是"这份记忆属于哪个人设"的唯一关联。
	//   ⛔ 同样不带 autoIncrement：外键列。
	//   index            与 MemoryType 共用 idx_memory_persona_type（priority:1 = 组合索引首列）。
	//                    记忆列表查询只有 WHERE persona_id = ? 这一条读路径，走的就是这个前缀；
	//                    列顺序不能颠倒——memory_type 在前的话这条查询用不上索引。
	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_memory_persona_type,priority:1" json:"personaId"`

	// 对应 SQL：memory_type VARCHAR(20) NOT NULL   -- fact | preference | event
	//   ⛔ 不加 check: tag。权威 DDL 上**没有**这条约束（chat_messages.role 才有），
	//      加了就是偏离权威 DDL（AGENTS §1：契约 > 技术文档 > 代码现状）。AutoMigrate 会真的把
	//      这条约束建出来，于是"代码与文档不符"会变成"库里多了一条约束"。
	//      真要加属于改 schema，得先广播让队长拍板——不是本分支能顺手做的事。
	//   index            组合索引第二列，见 PersonaID。
	//   ⚠️ 代价：数据库这层没有兜底。拦截非法值只能在落库前做，而写入方只有一个
	//      （CreateBatch，由成员 1 的提取链路调用）——阶段二 LLM 返回的值必须先校验 / 归一。
	MemoryType MemoryType `gorm:"column:memory_type;type:varchar(20);not null;index:idx_memory_persona_type,priority:2" json:"memoryType"`

	// 对应 SQL：content TEXT NOT NULL
	//   记忆的原文，不解析、不裁剪、不转义。长度也不在这里限制（DDL 是 TEXT），
	//   上限由写入方的请求体校验决定。
	Content string `gorm:"column:content;type:text;not null" json:"content"`

	// 对应 SQL：embedding_id VARCHAR(64)（可空）—— 对应 ChromaDB 中的向量 id
	//   必须是**指针**：本列可空，阶段一恒为 NULL（技术文档 §6.3：保留此列是为了阶段二接入
	//   ChromaDB 时无需改表）。值类型会把 null 序列化成 ""，阶段二的补偿任务就分不清
	//   "还没索引"与"索引 id 是空串"。
	//   json:"-"         契约里没有这个字段。embedding 是内部状态，**前端物理上拿不到、
	//                    从结构上杜绝误渲染**（同 ai_moments.emotion_label 的处理）。
	EmbeddingID *string `gorm:"column:embedding_id;type:varchar(64)" json:"-"`

	// 对应 SQL：embedding_status VARCHAR(10) NOT NULL DEFAULT 'pending' -- pending | synced | failed
	//   单引号不能省：GORM 把 default: 后面的内容交给 dialector 拼进 DDL，不带引号会渲染成
	//   DEFAULT pending——未加引号的标识符，建表直接失败（persona.go 的 default:'{}' 同一手法）。
	//   ⚠️ 别误解这个默认值生效在哪一层：string 类型的 default: 会被 GORM 解析进
	//      DefaultValueInterface（非 nil），于是本字段**不属于** FieldsWithDefaultDBValue，
	//      Create 时这一列会被**显式写进 INSERT**（值是 GORM 从 tag 里取出的 'pending'），
	//      **不走数据库默认值**。也就是说同一个默认值被钉在了两处（本 tag + DDL），
	//      将来改默认值必须两处一起改——除非把 tag 去掉，但那样 DDL 里就没有 DEFAULT 了，
	//      会偏离权威 DDL，所以 tag 照留。
	//      写入方**依然不需要手写 EmbeddingStatus**：留零值即可，GORM 会补上。
	//   ⛔ 不要用 *string：DDL 是 NOT NULL，用指针反而允许写入 NULL。
	//   index            idx_memory_embedding_status 单列索引——阶段二补偿任务扫
	//                    WHERE embedding_status = 'pending' 用。
	EmbeddingStatus string `gorm:"column:embedding_status;type:varchar(10);not null;default:'pending';index:idx_memory_embedding_status" json:"-"`

	// 对应 SQL：importance_score NUMERIC(4,3) NOT NULL   -- 0.000 ~ 1.000
	//   用 float64 而不是引入 decimal 依赖：本项目只有两列需要小数（连同 emotion_score），
	//   为它加一个第三方包不划算。
	//   ⚠️ 本列比 chat_message.go 的 emotion_score 更需要注意：那一列可空，本列 **NOT NULL
	//      且是必写字段**——pgx 往 numeric 编码 float64 这条路径一定会被走到。若实测报类型
	//      不匹配，退到自写 driver.Valuer 返回字符串即可（仍零新增依赖），
	//      **不要为此改列类型、也不要引入 decimal 包**。
	//   ⛔ 用值类型 float64 而不是指针：DDL 是 NOT NULL。
	ImportanceScore float64 `gorm:"column:importance_score;type:numeric(4,3);not null" json:"importanceScore"`

	// 对应 SQL：source_message_id BIGINT REFERENCES chat_messages(id) ON DELETE SET NULL
	//   必须是**指针**：SET NULL 的前提是列可空。值类型会让 AutoMigrate 建出一列 NOT NULL，
	//   删消息时直接报错（不是静默失败）。
	//   为什么是 SET NULL 而不是 CASCADE（本表与 personas / chat_messages 最不一样的地方）：
	//   "这条记忆是从哪句话来的"只是**溯源**——句子没了，记忆本身还在。写成 CASCADE 会让
	//   "删一条消息顺手删掉一条长期记忆"，与"记忆是长期事实"的定位直接矛盾。
	//   json:"-"         契约里没有这个字段。
	SourceMessageID *uint64 `gorm:"column:source_message_id;type:bigint" json:"-"`

	// 对应 SQL：created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	//   autoCreateTime 让 GORM 插入时填 Go 侧的 time.Now()（不依赖 DB 默认值）。
	//   ⚠️ 排序必须补 id 兜底：Postgres 的 NOW() 是**事务开始时间**，而记忆提取是批量写入，
	//      同一事务内插入的多条会拿到完全相同的时间戳，只按 created_at 排序时同值行顺序不确定，
	//      翻页会重复或漏项。查询写法固定为 ORDER BY created_at DESC, id DESC（spec §4.4）。
	//   index 不带 sort:desc：DDL 的索引就是普通 btree，PG 可以反向扫描，DESC 查询照样用得上。
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime" json:"createdAt"`

	// 以下三个关联字段仅供 GORM 生成外键约束用，不参与序列化。
	// 只写标量 UserID / PersonaID / SourceMessageID 时 GORM **不会创建任何外键**——
	// "删账号清空其记忆""删人设级联清空记忆""删消息保留记忆"三条验收项都不成立。
	// Create 时不要给它们赋值（保持零值），否则 GORM 会尝试连带写入 users / personas / chat_messages。
	User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	// ⚠️ 唯一一个 SET NULL，也是唯一一个**指针关联**：值类型会让 GORM 把它当"必有行"，
	//    与 SET NULL 的语义冲突，且零值 ChatMessage 会被当成一个待保存的实体。
	//    tag 里 "SET NULL" 中间的空格是安全的：GORM 按 ; → , → : 三级切分后会把冒号后的
	// 注：值内部含空格是安全的，GORM 拼 DDL 时是 " ON DELETE " + OnDelete，
	// 值为 "SET NULL" 时最终渲染为 ON DELETE SET NULL。不要写成 SET_NULL。
	SourceMessage *ChatMessage `gorm:"...;constraint:OnDelete:SET NULL" json:"-"`

// TableName 显式指定表名。
//
// ⚠️ 这里不是"显式更清晰"而已，是**必需**的：GORM 默认把结构体名 UserMemory 复数化为
// **user_memories**，与权威 DDL 的表名 user_memory 不一致——不像 users / personas /
// chat_messages 那样恰好一致。少了这个方法，AutoMigrate 会默默建出另一张表 user_memories，
// 接口层查 user_memory 时才报"表不存在"，而且从代码上完全看不出哪里错了。
func (UserMemory) TableName() string {
	return "user_memory"
}

# spec · AI 朋友圈动态（AI Moment）· 数据模型

> 本文件是**合同**：定义本分支做什么、做到什么算完成，以及下游分支必须遵守的预写设计。
> 合并后即冻结，任何字段或行为变更必须升版本并在群里广播。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-ai-moment-model` |
| 状态 | ✅ v1.1 已按 9 条决策收尾，待人工审查 |
| **本支范围** | **模型层：`AIMoment` struct + `migrate.go` 一行**（§0） |
| 负责人 | 成员 3（`JMX2033`） |
| 依赖分支 | `feature/persona-model`（已合并）、`feature/persona-crud`（已合并） |
| 关联契约 | `docs/API_CONTRACT.md` §8（AI 朋友圈）、§1（全局约定）、§2（错误码） |
| 关联设计 | `docs/TECH_DESIGN.md` §6.2（DDL）、§5.5（AI 朋友圈）、§4.8（定时任务） |
| 关联红线 | `AGENTS.md` §4.3（数据边界）、§4.4（情绪是内部信号）、§4.6（没有手动触发）、§5.1（注释纪律）、§7.5 |

---

## 0. 范围声明（**先读这一节**）

### 0.1 本支只做两件事

| # | 文件 | 动作 | 内容 |
|---|------|------|------|
| 1 | `backend/internal/model/ai_moment.go` | 新增 | `AIMoment` struct（§3）+ `TableName()` |
| 2 | `backend/internal/model/migrate.go` | 修改 | 追加一行 `&AIMoment{}` |
| 3 | `backend/internal/model/ai_moment_test.go` | 新增 | JSON 序列化的键集断言（§9 分组 C，已确认） |
| 4 | `.learn/moment-gorm-tags.md` | 新增（**不入库**） | 学习笔记：DDL 对照、坑、反例 |

> **文件命名**：`ai_moment.go`（单数）。对齐既有的 `chat_message.go` ↔ `chat_messages` 惯例：struct 单数 PascalCase、文件名单数小写下划线、表名复数。

### 0.2 本支不做什么（**其余全部交接，去向逐个标明**）

| 不做的事 | 去向 |
|---------|------|
| `moment_comments` / `moment_likes` 两个 model | **各开独立分支**（PR 单一职责，避免单次改动过大）；**`\ds` 序列污染验收也在那时才能跑**（§3.6） |
| `dto/moment_dto.go` | `feature/backend-moment-api` |
| `repository/moment_repo.go`（§4 全部 SQL） | `feature/backend-moment-api` |
| `service/moment_service.go` + `handler/moment_handler.go` + `RegisterMomentRoutes` | `feature/backend-moment-api` |
| `ContentGenerator` + `service/moment_job.go` | `feature/backend-moment-job`（§6.3 已给完整接口设计） |
| `config.JobConfig` + `.env.example` + `go.mod` 的 `robfig/cron/v3` | `feature/backend-moment-job`（本支不需要定时任务） |
| `router.go` 的那一行 | **成员 1**（我不碰这个文件） |
| 手动触发接口 `POST /moments/generate` | **永不做**（红线，AGENTS §7.5） |
| 前端 `views/moments/**` | 前端分支 |

**§4 / §5 / §6 照写不误**——它们是**预写设计**，供下游分支直接执行，也是本支审查时的判断依据。每节都标了「本支落地 / 预写」。

---

## 1. 五条硬性要求（违反任何一条都不要提交）

### 1.1 ID 必须 `type:bigint` + `autoIncrement`，严禁 `bigserial`

```go
// ✅ 唯一正确写法
ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

// ❌ 严禁
ID uint64 `gorm:"column:id;type:bigserial;primaryKey;autoIncrement" json:"id"`
```

**为什么**：GORM 建关联时会把**被引用主键的 `DataType` 覆盖到外键字段上**
（`gorm.io/gorm/schema/relationship.go:596-600`）：

```go
for idx, foreignField := range foreignFields {
	// use same data type for foreign keys
	if copyableDataType(primaryFields[idx].DataType) {
		foreignField.DataType = primaryFields[idx].DataType   // ← 覆盖
	}
```

而 `copyableDataType`（同文件 775 行）是个**布尔判定**：类型串里含 `auto_increment` 或 `primary key` 就返回 `false`（不复制），
其余一律 `true`。`"bigserial"` 两个词都不含 → **返回 true，被原样抄走**。

后果：`AIMoment.ID` 写成 `bigserial`，将来 `moment_comments.moment_id` 会变成
**`bigint NOT NULL DEFAULT nextval('moment_comments_moment_id_seq')`**。

> ⚠️ **注意序列名**：不是 `ai_moments_id_seq`，而是**下游表自己的** `<子表>_<外键列>_seq`
> （2026-09-17 实测，见下方证据）。验收时按 `ai_moments_id_seq` 去找会**漏判**。

**漏传 `moment_id` 的 INSERT 不会报错**，它会从这条多余的序列里拿到一个自增值——
那个值可能撞上一条真实动态（评论挂错地方），也可能指向不存在的 id。**静默**才是它危险的地方。

**证据（2026-09-17 最小复现）**：两个父表分别用 `bigserial` / `bigint` 声明 ID，各挂一个子表引用它，
跑 `AutoMigrate` 后 `\d`：

| 父表 ID 写法 | 子表外键列 | 结论 |
|---|---|---|
| `type:bigserial` | `parent_id bigint NOT NULL DEFAULT nextval('child_serial_parent_id_seq')` | ❌ **被污染** |
| `type:bigint` | `parent_id bigint NOT NULL` | ✅ 干净 |

`\ds` 侧：干净的父/子对产出 2 个序列，被污染的父/子对产出 **3 个**（多出 `child_serial_parent_id_seq`）。

**这个坑全项目已经踩过三次**（`user.go` / `persona.go` / `chat_message.go`），本表是第四次机会，不许再踩。
渲染出来的 SQL 仍是 `BIGSERIAL`，但 `DataType` 是 `bigint`——**要的是 DataType 对，不是 SQL 字面对**。

> **本支已核验的部分**：`ai_moments.persona_id` 实测 `bigint NOT NULL`（**无默认值**），
> 说明上游 `personas.ID` 的 `type:bigint` 正确、且复制机制确实以"被引用主键"为准。
> **仍未闭合的部分**：`AIMoment.ID` 若写错，污染的是**下游**两表——本支不建它们，观测不到。
> 完整的序列污染验收（`moment_comments_moment_id_seq` 等）**随下游分支补跑**，已登记在 §9 分组 E。

详见 `.learn/moment-gorm-tags.md`（学习笔记，不入库），本 spec 只留结论。

### 1.2 `emotion_label` 必须 `json:"-"`，绝不进响应体

- GORM tag：`json:"-"`，**不是** `json:"emotionLabel"`。
- 数据库列**照建**（`type:varchar(20)`，可空），供内部使用。
- 契约 §8 的 `Moment` 实体共 8 个字段，**没有 `emotionLabel`**；契约变更记录里已有一条「`Moment` 移除 `emotionLabel`」。

**「回读字段」与「响应字段」的区别**（全项目最容易混淆的一点）：

| | `chat_messages.emotion_label` | `ai_moments.emotion_label` |
|---|---|---|
| 定位 | **回读字段** | **响应字段被排除** |
| json tag | `json:"emotionLabel"` | `json:"-"` |
| 前端能否拿到 | **能**拿到（供画像聚合与记忆打分） | **物理拿不到**，不在 JSON 里 |
| 防线性质 | 靠**纪律**：评审把关 + 前端自觉不渲染 | 靠**结构**：字段根本不存在，想渲染也没有 |
| 依据 | AGENTS §4.4「历史消息里的 `emotionLabel` 只是回读字段」 | 契约 §8「前端拿不到，也就无从渲染成情绪角标」 |

两者都受 §4.4 红线约束（**界面一律不得渲染**），区别只在防线强度：一个靠人，一个靠编译器。

> 本支的验证方式：**序列化测试**（§9 分组 C）——把 `AIMoment` marshal 成 JSON，断言键集**恰好**是 5 个且不含任何 `emotion` 字样。这比 grep tag 强，因为它验的是**行为**而不是**写法**。
> 下游的 `moment_dto.go` 里也不许出现任何 `emotion` 字样。

### 1.3 外键一律指向 `personas(id)`，带 `constraint:OnDelete:CASCADE`

朋友圈动态跟随人设的删除而级联清理：`personas` 没了，它的动态、评论、点赞都该没。

**两个动作缺一不可**：
1. 标量字段写 `PersonaID uint64`；
2. **另起一个关联字段** `Persona Persona \`gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"\``。

**只写标量字段 GORM 不会创建任何外键**——「删人设级联清理」这条验收直接不成立，且编译期毫无提示。本支至少要让 `ai_moments → personas` 这一条建出来（下游两个 model 落地时再补另外 5 条）。

### 1.4 所有查询必须 JOIN `personas` 并带 `p.user_id = ?`

`persona_id` 是**全局自增**的，只带它必然跨用户串号；只带 `user_id` 会跨人设。

**本表的特殊之处**：`ai_moments` **没有 `user_id` 列**（技术文档 §6.2 的 DDL 里就没有），因此 `user_id` 条件只能**通过 JOIN `personas`** 落到 SQL 里：

```sql
-- 所有涉及 ai_moments 的查询，只有这一种正确形态
JOIN personas p ON p.id = m.persona_id
WHERE p.user_id = ?
```

完整展开见 **§4**（含逐端点必带条件、四条 SQL 全文、仓储方法形态、反例清单）。

### 1.5 注释只写必要的（`AGENTS.md` §5.1）

`.go` 业务代码里**只留三类注释**：① 安全/越权红线；② 非显然的 GORM tag 语义；③ 函数职责一句话。

**禁止**在 `.go` 里保留：DDL 原文逐字对照、长篇学习性解释、反例分析、历史变更故事。这些统一放 `.learn/`（已在 `.gitignore`，不入库）。

**目标密度**：`ai_moment.go` 全文件**不超过 15 行注释**，三类各覆盖一处（§3.4 的定稿实测 **7 行**）。
上限才是要防的东西——为凑行数而加注释，本身就违反 §5.1。

> **风格冲突（已决策）**：既有 `user.go` / `persona.go` / `chat_message.go` 的注释远超 §5.1（单字段 20-40 行，含 DDL 逐字对照与反例分析）。**本支不清理它们**——等 10 张表全部落地后统一 refactor。新文件按 §5.1 从简。
> 因此审查时**不以"与既有文件风格不一致"为退回理由**。

---

## 2. 背景与目标

人设 CRUD 与聊天记录已完成，库里已有 `users` / `personas` / `chat_messages` / `user_memory` / `user_profile` / `proactive_settings` 六张表。**AI 朋友圈一个字符都还没写**：

- `internal/model/` 下没有 moment 相关文件——10 张表的清单里还差 4 张（`ai_moments` / `moment_comments` / `moment_likes` / `schedules`）
- 契约 §8 的 4 个端点全部是 `⬜`
- 没有 `moment_repo.go` / `moment_service.go` / `moment_handler.go`
- `ai-service/` **目录不存在**；`robfig/cron` 也还没进 `go.mod`

本支先把**第一块也是最不可逆的一块**落地：`ai_moments` 的表结构。表结构一旦建错，下游所有代码都要返工；而它又是唯一被下游三处引用的表（`moment_comments` / `moment_likes` 的外键，以及阶段二的生成链路），所以**单独一支、单独审**。

**目标（可验证）**：`\d ai_moments` 与 §3.2 对照表逐列一致，且序列、外键、索引全部符合 §9 分组 A。

---

## 3. 数据模型：`ai_moments`（**本支落地**）

### 3.1 权威 DDL（技术文档 §6.2 照抄，供对照）

```sql
-- AI 朋友圈动态表
CREATE TABLE ai_moments (
    id         BIGSERIAL PRIMARY KEY,
    persona_id BIGINT      NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    content    TEXT        NOT NULL,
    emotion_label VARCHAR(20),                 -- 内部信号：只用于决定生成语气，不返回给前端
    like_count INT         NOT NULL DEFAULT 0, -- 冗余计数，真值以 moment_likes 为准
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_moments_persona_time ON ai_moments(persona_id, created_at DESC);
```

> 下游两张表的 DDL 见 §7（预写）。

### 3.2 字段对照表（DDL ↔ GORM ↔ JSON）

| DDL 列 | Go 字段 | GORM tag 关键部分 | JSON | 说明 |
|--------|---------|-------------------|------|------|
| `id BIGSERIAL` | `ID uint64` | `type:bigint;primaryKey;autoIncrement` | `id` | §1.1 |
| `persona_id BIGINT NOT NULL` | `PersonaID uint64` | `type:bigint;not null;index:idx_moments_persona_time,priority:1` | `personaId` | 索引首列；**不带 autoIncrement** |
| `content TEXT NOT NULL` | `Content string` | `type:text;not null` | `content` | |
| `emotion_label VARCHAR(20)` | `EmotionLabel *string` | `type:varchar(20)` | **`-`** | §1.2；指针 = 可空 |
| `like_count INT NOT NULL DEFAULT 0` | `LikeCount int` | `type:integer;not null;default:0` | `likeCount` | 冗余计数 |
| `created_at TIMESTAMPTZ NOT NULL` | `CreatedAt time.Time` | `type:timestamptz;not null;default:now();autoCreateTime;index:...,priority:2,sort:desc` | `createdAt` | |
| （无此列） | `Persona Persona` | `foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE` | `-` | §1.3，**只为建外键** |

**完整 struct（供审查逐字对照）**：

```go
package model

import "time"

// AIMoment AI 朋友圈动态，对应数据库表 ai_moments。
// 动态只由定时任务产生，没有手动触发接口（AGENTS §4.6）。
type AIMoment struct {
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 外键列：不带 autoIncrement，否则动态会挂到不存在的人设上（spec §1.1）
	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_moments_persona_time,priority:1" json:"personaId"`

	Content string `gorm:"column:content;type:text;not null" json:"content"`

	// 内部信号：只决定生成语气，不进任何响应体（AGENTS §4.4）。前端物理拿不到，从结构上杜绝误渲染。
	// 指针 = 可空：分析不出情绪时是 NULL，不是空串。
	EmotionLabel *string `gorm:"column:emotion_label;type:varchar(20)" json:"-"`

	LikeCount int `gorm:"column:like_count;type:integer;not null;default:0" json:"likeCount"`

	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime;index:idx_moments_persona_time,priority:2,sort:desc" json:"createdAt"`

	// 仅供 GORM 生成外键约束用，不参与序列化。只写标量 PersonaID 时不会建出 ON DELETE CASCADE。
	Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName 显式指定表名。
func (AIMoment) TableName() string { return "ai_moments" }
```

> 这就是**全文件**——注释共 **7 行**，在 §1.5 的 15 行上限内。DDL 逐列对照、"为什么 bigserial 会污染下游"的原理、反例展开，**一行都不在这里**，全部在 `.learn/moment-gorm-tags.md`。
>
> 注意这段注释里**刻意不出现** `bigserial` / `json:"emotionLabel"` 这两个被禁值：注释里写了，§9 分组 A 的字面 grep 就会命中。注释描述"该怎么做"，被禁值只在 §1.1 / §1.2 与 `.learn/` 里出现。

### 3.3 三条必须记住的结论

1. **`EmotionLabel *string` 是指针**。词典档分析不出情绪时为空，值类型会把 NULL 变成 `""`。
2. **`like_count` 是冗余计数，真值在 `moment_likes`**。任何时候二者不一致都算 bug；写入必须走 §4.4 的幂等路径。
3. **`Content` 原样存取**。本表不解析、不裁剪、不转义；长度也不在这里限制（DDL 是 TEXT）。

### 3.4 索引

| 表 | 索引 | GORM 表达 |
|----|------|-----------|
| `ai_moments` | `idx_moments_persona_time(persona_id, created_at DESC)` | `index:idx_moments_persona_time,priority:1` / `priority:2,sort:desc` |

### 3.5 「表里没有的东西」也是硬约束

`Moment` 响应体里的 `personaName` / `liked` / `commentCount` **都不是列**：

| 字段 | 来源 |
|------|------|
| `personaName` | JOIN `personas.name` |
| `liked` | LEFT JOIN `moment_likes` 判当前用户（`l.id IS NOT NULL`） |
| `commentCount` | 聚合 `moment_comments`（契约标注为"派生字段"） |

> 🚨 **AutoMigrate 陷阱（本支最容易犯、且完全没有报错的一个）**：这三个字段**绝不能**加进 `model.AIMoment`。只要在 model 里写一个没有 `gorm:"-"` 的字段，AutoMigrate 就会**真的建出 `persona_name` / `liked` / `comment_count` 三列**——表结构与 DDL 静默偏离，`\d ai_moments` 多出三列，**不报错、不影响编译**。
>
> 它们属于 `dto.MomentItem`（§5.6），不属于 model。本支用**序列化测试**（§9 分组 C）把这条钉死：多一个字段，键集断言立刻报红。

### 3.6 级联与序列污染（**本支只能部分验收**）

| 检查 | 本支能否跑 | 说明 |
|------|:---:|------|
| `ai_moments → personas` 外键是 `ON DELETE CASCADE` | ✅ | `\d ai_moments` |
| 删人设后 `ai_moments` 清零 | ✅ | 手工删一个人设，`SELECT count(*)` |
| `\ds` 只有 `ai_moments_id_seq`（无 `ai_moments_persona_id_seq`） | ✅ | 抓"外键列误加 autoIncrement" |
| `moment_comments_moment_id_seq` 之类**不存在** | ❌ **随下游分支** | 下游表还没落地，观测不到 |
| 完整的三表级联链（人设 → 动态 → 评论/点赞） | ❌ **随下游分支** | 同上 |

> **不要因为跑不了就跳过前三条**——它们是本支唯一能拿到的运行时证据。

---

## 4. 越权防线（**预写；本支定死设计，下游分支执行**）

> 对应红线 3：**❌ 跨用户 / 跨人设数据泄漏**。
> 本节的 SQL 由 `feature/backend-moment-api` 落地；本支负责把形态定死，避免下游自由发挥。

### 4.1 本功能与前面几张表最大的不同

| | `chat_messages` / `user_memory` / `user_profile` | `ai_moments` |
|---|---|---|
| `user_id` 列 | **有**，越权防线冗余在表上 | **没有**（§6.2 DDL 里就没有） |
| 防线落点 | `WHERE persona_id = ? AND user_id = ?` | `JOIN personas p ON ... WHERE p.user_id = ?` |
| 写错时的表现 | 少写一个条件，静默串号 | **连错法都写不出来**（见 §4.2） |

**这是全功能唯一一道防线**：忘了 JOIN 就是全表泄漏；写对了就是账号隔离。因此下游的仓储方法命名必须让"带 userID"**显式可见**（§4.5）：`ListByUser` 而不是 `List`，`FindOwnedMoment` 而不是 `FindMoment`。**名字里没有 `User` / `Owned` 的方法，评审时一律当可疑对象。**

### 4.2 ✅ 正面证据：本表的经典错法**编译不过**

「先查出来、再在 Go 里比」是越权 bug 最经典的形态——它骗得过 review（代码"看起来逻辑完整"），因为**漏掉的那个 `if` 在代码里是个空洞**，读代码时不容易注意。

**这个错法在 `ai_moments` 上根本写不出来**：

```go
// ❌ 这个"错法"编译不过 —— model.AIMoment 没有 UserID 字段
m, err := repo.FindByID(ctx, momentID)      // 假设存在这么一个方法
if m.UserID != userID { return 4040 }       // ← 编译错误：m.UserID undefined
```

原因：`ai_moments` 表（以及本支落地的 `AIMoment` struct）**没有 `user_id` 列**。归属信息只存在于 `personas.user_id`，所以：

- 想判断归属，**只能**去 JOIN `personas` → 归属条件**必然出现在 SQL 里**；
- SQL 里出现 `p.user_id = ?` 之后，"先查出来再比"就失去了存在意义。

**这条要守住**：将来有人为了"方便比较"给 `AIMoment` 加一个 `UserID` 字段（哪怕 `json:"-"`），这条结构性保护就没了，越权防线会退回"靠纪律"。**加这个字段等同于删掉一道防线，评审时一律拒绝。**

> 与 §4.1 合起来看：**风险是"忘了 JOIN"（唯一一道防线），保护是"经典错法写不出来"。**

### 4.3 逐端点的必带条件（**下游背诵这四行**）

| 端点 | SQL 里必须出现的条件 | 漏了会怎样 |
|------|---------------------|-----------|
| `GET /moments` | `WHERE p.user_id = ?` | 返回**全库所有人的动态** |
| `POST /moments/:id/like` | `WHERE m.id = ? AND p.user_id = ?` | 给别人的动态点赞、改别人的计数 |
| `GET /moments/:id/comments` | `WHERE m.id = ? AND p.user_id = ?` | 读别人动态下的评论 |
| `POST /moments/:id/comments` | `WHERE m.id = ? AND p.user_id = ?` | 往别人的动态里写评论 |

> **`GET /moments` 为什么没有 `persona_id` 条件**：它返回该用户**全部**人设的动态，请求里没有 `personaId` 入参（契约只给 `page` `pageSize`），SQL 里自然没有可绑定的值。这里要守住的是 §1.4 的**意图**——"任何查询都只作用在当前用户自己的人设上"，`p.user_id = ?` 是这条意图的唯一实现。
>
> 这是一处**刻意的例外**，评审时不要因为"没看到 persona_id 条件"就判不通过——但**必须**看到 `p.user_id = ?`。

### 4.4 四条 SQL 的完整形态（下游照抄）

**① 列表（`ListByUser` + `CountByUser`）**

```sql
SELECT m.id AS id,
       m.persona_id AS persona_id,
       p.name AS persona_name,
       m.content AS content,
       m.like_count AS like_count,
       (l.id IS NOT NULL) AS liked,
       (SELECT count(*) FROM moment_comments c WHERE c.moment_id = m.id) AS comment_count,
       m.created_at AS created_at
FROM ai_moments m
JOIN personas p ON p.id = m.persona_id
LEFT JOIN moment_likes l ON l.moment_id = m.id AND l.user_id = ?
WHERE p.user_id = ?
ORDER BY m.created_at DESC, m.id DESC
LIMIT ? OFFSET ?
```

- **`SELECT` 必须显式写 `AS` 别名**（本条关键）：`p.name` 不带别名时，GORM 会把它扫进 `Name` 字段——而投影结构里没有 `Name`，于是**静默留空**，`personaName` 变成 `""`，**不报错**。别名写成 `p.name AS persona_name`（GORM 按 snake_case 映射到 `PersonaName`）。
- `l.user_id = ?` 写在 **ON 里**，不是 WHERE——写进 WHERE 会把"没点赞"的行过滤掉，`LEFT JOIN` 退化成 `INNER JOIN`。
- `ORDER BY` 必须带 `m.id DESC` 兜底：Postgres 的 `NOW()` 是**事务开始时间**，同一事务插入的多行会拿到相同时间戳，只按 `created_at` 排序时同值行顺序不确定，翻页会重复或漏项。定时任务批量生成正好命中。
- **`CountByUser` 的 `JOIN` 与 `WHERE` 必须与上面逐字相同**，只换 `SELECT` 部分；漏 JOIN 会让 `total` 变成全表行数，泄漏"系统里共有多少条动态"。建议抽一个私有函数返回组装好的 `*gorm.DB`，杜绝两处不一致。

**② 归属闸门（`FindOwnedMoment`，三个写端点共用）**

```sql
SELECT m.id FROM ai_moments m
JOIN personas p ON p.id = m.persona_id
WHERE m.id = ? AND p.user_id = ?
```

未命中返回 `gorm.ErrRecordNotFound`，service 转 `4040`。**它同时承担"不存在"与"别人的"两种情形**，像 `persona_repo.FindOwned` 一样不可区分。

**③ 点赞（`InsertLikeIfAbsent`）——带守卫的 INSERT...SELECT**

```sql
INSERT INTO moment_likes (user_id, moment_id, created_at)
SELECT ?, m.id, ?
FROM ai_moments m
JOIN personas p ON p.id = m.persona_id
WHERE m.id = ? AND p.user_id = ?
ON CONFLICT (user_id, moment_id) DO NOTHING
```

**为什么是这个形态**：归属条件**物理上写进了写入语句**——忘了它，这条 SQL 就写不出来；而"先查后插"漏掉那个 `if` 会**静默越权**。同时没有 TOCTOU 窗口：查归属与写入是同一条语句。

若 GORM 拼不出 `INSERT ... SELECT`，**退到 `tx.Exec` 写原生 SQL 完全可以**——可读性与安全性都更强。

返回值语义（service 据此分流）：

| `RowsAffected` | 含义 | service 动作 |
|:---:|---|---|
| `1` | 归属通过 **且** 首次点赞 | `IncrementLikeCount`，返回 `liked: true` |
| `0` | 二义：**要么**已点过，**要么**不是自己的动态 | 再跑一次归属查询消歧：命中 → 幂等成功（**不**自增）；未命中 → `4040` |

**④ 自增与读回**

```sql
UPDATE ai_moments SET like_count = like_count + 1 WHERE id = ?   -- 只在 RowsAffected=1 时执行
SELECT like_count FROM ai_moments WHERE id = ?                    -- 同事务内读回
```

**⑤ 评论列表**

```sql
SELECT c.id AS id, c.moment_id AS moment_id, c.persona_id AS persona_id, c.user_id AS user_id,
       COALESCE(p.name, u.username) AS author_name,
       c.content AS content, c.created_at AS created_at
FROM moment_comments c
LEFT JOIN personas p ON p.id = c.persona_id
LEFT JOIN users u ON u.id = c.user_id
WHERE c.moment_id = ? AND c.moment_id IN (
        SELECT m.id FROM ai_moments m
        JOIN personas p2 ON p2.id = m.persona_id
        WHERE m.id = ? AND p2.user_id = ?
      )
ORDER BY c.created_at ASC, c.id ASC
LIMIT ? OFFSET ?
```

`COALESCE(p.name, u.username)` 正好吃住"二选一"：AI 评论取人设名，用户评论取用户名。这里**再带一次归属条件**是刻意的第二道闸门（§4.5）。

### 4.5 仓储层允许与禁止的方法形态（下游执行）

**允许**（名字里必须能看出作用域）：

```go
ListByUser(ctx, userID, offset, limit) ([]MomentRow, error)
CountByUser(ctx, userID) (int64, error)
FindOwnedMoment(ctx, userID, momentID) (*model.AIMoment, error)   // JOIN personas
ListCommentsByUser(ctx, userID, momentID, offset, limit) ([]CommentRow, error)
CountCommentsByUser(ctx, userID, momentID) (int64, error)
CreateComment(ctx, tx, c *model.MomentComment) error
InsertLikeIfAbsent(ctx, tx, userID, momentID) (int64, error)      // 返回 RowsAffected
IncrementLikeCount(ctx, tx, momentID) error
GetLikeCount(ctx, momentID) (int, error)
```

**明确禁止**：

| 禁法 | 后果 |
|------|------|
| `FindByID(ctx, momentID)` | 任意 `moment_id` 都能读出，跨用户 |
| `First(&m, id)` / `Model(&AIMoment{}).Where("id = ?", id)` | GORM 主键查询**不带任何归属条件** |
| `Where("persona_id = ?", personaID)` | `persona_id` 全局自增，跨用户串号 |
| `Where("moment_id = ?")` 单独出现 | 同上 |
| 任何 `List()` / `FindAll()` 无参形态 | 全表扫描 |

**唯一合法的"跨用户"方法**（给定时任务用）：定时任务要遍历所有人设，它是系统级任务、不属于任何用户。这类方法必须：

1. 命名醒目（如 `ListGenerationCandidates`），注释标明用途；
2. **只允许被 `moment_job.go` 调用**，handler / service 的请求路径一律不得引用；
3. 返回行必须携带 `persona_id` 与 `user_id`（供 job 判断"同账号的其他 AI"），但**不得**出现在任何响应体里。

> 评审态度：**看到全表查询，先问它是不是只在 job 里被调用。**

### 4.6 反例清单（下游 AI 最容易写出来的错法）

1. **忘记 JOIN** —— `db.Model(&model.AIMoment{}).Order("created_at DESC").Find(&list)`。最危险的一条：代码"看起来完整"，返回的却是全库动态。
2. **JOIN 了但 `WHERE` 只带 `persona_id`** —— `persona_id` 全局自增，别的用户同号人设的动态照串。
3. **count 漏 JOIN** —— 列表加了防线，`total` 忘了加，泄漏全表行数。
4. **`liked` 用错人** —— `LEFT JOIN moment_likes l ON l.moment_id = m.id` 漏掉 `AND l.user_id = ?`，A 会看到 B 点过的痕迹。
5. **无条件自增 `like_count`** —— 重复点击累加，与 `moment_likes` 的真值漂移。
6. **`Save()` 写回整行** —— 会把 `like_count` 覆盖成内存里的旧值（与 `persona_repo.go` 的"绝不能用 `Save`"同源）。
7. **`ON CONFLICT ON CONSTRAINT uq_moment_like`** —— GORM 的 `uniqueIndex` 建的是唯一索引不是具名约束，这句会直接报错。**用列清单形式** `ON CONFLICT (user_id, moment_id)`。
8. **把 `personaName` / `liked` / `commentCount` 加进 `model.AIMoment`** —— AutoMigrate 建出三列真列（§3.5）。**本支已用测试钉死。**
9. **给 `AIMoment` 加 `UserID` 字段"方便比较"** —— 等于删掉 §4.2 的结构性保护。
10. **评论作者不校验同账号** —— job 从**全库**挑评论者而不是从"该动态所属用户的其他人设"里挑，A 的动态下会出现 B 的人设名。
11. **`emotion_label` 给了 json tag** —— 哪怕写成 `json:"emotionLabel,omitempty"` 也是红线（§1.2）。

### 4.7 错误码：只有 `4040`，不产生 `4043` / `4030`

| 情形 | 返回 | 理由 |
|------|------|------|
| 动态属于自己的账号 | `200` | |
| 动态是**别人的** | `4040` | 与"不存在"**同码同文案**，外部无法区分——这正是隐藏存在性的效果 |
| 动态**不存在** | `4040` | 同上 |
| 不带 / 无效 Token | `4010` / `4011` | 由中间件产生 |

> **为什么不是 `4043`**：`4043`（`ErrPersonaNotFound`）专用于**人设**归属失败。动态的资源是 `moment`，落在 `ErrNotFound = 4040`——契约 §8 的错误码列写的就是 `4040`。
> **为什么不是 `4030`**：`4030` 只用于**功能越权**（封禁用户），资源越权一律按"不存在"返回，否则等于确认这个 `moment_id` 存在。

---

## 5. 接口定义（**预写，本支不实现**，由 `feature/backend-moment-api` 落地）

### 5.1 通用约定（契约 §1 / §2）

- **4 个端点全部需要 Token**，挂 `protected` 组（`middleware.JWTAuth`）。
- `user_id` **只能**从 JWT 声明取（复用 `persona_handler.go` 的 `currentUserID(c)`），永不从 body / query / path / header 读。取不到或为 `0` → `4010` 中断，**绝不降级成 `0` 继续查**（`WHERE user_id = 0` 返回空集，把鉴权失败伪装成"没有数据"）。
- 响应体统一 `{ code, message, data, timestamp }`，用 `response.Success(c, data)`；handler **不写** `response.Fail`，错误用 `_ = c.Error(err)` 上抛。
- 分页走 `dto.ClampPage` + `dto.PageResult[T]`（默认 20、上限 100，**只收敛不报错**）。
- 时间格式：`time.Time` 直接序列化（与 `persona_dto.go` 一致）。

### 5.2 `GET /moments` — 动态流

**请求**：`page` `pageSize`（**没有 `personaId` 过滤参数**，见 §5.7）
**响应 data**：`PageResult<MomentItem>`
**语义**：返回当前用户**全部人设**的动态，`created_at DESC, id DESC`；`liked` 是**当前登录用户**是否点过；无动态 → `200` + `list: []`（不是 `null`）；越界页 → `200` + `list: []` + 真实 `total`。

### 5.3 `POST /moments/:id/like` — 点赞（幂等）

**响应 data**：`{ "likeCount": 3, "liked": true }`　**错误码**：`4040`

实现见 §4.4 ③④。**第 4 步的条件自增是整个幂等的铰链**：无条件自增会让重复点击把 `like_count` 越点越高，且与 `moment_likes` 的 `COUNT(*)` 静默不一致。**本期 `liked` 恒为 `true`**（没有取消点赞），但仍要从插入结果推导，不要硬编码。

### 5.4 `GET /moments/:id/comments` — 评论列表

**响应 data**：`PageResult<CommentItem>`　**错误码**：`4040`
**语义**：**先判归属、再查评论**——动态不属于当前用户 → `4040`，**不返回空列表**（返回空列表会让攻击者把"别人的动态"与"没有评论的动态"混为一谈）。排序 `created_at ASC, id ASC`。`total` 必须是该动态的评论数。

### 5.5 `POST /moments/:id/comments` — 发表评论

**请求**：`{ "content": "..." }`　**响应 data**：`CommentItem`　**错误码**：`4001`（content 为空）、`4040`
**语义**：用户评论 → 写 `user_id`，`persona_id` 为 `NULL`（CHECK 约束保证二选一）；`authorName` = 当前用户的 `users.username`。**AI 评论不从这个端点进**，由定时任务直接写库（§6）。内容 trim 后为空 → `4001`；长度不设上限（DDL 是 TEXT）。

### 5.6 响应结构（`moment_dto.go`）

```go
// MomentItem 是契约 §8 的 Moment 实体，8 个字段，一个不多一个不少。
// personaName / liked / commentCount 都不是 ai_moments 的列（spec §3.5）。
type MomentItem struct {
	ID           uint64    `json:"id"`
	PersonaID    uint64    `json:"personaId"`
	PersonaName  string    `json:"personaName"`
	Content      string    `json:"content"`
	LikeCount    int       `json:"likeCount"`
	Liked        bool      `json:"liked"`
	CommentCount int64     `json:"commentCount"`
	CreatedAt    time.Time `json:"createdAt"`
}

// CommentItem 是契约 §8 的 Comment 实体。
// personaId 与 userId 恰好一个非空 —— 必须是指针，否则 null 会变成 0。
type CommentItem struct {
	ID         uint64    `json:"id"`
	MomentID   uint64    `json:"momentId"`
	PersonaID  *uint64   `json:"personaId"`
	UserID     *uint64   `json:"userId"`
	AuthorName string    `json:"authorName"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"createdAt"`
}

type CreateCommentRequest struct {
	Content string `json:"content" binding:"required"`
}
```

**`MomentItem` 里没有 `EmotionLabel` 字段——这是 §1.2 的落点，下游评审重点确认。**

### 5.7 契约空白处（**本文定，需广播**）

| # | 空白 | 本文的裁定 | 理由 |
|---|------|-----------|------|
| 1 | 评论列表排序方向 | `created_at ASC, id ASC` | 评论是对话式阅读，倒序读不通 |
| 2 | `content` 长度上限 | 不设上限（DDL 为 TEXT） | 与 `chat_messages` 一致；前端 maxlength 属页面分支 |
| 3 | `GET /moments` 是否支持按人设筛选 | **不支持** | 契约请求列只有 `page` `pageSize`，加参数即契约漂移 |
| 4 | `liked` 恒为 true 时的语义 | 仍返回真实推导值 | 为将来加取消点赞留出语义空间 |
| 5 | 下游两张表 `uq_moment_like` 在 `\d` 里出现在 Indexes 段 | 正常，不需对齐 | GORM 的 `uniqueIndex` 与 DDL 的 `CONSTRAINT ... UNIQUE` 在 Postgres 内部同为唯一索引，**功能等价**；不要为"对齐"手写 `ALTER TABLE`（红线 8） |

---

## 6. 定时任务与内容生成（**交接设计**，由 `feature/backend-moment-job` 落地）

### 6.1 没有手动触发接口（重申）

| | 主动消息 | 日程提醒 | **朋友圈** |
|---|---|---|---|
| 手动触发 | `POST /proactive/trigger` 🚨 | `POST /schedules/:id/trigger` 🚨 | **没有** |
| 依据 | AGENTS §4.6 演示硬性依赖 | 同上 | AGENTS §4.6 / §7.5、契约 §8 |

「给朋友圈加手动触发接口」是**红线**。演示兜底只有两件事：① `MOMENT_JOB_INTERVAL` 调到 `2m`；② **提前灌好一批历史动态**，页面点开就有内容。

### 6.2 任务形态

- **跑在 Go 侧**，用 `github.com/robfig/cron/v3`（TECH_DESIGN §4.8 钦定，**不用 Python APScheduler**）。
- 间隔从 `MOMENT_JOB_INTERVAL` 读，进 `config.JobConfig`（默认 `30m`）；全项目只有 `internal/config` 读环境变量。
- `go.mod` 的 cron 依赖**随该分支一起加**，本支不加。

### 6.3 `ContentGenerator` 接口定义（**核心交接物**）

TECH_DESIGN §5.5 写的是"定时任务触发 Agent 生成一条动态"，但 **`ai-service/` 目录当前不存在**。因此把"生成"抽象成一个接口，阶段一用模板实现，阶段二换 HTTP 调用——符合 AGENTS §4.7「阶段一/阶段二接口不变只换实现」：

```go
// ContentGenerator 产出动态正文。阶段一是模板，阶段二换成调 ai-service，调用方（moment_job）不改。
type ContentGenerator interface {
	Generate(ctx context.Context, in GenerateInput) (Generated, error)
}

type GenerateInput struct {
	Persona      model.Persona
	RecentEvents []string // 该人设近期 event 类记忆（user_memory.memory_type = 'event'）
}

type Generated struct {
	Content string
	// 内部信号：只决定生成语气，绝不进响应体（AGENTS §4.4）。分析不出时为 nil。
	EmotionLabel *string
}
```

**阶段一 `TemplateGenerator`（零依赖）**

| 项 | 做法 |
|----|------|
| 正文来源 | 模板池（按情绪分组）+ `persona.SpeakingStyle` / `PersonalityDesc` 拼句 |
| 生活事件 | 从 `RecentEvents` 随机挑一条填进槽位；为空时退到通用模板 |
| `EmotionLabel` | 由命中的模板槽位决定（如"雨天/热可可" → `neutral` / `joy`） |
| 依赖 | **零外部依赖**，可单测 |
| 为什么先做它 | 不依赖尚未存在的 `ai-service`，本功能可以独立跑通与演示 |

**阶段二 `AIGenerator`（HTTP 调用）**

| 项 | 做法 |
|----|------|
| 调用 | `POST {AI_SERVICE_URL}/internal/moment/generate`，带 `X-Internal-Token`（值 = `AI_SERVICE_TOKEN`，两边必须一致） |
| 请求体 | `{ personaId, name, personalityDesc, speakingStyle, recentEvents }` |
| 响应体 | `{ content, emotionLabel }` |
| prompt | 注入该 AI 的人设 + 最近对话提取的"生活事件"（TECH_DESIGN §5.5 的动态生成 Prompt 要点） |
| **前置条件** | 需要 `ai-service` 先提供该内部端点（属 ai-service 分支） |

**切换方式**：工厂 + 环境变量，沿用 `EMOTION_BACKEND` / `MEMORY_BACKEND` 的既有模式：

```
MOMENT_GENERATOR=template | ai     # 默认 template
```

**降级**：`ai` 档调用失败时回退模板实现并记日志（保证定时任务不会因 AI 抖动而空转）。

**交接清单**（给 `feature/backend-moment-job`）：

- [ ] `ai-service` 提供 `POST /internal/moment/generate`（否则只能停在 template 档）
- [ ] `MOMENT_GENERATOR` 开关加入 `backend/.env.example`（与 `ai-service/.env.example` 同步）
- [ ] 工厂函数与两个实现分文件，`moment_job.go` 只依赖接口
- [ ] 单测：模板档不依赖外部服务；`ai` 档用 httptest 打桩

### 6.4 节流（不加新列）

`ai_moments` 没有 `last_moment_at` 列，也不该为节流加。判断"这个人设最近刚发过"：

```sql
SELECT max(created_at) FROM ai_moments WHERE persona_id = ?
```

正好走 `idx_moments_persona_time(persona_id, created_at DESC)` 首列反向扫描，成本极低。阈值复用 `MOMENT_JOB_INTERVAL`。

### 6.5 一次 tick 的顺序

1. `ListGenerationCandidates` 取一批人设（§4.5 唯一合法的跨用户方法）；
2. 逐个用 §6.4 节流跳过"刚发过"的；
3. `generator.Generate(...)` 产出 `content`（+ 可空 `emotion_label`）；
4. 落 `ai_moments`；
5. 按概率从**同一 `user_id` 的其他人设**里挑评论者，写入 `moment_comments`（`persona_id` 非空、`user_id` 为 NULL）。

**要点**：第 5 步的"同账号"是硬性的（否则 A 的动态下会出现 B 的人设名）；每人设每 tick 最多 1 条；生成失败**只记日志不中断整轮**；不做单条重试（下一轮自然再来）。

---

## 7. 下游两张表的 DDL（**预写，已定稿待落地**）

> 本支不实现，但**结构已定死**，下游分支照此翻译即可。字段对照表随下游 spec 补齐。

```sql
-- 动态评论表（persona_id 与 user_id 二选一，表示是 AI 评论还是用户评论）
CREATE TABLE moment_comments (
    id         BIGSERIAL PRIMARY KEY,
    moment_id  BIGINT      NOT NULL REFERENCES ai_moments(id) ON DELETE CASCADE,
    persona_id BIGINT      REFERENCES personas(id) ON DELETE CASCADE,
    user_id    BIGINT      REFERENCES users(id) ON DELETE CASCADE,
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_comment_author CHECK (
        (persona_id IS NOT NULL AND user_id IS NULL) OR
        (persona_id IS NULL AND user_id IS NOT NULL)
    )
);
CREATE INDEX idx_comments_moment ON moment_comments(moment_id, created_at);

-- 点赞表：每个用户对每条动态只能点一次（UNIQUE 约束挡住重复点赞）
CREATE TABLE moment_likes (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    moment_id  BIGINT      NOT NULL REFERENCES ai_moments(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_moment_like UNIQUE (user_id, moment_id)
);
CREATE INDEX idx_likes_moment ON moment_likes(moment_id);
CREATE INDEX idx_likes_user ON moment_likes(user_id);
```

**下游落地时必须注意的四点**：

1. **`PersonaID *uint64` / `UserID *uint64` 必须是指针**——契约里 AI 评论是 `"personaId": 2, "userId": null`，值类型会把 `null` 序列化成 `0`。
2. **6 条外键全部 `ON DELETE CASCADE`**，且每个都要有对应的关联字段（§1.3）。
3. **`migrate.go` 的追加顺序**：`&AIMoment{}`（已在）→ `&MomentComment{}`（引用三张表）→ `&MomentLike{}`。
4. **`\ds` 序列污染验收在这一支补跑**（§3.6 表格最后两行）：新增序列恰好 3 个，没有 `moment_comments_moment_id_seq` / `moment_likes_moment_id_seq` / `moment_comments_persona_id_seq` / `moment_comments_user_id_seq`。

**`idx_likes_user` 是冗余的**（`uq_moment_like(user_id, moment_id)` 已覆盖 `WHERE user_id = ?`），但仍**照 DDL 建**——保持 `\d` 与 §6.2 逐行可对照，比为省几 MB 更有价值。

---

## 8. 已决策与剩余待确认

### 8.1 已决策（2026-09-17，负责人拍板）

| # | 事项 | 决策 |
|---|------|------|
| 1 | `ai_moments` 无 `user_id` 列 | **不改 DDL，用 JOIN `personas`**；「经典错法编译不过」作为正面证据写进 §4.2 |
| 2 | `ai-service/` 不存在 | **抽象 `ContentGenerator`，本支不落代码**（§6.3 作交接设计） |
| 3 | `robfig/cron/v3` | **本支不加**，随 `feature/backend-moment-job` |
| 4 | AutoMigrate 派生字段陷阱 | 三个派生字段**只能**出现在 `dto.MomentItem` |
| 5 | 点赞幂等 | 方案已定（§4.4 ③），本支不落实现 |
| 6 | `SELECT AS` 别名 | **本条关键**，写进 §4.4 ① |
| 7 | `router.go` | **本支不碰**，`RegisterPersonaRoutes` 挂载是成员 1 的交接项 |
| 8 | 注释风格冲突 | 新文件从简（10-15 行）；**本支不清理**既有文件，等 10 张表齐了统一 refactor |
| 9 | 验收正反两面 | 已按此重写 §9（每条"不许有什么"都配"必须有什么"） |
| 10 | 本支范围 | **只落 `AIMoment` 一个 struct + `migrate.go` 一行**；下游两表各开独立分支 |
| 11 | 序列污染观测不到的缺口 | **接受**。本支只验「没有 `ai_moments_persona_id_seq`」；`moment_comments` 落地后，那条完整的反向验收自动生效 |
| 12 | 序列化测试 | **加**（§9 分组 C / plan §3.3）。断言"键集**恰好**相等"，可同时抓情绪泄露与 AutoMigrate 多列；这是 `internal/model` 下的第一个测试文件，模式可复用到后续所有 model |
| 13 | 文件命名 | `ai_moment.go`（单数） |
| 14 | §7 预写 DDL 的 `BIGSERIAL` | **照抄不改**。`IDENTITY` 虽然是更新的推荐，但本项目冻结写法是 `BIGSERIAL`，擅自改反而破坏一致性 |

### 8.2 剩余待确认（不阻塞本支，落地下游分支前问清）

| # | 事项 | 本文的倾向 | 影响 |
|---|------|-----------|------|
| 1 | 阶段一的动态正文是否就用模板（`ContentGenerator` 的 template 档） | 是 | 不定则 `moment_job` 落不了地 |
| 2 | `ai-service` 何时提供 `POST /internal/moment/generate` | —— | 决定阶段二能否开工 |

---

## 9. 验收标准

> **正反两面都写**：每条"不许有什么"都配一条"必须有什么"，否则"写漏"类缺陷会全部静默通过。
> 分组 A/C/D 是**本支的验收范围**；分组 B/E 是下游分支的，列在此处是为了交接不断档。

### 分组 A · 建表与结构（**本支，必须全绿**）

- [ ] `AutoMigrate` 追加 `&AIMoment{}` 后，`\d ai_moments` 与 §3.2 对照表**逐列一致**
  - [ ] **必须恰好 6 列**：`id` `persona_id` `content` `emotion_label` `like_count` `created_at`
  - [ ] **多出的列必须是零**——特别是**没有** `persona_name` / `liked` / `comment_count`（§3.5）
  - [ ] **列宽/类型也要逐个看**，不能只看列名——`like_count` 必须是 `integer`，
        **不是 `bigint`**（写 `type:int` 会静默变成 bigint，2026-09-17 实测踩到，见 `.learn` §3）
  - [ ] `persona_id` / `id` 是 `bigint`、`content` 是 `text`、`emotion_label` 是 `character varying(20)`、
        `created_at` 是 `timestamp with time zone`
  - [ ] `persona_id` 的 **Default 必须为空**（§1.1 的复制污染的检查点）
- [ ] 序列：`\ds` 里新增的序列**恰好 1 个**（`ai_moments_id_seq`）
  - [ ] **没有** `ai_moments_persona_id_seq`（抓"外键列误加 autoIncrement"，§1.1）
- [ ] `ai_moments → personas` 外键存在且是 **ON DELETE CASCADE**
- [ ] `idx_moments_persona_time` 存在，且是 `(persona_id, created_at DESC)`
- [ ] **tag 静态检查**（本支唯一能验 §1.1 的手段，§3.6）：
  - [ ] `grep -n "bigserial" internal/model/ai_moment.go` → **零命中**
  - [ ] `grep -n 'gorm:"[^"]*autoIncrement' internal/model/ai_moment.go` → **恰好 1 行**（只在 ID 上）
        > 必须限定在 `gorm:"..."` 内。裸 grep `autoIncrement` 会命中 §3.4 定稿里 PersonaID 那行注释（"外键列：不带 autoIncrement"），
        > 那是**注释在描述规则**，不是 tag 违规。同理 `bigserial` 那条保持裸 grep——因为定稿里该词零出现，
        > 一旦有人把它写进注释，说明注释开始承担"讲原理"的职责，本身就该退回 `.learn/`。
  - [ ] `EmotionLabel` 行含 `json:"-"`；`grep -c "json:\"emotionLabel\""` → 0
- [ ] `go build ./... && go vet ./... && go test ./...` 全过
- [ ] 删一个人设 → `SELECT count(*) FROM ai_moments WHERE persona_id = <已删id>` = 0

### 分组 B · 越权防线（**下游 `feature/backend-moment-api`**，此处登记备查）

- [ ] **反例（必须返回空 / 4040）**：建 A、B 两账号各有人设与动态
  - [ ] A 的 Token 打 `GET /moments` → `list` 不含 B 的任何动态
  - [ ] A 的 Token 打 `GET /moments` 的 `total` → **等于 A 的动态数**，不是全表行数
  - [ ] A 打 `POST /moments/:id/like`，`:id` 用 **B 的动态** → `4040`，且 B 的 `like_count` **不变**
  - [ ] A 打 `GET /moments/:id/comments`，`:id` 用 **B 的动态** → `4040`（**不是空列表**）
  - [ ] A 打 `POST /moments/:id/comments`，`:id` 用 **B 的动态** → `4040`，库里**没有**新增行
  - [ ] 不存在的 id → `4040`，与"别人的动态"**同码同文案**
- [ ] **正面（必须正常返回）**：A 打自己的动态 → 四个端点全 `200`
- [ ] **不产生 `4030` / `4043`**：任意输入组合下只出现 `4040` / `4010` / `4001` / `200`
- [ ] **AI 评论不跨账号**：A 的动态下的 AI 评论，作者人设必然属于 A
- [ ] **代码级检查**：`moment_repo.go` 里每处查询都能指出它带 `p.user_id = ?`

### 分组 C · 序列化行为（**本支，已确认**）

> 本支没有 HTTP 层，`json:"-"` 与派生字段陷阱**无法用接口验证**。用一个纯 model 单测补上——**验的是行为，不是写法**，比 grep tag 强。
> 这是 `internal/model` 下的第一个测试文件；这套「键集恰好相等」的断言模式**可复用到后续所有 model**。

- [ ] `internal/model/ai_moment_test.go`：把 `AIMoment` marshal 成 JSON，断言**键集恰好**是
      `{id, personaId, content, likeCount, createdAt}`（**5 个**）
  - [ ] 键集里**没有** `emotionLabel` / `emotion_label`（§1.2）
  - [ ] 键集里**没有** `personaName` / `liked` / `commentCount`（§3.5 的 AutoMigrate 陷阱）
- [ ] **反向验证**：给 struct 临时加一个 `PersonaName string \`json:"personaName"\`` → 该测试**必须报红**；删掉后恢复。**跑不通这条，说明测试是假的。**
- [ ] **反向验证**：把 `EmotionLabel` 的 tag 改成 `json:"emotionLabel"` → 该测试**必须报红**

> 这张表就是 §1.5 之外的唯一创新点：把一个"模型层无法验收"的问题，用序列化断言变成可执行检查。

### 分组 D · 注释纪律与流程（**本支**）

- [ ] `internal/model/ai_moment.go` 注释 **≤ 15 行**（§1.5）——定稿实测 **7 行**
- [ ] **没有** DDL 逐字对照：`grep -cn "对应 SQL\|CREATE TABLE" internal/model/ai_moment.go` → **零命中**
- [ ] **没有**长篇学习性解释 / 反例分析 / 历史故事（内容已挪到 `.learn/moment-gorm-tags.md`）
- [ ] `.learn/moment-gorm-tags.md` 已写；`git status` 里**不出现**它（`.gitignore` 已覆盖）
- [ ] 未改 `router.go`，未改他人 model 文件（`git diff --name-only` 只含本支三个文件 + `migrate.go`）
- [ ] PR 已开、至少 1 人 Approve；commit 符合 `<type>(<scope>): <subject>`，scope 用 `moment`

### 分组 E · 交接给下游的验收（**不在本支**，登记以免断档）

- [ ] 序列污染完整验收：`\ds` 新增序列**恰好 3 个**，**没有** `moment_comments_moment_id_seq` 等（随 §7 两个 model 落地）
- [ ] `chk_comment_author` CHECK 约束在（`\d moment_comments`）——**CHECK 是最容易整行丢的东西**
- [ ] 完整级联链：删人设 → 动态、评论、点赞三表清零；删账号同理
- [ ] 评论 CHECK 生效：手工 INSERT 一行两个作者都非空 → 数据库**报错拒绝**
- [ ] 评论作者二选一：用户评论 `personaId: null`、AI 评论 `userId: null`（**不是 `0`**）
- [ ] `authorName` 正确：AI 评论 = `personas.name`，用户评论 = `users.username`
- [ ] 点赞幂等：同一用户连续 3 次 → `likeCount` 不变，`moment_likes` 只有 1 行
- [ ] `like_count` == `SELECT count(*) FROM moment_likes WHERE moment_id = ?`（任意操作序列后）
- [ ] 分页稳定：两条动态 `created_at` 相同时 `pageSize=1` 逐页拉**不重不漏**
- [ ] `RegisterMomentRoutes` 已由成员 1 在 `router.go` 挂到 `protected` 组
- [ ] **没有** `POST /moments/generate`（`grep` 零命中）
- [ ] 定时任务：间隔调 `1m` 后台有新增动态（正面），且**没有任何端点能手动触发**（反面）

---

## 10. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|------|------|------|------|
| 2026-09-17 | v1 | 创建 | AI 朋友圈（三表 + 四端点 + 定时任务）开工前的设计与验收基线 |
| 2026-09-17 | v1.1 | **范围收窄至模型层**（只落 `AIMoment` + `migrate.go`）；§4/§5/§6/§7 改为预写设计并标明去向；§4.2 补「经典错法编译不过」正面证据；§6.3 补 `ContentGenerator` 完整交接设计；§9 重构为「本支可验 / 下游交接」五组，新增序列化测试（分组 C）；§8 改为「已决策 9 条 + 剩余待确认 5 条」 | 按负责人 2026-09-17 的 9 条决策收尾 |
| 2026-09-17 | v1.2 | 五项确认落地：单表范围锁定、序列污染缺口接受、序列化测试转为正式验收项、文件名定 `ai_moment.go`、预写 DDL 保留 `BIGSERIAL`。§8.1 决策增至 14 条，§8.2 待确认收敛到 2 条 | 按负责人 2026-09-17 的五点确认收尾 |
| 2026-09-17 | v1.3 | **落码后回填**：§1.5 注释目标由"10-15 行"改为"≤ 15 行"（定稿实测 7 行，凑数会违反 §5.1）；§3.4 注释计数更正为 7，并写明"注释里刻意不出现被禁值"；§9 分组 A 的 `autoIncrement` 检查限定到 `gorm:"..."` 内（裸 grep 会命中注释） | 代码与文档对齐时发现三处前后不一致 |
| 2026-09-17 | v1.4 | **真跑 migrate 后的三处修正**：① `like_count` 的 tag `type:int` → `type:integer`（`type:int` 命中 GORM 内部常量 `schema.Int`，按 size 映射落到 `bigint`，实测偏差已修）；② §1.1 的污染后果更正序列名为 `<子表>_<外键列>_seq`（原文写 `ai_moments_id_seq`，按此搜索会**漏判**）；③ §1.1 补最小复现证据（父表 bigserial → 子表外键长出 nextval，并多出一个序列）。§9 分组 A 增加"列宽必须逐个看"与 `persona_id` Default 为空的检查点 | 落码后按验收清单实跑 `cmd/migrate` + `\d`/`\ds` 对照，发现代码 1 处 + 文档 2 处不一致 |

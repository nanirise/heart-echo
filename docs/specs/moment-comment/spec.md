# spec · 朋友圈评论（Moment Comment）· 数据模型

> 本文件是**合同**：定义本分支做什么、做到什么算完成，以及下游分支必须遵守的预写设计。
> 合并后即冻结，任何字段或行为变更必须升版本并在群里广播。
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：DDL 原文、契约字段、错误码表、通用约定一律**不复制**，只给链接；
> 本文件只写**本表独有**的内容——独有陷阱、独有决策、独有验收项。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-moment-comment-model` |
| 状态 | v1.3 · **代码已落，待人工审查**（spec 已过审，§5 分组 C 的 6 条注入由审查人跑） |
| **本支范围** | **模型层：`MomentComment` struct + `migrate.go` 一行**（§0.1） |
| 负责人 | 成员 3（`JMX2033`） |
| 依赖分支 | `feature/persona-model`、`feature/backend-ai-moment-model`（均已合并） |
| 关联契约 | [API_CONTRACT §8](../../API_CONTRACT.md#8-ai-朋友圈moments)（`Comment` 实体 + 评论两个端点） |
| 关联设计 | [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl)（`moment_comments` DDL）、[§5.5](../../TECH_DESIGN.md#55-ai-朋友圈p1) |
| 关联 spec | [ai-moment/spec.md](../ai-moment/spec.md)（§7 预写 DDL、§1.1 序列污染、§9 分组 E 遗留项、§4.4⑤ 评论 SQL） |
| 关联红线 | [AGENTS §4.3](../../../AGENTS.md)（数据边界）、§5.1（注释纪律）、§7 |

---

## 0. 范围声明（**先读这一节**）

### 0.1 本支只做四件事

| # | 文件 | 动作 | 内容 |
|---|------|------|------|
| 1 | `backend/internal/model/moment_comment.go` | 新增 | `MomentComment` struct（§3）+ `TableName()` |
| 2 | `backend/internal/model/migrate.go` | 修改 | 追加一行 `&MomentComment{}` |
| 3 | `backend/internal/model/moment_comment_test.go` | 新增 | JSON 键集 + `null`/`0` 断言（§5 分组 C） |
| 4 | `.learn/moment-comment-model.md` | 新增（**不入库**） | 学习笔记：CHECK tag 语义、二选一模型的取舍、反例展开 |

> **文件命名** `moment_comment.go`（单数），对齐既有 `ai_moment.go` / `chat_message.go` ↔ 表名复数的惯例。

### 0.2 本支不做什么（**其余全部交接，去向逐个标明**）

| 不做的事 | 去向 |
|---------|------|
| `moment_likes` model | **独立分支**（ai-moment §7 已定：两个下游表各开一支）。`\ds` 的**完整**序列验收（3 个序列、无 `moment_likes_moment_id_seq`）随那一支收口 |
| `dto/moment_dto.go`（`CommentItem`，含派生字段 `authorName`） | `feature/backend-moment-api` |
| `repository/moment_repo.go`（§4 的越权防线、评论列表/新增两条 SQL） | `feature/backend-moment-api` |
| `service` / `handler` / `RegisterMomentRoutes` | `feature/backend-moment-api` |
| 定时任务按概率写 AI 评论（评论作者必须**同账号**） | `feature/backend-moment-job`（ai-moment §6 第 5 步、§4.6 反例 10） |
| `router.go` 的那一行 | **成员 1**（我不碰这个文件） |
| 前端 `views/moments/**` | 前端分支 |

> ⚠️ **本支不含任何 SQL**。§1.3 与 §4 的越权防线是**预写设计**，供 `feature/backend-moment-api` 落地；
> 与之配套的**运行时**验收（跨账号返回 `4040`）登记在 §5 分组 E，不在本支跑。

---

## 1. 本表独有的四条硬性要求（违反任何一条都不要提交）

### 1.1 `moment_id` 的 Default 必须为空（**承接 ai-moment 的遗留未闭合项**）

> 来源：[ai-moment/spec.md §9 分组 E 第 1 条](../ai-moment/spec.md) —— "完整的序列污染验收随下游分支补跑"。
> **本支就是那一支**，这是本支最有分量的一条验收。

`moment_comments.moment_id` 一旦带上数据库默认值，漏传 `moment_id` 的 INSERT **不会报错**，
它会拿到一个自增值——可能撞上一条真实动态（评论挂到别人的动态下），也可能指向不存在的 id。**静默**才是危险的地方。

**两个产生机制**（都要堵）：

| # | 机制 | 状态 |
|---|------|------|
| ① | 上游 `ai_moments.id` 写成 `bigserial`，GORM 把该 `DataType` **复制**到本列（[ai-moment §1.1](../ai-moment/spec.md) 的原理与实测） | ai-moment 分支已用 `type:bigint` 切断，**本支负责确认它真的生效** |
| ② | 本字段自己误加 `autoIncrement` | 本支的 tag 纪律（§2 对照表） |

**验收（§5 分组 A）**：`\d moment_comments` 的 `moment_id` 行 Default 列为**空**，
且 `\ds` 里搜 `moment_comments_moment_id_seq` 得到 **0 结果**。

> **反面同样要验**：`\ds` 里**必须有** `moment_comments_id_seq`（恰好 1 个，来自 `id` 的 `autoIncrement`）。
> 只有"没有多余序列"是不够的——把 `ID` 的 `autoIncrement` 也删掉会同时满足那一条，却让本表再也插不进数据。

> **若验收失败怎么办**：说明上游 `ai_moment.go` 的 `ID` tag 被改坏了（不是本支的代码）。
> 修复要去改 `ai_moment.go` 并**另开 `fix/` 分支**，不要在本支顺手改别人已合并的 model 文件（§5 分组 D）。
> 注意 GORM `AutoMigrate` **不会**移除已存在列的 DEFAULT——改完 tag 后需要**重建该表**（仅限开发库，[AGENTS §7.8](../../../AGENTS.md) 禁止生产破坏性操作）。

### 1.2 作者二选一：两列都必须是**指针**，`chk_comment_author` 必须**照抄**

本表的定位是"AI 评论 / 用户评论共用一张表"，靠 `persona_id` 与 `user_id` **恰好一个非空**区分。
DDL 上**有** `CONSTRAINT chk_comment_author CHECK (...)`（[TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl)），因此：

1. **照抄，不加不减**——这条约束是权威 DDL 的一部分。
   > 与 [user_memory.go](../../../backend/internal/model/user_memory.go) 的处理**正好相反**：那里 DDL 上没有 CHECK，所以**明确不加**。
   > 判据永远是"DDL 上有没有"，不是"加了更安全"。加了会建出 DDL 上没有的约束，属于改 schema。
2. **`PersonaID` / `UserID` 必须是 `*uint64`**，两个理由：

   **理由一（契约）**：契约 §8 的 `Comment` 要求非作者的一侧是 **`null`**。值类型会序列化成 **`0`**——前端拿到 `"userId": 0` 会以为"用户 0 评论的"。

   **理由二（CHECK 会连带炸，但只炸一半）**：值类型字段即使留零值也会被写进 INSERT，
   于是那一列恒为 **`0`（非空）**。后果分两种：

   | 写错的范围 | 后果 |
   |-----------|------|
   | **两列都写成值类型** | 两侧恒非空 → **所有** INSERT 被 CHECK 拒绝（`23514`），连 AI 评论也插不进去 |
   | **只有一列写成值类型** | 该列当"非作者侧"时恒为 `0` → **那一类评论写不进去**，另一类照常 |

   也就是说"只有一列写错"时，**列表照常有数据、另一类评论照常能发**——
   直到有人发某一类评论拿到 500 才暴露。**编译期毫无提示，接口层早期自测也发现不了。**

   > **v1.2 实跑修正**：本节初稿写作"值类型会让**所有**插入失败"——那只对**两列都写错**成立。
   > 重新推演后拆成上表两行。**"只炸一半"才是更危险的那个**：它连"某一类评论整体挂掉"都不会立刻显现。
   > 同批实跑还纠正了分组 B 的一条验收（裸 grep 的判别力问题，见该处）。

### 1.3 `user_id` 在本表是「**作者**」不是「**归属者**」——越权防线**不能照抄** AGENTS §4.3

这是本表与全项目其他 7 张表**最本质的区别**，也是本支最容易被写错的一条。

> ⚠️ [AGENTS §4.3](../../../AGENTS.md) 的字面规则是"查询必须同时带 `persona_id` 和 `user_id`"。
> **本表是全项目唯一一张不能照抄这条规则的表**——照搬会写出一个**静默丢数据**的 bug。

**为什么不能照抄（三条理由，缺任何一条这个结论都不成立）**：

1. **同名不同义**。§4.3 那句话里的 `user_id` 指的是"这行数据**归谁**"；
   本表的同名列指的是"这条评论是**谁写的**"。字段名一样，语义是两回事。
2. **归属信息根本不在本表**。它只存在于 `ai_moments.persona_id → personas.user_id`，
   必须多 JOIN 一层才拿得到——`moment_comments` 的**任何一列都无法单独回答**"这条评论归谁"。
3. **两个作者列各有一半恒为 `NULL`**（AI 评论的 `user_id` 恒空、用户评论的 `persona_id` 恒空，§1.2）。
   而 `NULL = ?` 永不为真——拿它们当归属条件，等于给一整类评论判了"不存在"。

| 错误写法 | 后果（**静默**，不报错） |
|---------|----------------------|
| `WHERE c.user_id = ?` 当归属条件 | AI 评论的 `user_id` 是 `NULL`，`NULL = ?` 永不为真 → **AI 评论全部消失**，用户评论照常显示 |
| `WHERE c.persona_id = ?` 当归属条件 | 用户评论的 `persona_id` 是 `NULL` → **用户评论全部消失**，AI 评论照常显示 |

两种错法都会让"评审时点一下有数据"通过——直到有人问"我自己的评论怎么没了"。

**归属只能经由 评论 → 动态 → 人设 → 账号**，两个条件缺一不可：

```sql
WHERE c.moment_id = ? AND p.user_id = ?     -- p = JOIN personas p ON p.id = m.persona_id
--    ↑ 限定属于哪条动态        ↑ 限定那条动态属于当前账号
```

**"该评论所属的人设" = `m.persona_id`（动态的归属人设），不是 `c.persona_id`（评论作者人设）。**
两者可以不同：AI 评论的作者是**同账号的其他人设**（ai-moment §6 第 5 步），拿 `c.persona_id` 当归属条件必然错。

完整 SQL 形态见 [ai-moment §4.4⑤](../ai-moment/spec.md)（已预写定稿，**以它为准，本支不改**）；
后端逐端点的必带条件见 [ai-moment §4.3](../ai-moment/spec.md)。

### 1.4 注释只写必要的（[AGENTS §5.1](../../../AGENTS.md)）

`.go` 里**只留三类注释**：① 安全/越权红线；② 非显然的 GORM tag 语义；③ 函数职责一句话。
**禁止** DDL 原文逐字对照、长篇学习性解释、反例分析、历史故事——全部进 `.learn/moment-comment-model.md`。

**目标密度：全文件 ≤ 15 行注释**（§3 定稿实测 8 行）。上限才是要防的东西，为凑行数加注释本身就违反 §5.1。

> **风格冲突沿用 ai-moment 的决策**：既有 `user.go` / `persona.go` / `chat_message.go` 的注释远超 §5.1，
> **本支不清理它们**；新文件从简。审查时**不以"与既有文件风格不一致"为退回理由**。

---

## 2. 字段对照表（DDL ↔ Go ↔ tag ↔ JSON）

> DDL 原文见 [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl)，**此处不复制**，只做翻译对照。

| DDL 列 | Go 字段 | GORM tag 关键段 | JSON 键 |
|--------|---------|----------------|---------|
| `id BIGSERIAL PRIMARY KEY` | `ID uint64` | `type:bigint;primaryKey;autoIncrement` | `id` |
| `moment_id BIGINT NOT NULL REFERENCES ai_moments(id)` | `MomentID uint64`（**值**类型，NOT NULL） | `type:bigint;not null;index:idx_comments_moment,priority:1` | `momentId` |
| `persona_id BIGINT REFERENCES personas(id)` | `PersonaID *uint64` | `type:bigint;check:chk_comment_author,...` | `personaId` |
| `user_id BIGINT REFERENCES users(id)` | `UserID *uint64` | `type:bigint` | `userId` |
| `content TEXT NOT NULL` | `Content string` | `type:text;not null` | `content` |
| `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` | `CreatedAt time.Time` | `type:timestamptz;not null;default:now();autoCreateTime;index:idx_comments_moment,priority:2` | `createdAt` |
| `CONSTRAINT chk_comment_author CHECK (...)` | 挂在 `PersonaID` 上 | `check:chk_comment_author,<条件>` | — |

**三条翻译纪律**：

- **只有 `ID` 带 `autoIncrement`，三个外键列都不带**（`moment_id` / `persona_id` / `user_id`）→ 直接决定 §1.1 能不能过。
- **索引 `idx_comments_moment` 是 `(moment_id, created_at)`，`created_at` 不带 `sort:desc`**——
  与 `ai_moments` 的 `idx_moments_persona_time`（DDL 上就是 DESC）不同。
  照抄上一条的 tag 会建出 `DESC` 索引，与 DDL 不符。**逐列看 DDL，不要照抄隔壁文件。**
- **CHECK 只挂在 `PersonaID` 一处**：GORM 按**约束名**做 map key（`schema/constraint.go:32`），
  同名在两处挂不会重复建约束，但没有必要，且两处挂会给"改一处漏一处"留下空间。

### 2.1 预期渲染结果（2026-09-18 已实测，非推断）

> **实测方法**：把 **§3 的代码本身**（不是近似写法）作为临时文件放进 `package model`，`go build ./...` + `go vet` 通过后，
> 用 `gorm.Config{DryRun: true}` 捕获 `CreateTable` 的语句——**未连库、未改任何表**。
> 同时跑了 §5 分组 A/D 的全部静态检查，读数见下。
> **验收时拿这份逐字对照 `\d moment_comments`**，比对着 DDL 原文找差异更直接。
> （下为**格式化后的等价形式**，GORM 实际发的是一条单行语句。）

```sql
CREATE TABLE "moment_comments" (
  "id" bigserial,
  "moment_id" bigint NOT NULL,            -- ← 无 DEFAULT：§1.1 的头号验收项
  "persona_id" bigint,                    -- ← 可空
  "user_id" bigint,                       -- ← 可空
  "content" text NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_moment_comments_persona" FOREIGN KEY ("persona_id") REFERENCES "personas"("id") ON DELETE CASCADE,
  CONSTRAINT "fk_moment_comments_user"    FOREIGN KEY ("user_id")    REFERENCES "users"("id")    ON DELETE CASCADE,
  CONSTRAINT "fk_moment_comments_moment"  FOREIGN KEY ("moment_id")  REFERENCES "ai_moments"("id") ON DELETE CASCADE,
  CONSTRAINT "chk_comment_author" CHECK ((persona_id IS NOT NULL AND user_id IS NULL) OR (persona_id IS NULL AND user_id IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS "idx_comments_moment" ON "moment_comments" ("moment_id","created_at");
```

**同批实跑出的读数**（供落码后对照，都是**真实读数**不是预期值；grep 一律针对 `internal/model/moment_comment.go`）：

| 检查 | 命令 | 读数 |
|------|------|:---:|
| §1.1 头号项 | `moment_id` 的 DataType / HasDefault | `bigint` / `false` |
| 分组 A | `grep -c "bigserial"` | **0** |
| 分组 A | `grep -c 'gorm:"[^"]*autoIncrement'` | **1** |
| 分组 A | `grep -c "authorName\|author_name"` | **0** |
| 分组 B | `grep -c 'check:chk_comment_author'` | **1** |
| 分组 D | `grep -c "对应 SQL\|CREATE TABLE"` | **0** |
| 分组 D | 注释行数 | **8** |
| 分组 A | `go build ./... && go vet` | 通过 |

**三条读法**（都是实测读出来的，不是推的）：

1. `id` 渲染成 `bigserial` 是**正常的**——`type:bigint + autoIncrement` 的既定渲染结果，与既有 7 张表一致。
   关键在 `DataType` 是 `bigint` 而不是 `bigserial`，那才是被复制给外键列的东西（[ai-moment §1.1](../ai-moment/spec.md)）。
2. `moment_id` 是 **`bigint NOT NULL`**，没有 `DEFAULT`。同一 struct 把该列的 `autoIncrement` 打开后，
   实测渲染为 **`"moment_id" bigserial NOT NULL`**——这就是 §4.3 反例 7 的物理形态，也是 §1.1 ② 要堵的机制。
3. **外键约束名是 GORM 自动派生的**（`fk_moment_comments_*`），与 DDL 里那种内联 `REFERENCES` 被 PG 自动命名的
   `moment_comments_persona_id_fkey` **不同**。这是既有 7 张表一贯的现状，**不是缺陷，不要当验收项退回**。

---

## 3. 定稿代码（**逐字对照，不要临场发挥**）

`backend/internal/model/moment_comment.go`：

```go
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
```

`backend/internal/model/migrate.go`——**只加一行**，位置在 `&AIMoment{}` 之后：

```go
		&MomentComment{},    // 引用 ai_moments(id) / personas(id) / users(id)，三张表都要先建好
```

> 顺序理由与 `migrate.go` 顶部注释一致：被引用的表必须排在前面。`AIMoment` / `Persona` / `User` 都已在它之前。
>
> **`authorName` 不在上面的 struct 里，且永远不许加**——它是派生字段（`COALESCE(p.name, u.username)`），
> 属于契约 §8 的响应字段、不属于表。加进 model 会让 `AutoMigrate` 建出 `author_name` **真列**。
> 这是 [ai-moment §3.5](../ai-moment/spec.md) 那个 AutoMigrate 派生字段陷阱在本表的实例，本支用 §5 分组 C 的行为断言钉死。

---

## 4. 越权防线（**预写，本支不实现**，由 `feature/backend-moment-api` 落地）

### 4.1 唯一正确的归属路径

```sql
FROM moment_comments c
JOIN ai_moments m ON m.id = c.moment_id       -- 评论 → 动态
JOIN personas   p ON p.id = m.persona_id      -- 动态 → 归属人设
WHERE c.moment_id = ? AND p.user_id = ?       -- 两个条件缺一不可（§1.3）
```

两个端点（契约 §8 的 `GET/POST /moments/:id/comments`）**共用同一道归属闸门**：
先 `FindOwnedMoment(userID, momentID)`（[ai-moment §4.4②](../ai-moment/spec.md)），未命中即 `4040`，再查/写评论。
**新的写路径不要绕过闸门**——`POST` 的归属校验不能只做在 service 的 `if` 里。

### 4.2 本表**没有** ai_moments 的结构性保护（重要）

[ai-moment §4.2](../ai-moment/spec.md) 有一处很值钱的性质：`ai_moments` 没有 `user_id` 列，
所以"先查出来、再在 Go 里比"这个经典越权错法**编译不过**（`m.UserID undefined`）。

**这条保护在本表不存在**——本表**必须**有 `UserID` 字段（契约 §8 的 `"userId"`）。于是下面这段**能编译、能跑、且 review 时看起来很完整**：

```go
// ❌ 能编译过，且 review 时"看起来逻辑完整"
c, _ := repo.FindByID(ctx, commentID)          // ① 不带任何归属条件：传任意 commentID 都读得出来
if c.UserID == nil || *c.UserID != userID {    // ② 把 UserID 当"谁能看"：AI 评论恒为 nil，被整批拒掉
	return 4040
}
```

两个坑叠在一起：① 是**跨用户越权**（漏的那个 `if` 在代码里是个空洞），
② 是 §1.3 那个静默丢数据——而 ② 恰恰会让 ① 在手工点测时"看起来没问题"（列表空空如也，像是正常）。

因此本表的防线**只能靠纪律**，加固手段有三条，下游必须逐条落实：

1. **命名显式可见**（沿用 [ai-moment §4.5](../ai-moment/spec.md)）：`ListCommentsByUser` / `CountCommentsByUser`，
   名字里没有 `User` / `Owned` 的方法一律当可疑对象。
2. **禁止无作用域的读方法**：`FindByID(commentID)`、`First(&c, id)`、`Where("moment_id = ?")` 单独出现——
   全部禁止；评论的读路径**只有**经 `moment_id` + `user_id` 的闸门这一条。
3. **加一条运行时越权用例**（§5 分组 E），不靠 code review 的"我看了"。

### 4.3 本表反例清单（下游 AI 最容易写出来的错法）

| # | 错法 | 后果 |
|---|------|------|
| 1 | `WHERE c.user_id = ?` 当归属条件 | AI 评论全部消失（§1.3） |
| 2 | `WHERE c.persona_id = ?` 当归属条件 | 用户评论全部消失（§1.3） |
| 3 | 只 `WHERE c.moment_id = ?`，不 JOIN `personas` | 读/写**别人**动态下的评论 |
| 4 | `PersonaID` / `UserID` 写成值类型 `uint64` | 契约违反（`0` 而非 `null`）；该列恒非空 → 见 §1.2 的两种后果（**只有一列写错时只炸一半，最难发现**） |
| 5 | `authorName` 加进 `model.MomentComment` | AutoMigrate 建出 `author_name` 真列；同时构造了"归属信息可以离开 SQL"的错觉 |
| 6 | CHECK 漏写约束名（`check:persona_id IS NOT NULL ...`） | 约束名变成派生的 `chk_moment_comments_persona_id`（**2026-09-18 实测**：同形态的 `chk_unnamed_check_a`），与权威 DDL 不符 |
| 7 | `MomentID` 误加 `autoIncrement` | 漏传 `moment_id` 静默挂错动态（§1.1） |
| 8 | 两条 `LEFT JOIN` 只写了 `personas` 那条 | AI 评论的 `authorName` 恒为空（用户评论那条没 join `users`） |
| 9 | 给 `UserID` / `PersonaID` 写 `json:"-"`（**照抄 `chat_messages.user_id` 的习惯**） | 契约 §8 的 `Comment` 少了 `userId` / `personaId`，前端拿不到作者身份。**本表是唯一反向的表**：别处的 `user_id` 是"越权防线载体，不给前端看"，本表的 `user_id` 是**响应字段** |

> 反例 4 / 5 / 7 / 9 是**本支可验**的（§5 分组 C）；1 / 2 / 3 / 8 属于 API 分支，登记在 §5 分组 E。

---

## 5. 验收标准

> **正反两面都写**（沿用 [ai-moment §9](../ai-moment/spec.md) 的既定格式）：
> 每条"不许有什么"都配一条"必须有什么"，否则"写漏"类缺陷会全部静默通过。

### 分组 A · 建表与结构（**本支，必须全绿**）

- [ ] `AutoMigrate` 追加 `&MomentComment{}` 后，`\d moment_comments` 与 [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl) **逐列一致**
  - [ ] **必须恰好 6 列**：`id` `moment_id` `persona_id` `user_id` `content` `created_at`
  - [ ] **多出的列必须是零**——特别是**没有** `author_name`（§3 派生字段陷阱）
  - [ ] **列宽/类型逐个看**，不能只看列名：`id` / `moment_id` / `persona_id` / `user_id` 都是 `bigint`，
        `content` 是 `text`，`created_at` 是 `timestamp with time zone`
  - [ ] `persona_id` / `user_id` 必须是**可空**列（`\d` 里这两行**没有** `not null`）
  - [ ] **`moment_id` 的 Default 必须为空**（§1.1 ①，**本支头号验收项**）
  - [ ] `persona_id` / `user_id` 的 Default 也必须为空（§1.1 复制污染的另两个检查点）
- [ ] **序列（`\ds`）——正反两面**：
  - [ ] 新增序列**恰好 1 个**：`moment_comments_id_seq`（来自 `id` 的 `autoIncrement`）
  - [ ] 搜 `moment_comments_moment_id_seq` → **0 结果**（§1.1 ①，**头号验收项**）
  - [ ] 搜 `moment_comments_persona_id_seq` / `moment_comments_user_id_seq` → **各 0 结果**
- [ ] **三条外键全部存在且都是 `ON DELETE CASCADE`**：`moment_id → ai_moments`、`persona_id → personas`、`user_id → users`
- [ ] `idx_comments_moment` 存在，且是 `(moment_id, created_at)`——**不是** `(moment_id, created_at DESC)`（§2 第三条纪律）
- [ ] `chk_comment_author` 存在（见分组 B）
- [ ] **tag 静态检查**：
  - [ ] `grep -n "bigserial" internal/model/moment_comment.go` → **零命中**
  - [ ] `grep -n 'gorm:"[^"]*autoIncrement' internal/model/moment_comment.go` → **恰好 1 行**（只在 `ID` 上）
        > 必须限定在 `gorm:"..."` 内：`MomentID` / `UserID` 的注释在描述"不带 autoIncrement"，裸 grep 会误命中。
  - [ ] `grep -c "authorName\|author_name" internal/model/moment_comment.go` → **0**
- [ ] `go build ./... && go vet ./... && go test ./...` 全过
- [ ] **级联（正面）**：删一个人设 → `SELECT count(*) FROM moment_comments WHERE moment_id IN (SELECT id FROM ai_moments WHERE persona_id = <已删id>)` = 0
- [ ] **级联（反面）**：上一步之后，**别人**动态下的评论**仍然在**（不能把全表清了才会有这个结果——所以先造两条动态）

### 分组 B · CHECK 与二选一行为（**本支，可跑**）

> CHECK 是"翻译 Struct"时最容易**整行丢掉**的东西：丢了不影响编译、不影响跑通，只在数据脏了以后才暴露。
> 所以不能只 `\d` 看一眼就算过——**要证明它在拦人**。

- [ ] `\d moment_comments` 里 `chk_comment_author` 在，条件文本与 DDL 一致
- [ ] **它在拦人（正面证据）**：手工 `INSERT` 两个作者**都非空** → 数据库**报错拒绝**（`23514`）
- [ ] **它在拦人**：手工 `INSERT` 两个作者**都为空** → **报错拒绝**
- [ ] **它不误伤**：恰好一个非空 → **成功**（AI 评论、用户评论各插一行验证）
- [ ] `grep -c 'check:chk_comment_author' internal/model/moment_comment.go` → **恰好 1**（带 `check:` 前缀的只有 `PersonaID` 那一行）
      > **v1.2 实跑修正**：原验收写作"裸 grep `chk_comment_author` ≥1 且只挂在 `PersonaID` 一处"，
      > 实跑读数是 **3**（类型注释、字段注释、tag 各一）——注释里的那两处让该写法**没有判别力**，
      > 漏写 tag 时仍会命中 2 次而"看起来通过"。计数必须带 `check:` 前缀。

### 分组 C · 序列化行为（**本支**）

> 沿用 [ai-moment §9 分组 C](../ai-moment/spec.md) 的"键集**恰好**相等"模式（`internal/model` 下的第二个测试文件）。
> 本表比 `ai_moments` 多一个必须验的东西：**`null` 与 `0` 的区别**——那是本表二选一模型的全部意义。

- [ ] `internal/model/moment_comment_test.go`：marshal 一个 `MomentComment`，断言键集**恰好**是
      `{id, momentId, personaId, userId, content, createdAt}`（**6 个**）
  - [ ] 键集里**没有** `authorName`（§3 派生字段陷阱）
  - [ ] 键集里**没有** `emotionLabel` / 任何 `emotion` 字样（[AGENTS §4.4](../../../AGENTS.md)）
- [ ] **`null` 而非 `0`（本表独有）**：
  - [ ] 造 AI 评论（`PersonaID` 有值、`UserID` 为 `nil`）→ JSON 里 `userId` 是 **`null`**，**不是 `0`**
  - [ ] 造用户评论（反过来）→ JSON 里 `personaId` 是 **`null`**，**不是 `0`**
- [ ] **反向验证（注入缺陷，确认检查会报红）**：

  | # | 注入的缺陷 | 期望哪条报红 | 实际读数 |
  |---|-----------|-------------|---------|
  | 1 | `PersonaID` 改成值类型 `uint64` | 分组 C 的 `null` 断言 | ⬜ 待跑（tag 语义已于 2026-09-18 预热验证：值类型字段留零值也会被写进 INSERT） |
  | 2 | 给 struct 加 `AuthorName string \`json:"authorName"\`` | 分组 C 的键集断言 | ⬜ 待跑 |
  | 3 | `MomentID` 的 tag 加 `autoIncrement` | 分组 A 的 `autoIncrement` 恰好 1 行 | ⬜ 待跑（渲染结果已于 2026-09-18 预热验证：变成 `"moment_id" bigserial`） |
  | 4 | 删掉 `check:chk_comment_author,...` | 分组 B 的 `\d` 里约束消失 | ⬜ 待跑 |
  | 5 | `ID` 的 tag 改成 `type:bigserial` | 分组 A 的 `bigserial` 零命中 | ⬜ 待跑 |
  | 6 | 删掉 `Moment AIMoment` 关联字段 | 分组 A 的 `\d` 里外键消失 | ⬜ 待跑 |

  **任何一条注入后检查仍然"通过"，说明那条验收是假的**——先修验收，再改代码。读数连同日期提交。

> **"预热验证"是什么意思**：§2.1 那张读数表是拿 §3 的代码本身跑出来的，所以**当前设计**下静态检查全绿、DDL 渲染正确。
> 但**它替代不了注入**——注入验的是反方向的问题："代码变坏时，这条检查会不会报红"。
> 只有把代码**故意改坏**才测得出来。6 条注入一条都不能省。

> ⚠️ **诚实说明第 5 条的局限**：`bigserial` 的真正后果（本列长出 `nextval`）**本支观测不到**——
> 上游 `ai_moment.go` 已是对的，本支也改了 `ai_moment.go` 就违反范围。本支能证明的是"静态检查会报红"，
> 以及"当前代码下 `\ds` 是干净的"。**污染的可观测性在 ai-moment 分支就已接受为缺口**（§8.1 决策 11）。

### 分组 D · 注释与流程（**本支**）

- [ ] `moment_comment.go` 注释 **≤ 15 行**（§1.4）——定稿实测 **8 行**
- [ ] `grep -cn "对应 SQL\|CREATE TABLE" internal/model/moment_comment.go` → **零命中**
- [ ] 无长篇学习性解释 / 反例分析 / 历史故事（内容已挪到 `.learn/`）
- [ ] `.learn/moment-comment-model.md` 已写；`git status` 里**不出现**它（`.gitignore:75` 已覆盖）
- [ ] 未改 `router.go`、`go.mod`、他人的 model 文件；**未改 `ai_moment.go`**
- [ ] `git diff --name-only` 只含本支 4 个文件（含 `.learn/` 则为异常）
- [ ] commit 符合 `<type>(<scope>): <subject>`，scope 用 `moment`；PR 已开、至少 1 人 Approve

### 分组 E · 交接给下游的验收（**不在本支**，登记以免断档）

**给 `feature/backend-moment-api`**（越权防线，§4）：

- [ ] **反例（必须 `4040`，且库里无新增行）**：A、B 两账号各有人设与动态
  - [ ] A 的 Token 打 `GET /moments/:id/comments`，`:id` 用 **B 的动态** → `4040`（**不是空列表**）
  - [ ] A 打 `POST /moments/:id/comments`，`:id` 用 **B 的动态** → `4040`，`moment_comments` **没有新增行**
  - [ ] 不存在的 id → `4040`，与"别人的动态"**同码同文案**
- [ ] **正面**：A 打自己的动态 → 两个端点全 `200`
- [ ] **不产生 `4030` / `4043`**：任意输入组合下只出现 `4040` / `4010` / `4001` / `200`（[ai-moment §4.7](../ai-moment/spec.md)）
- [ ] **注入缺陷 7**：把归属闸门从 `ListCommentsByUser` 里删掉 → 上面那条反例必须报红
- [ ] **`authorName` 正确**：AI 评论 = `personas.name`，用户评论 = `users.username`（`COALESCE`，两条 `LEFT JOIN` 缺一不可）
- [ ] **分页稳定**：两条评论 `created_at` 相同时 `pageSize=1` 逐页拉**不重不漏**（`ORDER BY c.created_at ASC, c.id ASC`）
- [ ] `grep -i emotion backend/internal/dto/moment_dto.go` → **零命中**

**给 `feature/backend-moment-job`**（**写入侧**的「评论作者人设归属」校验归这里）：

> 分工界线：**本支只做查询侧归属**（§1.3 / §4，已定稿预写）；**写入侧**校验不在本支，也不在 `backend-moment-api`。

- [ ] **评论作者必须同账号**：AI 评论的 `persona_id` 所属 `user_id` **等于**该动态所属 `user_id`（[ai-moment §4.6 反例 10](../ai-moment/spec.md)）——
      否则 AI 会以**别人账号的人设**身份在动态下留言
- [ ] **CHECK 生效于写入路径**：job 写 AI 评论时 `user_id` 留 `nil`，不是 `0`（否则 INSERT 直接 `23514`）

**给 `moment_likes` 分支**：

- [ ] 完整序列验收：`\ds` 里与朋友圈相关的序列**恰好 3 个**，**没有** `moment_likes_moment_id_seq`（[ai-moment §9 分组 E 第 1 条](../ai-moment/spec.md) 的最终收口）
- [ ] 完整级联链：删人设 → 动态、评论、点赞三表清零；删账号同理

---

## 6. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|------|------|------|------|
| 2026-09-18 | v1 | 创建 | 朋友圈评论模型开工前的设计与验收基线；承接 [ai-moment §9 分组 E](../ai-moment/spec.md) 的 `moment_id` Default 遗留项 |
| 2026-09-18 | v1.1 | 新增 §2.1「预期渲染结果」——用 DryRun 实测出 `AutoMigrate` 会发的语句（未连库、未改表），把 §1.1 的 `moment_id` 无 DEFAULT 与 §4.3 反例 6/7 从"推断"变成"实测"；分组 C 的注入表标注了 3 条预热验证及其**不能替代**最终注入的说明 | 设计阶段先证明验收项可触发，避免落码后才发现验收是假的 |
| 2026-09-18 | v1.3 | 人工审查拍板：§1.3 补「**为什么不能照抄 AGENTS §4.3**」三条理由（同名不同义 / 归属信息不在本表 / 两个作者列各有一半恒 `NULL`）；确认**本支只做查询侧归属**，**写入侧**的「评论作者人设归属」校验明确交接 `feature/backend-moment-job`（分组 E）；§1.2 与分组 B 各自标注 v1.2 那两处实跑修正的来龙去脉 | 审查意见① ② ③ 落纸 |
| 2026-09-18 | v1.2 | **用 §3 的代码本身实跑**（临时文件放进 `package model`，`go build` + `go vet` 通过后 DryRun 渲染）：§2.1 补「同批实跑出的读数」表（7 项静态检查的真实读数）；`chk_comment_author` 的验收从"裸 grep ≥1 且只在 PersonaID 一处"改为带 `check:` 前缀的**恰好 1**（裸 grep 实测 3 处，原写法没有判别力）；§1.2 更正"值类型让所有插入失败"为**两种后果**（只有一列写错时只炸一半，更难发现） | 落码前的实跑暴露了验收项与 §1.2 的两处不准确 |

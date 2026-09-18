# spec · 朋友圈点赞（Moment Like）· 数据模型

> 本文件是**合同**：定义本分支做什么、做到什么算完成，以及下游分支必须遵守的预写设计。
> 合并后即冻结，任何字段或行为变更必须升版本并在群里广播。
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：DDL 原文、契约字段、错误码表、通用约定一律**不复制**，只给链接；
> 本文件只写**本表独有**的内容——独有陷阱、独有决策、独有验收项。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-moment-like-model` |
| 状态 | v1 待人工审查（**未过审不动手写代码**） |
| **本支范围** | **模型层：`MomentLike` struct + `migrate.go` 一行**（§0.1） |
| 负责人 | 成员 3（`JMX2033`） |
| 依赖分支 | `feature/backend-ai-moment-model`、`feature/backend-moment-comment-model`（**均已合并进 main**） |
| 关联契约 | [API_CONTRACT §8](../../API_CONTRACT.md#8-ai-朋友圈moments)（`Moment.liked` / `likeCount` + 点赞端点） |
| 关联设计 | [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl)（`moment_likes` DDL）、[§5.5](../../TECH_DESIGN.md#55-ai-朋友圈p1) |
| 关联 spec | [ai-moment/spec.md](../ai-moment/spec.md)（§7 预写 DDL、§4.4③④ 点赞 SQL、§4.6 反例、§9 分组 E 遗留项）、[moment-comment/spec.md](../moment-comment/spec.md)（姊妹表） |
| 关联红线 | [AGENTS §4.3](../../../AGENTS.md)（数据边界）、§5.1（注释纪律）、§7 |

---

## 0. 范围声明（**先读这一节**）

### 0.1 本支只做四件事

| # | 文件 | 动作 | 内容 |
|---|------|------|------|
| 1 | `backend/internal/model/moment_like.go` | 新增 | `MomentLike` struct（§3）+ `TableName()` |
| 2 | `backend/internal/model/migrate.go` | 修改 | 追加一行 `&MomentLike{}` |
| 3 | `backend/internal/model/moment_like_test.go` | 新增 | JSON 键集断言（§5 分组 C） |
| 4 | `.learn/moment-like-model.md` | 新增（**不入库**） | 学习笔记：幂等约束形态、事务边界、反例展开 |

> **文件命名** `moment_like.go`（单数），对齐既有 `ai_moment.go` / `moment_comment.go` ↔ 表名复数的惯例。

### 0.2 本支不做什么（**其余全部交接，去向逐个标明**）

| 不做的事 | 去向 |
|---------|------|
| `repository/moment_repo.go` 的点赞 SQL（§4 的幂等三连、事务边界） | `feature/backend-moment-api` |
| `dto/moment_dto.go`（`MomentItem.liked` 的 `LEFT JOIN`） | `feature/backend-moment-api` |
| `service` / `handler` / `RegisterMomentRoutes` | `feature/backend-moment-api` |
| 计数对账（`like_count` 与 `COUNT(*)` 的重算） | `feature/backend-moment-api`；若做成定时任务则由 `feature/backend-moment-job` 接手 |
| `router.go` 的那一行 | **成员 1**（我不碰这个文件） |
| 前端 `views/moments/**` | 前端分支 |

> ⚠️ **本支不含任何 SQL**。§4 的写入路径是**预写设计**，供 `feature/backend-moment-api` 落地；
> 配套的**运行时**验收（重复点不累加、跨账号 `4040`）登记在 §5 分组 E，不在本支跑。

---

## 1. 本表独有的硬性要求（违反任何一条都不要提交）

### 1.0 先记一条设计事实：**本表没有 `persona_id`**

设计过程中出现过一次「要求 ↔ DDL」的冲突：口头要求写的是"外键指向 `personas(id)`""`UNIQUE (persona_id, moment_id)`"，
而权威 DDL 里**没有这一列**。核对三处独立来源后按 DDL 定案：

| 来源 | 原文 |
|------|------|
| [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl) | `user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE`；`CONSTRAINT uq_moment_like UNIQUE (user_id, moment_id)` |
| [API_CONTRACT §8](../../API_CONTRACT.md#8-ai-朋友圈moments) | `liked` = "**当前登录用户**是否已点赞"；正文"`moment_likes` 上有 `UNIQUE (user_id, moment_id)`" |
| TECH_DESIGN 实体关系图 | "moment_likes（**用户点赞**，每人每条约一次）" |

**结论**：点赞人 = 当前登录用户，**记在 `user_id`**；本表**不挂人设**。

> 这条单独成节，是因为它是本支最容易从「上一支的肌肉记忆」里带错的点：
> `moment_comments` 那张表**确实有** `persona_id`（AI 评论要挂人设），本表**没有**——**两张姊妹表的形状不同，不要互抄。**

### 1.1 两个外键列的 Default 都必须为空（**ai-moment 遗留项的最终收口**）

> 来源：[ai-moment/spec.md §9 分组 E 第 1 条](../ai-moment/spec.md) —— "`\ds` 新增序列恰好 3 个，
> 没有 `moment_comments_moment_id_seq` 等（随 §7 两个 model 落地）"。
> `moment_comments` 支已闭合**一半**（[该支 spec §1.1](../moment-comment/spec.md)）；**本支闭合另一半**，此后这条遗留项彻底关闭。

`moment_likes` 有**两个**外键列（`user_id` / `moment_id`），**两个都 `NOT NULL`**——
比 `moment_comments` 更像"该有默认值"的样子，也更像上游 `bigserial` 污染会咬的地方。
一旦带上默认值，漏传的那一列**不会报错**，会拿到一个自增值。**静默**才是危险的地方。

**两个产生机制**（原理见 [ai-moment §1.1](../ai-moment/spec.md)，本支不重复）：

| # | 机制 | 状态 |
|---|------|------|
| ① | 上游 `ai_moments.id` 写成 `bigserial`，GORM 把该 `DataType` **复制**到本列 | ai-moment 支已用 `type:bigint` 切断；**本支与 comment 支共同确认它真的生效** |
| ② | 本字段自己误加 `autoIncrement` | 本支的 tag 纪律（§2 对照表） |

**验收（§5 分组 A）**：`\d moment_likes` 里 `user_id` / `moment_id` 两行的 Default 列**都为空**。

> ⚠️ **要求里给的那条验收命令必须改**。原话是"`\ds` 里搜 `moment_likes_persona_id_seq` 应 0 结果"——
> **那条是无判别力的**：本表没有 `persona_id` 列（§1.0），这个序列名**恒不存在**，
> 无论代码写得多烂都会得到 0。它长得像验收，实际是空的。
> 真正有判别力的两个名字是本表两个外键列的序列名：
> **`moment_likes_user_id_seq`** 和 **`moment_likes_moment_id_seq`**。

**序列验收的正反两面**（§5 分组 A）：

- **不许有**：`moment_likes_user_id_seq` / `moment_likes_moment_id_seq` → 各 **0 结果**
- **必须有**：`moment_likes_id_seq` **恰好 1 个**（来自 `id` 的 `autoIncrement`）。
  只有"没有多余序列"是不够的——把 `ID` 的 `autoIncrement` 也删掉会同时满足那一条，却让本表再也插不进数据。
- **完整收口**：与朋友圈相关的序列**恰好 3 个**（`ai_moments_id_seq` / `moment_comments_id_seq` / `moment_likes_id_seq`）。
  三个 model 此刻才首次同时出现在 `migrate.go` 里，这条验收**本支才第一次跑得动**。

> **若验收失败怎么办**：说明上游 `ai_moment.go` 的 `ID` tag 被改坏了（不是本支的代码）。
> 修复要去改 `ai_moment.go` 并**另开 `fix/` 分支**，不要在本支顺手改别人已合并的 model 文件。
> 注意 GORM `AutoMigrate` **不会**移除已存在列的 DEFAULT——改完 tag 后需要**重建该表**（仅限开发库，[AGENTS §7.8](../../../AGENTS.md)）。

### 1.2 `uq_moment_like` 是幂等的物理基础，且**必须按列清单引用**

本表是**全项目第一张靠数据库约束（而不是靠 Go 代码的 `if`）保证幂等的表**。
契约 §8 明写"点赞是幂等的……点第二次返回相同结果，不会减一"，兜底的就是这条唯一约束。

**tag 是一个复合 `uniqueIndex`，必须同时挂在两列上**：

```go
UserID   ...;uniqueIndex:uq_moment_like,priority:1;...   // priority 决定列序，user_id 在前
MomentID ...;uniqueIndex:uq_moment_like,priority:2;...
```

- 只挂一列 → 退化成**单列**唯一索引。两列里少了哪一列都同样是灾难：
  只挂 `user_id` = **一个人一辈子只能点一次赞**（整张表废掉）；只挂 `moment_id` = **一条动态只能被一个人点**。
- priority 写反 → 索引列序变成 `(moment_id, user_id)`，`\d` 与 DDL 不符。

> ⛔ **不要试图用 `unique:` 换掉 `uniqueIndex:` 去"凑出"一个 CONSTRAINT**（本支实测，见下方 🚨）。
> GORM 的 `unique:` **表达不了复合唯一**——它在两列上各建一个**单列** UNIQUE 约束，
> 且**完全忽略你给的名字**：
>
> ```
> A uint64 `gorm:"...;unique:uq_y,priority:1"`   ┐  渲染成
> B uint64 `gorm:"...;unique:uq_y,priority:2"`   ┘  CONSTRAINT "uni_<表名>_a" UNIQUE ("a")
>                                                   CONSTRAINT "uni_<表名>_b" UNIQUE ("b")
> ```
>
> `uq_y` 这个约束**根本不存在**。而这两条单列约束正好就是上面那两个灾难分支的**叠加**：
> 一个人整个平台只能点一次赞 + 一条动态整个平台只能被点一次。
> 它**建表不报错**，症状要等到第二个用户去点赞时才出现——**比只挂一列更隐蔽**。

> 🚨 **实测出来的一个形态差异（本支新发现）**：DDL 写的是 `CONSTRAINT uq_moment_like UNIQUE (...)`，
> 而 GORM 的 `uniqueIndex` 渲染出来是 **`CREATE UNIQUE INDEX "uq_moment_like" ...`**——**唯一索引，不是唯一约束**。
>
> | | DDL 字面（`CONSTRAINT ... UNIQUE`） | GORM 实际建的 |
> |---|---|---|
> | `\d` 里显示为 | `"uq_moment_like" UNIQUE **CONSTRAINT**, btree (...)` | `"uq_moment_like" UNIQUE, btree (...)` |
> | `ON CONFLICT ON CONSTRAINT uq_moment_like` | 可用 | **报错** `constraint "uq_moment_like" for table "moment_likes" does not exist` |
> | `ON CONFLICT (user_id, moment_id)` | 可用 | 可用 |
>
> **这不是缺陷**（唯一性照常强制、列清单形式照常工作），但它把 [ai-moment §4.6 反例 7](../ai-moment/spec.md)
> 从一句警告变成了**物证**。审查时**不要**因为 `\d` 里少了个 `UNIQUE CONSTRAINT` 字样就退回本支
> （同 [moment-comment §2.1 读法 3](../moment-comment/spec.md) 对外键名的处理）。

**由此得到本支对外的一条硬规则（下游一律照此写，`moment_like.go` 的注释里也写了同一句）：**

```sql
-- ✅ 唯一正确形态：列清单
INSERT INTO moment_likes (user_id, moment_id) VALUES (?, ?)
ON CONFLICT (user_id, moment_id) DO NOTHING;

-- ❌ 禁止：uq_moment_like 建成的是「唯一索引」不是「唯一约束」，这句直接报错
--    错误信息：constraint "uq_moment_like" for table "moment_likes" does not exist
INSERT INTO moment_likes (user_id, moment_id) VALUES (?, ?)
ON CONFLICT ON CONSTRAINT uq_moment_like DO NOTHING;
```

> **为什么容易写错**：DDL 明明白白写着 `CONSTRAINT uq_moment_like UNIQUE (...)`，
> 照着 DDL 的字面去写 `ON CONSTRAINT uq_moment_like` 是**最自然**的推断。
> 差别只在"GORM 把它建成了索引而非约束"这一层——**这层不出现在任何文档里，只在 `\d` 的输出里**。
> §5 分组 B 因此把"`ON CONSTRAINT` 必须报错"也列为验收项：**报错说明形态与本文一致**。

### 1.3 `like_count` 是冗余计数：**两次写必须同事务，且自增必须有条件**

这是本表**真正的功能核心**，也是全项目唯一一处"一次用户操作要写两张表"的地方。

`ai_moments.like_count` 的 DDL 注释写着"**冗余计数，真值以 `moment_likes` 为准**"——
同一件事存了两份，于是有一条**不变量**：

```
ai_moments.like_count  ==  SELECT count(*) FROM moment_likes WHERE moment_id = ?
```

一次点赞 = 三个动作（[ai-moment §4.4③④](../ai-moment/spec.md) 已预写定稿，**以它为准，本支不改**）：

```
① INSERT INTO moment_likes ... ON CONFLICT (user_id, moment_id) DO NOTHING   -- 幂等的闸门
② UPDATE ai_moments SET like_count = like_count + 1 WHERE id = ?             -- 只在 ① 的 RowsAffected = 1 时执行
③ SELECT like_count FROM ai_moments WHERE id = ?                             -- 同事务内读回，作为响应
```

**两种写错的后果必须分开记——它们是两个物种：一个是「可见的错」，一个是「安静的错」。**

**表 A · 可见的错**（无条件自增）

| 错法 | 症状 | 为什么它反而"幸运" |
|---|---|---|
| 自增不看 `RowsAffected` | 点 5 次 → `like_count = 5`，而 `COUNT(*) = 1` | 用户**看得见**：按钮说"已点赞"、数字却还在涨。**错会越来越大，迟早有人问** |

**表 B · 安静的错**（两次写不在同一事务）—— **这才是本表最贵的缺陷**

| 错法 | 症状 | 为什么它不可自愈 |
|---|---|---|
| INSERT 与 UPDATE 拆成两个独立事务 | INSERT 成功、UPDATE 失败 → `like_count` 少 1 | 用户**再点一次是幂等的**（`RowsAffected = 0`），**根本不会触发补写**。错误**只错一次就再也不动** |

**一句话对比**：表 A 的错**会持续放大**，所以一定会被发现；
表 B 的错**错完就冻结**，所以能一直躺在那儿没人知道。

> **安静的错比可见的错危险得多**——这正是本表与"无条件自增"那种普通 bug 的分水岭。
> 所以事务边界不是"最好加上"，是**必须**：
> [ai-moment §4.4④](../ai-moment/spec.md) 把 ② 和 ③ 写在同一段里，就是这个意思。

> **反面同样要记**：`like_count` **可以被重算**。对账 SQL 就是上面那句不变量本身
> （`UPDATE ai_moments SET like_count = (SELECT count(*) FROM moment_likes WHERE moment_id = ai_moments.id)`），
> §5 分组 B 用它做验收的最终判据。**知道有修复路径，不等于可以不做事务**——它是事故后的止血，不是设计。

### 1.4 越权防线：`user_id` 是「**点赞人**」且**恒非空**——"恒非空"反而让错觉更危险

> **与 [moment-comment §1.3](../moment-comment/spec.md) 的陷阱形态不同，结论相同。**
> 那张表的 `user_id` 一半是 `NULL`（AI 评论），错误会**丢数据**，症状是"某类评论整个消失"；
> 本表的 `user_id` **恒非空**（每行都是一个真人点的），于是不会丢数据——**错得更安静**。

本表**没有** `NULL` 陷阱，很容易滑向另一个错觉：

```sql
-- ⚠️ 看起来够了，其实不够
WHERE l.user_id = ?       -- 只证明「这一行是你点的」，不证明「这条动态是你的」
```

**归属仍然必须走三层**（与 [moment-comment §4.1](../moment-comment/spec.md) 同一条路径）：

```sql
FROM moment_likes l
JOIN ai_moments m ON m.id = l.moment_id       -- 点赞 → 动态
JOIN personas   p ON p.id = m.persona_id      -- 动态 → 归属人设
WHERE l.moment_id = ? AND p.user_id = ?       -- 两个条件缺一不可
```

**为什么"用户只能看到自己的动态，所以不可能点到别人的"这个推论不能当防线**：
那是**产品层**的推论，不是 **schema 保证**的。本表上**没有任何约束**把 `l.user_id` 绑到 `m.persona_id` 的归属者——
定时任务、将来的批量接口、一次数据修复，都能造出 `l.user_id = A` 而动态属于 B 的行。
防线不能建立在"别的分支不会写坏"之上。

**写入路径已经有结构性保护**：[ai-moment §4.4③](../ai-moment/spec.md) 预写的
`INSERT INTO moment_likes ... SELECT ... JOIN personas ... WHERE m.id = ? AND p.user_id = ?`
把归属条件**物理上焊进了写入语句**——忘了它，这条 SQL 就写不出来；而"先查后插"漏掉那个 `if` 会**静默越权**。
读路径（`liked` 的 `LEFT JOIN`）**没有**这层保护，只能靠纪律与运行时用例（§5 分组 E）。

> **本期没有取消点赞端点**（契约 §8："本期**不支持取消点赞**，点第二次返回相同结果，不会减一"）。
> 若将来新增 `DELETE /moments/:id/like`，闸门形态**完全相同**——这条记在这里，免得将来重新推一遍。

### 1.5 本表**没有** CHECK 约束（要求 6 的答案）

DDL 上**唯一的具名约束是 `uq_moment_like`，而它是 UNIQUE，不是 CHECK**。本表**一条 CHECK 都没有**。

按 [ai-moment §7 已定的判据](../ai-moment/spec.md)——"**判据永远是 DDL 上有没有，不是加了更安全**"：

| 表 | DDL 上有 CHECK 吗 | 怎么做 |
|---|---|---|
| `moment_comments` | 有（`chk_comment_author` 二选一） | 照抄（已合并） |
| **`moment_likes`** | **没有** | **不加** |

> 本表也确实不需要：`user_id` / `moment_id` 都 `NOT NULL` 且都有独立含义，
> 不存在"二选一"那种必须在数据库层拦住的非法状态。
> 加一条 DDL 上没有的 CHECK 属于**改 schema**，与 [user_memory.go](../../../backend/internal/model/user_memory.go) 的处理同源。

### 1.6 注释只写必要的（[AGENTS §5.1](../../../AGENTS.md)）

与 [moment-comment §1.4](../moment-comment/spec.md) 同一纪律，不重复。
**目标密度：全文件 ≤ 15 行注释**（§3 定稿实测 **8 行**）。上限才是要防的东西，为凑行数加注释本身就违反 §5.1。

> **风格冲突沿用既有决策**：既有 model 文件的注释远超 §5.1，**本支不清理它们**；新文件从简。
> 审查时**不以"与既有文件风格不一致"为退回理由**。

---

## 2. 字段对照表（DDL ↔ Go ↔ tag ↔ JSON）

> DDL 原文见 [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl)，**此处不复制**，只做翻译对照。

| DDL 列 | Go 字段 | GORM tag 关键段 | JSON 键 |
|--------|---------|----------------|---------|
| `id BIGSERIAL PRIMARY KEY` | `ID uint64` | `type:bigint;primaryKey;autoIncrement` | `id` |
| `user_id BIGINT NOT NULL REFERENCES users(id)` | `UserID uint64`（**值**类型，NOT NULL） | `type:bigint;not null;uniqueIndex:uq_moment_like,priority:1;index:idx_likes_user` | `userId` |
| `moment_id BIGINT NOT NULL REFERENCES ai_moments(id)` | `MomentID uint64`（同上） | `type:bigint;not null;uniqueIndex:uq_moment_like,priority:2;index:idx_likes_moment` | `momentId` |
| `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` | `CreatedAt time.Time` | `type:timestamptz;not null;default:now();autoCreateTime` | `createdAt` |
| `CONSTRAINT uq_moment_like UNIQUE (user_id, moment_id)` | 复合 `uniqueIndex` **挂在两列上** | 见 §1.2 | — |

**三条翻译纪律**：

- **只有 `ID` 带 `autoIncrement`，两个外键列都不带** → 直接决定 §1.1 能不能过。
  **两条都要查**，漏查一条就漏掉一个 `nextval`。
- **索引对象一共 3 个**，`\d` 里都要在：`uq_moment_like`（唯一，两列）、`idx_likes_user`、`idx_likes_moment`。
  `idx_likes_user` 在功能上是**冗余的**（`uq_moment_like` 已覆盖 `WHERE user_id = ?` 这个前缀），
  但**照 DDL 建**——这是 [ai-moment §7 已决策的取舍](../ai-moment/spec.md)：与 DDL 逐行可对照，比省几 MB 更有价值。
- **`like_count` 不在本表**。它属于 `ai_moments`；本表提供的是 `idx_likes_moment`（统计某条动态的点赞数走它）。
  把 `like_count` 加进 `MomentLike` 会让 AutoMigrate 在本表建出**多余的一列**（§1.3）。

### 2.1 预期渲染结果（2026-09-18 已实测，非推断）

> **实测方法**：把 **§3 的代码本身**（不是近似写法）作为临时文件放进 `package model`，`go build ./...` + `go vet` 通过后，
> 用 `gorm.Config{DryRun: true}` 捕获 `CreateTable` / `CreateIndex` 语句——**未连库、未改任何表**。
> 同时跑了 §5 分组 A/C/D 的全部静态检查，读数见下。
> **验收时拿这份逐字对照 `\d moment_likes`**，比对着 DDL 原文找差异更直接。
> （下为**格式化后的等价形式**，GORM 实际发的是一条单行语句。）

```sql
CREATE TABLE "moment_likes" (
  "id"         bigserial,
  "user_id"    bigint NOT NULL,           -- ← 无 DEFAULT：§1.1 的头号验收项之一
  "moment_id"  bigint NOT NULL,           -- ← 无 DEFAULT：另一个
  "created_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_moment_likes_user"   FOREIGN KEY ("user_id")   REFERENCES "users"("id")      ON DELETE CASCADE,
  CONSTRAINT "fk_moment_likes_moment" FOREIGN KEY ("moment_id") REFERENCES "ai_moments"("id") ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS "idx_likes_moment" ON "moment_likes" ("moment_id");
CREATE INDEX IF NOT EXISTS "idx_likes_user"   ON "moment_likes" ("user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "uq_moment_like" ON "moment_likes" ("user_id","moment_id");   -- ← 索引，不是约束（§1.2）
```

**同批实跑出的读数**（供落码后对照，都是**真实读数**不是预期值；grep 一律针对 `internal/model/moment_like.go`）：

| 检查 | 命令 | 读数 |
|------|------|:---:|
| §1.1 头号项 | `schema.Parse` → `UserID` 的 `DataType` / `HasDefault` / `AutoIncrement` | `bigint` / **`false`** / **`false`** |
| §1.1 头号项 | 同上 `MomentID` | `bigint` / **`false`** / **`false`** |
| （对照）| 同上 `ID` | `bigint` / `true` / `true`（正常，`HasDefault` 由 `autoIncrement` 带来） |
| 分组 A | `grep -c "bigserial"` | **0** |
| 分组 A | `grep -c 'gorm:"[^"]*autoIncrement'` | **1** |
| 分组 A | `grep -c 'likeCount\|like_count'` | **0** |
| 分组 B | `grep -c 'uniqueIndex:uq_moment_like'` | **2**（见读法 2） |
| 分组 A | `schema.ParseIndexes()` | 3 个：`uq_moment_like`(**class=UNIQUE**) / `idx_likes_user` / `idx_likes_moment` |
| 分组 D | 注释行数 | **8** |
| 分组 A | `grep -c 'gorm:"[^"]*check:'` | **0**（§1.5：本表没有 CHECK） |
| 分组 C | `grep -ci 'persona'` | **0**（§1.0：本表没有 `persona_id`） |
| 分组 A | `go build ./... && go vet` | 通过 |
| 分组 C | 序列化键集 | **恰好 4**：`id` `userId` `momentId` `createdAt` |

> 两个头号项**不是从渲染出的 SQL 里目测的**：`schema.Parse` 直接把
> `UserID` / `MomentID` 读成 `HasDefault=false`、`AutoIncrement=false`，
> 与 `ID` 的 `true/true` 形成对照。渲染结果（§上方 DDL）与这个读数互相印证。
>
> **这批读数是在代码落进仓库之后、对 `internal/model/moment_like.go` 本体重测的**
> （不是拿设计阶段的临时文件），且 `AutoMigrate` 走的是**真实列表**（`MomentLike` 已在 `migrate.go` 里），
> 顺带证明了 10 张表的顺序合法。

**四条读法**（都是实测读出来的，不是推的）：

1. `id` 渲染成 `bigserial` 是**正常的**——`type:bigint + autoIncrement` 的既定渲染结果，与既有 8 张表一致。
   关键在 `DataType` 是 `bigint`，那才是会被复制给外键列的东西（[ai-moment §1.1](../ai-moment/spec.md)）。
2. **`grep -c 'uniqueIndex:uq_moment_like'` 是 2 不是 1**，因为复合索引**必须挂两列**。
   > ⚠️ 别与 [moment-comment 分组 B 的 `check:` 恰好 1](../moment-comment/spec.md) 记混：
   > **命名 CHECK 挂一处（=1），复合唯一索引挂两处（=2）**。两者都是"恰好"，但数字不同，
   > 两个计数都**不能**写成"≥1"——少一处唯一性就残缺。
3. **`uq_moment_like` 是索引不是约束**（§1.2），`\d` 里**不会**出现 `UNIQUE CONSTRAINT` 字样。**这不是缺陷，不要当验收项退回。**
4. **`grep -c 'likeCount\|like_count'` 能停在 0，是注释刻意避开了这个词**（§3 那句 ⚠️ 写的是"计数列不在本表"，没写出列名）。
   这不是巧合，是 [ai-moment §3.2 定下的惯例](../ai-moment/spec.md)：**注释描述"该怎么做"，被禁值只在 spec 与 `.learn/` 里出现**，
   否则字面 grep 会被自己的注释命中而失去判别力。
   > **代价要说清楚**：这条验收因此**依赖注释的措辞**——将来有人在注释里补上列名，就会得到一个假报红。
   > 想要不依赖措辞的版本，用字段级 grep（分组 C 已并列给出）。

---

## 3. 定稿代码（**逐字对照，不要临场发挥**）

`backend/internal/model/moment_like.go`：

```go
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
```

`backend/internal/model/migrate.go`——**只加一行**，位置在 `&MomentComment{}` 之后：

```go
		&MomentLike{},       // 引用 users(id) 与 ai_moments(id)，两张表都要先建好
```

> 顺序理由与 `migrate.go` 顶部注释一致：被引用的表必须排在前面。
> 这也就是 [ai-moment §7 落地注意点 3](../ai-moment/spec.md) 说的那个顺序：`&AIMoment{}` → `&MomentComment{}` → `&MomentLike{}`。

> **`liked` / `likeCount` / `personaName` 都不在上面的 struct 里，且永远不许加**：
> 它们属于 `dto.MomentItem`——`liked` 是 `LEFT JOIN moment_likes` 的**存在性**、`likeCount` **读自 `ai_moments`**、
> `personaName` 来自 `personas.name`。加进 model 会让 `AutoMigrate` 建出**真列**。
> 这是 [ai-moment §3.5](../ai-moment/spec.md) 那个 AutoMigrate 派生字段陷阱在本表的实例，本支用 §5 分组 C 钉死。

### 3.1 本表是唯一一张「**没有任何端点返回它的行**」的表

值得单独记一笔，因为它改变了 §5 分组 C 那条测试的**性质**。

契约 §8 里点赞端点的响应是 `{ "likeCount": 3, "liked": true }`——
`likeCount` 读自 `ai_moments.like_count`，`liked` 是 `LEFT JOIN moment_likes` 的**存在性**。
**没有任何一个端点会返回一条 `MomentLike` 记录**，契约里也没有 `Like` 实体。

| | `moment_comments` | **`moment_likes`** |
|---|---|---|
| 有没有响应实体 | 有（`Comment`） | **没有** |
| `json` tag 从哪来 | 契约字段**逐字要求** | **为一致性保留**（[AGENTS §5.0](../../../AGENTS.md) 要求 JSON 一律 camelCase，不是契约要求） |
| 序列化测试验的是 | 契约键集 | **派生字段没混进 model**（防 AutoMigrate 建出真列） |

**由此得到的定位（已写进 `moment_like_test.go` 的文件注释里，逐字一致）：**

> **这条序列化测试不是在「校验契约字段」，它是一道「防派生字段混进 model 的结构探针」。**

它防的不是"契约对不上"（本表无契约可对），而是 [ai-moment §3.5](../ai-moment/spec.md) 那个
**静默**的 AutoMigrate 陷阱：往 `MomentLike` 里写一个没有 `gorm:"-"` 的字段 →
`AutoMigrate` 在本表**真的建出那一列**，`\d` 多出一列，**不报错、不影响编译**。
探针的作用就是让这件事在测试里立刻报红。

**结论**：`json` tag 照常写（取值与 struct 字段一一对应，符合 [AGENTS §5.0](../../../AGENTS.md) 的 camelCase 约定），
但**理由是这个**，不是"契约要求"。将来若真有端点要返回点赞记录
（TECH_DESIGN §6.2 给 `idx_likes_user` 的注释是"查「我点过哪些」"），届时再按契约补断言。

---

## 4. 写入路径（**预写，本支不实现**，由 `feature/backend-moment-api` 落地）

### 4.1 三条 SQL 的归属

**SQL 原文见 [ai-moment §4.4③④](../ai-moment/spec.md)，已定稿，以它为准，本支不改也不复制。**
本支只登记**它为什么必须是这个形态**（§1.3 / §1.4 已展开）。

### 4.2 `RowsAffected = 0` 是**二义**的

`ON CONFLICT DO NOTHING` 命中时返回 `0`，但它同时覆盖两种情况
（[ai-moment §4.4③ 的返回值表](../ai-moment/spec.md)）：

| `RowsAffected` | 含义 | service 动作 |
|:---:|---|---|
| `1` | 归属通过 **且** 首次点赞 | `IncrementLikeCount`，返回 `liked: true` |
| `0` | **二义**：要么已点过，**要么**不是自己的动态 | 再跑一次归属查询消歧：命中 → 幂等成功（**不**自增）；未命中 → `4040` |

**这两种情形的区分必须靠"再查一次归属"，不能靠"再查一次点赞存在性"**——
后者会把"别人的动态"和"已点过"混为一谈，于是**越权返回 `200`**。
它是本端点唯一的错误码分支，也是 §5 分组 E 那条 `4040` 用例真正在测的东西。

> 本期 `liked` 恒为 `true`（没有取消点赞），但**仍要从插入结果推导，不要硬编码**——
> 硬编码会在将来加取消点赞时静默变成 bug。

### 4.3 本表反例清单（下游 AI 最容易写出来的错法）

| # | 错法 | 后果 |
|---|------|------|
| 1 | 自增不看 `RowsAffected` | 重复点击把 `like_count` 越点越高（§1.3 错法①） |
| 2 | 两次写不同事务 | 计数永久漂移且**无法自愈**（§1.3 错法②，最危险） |
| 3 | `ON CONFLICT ON CONSTRAINT uq_moment_like` | **直接报错**：它是索引不是约束（§1.2，已实测形态） |
| 4 | `ON CONFLICT` 的列顺序写成 `(moment_id, user_id)` | 无匹配的唯一索引可用，报错 |
| 5 | 只 `WHERE l.user_id = ?` 当归属条件 | 读/写别人动态下的点赞（§1.4） |
| 6 | `liked` 的 `LEFT JOIN` **漏掉 `AND l.user_id = ?`** | A 会看到 B 点过的痕迹（[ai-moment §4.6 反例 4](../ai-moment/spec.md)） |
| 7 | 上面那个 `l.user_id = ?` 写进 `WHERE` 而不是 `ON` | `LEFT JOIN` 退化成 `INNER JOIN`，"没点赞"的行整批消失 |
| 8 | `Save()` 写回整行 | 把 `like_count` 覆盖成内存里的旧值（[ai-moment §4.6 反例 6](../ai-moment/spec.md)） |
| 9 | 把 `likeCount` / `liked` / `personaName` 加进 `model.MomentLike` | AutoMigrate 在本表建出多余真列（§3 / §3.1） |
| 10 | 给 `UserID` / `MomentID` 写 `json:"-"` | 本表的 `user_id` **不是**越权防线载体，它是唯一标识"谁点的"的字段。照抄 `chat_messages.user_id` 的习惯会中招（同 [moment-comment §4.3 反例 9](../moment-comment/spec.md)） |

> 反例 9 是**本支可验**的（§5 分组 C）；1–8、10 属于 API 分支，登记在 §5 分组 E。

---

## 5. 验收标准

> **正反两面都写**（沿用 [ai-moment §9](../ai-moment/spec.md) 的既定格式）：
> 每条"不许有什么"都配一条"必须有什么"，否则"写漏"类缺陷会全部静默通过。

### 分组 A · 建表与结构（**本支，必须全绿**）

- [ ] `AutoMigrate` 追加 `&MomentLike{}` 后，`\d moment_likes` 与 [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl) **逐列一致**
  - [ ] **必须恰好 4 列**：`id` `moment_id` `user_id` `created_at`
  - [ ] **多出的列必须是零**——特别是**没有** `like_count`（§3 派生字段陷阱）
  - [ ] **列宽/类型逐个看**，不能只看列名：三个 id 列都是 `bigint`，`created_at` 是 `timestamp with time zone`
  - [ ] **`user_id` / `moment_id` 的 Default 都必须为空**（§1.1，**本支头号验收项**）
  - [ ] `user_id` / `moment_id` 都**不是**可空列（DDL 是 `NOT NULL`，与 `moment_comments` 的两个作者列**相反**）
- [ ] **索引对象恰好 3 个**：
  - [ ] `uq_moment_like` —— **UNIQUE**，列序是 `(user_id, moment_id)`
  - [ ] `idx_likes_moment` —— `(moment_id)`
  - [ ] `idx_likes_user` —— `(user_id)`（冗余但照建，§2 第三条纪律）
  - [ ] 多出的索引是零
- [ ] **序列（`\ds`）——正反两面**：
  - [ ] 搜 `moment_likes_user_id_seq` / `moment_likes_moment_id_seq` → **各 0 结果**（§1.1 ①，**头号验收项**）
  - [ ] `moment_likes_id_seq` **恰好 1 个**（来自 `id` 的 `autoIncrement`）
  - [ ] **完整收口**：与朋友圈相关的序列**恰好 3 个**——`ai_moments_id_seq` / `moment_comments_id_seq` / `moment_likes_id_seq`；
        且**没有** `moment_comments_moment_id_seq` / `moment_comments_persona_id_seq` / `moment_comments_user_id_seq`
        （[ai-moment §9 分组 E 第 1 条](../ai-moment/spec.md) 的最终关闭）
- [ ] **两条外键全部存在且都是 `ON DELETE CASCADE`**：`user_id → users`、`moment_id → ai_moments`
- [ ] **tag 静态检查**：
  - [ ] `grep -n "bigserial" internal/model/moment_like.go` → **零命中**
  - [ ] `grep -n 'gorm:"[^"]*autoIncrement' internal/model/moment_like.go` → **恰好 1 行**（只在 `ID` 上）
        > 必须限定在 `gorm:"..."` 内：`UserID` / `MomentID` 的注释在描述"不带 autoIncrement"，裸 grep 会误命中。
  - [ ] `grep -c 'uniqueIndex:uq_moment_like' internal/model/moment_like.go` → **恰好 2**（§2.1 读法 2）
  - [ ] `grep -c "likeCount\|like_count" internal/model/moment_like.go` → **0**
- [ ] `go build ./... && go vet ./... && go test ./...` 全过
- [ ] **级联（正面）**：删一个人设 → 该人设动态下的点赞清零
- [ ] **级联（反面）**：上一步之后，**别人**动态下的点赞**仍然在**（不能把全表清了才会有这个结果——所以先造两条动态各带点赞）

### 分组 B · 唯一约束与幂等行为（**本支，可跑**）

> UNIQUE 是"翻译 Struct"时最容易**只挂一列**的东西：挂了就有唯一性、`\d` 里也有那行，
> 看起来完全正常，只在"第二个人来点赞"或"给第二条动态点赞"时才暴露。**所以要证明它真的在拦人。**

- [ ] `\d moment_likes` 里 `uq_moment_like` 在，且是 `(user_id,"moment_id")` 这个顺序
- [ ] **它在拦人（正面证据）**：同一 `(user_id, moment_id)` 连插两次 → 第二次报 **`23505`** 唯一冲突
- [ ] **它在拦人（两个方向的证明）**：
  - [ ] 同一个 user 再赞**另一条**动态 → **成功**（证明唯一性不是"一个人只能赞一次"）
  - [ ] **另一个** user 赞**同一条**动态 → **成功**（证明唯一性不是"一条动态只能被赞一次"）
- [ ] **`ON CONFLICT` 只能用列清单形式**：
  - [ ] `ON CONFLICT (user_id, moment_id) DO NOTHING` → 成功且 `RowsAffected = 0`
  - [ ] `ON CONFLICT ON CONSTRAINT uq_moment_like DO NOTHING` → **报错**（§1.2 的物证；这条**是预期报错**，不是缺陷）
- [ ] **不变量对账**（§1.3）：手工点赞若干次（**含至少一次重复点击**）后，
      `SELECT like_count FROM ai_moments WHERE id = X` **等于** `SELECT count(*) FROM moment_likes WHERE moment_id = X`

### 分组 C · 序列化行为（**本支**）

> 本表**没有契约实体**（§3.1），所以这条测试不是"校验契约"，而是**结构探针**：
> 它证明 `liked` / `likeCount` / `personaName` 这些派生字段**没有**混进 model——混进去就会被 AutoMigrate 建出真列。

- [ ] `internal/model/moment_like_test.go`：marshal 一个 `MomentLike`，断言键集**恰好**是
      `{id, userId, momentId, createdAt}`（**4 个**）
  - [ ] 键集里**没有** `likeCount` / `liked`（§3.1，本表最容易混进来的两个）
  - [ ] 键集里**没有** `personaId` / `personaName`（§1.0——本表没有 `persona_id` 列）
  - [ ] 键集里**没有** `emotionLabel` / 任何 `emotion` 字样（[AGENTS §4.4](../../../AGENTS.md)）
- [ ] **字段级静态检查**（不依赖注释措辞，见 §2.1 读法 4）：
      `grep -cE '^[[:space:]]+(LikeCount|Liked|PersonaName|PersonaID)[[:space:]]' internal/model/moment_like.go` → **0**
      > 它匹配的是 **struct 字段声明行**，注释整行以 `//` 开头、不会命中。
      > 这条与上面的键集断言是**两道独立的闸门**：键集验的是"序列化出来多了什么"，
      > 这条验的是"struct 里多了什么"——后者才是 AutoMigrate 建列的直接原因。
- [ ] **反向验证（注入缺陷，确认检查会报红）**：

  | # | 注入的缺陷 | 期望哪条报红 | 实际读数 |
  |---|-----------|-------------|---------|
  | 1 | `MomentID` 的 tag 加 `autoIncrement` | 分组 A 的 `autoIncrement` 恰好 1 行 + `\ds` 的 `moment_id_seq` | ⬜ 待跑 |
  | 2 | `UserID` 的 tag 加 `autoIncrement` | 同上（**本表多一条外键列**，要单独注入） | ⬜ 待跑 |
  | 3 | 删掉 `uniqueIndex:uq_moment_like,...`（只留一处或全删） | 分组 A 的 `uniqueIndex` 恰好 2 + 分组 B 的重复点赞 | ⬜ 待跑 |
  | 4 | 两处 `priority` 对调 | 分组 A 的索引列序 `(user_id,"moment_id")` | ⬜ 待跑 |
  | 5 | 给 struct 加 `LikeCount int \`json:"likeCount"\`` | 分组 C 的**字段级 grep** + 键集断言 + 分组 A 的"恰好 4 列" | ⬜ 待跑 |
  | 6 | 删掉 `Moment AIMoment` 关联字段 | 分组 A 的 `moment_id → ai_moments` 外键消失 | ⬜ 待跑 |
  | 7 | 删掉 `idx_likes_user` 的 tag | 分组 A 的"索引对象恰好 3 个" | ⬜ 待跑 |
  | 8 | `ID` 的 tag 改成 `type:bigserial` | 分组 A 的 `bigserial` 零命中 | ⬜ 待跑 |
  | 9 | 两处 `uniqueIndex:uq_moment_like` **换成** `unique:uq_moment_like` | 分组 A 的"索引对象恰好 3 个"（`uq_moment_like` 消失、多出两个 `uni_*` 约束）+ `\d` 里出现两条**单列** UNIQUE | ⬜ 待跑 |

  **任何一条注入后检查仍然"通过"，说明那条验收是假的**——先修验收，再改代码。读数连同日期提交。

> **注入 1 与 2 必须分开跑**：本表有**两个**外键列，只测其中一个会漏掉另一个的 `nextval`。
> 这是本表与 `moment_comments`（一个 `moment_id` 是唯一的"真外键列"风险点）的差别。

> **第 9 条是本支新加的，也是最"阴"的一条**（§1.2 实测）：它**建表不报错**，`\d` 里也确实有两个
> "UNIQUE"字样，只有比对**列数**（`(a)` 而不是 `(a,b)`）才看得出。它正好落在"把 DDL 的
> `CONSTRAINT uq_moment_like UNIQUE (user_id, moment_id)` 逐字翻译成 tag"这条最自然的思路上。
> 顺带：**别用"`\d` 里有没有 UNIQUE 字样"当判据**——本支的正确形态是 `UNIQUE, btree`（索引），
> 错误形态是 `UNIQUE CONSTRAINT`（约束），两者都有 `UNIQUE` 字样。判据是**索引对象 3 个 + 列清单是两列**。

> ⚠️ **诚实说明第 8 条的局限**：`bigserial` 的真正后果（本列长出 `nextval`）**本支观测不到**——
> 上游 `ai_moment.go` 已是对的，本支也改了它就违反范围。本支能证明的是"静态检查会报红"，
> 以及"当前代码下 `\ds` 是干净的"。**污染的可观测性在 ai-moment 分支就已接受为缺口。**

### 分组 D · 注释与流程（**本支**）

- [ ] `moment_like.go` 注释 **≤ 15 行**（§1.6）——定稿实测 **8 行**
- [ ] 无 DDL 逐字对照 / 长篇学习性解释 / 反例分析（内容已挪到 `.learn/`）
- [ ] `.learn/moment-like-model.md` 已写；`git status` 里**不出现**它（`.gitignore:75` 已覆盖）
- [ ] 未改 `router.go`、`go.mod`、他人的 model 文件；**未改 `ai_moment.go` / `moment_comment.go`**
- [ ] `git diff --name-only` 只含本支 4 个文件（含 `.learn/` 则为异常）
- [ ] commit 符合 `<type>(<scope>): <subject>`，scope 用 `moment`；PR 已开、至少 1 人 Approve

### 分组 E · 交接给下游的验收（**不在本支**，登记以免断档）

**给 `feature/backend-moment-api`**（写入路径与越权防线，§1.3 / §1.4 / §4）：

- [ ] **幂等（正面）**：同一用户对同一动态连点 5 次 → 每次 `200`，`likeCount` **停在 1**，
      库里 `moment_likes` **只有 1 行**
- [ ] **幂等（反面）**：上一步之后 `ai_moments.like_count` **等于** `COUNT(*)`（不变量，§1.3）
- [ ] **越权（必须 `4040`，且库里无新增行）**：A、B 两账号各有人设与动态
  - [ ] A 的 Token 打 `POST /moments/:id/like`，`:id` 用 **B 的动态** → `4040`
  - [ ] `moment_likes` **没有新增行**，且 `ai_moments.like_count` **没变**（自增没有被误触发）
  - [ ] 不存在的 id → `4040`，与"别人的动态"**同码同文案**
- [ ] **`RowsAffected = 0` 的二义消歧**（§4.2）：对 B 的动态**重复**点击（已点过 + 不是自己的）→ 仍然是 `4040`，
      **不是** `200`（把二义解成"已点过"就会越权放行）
- [ ] **不产生 `4030` / `4043`**：任意输入组合下只出现 `4040` / `4010` / `200`
- [ ] **注入缺陷 11**：把自增改成无条件执行 → 上面那条幂等正面用例必须报红
- [ ] **注入缺陷 12**：把两次写拆成两个独立事务（去掉 `tx`）→ 不变量检查必须报红
- [ ] **`liked` 的 `LEFT JOIN`**：`AND l.user_id = ?` 写在 **`ON` 里**，不是 `WHERE`（反例 7）
- [ ] `grep -i emotion backend/internal/dto/moment_dto.go` → **零命中**

**给 `moment_likes` 之后的表（`schedules` 等）**：

- [ ] 本次已达成的"朋友圈三序列干净"结论**不随新表回归**——新表落地后重跑分组 A 的 `\ds` 三条

---

## 6. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|------|------|------|------|
| 2026-09-18 | v1.2 | 分组 C **注入从 8 条加到 9 条**：新增第 9 条（`uniqueIndex:` → `unique:`），并写明它的判据是**列清单是几列**、不是"有没有 UNIQUE 字样" | 落码后跑对照实验（DryRun 两种 tag 各渲一遍）实测出：`unique:` 表达不了复合唯一，会**静默**建出两条单列约束且丢弃自定义名（§1.2 的 ⛔ 块）。它正好落在"把 DDL 逐字翻译成 tag"这条最自然的思路上，而 `\d` 一眼看不出——必须进注入清单 |
| 2026-09-18 | v1.1 | 代码落地后**对仓库本体重测全部读数**，纠正 §2.1 / §1.6 / 分组 D 的注释行数 **7 → 8**（加 ON CONFLICT 那句之后没复测）；§3 代码块同步为定稿（含 ON CONFLICT 禁令注释）；§1.2 补 ✅/❌ 的 `ON CONFLICT` 写法对照；§1.3 拆成**表 A「可见的错」/ 表 B「安静的错」**两张；§3.1 把"**结构探针**"定位写成明确结论；§2.1 补读法 4（注释避开禁用词的惯例**及其代价**）与分组 C 的**字段级** grep | 审查意见 ①②③；并把"落码后复测"当成纪律——行数漂移就是这么抓出来的 |
| 2026-09-18 | v1 | 创建 | 朋友圈点赞模型开工前的设计与验收基线；闭合 [ai-moment §9 分组 E 第 1 条](../ai-moment/spec.md) 的**后半**（`moment_comments` 支已闭合前半）。**§1.0 记录了"要求写 `persona_id`、DDL 无此列"的冲突与按 DDL 定案的依据**；§1.1 把要求 5 里那条无判别力的 `persona_id_seq` 验收改为 `user_id_seq` / `moment_id_seq`；§1.2 记下实测发现的 `uq_moment_like` 是**索引非约束**；§1.3 把 `like_count` 的事务边界与条件自增立为功能核心 | 设计阶段先证明验收项可触发，且把过程里发现的要求↔DDL 冲突留痕 |

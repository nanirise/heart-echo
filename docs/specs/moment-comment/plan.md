# plan · 朋友圈评论（Moment Comment）· 施工与审查计划

> 配套合同：[spec.md](./spec.md)。分歧时以 spec 为准。
> 本文件写"怎么做"和"怎么审"，不重复 spec 的约束条文。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-moment-comment-model` |
| 负责人 | 成员 3（`JMX2033`） |
| **本支范围** | **模型层：`moment_comment.go` + `migrate.go` 一行**（spec §0.1） |
| 前置 | spec 已人工审查通过（**未过审不动手写代码**） |
| 分支存活 | ≤ 3 天（[AGENTS §6](../../../AGENTS.md)）——本支只有 4 个文件，1 天内应收口 |

---

## 1. 目标与产出物

**一句话**：把 `moment_comments` 的表结构钉死，**并闭合 [ai-moment §9 分组 E](../ai-moment/spec.md) 留下的那条账**
——`moment_comments.moment_id` 不得长出数据库默认值。

**本支产出（4 个文件）**：

| # | 文件 | 动作 | 内容 |
|---|------|------|------|
| 1 | `internal/model/moment_comment.go` | 新增 | `MomentComment` struct + `TableName()`（spec §3 有完整代码，逐字对照） |
| 2 | `internal/model/migrate.go` | 修改 | 追加一行 `&MomentComment{}` |
| 3 | `internal/model/moment_comment_test.go` | 新增 | JSON 键集 + `null`/`0` 断言（spec §5 分组 C） |
| 4 | `.learn/moment-comment-model.md` | 新增（**不入库**） | 学习笔记：CHECK tag 语义、二选一模型取舍、反例展开 |

**交接物（本支只写设计，不写代码）**：

| 交付给 | 内容 | spec 位置 |
|--------|------|----------|
| `feature/backend-moment-api` | 越权防线的正确形态（**不能照抄 AGENTS §4.3**）、反例清单、`authorName` 的两条 `LEFT JOIN` | spec §1.3、§4 |
| `feature/backend-moment-job` | 评论作者必须同账号、`user_id` 留 `nil` 不留 `0` | spec §5 分组 E |
| `moment_likes` 分支 | 完整序列验收的最终收口（3 个序列） | spec §5 分组 E |

---

## 2. 施工步骤

> 原则：**建表 → 真库核对 → 再写代码**。表结构错了，下游全部返工。

| 步 | 动作 | 完成判据 |
|----|------|---------|
| 1 | 写 `model/moment_comment.go` | `go build ./...` 过 |
| 2 | `migrate.go` 追加一行 `&MomentComment{}` | `go run ./cmd/migrate` 建表成功 |
| 3 | **真库核对**（§3.2）——含 spec §1.1 的头号验收项 | spec §5 分组 A 全绿 |
| 4 | 手工验 CHECK 在拦人（§3.2 末段） | 分组 B 全绿 |
| 5 | 写 `moment_comment_test.go`（§3.3） | `go test ./internal/model/` 过 |
| 6 | **反向验证**（§5.4）：6 条注入逐一确认报红 | 6/6 报红，读数记回 §5.4 |
| 7 | 写 `.learn/moment-comment-model.md`（§3.4） | 长注释已从 `.go` 挪走 |
| 8 | 自测 + PR（§7） | `go build ./... && go vet ./... && go test ./...` 过 |

**建议的 commit 切分**（[AGENTS §6](../../../AGENTS.md)「产出即提交」）：

```
feat(moment): add moment comment model
feat(moment): register moment comment in automigrate
test(moment): assert moment comment json keys
docs(moment): add spec and plan for moment-comment
```

### ⛔ 不在本支的范围

| 事项 | 去向 | 触发条件 |
|------|------|---------|
| `moment_likes` model | 独立分支 | 与本支并行即可 |
| `dto` / `repository` / `service` / `handler` | `feature/backend-moment-api` | 本支合并后 |
| 定时任务写 AI 评论 | `feature/backend-moment-job` | [ai-moment §8.2](../ai-moment/spec.md) 的两条待确认落定后 |
| `router.go` 的一行 | **成员 1** | API 分支完成后通知 |

> **为什么不让本支顺手把 API 也写了**：本表的越权防线形态与全项目其他表**都不一样**（spec §1.3），
> 它值得一个**独立的、有完整运行时验收的**PR 来承载（spec §5 分组 E 的跨账号用例）。
> 混在 model 分支里，那组用例没有地方跑。

---

## 3. 本支的实现要点

### 3.1 写 `moment_comment.go`（步骤 1）

代码在 **spec §3**——完整 struct，**逐字对照**，不要临场发挥。

**四件最容易写错的事**（只在这里重复一次）：

```go
// ① 外键列不带 autoIncrement（spec §1.1 ②）。带上之后实测渲染成 "moment_id" bigserial —— 默认值就来了
MomentID uint64 `gorm:"column:moment_id;type:bigint;not null;index:idx_comments_moment,priority:1" json:"momentId"`

// ② 两个作者列都必须是 *uint64（spec §1.2）。值类型 → 序列化成 0（违约）；且该列留零值也会被写进
//    INSERT，于是恒为 0（非空）—— 只写错一列时只炸"那一类评论"，列表照常有数据，最难发现
PersonaID *uint64 `gorm:"column:persona_id;type:bigint;check:chk_comment_author,..." json:"personaId"`
UserID    *uint64 `gorm:"column:user_id;type:bigint" json:"userId"`

// ③ 索引不带 sort:desc —— 本表 DDL 是 (moment_id, created_at)，不是 ... DESC（spec §2 第三条纪律）
CreatedAt time.Time `gorm:"...;index:idx_comments_moment,priority:2" json:"createdAt"`

// ④ 三条关联字段都要有，缺一条就没有那个 ON DELETE CASCADE（spec §3）
```

**注释预算：≤ 15 行**（spec §1.4），三类各覆盖若干处即可。
`default:now();autoCreateTime` 为什么要两个都写、`priority` 的含义、CHECK tag 为什么按约束名合并——**都不写**，进 `.learn/`。

### 3.2 真库核对（步骤 3 + 4，**别跳**）

> ⚠️ `cmd/migrate` 读的是 `SCHEMA_CHECK_DSN`，**不随 `.env` 自动生效**，必须自行 export（见 `cmd/migrate/main.go` 顶部注释）。

```bash
docker compose -f deploy/docker-compose.dev.yml up -d
export SCHEMA_CHECK_DSN='postgres://heart_echo:<password>@localhost:5432/heart_echo?sslmode=disable'
cd backend && go run ./cmd/migrate
```

**逐条确认**（spec §5 分组 A）：

| 看什么 | 期望 | 抓什么错 |
|--------|------|---------|
| 列 | **恰好 6 列**，无 `author_name` | 派生字段被写进 model |
| ▲ **`moment_id` 的 Default** | **空** | **spec §1.1，本支头号验收项** |
| `persona_id` / `user_id` 的 Default | 空 | spec §1.1 的另两个检查点 |
| `\ds` | 只有 `moment_comments_id_seq` | 外键列误加 `autoIncrement` |
| 外键 | 三条 `→ ai_moments/personas/users` 且 `ON DELETE CASCADE` | 漏写关联字段 |
| 索引 | `idx_comments_moment (moment_id, created_at)` | priority / `sort:desc` 写错 |
| CHECK | `chk_comment_author` 在，条件文本与 DDL 一致 | CHECK 整行丢失 |

**头号验收项的两条命令**（spec §1.1，逐条给读数）：

```bash
# ① 人读版：应 0 结果
psql "$SCHEMA_CHECK_DSN" -c '\ds' | grep moment_comments_moment_id_seq

# ② 机器版：应 1 / 0 / 0 / t 四项全对
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM pg_class WHERE relkind='S' AND relname LIKE 'moment_comment%';"                        # 期望 1
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM pg_class WHERE relkind='S' AND relname='moment_comments_moment_id_seq';"                # 期望 0
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM pg_class WHERE relkind='S' AND relname IN ('moment_comments_persona_id_seq','moment_comments_user_id_seq');"  # 期望 0
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT column_default IS NULL FROM information_schema.columns WHERE table_name='moment_comments' AND column_name='moment_id';"      # 期望 t
```

> **第 ② 组必须全跑**。只跑 grep 那一条会放过"把 `ID` 的 `autoIncrement` 也删了"这种改法——
> 它同样没有 `moment_id_seq`，但 `moment_comments_id_seq` 也没了，本表再也插不进数据（spec §1.1 的反面）。

**再手工验 CHECK 在拦人（步骤 4，spec §5 分组 B）**——不能只看 `\d` 一眼就算过：

```sql
-- 先造一个真实动态，拿到 <moment_id>
INSERT INTO moment_comments (moment_id, persona_id, user_id, content) VALUES (<moment_id>, 1, 1, 'both');      -- 必须报错 23514
INSERT INTO moment_comments (moment_id, persona_id, user_id, content) VALUES (<moment_id>, NULL, NULL, 'none'); -- 必须报错 23514
INSERT INTO moment_comments (moment_id, persona_id, user_id, content) VALUES (<moment_id>, 1, NULL, 'ai');      -- 必须成功
INSERT INTO moment_comments (moment_id, persona_id, user_id, content) VALUES (<moment_id>, NULL, 1, 'human');   -- 必须成功
```

验完删掉这两行测试数据。

**级联的正反两面**（分组 A 最后两条）：先造 A、B 两条人设各带动态与评论 → 删 A 的人设 →
A 的动态下评论清零，而 **B 的评论仍在**。只验"清零"会被"把全表清了"蒙混过关。

### 3.3 序列化测试（步骤 5）

沿用 [ai-moment §9 分组 C](../ai-moment/spec.md) 的「键集**恰好**相等」模式（`internal/model` 下的第二个测试文件）。

**本表比 `ai_moments` 多一个必须验的东西：`null` 与 `0` 的区别**——那是"二选一作者模型"的全部意义，
也是值类型误写的唯一行为层探针。

两个测试函数，职责分开：

```go
// ① 键集恰好 6 个。契约 §8 的 Comment 没有 authorName，表里也没有 author_name 列（spec §3）。
//    用零值直接 marshal 即可——这条验的是"有没有多出字段"。
func TestMomentCommentJSONKeys(t *testing.T) { ... }

// ② 非作者的一侧必须是 null 而不是 0 —— 值类型会把这里变成 0（spec §1.2）。
//    AI 评论断言 m["userId"] == nil，用户评论断言 m["personaId"] == nil。
func TestMomentCommentAuthorNullNotZero(t *testing.T) { ... }
```

**关键**：断言的是**键集相等**（不是"包含"），多一个字段就报红——正好抓住 `authorName` 混进 model 的陷阱。

**`null` 那条断言要用 JSON 往返构造值**（不是直接写 struct 字面量），这是为了让**断言本身**成为探针：

```go
// 直接字面量：PersonaID: &pid —— 注入缺陷 1（改成值类型）时测试文件先编译不过，
// 报的是 compile error，验不到"断言会不会红"。
var c MomentComment
json.Unmarshal([]byte(`{"personaId":2,"userId":null,...}`), &c)   // 值类型下 null 被忽略 → UserID 留 0
```

JSON 往返构造在值类型下**照样编译、照样跑**，于是报红的一定是 `m["userId"] != nil` 那条断言。
**注入 1 的读数要按这个口径记**（spec §5 分组 C）。

### 3.4 `.learn/moment-comment-model.md`（步骤 7）

从 `.go` 里**挪出来**的长内容，全部落这里：

- DDL 逐列 ↔ GORM tag ↔ `\d` 渲染结果 的对照（含 spec §2.1 那份实测 DDL 的推导过程）；
- `check:` tag 的三段切分规则（`;` → `,` → `:`）与"为什么按约束名做 map key"；
- "二选一作者"模型的取舍：为什么用两列 + CHECK 而不是两张表 / 一个 `author_type` 列；
- 为什么本表的 `user_id` 不能当越权防线的闸门（spec §1.3 的完整推演与两个静默错法）；
- spec §4.3 八条反例逐条展开。

`.learn/` 在 `.gitignore:75`，**不入库**。

---

## 4. 交接速查（下游要执行的，本支不落）

不复制内容，只给指针——**单一事实来源在 spec**：

| 下游任务 | 抄哪里 | 最容易翻车的点 |
|---------|--------|--------------|
| 评论的两条 SQL | [ai-moment §4.4⑤](../ai-moment/spec.md)（已定稿） | 归属条件是 `c.moment_id = ? AND p.user_id = ?`，**不是** `c.user_id = ?`（spec §1.3） |
| 评论的仓储方法 | [ai-moment §4.5](../ai-moment/spec.md) | `ListCommentsByUser` / `CountCommentsByUser`，名字里必须有 `User` |
| `CommentItem` DTO | 契约 §8 | `authorName` 是派生字段，**只能**在 DTO；`grep -i emotion` 零命中 |
| AI 评论的写入 | ai-moment §6 第 5 步 | 作者必须**同账号**的其他人设；`user_id` 留 `nil` 不留 `0` |

---

## 5. 我手动审查 AI 代码的计划

> 前提：代码由 AI 生成，我**逐行审**，不"看着像对就过"。审查的是 spec 的落实，不是代码风格。

### 5.1 审查顺序

`moment_comment.go` → `migrate.go` → `moment_comment_test.go` → 真库 `\d` / `\ds`。
**先审 tag（静态），再审真库（运行时）**——两者互为交叉验证：tag 对而 `\d` 不对，说明 tag 语义理解错了。

### 5.2 逐文件审查清单

| 文件 | 只看这几件事 |
|------|-------------|
| `moment_comment.go` | ① `ID` 是否 `type:bigint;primaryKey;autoIncrement`，全文件**无 `bigserial`**；② `gorm:"..."` 内 `autoIncrement` **恰好 1 处**；③ `PersonaID` / `UserID` 是否**都是 `*uint64`**；④ 是否**没有** `AuthorName` / `author_name` / `emotion`；⑤ CHECK 是否带 `chk_comment_author` 名且**只挂一处**；⑥ `idx_comments_moment` 是否**没有** `sort:desc`；⑦ 三条关联字段是否都在且带 `OnDelete:CASCADE`；⑧ 注释 ≤ 15 行、无 DDL 对照 |
| `migrate.go` | 是否只加了一行 `&MomentComment{}`，且在 `&AIMoment{}` 之后；没动其他行 |
| `moment_comment_test.go` | 断言的是**键集相等**还是"包含"？后者是假测试。**有没有** `null`/`0` 那条断言？ |
| `.learn/` | 是否**未被** `git status` 列出 |
| `git diff --name-only` | 是否只有 4 个文件（含 `.learn/` 则为异常） |

### 5.3 高危点排序（**只审三条的话，审这三条**）

1. **`moment_id` 的 tag**——`autoIncrement` 一旦误加，本表就有了数据库默认值，漏传静默挂错动态。
   这是**全项目第四次机会**（[ai-moment §1.1](../ai-moment/spec.md) 记着前三次），且**本支就是来闭合它的**。
2. **两个作者列是不是指针**——契约违反（`0` vs `null`）与"那一类评论写不进库"，一次踩两个；
   且只写错一列时**列表看不出问题**（spec §1.2 的两种后果）。
3. **CHECK 约束在不在、名字对不对**——CHECK 是"翻译 Struct"时最容易**整行丢掉**的东西，
   丢了不影响编译、不影响跑通，只在数据脏了以后才暴露。

### 5.4 反向验证记录（**注入缺陷，确认检查会报红**）

只勾"通过"不够——**要证明检查在代码变坏时真的会失败**，否则可能是"检查本身失效"。

**本支可跑的 6 条**（spec §5 分组 C 的表）：

| # | 注入的缺陷 | 期望哪条报红 | 实际读数 |
|---|-----------|-------------|---------|
| 1 | `PersonaID` 改成值类型 `uint64` | 分组 C 的 `null` 断言 | ⬜ 待跑 |
| 2 | 给 struct 加 `AuthorName string \`json:"authorName"\`` | 分组 C 的键集断言 | ⬜ 待跑 |
| 3 | `MomentID` 的 tag 加 `autoIncrement` | 分组 A 的 `autoIncrement` 恰好 1 行 | ⬜ 待跑 |
| 4 | 删掉 `check:chk_comment_author,...` | 分组 B 的 `\d` 里约束消失 | ⬜ 待跑 |
| 5 | `ID` 的 tag 改成 `type:bigserial` | 分组 A 的 `bigserial` 零命中 | ⬜ 待跑 |
| 6 | 删掉 `Moment AIMoment` 关联字段 | 分组 A 的 `\d` 里外键消失 | ⬜ 待跑 |

**下游分支补跑的 1 条**（本支无 SQL，跑不了）：

| # | 注入的缺陷 | 期望报红 | 归属 |
|---|-----------|---------|------|
| 7 | 删掉 `ListCommentsByUser` 的归属条件 | 分组 E「A 读 B 动态的评论 → `4040`」 | `backend-moment-api` |

**任何一条注入后检查仍然"通过"，说明那条验收是假的**——先修验收，再改代码。读数连同日期提交。

> ⚠️ **honest 说明**：spec §2.1 的读数表是拿**最终的 §3 代码**跑出来的（编译 + DryRun + 全部静态检查），
> 其中注入 1 与 3 所依赖的 tag 语义也一并确认了。但**注入一条都不能省**：
> 注入验的是反方向的问题——"代码被改坏时，这条检查会不会报红"。静态度量（grep / 编译）全部放行、
> 而检查本身失效，正是"假验收"的典型形态。只有把代码**故意改坏**才测得出来。

### 5.5 拒绝标准（出现任一条就退回重写，不做"小修小补"）

1. `moment_comment.go` 出现 `bigserial`；
2. `gorm:"..."` 内 `autoIncrement` 出现 2 次以上（外键列误加）；
3. `PersonaID` / `UserID` 有一处不是 `*uint64`；
4. `model.MomentComment` 出现 `AuthorName` / 任何 `emotion` 字样；
5. 缺 `chk_comment_author`，或约束名写成派生的 `chk_moment_comments_*`；
6. 缺任一关联字段（对应的外键建不出来）；
7. `.go` 里出现 DDL 逐字对照 / 长篇学习性解释 / 反例分析（spec §1.4）；
8. 改了 `router.go`、`go.mod`、`ai_moment.go` 或他人的 model 文件；
9. 序列化测试断言的是"包含"而不是"键集相等"，或**没有** `null`/`0` 那条断言；
10. **`\d moment_comments` 的 `moment_id` 有 Default**（spec §1.1，头号验收项），
    或 `\ds` 多出任何序列。**只看列名不看 Default，等于没查。**

---

## 6. 风险与对策

| 风险 | 概率 | 对策 |
|------|:---:|------|
| **`moment_id` 带上了 Default（ai-moment 遗留项没闭合）** | 中 | §3.2 的四条命令 + 分组 A；失败时的处置与"别在本支改 `ai_moment.go`"已写在 spec §1.1 |
| 作者列写成值类型，编译无提示 | 中 | §3.3 的 `null`/`0` 断言；§5.4 注入 1 |
| CHECK 整行丢失 | 中 | §3.2 手工 INSERT 两正两反；§5.4 注入 4 |
| tag 写错但 `go build` 通过 | 中 | 步骤 3 真库核对，**每改一次 tag 重跑**。**已在 ai-moment 分支实际发生过**（`type:int` → `bigint`） |
| 照抄 `ai_moments` 的 `index` tag（带 `sort:desc`） | 中 | spec §2 第三条纪律 + §3.1 ④；正文反复强调"逐列看 DDL，不照抄隔壁文件" |
| 范围蔓延（顺手把 repo / DTO 写了） | 中 | §2 的交接表；`git diff --name-only` 必须是 4 个文件 |
| 与既有 model 注释风格不一致，评审反复 | 中 | spec §1.4 已决策：新文件从简，**不以风格不一致为退回理由** |
| 下游把 `c.user_id` 当越权闸门（本表独有陷阱） | 中 | spec §1.3 单独立节 + §4.3 反例 1/2；交接时**当面说一遍** |

---

## 7. PR 描述草稿

```markdown
# feat(moment): add moment comment model and migration

## 范围（模型层，只有 4 个文件）
- `internal/model/moment_comment.go`：`MomentComment` struct
- `internal/model/migrate.go`：追加一行 `&MomentComment{}`
- `internal/model/moment_comment_test.go`：JSON 键集 + null/0 断言
- `.learn/moment-comment-model.md`（不入库）

## 明确不在本 PR 内（已交接，见 spec §0.2）
- `moment_likes` model —— 独立分支
- dto / repo / service / handler / 路由 —— `feature/backend-moment-api`
- 定时任务写 AI 评论 —— `feature/backend-moment-job`

## 实现要点
- `moment_id` 不带 `autoIncrement`：实测有/无这一个 tag，渲染结果是
  `bigint NOT NULL` 与 `bigserial NOT NULL` 的区别 —— 后者会给本列长出数据库默认值
- 两个作者列是指针：契约要求 null，值类型会变 0，且那一类评论会被 CHECK 拒绝
- `chk_comment_author` 照抄权威 DDL（命名 CHECK 跨两列）
- `authorName` 不进 model（派生字段，加了会建出真列）

## 验证
- [ ] `go build ./... && go vet ./... && go test ./...`
- [ ] `\d moment_comments` 恰好 6 列；**`moment_id` 的 Default 为空**
- [ ] `\ds` 只有 `moment_comments_id_seq`（没有 `moment_comments_moment_id_seq`）
- [ ] 三条外键均为 `ON DELETE CASCADE`；删人设后评论清零且他人评论仍在
- [ ] `chk_comment_author` 在，且两侧非空/两侧为空都被拒绝
- [ ] spec §5.4 的 6 条注入全部报红（附读数）

## 顺带闭合
- [ ] closes ai-moment spec §9 分组 E 第 1 条（`moment_id` 的 Default 遗留未闭合项）
```

---

## 8. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|------|------|------|------|
| 2026-09-18 | v1.1 | §3.3 补「`null` 断言要用 JSON 往返构造值」及其理由——字面量构造会在注入缺陷 1 时先编译失败，验不到断言本身；同步更新 spec §5 分组 C 的读数口径 | 审查确认注入验的是"检查会不会报红"，构造方式决定了红在哪里 |
| 2026-09-18 | v1 | 创建 | 与 spec v1.2 配套的施工步骤、真库核对命令、人工审查计划 |

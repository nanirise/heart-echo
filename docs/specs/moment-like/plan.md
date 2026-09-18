# plan · 朋友圈点赞（Moment Like）· 施工与审查计划

> 配套合同：[spec.md](./spec.md)。分歧时以 spec 为准。
> 本文件写"怎么做"和"怎么审"，不重复 spec 的约束条文。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-moment-like-model` |
| 负责人 | 成员 3（`JMX2033`） |
| **本支范围** | **模型层：`moment_like.go` + `migrate.go` 一行**（spec §0.1） |
| 前置 | spec 已人工审查通过（**未过审不动手写代码**）；`ai-moment` / `moment-comment` 两支均已合并 |
| 分支存活 | ≤ 3 天（[AGENTS §6](../../../AGENTS.md)）——本支只有 4 个文件，1 天内应收口 |

---

## 1. 目标与产出物

**一句话**：把 `moment_likes` 的表结构钉死——**幂等靠数据库的唯一约束，不靠 Go 的 `if`**——
并闭合 [ai-moment §9 分组 E 第 1 条](../ai-moment/spec.md) 遗留项的**后半**，
之后"朋友圈三序列干净"这条结论**首次可以完整跑通**。

**本支产出（4 个文件）**：

| # | 文件 | 动作 | 内容 |
|---|------|------|------|
| 1 | `internal/model/moment_like.go` | 新增 | `MomentLike` struct + `TableName()`（spec §3 有完整代码，逐字对照） |
| 2 | `internal/model/migrate.go` | 修改 | 追加一行 `&MomentLike{}` |
| 3 | `internal/model/moment_like_test.go` | 新增 | JSON 键集断言（spec §5 分组 C） |
| 4 | `.learn/moment-like-model.md` | 新增（**不入库**） | 学习笔记：幂等约束形态、事务边界、反例展开 |

**交接物（本支只写设计，不写代码）**：

| 交付给 | 内容 | spec 位置 |
|--------|------|----------|
| `feature/backend-moment-api` | 幂等三连与事务边界、`RowsAffected = 0` 的二义消歧、`liked` 的 `LEFT JOIN` 写法 | spec §1.3 / §1.4 / §4 |
| 下游所有分支 | **`ON CONFLICT` 只能用列清单形式**（`uq_moment_like` 是索引不是约束） | spec §1.2 |
| `feature/backend-moment-job` | 计数对账（若要做成定时任务） | spec §1.3 |

---

## 2. 施工步骤

> 原则：**建表 → 真库核对 → 再写代码**。表结构错了，下游全部返工。

| 步 | 动作 | 完成判据 |
|----|------|---------|
| 1 | 写 `model/moment_like.go` | `go build ./...` 过 |
| 2 | `migrate.go` 追加一行 `&MomentLike{}` | `go run ./cmd/migrate` 建表成功 |
| 3 | **真库核对**（§3.2）——含 spec §1.1 的两条头号验收项 | spec §5 分组 A 全绿 |
| 4 | 手工验唯一约束在拦人（§3.2 末段） | 分组 B 全绿 |
| 5 | 写 `moment_like_test.go`（§3.3） | `go test ./internal/model/` 过 |
| 6 | **反向验证**（§5.4）：9 条注入逐一确认报红 | 9/9 报红，读数记回 §5.4 |
| 7 | 写 `.learn/moment-like-model.md`（§3.4） | 长注释已从 `.go` 挪走 |
| 8 | 自测 + PR（§7） | `go build ./... && go vet ./... && go test ./...` 过 |

**建议的 commit 切分**（[AGENTS §6](../../../AGENTS.md)「产出即提交」）：

```
feat(moment): add moment like model
feat(moment): register moment like in automigrate
test(moment): assert moment like json keys
docs(moment): add spec and plan for moment-like
```

### ⛔ 不在本支的范围

| 事项 | 去向 | 触发条件 |
|------|------|---------|
| 点赞 SQL / 事务 / 自增 | `feature/backend-moment-api` | 本支合并后 |
| `dto` / `service` / `handler` | `feature/backend-moment-api` | 同上 |
| 计数对账任务 | `feature/backend-moment-job` | 若决定做成定时任务 |
| `router.go` 的一行 | **成员 1** | API 分支完成后通知 |

> **为什么不让本支顺手把 API 也写了**：本支的核心风险在**跨表事务**（`moment_likes` + `ai_moments` 两次写），
> 它值得一个**独立的、有运行时验收的** PR 来承载（spec §5 分组 E 的幂等与越权用例）。
> 混在 model 分支里，那组用例没有地方跑。

---

## 3. 本支的实现要点

### 3.1 写 `moment_like.go`（步骤 1）

代码在 **spec §3**——完整 struct，**逐字对照**，不要临场发挥。

**四个最容易写错的事**（只在这里重复一次）：

```go
// ① 两个外键列都不带 autoIncrement（spec §1.1 ②）。本表有【两条】外键列，两条都要查
UserID   uint64 `gorm:"column:user_id;type:bigint;not null;uniqueIndex:uq_moment_like,priority:1;index:idx_likes_user" json:"userId"`
MomentID uint64 `gorm:"column:moment_id;type:bigint;not null;uniqueIndex:uq_moment_like,priority:2;index:idx_likes_moment" json:"momentId"`

// ② 复合唯一索引必须【挂两列】，priority 决定列序（user_id 在前）。只挂一列 = 唯一性残缺（spec §1.2）
// ③ 没有 CHECK 可抄 —— DDL 上一条 CHECK 都没有，唯一的具名约束是 UNIQUE（spec §1.5）
// ④ 两条关联字段都要有，缺一条就没有那个 ON DELETE CASCADE
```

**注释预算：≤ 15 行**（spec §1.6），三类各覆盖若干处即可。
`priority` 的含义、`uniqueIndex` 为什么渲染成索引、`like_count` 为什么不在本表——**都不写**，进 `.learn/`。

> ⚠️ **不要从 `moment_comment.go` 照抄任何东西**。那张表的 `persona_id`、`check:` tag、指针类型、
> `authorName` 在本表**全部不存在**（spec §1.0）。本表是两张姊妹表里**更简单**的那张——
> 简单到照抄反而会写错。

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
| 列 | **恰好 4 列**，无 `like_count` | 派生字段被写进 model |
| ▲ **`user_id` 的 Default** | **空** | **spec §1.1，头号验收项之一** |
| ▲ **`moment_id` 的 Default** | **空** | **另一个头号验收项** |
| `\ds` | 只有 `moment_likes_id_seq` | 外键列误加 `autoIncrement` |
| 外键 | 两条 `→ users / ai_moments` 且 `ON DELETE CASCADE` | 漏写关联字段 |
| 索引 | **恰好 3 个**：`uq_moment_like(user_id,moment_id)` / `idx_likes_moment` / `idx_likes_user` | priority 写反、漏挂一列 |

**头号验收项的命令**（spec §1.1，逐条给读数）：

```bash
# ① 人读版：应 0 结果
psql "$SCHEMA_CHECK_DSN" -c '\ds' | grep -E 'moment_likes_(user_id|moment_id)_seq'

# ② 机器版：应 1 / 0 / 0 / t / t / 3 六项全对
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM pg_class WHERE relkind='S' AND relname LIKE 'moment_like%';"                         # 期望 1
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM pg_class WHERE relkind='S' AND relname IN ('moment_likes_user_id_seq','moment_likes_moment_id_seq');"  # 期望 0
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT column_default IS NULL FROM information_schema.columns WHERE table_name='moment_likes' AND column_name='user_id';"    # 期望 t
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT column_default IS NULL FROM information_schema.columns WHERE table_name='moment_likes' AND column_name='moment_id';"  # 期望 t
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM pg_indexes WHERE tablename='moment_likes';"                                            # 期望 3

# ③ 完整收口（本支才第一次跑得动）
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT relname FROM pg_class WHERE relkind='S' AND relname LIKE '%moment%' ORDER BY 1;"   # 期望恰好 3 行
```

> **第 ② 组必须全跑**。只跑 grep 那一条会放过"把 `ID` 的 `autoIncrement` 也删了"这种改法——
> 它同样没有外键序列，但 `moment_likes_id_seq` 也没了，本表再也插不进数据（spec §1.1 的反面）。
>
> ⚠️ **别去搜 `moment_likes_persona_id_seq`**（原要求里写的那条）。本表没有 `persona_id` 列（spec §1.0），
> 这个名字**恒不存在**，搜它等于没搜——spec §1.1 已把这条验收改正。

**再手工验唯一约束在拦人（步骤 4，spec §5 分组 B）**——不能只看 `\d` 一眼就算过：

```sql
-- 先造两条动态（各自属于不同人设）、两个账号，拿到 <moment_a> / <moment_b> / <user_a> / <user_b>
INSERT INTO moment_likes (user_id, moment_id) VALUES (<user_a>, <moment_a>);   -- 成功
INSERT INTO moment_likes (user_id, moment_id) VALUES (<user_a>, <moment_a>);   -- 必须报错 23505
INSERT INTO moment_likes (user_id, moment_id) VALUES (<user_a>, <moment_b>);   -- 必须成功（不是"一人只能赞一次"）
INSERT INTO moment_likes (user_id, moment_id) VALUES (<user_b>, <moment_a>);   -- 必须成功（不是"一条只能被赞一次"）

-- ON CONFLICT 的两种写法（spec §1.2 的物证）
INSERT INTO moment_likes (user_id, moment_id) VALUES (<user_a>, <moment_a>) ON CONFLICT (user_id, moment_id) DO NOTHING;   -- 成功，0 行
INSERT INTO moment_likes (user_id, moment_id) VALUES (<user_a>, <moment_a>) ON CONFLICT ON CONSTRAINT uq_moment_like DO NOTHING;  -- 必须报错
```

**第二条 `ON CONFLICT` 报错是预期的**——`uq_moment_like` 是索引不是约束（spec §1.2 实测）。
把它记进验收：**报错说明形态与 spec 一致，不报错才要回头查。**

验完删掉这些测试数据。

**级联的正反两面**（分组 A 最后两条）：先造 A、B 两条人设各带动态与点赞 → 删 A 的人设 →
A 的动态下点赞清零，而 **B 的点赞仍在**。只验"清零"会被"把全表清了"蒙混过关。

### 3.3 序列化测试（步骤 5）

`internal/model` 下的第三个测试文件，沿用 [ai-moment §9 分组 C](../ai-moment/spec.md) 的「键集**恰好**相等」模式。

**但理由与前面两张表不同**（spec §3.1）：本表**没有任何端点返回它的行**，
契约里也没有 `Like` 实体。所以这条测试**不是校验契约**，它是一道**结构探针**——
证明 `likeCount` / `liked` / `personaName` 这些派生字段**没有**混进 model（混进去就会被 AutoMigrate 建出真列）。

```go
// TestMomentLikeJSONKeys 锁死 MomentLike 对外的键集（恰好 4 个）。
//
// 本表没有契约实体，所以这不是在校验契约，而是一道结构探针：
// likeCount / liked / personaId 都是【别的表或派生】的东西，
// 一旦混进 model 就会被 AutoMigrate 在本表建成真列（spec §3 / §3.1）。
func TestMomentLikeJSONKeys(t *testing.T) {
	// 零值 marshal → 键集恰好 {id, userId, momentId, createdAt}
}
```

**关键**：断言的是**键集相等**（不是"包含"），多一个字段就报红。

### 3.4 `.learn/moment-like-model.md`（步骤 7）

从 `.go` 里**挪出来**的长内容，全部落这里：

- 「一次点赞写两张表」的完整推演：为什么事务边界不可省，为什么错法 ② 是**不可自愈**的；
- `uniqueIndex` 为什么渲染成 `CREATE UNIQUE INDEX` 而不是 `CONSTRAINT`，以及 `ON CONFLICT` 两种写法的分界；
- 为什么复合唯一索引必须挂两列，只挂一列的两个分支各是什么灾难；
- 为什么本表的 `user_id` 与 `moment_comments` 的 `user_id` **不能互相参考**（一个恒非空、一个一半是 NULL）；
- spec §4.3 十条反例逐条展开。

`.learn/` 在 `.gitignore:75`，**不入库**。

---

## 4. 交接速查（下游要执行的，本支不落）

不复制内容，只给指针——**单一事实来源在 spec / ai-moment**：

| 下游任务 | 抄哪里 | 最容易翻车的点 |
|---------|--------|--------------|
| 点赞的三条 SQL | [ai-moment §4.4③④](../ai-moment/spec.md)（已定稿） | `ON CONFLICT` 只能用**列清单**；自增只能**有条件**；两次写必须**同事务** |
| `RowsAffected = 0` 的分流 | [ai-moment §4.4③ 返回值表](../ai-moment/spec.md) | `0` 是**二义**的，消歧要靠"再查归属"，不是"再查点赞存在性" |
| 仓储方法命名 | [ai-moment §4.5](../ai-moment/spec.md) | 名字里要有 `User` / `Owned` |
| `liked` 的 `LEFT JOIN` | [ai-moment §4.4①](../ai-moment/spec.md) | `AND l.user_id = ?` 写在 `ON` 里，写进 `WHERE` 会把没点赞的行过滤掉 |
| `MomentItem` DTO | 契约 §8 | `liked` / `likeCount` / `personaName` 都是派生字段，**只能**在 DTO |

---

## 5. 我手动审查 AI 代码的计划

> 前提：代码由 AI 生成，我**逐行审**，不"看着像对就过"。审查的是 spec 的落实，不是代码风格。

### 5.1 审查顺序

`moment_like.go` → `migrate.go` → `moment_like_test.go` → 真库 `\d` / `\ds`。
**先审 tag（静态），再审真库（运行时）**——两者互为交叉验证：tag 对而 `\d` 不对，说明 tag 语义理解错了。

### 5.2 逐文件审查清单

| 文件 | 只看这几件事 |
|------|-------------|
| `moment_like.go` | ① `ID` 是否 `type:bigint;primaryKey;autoIncrement`，全文件**无 `bigserial`**；② `gorm:"..."` 内 `autoIncrement` **恰好 1 处**（**两条外键列都要看**）；③ `uniqueIndex:uq_moment_like` 是否**恰好 2 处**且 priority 是 1/2；④ 是否**没有** `likeCount` / `liked` / `personaId` / `personaName` / `emotion`；⑤ 是否**没有**任何 `check:`（§1.5）；⑥ 两条关联字段是否都在且带 `OnDelete:CASCADE`；⑦ 注释 ≤ 15 行、无 DDL 对照 |
| `migrate.go` | 是否只加了一行 `&MomentLike{}`，且在 `&MomentComment{}` 之后；**没动其他行** |
| `moment_like_test.go` | 断言的是**键集相等**还是"包含"？后者是假测试。键集是否为 **4 个**？ |
| **字段级静态检查**（§5.4 之外，不依赖运行） | `grep -cE '^[[:space:]]+(LikeCount\|Liked\|PersonaName\|PersonaID)[[:space:]]' internal/model/moment_like.go` 是否 **0**？它匹配的是 struct 字段声明行，注释整行以 `//` 开头不会命中——**与键集断言是两道独立的闸门**（spec §5 分组 C） |
| `.learn/` | 是否**未被** `git status` 列出 |
| `git diff --name-only` | 是否只有 4 个文件（含 `.learn/` 则为异常） |

### 5.3 高危点排序（**只审三条的话，审这三条**）

1. **两条外键列的 Default**——本表**有两条**，只查 `moment_id` 会漏掉 `user_id`。
   这是 ai-moment 遗留项的最终收口，**本支就是来闭合它的**。
2. **复合唯一索引是不是挂了两列、priority 对不对**——只挂一列时唯一性**残缺但看起来正常**
   （有一半的幂等还在工作），最难发现；priority 写反则 `\d` 与 DDL 不符。
3. **是不是从 `moment_comment.go` 抄来了 `persona_id` / `check:` / 指针**（spec §1.0）——
   本表**没有** `persona_id`，抄过来会建出 DDL 上不存在的列，且 `\d` 一比对就露馅。

### 5.4 反向验证记录（**注入缺陷，确认检查会报红**）

只勾"通过"不够——**要证明检查在代码变坏时真的会失败**，否则可能是"检查本身失效"。

**本支可跑的 9 条**（spec §5 分组 C 的表）：

| # | 注入的缺陷 | 期望哪条报红 | 实际读数 |
|---|-----------|-------------|---------|
| 1 | `MomentID` 的 tag 加 `autoIncrement` | 分组 A 的 `autoIncrement` 恰好 1 行 + `\ds` | ⬜ 待跑 |
| 2 | `UserID` 的 tag 加 `autoIncrement` | 同上（**必须与 1 分开跑**） | ⬜ 待跑 |
| 3 | 删掉 `uniqueIndex:uq_moment_like,...` | 分组 A 的 `uniqueIndex` 恰好 2 + 分组 B | ⬜ 待跑 |
| 4 | 两处 `priority` 对调 | 分组 A 的索引列序 | ⬜ 待跑 |
| 5 | 给 struct 加 `LikeCount int` | 分组 C 的**字段级 grep** + 键集 + 分组 A 的"恰好 4 列" | ⬜ 待跑 |
| 6 | 删掉 `Moment AIMoment` 关联字段 | 分组 A 的外键消失 | ⬜ 待跑 |
| 7 | 删掉 `idx_likes_user` 的 tag | 分组 A 的"索引对象恰好 3 个" | ⬜ 待跑 |
| 8 | `ID` 的 tag 改成 `type:bigserial` | 分组 A 的 `bigserial` 零命中 | ⬜ 待跑 |
| 9 | 两处 `uniqueIndex:uq_moment_like` **换成** `unique:uq_moment_like` | 分组 A 的"索引对象恰好 3 个" + `\d` 里出现两条**单列** UNIQUE（`uq_moment_like` 本身消失，变成 `uni_moment_likes_user_id` / `uni_moment_likes_moment_id`） | ⬜ 待跑 |

**下游分支补跑的 4 条**（本支无 SQL，跑不了）：

| # | 注入的缺陷 | 期望报红 | 归属 |
|---|-----------|---------|------|
| 9 | 自增改成无条件执行 | 分组 E 的「连点 5 次 `likeCount` 停在 1」 | `backend-moment-api` |
| 10 | 两次写拆成两个独立事务 | 分组 E 的不变量检查 | `backend-moment-api` |
| 11 | `RowsAffected = 0` 直接当"已点过" | 分组 E 的「对 B 的动态重复点击仍是 `4040`」 | `backend-moment-api` |
| 12 | `l.user_id = ?` 从 `ON` 挪到 `WHERE` | 分组 E 的 `liked` 断言 | `backend-moment-api` |

**任何一条注入后检查仍然"通过"，说明那条验收是假的**——先修验收，再改代码。读数连同日期提交。

> ⚠️ **honest 说明**：spec §2.1 的读数表是拿**最终的 §3 代码**跑出来的（编译 + DryRun + 全部静态检查）。
> 但**注入一条都不能省**：注入验的是反方向的问题——"代码被改坏时，这条检查会不会报红"。
> 静态度量（grep / 编译）全部放行、而检查本身失效，正是"假验收"的典型形态。
>
> 本支尤其要注意 **1 与 2 必须分开**：上一支 `moment_comments` 只有一个"真外键列"风险点，
> 本支有**两个**，只注入一个会留下另一半没验。

### 5.5 拒绝标准（出现任一条就退回重写，不做"小修小补"）

1. `moment_like.go` 出现 `bigserial`；
2. `gorm:"..."` 内 `autoIncrement` 出现 2 次以上（外键列误加）；
3. `uniqueIndex:uq_moment_like` 不是**恰好 2 处**，或 priority 不是 1/2；
4. 出现 `persona_id` / `PersonaID` / `personaName` 字段（spec §1.0）；
5. 出现任何 `check:` tag（spec §1.5）；
6. 出现 `LikeCount` / `liked` 字段（spec §3.1）；
7. 缺任一关联字段（对应的外键建不出来）；
8. `.go` 里出现 DDL 逐字对照 / 长篇学习性解释 / 反例分析（spec §1.6）；
9. 改了 `router.go`、`go.mod`、`ai_moment.go`、`moment_comment.go` 或他人的 model 文件；
10. 序列化测试断言的是"包含"而不是"键集相等"，或键集不是 4 个；
11. **`\d moment_likes` 的 `user_id` 或 `moment_id` 有 Default**（spec §1.1，头号验收项），
    或 `\ds` 多出任何序列。**只看列名不看 Default，等于没查。**

---

## 6. 风险与对策

| 风险 | 概率 | 对策 |
|------|:---:|------|
| **只查了 `moment_id` 的 Default，漏掉 `user_id`** | 中 | §3.2 的六项命令 + 分组 A；**本表有两条外键列**，§5.3 高危点 1 |
| **从 `moment_comment.go` 抄来 `persona_id` / `check:` / 指针** | 中 | spec §1.0 单独立节；§5.5 第 4/5 条直接退回；`\d` 一比对就露馅 |
| 复合唯一索引**只挂一列**（唯一性残缺但看起来正常） | 中 | §3.2 的两正两反四条 INSERT；§5.4 注入 3 |
| `priority` 写反，索引列序与 DDL 不符 | 中 | §3.2 的 `\d` 核对；§5.4 注入 4 |
| 照抄 `moment_comments` 的 `check:` 计数（恰好 1） | 中 | spec §2.1 读法 2 明确写了两者数字不同（本表是 **2**） |
| **把 `uniqueIndex:` 换成 `unique:` 去"凑 CONSTRAINT"**（建表不报错，症状延迟到第二个用户） | 中 | spec §1.2 有实测渲染对照；§5.4 注入 9；`\d` 看**列清单是几列**而不是看有没有 UNIQUE 字样 |
| 把 `ON CONFLICT ON CONSTRAINT` 当标准写法 | 中 | spec §1.2 有实测物证；§5.5 已并入交接速查 |
| 范围蔓延（顺手把 repo / DTO 写了） | 中 | §2 的交接表；`git diff --name-only` 必须是 4 个文件 |
| 与既有 model 注释风格不一致，评审反复 | 低 | spec §1.6 已决策：新文件从简，**不以风格不一致为退回理由** |
| 下游把两次写拆成两个事务（本表最贵的缺陷） | 中 | spec §1.3 写了"不可自愈"的完整推演；§5.4 注入 10 交接给 API 分支 |

---

## 7. PR 描述草稿

```markdown
# feat(moment): add moment like model and migration

## 范围（模型层，只有 4 个文件）
- `internal/model/moment_like.go`：`MomentLike` struct
- `internal/model/migrate.go`：追加一行 `&MomentLike{}`
- `internal/model/moment_like_test.go`：JSON 键集断言
- `.learn/moment-like-model.md`（不入库）

## 明确不在本 PR 内（已交接，见 spec §0.2）
- 点赞 SQL / 事务 / 自增 —— `feature/backend-moment-api`
- dto / service / handler / 路由 —— `feature/backend-moment-api`
- 计数对账任务 —— `feature/backend-moment-job`

## 实现要点
- **本表没有 `persona_id`**：点赞人 = 当前登录用户，记在 `user_id`（spec §1.0 记录了按 DDL 定案的依据）
- 两条外键列都 `bigint` 且都不带 `autoIncrement`：实测有/无这一个 tag，渲染结果是
  `bigint NOT NULL` 与 `bigserial NOT NULL` 的区别
- `uq_moment_like` 是**复合唯一索引**（挂两列，priority 1/2），幂等靠它不靠 Go 的 `if`。
  **实测渲染为 `CREATE UNIQUE INDEX` 而非 `CONSTRAINT`** —— 下游 `ON CONFLICT` 必须用列清单形式
- 本表**没有** CHECK 约束（DDL 上就没有，判据是"DDL 有没有"而不是"加了更安全"）
- `like_count` 不在本表（在 `ai_moments`），加进来会被 AutoMigrate 建成多余真列

## 验证
- [ ] `go build ./... && go vet ./... && go test ./...`
- [ ] `\d moment_likes` 恰好 4 列；**`user_id` 与 `moment_id` 的 Default 都为空**
- [ ] 索引恰好 3 个；`uq_moment_like` 是 `(user_id,"moment_id")`
- [ ] `\ds` 只有 `moment_likes_id_seq`；朋友圈相关序列**恰好 3 个**
- [ ] 两条外键均为 `ON DELETE CASCADE`；删人设后其动态下点赞清零且他人点赞仍在
- [ ] 重复点赞报 `23505`；同 user 赞另一条 / 另一 user 赞同一条都成功
- [ ] spec §5 分组 C 的 9 条注入全部报红（附读数）

## 顺带闭合
- [ ] closes ai-moment spec §9 分组 E 第 1 条（`moment_id` 的 Default 遗留未闭合项 —— 本支闭合后半，
      `moment_comments` 支已闭合前半；此后"朋友圈三序列干净"完整成立）
```

---

## 8. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|------|------|------|------|
| 2026-09-18 | v1.2 | **注入从 8 条加到 9 条**：新增第 9 条（`uniqueIndex:` → `unique:`），§2 步骤 6、§5.4 表、§6 风险表、§7 PR 勾选项同步为 9；§7 把"spec §5.4"的错误交叉引用改为"spec §5 分组 C" | 落码后跑对照实验（DryRun 两种 tag）实测出：`unique:` **表达不了复合唯一**，会静默建出两条单列约束并丢弃自定义名。这是"照 DDL 字面翻译"最自然会踩的坑，且 `\d` 一眼看不出 |
| 2026-09-18 | v1.1 | §5.2 增加**字段级静态检查**一行（不依赖注释措辞，与键集断言互为独立闸门）；§5.4 注入 5 的期望报红补上这条；§3.3 的结构探针定位与定稿代码对齐 | 审查意见 ②；spec v1.1 的 §2.1 读法 4 暴露了"键集断言依赖注释措辞"这个弱点，用一条字段级 grep 补掉 |
| 2026-09-18 | v1 | 创建 | 与 spec v1 配套的施工步骤、真库核对命令、人工审查计划。**实跑纠出并改正了原始要求 5 的两处**：`persona_id_seq` 那条验收无判别力（本表无该列）、`ON CONFLICT` 的形态差异 |

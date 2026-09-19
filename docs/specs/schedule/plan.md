# plan · 日程提醒（Schedule）· 施工与审查计划

> 配套合同：[spec.md](./spec.md)。分歧时以 spec 为准。
> 本文件写"怎么做"和"怎么审"，不重复 spec 的约束条文。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-schedule-model` |
| 负责人 | 成员 3（`JMX2033`） |
| **本支范围** | **模型层：`schedule.go` + `migrate.go` 一行**（spec §0.1） |
| 前置 | spec 已人工审查通过（**未过审不动手写代码**）；前 9 张表的 model 分支均已合并 |
| 分支存活 | ≤ 3 天（[AGENTS §6](../../../AGENTS.md)）——本支只有 4 个文件，1 天内应收口 |

---

## 1. 目标与产出物

**一句话**：把 `schedules` 的表结构钉死——**三条外键（两条 CASCADE、一条 SET NULL）一条都不能少，
三个外键列的默认值一个都不能有**——并作为**全项目最后一张表**，第一次把"全库 10 个序列干净"这条结论**完整跑通**。

**本支产出（4 个文件）**：

| # | 文件 | 动作 | 内容 |
|---|------|------|------|
| 1 | `internal/model/schedule.go` | 新增 | `ScheduleStatus` + 三个常量 + `Schedule` struct + `TableName()`（spec §3 有完整代码，逐字对照） |
| 2 | `internal/model/migrate.go` | 修改 | 追加一行 `&Schedule{}`（**第 10 行**） |
| 3 | `internal/model/schedule_test.go` | 新增 | JSON 键集断言（spec §5 分组 C） |
| 4 | `.learn/schedule-model.md` | 新增（**不入库**） | 学习笔记：三外键形态、触发链路越权推演、排序兜底推演、named type + `default:` 实验 |

**交接物（本支只写设计，不写代码）**：

| 交付给 | 内容 | spec 位置 |
|--------|------|----------|
| `feature/backend-schedule-api` | 三条归属条件的形态、`ORDER BY remind_at ASC, id DESC`、触发链路的 persona 归属校验、四条反例 | spec §1.2 / §1.3 / §4 |
| `feature/backend-schedule-job` | 扫描排序同样要带兜底列 | spec §1.3 第 3 条 |
| ai-service（成员 1） | 解析器不碰数据库、无提醒意图不建日程、解析不出要回问 | spec §5 分组 E |
| **群里** | 两处契约口径待对齐：缺 `personaId` 是 `4001` 还是 `4002`；对 `sent` 的行再 `DELETE` 的语义 | spec §1.2 / §4.2 |

---

## 2. 施工步骤

> 原则：**建表 → 真库核对 → 再写代码**。表结构错了，下游全部返工。

| 步 | 动作 | 完成判据 |
|----|------|---------|
| 1 | 写 `model/schedule.go`（spec §3 逐字） | `go build ./...` 过 |
| 2 | `migrate.go` 追加一行 `&Schedule{}` | `go run ./cmd/migrate` 建表成功 |
| 3 | **DryRun 回填 spec §2.1 的读数**（10 项，含 §1.4 的 named type + `default:` 组合） | 读数逐项与期望一致；不一致先停下来改设计 |
| 4 | **真库核对**（§3.2）——含 spec §1.1 的三条头号验收项 | spec §5 分组 A 全绿 |
| 5 | 手工验三条外键的三种行为（§3.2 末段） | 分组 B 全绿 |
| 6 | 写 `schedule_test.go`（§3.3，**两条**：键集产物 + tag 声明） | `go test ./internal/model/` 过 |
| 7 | **反向验证**（§5.4）：14 条注入**逐条**确认报红 | 14/14 报红，读数记回 §5.4 |
| 8 | 写 `.learn/schedule-model.md`（§3.4） | 长注释已从 `.go` 挪走 |
| 9 | 自测 + PR（§7） | `go build ./... && go vet ./... && go test ./...` 过 |

**建议的 commit 切分**（[AGENTS §6](../../../AGENTS.md)「产出即提交」）：

```
feat(schedule): add schedule model
feat(schedule): register schedule in automigrate
test(schedule): assert schedule json keys
docs(schedule): add spec and plan for schedule-model
```

### ⛔ 不在本支的范围

| 事项 | 去向 | 触发条件 |
|------|------|---------|
| 落库 / 列表 / 取消三条 SQL | `feature/backend-schedule-api` | 本支合并后 |
| `service` / `handler` / `dto` | `feature/backend-schedule-api` | 同上 |
| 到期扫描任务 | `feature/backend-schedule-job` | 同上 |
| 时间解析器 | ai-service（成员 1） | 独立 |
| `router.go` 的一行 | **成员 1** | API 分支完成后通知 |

> **为什么不让本支顺手把 API 也写了**：本支的核心风险在**跨表写**（触发链路要往 `chat_messages` 写一行，
> 见 spec §1.2 的例外），它值得一个**有运行时验收**的 PR 来承载。混在 model 分支里，那组用例没有地方跑。

---

## 3. 本支的实现要点

### 3.1 写 `schedule.go`（步骤 1）

代码在 **spec §3**——完整 struct，**逐字对照**，不要临场发挥。

**五件最容易写错的事**：

```go
// ① 三个外键列都不带 autoIncrement（spec §1.1）。本表有【三条】外键列，三条都要查
UserID          uint64  `gorm:"column:user_id;type:bigint;not null" json:"-"`
PersonaID       uint64  `gorm:"column:persona_id;type:bigint;not null;index:idx_schedules_persona" json:"personaId"`
SourceMessageID *uint64 `gorm:"column:source_message_id;type:bigint" json:"-"`

// ② 三条关联字段都要有，且 source_message_id 那条只能是 SET NULL（spec §1.0 / §1.5）
// ③ 没有 CHECK 可抄 —— DDL 上一条 CHECK 都没有（spec §1.4）
// ④ JSON 判据是契约的 Schedule 有几个键（6 个）：UserID / SourceMessageID 都是 json:"-"
// ⑤ user_id 上加索引是【错的】—— DDL 只给了两个（spec §2 第三条纪律）
```

**注释预算：≤ 15 行**（spec §1.6），三类各覆盖若干处即可（设计稿计 11 行，落码后复测回填）。
`priority` 的含义、named type 为什么配 `default:`、为什么 `SET NULL` 用指针——**长版进 `.learn/`**，
`.go` 里只留一句结论 + spec 段号。

> ⚠️ **注释里不要写 `foreignKey` / `references` / `check` 这类 tag key 的字面量**——
> spec §5 分组 A 有几条**字面 grep** 靠它们计数，注释一写就失去判别力（[ai-moment §3.2 惯例](../ai-moment/spec.md)）。
> `autoIncrement` 例外：那条 grep 已限定在 `gorm:"` 内，注释里描述它不会误命中。

> ⚠️ **不要从 `user_memory.go` 整段照抄**。它的形状最接近（也是三外键 + `SET NULL`），但：
> 它的 `user_id` **有索引**（`idx_memory_user_id`）、它没有 `status`、它的 `memory_type` 是**组合索引的一员**。
> 本表照抄会多建一个索引、漏掉 `status` 的两个 tag 段。**只抄列对照，不抄 tag 全貌。**

### 3.2 真库核对（步骤 4 + 5，**别跳**）

> ⚠️ `cmd/migrate` 读的是 `SCHEMA_CHECK_DSN`，**不随 `.env` 自动生效**，必须自行 export（见 `cmd/migrate/main.go` 顶部注释）。

```bash
docker compose -f deploy/docker-compose.dev.yml up -d
export SCHEMA_CHECK_DSN='postgres://heart_echo:<password>@localhost:5432/heart_echo?sslmode=disable'
cd backend && go run ./cmd/migrate
```

**逐条确认**（spec §5 分组 A）：

| 看什么 | 期望 | 抓什么错 |
|--------|------|---------|
| 列 | **恰好 8 列**，无 `personaName` / `lastNudgeAt` 之类 | 派生字段被写进 model |
| ▲ **`user_id` 的 Default** | **空** | **spec §1.1，头号验收项之一** |
| ▲ **`persona_id` 的 Default** | **空** | 另一个头号验收项 |
| ▲ **`source_message_id` 的 Default** | **空** | 第三个（本表比前两支多一列） |
| `status` | `character varying(10)`，Default **`'pending'`**，**无 CHECK 约束** | §1.4：`check:` tag 被误加；或默认值引号丢了 |
| `\ds` | 只有 `schedules_id_seq` | 外键列误加 `autoIncrement` |
| 外键 | **3 条**：`→ users` / `→ personas`（都 CASCADE）、`→ chat_messages`（**SET NULL**） | 漏写关联字段；或 `SET NULL` 被写成 CASCADE |
| 索引 | **恰好 2 个具名**：`idx_schedules_due(status,remind_at)` / `idx_schedules_persona(persona_id)` | `priority` 写反、或给 `user_id` 多加了索引 |

**头号验收项的命令**（spec §1.1，逐条给读数）：

```bash
# ① 人读版：三个名字应各 0 结果
psql "$SCHEMA_CHECK_DSN" -c '\ds' | grep -E 'schedules_(user_id|persona_id|source_message_id)_seq'

# ② 机器版：应 1 / 0 / 3 / t / t / t / 2 七项全对
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM pg_class WHERE relkind='S' AND relname LIKE 'schedules%';"                        # 期望 1
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM pg_class WHERE relkind='S' AND relname IN ('schedules_user_id_seq','schedules_persona_id_seq','schedules_source_message_id_seq');"  # 期望 0
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM information_schema.columns WHERE table_name='schedules';"                       # 期望 8
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT column_default IS NULL FROM information_schema.columns WHERE table_name='schedules' AND column_name='user_id';"            # 期望 t
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT column_default IS NULL FROM information_schema.columns WHERE table_name='schedules' AND column_name='persona_id';"         # 期望 t
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT column_default IS NULL FROM information_schema.columns WHERE table_name='schedules' AND column_name='source_message_id';"  # 期望 t
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT count(*) FROM pg_indexes WHERE tablename='schedules';"                                        # 期望 3（2 具名 + schedules_pkey）
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT indexdef FROM pg_indexes WHERE tablename='schedules' AND indexname='idx_schedules_due';"      # 期望列序 (status, remind_at)

# ③ 全库收口（本支第一次跑得动，spec §1.1）
psql "$SCHEMA_CHECK_DSN" -Atc "SELECT relname FROM pg_class WHERE relkind='S' ORDER BY 1;"   # 期望【恰好 10 行】，且逐字等于 spec §1.1 的名单
```

> **第 ② 组必须全跑**。只跑 grep 那一条会放过"把 `ID` 的 `autoIncrement` 也删了"这种改法——
> 它同样没有外键序列，但 `schedules_id_seq` 也没了，本表再也插不进数据（spec §1.1 的反面）。
>
> ⚠️ 三个 `<表>_<列>_seq` 的**名字要一个一个列**（用 `IN (...)`），**不要**写成 `LIKE 'schedules\_%\_id\_seq'`
> 之类的模糊匹配：`_` 在 LIKE 里是单字符通配符，模糊匹配给出的"0 结果"比显式名单弱（spec §1.1 精度修正）。

**再手工验三条外键的三种行为（步骤 5，spec §5 分组 B）**——每一步都要留一条**不该被动的行**：

```sql
-- 准备：账号 A / B，各带人设；A 的两个人设 A1 / A2；各造日程，其中一条带 source_message_id
-- ① 删人设（CASCADE）：删 A1 → A1 的日程清零，A2 的仍在
-- ② 删账号（CASCADE）：删 B → B 的日程清零，A 的仍在
-- ③ 删消息（SET NULL）：删那条 chat_messages → 日程【行还在】，source_message_id 变 NULL（不是 0、不是删行）
SELECT count(*) FROM schedules;                       -- ③ 前后对比：不变
SELECT source_message_id IS NULL FROM schedules WHERE id = <schedule_x>;   -- 期望 t
```

> ③ **是三条里唯一"看起来该被删却没删"的行为**，所以它必须单独有一条正面断言（行还在 + 列变 NULL）。
> 只验"没报错"是不够的——把 `SET NULL` 写成 `CASCADE` 也不会报错。

验完删掉这些测试数据。

### 3.3 序列化测试（步骤 6）

`internal/model` 下的第四个测试文件，沿用 [ai-moment §9 分组 C](../ai-moment/spec.md) 的「键集**恰好**相等」模式。

**本表与 [moment_likes](../moment-like/spec.md) 的性质不同**（spec §5 分组 C）：
`moment_likes` 没有契约实体，那条测试是**结构探针**；本表**有**契约实体（`Schedule`），
所以这条是**真正的契约校验**——键集来自契约 §10，**不是**从 Go struct 反推的。

```go
// TestScheduleJSONKeys 锁死 Schedule 对外的键集（恰好 6 个，与契约 §10 的 Schedule 逐字一致）。
//
// userId / sourceMessageId 是本表最容易抄错的两个：契约里没有它们，
// 但 moment_like.go / user_memory.go 对同名列的写法【相反】，照抄哪一张都可能错（spec §2）。
func TestScheduleJSONKeys(t *testing.T) {
	// 零值 marshal → 键集恰好 {id, personaId, content, remindAt, status, createdAt}
}
```

**关键**：断言的是**键集相等**（不是"包含"），多一个字段就报红。

**但只有这一条不够 —— 必须同时有第二条 `TestScheduleJSONTags`。**
`encoding/json` 对**同名**字段会**静默丢弃全部、且不报错**：把 `UserID` 的 `json:"-"` 改成
`json:"userId"`、**同时**把关联字段 `User` 也写成 `json:"userId"` 时，序列化输出与合规时
**逐字节一致**（六个键一个不少），上面那条 marshal 断言**照样绿**。
所以第二条**不看产物、看声明**：

```go
// TestScheduleJSONTags 查字段的 json tag 本身，而不是序列化结果——上一条看产物，这条看声明。
//
// 为什么产物不够：encoding/json 遇到同名字段会静默丢弃全部、不报错（spec §5 分组 C 注入 14）。
func TestScheduleJSONTags(t *testing.T) {
	// reflect.TypeFor[Schedule]().Fields() → map[json 名][]Go 字段名
	// 双向断言：① 名字集合 == scheduleJSONKeys ② 没有任何两个字段共用同一个 json 名
}
```

**契约键集只写一份**：`var scheduleJSONKeys = map[string]bool{...}` 提到**包级**，两条测试共用。
两处各写一份，改了一处忘另一处，就变成两条标准互相打架。

**三条闸门各瞎一处，删任一条对应缺陷全静默**：
键集断言验"序列化出来多了什么" / §5.2 的字段级 grep 验"struct 里多了什么" /
这里的 tag 名集合验"**tag 声明了什么**"。

### 3.4 `.learn/schedule-model.md`（步骤 8）

从 `.go` 里**挪出来**的长内容，全部落这里：

- 三外键两种 `ON DELETE` 的完整形态表，以及"从 `moment_likes` 抄 CASCADE 会写宽"的推演；
- **触发链路的越权推演**：`TriggerNow` 为什么要联 `personas`，而列表为什么不用（spec §1.2）；
- **排序兜底推演**：`remind_at` 是解析器输出而非数据库生成 → 同值撞车为何是结构性的；`id DESC` 方向为什么无所谓但已定死；
- `named string type` + `default:` tag 的实验记录（DryRun 两种写法各渲一遍，与 `memory_type` / `embedding_status` 的差异）；
- 为什么本表**不加** `check:`，以及"有常量了所以可以加约束"这个推论错在哪（spec §1.4）；
- user_memory tag 截断事故的**重放记录**：截断后六条 grep 逐条跑一遍，看哪条会在注入 4 时报红；
- spec §4.3 十二条反例逐条展开。

`.learn/` 在 `.gitignore:75`，**不入库**。

---

## 4. 交接速查（下游要执行的，本支不落）

不复制内容，只给指针——**单一事实来源在 spec**：

| 下游任务 | 抄哪里 | 最容易翻车的点 |
|---------|--------|--------------|
| 列表查询 | spec §1.2 / §4.1 | **两个条件**（`persona_id` + `user_id`）不是两个分支选一个 |
| 列表排序 | spec §1.3 | 兜底列不能省；前端**不要**重排 |
| 取消 | spec §4.1 第 3 条 | 用 `Updates` + 非零值，别用 `Save()`；重复删除要幂等 |
| 触发 | spec §1.2 例外 / §4.1 第 4 条 | 必须复用 `TriggerNow`；**归属校验要覆盖 `persona`** |
| 状态取值 | spec §3（常量） | 不写裸字符串；不新增第四态 |
| 扫描任务 | spec §1.3 第 3 条 | 带 `LIMIT` 就要带兜底列 |
| 契约口径 | spec §1.2 / §4.2 | **两处待群里对齐**，别自己挑 |

---

## 5. 我手动审查 AI 代码的计划

> 前提：代码由 AI 生成，我**逐行审**，不"看着像对就过"。审查的是 spec 的落实，不是代码风格。

### 5.1 审查顺序

`schedule.go` → `migrate.go` → `schedule_test.go` → 真库 `\d` / `\ds`。
**先审 tag（静态），再审真库（运行时）**——两者互为交叉验证：tag 对而 `\d` 不对，说明 tag 语义理解错了。

### 5.2 逐文件审查清单

| 文件 | 只看这几件事 |
|------|-------------|
| `schedule.go` | ① `ID` 是 `type:bigint;primaryKey;autoIncrement`，全文件**无 `bigserial`**；② `gorm:"..."` 内 `autoIncrement` **恰好 1 处**（**三个外键列都要看**）；③ `gorm:"..."` 内 `check:` **0 处**（§1.4）；④ `foreignKey:` / `references:` **各 3 处**（§1.5）；⑤ `OnDelete:CASCADE` **2 处** + `OnDelete:SET NULL` **1 处**；⑥ 三条关联字段都在、`SourceMessage` 是**指针**；⑦ `UserID` **没有** `index:`；⑧ `SourceMessageID` 是 `*uint64`；⑨ `status` 的 `default:'pending'` **带单引号**；⑩ 注释 ≤ 15 行、**不含 tag key 字面量**、无 DDL 对照 |
| `migrate.go` | 是否只加了一行 `&Schedule{}`，且在 **最后**（`&MomentLike{}` 之后）；**没动其他行** |
| `schedule_test.go` | 断言的是**键集相等**还是"包含"？后者是假测试。键集是否为 **6 个**？有没有断言**没有** `userId` / `sourceMessageId`？**两条测试是否都在**——产物（`TestScheduleJSONKeys`）+ **tag 声明级**（`TestScheduleJSONTags`：名字集合 == 契约键集、且无两个字段共用一名）？契约键集是否只有**一份**（包级 `scheduleJSONKeys`）？**只有产物那一条 = 对"字段被静默丢弃"完全瞎**（§5.4 注入 14） |
| **字段级静态检查** | `grep -cE '^[[:space:]]+(PersonaName|LastNudgeAt|EmotionLabel|EmotionScore|Username)[[:space:]]' internal/model/schedule.go` 是否 **0**？与键集断言是**两道独立闸门**（spec §5 分组 C） |
| `.learn/` | 是否**未被** `git status` 列出 |
| `git diff --name-only` | 是否只有 4 个文件（含 `.learn/` 则为异常） |

### 5.3 高危点排序（**只审三条的话，审这三条**）

1. **三个外键列的 Default**——本表**有三个**，比前两支都多。只查 `persona_id` 会漏掉另外两个；
   `source_message_id` 因为可空、又常被"给个默认值"的直觉盯上，是三条里最危险的。
2. **`source_message_id` 的 `SET NULL` + 指针 + tag 三段**（spec §1.5）——这三种错**全部静默**：
   写成 CASCADE 建表不报错、tag 截断 `go build` 不报错、值类型要到"删消息"才暴露。
   这是**重放 user_memory 的真实事故**，不是假想。
3. **越权条件是不是两个**（spec §1.2）——只 `persona_id` 会越权（红线 3），
   只 `user_id` 会跨人设串号；而"要求 3 说有 `user_id` 就直接过滤"这句话**很容易被读成"只过滤 `user_id`"**。

### 5.4 反向验证记录（**注入缺陷，确认检查会报红**）

只勾"通过"不够——**要证明检查在代码变坏时真的会失败**，否则可能是"检查本身失效"。

**本支可跑的 14 条**（spec §5 分组 C 的表）：

| # | 注入的缺陷 | 期望哪条报红 | 实际读数 |
|---|-----------|-------------|---------|
| 1 | `UserID` 的 tag 加 `autoIncrement` | 分组 A 的 `autoIncrement` 恰好 1 + `\ds` | ⬜ 待跑 |
| 2 | `PersonaID` 的 tag 加 `autoIncrement` | 同上（**分开跑**） | ⬜ 待跑 |
| 3 | `SourceMessageID` 的 tag 加 `autoIncrement` | 同上（**三个外键列各跑一遍**） | ⬜ 待跑 |
| 4 | `SourceMessage` 的 tag 截断成只剩 `constraint:OnDelete:SET NULL` | `foreignKey:` / `references:` 恰好 3 + `\d` 缺外键 | ⬜ 待跑 |
| 5 | `SourceMessageID` 改成值类型 `uint64` | `\d` 该列变 `NOT NULL` + 分组 B"删消息"报错 | ⬜ 待跑 |
| 6 | `SourceMessage` 的 `OnDelete` 改 `CASCADE` | 分组 B 的"日程行还在" | ⬜ 待跑 |
| 7 | 给 `Status` 加 `check:...` | 分组 A 的 `check:` 零命中 | ⬜ 待跑 |
| 8 | `default:'pending'` 去掉单引号 | 建表失败 | ⬜ 待跑 |
| 9 | `idx_schedules_due` 两处 `priority` 对调 | `\d` 列序变 `(remind_at, status)` | ⬜ 待跑 |
| 10 | 给 `UserID` 加 `index:idx_schedules_user` | 分组 A 的"索引恰好 2 个" | ⬜ 待跑 |
| 11 | 给 struct 加 `PersonaName string` | "恰好 8 列" + 字段级 grep + 键集 | 分组 C **两条测试均报红** ✅（2026-09-19，口径 `go test ./internal/model/ -run TestScheduleJSON`，非真库）；"恰好 8 列" ⬜ 待真库复跑 |
| 12 | 删掉 `User User` 关联字段 | 分组 A 的 `user_id → users` 外键消失 | ⬜ 待跑 |
| 13 | `ID` 改成 `type:bigserial` | 分组 A 的 `bigserial` 零命中 | ⬜ 待跑 |
| 14 | `UserID` 与关联字段 `User` **同时**写成 `json:"userId"`（**撞名**） | **只有** `TestScheduleJSONTags`；键集断言**看不见** | ✅ 声明检查红、产物检查绿（2026-09-19，同口径）——**实测中撞出来的真盲区，不是设计的** |

**任何一条注入后检查仍然"通过"，说明那条验收是假的**——先修验收，再改代码。读数连同日期提交。

> ⚠️ **注入 4 是本支最重要的一条**：它重放的是 [user_memory 的真实事故](../../../backend/internal/model/user_memory.go)——
> tag 被截断后，`go build` / `go vet` / `gofmt` 与当时**全部六条 grep** 都是绿的，
> 因为那六条全在查"有没有违规"，没有一条查"**该有的 key 是否齐全**"。
> 本支的 §1.5 就是补上那一半：**跑之前先写下预期**（`foreignKey:` 会从 3 掉到 2），跑完拿读数对。

> ⚠️ **注入 1/2/3 必须分开跑**：本表有三个外键列，`user_memory` 之后第二个三外键表，
> 只注入一个会留下另外两个没验。

> ⚠️ **注入 14 是"验收本身有盲区"的实证，而且它不在原始设计里**：这是做注入 11 时**意外**撞出来的——
> 第一次的替换命令没加锚点，把 struct 里**六处** `json:"-"` 一起改成了同一个名字，等于自己制造出撞名，
> 产物测试于是"该红而红不了"。逐条重做后才发现：初稿的 marshal 断言**不是假标准，是覆盖不全**
> （注入 1/2/11 上都正常报红），它对"字段被静默丢弃"这一类完全瞎。据此补了 §3.3 的第二条测试。
> **过程教训**：反向验证必须**逐条注入**——一次改多个字段会自己制造撞名，
> 把"验收失效"掩盖成"我在故意测盲区"，两者现象完全一样，事后分不出来。

### 5.5 拒绝标准（出现任一条就退回重写，不做"小修小补"）

1. `schedule.go` 出现 `bigserial`；
2. `gorm:"..."` 内 `autoIncrement` 出现 2 次以上（外键列误加）；
3. 出现任何 `check:` tag（spec §1.4）；
4. `foreignKey:` / `references:` 不是**恰好 3 处**（spec §1.5 的截断事故）；
5. `constraint:OnDelete:SET NULL` 不是**恰好 1 处**，或出现 `SET_NULL` / 该列被写成 CASCADE；
6. `SourceMessageID` 是值类型（`uint64`），或 `SourceMessage` 是值类型；
7. `UserID` 上出现 `index:`（spec §2 第三条纪律）；
8. 出现 `json:"userId"` 或 `json:"sourceMessageId"`；
9. 缺失任一关联字段（对应的外键建不出来）；
10. `.go` 里出现 DDL 逐字对照 / 长篇学习性解释 / 反例分析，或注释里出现 tag key 字面量（spec §1.6 / §1.5）；
11. 改了 `router.go`、`go.mod` 或他人的 model 文件；
12. 序列化测试断言的是"包含"而不是"键集相等"，或键集不是 6 个；**或只有产物检查、缺 tag 声明级检查**
    （`TestScheduleJSONTags`）——同名字段会被 `encoding/json` **静默丢弃**，产物检查看不见这一类（§5.4 注入 14）；
13. **`\d schedules` 的 `user_id` / `persona_id` / `source_message_id` 任一个有 Default**（spec §1.1，头号验收项），
    或 `\ds` 多出任何序列。**只看列名不看 Default，等于没查。**

---

## 6. 风险与对策

| 风险 | 概率 | 对策 |
|------|:---:|------|
| **只查了 `persona_id` 的 Default，漏掉 `user_id` / `source_message_id`** | 高 | §3.2 命令组 + 分组 A；**本表有三个外键列**，§5.3 高危点 1 |
| **`source_message_id` 被写成 CASCADE**（要求里"CASCADE"三字被推广） | 中 | spec §1.0 单独立节；§5.5 第 5 条直接退回；分组 B 的"行还在"正反两面 |
| **`SourceMessage` 的 tag 被截断，`go build` 照样过** | 中 | §3.2 tag 静态检查；§5.4 注入 4（重放真实事故） |
| **越权条件只写一个**（把"有 `user_id` 就直接过滤"读成"只过滤 `user_id`"） | 中 | spec §1.2 的两列表格；§5.3 高危点 3；分组 E 的两向用例 |
| 触发链路没校验 `persona` 归属 → 往别人对话写消息 | 中 | spec §1.2 例外单独立小节；分组 E 的"越权触发"用例 |
| 给 `user_id` 顺手加索引（"越权防线靠它"的肌肉记忆） | 中 | spec §2 第三条纪律；§5.4 注入 10；分组 A 的"索引恰好 2 个" |
| **named type + `default:` 组合渲染不出默认值**（全项目第一次） | 中 | spec §1.4 已写回退方案（退回裸 `string` + 常量）；步骤 3 先 DryRun 再定稿 |
| 从 `user_memory.go` 整段照抄（多出 `idx_memory_user_id`、漏掉 `status`） | 中 | §3.1 的警告块；`\d` 一比对就露馅 |
| 排序漏掉 `id` 兜底 | 中 | spec §1.3 三条理由；分组 E 的"三条同 `remind_at` 逐页拉"用例 |
| 范围蔓延（顺手把 repo / DTO 写了） | 中 | §2 的交接表；`git diff --name-only` 必须是 4 个文件 |
| 契约口径不一致（`4001`/`4002`、过期的 `4031`）被 AI 私自定掉 | 中 | spec §1.2 的登记表；§4 交接速查最后一行 |
| 与既有 model 注释风格不一致，评审反复 | 低 | spec §1.6 已决策：新文件从简，**不以风格不一致为退回理由** |

---

## 7. PR 描述草稿

```markdown
# feat(schedule): add schedule model and migration

## 范围（模型层，只有 4 个文件）
- `internal/model/schedule.go`：`ScheduleStatus` + 常量 + `Schedule` struct
- `internal/model/migrate.go`：追加一行 `&Schedule{}`（第 10 行，全项目最后一张表）
- `internal/model/schedule_test.go`：JSON 键集断言（契约 §10 的 6 个键）
- `.learn/schedule-model.md`（不入库）

## 明确不在本 PR 内（已交接，见 spec §0.2）
- 落库 / 列表 / 取消 SQL、dto / service / handler —— `feature/backend-schedule-api`
- 到期扫描任务 —— `feature/backend-schedule-job`
- 时间解析器 —— ai-service（成员 1）
- `router.go` 的一行 —— 成员 1

## 实现要点
- **三条外键、两种 ON DELETE**：`user_id` / `persona_id` 是 CASCADE，`source_message_id` 是 **SET NULL**
  （要求里只提了 `persona_id` 那一条；把 CASCADE 推广过去会让"删消息顺手删日程"）
- 三个外键列 `bigint` 且**都**不带 `autoIncrement`（本表比前两支多一列）
- `source_message_id` 必须是**指针**（SET NULL 的前提），且 `SourceMessage` 的 tag 三段齐全 ——
  仓库里同名列上次被截断过一次，`go build` 没拦住（spec §1.5）
- **本表没有 CHECK**（DDL 上就没有）；`status` 的防线是 Go 常量，不是 `check:` tag
- 索引照 DDL **恰好 2 个**：`idx_schedules_due(status, remind_at)` / `idx_schedules_persona`
  —— `user_id` 上**不加**索引
- 排序契约：`ORDER BY remind_at ASC, id DESC`（契约未规定，本 spec 定死并交接给 API / 前端）

## 验证
- [ ] `go build ./... && go vet ./... && go test ./...`
- [ ] `\d schedules` 恰好 8 列；**三个外键列的 Default 都为空**
- [ ] 索引恰好 2 个具名对象，`idx_schedules_due` 列序是 `(status, remind_at)`
- [ ] `\ds` 只有 `schedules_id_seq`；**全库 `*_id_seq` 恰好 10 个**
- [ ] 三条外键 `ON DELETE` 依次 CASCADE / CASCADE / **SET NULL**
- [ ] 删人设 / 删账号各自级联，对方的数据仍在；删消息后日程**行还在**且该列变 `NULL`
- [ ] spec §5 分组 C 的 14 条注入全部报红（附读数）

## 顺带闭合
- [ ] closes moment-like spec §5 分组 E（"朋友圈三序列干净"不随新表回归）——本支是**最后一张表**，
      全库 10 个序列的收口在本支第一次完整成立
```

---

## 8. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|------|------|------|------|
| 2026-09-19 | v1 | 创建 | 与 spec v1 配套的施工步骤、真库核对命令、人工审查计划。**把要求 4 的序列验收从"一条 grep"扩成"三项命令组 + 全库 10 序列收口"**；注入清单从上一支的 9 条扩到 **13 条**（本表多两个外键列 + `SET NULL` 形态 + 排序/索引项）；`named type + default:` 列为**必须实测**的新组合 |
| 2026-09-19 | v1（同日补） | §3.1 注释预算与 §5.2 的字段级 grep 与 spec v1 补订同步（设计稿读数非定稿读数；`-E` 模式下 `\|` 是字面竖线，已改回 `|`） | 自查：spec 同日补了"设计稿静态读数"一节，plan 的对应表述必须跟上，否则两文件对"11 行"的口径不一致 |
| 2026-09-19 | v1.0.2（验收补强） | §3.3 增加**第二条测试** `TestScheduleJSONTags`（tag 声明级：名字集合 == 契约键集、且无撞名）与"契约键集只写一份"的约束；§5.4 注入表补**第 14 条**（撞名）并回填注入 11 读数；§5.5 拒绝标准 12 补"缺声明级检查"；§2 步骤 6/7 与 §7 的条数 13 → 14 | **做注入 11 时实测撞出的真盲区**：`encoding/json` 对同名字段静默丢弃全部且不报错，初稿的 marshal 键集断言在该状态下照样绿。原 13 条注入没有一条能覆盖，验收标准缺一半 |

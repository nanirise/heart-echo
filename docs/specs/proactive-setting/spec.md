# spec · 主动消息配置（Proactive Setting）· 仓储层

> 本文件是**合同**：定义本支做什么、做到什么算完成，以及下游分支必须遵守的预写设计。
> 合并后即冻结，任何字段或行为变更必须升版本并在群里广播。
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：DDL 原文、契约字段、错误码表、通用约定一律**不复制**，只给链接；
> 本文件只写**本表独有**的内容——独有陷阱、独有决策、独有验收项。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-proactive-setting-model`（基点 `3ea4051` = PR #43 合并后） |
| 状态 | v1 **已过人工审查**（2026-09-20）——三处拍板见 §6 变更记录第二行；代码与文档**留在工作区未提交** |
| **本支范围** | **仓储层：`proactive_repo.go` 增 `Get` / `UpdateOwned` 两个方法**（模型层已落地，本支不动） |
| 负责人 | 成员 3（`JMX2033`） |
| 依赖分支 | 模型层已随 `6fc970e`（人设 CRUD 分支）进入 main：`internal/model/proactive_setting.go` + `migrate.go` 一行 + `CreateDefaultSettings`（用户口径为 **PR #28**；`git log` 里可查的是 `6fc970e`） |
| 关联契约 | [API_CONTRACT §9](../../API_CONTRACT.md#9-主动消息proactive)（`Settings` 实体 + 3 个端点） |
| 关联设计 | [TECH_DESIGN §5.4](../../TECH_DESIGN.md#54-主动消息实现)（空闲阈值、防骚扰约束）、[§6.2](../../TECH_DESIGN.md#62-核心表-ddl)（`proactive_settings` DDL） |
| 关联 spec | [persona-crud §3.2 / §4.3.1 / §7.2](../persona-crud/spec.md)（**模型落地、播种机制、交接清单**）、[schedule §1.2](../schedule/spec.md)（**同款"一人设一份"的越权口径**）、[user-memory §2](../user-memory/spec.md)（`4043` 与空态的分界） |
| 关联红线 | [AGENTS §4.3](../../../AGENTS.md)（数据边界）、§5.1（注释纪律）、[§7](../../../AGENTS.md)（红线 8：改表结构只走 struct + `AutoMigrate`） |
| 不重复的推导 | 三列 `INT` 为什么必须写 `type:integer`、组合唯一索引的 tag 写法、外键关联字段缺一不可——**见 [persona-crud §3.2](../persona-crud/spec.md)**，本支只做反面验收 |
| 学习内容落点 | `.learn/proactive-settings.md`（**不入库**，`.gitignore:75`）——GORM 三处源码链条与本支实测读数原文 |

---

## 0. 范围声明（先读这一节）

这张表是**主动消息的开关面板**：一个人设一份配置，用户在这里决定"它能不能主动找我、多久找一次、一天最多几次"。
它同时被两条链路读——用户的设置页（`GET` / `PUT`），和 Go 侧的定时任务（扫描空闲人设注入 `[nudge]`）。
**本支只做后者所需的数据访问形态，不做任何端点。**

### 0.1 本支只做三件事

| # | 产物 | 内容 |
|---|---|---|
| 1 | `internal/repository/proactive_repo.go` | 增 `Get` / `UpdateOwned` 两个方法（§3） |
| 2 | `docs/specs/proactive-setting/spec.md` / `plan.md` | 本文件与配套计划 |
| 3 | 一次临时库实测 | §4 分组 A 的读数（**已在写本合同前跑过**，见 `.learn/proactive-settings.md` §4） |

### 0.2 本支不做什么（去向逐个标明）

| 不做的事 | 去向 |
|---|---|
| `dto/proactive_dto.go`、`service/proactive_service.go`、`handler/proactive_handler.go`、`router.go` 一行 | 后续的接口支（建议名 `feature/backend-proactive-api`，尚未创建）。§2 是**给它的预写规格**，本支不写这些文件 |
| `GET` / `PUT /proactive/settings` 两个端点 | 同上。**本支不含任何端点** |
| `POST /proactive/trigger` 与 `TriggerNow` | 同上（契约 §9 第三个端点）。**它写 `last_nudge_at`，但那个写入路径不在本支**（§1.5） |
| 定时任务 / `proactive_job.go` / `robfig/cron` | 同上，且属 TECH_DESIGN §5.4 的扫描逻辑 |
| `intervalMin < intervalMax` 等跨字段校验 | service 层（§2.2）。本支只有仓储层，仓储层不校验业务规则 |
| 修改 `model/proactive_setting.go` | **不在本支**：模型已随 `6fc970e` 落地并过审（2026-09-20 用户拍板：**本支不动**）。该文件的注释密度（约 20 行）高于 AGENTS §5.1 的目标，已定为**另开 refactor 分支清理**（可与前 3 张表一起），见 [plan §8](plan.md)；机制内容已先落在 `.learn/proactive-settings.md`，随时可搬 |
| 新增索引 | `UNIQUE (user_id, persona_id)` 自带的索引已够用（[persona-crud §3.3](../persona-crud/spec.md)：本表不新增任何索引）。**不要顺手加** |

---

## 1. 本表独有的硬性要求

### 1.1 零值 == 缺失：同一条陷阱在**三层各出现一次**（**头号验收项**）

四列带 `default:` tag（`enabled` / `interval_min` / `interval_max` / `daily_limit`），
于是"零值"有三种不同的解读，其中**两种会让"关掉主动消息"静默失败**：

| 层 | 零值被当成 | 后果 | 正确写法 |
|---|---|---|---|
| DTO / binding | "没传" | `{"enabled":false}` 被 `required` 拒掉 → `4001`，**开关永远关不掉** | **指针** `*bool` / `*int` |
| 仓储 `Updates(struct)` | "不用改" | `Error=nil` + `RowsAffected=0` 的**静默空操作** | **`map[string]any`** |
| 仓储 `Create(struct)` | "请用 tag 默认值" | `Enabled:false` 入库成 `true` | 显式赋值（播种处已显式写出四个值） |

三条都是**实测读数**，不是推理（`.learn/proactive-settings.md` §4 有原文与执行口径）：

- `UpdateOwned(enabled=false, 0,0,0)`（map 路径）→ `RowsAffected=1`，落库 `enabled=false` 且三个 `0` 都在；
- `Updates(model.ProactiveSetting{Enabled:false, DailyLimit:0})`（struct 路径）→ **`Error=nil`、`RowsAffected=0`**，库里值一动不动；
- 零值 struct 直接 `INSERT`（只给 `user_id` / `persona_id`）→ 落库 `true / 30 / 120 / 3`。

> ⚠️ **报错的那条最容易被误判成"成功"**：`Error=nil` 让人以为改完了，`RowsAffected` 才是真话。
> 所以 §3 的 `UpdateOwned` **返回 `RowsAffected` 并要求调用方判它**——这个返回值不是可选的信息，是判定依据（§1.2）。

### 1.2 越权防线：两个条件都带；`RowsAffected == 0` 就是 `4043` 的信号

[AGENTS §4.3](../../../AGENTS.md) 对"一人设一份"的表要求**同时带 `persona_id` 与 `user_id`**，
`WHERE persona_id = ? AND user_id = ?` 两个条件缺一不可（`user_id` 只从 Token 取）。
本表**不需要**像 `user_memory` 那样单独做一次归属探针——**配置行自己就带这两个键**，一条查询同时完成"取数"与"验归属"。

`RowsAffected == 0` 的语义**唯一**：WHERE 没匹配到行。

- Postgres 的 `UPDATE` 即使新旧值相同**也算匹配到行**，所以"值没变"不会产生 `0`；
- 于是 `0` 只可能是**播种缺失**或**不是自己的行**。两种情形**同码同文案 → `4043`**（不区分"不存在"与"不属于你"，口径见 [user-memory §2.1](../user-memory/spec.md)）；
- **不允许**返回 `4030`（功能越权的码，本模块无此场景），**也不允许**返回 `4040`（`4040` 与 `4043` 并存会泄漏存在性）。

> 播种缺失也被压进这个 `4043` 里，是一个**刻意的取舍**：它对外与"不是你的"不可区分（防探测优先），
> 对内则是**运维信号**——`GET` 老返回 `4043` 而人设明明是自己的，就去查 `POST /personas` 的播种事务（[persona-crud §4.3.1](../persona-crud/spec.md)）。

### 1.3 两条外键都必须是 `ON DELETE CASCADE`（DDL 已定，本支只做反面验收）

`users` 与 `personas` 各一条，**两条都要在**，删账号 / 删人设时配置行跟着消失（不留孤儿配置）。
缺任何一条，"级联清配置"就不成立——而**缺一条同样能编译、能跑**，只有 `\d proactive_settings` 看得出来。
本支不改这两个 tag，只把它们写进验收（§4 分组 A 的 G 项）。

### 1.4 查不到行是**异常**，所以**不用 `Upsert`**

[persona-crud §7.2](../persona-crud/spec.md) 的交接清单写的是"继续加 `Get` / `Upsert`"。**本支定为 `UpdateOwned`**，理由：

1. 播种机制保证**恰好一行**（`POST /personas` 同事务写入 + `UNIQUE (user_id, persona_id)`）。查不到行 = **播种没跑**，是 bug，不是"需要补一行"的正常态；
2. `Upsert` 会**把这个 bug 掩盖成一个静默的插入**：`RowsAffected` 恒为 1，调用方再也拿不到"行不存在"这个信号，而新插入的行带着**用户没要求过的默认值**；
3. `Upsert` 还要求调用方把 `user_id` / `persona_id` 一起塞进写入值——**多一个能写归属列的口子**，与本表"归属只从 Token 来"直接冲突。

**这条差异要广播**（2026-09-20 用户已接受该改动）：说明 [persona-crud §7.2](../persona-crud/spec.md) 的"继续加 `Get` / `Upsert`"**被本文修正**，下游不要照着 `Upsert` 写。广播草稿见 [plan §6](plan.md)。

### 1.5 `last_nudge_at` 是只读列，**不进任何 `map`**

契约 §9 明写 `lastNudgeAt` **只读、`PUT` 时忽略**。防御落在签名上：

- `UpdateOwned` 的四个参数**没有**它，方法体里的 `map` 也**不出现**这个键——想写它必须改签名，不可能"顺手带进去"；
- 它由**触发链路**写（`POST /proactive/trigger` 与定时任务），不在本支；
- 它 `*time.Time` 且**可空**：`NULL` = 从未触发过。**不要**用值类型（会变成 `0001-01-01`，定时任务会以为它刚被触发过）。

### 1.6 注释只写必要的（[AGENTS §5.1](../../../AGENTS.md)）

与 [schedule §1.6](../schedule/spec.md) 同一纪律，不重复。本支的落点：
`proactive_repo.go` 里只留"**为什么必须用 map**"这条红线（两行）与"`RowsAffected==0` 的含义"（一行）；
GORM 源码链条、实测读数、反例分析**全部进 `.learn/proactive-settings.md`**。

> **风格冲突沿用既有决策**：既有文件注释超密度，**本支不清理它们**（除 §0.2 注明的建议外）；
> 审查时**不以"与既有文件风格不一致"为退回理由**。

---

## 2. 接口定义（**预写规格，本支不实现**）

两个端点的形态、绑定规则、错误码分界。**下一支照着写，不要临场发挥。**

### 2.1 `GET /proactive/settings`

| 项 | 内容 |
|---|---|
| 请求 | query：`personaId`（必传） |
| 响应 data | 契约 §9 的 `Settings`，**恰好 6 个字段**（`personaId` / `enabled` / `intervalMin` / `intervalMax` / `dailyLimit` / `lastNudgeAt`）；`id` / `user_id` 是 `json:"-"` |
| 错误码 | `4001`（缺 / 非法 `personaId`，**契约空白**，见 §2.3）、`4043`（不是自己的 / 不存在的 / 播种缺失，**同一个码**） |

- `user_id` **只从 Token 取**（`c.GetUint64(middleware.ContextKeyUserID)`，**必须检查 `ok`**；取不到 → `4010`，不许退化成 `0`）。
- query 参数的 tag 必须是 **`form:"personaId"`**：Gin 的 `ShouldBindQuery` 读 `form` tag 且**区分大小写**，写成 `persona_id` 会**静默绑不上**、`PersonaID` 恒为 `0`（同坑见 [user-memory §3.1](../user-memory/spec.md)）。
- 落地路径：service → `repo.Get(userID, personaID)` → `errors.Is(err, gorm.ErrRecordNotFound)` → `4043`。

### 2.2 `PUT /proactive/settings`

| 项 | 内容 |
|---|---|
| 请求 | body：`personaId` + **四个可改字段全部必传**（`enabled` / `intervalMin` / `intervalMax` / `dailyLimit`） |
| 响应 data | 与 `GET` 同一个 `Settings`（**更新后回读**，不是把请求体原样回显） |
| 错误码 | `4001`（字段缺失 / 越界 / `intervalMin >= intervalMax`）、`4043`（不是自己的 / 不存在的 / 播种缺失） |

**四条要求：**

1. **DTO 用指针**（`*bool` / `*int`），否则 `{"enabled":false}` 与 `{"dailyLimit":0}` 会被 `binding:"required"` 判成"没传"→ `4001`，**开关永远关不掉**（§1.1 第一层；实测见 `.learn` §4 的 F1/F2 对照）。
2. **校验规则照契约 §9 的表**（`5 ≤ interval ≤ 1440`、`intervalMin < intervalMax`、`1 ≤ dailyLimit ≤ 10`），**越界一律 `4001`**，在 service 层做，**不写进 DTO 的 tag**（跨字段规则写不进 `binding`）。
3. **写入必须走 `UpdateOwned`**（内部是 `map[string]any`）。**禁止** `Model(...).Updates(model.ProactiveSetting{...})`、**禁止** `Save`（`Save` 写回全部列，会把 `last_nudge_at` 一起覆盖成零值）。
4. **`PUT` 包事务**：`UPDATE` + 回读 `SELECT` 要么都看到，要么都不写。**`RowsAffected == 0` → 直接 `4043`**（§1.2），不要再补一次存在性查询。

**一个刻意的"不判断"**：`PUT` **不校验** `personaId` 指向的人设是否存在——`WHERE` 里两个条件已经覆盖了"存在且属于我"，多加一次查询只会多一个出口、多一处漏判。

### 2.3 错误码口径（**契约空白处，本文定，需广播**）

| 情形 | 返回 | 依据 |
|---|---|---|
| `GET` 缺 / 非法 `personaId` | **`4001`** | 契约 §9 的错误码列**只写了 `4043`**——这是**空白**。缺参时没有归属可校验，"查不到 → `4043`"在语义上不成立，**且不能当 `personaId = 0` 查库**（命中空集 → `200` + 空对象，把调用方 bug 伪装成正常态）。先例：`GET /schedules`（契约 §10）同为"必带 `personaId`"的分页端点，错误码列写的是 `4001 4043`；`user-memory` 也按同一口径待广播 |
| `PUT` 的 `4001` | 契约已列 | 字段缺失 / 越界 / 跨字段关系 |
| 任意输入组合 | **不出现 `4030` / `4040`** | `4030` 是功能越权的码、本模块无此场景；`4040` 与 `4043` 并存**会泄漏存在性** |

> 🚧 **阻塞合并项（2026-09-20 用户拍板：按先例补，本支不改）**：契约 §9 的 `GET` 行应由 `4043` 改为 `4001 4043`——
> 缺参时没有归属可校验、不能当 `personaId = 0` 查库，而 `GET /schedules`（同为"必带 `personaId`"）已经是 `4001 4043` 双码。
> 按契约纪律**登记 §12 变更记录 + 群里广播**后由契约 Owner 统一改；**`API_CONTRACT.md` 是全局文件，本支不直接改**。
> 广播草稿见 [plan §6](plan.md)。

---

## 3. 仓储层（**本支实现**）

### 3.1 两个方法

```go
// 读：两个条件都带；未命中返回 gorm.ErrRecordNotFound（不区分"不是你的"与"不存在"）
func (r *ProactiveRepo) Get(ctx context.Context, userID, personaID uint64) (*model.ProactiveSetting, error)

// 写：四个可改列；map[string]any 是硬要求（§1.1）；收调用方的事务句柄；返回 RowsAffected
func (r *ProactiveRepo) UpdateOwned(
	ctx context.Context, tx *gorm.DB, userID, personaID uint64,
	enabled bool, intervalMin, intervalMax, dailyLimit int,
) (int64, error)
```

- 读路径用包级 `r.db`，写路径用调用方传入的 `tx`——与 `persona_repo` / `CreateDefaultSettings` 同一形态。
- `UpdateOwned` 的四个参数是**值类型**：`PUT` 要求四个字段全传，不存在"没传"的中间态（"没传"在 DTO 层就已经被 `4001` 拦住了）。**不要**改成可变参数或 `map` 入口——那会把"能写哪些列"的决定权交给调用方。

### 3.2 允许与禁止的方法形态

| | 形态 | 理由 |
|---|---|---|
| ✅ 允许 | `Get`（双条件） | 唯一的读入口 |
| ✅ 允许 | `UpdateOwned`（双条件 + map） | 唯一的写入口 |
| ✅ 已存在 | `CreateDefaultSettings`（收 `tx`） | PR #28 落地，播种专用 |
| ⛔ 禁止 | `GetByID(id)` / `ExistsByID(id)` | 单条件查询能回答"这个 id 存不存在"，可被顺序试号枚举 |
| ⛔ 禁止 | `ListByUser` / `GetByPersona(personaID)` | 少一个条件就是跨用户串号 |
| ⛔ 禁止 | `Delete*` | 配置随人设级联删除，没有独立删除路径（契约无此端点） |
| ⛔ 禁止 | `Upsert` | §1.4：会掩盖"播种缺失"，且多一个写归属列的口子 |
| ⛔ 禁止 | 任何写 `last_nudge_at` 的通用更新方法 | §1.5：那一列的写入方是触发链路，不是设置页 |

---

## 4. 验收标准

### 分组 A · 本支（仓储层，✅ 已跑，读数见 `.learn/proactive-settings.md` §4）

| # | 项 | 判据 |
|---|---|---|
| A1 | 编译与静态检查 | `gofmt -l internal/` 无输出；`go build ./...` / `go vet ./...` / `go test ./...` 全过 |
| A2 | **正**：`UpdateOwned` 能写零值 | `enabled=false, 0,0,0` → `RowsAffected=1`，**回读 `enabled=false` 且三个 `0` 都在** |
| A3 | **反**：struct 路径确实写不进零值 | 同库 `Updates(struct{Enabled:false, DailyLimit:0})` → `Error=nil`、`RowsAffected=0`、**值一动不动**。这条**必须能读出 0**——读不出 0 说明 A2 的对比不成立 |
| A4 | 越权：`UpdateOwned` 打别人的 persona | `RowsAffected=0`，**对方行一行都没动**（回读比对 `enabled` / `interval_min`） |
| A5 | `Get` 命中 / 未命中 | 自己的 → `err=nil`；别人的 → `ErrRecordNotFound`；不存在的 id → **同一个** `ErrRecordNotFound`（两者不可区分） |
| A6 | 零值 struct 直接 `INSERT` | 落库 `true / 30 / 120 / 3`——证明 `default:` tag 真的会把零值替换掉（E0 读数），**这是 A2/A3 那条陷阱的另一面** |
| A7 | 静态探针（**每条都锚定代码形态**，命令见 [plan §4.3](plan.md)） | 三种形态——`Updates(model.ProactiveSetting{`、`clause.OnConflict` / `.Save(` / `Upsert(`、**带引号的 map 键** `"last_nudge_at"`——都期望 **0**；两个方法体内都能看到 `persona_id = ? AND user_id = ?`（期望 2）。⚠️ **裸词 grep 会误报**：注释里正写着"不用 upsert""`last_nudge_at` 不进 map"，实测三条裸 grep 全部命中**正确**的注释 |
| A8 | `gofmt` 干净 | `gofmt -l internal/` 无输出（**排在最后**：本轮实测第一版就被它拦下一次——注释里的续行被 gofmt 重排成缩进块） |
| A9 | **契约面反验：两条外键都在且都是 `CASCADE`** | 临时库 `\d proactive_settings` 有 **2 条** `ON DELETE CASCADE`（`users` / `personas` 各一条）。本支不动这两个 tag，**只验证"没被改坏"** |
| A10 | 红线 1 自查 | `git status --short` 无 `.env` / `.key` / `.pem`；临时库跑完 `dropdb`，**开发库 `heart_echo` 一行未碰** |

> **A2 与 A3 必须成对**：只跑 A2 证明不了"用 struct 会失败"，只跑 A3 证明不了"用 map 就好了"。
> 这两条是本表**唯一会静默失败**的地方，也是本支存在的理由。

### 分组 B · 交接给接口支（建议 `feature/backend-proactive-api`，**不在本支跑**）

- [ ] `GET`：自己的 → `200` + **恰好 6 个字段**（`jq '.data | keys | length'` = 6，**没有 `id` / `userId`**）
- [ ] `GET` 缺 `personaId` → `4001`（**不是** `200` + 空对象）
- [ ] `GET` 打别人的 `personaId` → `4043`；不存在的 `personaId` → **同码同文案**
- [ ] `PUT {"enabled":false,...}` → `200`，且**直连数据库**看到 `enabled = false`（**不要只看响应体**：响应体回读自库，但 A3 的坑会让"没写进去"也回显成 `false`）
- [ ] `PUT` 只传 `personaId`（四个字段一个不传）→ `4001`；`intervalMin >= intervalMax` → `4001`；`dailyLimit=11` → `4001`
- [ ] `PUT` 打别人的 `personaId` → `4043`，且**对方行回读无变化**
- [ ] 任意输入组合都不出现 `4030` / `4040` / `5003`
- [ ] 无 Token → `4010`（且**不是** `200`）
- [ ] `last_nudge_at` 在 `PUT` 前后不变（**只读列**，§1.5）

### 分组 C · 流程（阻塞合并）

- [ ] 🚧 **【阻塞合并】契约空白已广播**（§2.3 的 `GET` 补 `4001`）：**本支不改全局文件**，广播后由契约 Owner 统一改（2026-09-20 用户拍板：按 `GET /schedules` 先例补）
- [ ] **`Upsert` → `UpdateOwned` 的差异已广播**（§1.4；2026-09-20 用户已接受该改动）：说明 [persona-crud §7.2](../persona-crud/spec.md) 的旧清单**被本文修正**，下游不要照着 `Upsert` 写
- [ ] PR 至少 1 人 Approve；commit 格式 `<type>(<scope>): <subject>`（scope `proactive`）

---

## 5. 交接

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `repo.Get` / `repo.UpdateOwned` | **成员 3（下一支 `-api`）** | 两个端点各自的唯一出入口；签名见 §3.1，**不要另起方法** |
| §2 的接口规格 | 同上 | 绑定规则、指针 DTO、校验分界、错误码——**照写，不要临场发挥** |
| `RowsAffected == 0 → 4043` | 同上 | `PUT` 的越权判定**不需要**额外查询（§1.2） |
| `Settings` 的 6 字段（无 `id` / `userId`） | **成员 2** | 写 `types/proactive.ts` 与 Mock，字段名逐字 camelCase；`lastNudgeAt` 初始 `null` |
| 主动消息设置页的两个交互事实 | **成员 2** | ① 关开关就是 `enabled:false`，服务端**能**存下来（别在 Mock 里假装成功）；② `intervalMin/Max` 是分钟、`dailyLimit` 是条数，**范围校验在服务端**，前端可以拦但不要只靠前端 |
| 契约 §9 `GET` 行需补 `4001`（阻塞合并） | 队长 / 契约负责人 | §2.3 的空白，广播后由契约 Owner 统一改 |

---

## 6. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-20 | v1 | 创建。范围定为**仓储层两个方法**（模型已随 `6fc970e` 落地）；沉淀"零值 == 缺失"的三层同源陷阱（DTO 指针 / 仓储 map / `default:` tag），并把它与 `RowsAffected == 0 → 4043` 的越权判定绑成一体；定 `UpdateOwned` 替代交接清单里的 `Upsert`；登记契约 §9 `GET` 行的 `4001` 空白 | 主动消息模块开工前的基线：这张表**唯一会静默失败**的地方必须先写成合同 |
| 2026-09-20 | v1（同日拍板回填） | **三项用户裁定回填**：① 契约 §9 `GET` 缺 `4001` → **按 `GET /schedules` 先例补，登记为阻塞合并项，本支不改全局文件**（§2.3）；② `model/proactive_setting.go` 的注释密度超标 → **本支不动**，另开 refactor 分支与前 3 张表一起清理（§0.2）；③ `.learn/` 不入库 → 认可，关键结论都在本合同内。另：**`Upsert` → `UpdateOwned` 的改动被接受**，需广播"persona-crud §7.2 旧清单被修正"（§1.4） | 三处拍板落进合同，避免"决议在聊天记录里、文档里查不到" |

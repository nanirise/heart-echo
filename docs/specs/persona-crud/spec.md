# spec · 人设 CRUD（Persona CRUD）· 接口层

> 功能名：persona-crud ｜ 分支：`feature/backend-persona-crud`
> 负责人：成员 3（数据 + 人设 + 部署） ｜ 状态：设计完成，待实现
> 创建：2026-09-16 ｜ 最后更新：2026-09-16（v1）
> 上游文档：[persona-model/spec.md](../persona-model/spec.md)（模型层，已合并）｜ [API_CONTRACT.md §4](../../API_CONTRACT.md)（已冻结）｜ [AGENTS.md](../../../AGENTS.md) ｜ [MASTER.md §4.5](../dev/MASTER.md)
> 关联分支产物：`internal/dto/persona_dto.go`、`internal/repository/persona_repo.go`、`internal/service/persona_service.go`、`internal/handler/persona_handler.go`

> ⚠️ **本文件不是 persona-model/spec.md 的复述。** 模型层已经落地（PR #26 前后），`personas` 表、`migrate.go`、`model.JSONB` 都不在本次范围。
> 本文只做一件事：把 persona-model 的 §4（接口定义）与 §5（越权防线）**落到四个接口层文件上**，并修正那 4 处**已经被现实推翻**的旧结论（§0）。

---

## 0. 与 persona-model/spec.md 的四处修正

写这份文档时逐条核对了仓库现状，发现 persona-model 里有 4 条说法已经过时或过宽。**先列在这里，避免 reviewer 拿着旧文档判我违规。**

| # | persona-model 的旧说法 | 核到的事实（含位置） | 本文怎么处理 |
|---|---|---|---|
| 1 | §5.1 ② / §5.3：仓储层**不提供任何"判断某 id 是否存在"的方法**，`ExistsByID` 探针是反例 | 成员 1 的 [user-memory/plan.md](../user-memory/plan.md) H2 行**依赖** `internal/repository/persona_repo.go` 的 `ExistsOwnedByUser(ctx, userID, personaID)`，且 persona / chat-message / user-memory **三个分支都要改这个文件** | **旧措辞过宽，本文收窄**：被禁的是「**单条件**、只问存在性」的探针；「**双条件**、只回 bool、不区分『不是你的』与『不存在』」的归属判定是**必需的共享闸门**。判据一句话写在第 §5.2 |
| 2 | §5.2 末尾：「契约尚未写入这条规则 —— **走完流程再合 PR**」（7 项待办，含 `API_CONTRACT.md` / `AGENTS.md`） | **已由成员 1 于 2026-09-13 全部完成**：契约 §2 已把 `4031` 划掉（`API_CONTRACT.md:69`）、§4 端点表已是 `4001 4043` / `4043`（`:182-183`）、§4 末尾已写明"不属于本人时按「人设不存在」返回 `4043`"（`:185`）、`4030` 文案已收窄为"无权限使用该功能"（`:68`）、§12 变更记录两条已登记（`:443-444`）；`pkg/errcode/errcode.go` 的常量注释同步 | **闸门已解除**，本文不再把"契约未同步"列为阻塞。`API_CONTRACT.md` / `AGENTS.md` 仍是全局文件，**本分支不碰**（它们已是最新） |
| 3 | plan 步骤 0 / 2：`internal/dto/common_dto.go`（`PageResult[T]`）与 `internal/model/proactive_setting.go` 是本功能前置，状态"未开始" | 两者**至今都不存在**（`internal/` 下只有 `config` / `middleware` / `model`）；`internal/model/migrate.go:18` 的 `&ProactiveSetting{}` **仍是注释行** | 列为本次的**前置步骤**，见 §2.1 与 §7.3 待确认 #1。**注意这超出了任务书给的 4 文件清单**，需拍板 |
| 4 | §5.1 ①：`userID` 用 `c.GetUint64(...)` 读，"取不到（类型断言失败 / `0`）→ 返回 `4010`" | **gin v1.12.0 的 `GetUint64` 没有第二个返回值**：`context.go:366` → `getTyped[uint64]`（`context.go:303`），内部是 `val.(T)` **直接类型断言**，取不到时静默返回零值、**不报错不 panic**。`userID, ok := c.GetUint64(k)` 直接**编译不过**（assignment mismatch） | handler 只能写 `userID := c.GetUint64(...)`，**唯一判据是 `userID == 0`**。连带两个后果：① `JWTAuth` 必须原样传 `uint64`，写成 `int` 会让**每个请求都 4010**；② `middleware/logger.go:49` 的 `c.GetString(ContextKeyUserID)` 对 `uint64` 断言失败 → **访问日志里的 `userId` 永远是空串**（成员 1 的文件，**只报不改**，见 §7.1） |

> **第 4 条是本轮最值得广播的一条**：它同时推翻了 persona-model §5.1 ① 与 user-memory spec/plan 的审查清单（那两处都写着"`c.GetUint64` 取**且检查了 `ok`**"）。按那两份文档写，代码根本编不过；而"顺手改成 `userID, _ :=`"同样编不过——所以它会**强制**暴露，属于好运。真正危险的是 §7.1 里那条**静默**的（`c.Set` 传了 `int`）。

---

## 1. 背景与目标

`personas` 表既是人设存储，**同时就是对话列表**（无会话表）。persona-model 已经把表建好了，但**没有任何 HTTP 端点能碰到它**——`internal/handler/`、`internal/service/`、`internal/repository/`、`internal/dto/` 四个目录**至今都不存在**。

所以本功能是**全项目第一个落地的接口层模块**，它同时承担两个职责：

1. 交付契约 §4 的 4 个端点；
2. **把分层写法的样板立起来**——成员 1 的 `memory_*` / `chat_*`、成员 3 后续的 `moment_*` / `proactive_*` / `schedule_*` 都会照着这 4 个文件写。

**目标（一句话）**：交付 4 个端点，字段与契约 §4 逐字一致；**任何输入组合都不可能读到、也不可能探测到别人的数据**；并让"越权 = `4043`"这条规则在代码里有唯一且可 grep 的落点。

**为什么它在关键路径上**：成员 2 的聊天页靠 `GET /personas` 决定打开哪个对话；`user_memory` / `user_profile` / `proactive_settings` / `schedules` / `chat_messages` 全都挂在 `persona_id` 上，而 `persona_repo.ExistsOwnedByUser` 是它们**共同的第一道闸门**——这里漏了，后面每张子表都跟着漏。

---

## 2. 范围

### 2.1 做什么（In Scope）

**本 PR 的完整交付范围 = 下列 6 个文件**（4 个核心 + 2 个前置，**6 个全部在本 PR 内**）。

> 📌 **范围说明（2026-09-16 用户裁定）**：任务书只点了 4 个文件，但 `GET /personas` 与 `POST /personas` 硬依赖另外 2 个尚不存在的文件。用户裁定**一并纳入本 PR**，理由：`internal/dto/**` 与 `internal/model/**` 本来就属成员 3 的职责范围（10 张表设计 + 分页结构），**不是越界**；缺了它们，`GET /personas` 的返回类型与契约 §4 不符，主动消息模块上线时会撞上孤儿配置。**PR 描述里要写明这次扩大范围的理由**（草稿见 plan §6）。

**核心交付（任务书给的 4 个文件）：**

| # | 产物 | 作用 | 谁会用到 |
|---|---|---|---|
| 1 | `internal/repository/persona_repo.go` | 人设数据访问；**每个方法签名强制带 `userID`** | 成员 1（`ExistsOwnedByUser`）、成员 3（主动消息读 `last_message_at`） |
| 2 | `internal/service/persona_service.go` | 归属判定 + 事务编排 + DTO 转换 | 成员 3（后续模块照抄这个分层写法） |
| 3 | `internal/handler/persona_handler.go` | 4 个端点 + `RegisterPersonaRoutes` | 成员 1（`router.go` 挂一行） |
| 4 | `internal/dto/persona_dto.go` | 请求 / 响应结构；`familiarity` 拍平 | 成员 2（据此写 `types/persona.ts` 与 Mock） |

**前置文件（硬依赖，已裁定纳入本 PR）：**

| # | 产物 | 为什么绕不过去 |
|---|---|---|
| 5 | `internal/dto/common_dto.go` | `GET /personas` 的响应是 `PageResult<Persona>`。它是**五个分页端点共用的一份**（persona / chat-message / user-memory / moments / schedules），persona-model §4.1 已定位置；user-memory spec 明确记着"**尚未落地**，谁先合谁建，后来者消费，**不要重建**"。本分支是**最先落地的那个** |
| 6 | `internal/model/proactive_setting.go` + `migrate.go` 追加一行 | `POST /personas` **必须同事务播种一行 `proactive_settings`**（persona-model §4.3.1 已定，理由是消灭"某伴侣设置页打不开"的孤儿）。而 `model.ProactiveSetting` 现在不存在，`migrate.go:18` 那行还是注释 |

**不属于本 PR 的相邻文件（明确排除）：** `router.go`（成员 1 加一行）、`middleware/**`、`pkg/**`、`personas` 表与 `persona.go`。

> **不额外动 `router.go`**：`RegisterPersonaRoutes` 由本分支写好，`router.go` 那一行**由成员 1 加**（MASTER §4.5 冻结：两人错开，不同时改这个文件）。本分支只负责把注册函数交出去。

### 2.2 不做什么（Out of Scope）

| 不做的事 | 原因 |
|---|---|
| 改 `personas` 表 / `persona.go` / `jsonb.go` | 模型层已合并，本功能零 schema 改动 |
| 改 `pkg/errcode`、`pkg/response`、`internal/middleware` | 成员 1 的文件，且**契约已同步到最新**（§0 第 2 条），没有要补的 |
| 实现 `JWTAuth` | 成员 1 的文件，**目前是空壳**（见 §7.1）。本分支撞上它但要等它 |
| 改 `middleware/logger.go:49` 的 `userId` 日志 bug | 成员 1 的文件。**只报不改**（§7.1） |
| `familiarity` 的累加 | 归对话链路（每轮 +1）。本功能**只读展示**，不写 |
| 人设数量上限 / 人设名去重 | 总纲 §0.3：数量不限制；DDL 上 `name` 无 UNIQUE，**重名合法**，不要自作主张加 `4004` |
| `state` 的解析与改写 | 只有两处例外：创建时初始化 `{"familiarity":0}`、读时**只取** `familiarity`。其余一律原样透传 |
| 主动消息配置的读写端点 | `GET/PUT /proactive/settings` 是后续模块。**本次只在创建人设时播种一行**，不实现读写 |
| 会话/对话相关端点 | 没有会话实体，对话列表就是 `GET /personas` |
| 新增错误码 | **一个都不新增**，只用 `4001` / `4010` / `4043` / `5003` |
| 手写 `ALTER TABLE` / `DropTable` | 红线 8 |
| `user_profile.go` 缺失外键的修复 | **已发现，属另一条线**（见 §7.3 待确认 #5），本功能不顺手改 |

---

## 3. 数据模型

### 3.1 `personas`（已落地，只引用）

字段、tag、索引、外键全部在 [persona-model/spec.md §3](../persona-model/spec.md) 与 `internal/model/persona.go`。本功能**只读不改**，此处不重复。

三条与本功能直接相关的既有事实（实测，写代码时按这个来）：

1. `persona.go:58` 有 `User User \`gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"` → `personas.user_id` 的外键**会**被 `AutoMigrate` 建出来。
2. `LastMessageAt *time.Time` 是**可空指针**（契约要求"从未聊过为 `null`"），排序时 Postgres 的 `DESC` 默认 `NULLS FIRST`，**查询侧必须显式补 `NULLS LAST`**。
3. `Name` 上**没有** UNIQUE（`grep -c` 确认 tag 里无 `unique`）→ 重名是合法数据，不要加冲突校验。

### 3.2 `proactive_settings`（本次新增的前置模型）

**权威 DDL（技术文档 §6.2，照抄待翻译）**

```sql
CREATE TABLE proactive_settings (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id   BIGINT  NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    enabled      BOOLEAN NOT NULL DEFAULT TRUE,
    interval_min INT     NOT NULL DEFAULT 30,      -- 分钟
    interval_max INT     NOT NULL DEFAULT 120,
    daily_limit  INT     NOT NULL DEFAULT 3,
    last_nudge_at TIMESTAMPTZ,
    UNIQUE (user_id, persona_id)
);
```

**字段对照表（DDL ↔ GORM ↔ 说明）**

| DDL 列 | Go 类型 | GORM tag 要点 | 说明 |
|---|---|---|---|
| `id BIGSERIAL PK` | `uint64` | `type:bigint;primaryKey;autoIncrement` | **不是 `type:bigserial`**：`Persona.ID` / `User.ID` 已是 `bigint`，这里照写即可；源头切断复制污染的做法**不要回退** |
| `user_id BIGINT NOT NULL` | `uint64` | `type:bigint;not null` + 组合唯一索引首列 | 越权防线载体 |
| `persona_id BIGINT NOT NULL` | `uint64` | `type:bigint;not null` + 组合唯一索引次列 | 外键指向 `personas(id)` |
| `enabled BOOLEAN NOT NULL DEFAULT TRUE` | `bool` | `type:boolean;not null;default:true` | — |
| `interval_min/max INT NOT NULL` | `int` | ⛔ `type:integer;not null;default:30` / `default:120` | 单位分钟。**不能写 `type:int`**，见结论 5 |
| `daily_limit INT NOT NULL` | `int` | ⛔ `type:integer;not null;default:3` | 每日上限。**不能写 `type:int`**，见结论 5 |
| `last_nudge_at TIMESTAMPTZ`（可空） | `*time.Time` | `type:timestamptz` | **必须指针**，否则"从未触发过"会变成 `0001-01-01` |
| `UNIQUE (user_id, persona_id)` | — | `uniqueIndex:uq_proactive_settings_user_persona,priority:1/2` | 见下 |

**四条必须记住的结论：**

1. **组合唯一约束用 `uniqueIndex` 表达，落点在 `\d` 的 `Indexes` 段，不是 `Constraints` 段。**
   这不是细节洁癖：`user_profile` 那次实测已经确认——GORM 的 `uniqueIndex` tag 生成的是 `CREATE UNIQUE INDEX`，`\d` 里显示在 **Indexes** 段；写 `unique` tag 才会生成 `ALTER TABLE ... ADD CONSTRAINT`。**验收 grep 若去 Constraints 段找，会在正确实现上判出假失败**（user-profile spec 的 A 组就栽过一次，已改）。
   落点：`UserID` 写 `...;uniqueIndex:uq_proactive_settings_user_persona,priority:1`，`PersonaID` 写 `...;uniqueIndex:uq_proactive_settings_user_persona,priority:2`。**两个字段的索引名必须逐字相同**，写错一个字母会**静默建出两个单列唯一索引**（结果：同一 persona 能有多个 user 的配置行？不会——单列 unique 反而更严，会让第二个用户彻底建不出人设配置）。这是一个"看起来生效、实际语义完全不同"的错法。

2. **两个关联字段都要声明，否则外键一个都不建。**
   ```go
   // 仅供 GORM 生成 ON DELETE CASCADE 外键用；不参与序列化
   User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
   Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
   ```
   写法与 `chat_message.go:99-100`、`user_memory.go:132-133` 完全一致（这两处是全项目的参照）。
   漏掉 `Persona` 关联的后果不是"报错"，是**删人设时留下一行孤儿配置**——正是 persona-model §4.3.1 要消灭的那个问题；而且它在演示现场表现为"某个伴侣的设置页打不开"，**不报错、只失灵**。
   ⚠️ **`Create` 时不要给这两个字段赋值**（保持零值）。GORM 对 belongs-to 关联会尝试连带写入被引用表；赋了值就可能出现"插一行配置顺手 upsert 了一次 users/personas"的行为。

3. ⛔ **`default:` 会改写零值——`enabled` 与三个 `INT` 列不能用 struct 写 `0` / `false`。**
   persona-model 时代的说法是"写 `0`/`false` 会原样进库（不会被 DB 默认值救回来）"。**2026-09-16 实测推翻了它**（读数见 §8.6）：GORM 把 `default` tag 解析成 Go 值（`schema/field.go` 的 `DefaultValueInterface`），**在该字段为零值时把它替换进去**——列照常出现在 INSERT 里，但绑的是 tag 值、不是零值。四条路径的实测结果：

   | 写法 | 实测结果 |
   |---|---|
   | `Create(&ProactiveSetting{Enabled:false, IntervalMin:0, …})` | 入库 **`true / 30 / 120 / 3`** —— 显式写的 `false` 被静默改成 `true` |
   | `Create(map[string]any{…, "daily_limit": 0})` | 按 map 原样写，**`0` 能写进去** |
   | `Updates(model.ProactiveSetting{Enabled:false})` | **`Error=nil`、`RowsAffected=0`**、那一行纹丝不动 —— **静默空操作**（零值字段被整条丢弃，SET 列表为空） |
   | `Updates(map[string]any{"enabled": false})` | 正常生效（`RowsAffected=1`） |

   三条结论：
   - **要写 `false` / `0`，一律用 `map`。** struct 路径写不进去；`Updates(struct)` 连错都不报，只查 `err != nil` 的调用方会**以为关掉了**——正是本项目反复出现的那类"不报错、只失灵"。
   - 播种处显式写 `true/30/120/3` **是显式可读，不是正确性所必需**：留零值入库的也是这组值。**保留它，但别把它写成"不写就错"。**
   - `default` tag **对 INSERT 有实质影响**，不只是 DDL 声明。`Save` 走全列回写、**不走**这个替换（实测能把 `false` 写进去），但它会覆盖全部列——`persona_repo.go` 已写明本模块为什么禁用 `Save`。
   - ⚠️ **对后续的 `PUT /proactive/settings`（成员 3）是硬约束**：那个端点的核心动作就是"关掉主动消息"，用 struct 写会静默失败。已进 §7.2 广播清单。

4. **表名 `proactive_settings` 是复数，GORM 默认复数化恰好一致**，但按项目习惯（`persona.go` / `user_memory.go` 都显式写了 `TableName()`）**显式返回**，避免将来改名或配置变化导致静默建错表。

5. ⛔ **三个 `INT` 列必须写 `type:integer`，写 `type:int` 会建出 `bigint`。**
   这条是**代码落地当天实测发现的真缺陷**（编译过、`vet` 过、任何 grep 都查不出，只有读 `\d` 才看得见）；当时的实现写的是 `type:int`，`\d proactive_settings` 里 `interval_min` / `interval_max` / `daily_limit` 三列是 **`bigint`**，与 DDL 的 `INT` 不符。链条如下（源码位置已逐条核对）：

   - `schema.Int == "int"`（`schema/field.go` 的常量），`schema.Uint == "uint"`；
   - 该文件里 `TYPE` tag 是**最后**才被处理的：`lowerVal = strings.ToLower(val)` 后 `switch lowerVal { case Bool, Int, Uint, Float, String, Time, Bytes: field.DataType = lowerVal; default: field.DataType = DataType(val) }`。所以 `type:int` **不落到 `default:` 分支**，而是把 `field.DataType` 设为常量 `"int"`；
   - Postgres dialector 的 `getSchemaBaseType` 对 `case schema.Int, schema.Uint:` 调 `getSchemaIntType(field)`，而它 **switch 的是 `field.Size`** —— `≤16` → `smallint`，`≤32` → `integer`，否则 → `bigint`。**完全无视那个 tag**；
   - Go 的 `int` 在 64 位平台 `Size = 64` → 建出 **`bigint`**。
   - 写 `type:integer` 为什么对：`"integer"` 不等于任何一个常量，落进 **`default:`** 分支 → `field.DataType = DataType("integer")` → 原样透传成 `integer`。

   **教训（比这条结论本身更重要）**：GORM 的 `type:` tag **不是万能的逃生舱**。有一小撮值与 GORM 的内部常量同名（`int` / `uint` / `bool` / `float` / `string` / `time` / `bytes`），撞上就会**改走 dialector 的类型推导**、把 tag 的语义整个吞掉。写 tag 时若拿不准，**宁可写成数据库的原生类型名**（`integer` / `boolean` / `timestamptz`），它们不在那张常量表里，永远是透传。
   **为什么不能用"反正只是宽一点"放过**：`bigint` 与 `INT` 的存储、比较、索引行为都不同，属于**与人无感的静默 schema 漂移**；而且它会让 `\d` 实测（A2）与 DDL 逐列对不上，把一条本该生效的验收检查变成假失败。

### 3.3 本次不新增任何索引

`personas` 的索引（`idx_personas_user_last_msg`）已存在。`proactive_settings` 只靠 `UNIQUE (user_id, persona_id)` 自带的索引——它同时服务"播种后按 persona 查"这一类查询。**不要顺手加索引**：每人设 3-5 行，加了是纯负担。

---

## 4. 接口定义

### 4.1 通用约定（契约 §1 / §2）

| 项 | 约定 |
|---|---|
| Base URL | `/api/v1` |
| 鉴权 | 4 个端点**全部需要** `Authorization: Bearer <accessToken>`；缺失/过期 → `4010`。**不在免鉴权白名单里** |
| 响应体 | `{ code, message, data, timestamp }`，由成员 1 的 `pkg/response` 统一产出 |
| 分页 | `page` 从 1 开始；`pageSize` 默认 20、上限 100 |
| 分页响应 | `{ list, total, page, pageSize }`，类型 `PageResult[T]`，**定义在 `internal/dto/common_dto.go`** |
| 时间格式 | RFC3339 |
| 错误出口 | handler 一律 `_ = c.Error(err)` 上抛，由 `BizErrorHandler` 中间件统一出口；**handler 里不出现 `response.Fail`** |
| 路由注册 | `RegisterPersonaRoutes(rg *gin.RouterGroup, h *PersonaHandler)`，签名照 MASTER §4.5 冻结的那份写 |

**`PageResult[T]` 的形状（本次率先落地，冻结给成员 1 消费）**

```go
// internal/dto/common_dto.go —— 只放这一个类型 + 分页常量 + ClampPage，不放别的东西
const (
    DefaultPage     = 1
    DefaultPageSize = 20
    MaxPageSize     = 100
)

type PageResult[T any] struct {
    List     []T   `json:"list"`
    Total    int64 `json:"total"`
    Page     int   `json:"page"`
    PageSize int   `json:"pageSize"`
}

// ClampPage 把分页参数收敛到契约允许的范围：page<1 → 1；pageSize<1 → 20；pageSize>100 → 100
func ClampPage(page, pageSize int) (int, int)
```

> **`ClampPage` 是"收敛"不是"报错"**：契约写的是"默认 20、上限 100"，`pageSize=1000` 应当被夹到 100，**不是**返回 `4001`。同理 `page=0`（查询串里没传）→ 夹到 1。分页参数**不产生任何错误码**——契约 §4 的 `GET /personas` 错误码列是空的（只有中间件产生的 `4010`）。把它写成 `4001` 是契约漂移。

**Persona 响应结构（契约 §4 冻结，逐字）**

```json
{
  "id": 1,
  "name": "小暖",
  "personalityDesc": "温柔、耐心，喜欢倾听",
  "speakingStyle": "语气轻柔，偶尔用颜文字",
  "state": {},
  "familiarity": 12,
  "lastMessageAt": "2026-09-10T14:30:00+08:00",
  "createdAt": "2026-09-10T14:00:00+08:00"
}
```

### 4.2 `GET /personas` — 人设列表（同时就是对话列表）

| 项 | 内容 |
|---|---|
| 请求 | `page`、`pageSize`（query，均可选） |
| 响应 data | `PageResult<PersonaResponse>` |
| 错误码 | **无业务错误码**（`4010` 由中间件产生） |

**关于"越权"：列表端点不存在越权场景。** 它不带任何 `:id` 或 `personaId` 参数，`user_id` 只来自 Token，所以查询天然被限制在当前用户名下——**没有任何"访问别人的列表"这种输入存在**。因此它**不返回任何 403/404 族错误码，空就是空**：

```json
{ "code": 200, "message": "success",
  "data": { "list": [], "total": 0, "page": 1, "pageSize": 20 }, "timestamp": 1789000000000 }
```

> **不要**给列表加"查不到就 4043"的逻辑：**没有人设是正常状态**（总纲 §0.3 要求前端做空状态引导），返回错误码会让前端无法区分"我还没有人设"和"请求失败"。

**五条容易写错的要求：**

1. **排序必须显式带 `NULLS LAST`**：
   ```sql
   ORDER BY last_message_at DESC NULLS LAST, id DESC
   ```
   Postgres 的规则是「NULL 比任何非 NULL 值都大」，所以 **`DESC` 的默认行为是 `NULLS FIRST`**——只写 `DESC` 的话，**刚创建、还没聊过的人设会排在最前面**，而需求恰恰相反（DDL 里索引也写的是 `NULLS LAST`，这是权威意图）。写错了页面看着"能用"，只是顺序反了。
2. **必须补 `id DESC` 兜底**：大量人设的 `last_message_at` 都是 `NULL`，只按它排序时同值行顺序不确定——翻页会出现「第 2 页冒出第 1 页的人」或漏项。
3. **`total` 必须用同一个 `user_id` 条件统计**，不能 `COUNT(*)` 全表；且 **`Count` 与 `Find` 的 WHERE 子句必须逐字相同**（写两遍，很容易只改一处）。
4. **空列表要返回 `[]` 而不是 `null`**：Go 的 nil slice 会序列化成 `null`，前端 `v-for` 直接崩。切片必须初始化（`make([]PersonaResponse, 0)`）。
5. **`page` 超出范围**时返回空列表 + **真实 `total`**，仍是 `200`。不要为了"更友好"改成 404。

### 4.3 `POST /personas` — 创建人设

| 项 | 内容 |
|---|---|
| 请求 | `{name, personalityDesc, speakingStyle}`，**三个都必填** |
| 响应 data | 新建的 `PersonaResponse` |
| 错误码 | `4001` |

**要求：**

1. **`user_id` 只能来自 Token**，绝不从 body 取。请求 DTO 里**连这个字段都不声明**——字段不存在，就没有被前端传进来的可能（结构性防御，比"记得校验"可靠）。
2. **`state` 显式初始化为 `{"familiarity":0}`**，不要依赖 DDL 的 `DEFAULT '{}'`。理由：读者可以无条件假设 `familiarity` 键存在，少一个分支（读侧仍要容错，见 §4.6）。
3. **不做重名校验**：`name` 无 UNIQUE，人设数量不限制（总纲 §0.3），**重名合法**。
4. **校验失败的语义**：契约 §4 这组端点只列了 `4001`，所以 **`binding` 失败一律 `4001`**。**不要**因为"缺必填字段"就改成 `4002`——那是契约里别的端点的语义，这里擅自细化就是契约漂移。
5. **必须在同一个事务里同步播种一行 `proactive_settings`**（§4.3.1），四个值取 DDL 默认值。
6. 响应里 `familiarity` 为 `0`、`lastMessageAt` 为 `null`（新建的人设必然没聊过）。**这两个值来自刚插入的结构体本身**，不需要回读（`createdAt` 由 GORM 的 `autoCreateTime` 回填，`id` 由 `BIGSERIAL` 回填，都是真值）。

#### 4.3.1 创建人设时播种 `proactive_settings`

**行为**：`POST /personas` 成功时，`proactive_settings` 表**必须存在恰好一行**对应该人设的配置：

| 列 | 值 | 来源 |
|---|---|---|
| `user_id` / `persona_id` | 同新建的人设 | — |
| `enabled` | `true` | DDL 默认 |
| `interval_min` | `30` | DDL 默认 |
| `interval_max` | `120` | DDL 默认 |
| `daily_limit` | `3` | DDL 默认 |
| `last_nudge_at` | `NULL` | 从未触发过 |

**为什么必须播种**：主动消息模块靠 `WHERE persona_id = ?` 读这张表。创建若不留行，该人设的 `GET /proactive/settings` 就会查不到 → **报 404**，而这个 404 在演示现场表现为"某个 AI 伴侣的主动消息设置打不开"。**用"创建时播种"把问题消灭在源头**。

**三条实现约束：**

1. **同一个事务**：人设与配置行要么都成功、要么都不写入。事务边界在 **service 层**（红线 7），repo 方法接受事务句柄。
2. **顺序不能反**：必须先 `Create` 人设、**拿到回填的 `p.ID`**，再播种。
3. **用 `INSERT` 而不是 upsert**：唯一索引已在，重复插入就该报错——**不要**写成 upsert 去掩盖"重复创建"的 bug。

### 4.4 `PUT /personas/:id` — 编辑人设

| 项 | 内容 |
|---|---|
| 请求 | `{name, personalityDesc, speakingStyle}`，**同 POST，三个都必填**（整体替换，不是部分更新） |
| 响应 data | 更新后的 `PersonaResponse` |
| 错误码 | `4001` **`4043`**（资源越权 / 不存在，**同一个码**，见 §5.2） |

**要求：**

1. **这是 PUT 不是 PATCH**：三个字段全传、全必填。**不要参照 `PUT /user/profile` 的"只传要改的"来写**——那是另一个端点的约定。
2. **只更新这三列，别的一律不许动**：`state`、`last_message_at`、`created_at`、`user_id` 全部保持原值。**`proactive_settings` 也不动**（改人设名字与主动消息配置无关）。
3. **`Updates` 传 `map[string]any`，不要传 struct**：传 struct 时 GORM 会**忽略零值字段**，`name` 传空串会被静默跳过。虽然 `required` 在校验层拦住了空串，但**别让这一层的正确性依赖另一层**。
4. **必须用 `Updates`，绝不能用 `Save`**：`Save()` 写回全部列，会把 `state` 覆盖成零值、把 `last_message_at` 清空——**数据损坏且不可逆**。
5. **响应里的 `state` / `familiarity` / `lastMessageAt` 必须是数据库里的真值**，不是请求体里的（请求体根本没有这些字段）。所以更新后要**重新读一次**该行再返回，不要手工拼装（拼装容易漏）。
6. **客户端多传的 `state` / `familiarity` 应被静默忽略**：Gin 默认忽略未知 JSON 字段，正合契约「`PUT` 时忽略」的要求。**不要**在 DTO 里声明这些字段，**也不要**开 `DisallowUnknownFields()`（开了会让"多传字段"从"忽略"变成 `4001`，与契约不符）。
7. **`RowsAffected == 0` 是唯一判 `4043` 的依据**，它的含义是「WHERE 没匹配到」——**不是**「值没变」。同值更新会返回 `RowsAffected = 1`（Postgres 的 `UPDATE` 计数的是匹配行，不比较内容），所以重复提交同样的内容仍然是 `200`，这是正确的。这条要实跑验证（A 组），因为它是 `4043` 判定的基石。

### 4.5 `DELETE /personas/:id` — 删除人设

| 项 | 内容 |
|---|---|
| 请求 | 无 body |
| 响应 data | `null` |
| 错误码 | **`4043`**（资源越权 / 不存在，**同一个码**，见 §5.2） |

**要求：**

1. **硬删除**（DDL 无 `deleted_at`）。**注意别和 `schedules` 的软删搞混**——日程的 `DELETE` 是置 `status='cancelled'`，人设不是。
2. **级联交给数据库外键**，**不要手写多表删除逻辑**。级联范围：`chat_messages`、`user_memory`、`user_profile`、`ai_moments`（→ 再级联 `moment_comments` / `moment_likes`）、**`proactive_settings`**、`schedules`。
3. **幂等性**：删第二次返回 `4043`（与"不存在"同一个码，天然幂等且不泄漏）。前端二次确认后调用，重复提交的第二次是 `4043` 而不是 `200`——按契约实现即可，**不要为它特判**。
4. **`data` 必须是 `null`**：写 `response.Success(c, nil)` **编译不过**（泛型 `T` 无法从 untyped nil 推断），必须显式指定类型参数：`response.Success[any](c, nil)`。

### 4.6 请求 / 响应结构（`persona_dto.go`）

| 结构 | 字段（JSON tag 逐字） | 校验 |
|---|---|---|
| `CreatePersonaRequest` | `name` `personalityDesc` `speakingStyle` | 三者 `binding:"required"`；`name` `max=50`、`speakingStyle` `max=255`（对齐 DDL）；`personalityDesc` 加防御性上限（`max=2000`，DDL 是 `TEXT` 无上限） |
| `UpdatePersonaRequest` | 同上 | 同上。**可以是同一个类型，也可以另起一个**；若复用，注意将来两者校验规则一旦分叉会互相影响 |
| `PersonaResponse` | `id` `name` `personalityDesc` `speakingStyle` `state` `familiarity` `lastMessageAt` `createdAt` | — |
| `NewPersonaResponse(p *model.Persona) PersonaResponse` | 转换函数 | `familiarity` 拍平在此 |

**三条约束：**

1. **DTO 里绝对不声明 `userId` / `user_id`**（任何名字都不行）。同样不声明 `liked` 之类派生字段——本功能没有。
2. **`state` 直接用 `model.JSONB` 透传，不要转成 `map[string]any` / `any` / `string`**：
   - 转 `map[string]any` 会经过一次 `json.Unmarshal` + `Marshal`，大整数会变 `float64`（精度隐患），且**违背契约"原样透传"的要求**；
   - 转 `string` 会序列化成 JSON 字符串而不是对象；
   - `model.JSONB` 自带 `MarshalJSON`（这是它实现里刻意补上的），响应体里就是 `"state":{}` 而不是 base64。
3. **`familiarity` 的提取必须容错**（定义在 dto 层，不写在 model 上）：

```go
// 只解需要的键；其余未知键（如将来的 self_note）留在 State 原文里，不受影响
var s struct {
    Familiarity int `json:"familiarity"`
}
_ = json.Unmarshal(p.State, &s)   // 解析失败或键不存在 → 取 0，不报错
```

**为什么容错**：DDL 默认值是 `'{}'`（键不存在），手工插入的历史数据、或将来别的模块写入的 `state` 都可能没有这个键。**不要让整个列表接口因此 500**。语义上 `0` = "初识"，是合理缺省。

> ⚠️ **契约文档的一处不一致**（是文档瑕疵，不是行为变更）：§4 的示例同时给出 `"state": {}` 与 `"familiarity": 12`，看起来像 `familiarity` 是独立列。但 DDL 注释与技术文档 §5.6 都指明它存在 `state` 里。**以 DDL 为准**；实现后提醒成员 2 一句，免得他写 Mock 时按"独立列"理解。

### 4.7 异常输入的行为（**契约空白处，本文定，需广播**）

契约 §4 没有规定这些输入，但代码必须对它们有确定行为。逐条定死，免得四个人写出四种：

| 输入 | 行为 | 理由 |
|---|---|---|
| `PUT`/`DELETE /personas/abc`（`:id` 非数字） | **`4043`** | `DELETE` 的契约错误码集合里**只有 `4043`**、没有 `4001`；若这里返回 `4001` 就是契约漂移。且保持「`:id` 未命中只有一个答案」这条不变式不被破口——一个解析不出数字的 id，其"存在性"问题本身就不成立。**代价（诚实记录）**：`/personas/1.0` 这种手滑会显示"人设不存在"，略绕。 |
| `PUT`/`DELETE /personas/0` | **`4043`（自然落到查询未命中）** | `id` 是 `BIGSERIAL` 从 1 起，`0` 永远不可能是合法行。**不需要特判**：`WHERE id = 0 AND user_id = ?` 本来就查不到。**不要**为它加 `if id == 0 { return 4001 }`。 |
| `?page=0` / `?pageSize=0` / `?pageSize=1000` | **夹到合法范围，`200`** | 见 §4.1 的 `ClampPage`。分页参数不产生错误码 |
| `?page=99999`（超界） | **`200` + 空列表 + 真实 `total`** | 契约未规定，按"空就是空"处理 |
| body 里多传 `state` / `familiarity` / `userId` | **静默忽略，`200`** | Gin 默认行为，正合契约 |
| `POST` body 是空 `{}` / 缺字段 / `name` 超 50 字 | **`4001`** | §4.3 第 4 条 |
| 无 Token / Token 无效 | **`4010`**（由中间件产生） | 不在白名单 |
| 任何输入组合 | **永不出现 `4030`** | 本模块没有功能越权场景（§5.3） |

---

## 5. 越权防线（本功能最重要的正确性约束）

> 对应红线 3：**❌ 跨用户 / 跨人设数据泄漏**。人设是这套防线的主闸门——这里失守，`user_memory` / `user_profile` / `schedules` / `chat_messages` 全部跟着失守，因为它们都是拿 `persona_id` 去查的。

### 5.1 四层防线

**① 入口层（Handler）—— `user_id` 只有一个来源**

- `user_id` 只能从 JWT 声明取（成员 1 的 `JWTAuth` 写入 `gin.Context`），**永不从 body / query / path / header 读**。
- **gin v1.12 的取法与旧文档不同**（§0 第 4 条）：`GetUint64` **只返回一个值**，没有 `ok`。
  ```go
  // 4 个方法共用这一个 file-local 辅助函数，只写一处
  func currentUserID(c *gin.Context) (uint64, bool) {
      userID := c.GetUint64(middleware.ContextKeyUserID)
      return userID, userID != 0
  }
  ```
  **唯一判据是 `userID == 0`**：`BIGSERIAL` 从 1 起，`0` 不可能是合法用户 → 取到 `0` 就是"没取到"。取不到 → `4010` 并中断，**绝不退化成 `userID = 0` 继续查**。因为 `WHERE user_id = 0` 会返回空集——**看起来没泄漏，实际是把鉴权失败伪装成了空列表**，这种"静默降级"比报错危险得多。
- **只有这一个地方读 `ContextKeyUserID`**：4 个方法各自写一遍 `c.GetUint64(...)`，就有 4 个地方可能写错。抽成一个函数后，grep 一处即可审计完。
- **请求 DTO 里不声明 `userId` 字段**：字段不存在就无法被传入。

**② 服务层（Service）—— 归属判定在这里，`4043` 从这里返回**

- 未命中一律 `errcode.New(errcode.ErrPersonaNotFound)`（`4043`），**不区分"不属于你"与"不存在"**。
- `userID == 0` 在 service 里**再拦一次**返回 `4010`。这不是重复：service 将来会被非 HTTP 路径调用（定时任务、SSE 收尾），那时 handler 那层根本不存在。**纵深防御，不是冗余**。
- `personaID == 0` **不特判**（§4.7）。
- 错误一律 `errcode.New` / `errcode.Wrap`，**不拼接自定义文案**（红线 6）。

**③ 仓储层（Repository）—— 让"只按 id 查"这件事在结构上不可用**

- 每个方法的签名**强制**带 `userID uint64`：调用方没有"忘了传"的选项。
- 归属条件**下沉到 SQL 的 `WHERE` 里**，而不是"先 `First(&p, id)` 查出来、再在 Go 里 `if p.UserID != userID`"。后者有两个问题：多一次往返；以及**忘写那个 `if` 就静默越权**——这类 bug 在 code review 里也容易被漏掉，因为它看起来"逻辑完整"。
- 事务句柄作为显式参数传入（`tx *gorm.DB`），事务边界由 service 决定。

**④ 数据层（DB）**

- `personas.user_id` 的外键（已建）+ 组合索引。
- 下游子表查询一律 `persona_id + user_id` **双条件**：只带 `persona_id` 会跨用户串号（它是全局自增，别的用户的同号人设会命中），只带 `user_id` 会跨人设。

### 5.2 `persona_repo` 允许与禁止的方法形态（**本节是 §0 第 1 条的裁决**）

persona-model §5.1 ② 写的是"不提供任何『按 id 单查』或『判断某 id 是否存在』的方法"，把话说满了。但成员 1 的 memory 模块**确实需要**一个归属闸门，而"存在性"这三个字被一刀切禁掉之后，reviewer 会拿它当违规。**把判据写清楚，比继续沿用宽泛措辞有用。**

**判据一句话：问「这个 id 存不存在」的禁止；问「这个 id 是不是我的」的允许。**

区别在于**答案集合**：

| # | 形态 | 形状 | 判定 | 理由 |
|---|---|---|---|---|
| ① | **融合条件**（写路径首选） | `Where("id = ? AND user_id = ?", ...).Updates(...)` → 看 `RowsAffected` | ✅ | 一次查询；归属条件在 SQL 里，"忘写 if"这种错法**不存在** |
| ② | **双条件读**（读路径） | `Where("id = ? AND user_id = ?", ...).First(&p)` | ✅ | 返回的是**你自己**的行；对别人的 id 与不存在的 id 都给 `ErrRecordNotFound`，**不可区分** |
| ③ | **双条件存在性**（跨模块共享闸门） | `Where("id = ? AND user_id = ?", ...)` → `count > 0`，**只回 `bool`** | ✅ | 对任意输入，答案只有"是我的 / 不是我的"。别人的资源与不存在的资源返回**同一个 `false`**——攻击者拿不到任何区分信息 |
| ④ | **单条件存在性** | `Where("id = ?", ...)` → `bool` | ❌ **禁止** | 它是**唯一**能回答"这个 id 到底存不存在"的形状。`id` 是全局自增，调用方可以逐号试出系统里共有多少人设 |

三种被允许的形态**都不泄漏任何东西**，因为它们的答案对"别人的 id"与"不存在的 id"完全一致。persona-model §5.3 反例表里那条"为区分而写 `ExistsByID` 探针"针对的是**形态 ④**；本文把它改写成：

> ❌ `ExistsByID(id uint64) (bool, error)` —— 单条件，只问存在性。
> ✅ `ExistsOwnedByUser(userID, personaID uint64) (bool, error)` —— 双条件，只问归属。

**本次交付的方法清单（冻结，成员 1 与另外两个分支按这个签名引）**

```go
// 读路径（不开事务）
ListByUser(ctx context.Context, userID uint64, offset, limit int) ([]model.Persona, int64, error)
FindOwned(ctx context.Context, userID, personaID uint64) (*model.Persona, error)      // 未命中 → gorm.ErrRecordNotFound
ExistsOwnedByUser(ctx context.Context, userID, personaID uint64) (bool, error)          // 跨模块共享闸门

// 写路径（全部接受事务句柄）
Create(ctx context.Context, tx *gorm.DB, p *model.Persona) error
UpdateOwned(ctx context.Context, tx *gorm.DB, userID, personaID uint64,
            name, personalityDesc, speakingStyle string) (int64, error)                  // 返回 RowsAffected
DeleteOwned(ctx context.Context, tx *gorm.DB, userID, personaID uint64) (int64, error)   // 返回 RowsAffected

// proactive_repo.go（本次只用这一个函数；后续主动消息模块在此文件继续加）
CreateDefaultSettings(ctx context.Context, tx *gorm.DB, userID, personaID uint64) error
```

**签名冻结的三条理由（写给 reviewer 与成员 1）**：

1. `ExistsOwnedByUser` 的签名与 user-memory/plan.md H2 行**逐字一致**，成员 1 直接引，不要再写第二份。
2. `FindOwned` 与 `ExistsOwnedByUser` **并存不是冗余**：前者要整行数据（`PUT` 的回读），后者只要一个 bool（memory / chat 的闸门，不需要人设内容）。让 memory 模块为了一个 bool 去拉一整行，是把不必要的数据搬进内存。
3. `UpdateOwned` 收三个 string 而不是 `*dto.UpdatePersonaRequest`：**repo 不引 dto**。repo 依赖 dto 会让"改响应结构"波及数据层，而分层纪律是 repo 只做 CRUD（技术文档 §4.1）。

### 5.3 反例清单（AI 最容易写出来的错法）

| ❌ 错法 | 后果 | ✅ 正确 |
|---|---|---|
| `db.First(&p, id)` 再在 Go 里比 `UserID` | 忘写比较就静默越权；多一次往返 | `WHERE id = ? AND user_id = ?` |
| 写 `ExistsByID(id)` / `FindByID(id)` 单条件方法 | 可被逐号枚举出系统里有多少人设 | 要归属就叫 `ExistsOwnedByUser`（§5.2） |
| 未命中返回 `4030` | `4030` 是**功能越权**的码，本模块无此场景；且契约 §4 里没有它 | 统一 `4043` |
| 用 `4040` 表示"不存在"、`4043` 表示"不属于你" | **两者并存就能被二分探测出 id 是否存在**，等于没隐藏 | 都收敛到 `4043` |
| 列表漏 `WHERE user_id = ?` | **跨用户泄漏全部人设** | 从 Token 取 `userID`，写进条件 |
| `Count` 与 `Find` 的 WHERE 不一致 | `total` 与 `list` 对不上，前端分页器错乱 | 两处子句**逐字相同** |
| 从 body/query 读 `userId` | 攻击者传谁的 id 就看谁的数据 | 只从 JWT 上下文取 |
| 取不到 userID 时用 `0` 兜底继续查 | 鉴权失败伪装成空列表 | 返回 `4010` 并中断 |
| 为了让接口"能跑"给 userID 兜底一个 `1` | **越权防线当场全废**，且看起来一切正常（见 §7.1） | 等 `JWTAuth` 落地，不许兜底 |
| `db.Save(&p)` 更新 | **`state` 被零值覆盖、`last_message_at` 被清空**，不可逆 | 只 `Updates` 指定三列（map 形式） |
| `Updates` 传 struct | 零值字段被静默跳过 | 传 `map[string]any` |
| 先提交人设、再插 `proactive_settings` | 后者失败就留下没人设配置的孤儿 | 同一个事务（service 层编排） |
| 事务内用包级 `db` 而非 `tx` | **回滚时这条插入不会被撤销**，事务白做 | repo 内一律 `tx.WithContext(ctx)` |
| 事务闭包内产生的 `4043` 被外层重新包成 `5003` | **越权从 `4043` 变 `5003`**，红线 3 的规则失效（详见 §5.4） | 事务之后 `return nil, err`，不 Wrap |
| `DELETE` 手写多表清理 | 漏一张表就留脏数据 | 交外键 `ON DELETE CASCADE` |
| `order by last_message_at desc`（无 `NULLS LAST`） | 没聊过的人设排最前 | `DESC NULLS LAST, id DESC` |
| `list` 用 nil slice | 序列化成 `null`，前端 `v-for` 崩 | `make([]PersonaResponse, 0)` |
| `ClampPage` 写成返回错误（`pageSize>100` → `4001`） | 契约漂移；分页参数本不该产生错误码 | 夹到范围内即可 |
| handler 里 `_ = c.Error(err)` 传 binding 的**原始 error** | 不是 `BizError` → `BizErrorHandler` 走 `5000` 分支，**`4001` 变成 `5000`** | 传 `errcode.New(errcode.ErrInvalidParams)`（§5.4） |

### 5.4 两处「错误码被中途改写」的陷阱（本功能最容易出、且最难发现的一类）

这两处的共同点是：**代码看起来完全正常，`go vet` 不响，只有按错误码断言时才发现拿到了别的码**。而且它们都不会让任何已有检查失败——只有"越权必须返回 `4043`"这条断言会失败，如果那条断言恰好没写，就一路静默到线上。

**陷阱 A：事务闭包里的 `4043` 被外层包装成 `5003`**

```go
// ❌ 错法：把 4043 洗成 5003
err := s.db.Transaction(func(tx *gorm.DB) error {
    n, err := s.repo.UpdateOwned(ctx, tx, userID, personaID, ...)
    if err != nil {
        return errcode.Wrap(errcode.ErrDBFailed, err)
    }
    if n == 0 {
        return errcode.New(errcode.ErrPersonaNotFound)   // ← 这里是对的
    }
    return nil
})
if err != nil {
    return nil, errcode.Wrap(errcode.ErrDBFailed, err)   // ← 到这里 4043 变成了 5003
}
```

`errcode.Wrap(ErrDBFailed, err)` **不检查 `err` 是不是已经是 `BizError`**，它无条件造一个新的 `5003`。`BizErrorHandler` 用 `errors.As` 取到的是**最外层那个**，于是：**任何越权/不存在都返回 `5003`**。

```go
// ✅ 正确：事务之后原样透传
if err != nil {
    return nil, err        // 是 BizError 就原样上抛；是原生 error 就交给 BizErrorHandler 按 5000 处理
}
```

**判据（可直接 grep）**：`db.Transaction` 这个调用**之后**的代码块里，**不得出现 `errcode.Wrap`**。事务闭包**内部**才允许把 repo 返回的原生 error 包成 `ErrDBFailed`。

**陷阱 B：binding 失败上抛原始 error**

```go
// ❌ 错法：4001 变成 5000
if err := c.ShouldBindJSON(&req); err != nil {
    _ = c.Error(err)          // 原始 error 不是 BizError → BizErrorHandler 走 "unknown error" → 5000
    return
}

// ✅ 正确
if err := c.ShouldBindJSON(&req); err != nil {
    _ = c.Error(errcode.New(errcode.ErrInvalidParams))   // 4001；原始 err 不返回前端（红线 6）
    return
}
```

这一条**修正了任务书第 3 条的字面理解**："handler 一律用 `_ = c.Error(err)` 上抛"这个要求里，**上抛的 `err` 必须是 `BizError`**——`BizErrorHandler`（`middleware/biz_error.go:28-53`）只对 `*errcode.BizError` 查表，其余一律 `5000`。上抛原始 error 不是"更透明"，是**把 4xx 洗成 5xx**。

**顺带一条**：`BizError` 里包着的 `Err` 字段（原始错误）**只进日志、不进响应体**（`biz_error.go` 就是这么做的）。所以 `Wrap(ErrDBFailed, dbErr)` 是安全的、不泄漏 SQL 细节；而**手写**一句中文文案拼进响应才是红线 6 的违规——`pkg/response.Fail` 在类型上就不接受 message 参数，想违反也违反不了。

---

## 6. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| 字段名**全 camelCase**，与契约 §4 逐字一致（`personalityDesc` 不是 `personality_desc`） | 契约 §1 / AGENTS §5 |
| **不新增错误码**；只用 `ErrInvalidParams`(4001) / `ErrUnauthorized`(4010) / `ErrPersonaNotFound`(4043) / `ErrDBFailed`(5003) | 契约 §4 + §2 总表 |
| **资源越权 → `4043`**；本模块无功能越权场景，**不产生 `4030`**，也不产生 `4040`/`4031` | 契约 §2 / §4 / AGENTS §4.3 |
| **禁止硬编码错误码数字或文案**；`Fail()` 只接受 `ErrorCode` | **红线 6** |
| handler **不写** `response.Fail`，错误 `_ = c.Error(err)` 上抛，且**上抛的必须是 BizError** | 技术文档 §4.4 / §5.4 陷阱 B |
| handler 不直接操作 DB；service **不依赖 `*gin.Context`**（只收 `context.Context`） | **红线 7** / 技术文档 §4.1 |
| **多表写入（人设 + 主动消息配置）的事务边界在 service 层**，repo 只接受事务句柄 | 技术文档 §4.1 / 红线 7 |
| repo **不引 dto**；service 不引 `gin`；dto 可以引 model | 技术文档 §4.1 分层图 |
| 所有查询带 `user_id`；归属条件写在 SQL 的 `WHERE` 里 | **红线 3** / 任务书要求 1 |
| `PageResult[T]` **只有 `internal/dto/common_dto.go` 一份定义**，本分支若为最先落地者则**建它**，否则消费现成的 | AGENTS §3 / user-memory spec §4.1 |
| 注释只写必要三类（安全/越权红线、非显然的 GORM tag 语义、函数职责一句话）；**学习内容放 `.learn/`，不进 `.go`** | **AGENTS §5.1** / 任务书要求 6 |
| 分支 `feature/backend-persona-crud`，走 PR，**禁止直推 `main` / `develop`**；commit 格式 `<type>(scope): <subject>`，scope 用 `persona` / `dto` | **红线 2** / AGENTS §6 |
| **不提交任何密钥**：本功能不新增任何 Key/Token/密码；禁止把 DSN/密码写进代码或 `_test.go` | **红线 1** |
| 改表结构只改 struct + `AutoMigrate`，**禁止手写 `ALTER TABLE` / `DropTable`** | **红线 8** |
| 表名 `personas` / `proactive_settings`、列名小写下划线；Go 缩写词全大写（`UserID` 不是 `userId`）；ID 一律 `uint64` | AGENTS §5 |
| 不碰 `API_CONTRACT.md` / `AGENTS.md` / `router.go` / `pkg/**` / `internal/middleware/**` | 文件归属（成员 1 的）+ MASTER §4.5 |

**红线自查（AGENTS.md §7 逐条对照）**

| 红线 | 本功能是否涉及 |
|---|---|
| ① 提交 `.env` / Key / 密码 / 模型权重 | 不涉及新增。**测试代码里也不许出现密码**——DSN 只能从环境变量读 |
| ② 直推 `main` / `develop` / force push | 流程约束：本分支走 PR，至少 1 人 Approve |
| ③ 跨用户 / 跨人设泄漏 | **主要战场**，见 §5 |
| ④ 明文 / 弱哈希存密码 | 不涉及（不碰密码） |
| ⑤ 情绪展示标签 / 朋友圈手动触发 / 日程新建端点 | 不涉及。**注意 `state` 是人格状态不是情绪**，别往情绪上靠 |
| ⑥ 硬编码错误码或文案；`Fail()` 传自定义 message | §5.4 + §6 上表 |
| ⑦ handler 直接操作 DB；service 依赖 `*gin.Context`；组件写 axios | §6 上表（axios 是前端，本功能不做前端） |
| ⑧ 生产 `DropTable`；手写 `ALTER TABLE` | 不涉及（模型层已合并，本次零 schema 改动，`migrate.go` 只加一行） |

---

## 7. 依赖与交接

### 7.1 我依赖谁

| 依赖 | 提供方 | 现状 | 影响 |
|---|---|---|---|
| `pkg/response`（`Success[T]` / `Fail`） | 成员 1 | ✅ **已落地**（`response.go`） | 无阻塞 |
| `pkg/errcode`（4001/4010/4043/5003 常量 + `New`/`Wrap`） | 成员 1 | ✅ **已落地**，且 4043 的语义注释与契约一致 | 无阻塞 |
| `internal/middleware`（`ContextKeyUserID` / `BizErrorHandler`） | 成员 1 | ✅ **已落地** | 无阻塞 |
| `internal/config` / `cmd/server` / `db` 连接 | 成员 1 | ❌ **尚未落地**（`backend/cmd/` 下只有 `migrate`） | 无法起 HTTP 服务做端到端验证 |
| **`JWTAuth` 中间件** | 成员 1 | ⛔ **是空壳**：`middleware/jwt.go` 直接 `c.Next()`，**不设置 `ContextKeyUserID`** | **本功能 4 个端点现在全部返回 `4010`**，curl 验收全线阻塞。见下 |
| `router.go` 挂载点 | 成员 1 | ❌ **不存在**（`internal/handler/router.go` 未落地） | 端点不可达 |

**⛔ 头号阻塞：`JWTAuth` 是空壳，4 个端点现在**全部**返回 `4010`。**

这不是缺陷，是**正确行为**：拿不到 `userID` → 按 §5.1 ① 返回 `4010`。但它的后果必须说清楚：

- 在成员 1 落地 `JWTAuth` **之前，4 个端点无法用 curl 联调**，B 组验收（§8）全部阻塞。
- **绝不能为了让它"能跑"而在 handler 里给 `userID` 兜底一个 `1`。** 这是本功能最可能发生的、也是最危险的"临时改动"：它让越权防线**当场全废**，而且表面上一切正常（每个人都能看到"自己的"人设——其实是第一个用户的）。**这条写进拒绝标准**（plan §4.5）。
- 等待期间的验证路径：A 组（§8）全部可以在**不经 HTTP** 的前提下完成——直接对 repo / service 层写临时 `_test.go`（DSN 从环境变量读），或在测试里手工 `c.Set(middleware.ContextKeyUserID, uint64(...))` 后挂 `BizErrorHandler` 跑完整链路。**唯一测不到的一段是真实 JWT 解析**，那段归成员 1。

**⚠️ 广播项：`c.Set` 的值必须是 `uint64`，不能是 `int`。**

gin v1.12 的 `GetUint64` 走 `getTyped[uint64]` → `val.(uint64)` **直接类型断言**，不是宽泛转换。所以：

- ✅ `c.Set(ContextKeyUserID, claims.UserID)`（`pkg/jwt.Claims.UserID` 就是 `uint64`，原样传即可）
- ❌ `c.Set(ContextKeyUserID, int(claims.UserID))` → 断言失败 → **静默返回 0** → **每个请求都 `4010`**，而日志里看不出任何异常（no panic, no error）

这条要主动告知成员 1。**同一根因还造成一个已存在的 bug**：`middleware/logger.go:49` 写的是 `zap.String("userId", c.GetString(ContextKeyUserID))`——对 `uint64` 值做 `val.(string)` 断言必然失败，返回 `""`，所以**访问日志里的 `userId` 字段永远是空串**。这是成员 1 的文件，**本分支只报不改**（也不在 PR 里顺手改，避免 review 无法界定范围）。

### 7.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `dto.PageResult[T]` + `ClampPage`（`common_dto.go`） | **成员 1、成员 3** | 五个分页端点共用；**本分支率先落地，后来者消费，不要重建** |
| `repo.ExistsOwnedByUser` | **成员 1**（`memory_service` / `chat`）、成员 3（`proactive` / `schedule`） | 归属闸门，签名已冻结（§5.2） |
| `repo.CreateDefaultSettings` | 成员 3（主动消息模块） | 主动消息模块在此文件继续加 `Get` / `Upsert` |
| 4 个端点 | **成员 2** | 聊天页侧栏（`GET /personas` 就是对话列表）、`/chat/:personaId` 跳转、人设管理页 |
| `PersonaResponse` 结构 | **成员 2** | 写 `types/persona.ts` 与 Mock；**字段名逐字对齐 camelCase**；提醒他 `familiarity` 存在 `state` 里（§4.6 末尾的文档瑕疵） |
| `RegisterPersonaRoutes` | 成员 1 | `router.go` 加一行（群里同步后） |
| `proactive_settings` 一定存在一行 | 成员 3（主动消息模块） | `GET /proactive/settings` 可无条件查表，不必写"无行则返回默认值"的兜底 |
| ⛔ **`enabled` / `interval_*` / `daily_limit` 写 `false` / `0` 必须用 `map`** | **成员 3（主动消息模块）—— 这一条是硬的** | 这四列带 `default:` tag，GORM 会在零值时把 tag 值替换进去：`Updates(model.ProactiveSetting{Enabled:false})` 是 **`Error=nil`、`RowsAffected=0` 的静默空操作**——"关掉主动消息"会不生效且不报错（实测读数见 §8.6 / §3.2 结论 3） |
| §4.7 的异常输入行为表 | **全员** | 契约空白处的约定，建议广播后补进 `API_CONTRACT.md` 的说明段 |

### 7.3 决策记录与剩余待确认

**已定（本文档范围内，不再讨论）：**

| # | 决策 | 落点 |
|---|---|---|
| 1 | `:id` 非数字 / `id=0` **一律 `4043`**，不引入 `4001` | §4.7 |
| 2 | 分页参数**只收敛不报错**（`ClampPage`） | §4.1 / §4.7 |
| 3 | 写路径**统一走 `db.Transaction`**（含只有一个语句的 `DELETE`），换取"`tx` 永远是事务句柄、永远不为 `nil`"这条不变式 | plan §3.3 |
| 4 | `UpdateOwned` 收三个 string，**repo 不引 dto** | §5.2 |
| 5 | `ExistsOwnedByUser`（双条件、只回 bool）**允许且必需**；被禁的是单条件存在性探针 | §5.2 |
| 6 | `PageResult[T]` 由**本分支率先落地**在 `internal/dto/common_dto.go` | §4.1 |
| 7 | **前置文件纳入本 PR**：`dto/common_dto.go` 与 `model/proactive_setting.go`（+ `migrate.go` 一行）**都是本 PR 的交付物**，与 4 个核心文件同批提交 | §2.1（2026-09-16 用户裁定，PR 描述里说明扩大范围的理由） |
| 8 | **`user_profile.go` 缺关联字段不在本分支修**：本分支是 `feature/backend-persona-crud`，范围只包括人设 CRUD。修复另开分支 | 见下方待办 #2 |

**剩余待确认（实现前定，不要自己拍板）：**

| # | 问题 | 建议 |
|---|---|---|
| 1 | `PUT` / `DELETE` 是否包事务（决策 3） | 已定为"统一包"。**代价**：`DELETE` 多一次 `BEGIN`/`COMMIT`，用户量级下可忽略；**收益**：`tx` 永不为 `nil`，review 时不必逐个方法判断"这个写方法到底在不在事务里" |
| 2 | `UpdatePersonaRequest` 是复用 `CreatePersonaRequest` 还是另起 | 建议**另起一个类型**（两个文件的字段名相同但校验规则将来可能分叉；且"可另起一个类型"是 persona-model §4.6 允许的）。**结构完全相同也可以接受**，不算违规——只要别让 `PUT` 变成部分更新 |
| 3 | §4.7 的异常输入行为是否补进 `API_CONTRACT.md` | 建议广播后由契约 Owner 补一段说明。**本分支不改契约文件**（全局文件） |

**已确认的待办（不在本 PR 内，合并后执行）：**

| # | 待办 | 触发时机与完成标准 |
|---|---|---|
| 1 | 本 PR 的**范围扩大**要在 PR 描述里说明理由 | 见 plan §6 的 PR 描述草稿 |
| 2 | **`fix(model): restore user_profile FK associations`** —— 补回 `user_profile.go` 的 `User` / `Persona` 两个关联字段 | **等本 PR 合并后**另开分支（**不在 `feature/backend-persona-crud` 上做**）。验收：跑临时库，`\d user_profile` 能看到**两条** `ON DELETE CASCADE` 外键（当前实测 0 条） |

> **待办 #2 的背景（2026-09-16 用户确认为真实 Bug）**：`persona.go:58` / `chat_message.go:99-100` / `user_memory.go:132-133` 都有关联字段，**`user_profile.go` 是全项目唯一没有的**（实测 0 处关联字段）。后果：删账号 / 删人设时 `user_profile` 行**不会级联删除**，留下孤儿画像——与 user-profile spec §8 分组 A「2 个外键都在，且都是 `ON DELETE CASCADE`」直接矛盾。**修复分支的验收必须包含"注入前 0 条 → 修复后 2 条"的双向读数**，只跑修复后的库看不出"本来应该有几条"。

---

## 8. 验收标准

> **规则（本项目立的规矩，`docs/dev_notes/user_memory_notes.md` §0）**：禁止类和必须类**成对**写。只写"不许出现 X"是半套标准——"写漏"类缺陷（少一个 tag、少一个 `WHERE`）能一路静默。
> **每条"必须类"检查都要能用真实缺陷反向验证**（注进去要响、不注不许响），否则它本身可能就是一条永远为真的假标准。本文 §8.4 记录已完成的测量。

### 分组 A · 可独立验证（✅ 本分支的验收范围，不依赖任何人）

**A1 · 编译与静态检查**

- [ ] `cd backend && go build ./... && go vet ./... && go test ./...` 全通过
- [ ] `gofmt -l internal/` 无输出

**A2 · `proactive_settings` 表结构（`\d` 实测）**

- [ ] 8 列类型/可空性与 DDL 一致；四个默认值显示为 `true / 30 / 120 / 3`
- [ ] **`Indexes` 段**有 `uq_proactive_settings_user_persona UNIQUE` —— ⚠️ **在 Indexes 段找，不是 Constraints 段**（§3.2 结论 1）
- [ ] **`Foreign-key constraints` 段有 2 条**，且**都是 `ON DELETE CASCADE`**（→ `users`、→ `personas`）—— 必须类，漏一个关联字段就只剩 1 条
- [ ] 序列只有 `proactive_settings_id_seq`，**没有** `proactive_settings_user_id_seq` / `_persona_id_seq`（`bigserial` 污染检查）
- [ ] 表名是 `proactive_settings`（复数），不是 `proactive_setting`

**A3 · 播种真值（同时验证 §3.2 结论 3）**

- [ ] Go 侧建一行人设后，`SELECT enabled, interval_min, interval_max, daily_limit, last_nudge_at FROM proactive_settings WHERE persona_id = <新id>` → **恰好 1 行**，值 `t | 30 | 120 | 3 | NULL`
- [x] 反向验证（**2026-09-16 实跑，判据已按结果改写，见 §8.6**）：把 `CreateDefaultSettings` 里的四个值临时改成 `false/0/0/0` → 入库**仍然是 `true/30/120/3`**，不是 `false/0/0/0`。
      ⚠️ **原判据写反了**：它假设"零值会原样进库"，实测是 GORM 用 `default:` tag 的 Go 值替换掉了零值。**原判据拿去判正确代码会失败，是一条假标准**——属于本文 §8.5「判据是照着我以为要写的代码拟的」的又一起，且这次连"源码结论"本身也是错的。

**A4 · 越权条件在 SQL 层的语义（不经 HTTP）**

- [ ] `UPDATE personas SET name='x' WHERE id=<别人的id> AND user_id=<我> ` → **`RowsAffected = 0`**
- [ ] **同值更新**：`UPDATE ... WHERE id=<我的id> AND user_id=<我> SET name=<原值>` → **`RowsAffected = 1`**
      （这条是 `4043` 判定的基石：证明 `RowsAffected == 0` 的唯一含义是"WHERE 没匹配到"，**不是**"值没变"。没有这条，`PUT` 的同值提交会被误判成 `4043`）
- [ ] `DELETE FROM personas WHERE id=? AND user_id=?` 未命中 → **`RowsAffected = 0`**
- [ ] 三条 SQL 的 `WHERE` 都是**两个条件**（`grep` 断言，不是靠肉眼）

**A5 · 级联与事务**

- [ ] 删人设 → `proactive_settings` 对应行随之消失（`count = 0`）
- [ ] 两级级联：删 `users` 行 → `personas` → `proactive_settings` 全部消失
- [ ] **事务回滚**：人为让播种失败（例如临时把 `personaID` 改成不存在的值触发外键违例）→ **`personas` 不得残留新行**

**A6 · 排序**

- [ ] 造 3 行（`last_message_at` 分别为 `NULL` / 较早 / 较晚）→ 列表顺序是「较晚、较早、NULL」——**未聊过的排最后**，验证 `NULLS LAST` 生效
- [ ] 反向验证：把 `NULLS LAST` 去掉 → 顺序变成「NULL 在最前」，证明这条检查有效

**A7 · 代码级锚定检查（**必须锚定到 `gorm:"` / `return` / 函数声明，不要用裸词**）**

| # | 检查 | 期望 | 实测（2026-09-16） |
|---|---|---|---|
| 1 | `grep -c '^type PageResult' internal/dto/common_dto.go` | **1**（必须类） | ✅ 1 |
| 2 | `grep -rl '^type PageResult' internal/ \| wc -l` | **1**（**只有一份定义**，本表最重要的一条） | ✅ 1 |
| 3 | `grep -cE '^\s+(User\|Persona)\s+(User\|Persona)\s+`gorm' internal/model/proactive_setting.go` | **2** | ✅ 2 |
| 4 | `grep -c 'OnDelete:CASCADE' internal/model/proactive_setting.go` | **2** | ✅ 2 |
| 5 | `grep -rhoE 'gorm:"[^"]*type:bigserial' internal/model/ \| wc -l` | **0**（禁止类） | ✅ 0 |
| 6 | 把每个方法签名合并成一行后数带 `userID` 的（下方 awk） | **5**，且**例外只有 `Create`** | ✅ 5 / Create |
| 7 | `grep -cE 'First\(&[a-z]+, *(id\|personaID)\b' internal/repository/persona_repo.go` | **0**（禁止类：单条件裸查） | ✅ 0 |
| 8 | `grep -cE 'func.*Exists(ByID\|ByPersonaID)\(' internal/repository/persona_repo.go` | **0**（禁止类：单条件探针） | ✅ 0 |
| 9 | `grep -c 'func.*ExistsOwnedByUser' internal/repository/persona_repo.go` | **1**（必须类，与 #8 配对） | ✅ 1 |
| 10 | `grep -cE '\bSave\(' internal/repository/persona_repo.go` | **0**（禁止类） | ✅ 0 |
| 11 | `grep -c 'gin-gonic' internal/service/persona_service.go` | **0**（禁止类：红线 7） | ✅ 0 |
| 12 | `grep -c 'response\.Fail(' internal/handler/persona_handler.go` | **0**（禁止类） | ✅ 0 |
| 13 | `grep -oE 'c\.Error\([^)]*\)' internal/handler/persona_handler.go \| sort \| uniq -c` | **只有 `c.Error(errcode.New(...))` 与 `c.Error(err)`（service 回传）**，不得出现裸的绑定错误 | ✅ 4×`err` + 8×`errcode.New` |
| 14 | `grep -A3 '^	}$' internal/service/persona_service.go \| grep -cE 'return.*Wrap'` | **0**（§5.4 陷阱 A） | ✅ 0 |
| 15 | `grep -rnE '\b(400[0-9]\|40[13][0-9]\|500[0-9])\b' <四个目录> \| grep -vcE ':[0-9]+:[[:space:]]*//'` | **0**（禁止硬编码数字，红线 6；**必须排掉注释行**，见下） | ✅ 0（不排注释是 4） |
| 16 | `grep -rhoE '\b(4030\|4040\|4031)\b' service handler \| wc -l` | **0**（本模块不产生这些码） | ✅ 0 |
| 17 | `grep -cE 'make\(\[\]dto\.PersonaResponse, 0' internal/service/persona_service.go` | **1**（必须类：空列表不能是 nil） | ✅ 1 |
| 18 | `grep -c '= c\.GetUint64(' internal/handler/persona_handler.go` | **1**（必须类：只有一处读 userID，§5.1 ①） | ✅ 1（裸词是 2） |

**#6 的 awk（把多行签名合并后判定，不能用裸词）：**

```bash
awk '/^func \(r \*PersonaRepo\)/{s=$0; while (s !~ /\{/) {getline; s=s" "$0}; print s}' \
  internal/repository/persona_repo.go | grep -c 'userID'          # 期望 5
# 例外名单（期望只有 Create —— 它的 userID 在 p *model.Persona 里）：
awk '/^func \(r \*PersonaRepo\)/{s=$0; while (s !~ /\{/) {getline; s=s" "$0}; print s}' \
  internal/repository/persona_repo.go | grep -v 'userID' | grep -oE '\) [A-Z][A-Za-z]+\('
```

**#14 为什么必须锚定到 `return.*Wrap`：** 写成 `grep 'Wrap'` 会命中**本仓库自己的注释**——`persona_service.go:87/147` 写着"事务内的 BizError 在这里再 Wrap 一次会被洗成别的码"，于是正确代码读出 **2**。这条检查的第一版就是这么错的（§8.5）。

> ⚠️ **锚定的理由（本项目已复发多次的教训）**：裸词 grep 会命中**解释性注释**。实测例：`grep -rn "response.Fail" internal/` 今天是 **4** 条，其中 `middleware/biz_error.go:15` 是**注释**，真正的调用只有 3 处、且全在中间件里；锚定成 `response\.Fail\(` 才得到 3。**任何期望为 0 的检查都必须先拿正确的文件跑一遍**，确认读数真的是 0。

### 8.5 反向验证：本表的 6 条检查在**正确代码**上读出过假失败（2026-09-16 实测）

> 代码落地当天，把 A7 的 18 条逐条跑了一遍，**6 条读出与预期不符**。逐条查下去，**全部是检查本身写错了，代码是对的**。这是本项目同类问题的第 N 次复发，但这次是**成批出现**，值得单独记一节。

| # | 原判据 | 读出 | 根因 | 改成的判据 |
|---|---|---|---|---|
| 6 | 正则 `^func \(r \*personaRepo\)` | 0 | 接收者类型名是 `PersonaRepo`（**导出**，因为成员 1 的模块要直接引 `ExistsOwnedByUser`——不导出就引不到） | 与接收者名解耦，改用 awk 合并签名 |
| 6 | `grep 'userID uint64'` | 1 / 6 | Go 允许 `userID, personaID uint64` **合并写类型**，字面量匹配不到 | `grep 'userID'`（在合并后的签名里，注释已被排除） |
| 14 | `grep 'errcode.Wrap'`（在事务收尾处） | 2 | 命中的是**我自己的注释**（"再 Wrap 一次会被洗成别的码"） | `grep -E 'return.*Wrap'` |
| 15 | `grep '\b(4001\|…)\b'` | 4 | 4 条**全是注释**（`persona_service.go:95`、`persona_handler.go:46`、`common_dto.go:20`） | 追加 `grep -v` 排除注释行 |
| 17 | `make(\[\]dto.PersonaResponse, 0)` | 0 | 实际代码是 `make(..., 0, len(list))` —— **带容量**才是对的，字面量少了后半截 | 去掉闭合括号 |
| 18 | `grep -c 'GetUint64'` | 2 | 1 处调用 + 1 处**注释**（解释 gin v1.12 的坑） | `grep '= c\.GetUint64('` |

**另有一条来自 `git diff` 的假失败**（plan §4.4 第 9 条）：`migrate.go` 判据写成"`git diff` 只有 1 行新增"，实测 **6 增 6 删**——gofmt 按块内最宽元素对齐行尾注释，新加的那行更长，把它上面 5 行重排了。改用 `git diff --ignore-all-space` 后是 **1 减 1 增**。

**共同根因（一句话）**：这些判据是**照着我以为要写的代码**拟的，不是照着**实际写出来的代码**拟的。格式（多行签名）、Go 语法（合并类型声明）、以及**我自己写的解释性注释**，三样都会让一个"看起来显然"的 grep 失效。

**因此新增一条立规**：A7 表里的**每一条**都必须**在真实代码上跑过一次、并把读数写进表里**（上面第 4 列）。**没有实测读数的检查项，不允许写进验收表**——它可能是一条永远为真（或永远为假）的假标准。这与 §8.4「期望为 0 的检查必须先拿正确文件跑一遍」是同一条规矩，只是这次把范围从"期望为 0 的"扩大到了**全部**。

**6 条里没有一条是"代码写错了"**，这本身也是结论：**checks 的假失败率远高于代码的真缺陷率**，把时间花在核对读数上比花在核对代码上更值得。

### 分组 B · 阻塞（🚧 依赖成员 1 的 `JWTAuth` + `cmd/server` + `router.go`，**本分支不做**）

全部 curl 级验收：4 个端点全通；无 Token → `4010`；缺 `name` → `4001`；空列表 `list` 为 `[]`；用 B 的 Token 打 A 的 `personaId` → `4043`，与"不存在的 id"**同码同文案**；任何输入组合下**都不出现 `4030`**；`state` 在响应体里是 JSON 对象**不是 base64**；`PUT` 之后 `proactive_settings` 行未被修改；`PUT` 之后 `state` 的未知键（`self_note`）仍在。

**B 组不落地不等于不设计**——这些断言的**代码级前置**已全部在 A 组覆盖（A4 覆盖越权的 SQL 语义、A3 覆盖播种、A6 覆盖排序）。B 组只是同一批断言的 HTTP 外观。

### 分组 C · 流程（阻塞合并）

- [ ] PR 已开、至少 1 人 Approve；commit 符合 `<type>(<scope>): <subject>`
- [ ] 人工审查按 plan §4 走完，笔记落到 `docs/dev_notes/persona_crud_notes.md`（该文件在 `.git/info/exclude` 里，**不进 PR**）
- [ ] §7.3 待确认 #1 已拍板
- [ ] 已将 §7.1 的 `c.Set` **类型必须是 `uint64`** 广播给成员 1（附 `logger.go:49` 的 `userId` 日志 bug）
- [ ] 已将 §4.7 的异常输入行为表广播给全员
- [ ] 已将 §7.2 新增的「`enabled` / `interval_*` / `daily_limit` 写 `false` / `0` 必须用 `map`」广播给**成员 3**（他的 `PUT /proactive/settings` 直接踩这个坑）

### 8.4 已完成的测量（2026-09-16，写文档时实跑，非事后补记）

写在前面：**代码还不存在**，所以下面记录的是"这批检查在今天能不能响"，用来证明它们不是永远为真的假标准。

| 检查 | 今天的读数 | 结论 |
|---|---|---|
| `grep -rn "^type PageResult" internal/` | **0** | 必须类检查**会响**（A7 #1/#2 有效） |
| `grep -rn "^func ClampPage" internal/` | **0** | 同上 |
| `grep -rn "ExistsOwnedByUser" internal/` | **0** | 同上（A7 #9 有效） |
| `grep -rn "func RegisterPersonaRoutes" internal/` | **0** | 同上 |
| `grep -rn "GetUint64" internal/` | **0** | 同上（A7 #18 有效） |
| `grep -rn "response.Fail" internal/`（**裸词**） | **4** | ⚠️ **其中 1 条是注释**（`biz_error.go:15`）→ 裸词形式不可用 |
| `grep -rnE "response\.Fail\(" internal/`（**锚定**） | **3** | 全部在 `middleware/`（合法位置）→ 锚定形式可用 |
| `grep -c "OnDelete:CASCADE" internal/model/persona.go` | **1** | 必须类检查在**已正确**的文件上读数为 1 → 不会假失败 |
| `grep -cE "gorm:\"[^\"]*type:bigserial" internal/model/*.go` | **0** | 禁止类；且这正是已知缺陷形态（persona-model 踩过 3 次） |
| `grep -c "idx_personas_user_last_msg" internal/model/persona.go` | **3** | 索引两个字段各 1 次 + 注释 1 次 → **裸词也会命中注释**，A6 的索引检查要锚定到 `gorm:"` |
| `grep -nE '^\s+(User\|Persona)\s+\*?(User\|Persona)\s+`gorm' internal/model/*.go` | persona 1 / chat_message 2 / user_memory 3 / **user_profile 0** | 确认 `user_profile.go` 是唯一漏关联字段的模型（§7.3 待办 #2） |

### 8.6 代码落地当天的实测读数（2026-09-16 下午，一次性临时库 + 一次性容器的真跑）

写在前面：代码写完后，**用一次性的 `postgres:16` 容器 + 一个跑完即删的 `cmd/scratchcheck` 脚本**，把 A 组全部跑了一遍。脚本与容器**都不进 PR**，本文只留读数。**下面每一条都是真跑出来的，不是推的。**

**A2 · `\d proactive_settings`（这是本轮唯一靠静态手段查不出的一类）**

| 检查项 | 读数 | 判定 |
|---|---|---|
| `Indexes` 段有 `uq_proactive_settings_user_persona UNIQUE` | ✅ 在 **Indexes** 段（不在 Constraints） | 过（§3.2 结论 1 得到验证） |
| `Foreign-key constraints` 有 2 条、都是 `ON DELETE CASCADE` | ✅ `fk_proactive_settings_user` / `fk_proactive_settings_persona` | 过 |
| 序列只有 `proactive_settings_id_seq` | ✅ 无 `_user_id_seq` / `_persona_id_seq` | 过（`bigserial` 未污染） |
| 表名 `proactive_settings`（复数） | ✅ | 过 |
| `interval_min` / `interval_max` / `daily_limit` 三列类型 | ❌ **首次实测是 `bigint`**，与 DDL 的 `INT` 不符 | **真缺陷**，已修（`type:int` → `type:integer`，见 §3.2 结论 5） |

> **A2 是本轮唯一抓到真缺陷的一组。** `go build` / `go vet` / `gofmt` / A7 的 18 条 grep **全部通过**，而三列的错误类型只有 `\d` 看得见。**"能编译 + 能 grep" 与 "schema 是对的" 之间没有蕴含关系。**

**A3 · 播种真值**

| 检查项 | 读数 | 判定 |
|---|---|---|
| 建一行人设后按 `persona_id` 查配置行 | 恰好 **1 行**，`t \| 30 \| 120 \| 3 \| NULL` | 过 |
| 反向验证（把四个值改成 `false/0/0/0`） | 入库**仍是** `true/30/120/3` | ⚠️ **原判据写反了，已改写**（§3.2 结论 3） |

**A4 / A5 / A6 · 行为读数**（全部经 `service` 层真跑，不经 HTTP）

| 用例 | 读数 | 判定 |
|---|---|---|
| A 改 / 删 B 的人设、改不存在的 id、改 id=0 | 四例全部 `4043` | 过 |
| 同值再改一次自己的 | `nil`（200），**不是 4043** | 过（`RowsAffected==0` 的语义基石成立） |
| 越权删除后 B 的人设仍在 | 行数 **1** | 过 |
| A6 排序（较晚 / 较早 / NULL） | `[小暖 12:04:34] [聊得早 11:04:34] [从没聊过 NULL]` —— **NULL 在最后** | 过 |
| A6 空列表 | `List == nil` 为 **false**（序列化成 `[]`，不是 `null`） | 过 |
| A6 分页收敛 `page=0&pageSize=1000` | → `page=1 pageSize=100` | 过 |
| A5 删人设 | 配置行 **1 → 0** | 过 |
| A5 重复删同一个人设 | `4043` | 过 |
| A5 删用户 B（两级级联） | B 的人设 **0** 行、B 的配置 **0** 行 | 过 |
| A5 事务回滚（播种撞外键违例） | 事务返回 error，`「会回滚」`人设残留 **0** 行 | 过 |

**A7 · 18 条锚定检查复跑**：**18 条全部与 §8.4 表内的"实测"列逐字相符**（含 #6 的 awk 合并签名 = 5/例外只有 `Create`、#13 = 4×`err` + 8×`errcode.New`、#14 = 0）。

**反向验证 · 6 次真实注入**（注进去必须响）

| # | 注入的缺陷 | 期望 | 实测 | 判定 |
|---|---|---|---|---|
| 1 | 删掉 `Order(...) ` 里的 `NULLS LAST` | 顺序翻转 | `[NULL] [较晚] [较早]` —— 没聊过的跑到最前 | ✅ 响 |
| 2 | `UpdateOwned` 的 WHERE 去掉 `user_id` | 越权改不该生效 | `code=5003`，且因**回读 `FindOwned` 充当了第二道闸门**、事务整体回滚 | ✅ 响（但**不是**静默写入） |
| 3 | `DeleteOwned` 的 WHERE 去掉 `user_id` | 越权删不该生效 | **`code=nil`（200），B 的人设真的被删了** | ✅ 响 —— **本条是最危险的形态**：删除路径没有回读，没有任何第二道闸门 |
| 4 | 播种改到事务外的包级 `db` | 人设不该残留 | **硬外键违例 5003**，人设一行都没进库 | ✅ 响（**实测与我的预测不符**，见下） |
| 5 | 复合注入：去掉事务 + 播种用错的 `personaID` | 不该留孤儿配置 | 库里留下 `id=1 \| 小暖 \| (NULL setting_id)` —— **孤儿人设**（"设置页打不开"） | ✅ 响 |
| 6 | `Update` 的收尾加一层 `errcode.Wrap(5003)`（§5.4 陷阱 A） | `4043` 被洗掉 | 静态检查 #14 由 0 → 1；行为上**越权改 / 不存在的 id / id=0 三例全部由 `4043` 变成 `5003`** | ✅ 响 —— 直接违反本任务的硬性要求 2 |

**两条从注入里得到的结构性结论（比注入本身值钱）：**

1. **陷阱 A 的注射点必须选 `Update` / `Delete`，选 `Create` 是测不出来的。** 我第一次把 `Wrap` 注在 `Create` 的收尾，静态检查响了、但 A4 六行读数**纹丝不动**——因为 `Create` 事务内只可能产生 `5003`，再 `Wrap` 一次还是 `5003`，**行为上不可观测**。陷阱只在**事务内会产生非 `5003` 码**的方法上才有后果（`Update` / `Delete` 的 `n == 0` → `4043` 分支）。**结论：反向验证选错注射点，会得出"这条检查无效"的错误结论。**
2. **外键约束把"忘了开事务"从静默损坏变成了响亮失败。** 第 4 次注入我预测会留下残留行（"事务白做"），实测是**硬违例、一行没进**——因为未提交的人设在另一个连接里不可见。**多层防御里，数据库约束是唯一在代码写错时还会替你拦一道的那层**；这也是 `proactive_setting.go` 那两个关联字段"漏一个就只剩 1 条外键"值得单独设一条验收（A2）的原因。

---

## 9. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-16 | v1 | 创建。把 persona-model 的 §4/§5 落到 4 个接口层文件上；发现并记录 4 处旧结论需修正（§0），其中 gin v1.12 `GetUint64` 无 `ok` 与 `user_profile.go` 缺外键两条是硬事实；给出 §5.2 的归属判定四形态判据；拆出 §4.7 契约空白处的异常输入行为表；A 组验收 18 条锚定检查 + 8 条基线测量记录 | 人设 CRUD 接口层开工前的设计与验收基线 |
| 2026-09-16 | v2 | 用户裁定：**6 个文件（4 核心 + 2 前置）全部纳入本 PR**（§2.1 加范围说明）；`user_profile.go` 外键缺失**确认是真实 Bug 但不在本分支修**，改为发布后的独立 fix 分支待办（§7.3 待办 #2，含双向读数验收） | 用户确认两点裁决 |
| 2026-09-16 | v3 | **代码落地当天的实测回合**。新增 §8.6（A2–A7 全部真跑读数 + 6 次反向注入）；**修掉一个真缺陷**：三个 `INT` 列被 GORM 建成了 `bigint`（`type:int` → `type:integer`，新增 §3.2 结论 5，含源码链条）；**推翻一条旧结论**：`default:` tag 会**替换零值**，`false`/`0` 用 struct 写不进去（§3.2 结论 3 重写，A3 的反向验证判据按实测改写——它原本是条**假标准**）；新增 §7.2 广播项（对成员 3 的 `PUT /proactive/settings` 是硬约束） | 一天内实测推翻了本文两处"读源码得出的结论" |

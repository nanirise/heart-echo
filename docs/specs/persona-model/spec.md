# spec · 人设 CRUD（Persona CRUD）

> 功能名：persona-model ｜ 分支：`feature/backend-persona-model`
> 负责人：成员 3（数据 + 人设 + 部署） ｜ 状态：设计完成，待实现
> 创建：2026-09-13 ｜ 最后更新：2026-09-13（第 3 版：套用队长「通用错误码规则」）
> 关联：[AGENTS.md](../../../AGENTS.md) ｜ [接口契约 §4](../../API_CONTRACT.md) ｜ [技术文档 §6.2 / §5.6](../TECH_DESIGN.md) ｜ [成员 3 任务书 §2](../dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md)

> ⚠️ **本功能遵循队长于 2026-09-13 明确的「通用错误码规则」**：**资源越权 → `4040`**（隐藏资源存在性）、**功能越权 → `4031`**。人设 CRUD 属于前者，故 `:id` 未命中一律 `4040`，见 §5.2。
> 契约 §4 目前仍写着 `4031`/`4043`，**规则广播与契约同步完成前不合并 PR**；全局文件由队长/成员 1 统一处理，本分支只改这两份文档。

---

## 1. 背景与目标

一个人设 = 一个 AI 伴侣 = 一个对话。`personas` 表既是「人设的存储」，**同时就是对话列表**（技术文档 §6.1：没有会话表）。因此本功能是全项目第一个业务模块，也是三个页面（人设管理页、聊天页、画像页）的共同前置。

**目标（一句话）**：交付 `personas` 的 GORM 实体与 4 个 CRUD 端点，字段与契约逐字一致；创建人设时把主动消息配置一并播种好；且**任何接口都不可能读到、也不可能探测到别人的数据**。

**为什么它在关键路径上**：聊天页（成员 2）靠 `GET /personas` 决定打开哪个对话；记忆、画像、日程（成员 1 / 成员 3）全部挂在 `persona_id` 上——人设的归属校验是它们共同的第一道闸门，这里漏了，后面每张子表都跟着漏。

## 2. 范围

### 2.1 做什么（In Scope）

| 项 | 产物 |
|---|---|
| 数据模型 | `internal/model/persona.go`（GORM 实体，含外键级联声明） |
| 主动消息配置模型 | `internal/model/proactive_setting.go`（创建人设时同步播种一行） |
| 建表 | `internal/model/migrate.go` 的 `AutoMigrate` 追加 `&Persona{}`、`&ProactiveSetting{}` |
| 通用分页结构 | `internal/dto/common_dto.go`（`PageResult[T]`，**全项目五个分页端点共用**） |
| 请求 / 响应结构 | `internal/dto/persona_dto.go` |
| 数据访问 | `internal/repository/persona_repo.go` |
| 业务逻辑 | `internal/service/persona_service.go`（**含创建人设的事务编排**） |
| HTTP 接口 | `internal/handler/persona_handler.go`（含 `RegisterPersonaRoutes`） |
| 路由挂载 | `router.go` 加一行 `RegisterPersonaRoutes(api, personaHandler)`（与成员 1 错开时间改，见总纲 §4.5） |

### 2.2 不做什么（Out of Scope）

| 不做的事 | 归属 / 原因 |
|---|---|
| 人设管理页 UI、`api/persona.ts`、`stores/persona.ts` | 本功能**只做后端**；前端页面是独立一步（成员 3 任务书 §3） |
| 主动消息配置的读写端点、定时扫描、触发逻辑 | 成员 3 的另一模块。**本功能只在创建人设时播种一行默认配置**，不实现 `GET/PUT /proactive/settings`、不实现 `TriggerNow` |
| 空状态引导（"还没有人设，去创建一个"） | 前端职责 |
| 人设数量上限、人设名去重 | 总纲 §0.3：**人设数量不限制**；DDL 上 `name` 无 UNIQUE，重名是合法的，**不要自作主张加 `4004` 冲突校验** |
| `familiarity` 的累加逻辑 | 归对话链路（技术文档 §5.6）：每轮对话结束 +1。本功能**只读展示**，不写 |
| 对话历史清空 | 总纲 §0.2：**不可单独清空**，只能随人设一起删 |
| `state.self_note` 等演化字段的写入 | 技术文档 §5.6 的加分项，本功能只负责**原样保留**，不解析、不写入 |
| 情绪相关任何字段 | 人设表没有情绪字段；`state` 是人格状态，**不是情绪**，不要混为一谈 |
| 新增错误码 | 本功能**一个都不需要新增**，复用 `4001` / `4040` / `5003` |
| 删除 `ErrPersonaNotFound`(4043) 常量 | 契约 §5/§7/§9/§10 的端点**仍在用它**，且 `pkg/errcode` 是成员 1 的文件——**不要动**。4043 是否随本次规则退役，由队长统一决定（§5.2） |
| 手写 `ALTER TABLE` / `DropTable` | 红线 8：改结构 = 改 struct + AutoMigrate |

## 3. 数据模型（`personas`）

### 3.1 权威 DDL（技术文档 §6.2，照抄待翻译）

```sql
CREATE TABLE personas (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name              VARCHAR(50)  NOT NULL,
    personality_desc  TEXT         NOT NULL,       -- 性格描述
    speaking_style    VARCHAR(255) NOT NULL,       -- 说话风格
    state             JSONB        NOT NULL DEFAULT '{}',  -- 人格状态，如 {"familiarity": 0}，见 §5.6
    last_message_at   TIMESTAMPTZ,                 -- 会话列表排序 + 主动消息空闲判定
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_personas_user_last_msg ON personas(user_id, last_message_at DESC NULLS LAST);
```

### 3.2 字段对照表（DDL ↔ GORM ↔ JSON）

| DDL 列 | Go 类型 | GORM tag 要点 | JSON 字段 | 契约来源 |
|---|---|---|---|---|
| `id BIGSERIAL PK` | `uint64` | `type:bigserial;primaryKey;autoIncrement` | `id` | §4 实体 |
| `user_id BIGINT NOT NULL` | `uint64` | `type:bigint;not null` | **`"-"`** | ⚠️ 契约实体里**没有** `userId` |
| `name VARCHAR(50) NOT NULL` | `string` | `type:varchar(50);not null` | `name` | §4 |
| `personality_desc TEXT NOT NULL` | `string` | `type:text;not null` | `personalityDesc` | §4 |
| `speaking_style VARCHAR(255) NOT NULL` | `string` | `type:varchar(255);not null` | `speakingStyle` | §4 |
| `state JSONB NOT NULL DEFAULT '{}'` | `datatypes.JSON` | `type:jsonb;not null;default:'{}'` | `state` | §4（原样透传） |
| `last_message_at TIMESTAMPTZ`（可空） | `*time.Time` | `type:timestamptz` | `lastMessageAt` | §4（可 `null`） |
| `created_at TIMESTAMPTZ NOT NULL` | `time.Time` | `type:timestamptz;not null;default:now();autoCreateTime` | `createdAt` | §4 |
| —（**不是列**） | `int`（派生） | — | `familiarity` | §4（只读派生） |

**所有 ID 类型统一 `uint64`**（`id` / `user_id` / `persona_id`），与既有的 `internal/model/user.go` 保持一致；仓储层与 service 的方法签名同理，不做 `uint` / `int64` 的来回转换。

**四条必须记住的结论：**

1. **`user_id` 不进响应体**：契约 §4 的 Persona 实体只有 8 个字段，其中没有 `userId`。用 `json:"-"` 关掉（与 `User.PasswordHash` 同一手法）——它是越权防线的载体，不是给前端看的。**不要为了"方便调试"把它改成 `json:"userId"`**。
2. **`familiarity` 不是数据库列**，它存在 `state` JSONB 里（DDL 注释与 §5.6 明确：`{"familiarity": 0}`）。响应体里它是**顶层字段**，需要从 `state` 里解出来再拍平。反过来 `state` 要**原样透传**，不能被重新序列化丢掉未知键。
3. **`lastMessageAt` 必须是指针**：契约要求"从未聊过为 `null`"，值类型 `time.Time` 会序列化成 `"0001-01-01T00:00:00Z"`，这是错的。
4. **`personas` 不是孤儿表**：创建它的同时必须存在一行 `proactive_settings`（§4.3 第 5 条），删除它时该行随外键级联消失（§4.5）。

### 3.3 `state` 的三条硬约束

| 约束 | 说明 |
|---|---|
| **只读透传** | 前端不解析、不回传修改（契约 §4）。服务端读出来原样放进响应，不 reshape |
| **写入时原样保留** | `PUT` 与 `DELETE` 路径**绝不触碰** `state` 列。§5 会说明为什么用 `Save()` 就会踩雷 |
| **未知键必须活下来** | 技术文档 §5.6 预留了 `state.self_note`。所以**不能**把 `state` 定义成固定字段的 struct（读写一次就把未知键丢了），必须用原始 JSON 类型 |

> **`datatypes.JSON` vs `json.RawMessage`**：`datatypes.JSON` 本质上就是 `json.RawMessage`（`type JSON json.RawMessage`）外加 GORM 的 `Value()` / `Scan()` 实现，是 GORM 官方对 JSONB 的答案。裸 `json.RawMessage` 走 GORM + pgx 写 `jsonb` 列时可能被当成 `bytea` 发送，报 `column "state" is of type jsonb but expression is of type bytea`。**本功能新增一个依赖 `gorm.io/datatypes`**，`go.mod` 与 `go.sum` 都要提交（群里说一声）。

### 3.4 外键与索引

**外键必须由 GORM 建出来**，否则"删人设级联删消息"这条验收项不成立。只写 `UserID uint64` 标量字段，GORM **不会**创建外键约束——必须额外声明 belongs-to 关联：

```go
// 仅供 GORM 生成外键约束用；json:"-" 不让它进响应体
User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
```

`ProactiveSetting` 对 `Persona` 的关联同理——**它的外键也要 `ON DELETE CASCADE`**，否则删人设会留下孤儿配置行。

| 索引 | 用途 | 写法 |
|---|---|---|
| `idx_personas_user_last_msg` | 列表按 `last_message_at` 倒序，避免每次排序全表 | 两个字段共用一个 `index:` 名，`last_message_at` 带 `sort:desc` |

> **GORM 的 `index` tag 表达不了 `NULLS LAST`**，只能落到 `DESC`（Postgres 上即 `DESC NULLS FIRST`）。本项目每人设数 3-5 个，排序性能毫无压力，**不要为它手写 DDL**（红线 8）。**但查询本身必须显式写 `NULLS LAST`**——理由见 §4.2。

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

**`PageResult[T]` 的位置（已定）**：`backend/internal/dto/common_dto.go`。personas / messages / memory / moments / schedules **五个分页端点共用这一个**——成员 1、成员 3 都从这里引，**不要各自再定义一份**（两份同名类型在联调时会出现字段对不上的诡异问题）。

**Persona 响应结构（契约 §4 冻结）**

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
| 响应 data | `PageResult<Persona>` = `{ list, total, page, pageSize }` |
| 错误码 | 无业务错误码（`4010` 由中间件产生） |

**关于"越权"：列表端点不存在越权场景**。它**不带任何 `:id` 或 `personaId` 参数**，`user_id` 只来自 Token，所以查询天然被限制在当前用户名下——你只能看到自己的（可能为空的）列表，**没有任何"访问别人的列表"这种输入存在**。因此它不返回任何 403/404 族错误码，**空就是空**：

```json
{ "code": 200, "message": "success",
  "data": { "list": [], "total": 0, "page": 1, "pageSize": 20 }, "timestamp": 1789000000000 }
```

> **不要**给列表加"查不到就 4040"的逻辑：**没有**任何人设是**正常状态**（总纲 §0.3 明确要求前端做空状态引导），返回错误码会让前端无法区分"我还没有人设"和"请求失败"。

**四条容易写错的要求：**

1. **排序必须显式带 `NULLS LAST`**：

   ```sql
   ORDER BY last_message_at DESC NULLS LAST, id DESC
   ```

   Postgres 的规则是「NULL 比任何非 NULL 值都大」，所以 **`DESC` 的默认行为是 `NULLS FIRST`**——只写 `DESC` 的话，**刚创建、还没聊过的人设会排在最前面**，而需求恰恰相反（DDL 里索引也写的是 `NULLS LAST`，这是权威意图）。这是本功能最容易漏的一处，写错了页面看着"能用"，但顺序是错的。

2. **必须补 `id DESC` 兜底排序**：大量人设的 `last_message_at` 都是 `NULL`，只按它排序时同值行顺序不确定——翻页会出现「第 2 页冒出第 1 页的人」或漏项。加 `id DESC` 让顺序稳定。

3. **`total` 必须用同一个 `user_id` 条件统计**，不能只 `COUNT(*)` 全表。

4. **空列表要返回 `[]` 而不是 `null`**：Go 的 nil slice 会序列化成 `null`，前端 `v-for` / `.map` 直接崩。切片必须初始化（`list := make([]PersonaResponse, 0)`）。`page` 超出范围时返回空列表 + 真实 `total`，仍是 `200`。

### 4.3 `POST /personas` — 创建人设

| 项 | 内容 |
|---|---|
| 请求 | `{name, personalityDesc, speakingStyle}`，**三个都必填** |
| 响应 data | 新建的 `Persona` |
| 错误码 | `4001` |

**要求：**

1. **`user_id` 只能来自 Token**，绝不从 body 取。请求 DTO 里连这个字段都不该存在——字段不存在，就没有被前端传进来的可能（这是结构性防御，比"记得校验"可靠）。
2. **`state` 显式初始化为 `{"familiarity": 0}`**，不要依赖 DDL 的 `DEFAULT '{}'`。原因：读者可以无条件假设 `familiarity` 键存在，少一个分支（读侧仍要容错，见 §4.6）。
3. **不要做重名校验**：`name` 无 UNIQUE 约束，人设数量不限制（总纲 §0.3），重名合法。
4. 校验失败的语义：契约 §4 这组端点只列了 `4001`，所以 **`binding` 失败一律返回 `4001`**（`errcode.ErrInvalidParams`）。**不要**因为"缺必填字段"就改成 `4002`——那是契约里别的端点的语义，这里擅自细化就是契约漂移。
5. **必须在同一个事务里同步播种一行 `proactive_settings`**（见 §4.3.1）。
6. 响应里 `familiarity` 为 `0`、`lastMessageAt` 为 `null`（新建的人设必然没聊过）。

#### 4.3.1 创建人设时播种 `proactive_settings`（已定）

**行为**：`POST /personas` 成功时，`proactive_settings` 表必须存在一行对应该人设的配置，取 DDL 默认值：

| 列 | 值 | 来源 |
|---|---|---|
| `user_id` / `persona_id` | 同新建的人设 | — |
| `enabled` | `true` | DDL 默认 |
| `interval_min` | `30` | DDL 默认 |
| `interval_max` | `120` | DDL 默认 |
| `daily_limit` | `3` | DDL 默认 |
| `last_nudge_at` | `NULL` | 从未触发过 |

**为什么必须播种**：主动消息模块（成员 3 的另一块）靠 `WHERE persona_id = ?` 读这张表。如果创建人设时不留行，该人设的 `GET /proactive/settings` 就会查不到 → **报 404**，而这个 404 在演示现场表现为"某个 AI 伴侣的主动消息设置打不开"。**用"创建时播种"把这个问题消灭在源头**，也让 `GET /proactive/settings` 可以无条件返回真实行、不必写"无行则返回默认值"的兜底分支。

**两条实现约束：**

1. **同一个事务**：人设与配置行要么都成功、要么都不写入。若先提交人设、再插配置而后者失败，就会**恰好留下我们想避免的那种"没有人设配置的孤儿"**——所以事务边界必须在 service 层（红线 7：service 编排事务），repo 方法接受事务句柄。
2. **`UNIQUE (user_id, persona_id)`**：DDL 上已有该约束，播种用 `INSERT` 即可；**不要**写成 upsert 去掩盖"重复创建"的 bug——真重复了就该报错。

> **契约 §9 的接口行为不受本功能影响**：`GET /proactive/settings` 仍返回 `{personaId, enabled, intervalMin, intervalMax, dailyLimit, lastNudgeAt}`，本功能只是保证那行**一定存在**（不必写"无行则返回默认值"的兜底）。`PUT /proactive/settings` 允许用户改这四个可改字段，改完也是同一行。
> 它自己的归属校验错误码**本分支不动**——按契约现状实现，属 §5.2 末尾那 9 个待队长统一整改的端点之一。

### 4.4 `PUT /personas/:id` — 编辑人设

| 项 | 内容 |
|---|---|
| 请求 | `{name, personalityDesc, speakingStyle}`，**同 POST，三个都必填**（整体替换，不是部分更新） |
| 响应 data | 更新后的 `Persona` |
| 错误码 | `4001` **`4040`**（资源越权 / 不存在，**同一个码**，见 §5.2） |

**要求：**

1. **这是 PUT 不是 PATCH**：契约 §4 写的是"同上"，指与 POST 相同的请求体。三个字段全传、全必填。**不要参照 `PUT /user/profile` 的"只传要改的"来写**——那是另一个端点的约定。
2. **只更新这三列，别的一律不许动**：`state`、`last_message_at`、`created_at`、`user_id` 全部保持原值。**`proactive_settings` 也不动**（用户的主动消息配置与改人设名字无关）。
3. **响应里的 `state` / `familiarity` / `lastMessageAt` 必须是数据库里的真值**，不是请求体里的（请求体根本没有这些字段）。所以更新后要**重新读一次**该行再返回，否则要么拿到零值，要么要手工拼装——拼装容易漏。
4. **客户端多传的 `state` / `familiarity` 应被静默忽略**：Gin 默认忽略未知 JSON 字段，正合契约「`PUT` 时忽略」的要求。不要在 DTO 里声明这些字段，也不要开 `DisallowUnknownFields()`。

### 4.5 `DELETE /personas/:id` — 删除人设

| 项 | 内容 |
|---|---|
| 请求 | 无 body |
| 响应 data | `null` |
| 错误码 | **`4040`**（资源越权 / 不存在，**同一个码**，见 §5.2） |

**要求：**

1. **硬删除**（DDL 无 `deleted_at`，契约说"级联删除全部消息"，不是软删）。**注意别和 `schedules` 的软删搞混**——日程的 `DELETE` 是置 `status='cancelled'`，人设不是。
2. **级联交给数据库外键**，不要手写多表删除逻辑（成员 3 任务书 §2）。级联范围：`chat_messages`、`user_memory`、`user_profile`、`ai_moments`（→ 再级联 `moment_comments` / `moment_likes`）、**`proactive_settings`**、`schedules`。
3. **幂等性**：删第二次返回 `4040`（与"不存在"同一个码，天然幂等且不泄漏）。前端二次确认后调用；如果前端重复提交，第二次是 4040 而不是 200——按契约实现即可，不要为它特判。

### 4.6 请求 / 响应结构与 `familiarity` 的拍平

| 结构 | 定义位置 | 字段 | 校验 |
|---|---|---|---|
| `PageResult[T]` | **`dto/common_dto.go`** | `list` `total` `page` `pageSize` | — |
| `CreatePersonaRequest` | `dto/persona_dto.go` | `name` `personalityDesc` `speakingStyle` | 三者 `required`；`name` `max=50`、`speakingStyle` `max=255`（对齐 DDL）、`personalityDesc` 加一个防御性上限 |
| `UpdatePersonaRequest` | 同上 | 同上（可另起一个类型，也可复用） | 同上 |
| `PersonaResponse` | 同上 | `id` `name` `personalityDesc` `speakingStyle` `state` `familiarity` `lastMessageAt` `createdAt` | — |
| `PersonaListResponse` | 同上 | `list []PersonaResponse` `total` `page` `pageSize`（或直接用 `PageResult[PersonaResponse]`） | — |

**`familiarity` 的提取（定义在 dto 层，不写在 model 上）**：`state` 用 `datatypes.JSON` 原样保留，另起一个只含 `familiarity` 的小 struct 做一次 `json.Unmarshal` 取值：

```go
// 只解需要的键；其余未知键（如将来的 self_note）留在 State 原文里，不受影响
var s struct {
    Familiarity int `json:"familiarity"`
}
_ = json.Unmarshal(p.State, &s)   // 解析失败不报错，familiarity 取 0
```

**为什么容错**：DDL 默认值是 `'{}'`（键不存在），手工插入的历史数据、或将来别的模块写入的 `state` 都可能没有这个键。解析失败或键缺失时取 `0`（语义 = "初识"），**不要让整个列表接口因此 500**。

> ⚠️ **契约文档的一处不一致**（不是行为变更，是文档瑕疵）：§4 的示例同时给出 `"state": {}` 与 `"familiarity": 12`，看起来像是 `familiarity` 独立于 `state`。但 DDL 注释与 §5.6 都指明它存在 `state` 里。以 DDL 为准；**实现后提醒成员 2 一句**，免得他写 Mock 时按"独立列"理解。

## 5. 越权防线（本功能最重要的正确性约束）

> 对应红线 3：**❌ 跨用户 / 跨人设数据泄漏**。人设是这套防线的主闸门——这里失守，`user_memory` / `user_profile` / `schedules` / `chat_messages` 全部跟着失守，因为它们都是拿 `persona_id` 去查的。

### 5.1 三层防线

**① 入口层（Handler）——`user_id` 只有一个来源**

- `user_id` 只能从 JWT 声明取（成员 1 的 `JWTAuth` 中间件写入 `gin.Context`），**永不从 body / query / path / header 读**。
- 契约 §7 已写明："前端永远不传 `user_id`"。
- **类型是 `uint64`**（已定，与 `user.go` 一致）；用 `c.GetUint64(...)` 读。
- 取不到（类型断言失败 / `0`）→ **返回 `4010`，直接中断**，绝不退化成 `userID = 0` 继续查。因为 `WHERE user_id = 0` 会返回空集——**看起来没泄漏，实际是把鉴权失败伪装成了空列表**，这种"静默降级"比报错危险得多。
- 请求 DTO 里**不声明** `userId` 字段：字段不存在就无法被传入。

**② 仓储层（Repository）——让"只按 id 查"这件事在结构上不可用**

- 每个方法的签名**强制**带 `userID uint64`：`ListByUser(ctx, userID, offset, limit)`、`UpdateOwned(ctx, tx, userID, personaID, ...)`、`DeleteOwned(ctx, tx, userID, personaID)`。
- 归属校验**下沉到 SQL 的 `WHERE` 里**，而不是"先 `First(&p, id)` 查出来、再在 Go 里 `if p.UserID != userID`"。后者有两个问题：多一次往返；以及**忘写那个 `if` 就静默越权**——这类 bug 在 code review 里也容易被漏掉，因为它看起来"逻辑完整"。
- **仓储层不提供任何"按 id 单查"或"判断某 id 是否存在"的方法**。§5.2 选定"不泄漏存在性"后，"存在性判断"这个能力在代码里**根本不需要出现**——少一个方法就少一处泄漏面。
- 事务句柄作为显式参数传入（`tx *gorm.DB`），事务边界由 service 决定。

**③ 数据层（DB）**

- `personas.user_id` 上的外键 + 组合索引（§3.4）。
- 下游子表查询一律 `persona_id + user_id` **双条件**（技术文档 §4.3）：只带 `persona_id` 会跨用户串号（它是全局自增，别的用户的同号人设会命中），只带 `user_id` 会跨人设。**记忆与画像尤其是重灾区**，那不是本功能的代码，但本功能的归属校验是它们的第一道闸门。

### 5.2 错误码：资源越权 → `4040`（队长规则，2026-09-13）

> **团队通用规则**（队长 2026-09-13 明确）
>
> | 越权类型 | 定义 | 错误码 |
> |---|---|---|
> | **资源越权** | 功能你能用，但这个**资源不属于你**（如用户 A 修改用户 B 的 persona） | **`4040`** —— 隐藏资源存在性 |
> | **功能越权** | 功能本身你无权使用（如普通用户访问管理员接口） | **`4031`** |
>
> 判据是**"越的是资源，还是功能"**，不是"属于哪个模块"。**人设 CRUD 全程属资源越权侧。**

**本功能的应用**：`PUT` / `DELETE /personas/:id` 的 `WHERE id = ? AND user_id = ?` 未命中，**一律返回 `4040`**（`errcode.ErrNotFound`），**不区分**「人设是别人的」与「人设不存在」。

**为什么"不存在"也不能给 `4043`（规则的关键推论）**：`id` 是 `BIGSERIAL` 全局自增。若"别人的"回 `4040`、"不存在的"回 `4043`，调用方拿同一个 id 各打一次，就能判断这个 id **到底存不存在**——隐藏存在性的目的当场失效。所以**同一个端点内 `4040` 与 `4043` 不能并存**：本模块 `:id` 未命中只有 `4040` 一个答案。

**本模块没有功能越权场景**：契约 §4 的 4 个端点对所有登录用户一视同仁（本项目无管理员接口、无角色区分），所以**人设模块不会产生 `4031`**。将来若出现真正的功能级权限区分，`4031` 才派上用场。

**代价（诚实记录）**：调试时"改别人的人设"与"改不存在的 id"返回同一个码，排查略麻烦——靠服务端日志区分（`BizErrorHandler` 会为 4xxx 记 warn 日志）。

**实现上反而更简单（附带收益）**：不需要"先带归属条件查、未命中再按 id 探针二分"的两步判定——两步判定的**唯一目的**就是区分这两个码。现在一条 `WHERE id = ? AND user_id = ?` 就够，未命中直接 `4040`，正常路径与异常路径都是**一次查询**。仓储层因此不需要 `ExistsByID`（§5.1 ②）。

**⚠️ 契约尚未写入这条规则 —— 走完流程再合 PR：**

| # | 动作 | 具体位置 |
|---|---|---|
| 1 | **群里广播规则本身**（不只是广播"我改了"） | 成员 2 的 Mock **直接受影响**（他按 `4031`/`4043` 写的错误分支要改） |
| 2 | 更新 `API_CONTRACT.md` §4 端点表 | `PUT /personas/:id` 错误码 `4001 4031 4043` → `4001 4040`；`DELETE /personas/:id` 的 `4031 4043` → `4040` |
| 3 | 更新 `API_CONTRACT.md` §4 末尾说明 | line 185「否则返回 `4031`」改写为「否则返回 `4040`（资源越权，不区分不存在与不属于当前用户，避免泄漏存在性）」 |
| 4 | 登记 `API_CONTRACT.md` §12 变更记录 + 升版本号 | v1 → v2（契约纪律：任何变更必须登记并广播） |
| 5 | **更新 `AGENTS.md` §4.3** | 该节现有「涉及 `:id` 的人设操作必须校验归属，失败返回 `4031`」——**不改这里，后续 AI 编码代理会照着它把 `4031` 加回来** |
| 6 | 更新前端 `types/errcode.ts` | 若成员 2 已把 `4031` 写成"人设越权"分支，需要同步 |
| 7 | **顺带解决 `4031` 的文案** | 契约 §2 里 `4031` 的 msg 是「该人设不属于当前用户」——那是**资源越权**的说法，与它现在承担的语义（功能越权）冲突，需一并改写 |

> **⚠️ 这是通用规则，波及面比 §4 大 —— 但不由本 PR 承包。**
> 按新规则，契约里**另有 10 个端点同样在做归属校验，且同属资源越权**，它们的错误码也要跟着收敛（清单已按契约逐行核对，可直接交给队长）：

| # | 端点（契约行） | 现状错误码 | 按新规则应为 | 改法 |
|---|---|---|---|---|
| 1 | `GET /chat/personas/:personaId/messages`（:216） | `4031 4043` | `4040` | 换码 |
| 2 | `POST /chat/stream`（:217，SSE） | `4001 4010 4031 4043 5001 5002` | `4001 4010 4040 5001 5002` | 换码 |
| 3 | `GET /memory`（:281） | `4031 4043` | `4040` | 换码 |
| 4 | `GET /profile/portrait`（:282） | `4031 4043` | `4040` | 换码 |
| 5 | `GET /proactive/settings`（:358） | `4031 4043` | `4040` | 换码 |
| 6 | `PUT /proactive/settings`（:359） | `4001 4031 4043` | `4001 4040` | 换码 |
| 7 | `POST /proactive/trigger`（:360） | `4031 4043 5001` | `4040 5001` | 换码 |
| 8 | `GET /schedules`（:402） | `4001 4031 4043` | `4001 4040` | 换码 |
| 9 | `DELETE /schedules/:id`（:403） | `4031 4040` | `4040` | **只删 `4031`** |
| 10 | `POST /schedules/:id/trigger`（:404） | `4031 4040 5001` | `4040 5001` | **只删 `4031`** |

> 第 1-8 条是"`4031`/`4043` 换 `4040`"；第 9、10 条（日程的两个 `:id` 端点）**已经带 `4040` 了**，只是多带了一个 `4031`——**只删那一个码**即可。这两条恰好说明「一个端点里 `4031` 与 `4040` 并存」是当前契约的既有形态，也正是新规则要清理的。
>
> **本分支只改人设端点。** 那 10 个端点的代码不是我 Own 的模块，契约行也还没改——在本 PR 里顺手改别人的 handler / service 会让 review 无法界定范围，而且"代码改了、契约没改"等于制造新的不一致。
> **正确做法**：把上面这张表**连同行号交给队长**（§7.3 待确认 #4），由契约与各模块负责人**一次性统一扫干净**，避免"半套规则"长期共存。

### 5.3 反例清单（AI 最容易写出来的错法）

| ❌ 错法 | 后果 | ✅ 正确 |
|---|---|---|
| `db.First(&p, id)` 再在 Go 里比 `UserID` | 忘写比较就静默越权；多一次往返 | `WHERE id = ? AND user_id = ?`，查 `RowsAffected` |
| 未命中时返回 `4031` 区分"别人的" | 违反资源越权规则；且泄漏 id 存在性，可被逐号枚举 | 统一 `4040`（§5.2） |
| 用 `4043` 表示"不存在"、`4040` 表示"不属于你" | **两者并存就能被二分探测出 id 是否存在**，等于没隐藏 | 都收敛到 `4040`（§5.2） |
| 为区分而写 `ExistsByID` 探针 | 让"存在性"这个概念进入代码，白增泄漏面与一次查询 | **不写这个方法** |
| 列表只 `WHERE persona_id`/不带条件 | **跨用户泄漏全部人设** | 一律 `WHERE user_id = ?`（从 Token 取） |
| 从 body/query 读 `userId` | 攻击者传谁的 id 就看谁的数据 | 只从 JWT 上下文取 |
| 取不到 userID 时用 `0` 兜底继续查 | 鉴权失败伪装成空列表 | 返回 `4010` 并中断 |
| `db.Save(&p)` 更新 | **`state` 被零值覆盖、`last_message_at` 被清空** | 只 `Updates` 指定列（见 plan §3.2） |
| 先提交人设、再插 `proactive_settings` | 后者失败就留下没人设配置的孤儿（正是 §4.3.1 要避免的） | 同一个事务（service 层编排） |
| `DELETEmessages` 手写多表清理 | 漏一张表就留脏数据 | 交外键 `ON DELETE CASCADE` |
| `order by last_message_at desc`（无 `NULLS LAST`） | 没聊过的人设排最前 | `DESC NULLS LAST, id DESC` |

## 6. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| 字段名**全 camelCase**，与契约 §4 逐字一致（`personalityDesc` 不是 `personality_desc`） | 协作规范 §5 / 契约 §1 |
| **不新增错误码**；本功能只用 `ErrInvalidParams`(4001) / `ErrNotFound`(4040) / `ErrDBFailed`(5003) | 技术文档 §4.3 分段规则 |
| **按通用规则：资源越权 → `4040`**；本模块无功能越权场景，**不产生 `4031`**；也不返回 `4043`（与 `4040` 并存即可被探测出存在性）。但**不要**去删 `pkg/errcode` 里的常量（其他端点仍在用，且那是成员 1 的文件） | 队长规则 2026-09-13 ｜ §5.2 |
| **禁止硬编码错误码数字或文案**；`Fail()` 只接受 `ErrorCode` | **红线 6** |
| handler **不写** `response.Fail`，错误 `_ = c.Error(err)` 上抛 | 技术文档 §4.4 |
| handler 不直接操作 DB；service **不依赖 `*gin.Context`**（只收 `context.Context`） | **红线 7** / 技术文档 §4.1 |
| **多表写入（人设 + 主动消息配置）的事务边界在 service 层**，repo 只接受事务句柄 | 技术文档 §4.1 / 红线 7 |
| 归属校验必须在 service 层，`4040` 由 service 返回 | 成员 3 任务书 §2 |
| 分支 `feature/backend-persona-model`，走 PR，**禁止直推 `main` / `develop`**；commit 格式 `<type>(scope): <subject>`，scope 用 `model` / `persona` | **红线 2** / 协作规范 §6 |
| **不提交任何密钥**：本功能不新增任何 Key/Token/密码；禁止把 DSN/密码写进代码或 `_test.go` | **红线 1** |
| 改表结构只改 struct + `AutoMigrate`，**禁止手写 `ALTER TABLE` / `DropTable`** | **红线 8** |
| 表名 `personas` / `proactive_settings`、列名小写下划线；Go 缩写词全大写（`UserID` 不是 `userId`）；ID 一律 `uint64` | 协作规范 §5 |
| **契约变更（§5.2）未广播、`API_CONTRACT.md` 未更新前，不得合并 PR** | 契约纪律 |

**红线自查（AGENTS.md §7 逐条对照）**

| 红线 | 本功能是否涉及 |
|---|---|
| ① 提交 `.env` / Key / 密码 / 模型权重 | 不涉及新增；已核对 `.gitignore` 生效（`deploy/.env` 被忽略，仓库只留 `.env.example`）。**测试代码里也不许出现密码** |
| ② 直推 `main` / `develop` / force push | 流程约束：本分支走 PR，至少 1 人 Approve |
| ③ 跨用户 / 跨人设泄漏 | **主要战场**，见 §5 |
| ④ 明文 / 弱哈希存密码 | 不涉及（不碰密码） |
| ⑤ 情绪展示标签 / 朋友圈手动触发 / 日程新建端点 | 不涉及。**注意 `state` 是人格状态不是情绪**，别往情绪上靠 |
| ⑥ 硬编码错误码或文案；`Fail()` 传自定义 message | 见 §6 上表 |
| ⑦ handler 直接操作 DB；service 依赖 `*gin.Context`；组件写 axios | 见 §6 上表 |
| ⑧ 生产 `DropTable`；手写 `ALTER TABLE` | 见 §6 上表 |

## 7. 依赖与交接

### 7.1 我依赖谁（**注意：有硬阻塞**）

| 依赖 | 提供方 | 现状 | 影响 |
|---|---|---|---|
| `pkg/response`（`Success[T]` / `Fail`） | 成员 1 | **仓库中尚不存在** | handler 无法编译，直到它落地 |
| `pkg/errcode`（4001/4040/5003 常量） | 成员 1 | **仓库中尚不存在** | service 无法编译 |
| `internal/middleware`（`JWTAuth` + `ContextKeyUserID` + `BizErrorHandler`） | 成员 1 | **仓库中尚不存在** | 越权防线取不到 `userID` |
| `internal/config` + `cmd/server/main.go` + `db` 连接 | 成员 1 | **仓库中尚不存在** | 无法起服务验证 |
| `router.go` 的挂载点 | 成员 1 | **仓库中尚不存在** | 端点不可达 |

> **当前的仓库状态**（2026-09-13）：`backend/` 下只有 `go.mod` / `go.sum` / `internal/model/user.go`。**成员 1 的公共层还没交付**。因此实现顺序必须先把**不依赖公共层的文件**做完，不能等。
> **可以立即开工**：`model/persona.go`、`model/proactive_setting.go`、`model/migrate.go`、`dto/common_dto.go`、`dto/persona_dto.go`、`repository/persona_repo.go`。
> **需要公共层才能编译**：service（`errcode`）、handler（`response` + `middleware`）。
> **验证策略**：公共层没到位前，repo 层可以先用一个临时的 `_test.go`（本地 PG）验证 SQL 正确性；**不要**为了"跑起来"去自己造一份 `pkg/errcode`——那会与成员 1 的版本冲突（红线：不 Own 的东西不要动）。

### 7.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `internal/model/persona.go` | 成员 1 | 他写 `message_repo` 要引 `Persona`；`chat_messages.persona_id` 的归属校验挂在它上面 |
| `internal/dto/common_dto.go` | **成员 1、成员 3** | 五个分页端点共用的 `PageResult[T]`，**别再各写一份** |
| `GET /personas`（同时就是对话列表） | 成员 2 | 聊天页侧栏、路由 `/chat/:personaId` 的跳转目标 |
| `Persona` 响应结构 | 成员 2 | 写 `types/persona.ts` 与 Mock；**字段名逐字对齐 camelCase** |
| `proactive_settings` 一定存在一行 | 成员 3（主动消息模块） | `GET /proactive/settings` 可以无条件查表，不必写"无行则返回默认值"的兜底 |
| ⚠️ **错误码规则：资源越权 → `4040`** | **全员**（成员 2 的 Mock；成员 1 的 memory/chat；成员 3 的 proactive/schedules） | **必须广播的是规则本身，不只是"我改了人设"**：人设端点先落地；契约里**另 10 个**带归属校验的端点按同一规则也应为 `4040`（逐行清单见 §5.2 末尾），但契约与代码由队长统一安排——**不在本 PR 内顺手改** |

### 7.3 决策记录与剩余待确认

**已定（本版并入，不再讨论）：**

| # | 决策 | 落点 |
|---|---|---|
| 1 | `PageResult[T]` 统一定义在 `backend/internal/dto/common_dto.go` | §4.1 |
| 2 | ID 类型统一 `uint64`，与 `user.go` 一致 | §3.2 / §5.1 ① |
| 3 | `POST /personas` 同事务播种一行 `proactive_settings` | §4.3.1 |
| 4 | `:id` 未命中统一返回 `4040`（不区分不属于你 / 不存在） | §5.2 |
| 5 | **错误码按队长通用规则**：**资源越权 → `4040`**（隐藏资源存在性）、**功能越权 → `4031`**。人设 CRUD 属资源越权，故 `:id` 未命中一律 `4040`，且**不产生 `4031`** | §5.2 |

**剩余待确认（实现前在群里问清，不要自己拍板）：**

| # | 问题 | 建议 |
|---|---|---|
| 1 | 新增 `gorm.io/datatypes` 依赖是否可接受？ | 建议接受（GORM 官方 JSONB 类型）；需广播，`go.mod` + `go.sum` 一并提交 |
| 2 | `proactive_settings` 的四个默认值是否就用 DDL 的 30/120/3？ | 建议照 DDL（§4.3.1 表）。若产品另有默认，需与主动消息模块一起定 |
| 3 | 时间字段精度：Go 默认 `RFC3339Nano`（带小数秒） | 与现有 `user.go` 一致即可；前端 `new Date()` 能正常解析。若要严格到秒，需自定义时间类型——**本期不必要** |
| 4 | **通用规则的落地范围**：除人设外，那 10 个端点何时统一为 `4040`？ | 已确认**不在本分支改**（用户去群里广播，`API_CONTRACT.md` / `AGENTS.md` 是全局文件）。建议队长排一次"错误码统一"小任务，把 §4 + `§5/§7/§9/§10` 那 10 行 + `AGENTS.md` §4.3 + `4031` 文案一起改干净（清单连带行号见 §5.2 末尾）；本 PR 只负责 §4 人设端点 |

## 8. 验收标准

- [ ] `cd backend && go build ./... && go vet ./... && go test ./...` 全通过
- [ ] `AutoMigrate` 建出 `personas` 与 `proactive_settings`；`psql` 里 `\d personas` 能看到 **`ON DELETE CASCADE` 外键**与组合索引
- [ ] 4 个端点 curl 全通：列表 / 创建 / 编辑 / 删除
- [ ] **列表空态**：无任何人设时 `data.list` 是 `[]` 而不是 `null`，且 `code` 为 `200`
- [ ] **资源越权（`:id`）**：用 B 的 Token 打 A 的 `personaId` → **`4040`**（`PUT` 与 `DELETE` 各验一次），且响应 `message` 与下一条**逐字相同**
- [ ] **不存在**：不存在的 id → **`4040`**（与上一条**同码同文案**，这正是 §5.2 隐藏存在性要的效果）
- [ ] **不产生 `4031`**：4 个端点在任意输入组合下（越权 / 不存在 / 缺参 / 无 Token / 超长字段）**都不出现 `4031`**，也不出现 `4043`——本模块没有功能越权场景（§5.2）
- [ ] 不带 Token / Token 无效 → `4010` / `4011`（由中间件产生）
- [ ] 校验失败（缺 `name`、超长）→ `4001`
- [ ] **级联删除**：删人设后 `SELECT count(*) FROM chat_messages WHERE persona_id = <已删id>` = 0
- [ ] **播种**：`POST /personas` 后 `SELECT * FROM proactive_settings WHERE persona_id = <新id>` 有且仅有 1 行，四个默认值正确、`last_nudge_at IS NULL`
- [ ] **事务性**：人为让 `proactive_settings` 插入失败（如临时加一个非法约束/在测试里注入错误）→ **`personas` 不得残留新行**
- [ ] **`PUT` 不误伤**：改人设名字后 `proactive_settings` 行**未被修改**（`enabled`/`interval_*`/`daily_limit` 原值不变）
- [ ] **排序**：新建一个从未聊过的人设，它排在列表**最后**（不是最前）
- [ ] **`state` 保留**：把某行 `state` 手工改成 `{"familiarity": 42, "self_note": "x"}`，`PUT` 之后两个键都还在，`familiarity` 仍为 42
- [ ] **`familiarity` 正确**：`GET` 返回顶层 `familiarity`，且 `state` 原样透传
- [ ] code 评审 grep 通过：无硬编码错误码数字/文案、无 `response.Fail` in handler、无 secrets、无 `Save(` on persona
- [ ] **规则已广播**，且 `API_CONTRACT.md`（§4 + §12 + 版本号 + §2 的 `4031` 文案）与 `AGENTS.md` §4.3 已由队长 / 成员 1 同步更新（清单见 §5.2 表、§7.3 #4）。**本分支不直接改这两个全局文件**
- [ ] PR 已开、至少 1 人 Approve；commit 符合 `<type>(<scope>): <subject>`

## 9. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-13 | v1 | 创建 | 人设 CRUD 开工前的设计与验收基线；含越权防线与契约偏差记录 |
| 2026-09-13 | v2 | 并入 4 项决策：① `PageResult[T]` → `internal/dto/common_dto.go`；② ID 统一 `uint64`；③ `POST /personas` 同事务播种 `proactive_settings`（新增 §4.3.1、新增 `model/proactive_setting.go`、`dto/common_dto.go`）；④ **`:id` 未命中统一 `4040`**，取代契约的 `4031`/`4043`（§5.2 改写，含广播与文档更新清单） | 评审决策 |
| 2026-09-13 | v3 | **套用队长「通用错误码规则」**：资源越权 → `4040`（隐藏存在性）、功能越权 → `4031`。§5.2 由「契约偏离」重写为「规则的应用」，补两条推论：①「不存在」也不能用 `4043`（与 `4040` 并存即可被二分探测）；②本模块无功能越权场景，**不产生 `4031`**。§7.2 交接项改为「广播规则本身 + 10 个端点待队长统一」（清单已按契约行号逐条核对并附于 §5.2），§8 新增「不产生 `4031`」验收项。**全局文件（`API_CONTRACT.md` / `AGENTS.md`）不在本分支改动** | 队长裁决 |

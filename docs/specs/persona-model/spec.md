# spec · 人设 CRUD（Persona CRUD）

> 功能名：persona-model ｜ 分支：`feature/backend-persona-model`
> 负责人：成员 3（数据 + 人设 + 部署） ｜ 状态：设计完成，待实现
> 创建：2026-09-13 ｜ 最后更新：2026-09-13
> 关联：[AGENTS.md](../../../AGENTS.md) ｜ [接口契约 §4](../../API_CONTRACT.md) ｜ [技术文档 §6.2 / §5.6](../TECH_DESIGN.md) ｜ [成员 3 任务书 §2](../dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md)

---

## 1. 背景与目标

一个人设 = 一个 AI 伴侣 = 一个对话。`personas` 表既是「人设的存储」，**同时就是对话列表**（技术文档 §6.1：没有会话表）。因此本功能是全项目第一个业务模块，也是三个页面（人设管理页、聊天页、画像页）的共同前置。

**目标（一句话）**：交付 `personas` 的 GORM 实体与 4 个 CRUD 端点，字段与契约逐字一致，且**任何接口都不可能读到别人的数据**。

**为什么它在关键路径上**：聊天页（成员 2）靠 `GET /personas` 决定打开哪个对话；记忆、画像、日程（成员 1 / 成员 3）全部挂在 `persona_id` 上——人设的归属校验是它们共同的第一道闸门，这里漏了，后面每张子表都跟着漏。

## 2. 范围

### 2.1 做什么（In Scope）

| 项 | 产物 |
|---|---|
| 数据模型 | `internal/model/persona.go`（GORM 实体，含外键级联声明） |
| 建表 | `internal/model/migrate.go` 的 `AutoMigrate` 追加 `&Persona{}` |
| 请求 / 响应结构 | `internal/dto/persona_dto.go` |
| 数据访问 | `internal/repository/persona_repo.go` |
| 业务逻辑 | `internal/service/persona_service.go` |
| HTTP 接口 | `internal/handler/persona_handler.go`（含 `RegisterPersonaRoutes`） |
| 路由挂载 | `router.go` 加一行 `RegisterPersonaRoutes(api, personaHandler)`（与成员 1 错开时间改，见总纲 §4.5） |

### 2.2 不做什么（Out of Scope）

| 不做的事 | 归属 / 原因 |
|---|---|
| 人设管理页 UI、`api/persona.ts`、`stores/persona.ts` | 本功能**只做后端**；前端页面是独立一步（成员 3 任务书 §3） |
| 空状态引导（"还没有人设，去创建一个"） | 前端职责 |
| 人设数量上限、人设名去重 | 总纲 §0.3：**人设数量不限制**；DDL 上 `name` 无 UNIQUE，重名是合法的，**不要自作主张加 `4004` 冲突校验** |
| `familiarity` 的累加逻辑 | 归对话链路（技术文档 §5.6）：每轮对话结束 +1。本功能**只读展示**，不写 |
| 对话历史清空 | 总纲 §0.2：**不可单独清空**，只能随人设一起删 |
| `state.self_note` 等演化字段的写入 | 技术文档 §5.6 的加分项，本功能只负责**原样保留**，不解析、不写入 |
| 情绪相关任何字段 | 人设表没有情绪字段；`state` 是人格状态，**不是情绪**，不要混为一谈 |
| 新增错误码 | 本功能**一个都不需要新增**，复用 `4001` / `4031` / `4043` / `5003` |
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

**三条必须记住的结论：**

1. **`user_id` 不进响应体**：契约 §4 的 Persona 实体只有 8 个字段，其中没有 `userId`。用 `json:"-"` 关掉（与 `User.PasswordHash` 同一手法）——它是越权防线的载体，不是给前端看的。**不要为了"方便调试"把它改成 `json:"userId"`**。
2. **`familiarity` 不是数据库列**，它存在 `state` JSONB 里（DDL 注释与 §5.6 明确：`{"familiarity": 0}`）。响应体里它是**顶层字段**，需要从 `state` 里解出来再拍平。反过来 `state` 要**原样透传**，不能被重新序列化丢掉未知键。
3. **`lastMessageAt` 必须是指针**：契约要求"从未聊过为 `null`"，值类型 `time.Time` 会序列化成 `"0001-01-01T00:00:00Z"`，这是错的。

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
| 分页响应 | `{ list, total, page, pageSize }` |
| 时间格式 | RFC3339 |
| 错误出口 | handler 一律 `_ = c.Error(err)` 上抛，由 `BizErrorHandler` 中间件统一出口；**handler 里不出现 `response.Fail`** |

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

### 4.4 `PUT /personas/:id` — 编辑人设

| 项 | 内容 |
|---|---|
| 请求 | `{name, personalityDesc, speakingStyle}`，**同 POST，三个都必填**（整体替换，不是部分更新） |
| 响应 data | 更新后的 `Persona` |
| 错误码 | `4001` `4031` `4043` |

**要求：**

1. **这是 PUT 不是 PATCH**：契约 §4 写的是"同上"，指与 POST 相同的请求体。三个字段全传、全必填。**不要参照 `PUT /user/profile` 的"只传要改的"来写**——那是另一个端点的约定。
2. **只更新这三列，别的一律不许动**：`state`、`last_message_at`、`created_at`、`user_id` 全部保持原值。
3. **响应里的 `state` / `familiarity` / `lastMessageAt` 必须是数据库里的真值**，不是请求体里的（请求体根本没有这些字段）。所以更新后要**重新读一次**该行再返回，否则要么拿到零值，要么要手工拼装——拼装容易漏。
4. **客户端多传的 `state` / `familiarity` 应被静默忽略**：Gin 默认忽略未知 JSON 字段，正合契约「`PUT` 时忽略」的要求。不要在 DTO 里声明这些字段，也不要开 `DisallowUnknownFields()`。

### 4.5 `DELETE /personas/:id` — 删除人设

| 项 | 内容 |
|---|---|
| 请求 | 无 body |
| 响应 data | `null` |
| 错误码 | `4031` `4043` |

**要求：**

1. **硬删除**（DDL 无 `deleted_at`，契约说"级联删除全部消息"，不是软删）。**注意别和 `schedules` 的软删搞混**——日程的 `DELETE` 是置 `status='cancelled'`，人设不是。
2. **级联交给数据库外键**，不要手写多表删除逻辑（成员 3 任务书 §2）。级联范围：`chat_messages`、`user_memory`、`user_profile`、`ai_moments`（→ 再级联 `moment_comments` / `moment_likes`）、`proactive_settings`、`schedules`。
3. **幂等性**：删第二次返回 `4043`（契约未承诺人设删除幂等，与日程的"重复删除幂等"不同）。前端二次确认后调用；如果前端重复提交，第二次是 4043 而不是 200——按契约实现即可，不要为它特判。

### 4.6 请求 / 响应结构与 `familiarity` 的拍平

| 结构 | 字段 | 校验 |
|---|---|---|
| `CreatePersonaRequest` | `name` `personalityDesc` `speakingStyle` | 三者 `required`；`name` `max=50`、`speakingStyle` `max=255`（对齐 DDL）、`personalityDesc` 加一个防御性上限 |
| `UpdatePersonaRequest` | 同上（可另起一个类型，也可复用） | 同上 |
| `PersonaResponse` | `id` `name` `personalityDesc` `speakingStyle` `state` `familiarity` `lastMessageAt` `createdAt` | — |
| `PersonaListResponse` | `list []PersonaResponse` `total` `page` `pageSize` | — |

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
- 取不到（类型断言失败）→ **返回 `4010`，直接中断**，绝不退化成 `userID = 0` 继续查。因为 `WHERE user_id = 0` 会返回空集——**看起来没泄漏，实际是把鉴权失败伪装成了空列表**，这种"静默降级"比报错危险得多。
- 请求 DTO 里**不声明** `userId` 字段：字段不存在就无法被传入。

**② 仓储层（Repository）——让"只按 id 查"这件事在结构上不可用**

- 每个方法的签名**强制**带 `userID uint64`：`ListByUser(ctx, userID, offset, limit)`、`UpdateOwned(ctx, userID, personaID, ...)`、`DeleteOwned(ctx, userID, personaID)`。
- 归属校验**下沉到 SQL 的 `WHERE` 里**，而不是"先 `First(&p, id)` 查出来、再在 Go 里 `if p.UserID != userID`"。后者有两个问题：多一次往返；以及**忘写那个 `if` 就静默越权**——这类 bug 在 code review 里也容易被漏掉，因为它看起来"逻辑完整"。
- 唯一允许按 id 单查的场景是 §5.2 的归属判定探针，写成一个语义明确、不被业务复用的内部方法。

**③ 数据层（DB）**

- `personas.user_id` 上的外键 + 组合索引（§3.4）。
- 下游子表查询一律 `persona_id + user_id` **双条件**（技术文档 §4.3）：只带 `persona_id` 会跨用户串号（它是全局自增，别的用户的同号人设会命中），只带 `user_id` 会跨人设。**记忆与画像尤其是重灾区**，那不是本功能的代码，但本功能的归属校验是它们的第一道闸门。

### 5.2 `4031` 与 `4043` 怎么判（含一处泄漏权衡）

契约 §4 为 `PUT` / `DELETE` 同时列了 `4031`（该人设不属于当前用户）与 `4043`（人设不存在），所以这两个码必须都能返回。判定次序：

1. 先用**带归属条件**的查询命中：`WHERE id = ? AND user_id = ?`
2. 命中 → 正常执行
3. 未命中 → 再用**仅按 id** 的存在性探针二分：
   - 存在（是别人的）→ **`4031`**
   - 不存在 → **`4043`**

**权衡（诚实记录）**：第 3 步会泄漏"这个 id 存在但不属于你"这一比特信息。**为什么仍然这么做**：契约已冻结这两个码的语义（`error codes 4031 4043`），而 `id` 是 `BIGSERIAL` 全局自增，攻击者本来就能从自增序列推断存在性——为这一比特去违背契约不划算。**替代方案**（更保守但要改契约）：未命中一律返回 `4031`，等于把 `4043` 从这两个端点移除。**若评审时倾向保守，需先在群里广播并更新 `API_CONTRACT.md`**，不要默默换。

**实现要点**：正常路径只花 1 次查询，只有"未命中"的异常路径才多一次探针查询——不要为了判定把两次查询都放在正常路径上。

### 5.3 反例清单（AI 最容易写出来的错法）

| ❌ 错法 | 后果 | ✅ 正确 |
|---|---|---|
| `db.First(&p, id)` 再在 Go 里比 `UserID` | 忘写比较就静默越权；多一次往返 | `WHERE id = ? AND user_id = ?`，查 `RowsAffected` |
| 列表只 `WHERE persona_id`/不带条件 | **跨用户泄漏全部人设** | 一律 `WHERE user_id = ?`（从 Token 取） |
| 从 body/query 读 `userId` | 攻击者传谁的 id 就看谁的数据 | 只从 JWT 上下文取 |
| 取不到 userID 时用 `0` 兜底继续查 | 鉴权失败伪装成空列表 | 返回 `4010` 并中断 |
| `db.Save(&p)` 更新 | **`state` 被零值覆盖、`last_message_at` 被清空** | 只 `Updates` 指定列（见 plan §3） |
| `DELETEmessages` 手写多表清理 | 漏一张表就留脏数据 | 交外键 `ON DELETE CASCADE` |
| `order by last_message_at desc`（无 `NULLS LAST`） | 没聊过的人设排最前 | `DESC NULLS LAST, id DESC` |

## 6. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| 字段名**全 camelCase**，与契约 §4 逐字一致（`personalityDesc` 不是 `personality_desc`） | 协作规范 §5 / 契约 §1 |
| **不新增错误码**；只用 `ErrInvalidParams`(4001) / `ErrPersonaNotOwned`(4031) / `ErrPersonaNotFound`(4043) / `ErrDBFailed`(5003) | 技术文档 §4.3 分段规则 |
| **禁止硬编码错误码数字或文案**；`Fail()` 只接受 `ErrorCode` | **红线 6** |
| handler **不写** `response.Fail`，错误 `_ = c.Error(err)` 上抛 | 技术文档 §4.4 |
| handler 不直接操作 DB；service **不依赖 `*gin.Context`**（只收 `context.Context`） | **红线 7** / 技术文档 §4.1 |
| 归属校验必须在 service 层，`4031` 由 service 返回 | 成员 3 任务书 §2 |
| 分支 `feature/backend-persona-model`，走 PR，**禁止直推 `main` / `develop`**；commit 格式 `<type>(scope): <subject>`，scope 用 `model` / `persona` | **红线 2** / 协作规范 §6 |
| **不提交任何密钥**：本功能不新增任何 Key/Token/密码；禁止把 DSN/密码写进代码或 `_test.go` | **红线 1** |
| 改表结构只改 struct + `AutoMigrate`，**禁止手写 `ALTER TABLE` / `DropTable`** | **红线 8** |
| 表名 `personas`、列名小写下划线；Go 缩写词全大写（`UserID` 不是 `userId`） | 协作规范 §5 |

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
| `pkg/response`（`Success[T]` / `Fail` / `PageResult[T]`?） | 成员 1 | **仓库中尚不存在** | handler 无法编译，直到它落地 |
| `pkg/errcode`（4001/4031/4043/5003 常量） | 成员 1 | **仓库中尚不存在** | service 无法编译 |
| `internal/middleware`（`JWTAuth` + `ContextKeyUserID` + `BizErrorHandler`） | 成员 1 | **仓库中尚不存在** | 越权防线取不到 `userID` |
| `internal/config` + `cmd/server/main.go` + `db` 连接 | 成员 1 | **仓库中尚不存在** | 无法起服务验证 |
| `router.go` 的挂载点 | 成员 1 | **仓库中尚不存在** | 端点不可达 |

> **当前的仓库状态**（2026-09-13）：`backend/` 下只有 `go.mod` / `go.sum` / `internal/model/user.go`。**成员 1 的公共层还没交付**。因此实现顺序必须先把**不依赖公共层的文件**（model / dto / repository 的一半）做完，不能等。
> **可以立即开工**：`internal/model/persona.go`、`internal/model/migrate.go`、`internal/dto/persona_dto.go`、`internal/repository/persona_repo.go`。
> **需要公共层才能编译**：service（`errcode`）、handler（`response` + `middleware`）。
> **验证策略**：公共层没到位前，repo 层可以先用一个临时的 `_test.go`（本地 PG）验证 SQL 正确性；**不要**为了"跑起来"去自己造一份 `pkg/errcode`——那会与成员 1 的版本冲突（红线：不 Own 的东西不要动）。

### 7.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `internal/model/persona.go` | 成员 1 | 他写 `message_repo` 要引 `Persona`；`chat_messages.persona_id` 的归属校验挂在它上面 |
| `GET /personas`（同时就是对话列表） | 成员 2 | 聊天页侧栏、路由 `/chat/:personaId` 的跳转目标 |
| `Persona` 响应结构 | 成员 2 | 写 `types/persona.ts` 与 Mock；**字段名逐字对齐 camelCase** |
| `4031` / `4043` 语义 | 成员 1 / 成员 3 | 记忆、画像、日程的越权判定沿用同一套 |

### 7.3 待确认（实现前在群里问清，不要自己拍板）

| # | 问题 | 建议 |
|---|---|---|
| 1 | `PageResult[T]` 放哪？ | 建议放成员 1 的 `pkg/response`（personas / messages / memory / moments / schedules **五个**分页端点共用）。**先问再写**，别在 dto 里造第二份 |
| 2 | `ContextKeyUserID` 的常量名与**值类型**（`uint64` 还是 `uint`）？ | 必须问清。用 `c.GetUint64(key)` 而实际存的是 `uint`，断言会失败 → 取到 0 → 空列表 |
| 3 | 新增 `gorm.io/datatypes` 依赖是否可接受？ | 建议接受（GORM 官方 JSONB 类型）；需广播，`go.mod` + `go.sum` 一并提交 |
| 4 | `POST /personas` 是否顺带播种 `proactive_settings` 一行？ | 跨模块决策（主动消息是成员 3 的另一块）。若不播种，`GET /proactive/settings` 需对"无行"返回默认值（30/120/3）而非 `4043`。**先定，否则主动消息上线时会撞车** |
| 5 | `4031` / `4043` 判定次序是否接受 §5.2 的泄漏权衡？ | 若选保守方案（一律 4031）**必须改契约并广播** |
| 6 | 时间字段精度：Go 默认 `RFC3339Nano`（带小数秒） | 与现有 `user.go` 一致即可；前端 `new Date()` 能正常解析。若要严格到秒，需自定义时间类型——**本期不必要** |

## 8. 验收标准

- [ ] `cd backend && go build ./... && go vet ./... && go test ./...` 全通过
- [ ] `AutoMigrate` 建出 `personas` 表；`psql` 里 `\d personas` 能看到 **`ON DELETE CASCADE` 外键**与组合索引
- [ ] 4 个端点 curl 全通：列表 / 创建 / 编辑 / 删除
- [ ] **跨账号**：用 B 的 Token 打 A 的 `personaId` → `4031`（`PUT` 与 `DELETE` 各验一次）
- [ ] 不存在的 id → `4043`
- [ ] 不带 Token / Token 无效 → `4010` / `4011`（由中间件产生）
- [ ] 校验失败（缺 `name`、超长）→ `4001`
- [ ] **级联删除**：删人设后 `SELECT count(*) FROM chat_messages WHERE persona_id = <已删id>` = 0
- [ ] **排序**：新建一个从未聊过的人设，它排在列表**最后**（不是最前）
- [ ] **空列表**：无人设时 `data.list` 是 `[]` 而不是 `null`
- [ ] **`state` 保留**：把某行 `state` 手工改成 `{"familiarity": 42, "self_note": "x"}`，`PUT` 之后两个键都还在，`familiarity` 仍为 42
- [ ] **`familiarity` 正确**：`GET` 返回顶层 `familiarity`，且 `state` 原样透传
- [ ] code 评审 grep 通过：无硬编码错误码数字/文案、无 `response.Fail` in handler、无 secrets、无 `Save(` on persona
- [ ] PR 已开、至少 1 人 Approve；commit 符合 `<type>(<scope>): <subject>`

## 9. 变更记录

| 日期 | 变更 | 原因 |
|---|---|---|
| 2026-09-13 | 创建 | 人设 CRUD 开工前的设计与验收基线；含越权防线与契约偏差记录 |

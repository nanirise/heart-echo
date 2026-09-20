# spec · 聊天记录（Chat Message）

> 本文件是**合同**：定义本功能做什么、做到什么算完成。合并后即冻结，任何字段或行为变更必须升版本并在群里广播。
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：`chat_messages` DDL、契约字段、错误码表、通用约定一律**不复制**，只给链接；
> 本文件只写**本功能独有**的内容——独有陷阱、独有决策、独有验收项。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-chat-message-model` |
| 状态 | 模型层已落地；仓储 / DTO / service / handler 待实现（见 §7.1） |
| **本功能范围** | `chat_messages` 实体 + `Create`（**三条链路共用**）+ `GET /chat/personas/:personaId/messages` 分页读端点 |
| 负责人 | 成员 3（数据 + 人设 + 部署）；`message_repo` / `chat_service` / `chat_handler` 的文件归属需与成员 1 对齐（§7.3） |
| 关联契约 | [API_CONTRACT §5 / §6](../../API_CONTRACT.md)（历史消息 + SSE） |
| 关联设计 | [TECH_DESIGN §5.1 / §5.4 / §6.1 / §6.2 / §10.3](../../TECH_DESIGN.md)、[总纲 §0](../../dev/MASTER.md)、[成员 1 任务书 §3](../../dev/MEMBER_1_BACKEND_AI.md) |
| 关联红线 | [AGENTS §4.3](../../../AGENTS.md)（数据边界）、§4.4（情绪内部信号）、§5.1 |

---

## 0. 范围声明（先读这一节）

⚠️ **本功能名里的「CRUD」只有 R 是端点。** 消息的 **C 只能由服务端产生**（SSE 对话链路 / 主动消息 / 日程提醒），**U 与 D 在契约与 DDL 两层都不存在**（总纲 §0：对话历史不可单独清空；P2 明确不做消息编辑撤回）。**不该写的代码比该写的更重要**——见 §2。

`chat_messages` 是上述三条链路的**共同落点**。之所以要先做它：Week 2 的生死线是流式对话，SSE 那一步在链路中间就要「落库 user message」，需要本功能先把**表结构与写入方法**定下来；否则 SSE 会顺手自己写一份 INSERT，两套写法必然对不上（尤其 `created_at` 与 `last_message_at` 的同步）。

### 0.1 做什么

`internal/model/chat_message.go`、`internal/model/migrate.go` 追加一行、`internal/dto/chat_dto.go`、`internal/repository/message_repo.go`（`ListByPersona` + `Create`）、`internal/repository/persona_repo.go` 增 `ExistsOwnedByUser`、`internal/service/chat_service.go`、`internal/handler/chat_handler.go`（含 `RegisterChatRoutes`）。

### 0.2 不做什么

| 不做的事 | 原因 / 去向 |
|---|---|
| **`POST /chat/stream`（SSE 全链路）** | 成员 1 的 Week 2 生死线。本功能只保证它需要的表、写入方法与不变量就位（§4），不实现 AI 客户端、不拼 prompt、不下发事件 |
| **情绪分析**（`emotion_label` / `emotion_score` 的**写入**） | 由 AI 服务分析后随消息写入，属 SSE 那一步。本功能只负责**列与字段的类型正确 + 原样回读** |
| **消息编辑 / 撤回 / 单独清空历史** | 总纲 §0：不可单独清空；P2 不做编辑撤回。**不提供端点，连预留路径都不留**（见 §2） |
| 跨人设的消息列表 / 全站消息 / 消息搜索 | 契约不存在这类端点；账号之间完全隔离。**不要顺手加 `GET /chat/messages`** |
| 前端聊天页、`api/chat.ts`、滚动加载 | 成员 2。但对齐清楚「倒序返回 + 前端自己反转」（§3.2 要求 2） |
| `personas.last_message_at` 的**写入** | 写入发生在 SSE / 主动消息链路里（消息落库的同一个事务）。本功能只在 §4 定义这条**不变量** |
| `familiarity` 的累加 | TECH_DESIGN §5.6：每轮对话结束 +1。属对话链路，本功能不碰 |
| 消息内容的长度上限 | DDL 是 `TEXT`（无上限）。上限由**写入方**决定，**repo 不做长度校验** |
| 新增错误码 | 一个都不需要，复用 `4043` / `5003` / `4010` |
| 手写 `ALTER TABLE` / `DropTable` | 红线 8：改结构 = 改 struct + `AutoMigrate` |

---

## 1. 本表独有的硬性要求

### 1.1 表里**没有**的东西也是硬约束

| 没有的东西 | 意味着 |
|---|---|
| `updated_at` | **没有"编辑"这个动作**：表结构上就不表达"某条消息被改过"。不要为了"以后可能要用"补一列 |
| `deleted_at` | **不做软删**（外键统一 CASCADE 是硬删）。也不要自己加 |
| `session_id` | 一个人设一个对话；`persona_id` 就是对话标识 |
| `emotion_*` 之外的任何情绪列 | 情绪只有两个内部字段，**不新增**（AGENTS §4.4：不加展示标签） |

**结论**：CRUD 里的 U、D **不是"这次先不做"，而是"结构上不存在"**。要加它们必须先改 DDL + 改契约 + 群里广播——本功能不预留任何钩子。

### 1.2 两个外键与 `bigint`

`user_id` 与 `persona_id` 的外键**都必须由 GORM 建出来**（显式声明 belongs-to 关联字段，`Create` 时保持零值），否则"删人设级联删消息"与"删账号清理数据"两条验收项直接不成立。`chat_message.go` 上有注释说明。

> ⚠️ **`ChatMessage.ID` 的 tag 必须是 `type:bigint` + `autoIncrement`，绝不能用 `type:bigserial`**——这是全项目已踩三次的关联复制坑（详见 [persona spec §1.3](../persona-model/spec.md)）。
> 本表是**第二处引用倒挂**：`user_memory.source_message_id` 引用本表主键，若写成 `bigserial`，那一列会被建出 `DEFAULT nextval(...)`，让漏传 `source_message_id` 的 INSERT **静默拿到一个不存在的消息 id**。
> **凡是被别的表引用的主键，`ID` tag 都写 `type:bigint`。**

**索引 tag 不要写 `sort:desc`**：DDL 的索引不带 `DESC`，Postgres 的 btree 可以反向扫描，`ORDER BY created_at DESC` 照样用得上。人设表的 `NULLS LAST` 是**语义要求**，消息表没有这个问题——`created_at` 非空，按它排就是自然语义。

### 1.3 字段语义（三条容易想当然的）

1. **`role` 用自定义类型 + 常量**，不要裸 `string`：写入方只能从 `RoleUser` / `RoleAssistant` 取，拼错在编译期暴露；DDL 的 CHECK 是第二道闸门。**CHECK 是最容易在"翻译 Struct"时整行丢掉的东西**——它不影响编译、不影响跑通，只在数据脏了以后才暴露。
2. **`emotion_label` / `emotion_score` 必须是指针**（可空列）：值类型会把 `null` 序列化成 `""` 和 `0`——前端拿到 `"emotionLabel": ""` 会当成"有情绪但标签是空串"，与"没有情绪"是两回事。它们**返回但不展示**（AGENTS §4.4）。
3. **`is_nudge` 不是"这条消息是不是 AI 发的"**：`role` 仍然是 `"user"`，它标记的是"这条 user 消息是系统注入的 `[nudge]`，不是用户本人打的字"。
4. **`content` 存的是原文**：`[nudge]` 标记随内容一起存（格式由写入方定），本功能**不解析、不裁剪、不转义**。

---

## 2. 不该写的代码：C / U / D 去了哪

| 动作 | 是否提供端点 | 说明 |
|---|---|---|
| **R** 读历史 | ✅ `GET /chat/personas/:personaId/messages` | 本功能唯一对外的端点 |
| **C** 创建 | ❌ **没有 `POST /chat/messages`** | 消息**只能由服务端产生**。**绝不允许前端直接 POST 一条消息**——那等于让客户端伪造对话历史（也能伪造 `role='assistant'`，让"AI 说过的话"变成用户可写的字段） |
| **U** 编辑 / 撤回 | ❌ **不存在** | 总纲 §0.3：P2 明确不做。DDL 也没有 `updated_at`（§1.1） |
| **D** 删除 | ❌ **不存在** | 总纲 §0：不可单独清空。唯一删除路径是 `DELETE /personas/:id` → 外键级联 |

**两条"不要写"的硬要求：**

1. **不写任何 `DELETE FROM chat_messages`**：手写删除等于绕开 CASCADE，还会在校验越权的地方多开一个口子。
2. **不留预留路径**：不写 `PUT /chat/messages/:id`、不写 `POST /chat/messages`，**连注释掉的空 handler 都不要留**——预留路径会被后来的人当成"已经规划好的功能"实现出来，而它与总纲 §0 冲突。

---

## 3. 接口定义

### 3.1 「先判归属、再查消息」——本功能与 persona CRUD 最大的不同

人设的 `PUT`/`DELETE /personas/:id` 可以**一条** `WHERE id = ? AND user_id = ?` 同时完成"归属 + 命中"，因为**人设本身就是被操作的那一行**。消息不是：

```
GET /chat/personas/999/messages     ← 被校验的资源是「人设」，返回的资源是「消息」，是两张表
```

于是 `SELECT ... FROM chat_messages WHERE persona_id = 999 AND user_id = <我>` 返回空集时，**有两种完全不同的可能**：

| 可能 | 应返回 | 如果偷懒只看消息表 |
|---|---|---|
| 人设 999 是别人的 / 不存在 | **`4043`** | 返回 `200 + []`——**把越权伪装成了"这个对话还是空的"** |
| 人设 999 是我的，但我还没聊过 | **`200 + list: []`** | 正确 |

**正确顺序（三步，一次不多）**：① 先 `personaRepo.ExistsOwnedByUser(ctx, userID, personaID)` 判人设归属 → 未命中一律 `4043`；② 再查消息，空集是合法结果 → `200 + []`。

`ExistsOwnedByUser` 放 `internal/repository/persona_repo.go`（属人设模块，本分支一并落地）——**归属语义只能有一个实现**，chat 侧不要自己再写一份 `SELECT ... FROM personas`。

> **这里的"归属探针"为什么不算被禁的那一个**（persona spec 要求仓储层不提供任何存在性判断方法）：
> 判据是**"结果是否造成可观测的差异"**，不是"有没有这个方法"。人设 CRUD 里被禁的探针，调用方拿到不同结果后会返回**不同的错误码** → 外部可区分 → 可逐个 id 二分探测；而 `ExistsOwnedByUser` **只回 bool**，调用方拿到 `false` 一律 `4043` → 所有非本人路径都收敛到同一个响应，**没有泄漏面**。且它是**必须有**的：人设归属在 `personas` 表、消息在 `chat_messages` 表，空集无法自证归属。
> 同理，`ListByPersona` 里那个看似冗余的 `user_id` 条件**必须留着**：它不是重复校验，是**第二道闸门**。将来若有人给 service 加了别的调用分支、或直接调 repo，第一道闸门可能没经过——`WHERE` 里的 `user_id` 是最后一道，且它是结构性的（忘了传参就编译不过）。

### 3.2 `GET /chat/personas/:personaId/messages`

**六条容易写错的要求：**

1. **必须先判人设归属、再查消息**（§3.1）。**一句话**：`WHERE persona_id = ? AND user_id = ?` 查出来是空的，**无法区分**「这个人设不是你的」和「这个人设是你的但还没聊过」，后者必须是 `200 + []`，前者必须是 `4043`。
2. **排序写 `ORDER BY created_at DESC, id DESC`**：倒序是 TECH_DESIGN §10.3 明确的（前端翻到最新一页、再往上翻旧页）。**`id DESC` 兜底不能省**：`NOW()` 是事务时间，同一事务里落的多条消息时间戳完全相同；只按 `created_at` 排序时同值行顺序不确定，**翻页会出现「第 2 页冒出第 1 页的消息」或漏项**。
3. **`total` 必须用同一个 `persona_id` + `user_id` 条件统计**。写成 `COUNT(*)` 全表**不只是数字错**——它会把"系统里一共有多少条消息"泄漏给任意登录用户，这是越权防线的一部分。
4. **空列表必须返回 `[]` 而不是 `null`**（Go 的 nil slice 序列化成 `null`，前端 `.map` 直接崩）。**`page` 超出范围时返回空列表 + 真实 `total`，仍是 `200`。**
5. **分页参数一律钳制，不返回 `4001`**：契约给本端点只列了 `4043`，所以 `page=abc`、`pageSize=100000` 这类输入**不能变成 `4001`**（凭空多一个错误码就是契约漂移）。规则：`page < 1 → 1`；`pageSize < 1 → 20`；`pageSize > 100 → 100`；**上限必须有**，否则一次拉全表。响应里的 `page` / `pageSize` 回填**钳制后**的值。
6. **`:personaId` 解析失败（非数字、负数、溢出）统一按 `4043`**，不要返回 `4001`。做法上**不写特判**：解析失败就把 `personaID` 置 `0` 传给 service，而 `WHERE id = 0 AND user_id = ?` 天然不命中 → 同一条 `4043` 出口。**少一个分支 = 少一处不一致的出口。**

> **给成员 2 的对齐项**：本端点**倒序返回**（最新在前）。前端渲染时要自己反转成"旧→新"再显示；往上翻页拿到的也是"更新的在前"，插入时注意别把顺序弄反。契约里的 `PageResult<ChatMessage>` 是排序无关的，**排序语义以本 spec 与技术文档 §10.3 为准**。

### 3.3 错误码

`:personaId` 未命中（不存在 / 不属于你 / 解析失败 / 为 0）→ **一律 `errcode.ErrPersonaNotFound`（4043）**，**message 逐字相同**。本模块没有功能越权场景，因此**不产生 `4030`**，也不产生 `4040`（"不存在"与"不属于你"必须同码，理由见 [persona spec §2.1](../persona-model/spec.md)）。

---

## 4. 写入能力（仓储层契约，供三条链路共用）

本功能**不实现写入端点**，但必须把写入方法定下来，否则 SSE / 主动消息 / 日程提醒会各写一份 INSERT。

```go
// 三条链路共用。事务句柄由调用方传（红线 7：事务边界在 service）；m.ID / m.CreatedAt 由 GORM 回填。
Create(ctx context.Context, tx *gorm.DB, m *model.ChatMessage) error
```

| 写入方 | 何时 | `role` / `is_nudge` |
|---|---|---|
| SSE 对话链路（成员 1，Week 2） | 收到用户消息 → 落库 user 消息 → 流式生成 → 完整回复落库 assistant 消息 | `user` / `false`；回复为 `assistant`。**落库完成后才发 `done`** |
| 主动消息（成员 3） | 到点注入 `[nudge]` | `user` / **`true`** |
| 日程提醒（成员 3，P1） | 到点触发 | `user` / **`true`** |

**`Create` 的三个约束**：`tx` **必传**，不写 `if tx == nil { tx = r.db }` 这类兜底——它会让"忘了传事务"静默变成"不在事务里"，而调用方恰恰依赖同事务；只赋 `UserID` / `PersonaID` 两个标量，**不给关联字段赋值**；`user_id` 由调用方从 Token / 人设行取，**repo 不做任何"猜"**。

**写入方的三条不变量**（写进注释，复用时照做）：

1. **`user_id` 与 `persona_id` 必须同时带上**，且 `user_id` 只能来自 Token / 人设行，**绝不从请求体取**（AGENTS §4.3：只带 `persona_id` 会跨用户串号，它是全局自增）。
2. **同一事务内更新 `personas.last_message_at = 该消息的 created_at`**（它是对话列表排序键 + 主动消息空闲判定）。写法必须是**指定单列更新**：`tx.Model(&model.Persona{}).Where("id = ? AND user_id = ?", personaID, userID).Update("last_message_at", m.CreatedAt)`。
   ⚠️ **绝不能用 `Save(&persona)`**：那是全列写回，会把 `state`（含 `familiarity`）覆盖成零值——**不可逆的数据损坏**，persona spec §1.4 已把这个坑列为反例，这里是它的第二次出现点。
3. **`familiarity` 的累加不在本功能**，但它是**同一事务里的同一个 `state`**：写入方改 `state` 时不要重新序列化整个 JSON，只更新 `familiarity` 键（否则会把 `self_note` 等未知键丢掉）。

---

## 5. 验收标准

**A · 模型层**

- [ ] `cd backend && go build ./... && go vet ./... && go test ./...` 全通过
- [ ] `AutoMigrate` 追加 `&ChatMessage{}` 后，`\d chat_messages` 能看到：**两个** `ON DELETE CASCADE` 外键（`user_id` / `persona_id`）、`role` 的 **CHECK 约束**、两个索引，且**没有** `updated_at` / `deleted_at` 列
- [ ] **序列不污染**：`\ds` 里只有 `chat_messages_id_seq`（冒出 `_persona_id_seq` 之类 = 关联复制坑）

**B · 端点与越权**

- [ ] 正常读取：造 3 条消息 → `200`、`total=3`、`list` 按 `createdAt` **倒序**
- [ ] **空对话**：属于自己但没消息的人设 → **`200` + `list: []`**（不是 `null`、不是 `4043`）
- [ ] **资源越权**：用 B 的 Token 打 A 的 `personaId` → **`4043`**
- [ ] **不存在**：不存在的 id → **`4043`**，且与上一条**同码同文案**（外部无法区分——这正是隐藏存在性的效果）
- [ ] **不产生 `4030` / `4040` / `4001`**：任意输入组合下（越权 / 不存在 / `page=abc` / `pageSize=100000` / `personaId=abc` / 无 Token）只出现 `4043` / `4010` / `200`
- [ ] **分页钳制**：`pageSize=100000` → 至多 100 条且响应 `pageSize` 为 `100`；`page=0` → 按第 1 页处理
- [ ] **分页稳定**：把两条消息的 `created_at` 手工改成完全相同 → `pageSize=1` 逐页拉，**不重不漏**
- [ ] **越界页**：`page=999` → `200` + `list: []` + 真实 `total`
- [ ] 不带 Token / Token 无效 → `4010` / `4011`（由中间件产生）
- [ ] **`total` 不透全局**：等于该人设的消息数，不是全表行数（造两个账号各若干消息，互相对照）
- [ ] **级联删除**：删人设后 `SELECT count(*) FROM chat_messages WHERE persona_id = <已删id>` = 0

**C · 字段与不变量**

- [ ] **情绪字段回读**：无情绪的行返回 `"emotionLabel": null` / `"emotionScore": null`（不是 `""` / `0`）；有情绪的返回 `0.000~1.000` 的小数
- [ ] **契约字段逐字对齐**：响应对象 **8 个字段**（`id` `personaId` `role` `content` `emotionLabel` `emotionScore` `isNudge` `createdAt`），**没有 `userId`**
- [ ] **`last_message_at` 不变量**：走一次 `Create` 后 `personas.last_message_at` 等于该消息的 `created_at`，且 `state` / `familiarity` **未被改动**

**D · 代码层**

- [ ] grep 通过：无硬编码错误码数字/文案、无 `response.Fail` in handler、无 `Save(`、无 `DELETE FROM chat_messages`、无 secrets
- [ ] `message_repo.go` 里**没有** `FindByID` / `ExistsByID` / `Delete*` / `Update*`（§2）
- [ ] PR 已开、至少 1 人 Approve；commit 符合 `<type>(<scope>): <subject>`

---

## 6. 依赖与交接

### 6.1 依赖

| 依赖 | 提供方 | 现状 | 影响 |
|---|---|---|---|
| `pkg/errcode`（4043 / 5003 / 4010）、`pkg/response` | 成员 1 | ✅ 已合入 | 无 |
| `internal/model/persona.go`、`jsonb.go` | 成员 3 | ✅ 已落地 | 无 |
| `internal/dto/common_dto.go`（`PageResult[T]` + 分页常量 + `ClampPage`） | 三个分支之一先落地 | ⚠️ **尚未落地** | **本功能第一个提交就是它**，**不要写第二份** |
| `internal/middleware`（`JWTAuth` + `BizErrorHandler`） | 成员 1 | ✅ 已落地 | 无 |
| `router.go` 挂载点 | 成员 1 | ✅ 已落地 | 端点可挂 |

**可以立即开工**：`chat_message.go`、`migrate.go` 追加、`common_dto.go`、`chat_dto.go`、`message_repo.go`、`persona_repo.go` 的归属方法——都不依赖成员 1 的公共层。

### 6.2 交接

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `internal/model/chat_message.go` | **成员 1** | SSE 落库要引 `ChatMessage`；`user_memory.source_message_id` 的外键指向它 |
| `message_repo.Create(ctx, tx, m)` | **成员 1 + 成员 3** | 三条写入链路共用一条 INSERT，签名固定后**不要各写一份** |
| `persona_repo.ExistsOwnedByUser(ctx, userID, personaID)` | **成员 1** | SSE 链路的 service 也要判归属。后续它会需要 `GetOwned`（拿人格字段拼 prompt），建议**一次加齐** |
| `GET /chat/personas/:personaId/messages` | **成员 2** | 聊天页历史加载 + 往上翻页 |
| **倒序语义**（最新在前） | **成员 2** | 前端渲染前要自己反转；契约里没写排序，**这条只能从这里拿到** |
| `isNudge=true` 的消息 | **成员 2** | 它是 `role='user'` 但**不是用户打的字**；渲染方式需与成员 2 单独对齐，后端只保证字段准确、`content` 原样返回 |
| `last_message_at` 的更新不变量 | **成员 1（SSE）、成员 3（主动消息）** | §4 不变量 2；写错会让对话列表排序与主动消息空闲判定一起失效 |

### 6.3 待确认

| # | 问题 | 建议 |
|---|---|---|
| 1 | **文件归属**：`message_repo.go` / `chat_service.go` / `chat_handler.go` 按任务书属成员 1 | 本分支先落 **model + repo + 读端点**（都不依赖公共层），但对齐后再动 `chat_service.go` / `chat_handler.go`，**避免两人同时写同一文件** |
| 2 | **分页钳制常量放哪** | 建议把 `PageSizeDefault` / `PageSizeMax` / `ClampPage()` 一并放进 `common_dto.go`——它已是"分页约定的唯一实现"，**不要两处各写一份默认值** |
| 3 | **`emotion_score` 的 Go 类型**：`NUMERIC(4,3)` ↔ `*float64` 在写入时（pgx 编码）是否顺畅 | 先用 `*float64`；若实测报类型不匹配，退到自写 `driver.Valuer`（返回字符串，Postgres 对 numeric 接受字符串字面量）。**不要引入 `decimal` 包**。这个决定要在 SSE 那一步写入前定下来 |
| 4 | `[nudge]` 的内容格式（前缀怎么写、要不要换行） | 由主动消息模块与 SSE 约定一次，**本功能只负责原样存取**；一旦定了就是两个模块的公共约定 |

---

## 7. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-14 | v1 | 创建 | 聊天记录（模型 + 仓储 + 读端点）开工前的设计与验收基线；含「CRUD 只有 R 是端点」的边界、倒序分页语义、越权防线的两张表判定 |
| 2026-09-20 | v2 | **按 AGENTS §5.2 引用优先精简**：删除从 TECH_DESIGN / API_CONTRACT 复制粘贴的 DDL 原文、字段对照表、通用约定表与红线自查表，改为链接；只保留本功能独有的陷阱、决策与验收项（443 → 243 行，行数按 `wc -l` 含空行） | 文档膨胀 |

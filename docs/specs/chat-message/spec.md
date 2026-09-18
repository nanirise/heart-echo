# spec · 聊天记录 CRUD（Chat Message）

> 功能名：chat-message ｜ 分支：`feature/backend-chat-message-model`
> 负责人：成员 3（数据 + 人设 + 部署） ｜ 状态：设计完成，待实现
> 创建：2026-09-14 ｜ 最后更新：2026-09-14（第 1 版）
> 关联：[AGENTS.md](../../../AGENTS.md) ｜ [接口契约 §5/§6](../../API_CONTRACT.md) ｜ [技术文档 §6.2 / §5.1 / §10.3](../TECH_DESIGN.md) ｜ [总纲 §0](../dev/MASTER.md) ｜ [成员 1 任务书 §3](../dev/MEMBER_1_BACKEND_AI.md)

> ⚠️ **本功能名里的「CRUD」只有 R 是端点。** 消息的 **C 只能由服务端产生**（SSE 对话链路 / 主动消息 / 日程提醒），**U 与 D 在契约与 DDL 两层都不存在**（总纲 §0：对话历史不可单独清空；P2 明确不做消息编辑撤回）。见 §4.3——这是本功能最需要写清楚的一段，因为它决定了「不该写什么代码」。
> ⚠️ **分工待对齐**：按 [总纲 §5](../dev/MASTER.md) 的列，`message_repo.go` / `chat_service.go` / `chat_handler.go` 属**成员 1**、`internal/model` 属成员 3。本分支按分支名先把**模型 + 仓储 + 读端点**落地，**动手前需在群里与成员 1 对齐**，避免两人同时写同一批文件（见 §7.3 待确认 #1）。

---

## 1. 背景与目标

一个人设 = 一个对话，消息直接挂 `persona_id`（技术文档 §6.1），**没有会话表**。聊天页要能打开历史、往上翻页；主动消息与日程提醒产生的 `[nudge]` 消息也要落在同一张表里。因此 `chat_messages` 是对话链路、主动消息、日程提醒三条链路的**共同落点**。

**目标（一句话）**：交付 `chat_messages` 的 GORM 实体、仓储（读 + 写）、`GET /chat/personas/:personaId/messages` 分页读端点，字段与契约 §5 逐字一致；并且**任何接口都不可能读到别人的对话，也不可能探测出某个人设属于谁**。

**为什么现在做**：Week 2 的生死线是流式对话。SSE 那一步（成员 1）在链路中间就要 `落库 user message`（技术文档 §5.1 时序图），它需要本功能先把**表结构与写入方法**确定下来；否则 SSE 会顺手自己写一份 INSERT，两套写法必然对不上（尤其 `created_at` 与 `last_message_at` 的同步）。

## 2. 范围

### 2.1 做什么（In Scope）

| 项 | 产物 |
|---|---|
| 数据模型 | `internal/model/chat_message.go`（GORM 实体，含两个外键级联声明、`role` 的 CHECK） |
| 建表 | `internal/model/migrate.go` 的 `AutoMigrate` 追加 `&ChatMessage{}`（顺序在 `&Persona{}` 之后） |
| 通用分页结构 | `internal/dto/common_dto.go`（`PageResult[T]` + 分页常量 + 钳制函数，**全项目五个分页端点共用**；若 persona 分支已落地则直接消费，**不要写第二份**） |
| 请求 / 响应结构 | `internal/dto/chat_dto.go` |
| 数据访问（读） | `internal/repository/message_repo.go`：`ListByPersona`（含 `total`） |
| 数据访问（写） | 同上：`Create`（**供 SSE 链路 / 主动消息 / 日程提醒三处共用**，见 §4.4） |
| 归属判定 | `internal/repository/persona_repo.go` 增 `ExistsOwnedByUser`（见 §5.2，它读的是 `personas` 表） |
| 业务逻辑 | `internal/service/chat_service.go`（归属校验 → `4043`；分页钳制） |
| HTTP 接口 | `internal/handler/chat_handler.go`（`GET /chat/personas/:personaId/messages` + `RegisterChatRoutes`） |
| 路由挂载 | `router.go` 加一行 `RegisterChatRoutes(protected, chatHandler)`（**不要与成员 1 同时改**） |

### 2.2 不做什么（Out of Scope）

| 不做的事 | 归属 / 原因 |
|---|---|
| **`POST /chat/stream`（SSE 全链路）** | 成员 1 的 Week 2 生死线任务。本功能**只保证它需要的表、写入方法与不变量就位**（§4.4），不实现 AI 客户端、不拼 prompt、不下发事件 |
| **情绪分析**（`emotion_label` / `emotion_score` 的**写入**） | 由 AI 服务分析后随消息写入，属 SSE 那一步。本功能只负责**列与字段的类型正确 + 原样回读** |
| **消息编辑 / 撤回** | 总纲 §0.3：**P2 不做**。**不提供端点，连预留路径都不留**（同 `/emotion/diary` 的处理方式） |
| **单独清空对话历史** | 总纲 §0：「对话历史按人设隔离，**不可单独清空**，只能随人设一起删」。**不写任何 `DELETE FROM chat_messages` 的代码** |
| **跨人设的消息列表 / 全站消息 / 消息搜索** | 契约不存在这类端点；账号之间完全隔离（AGENTS §4.3）。**不要顺手加 `GET /chat/messages`** |
| 前端聊天页、`api/chat.ts`、滚动加载 | 成员 2。本功能**只做后端**，但要把「倒序返回 + 前端自己反转」这条对齐清楚（§4.2 要求 6） |
| `personas.last_message_at` 的**写入** | 写入发生在 SSE / 主动消息链路里（消息落库的同一个事务）。本功能**只在 §4.4 定义这条不变量**；人设表本身的读写属人设功能 |
| `familiarity` 的累加 | 技术文档 §5.6：每轮对话结束 +1。属对话链路，本功能**不碰** |
| 消息内容的长度上限 | DDL 是 `TEXT`（无上限）。上限由**写入方**（SSE 请求体的 `4001` 校验）决定，**本功能的 repo 不做长度校验** |
| 新增错误码 | 本功能**一个都不需要新增**，复用 `4043` / `5003` / `4010` |
| 手写 `ALTER TABLE` / `DropTable` | 红线 8：改结构 = 改 struct + AutoMigrate |

## 3. 数据模型（`chat_messages`）

### 3.1 权威 DDL（技术文档 §6.2，照抄待翻译）

```sql
-- 消息表（直接挂人设，无中间会话表）
CREATE TABLE chat_messages (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id     BIGINT      NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    role           VARCHAR(10) NOT NULL CHECK (role IN ('user', 'assistant')),
    content        TEXT        NOT NULL,
    emotion_label  VARCHAR(20),                    -- 8 类情绪之一
    emotion_score  NUMERIC(4,3),                   -- 0.000 ~ 1.000
    is_nudge       BOOLEAN     NOT NULL DEFAULT FALSE,  -- 是否为主动消息注入的 [nudge]
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_messages_persona_time ON chat_messages(persona_id, created_at);
CREATE INDEX idx_messages_user_id ON chat_messages(user_id);
```

### 3.2 字段对照表（DDL ↔ GORM ↔ JSON）

| DDL 列 | Go 类型 | GORM tag 要点 | JSON 字段 | 契约来源 |
|---|---|---|---|---|
| `id BIGSERIAL PK` | `uint64` | `type:bigint;primaryKey;autoIncrement`（**不是 `bigserial`**，见 §3.4） | `id` | §5 实体 |
| `user_id BIGINT NOT NULL` | `uint64` | `type:bigint;not null;index:idx_messages_user_id` | **`"-"`** | ⚠️ 契约实体里**没有** `userId` |
| `persona_id BIGINT NOT NULL` | `uint64` | `type:bigint;not null;index:idx_messages_persona_time,priority:1` | `personaId` | §5 |
| `role VARCHAR(10) NOT NULL CHECK(IN('user','assistant'))` | `MessageRole`（string 底层） | `type:varchar(10);not null;check:role IN ('user','assistant')` | `role` | §5 |
| `content TEXT NOT NULL` | `string` | `type:text;not null` | `content` | §5 |
| `emotion_label VARCHAR(20)`（可空） | `*string` | `type:varchar(20)` | `emotionLabel` | §5（可为 `null`） |
| `emotion_score NUMERIC(4,3)`（可空） | `*float64` | `type:numeric(4,3)` | `emotionScore` | §5（可为 `null`） |
| `is_nudge BOOLEAN NOT NULL DEFAULT FALSE` | `bool` | `type:boolean;not null;default:false` | `isNudge` | §5 |
| `created_at TIMESTAMPTZ NOT NULL` | `time.Time` | `type:timestamptz;not null;default:now();autoCreateTime` + 组合索引 `priority:2` | `createdAt` | §5 |

**所有 ID 类型统一 `uint64`**（`id` / `user_id` / `persona_id`），与 `user.go` / `persona.go` 保持一致；仓储与 service 的方法签名同理，不做 `uint` / `int64` 的来回转换。

### 3.3 六条必须记住的结论

1. **`user_id` 不进响应体**：契约 §5 的 ChatMessage 实体只有 **8 个字段**，没有 `userId`。用 `json:"-"` 关掉（与 `Persona.UserID`、`User.PasswordHash` 同一手法）。它是越权防线的载体，不是给前端看的。**不要为了"方便调试"改成 `json:"userId"`**。
2. **`role` 用自定义类型 + 常量**，不要裸 `string`：

   ```go
   type MessageRole string
   const (
       RoleUser      MessageRole = "user"
       RoleAssistant MessageRole = "assistant"
   )
   ```
   写入方只能从这两个常量里取，拼错单词在编译期就暴露；DDL 的 CHECK 是第二道闸门（§3.5）。
3. **`emotion_label` / `emotion_score` 必须是指针**：契约要求这两个字段可以为 `null`（词典档分析不出时就是空）。值类型会把 `null` 序列化成 `""` 和 `0`——前端拿到 `"emotionLabel": ""` 会当成"有情绪但标签是空串"，与"没有情绪"是两回事。**这两个字段是回读字段**：随历史消息返回（供画像聚合），**界面一律不得渲染**（AGENTS §4.4）。
4. **`is_nudge` 不是"这条消息是不是 AI 发的"**：`role` 仍然是 `"user"`，它标记的是"这条 user 消息是系统注入的 `[nudge]`，不是用户本人打的字"（技术文档 §5.4）。
5. **`created_at` 不能只按它排序**，查询必须补 `id` 兜底——`NOW()` 在 Postgres 里是**事务开始时间**，同一事务内插入的多行会拿到**完全相同**的时间戳；排序不稳会让翻页重复或漏项（§4.2 要求 2）。
6. **`content` 存的是原文**：`[nudge]` 标记随内容一起存（写入方决定格式，见 §4.4），本功能**不解析、不裁剪、不转义**——原样入库、原样返回。

### 3.4 外键与索引（两个外键都必须建出来）

**外键必须由 GORM 建出来**，否则"删人设级联删消息"这条验收项不成立。只写标量 `UserID` / `PersonaID`，GORM **不会**创建外键约束——必须额外声明 belongs-to 关联：

```go
// 仅供 GORM 生成外键约束用；json:"-" 不让它们进响应体
User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
```

> **两个外键的分工**：`persona_id` 的 CASCADE 实现「删人设 = 删对话」（总纲 §0，本功能验收项）；`user_id` 的 CASCADE 实现「删账号 = 清空其全部数据」，同时它是**越权防线**在数据层的落点。少任何一个，都有一类孤儿数据清不掉。
>
> **`Create` 时不要给这两个关联字段赋值**（保持零值），否则 GORM 会尝试连带写入 `users` / `personas` 表——`persona.go` 的 `User` 关联同一个坑。

> ⚠️ **`ChatMessage.ID` 的 tag 必须是 `type:bigint` + `autoIncrement`，绝不能用 `type:bigserial`**（`user.go` / `persona.go` 已按此修正，这是同一个坑的第二处引用倒挂）：
> GORM 建关联时会把**被引用主键的 DataType 复制到外键字段**上。`user_memory.source_message_id` 正是引用 `chat_messages(id)` 的外键——若本表 `ID` 写成 `bigserial`，那一列会被建出 `DEFAULT nextval('...')` 的序列默认值（Postgres 里 `bigserial` 的语义就是"建序列 + 设默认值"），既偏离 DDL，又让漏传 `source_message_id` 的 INSERT **静默拿到一个不存在的消息 id**。
> 这条对整个项目生效：**凡是会被别的表引用的主键，`ID` tag 都写 `type:bigint`**。

| 索引 | 用途 | 写法 |
|---|---|---|
| `idx_messages_persona_time (persona_id, created_at)` | 历史消息分页（本功能唯一的读路径） | 两字段共用一个 `index:` 名，`persona_id` 为 `priority:1`、`created_at` 为 `priority:2` |
| `idx_messages_user_id (user_id)` | 删账号级联、越权防线的辅助 | `UserID` 上的单列 `index:` |

> **DDL 的索引不带 `DESC`，tag 也就不要写 `sort:desc`**：Postgres 的 btree 可以反向扫描，`ORDER BY created_at DESC` 照样用得上这个索引。人设表的 `NULLS LAST` 是**语义要求**（没聊过的不能排最前），消息表没有这个问题——`created_at` 非空，按它排就是自然语义。

### 3.5 「表里没有的东西」也是硬约束

| 没有的东西 | 意味着 |
|---|---|
| `updated_at` | **没有"编辑"这个动作**：表结构上就不表达"某条消息被改过"。不要为了"以后可能要用"补一列 |
| `deleted_at` | **不做软删**（外键统一 CASCADE 是硬删）。也不要自己加 |
| `session_id` | 一个人设一个对话（契约 §5 末注）；`persona_id` 就是对话标识 |
| `emotion_*` 之外的任何情绪列 | 情绪只有两个内部字段，**不新增**（AGENTS §4.4：不加展示标签） |

**结论**：CRUD 里的 U、D **不是"这次先不做"，而是"结构上不存在"**。要加它们必须先改 DDL + 改契约 + 群里广播——本功能不预留任何钩子。

## 4. 接口定义

### 4.1 通用约定（契约 §1 / §2）

| 项 | 约定 |
|---|---|
| Base URL | `/api/v1` |
| 鉴权 | 需要 `Authorization: Bearer <accessToken>`；缺失/过期 → `4010`。**不在免鉴权白名单里** |
| 响应体 | `{ code, message, data, timestamp }`，由 `pkg/response` 统一产出 |
| 分页 | `page` 从 1 开始；`pageSize` 默认 20、上限 100 |
| 分页响应 | `{ list, total, page, pageSize }`，类型 `PageResult[T]`，**定义在 `internal/dto/common_dto.go`** |
| 错误出口 | handler 一律 `_ = c.Error(err)` 上抛，由 `BizErrorHandler` 中间件统一出口；**handler 里不出现 `response.Fail`** |

### 4.2 `GET /chat/personas/:personaId/messages` — 历史消息（契约 §5）

| 项 | 内容 |
|---|---|
| 请求 | `page`、`pageSize`（query，均可选）；`:personaId` 路径参数 |
| 响应 data | `PageResult<ChatMessage>` |
| 业务错误码 | **只有 `4043`**（人设不存在 / 不属于当前用户，同一个码） |

**响应示例**（`list` 按 `createdAt` **倒序**，最新在前）：

```json
{
  "code": 200, "message": "success",
  "data": {
    "list": [
      { "id": 348, "personaId": 1, "role": "assistant", "content": "辛苦啦，今天发生什么了吗？",
        "emotionLabel": null, "emotionScore": null, "isNudge": false, "createdAt": "2026-09-10T14:30:05+08:00" },
      { "id": 347, "personaId": 1, "role": "user", "content": "今天上班好累啊",
        "emotionLabel": "sadness", "emotionScore": 0.87, "isNudge": false, "createdAt": "2026-09-10T14:30:00+08:00" }
    ],
    "total": 348, "page": 1, "pageSize": 20
  },
  "timestamp": 1789000000000
}
```

**六条容易写错的要求：**

1. **必须先判人设归属、再查消息**——这是本功能与人设 CRUD 最大的不同，也是最容易写错的一处，单独在 §5.2 展开。**一句话**：`WHERE persona_id = ? AND user_id = ?` 查出来是空的，**无法区分**「这个人设不是你的」和「这个人设是你的但还没聊过」，后者必须是 `200 + []`，前者必须是 `4043`。
2. **排序写 `ORDER BY created_at DESC, id DESC`**：
   - `DESC`：技术文档 §10.3 明确「历史消息按 `created_at` 倒序分页加载」（前端翻到最新一页、再往上翻旧页）。
   - **`id DESC` 兜底不能省**：`NOW()` 是事务时间，同一事务里落的多条消息时间戳完全相同；只按 `created_at` 排序时同值行顺序不确定，**翻页会出现「第 2 页冒出第 1 页的消息」或漏项**。`id` 是 BIGSERIAL，天然单调，能给出全序。
3. **`total` 必须用同一个 `persona_id` + `user_id` 条件统计**。写成 `COUNT(*)` 全表**不只是数字错**——它会把"系统里一共有多少条消息"泄漏给任意登录用户（跨用户的量级信息），这是越权防线的一部分，不是分页细节。
4. **空列表必须返回 `[]` 而不是 `null`**：Go 的 nil slice 会序列化成 `null`，前端 `v-for` / `.map` 直接崩。切片必须初始化（`list := make([]ChatMessageResponse, 0)`）。**`page` 超出范围时返回空列表 + 真实 `total`，仍是 `200`。**
5. **分页参数一律钳制，不返回 `4001`**：契约给本端点只列了 `4043`，所以 `page=abc`、`pageSize=100000` 这类输入**不能变成 `4001`**（凭空多一个错误码就是契约漂移）。规则：`page < 1 → 1`；`pageSize < 1 → 20`；`pageSize > 100 → 100`；**`pageSize` 上限必须有**，否则 `pageSize=100000` 一次拉全表。响应里的 `page` / `pageSize` 回填**钳制后**的值（前端据此算下一页）。
6. **`:personaId` 解析失败（非数字、负数、溢出）统一按 `4043`**，不要返回 `4001`。做法上**不写特判**：解析失败就把 `personaID` 置 `0` 传给 service，而 `WHERE id = 0 AND user_id = ?` 天然不命中 → 同一条 `4043` 出口。**少一个分支 = 少一处不一致的出口。**

> **给成员 2 的对齐项**：本端点**倒序返回**（最新在前）。前端渲染时要自己反转成"旧→新"再显示；往上翻页拿到的也是"更新的在前"，插入时注意别把顺序弄反。契约 §5 的 `PageResult<ChatMessage>` 是排序无关的，**排序语义以本 spec 与技术文档 §10.3 为准**。

### 4.3 CRUD 里的 C / U / D 去了哪

| 动作 | 是否提供端点 | 说明 |
|---|---|---|
| **R** 读历史 | ✅ `GET /chat/personas/:personaId/messages` | 本功能唯一对外的端点 |
| **C** 创建 | ❌ **没有 `POST /chat/messages`** | 消息**只能由服务端产生**：`role='user'` 的消息在 SSE 链路的入口落库、`role='assistant'` 的消息在流式生成完成后落库（技术文档 §5.1）。**绝不允许前端直接 POST 一条消息**——那等于让客户端伪造对话历史（也能伪造 `role='assistant'`，让"AI 说过的话"变成用户可写的字段） |
| **U** 编辑 / 撤回 | ❌ **不存在** | 总纲 §0.3：P2 明确不做消息编辑撤回。DDL 也没有 `updated_at`（§3.5） |
| **D** 删除 | ❌ **不存在** | 总纲 §0：对话历史不可单独清空。唯一删除路径是 `DELETE /personas/:id` → 外键级联（§3.4） |

**两条"不要写"的硬要求：**

1. **不写任何 `DELETE FROM chat_messages`**：删消息只有一条路——删人设，级联交给外键。手写删除等于绕开 CASCADE，还会在校验越权的地方多开一个口子。
2. **不留预留路径**：不写 `PUT /chat/messages/:id`、不写 `POST /chat/messages`，连注释掉的空 handler 都不要留（同 `/emotion/diary` 的处理方式——预留路径会被后来的人当成"已经规划好的功能"实现出来，而它与总纲 §0 冲突）。

### 4.4 写入能力（仓储层契约，供三条链路共用）

本功能**不实现写入端点**，但必须把写入方法定下来，否则 SSE / 主动消息 / 日程提醒会各写一份 INSERT。

| 方法 | 签名 | 用途 |
|---|---|---|
| 写一条消息 | `message_repo.Create(ctx, tx *gorm.DB, m *model.ChatMessage) error` | **三条链路共用**。事务句柄由调用方传（红线 7：事务边界在 service）；`m.ID` / `m.CreatedAt` 由 GORM 回填 |

| 写入方 | 何时 | `role` / `is_nudge` | 备注 |
|---|---|---|---|
| SSE 对话链路（成员 1，Week 2） | 收到用户消息 → 落库 user 消息 → 流式生成 → 完整回复落库 assistant 消息 | `user` / `false`；回复为 `assistant` | 情绪标签由 AI 服务分析后随消息写入（本功能的列已就位）。**落库完成后才发 `done`**（契约 §6） |
| 主动消息（成员 3） | 到点注入 `[nudge]`（技术文档 §5.4） | `user` / **`true`** | 与 SSE 复用同一条链路，不另写生成逻辑 |
| 日程提醒（成员 3，P1） | 到点触发（技术文档 §5.7） | `user` / **`true`** | 同上，复用 `TriggerNow` 那一段 |

**写入方的三条不变量**（写进注释，复用时照做）：

1. **`user_id` 与 `persona_id` 必须同时带上**，且 `user_id` 只能来自 Token / 人设行，**绝不从请求体取**（AGENTS §4.3：只带 `persona_id` 会跨用户串号，它是全局自增）。
2. **同一事务内更新 `personas.last_message_at = 该消息的 created_at`**（技术文档 §6.1：它是对话列表排序键 + 主动消息空闲判定）。写法必须是**指定单列更新**：

   ```go
   tx.Model(&model.Persona{}).Where("id = ? AND user_id = ?", personaID, userID).
       Update("last_message_at", m.CreatedAt)
   ```
   ⚠️ **绝不能用 `Save(&persona)`**：那是全列写回，会把 `state`（含 `familiarity`）覆盖成零值——`persona.md` spec §5.3 已把这个坑列为反例，这里是它的第二次出现点。
3. **`familiarity` 的累加不在本功能**（技术文档 §5.6），但它是**同一事务里的同一个 `state`**：写入方改 `state` 时不要重新序列化整个 JSON，只更新 `familiarity` 键（否则会把 `self_note` 等未知键丢掉）。

## 5. 越权防线（本功能最重要的正确性约束）

> 对应红线 3：**❌ 跨用户 / 跨人设数据泄漏**。消息是**最直接的隐私**——泄漏它等于泄漏两个人之间的全部对话内容，比泄漏一个人设名字严重得多。

### 5.1 三层防线

**① 入口层（Handler）——`user_id` 只有一个来源**

- `user_id` 只能从 JWT 声明取（`JWTAuth` 中间件写入 `gin.Context`），**永不从 body / query / path / header 读**。类型 `uint64`，用 `c.GetUint64(...)`。
- 取不到（类型断言失败 / `0`）→ **返回 `4010`，直接中断**，绝不退化成 `userID = 0` 继续查。`WHERE user_id = 0` 会返回空集——**看起来没泄漏，实际是把鉴权失败伪装成了空历史**，这种"静默降级"比报错危险得多。
- 本端点**没有任何请求体**，`personaId` 走路径参数（前端无法借 body 影响归属）。

**② 仓储层（Repository）——两个条件写进 SQL**

- `ListByPersona(ctx, userID, personaID, offset, limit)` **强制带 `userID`**，SQL 里 `WHERE persona_id = ? AND user_id = ?`。
- **归属校验下沉到 SQL 的 `WHERE` 里**，而不是"先查出来、再在 Go 里 `if m.UserID != userID`"。后者忘写那个 `if` 就静默越权，且这类 bug 在 review 里很难看出（代码"看起来逻辑完整"）。
- 即使 service 已经判过人设归属，**这里的 `user_id` 条件也不能省**（详见 §5.5 的"第二道闸门"）。

**③ 数据层（DB）**

- `chat_messages.user_id` / `persona_id` 两个外键 + 两个索引（§3.4）。
- 删人设 / 删账号的清理由外键 CASCADE 完成，不靠应用层补刀。

### 5.2 「先判归属、再查消息」——本功能与人设 CRUD 最大的不同

人设 CRUD 的 `PUT/DELETE /personas/:id` 可以**一条** `WHERE id = ? AND user_id = ?` 同时完成"归属 + 命中"，因为**人设本身就是被操作的那一行**。消息不是：

```
GET /chat/personas/999/messages     ← 被校验的资源是「人设」，返回的资源是「消息」，是两张表
```

于是 ``SELECT ... FROM chat_messages WHERE persona_id = 999 AND user_id = <我> `` 返回空集时，**有两种完全不同的可能**：

| 可能 | 应返回 | 如果偷懒只看消息表 |
|---|---|---|
| 人设 999 是别人的 / 不存在 | **`4043`** | 返回 `200 + []`——**把越权伪装成了"这个对话还是空的"** |
| 人设 999 是我的，但我还没聊过 | **`200 + list: []`** | 正确 |

**正确顺序（三步，一次不多）：**

```go
// ① 先判人设归属（读 personas 表，两个条件）——未命中一律 4043
owned, err := s.personaRepo.ExistsOwnedByUser(ctx, userID, personaID)
if err != nil { return nil, errcode.Wrap(errcode.ErrDBFailed, err) }
if !owned    { return nil, errcode.New(errcode.ErrPersonaNotFound) }

// ② 再查消息（读 chat_messages 表，两个条件）；空集是合法结果，返回 200 + []
list, total, err := s.messageRepo.ListByPersona(ctx, userID, personaID, offset, limit)
```

**`ExistsOwnedByUser` 放哪**：放 `internal/repository/persona_repo.go`（属人设模块，本分支一并落地）。它读的是 `personas` 表——**归属语义只能有一个实现**，chat 侧不要自己再写一份 `SELECT ... FROM personas`。签名：

```go
// ExistsOwnedByUser 判断该人设是否存在且属于该用户。
// 注意：调用方拿到 false 时必须一律按「人设不存在」返回 4043，
// 不允许区分「不存在」与「不属于你」——两者可区分就等于泄漏了 persona_id 的存在性。
ExistsOwnedByUser(ctx context.Context, userID, personaID uint64) (bool, error)
```

### 5.3 错误码：只有 `4043`，不产生 `4030`

沿用队长 2026-09-13 的通用规则：**资源越权 → `4043`**（隐藏资源存在性）、**功能越权 → `4030`**。本端点属**资源越权**侧，因此：

- `:personaId` 未命中（不存在 / 不属于你 / 解析失败 / 为 0）→ **一律 `errcode.ErrPersonaNotFound`（4043）**，**message 逐字相同**。
- 本模块**没有功能越权场景**（契约 §5 的端点对所有登录用户一视同仁），因此**不产生 `4030`**。
- 该规则已在契约 §2 生效（`4043` 的文案已含「资源越权也返回此码」，`4031` 已废弃），**本功能不需要改全局文件、不阻塞合并**。

**代价（诚实记录）**：调试时"看别人的对话"与"看不存在的人设"返回同一个码，排查略麻烦——靠服务端日志区分（`BizErrorHandler` 会为 4xxx 记 warn 日志）。

### 5.4 反例清单（AI 最容易写出来的错法）

| ❌ 错法 | 后果 | ✅ 正确 |
|---|---|---|
| 只查消息表，空集就返回 `200 + []` | **把"看别人的对话"伪装成"对话是空的"**，且与契约不符 | 先 `ExistsOwnedByUser` 判归属，未命中 `4043`（§5.2） |
| `db.First(&m, personaID)` 再在 Go 里比 `UserID` | 忘写比较就静默越权；多一次往返 | 归属条件写进 SQL |
| `ListByPersona` 不带 `user_id` 条件 | 凭一个人设 id 就能读别人的**全部聊天记录** | `WHERE persona_id = ? AND user_id = ?` |
| `total` 用 `COUNT(*)` 全表 | 数字错 + **泄漏全站消息量级** | 同条件 `COUNT` |
| 用 `4001` 表示"personaId 不是数字" | 契约本端点只列了 `4043`，凭空多一个码 = 契约漂移 | 统一 `4043`（§4.2 要求 6） |
| 让 `pageSize` 无上限 | `pageSize=100000` 一次拉全表 | 钳到 100（§4.2 要求 5） |
| 只按 `created_at DESC` 排序 | 同秒消息顺序不定，**翻页重复/漏项** | 补 `id DESC` |
| 空列表返回 `null` | 前端 `.map` 崩 | `make([]T, 0)` |
| `emotionLabel` 用 `string` 而非 `*string` | `null` 变成 `""`，前端无法区分"没分析出"与"空标签" | 指针（§3.3 第 3 条） |
| 顺手把 `emotionLabel` / `emotionScore` 从响应里删掉 | 契约 §5 明确它们是**回读字段**，删了就是契约漂移 | 照契约返回，**由 UI 保证不渲染** |
| 给消息加 `DELETE` / `PUT` 端点或预留路径 | 违反总纲 §0（不可单独清空、P2 不做编辑撤回） | 不写（§4.3） |
| 加 `POST /chat/messages` 让前端写消息 | 客户端能伪造 `role='assistant'`，对话历史失去可信度 | 消息只能由服务端落库（§4.3） |
| 写入时忘了更新 `last_message_at` | 对话列表不刷新、主动消息的空闲判定永远算不出来 | 同一事务内指定单列 `Update`（§4.4 不变量 2） |
| 更新 `last_message_at` 用 `Save(&persona)` | `state`（含 `familiarity`）被零值覆盖，**不可逆** | `Update("last_message_at", ...)` |
| `ChatMessage.ID` 写 `type:bigserial` | `user_memory.source_message_id` 长出 `nextval` 默认值，漏传值时静默指向不存在的消息 | `type:bigint` + `autoIncrement` |

### 5.5 一处必须说清楚：这里的"归属探针"为什么不算被禁的那一个

`persona-model` spec §5.1 明确要求**仓储层不提供任何"按 id 单查"或"判断存在性"的方法**。本功能却要一个 `ExistsOwnedByUser`，看似冲突，实际是两件事：

| | 人设 CRUD 里被禁的探针 | 本功能的 `ExistsOwnedByUser` |
|---|---|---|
| 调用方**拿到不同结果后的行为** | 返回**不同的错误码**（存在 vs 不存在）→ 外部可区分 | **一律 `4043`** → 外部无法区分 |
| 泄漏面 | 有：可逐个 id 二分探测出系统里有多少人设 | 无：所有非本人路径都收敛到同一个响应 |
| 为什么需要它 | **不需要**（一条 `WHERE id=? AND user_id=?` 就够了） | **必须有**：人设归属在 personas 表、消息在 chat_messages 表，空集无法自证归属（§5.2） |

**判据是"结果是否造成可观测的差异"，不是"有没有这个方法"**。这条也解释了 `message_repo.ListByPersona` 里那个看似冗余的 `user_id` 条件为什么必须留着：

> 它不是重复校验，是**第二道闸门**。如果将来有人给 service 加了别的调用分支、或直接调 repo（写测试、写脚本、SSE 落库后的回查），第一道闸门可能没经过——`WHERE` 里的 `user_id` 是最后一道，且它是结构性的（忘了传参就编译不过）。

## 6. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| 字段名**全 camelCase**，与契约 §5 逐字一致（`personaId` 不是 `persona_id`；**响应里没有 `userId`**） | 协作规范 §5 / 契约 §1 |
| **不新增错误码**；本功能只用 `ErrPersonaNotFound`(4043) / `ErrDBFailed`(5003) / `ErrUnauthorized`(4010) | 技术文档 §7.4 / AGENTS §4.1 |
| **资源越权 → `4043`**；本模块**不产生 `4030`**；`personaId` 非数字也走 `4043` | 队长规则 2026-09-13 ｜ §5.3 |
| **禁止硬编码错误码数字或文案**；`Fail()` 只接受 `ErrorCode` | **红线 6** |
| handler **不写** `response.Fail`，错误 `_ = c.Error(err)` 上抛 | 技术文档 §4.4 |
| handler 不直接操作 DB；service **不依赖 `*gin.Context`**（只收 `context.Context`） | **红线 7** ｜ AGENTS §4.1 |
| **归属校验在 service 层**，`4043` 由 service 返回；repo 只回 bool / 数据 | 成员 1 任务书 §3 |
| 消息查询**必须同时带 `persona_id` 与 `user_id`**（两个都带，缺一即泄漏） | **红线 3** ｜ AGENTS §4.3 |
| `PageResult[T]` **只有一份定义**（`dto/common_dto.go`），不要各写一份 | 协作规范 §3 / persona spec §4.1 |
| 情绪字段**返回但不展示**；SSE **没有 `emotion` 事件**；不给消息加展示标签 | **红线 5** ｜ AGENTS §4.4 |
| **不提供消息的 U / D 端点、不写 `DELETE FROM chat_messages`、不留预留路径** | 总纲 §0 / §0.3 ｜ §4.3 |
| 改表结构只改 struct + `AutoMigrate`，**禁止手写 `ALTER TABLE` / `DropTable`** | **红线 8** |
| 表名 `chat_messages`、列名小写下划线；Go 缩写词全大写（`UserID` 不是 `userId`）；ID 一律 `uint64` | 协作规范 §5 |
| 分支 `feature/backend-chat-message-model`，走 PR，**禁止直推 `main` / `develop`**；commit 格式 `<type>(<scope>): <subject>`，scope 用 `chat` / `model` | **红线 2** ｜ 协作规范 §6 |
| **不提交任何密钥**：本功能不新增任何 Key/Token/密码；验证用 Token 从登录接口取、走环境变量，**不写进脚本或 `_test.go`** | **红线 1** |

**红线自查（AGENTS.md §7 逐条对照）**

| 红线 | 本功能是否涉及 |
|---|---|
| ① 提交 `.env` / Key / 密码 / 模型权重 | 不涉及新增；**测试与验证脚本里也不许出现 Token 字面量** |
| ② 直推 `main` / `develop` / force push | 流程约束：本分支走 PR，至少 1 人 Approve |
| ③ 跨用户 / 跨人设泄漏 | **主要战场**，见 §5 |
| ④ 明文 / 弱哈希存密码 | 不涉及（不碰密码） |
| ⑤ 情绪展示标签 / 朋友圈手动触发 / 日程新建端点 | **相关**：情绪字段在本表上，`emotionLabel` / `emotionScore` **返回但不渲染**；不新增情绪列、不加 `emotion` 事件 |
| ⑥ 硬编码错误码或文案；`Fail()` 传自定义 message | 见 §6 上表 |
| ⑦ handler 直接操作 DB；service 依赖 `*gin.Context`；组件写 axios | 见 §6 上表 |
| ⑧ 生产 `DropTable`；手写 `ALTER TABLE` | 见 §6 上表 |

## 7. 依赖与交接

### 7.1 我依赖谁

| 依赖 | 提供方 | 现状（2026-09-14） | 影响 |
|---|---|---|---|
| `pkg/errcode`（4043 / 5003 / 4010） | 成员 1 | ✅ 已合入 develop | 无 |
| `pkg/response`（`Success[T]`） | 成员 1 | ✅ 已合入 develop | 无 |
| `internal/model/persona.go` | 成员 3（本分支的已有产物） | ✅ 已落地 | 无 |
| `internal/model/jsonb.go` | 成员 3 | ✅ 已落地 | 无（本功能不直接用它，但 `persona.state` 的更新依赖它） |
| `internal/dto/common_dto.go`（`PageResult[T]`） | 成员 3（persona spec 步骤 0） | **尚未落地** | **本功能第一个提交就是它**（见 plan 步骤 0），**不要写第二份** |
| `internal/middleware`（`JWTAuth` + `BizErrorHandler` + `ContextKeyUserID`） | 成员 1 | **尚未落地**（backend-skeleton 步骤 7） | service 能写，handler 末一段无法编译 |
| `router.go` 挂载点 | 成员 1 | **尚未落地**（backend-skeleton 步骤 8） | 端点不可达 |

> **可以立即开工**：`model/chat_message.go`、`migrate.go` 追加、`dto/common_dto.go`、`dto/chat_dto.go`、`repository/message_repo.go`、`persona_repo.go` 的归属方法——**全都不依赖成员 1 的公共层**（只需要已合入的 `pkg/errcode`）。
> **需要等**：service 的 error 分支要 `errcode`（已有）、handler 要 `middleware` + `router`（未落地）。**不要为了"跑起来"自己造一份 `pkg/middleware`**——那是成员 1 的文件，造了必冲突。

### 7.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `internal/model/chat_message.go` | **成员 1** | SSE 链路落库要引 `ChatMessage`；`user_memory.source_message_id` 的外键指向它 |
| `message_repo.Create(ctx, tx, m)` | **成员 1 + 成员 3** | 三条写入链路共用一条 INSERT，**签名固定后又不要各写一份** |
| `persona_repo.ExistsOwnedByUser(ctx, userID, personaID)` | **成员 1** | SSE 链路要用（它的 service 也要判归属）。后续它会需要 `GetOwned`（拿人格字段拼 prompt），建议**一次加齐**，避免两次改同一文件 |
| `GET /chat/personas/:personaId/messages` | **成员 2** | 聊天页历史加载 + 往上翻页 |
| **倒序语义**（最新在前） | **成员 2** | 前端渲染前要自己反转（§4.2 末注）；契约里没写排序，**这条只能从这里拿到** |
| `isNudge=true` 的消息 | **成员 2** | 它是 `role='user'` 但**不是用户打的字**。前端怎么渲染（是否展示 `[nudge]` 原文）**需要与成员 2 单独对齐**——后端只保证字段准确、`content` 原样返回 |
| `PageResult[T]` + 分页钳制常量 | **成员 1、成员 3** | 五个分页端点共用，**不要各写一份默认值** |
| `last_message_at` 的更新不变量 | **成员 1（SSE）、成员 3（主动消息）** | 见 §4.4 不变量 2；写错会让对话列表排序与主动消息空闲判定一起失效 |

### 7.3 待确认（开工前在群里问清，不要自己拍板）

| # | 问题 | 建议 |
|---|---|---|
| 1 | **文件归属**：`message_repo.go` / `chat_service.go` / `chat_handler.go` 按 MEMBER_1 任务书属成员 1；本分支要落地它们吗？ | 本分支按分支名先落 **model + repo + 读端点**（都不依赖公共层，能独立验收），但对齐后再动 `chat_service.go` / `chat_handler.go`，**避免两人同时写同一文件**（AGENTS §4.8 同款理由） |
| 2 | **分页钳制常量放哪**：`common_dto.go` 现在是「只放 `PageResult[T]`」（persona spec 步骤 0） | 建议把 `PageSizeDefault=20` / `PageSizeMax=100` / `ClampPage()` 一并放进 `common_dto.go`——它已经是"分页约定的唯一实现"，五个端点共用；persona 分支同步采用，**不要两处各写一份默认值** |
| 3 | **`emotion_score` 的 Go 类型**：`NUMERIC(4,3)` ↔ `*float64` 在写入时（pgx 编码）是否顺畅 | 先用 `*float64`；若实测报类型不匹配，退到自写 `driver.Valuer`（返回字符串，Postgres 对 numeric 接受字符串字面量）。**这个决定要在 SSE 那一步写入前定下来** |
| 4 | **`isNudge` 消息的前端渲染方式** | 与成员 2 对齐：显示 `[nudge]` 原文、还是折叠成"AI 主动发起"。后端不参与，但契约里 `content` 含标记这件事要说清 |
| 5 | `[nudge]` 的内容格式（前缀怎么写、要不要换行） | 由成员 3 的主动消息模块与成员 1 约定，**本功能只负责原样存取**；一旦定了就是两个模块的公共约定 |

## 8. 验收标准

- [ ] `cd backend && go build ./... && go vet ./... && go test ./...` 全通过
- [ ] `AutoMigrate` 追加 `&ChatMessage{}` 后，`psql` 里 `\d chat_messages` 能看到：
  - [ ] **两个** `ON DELETE CASCADE` 外键（`user_id → users`、`persona_id → personas`）
  - [ ] `role` 的 **CHECK 约束**（`role IN ('user','assistant')`）
  - [ ] `idx_messages_persona_time` 与 `idx_messages_user_id`
  - [ ] **没有** `updated_at` / `deleted_at` 列（§3.5）
- [ ] **序列不污染**：`\ds` 里只有 `chat_messages_id_seq`；`ChatMessage.ID` 的 tag 是 `type:bigint`（将来写 `user_memory.source_message_id` 时，那一列**不能**带 `nextval` 默认值）
- [ ] 正常读取：造 3 条消息 → `200`、`total=3`、`list` 按 `createdAt` **倒序**
- [ ] **空对话**：属于自己但没消息的人设 → **`200` + `list: []`**（不是 `null`、不是 `4043`）
- [ ] **资源越权**：用 B 的 Token 打 A 的 `personaId` → **`4043`**
- [ ] **不存在**：不存在的 id → **`4043`**，且与上一条**同码同文案**（外部无法区分——这正是隐藏存在性的效果）
- [ ] **不产生 `4030` / `4001`**：任意输入组合下（越权 / 不存在 / `page=abc` / `pageSize=100000` / `personaId=abc` / 无 Token）只出现 `4043` / `4010` / `200`
- [ ] **分页钳制**：`pageSize=100000` → 返回至多 100 条，响应里 `pageSize` 为 `100`；`page=0` → 按第 1 页处理
- [ ] **分页稳定**：把两条消息的 `created_at` 手工改成完全相同 → 用 `pageSize=1` 逐页拉，**不重不漏**（`id DESC` 兜底生效）
- [ ] **越界页**：`page=999` → `200` + `list: []` + 真实 `total`
- [ ] 不带 Token / Token 无效 → `4010` / `4011`（由中间件产生）
- [ ] **`total` 不透全局**：回应里的 `total` 等于该人设的消息数，不是全表行数（造两个账号各若干消息，互相对照）
- [ ] **级联删除**：删人设后 `SELECT count(*) FROM chat_messages WHERE persona_id = <已删id>` = 0
- [ ] **情绪字段回读**：无情绪的行返回 `"emotionLabel": null` / `"emotionScore": null`（不是 `""` / `0`）；有情绪的返回 `0.000~1.000` 的小数
- [ ] **契约字段逐字对齐**：响应对象 8 个字段（`id` `personaId` `role` `content` `emotionLabel` `emotionScore` `isNudge` `createdAt`），**没有 `userId`**
- [ ] **`last_message_at` 不变量**：写入一条消息（走 `Create`）后 `personas.last_message_at` 等于该消息的 `created_at`，且 `state` / `familiarity` **未被改动**
- [ ] code 评审 grep 通过：无硬编码错误码数字/文案、无 `response.Fail` in handler、无 `Save(`、无 `DELETE FROM chat_messages`、无 secrets
- [ ] PR 已开、至少 1 人 Approve；commit 符合 `<type>(<scope>): <subject>`

## 9. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-14 | v1 | 创建 | 聊天记录（模型 + 仓储 + 读端点）开工前的设计与验收基线；含「CRUD 只有 R 是端点」的边界、倒序分页语义、越权防线的两张表判定 |

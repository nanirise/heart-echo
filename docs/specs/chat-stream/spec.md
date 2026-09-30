# spec · 流式对话（chat-stream）

> 本文件是**合同**：定义本功能做什么、做到什么算完成。合并后即冻结，任何字段或行为变更必须升版本并在群里广播。
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：SSE 事件格式、错误码表、通用约定一律**不复制**，只给链接；
> 本文件只写**本功能独有**的内容——独有陷阱、独有决策、独有验收项。

| 项 | 值 |
|----|-----|
| 功能名 | `chat-stream` |
| 分支 | `feature/chat-stream-ai-service` → `feature/chat-stream-go` |
| 状态 | 🟡 草稿（骨架已定，细则待补） |
| **本功能范围** | `POST /api/v1/chat/stream` 的**全链路三段**：Go handler/service/ai_client ↔ Python `/internal/chat/stream` ↔ 前端 `streamChat`（已合入，格式冻结） |
| 负责人 | 成员 1 |
| 关联契约 | [API_CONTRACT §5 / §6](../../API_CONTRACT.md)（对话 + SSE，**§6 已冻结，逐字以此为准**） |
| 关联设计 | [TECH_DESIGN §5.1 / §5.2](../../TECH_DESIGN.md)、[总纲 §0](../../dev/MASTER.md)、[成员 1 任务书 §3](../../dev/MEMBER_1_BACKEND_AI.md) |
| 关联红线 | [AGENTS §4.3](../../../AGENTS.md)（数据边界）、§4.4（情绪内部信号）、§4.5（SSE 跨四段）、§5.1 |

---

## 0. 范围声明（先读这一节）

本功能是 [总纲 §7](../../dev/MASTER.md) 写的 **Week 2 生死线本体**。它的验收标准只有一条：
**前端能看着字一个个出来，而不是等到最后一次性出现。** 其他一切都是次要的。

### 0.1 做什么

| 层 | 文件 | 内容 |
|---|---|---|
| Python | `ai-service/app/main.py`、`api/routes.py`、`core/config.py`、`core/security.py`、`core/llm_client.py` | 服务骨架 + `X-Internal-Token` 校验 + DeepSeek 流式客户端 + `/internal/chat/stream` |
| Go | `internal/service/ai_client.go` | `AIClient` 接口 + HTTP 实现（接口抽象见 [TECH_DESIGN §5.0](../../TECH_DESIGN.md)） |
| Go | `internal/dto/chat_dto.go` | `StreamChatRequest` |
| Go | `internal/service/chat_service.go` | 编排：判归属 → 落 user → 调 AI → 转发 → 落 assistant → 发 `done` |
| Go | `internal/handler/chat_handler.go` | SSE 响应头 + 逐事件写出 + `RegisterChatRoutes` |
| Go | `internal/handler/router.go` | 加一行挂载到 `protected` 组 |

### 0.2 不做什么

| 不做的事 | 去向 |
|---|---|
| **情感分析**（`emotionLabel` / `emotionScore` 的写入） | Week 3 的 emotion spec。本功能**这两个字段一律写 `null`**，但**列必须存在且类型正确**（`chat-message` spec §1.3 已定） |
| 记忆检索 / 记忆注入 prompt / 记忆提取 | Week 3。本功能的 Python prompt **不含记忆段** |
| `GET /chat/personas/:personaId/messages` 读端点 | 属 [chat-message spec](../chat-message/spec.md)，**不重复实现** |
| `[nudge]` 注入、主动消息、日程提醒 | 成员 3。本功能只保证 `role` / `is_nudge` 的写入正确 |
| `familiarity` 累加 | Week 3 人格演化 |
| `POST /chat/messages` 之类的非流式端点 | `chat-message` spec §2：**消息只能由服务端产生**，不存在用户可写的消息端点 |
| 重新生成回复 | 总纲 §0.3：**不做**。仅 SSE `error` 时前端给重试提示 |
| 前端 `streamChat` / `consumeStream` | **成员 2，已合入 develop**。本功能只做**反向对齐**，不改前端 |

---

## 1. 本链路独有的硬要求

> 这一节是整份 spec 的价值所在。每条都对应一个「本地测不出来、上线才炸」或「炸了不知道去哪查」的坑。

### 1.1 归属校验必须在写响应头之前 —— 本功能最大的坑

契约 §5 给 `/chat/stream` 列的五个错误码（`4001` `4010` `4043` `5001` `5002`）**是两类东西**：

| 类 | 码 | 何时 | 形态 |
|---|---|---|---|
| **请求阶段** | `4001` `4010` `4043` | **进 SSE 之前** | 普通 JSON 响应 + HTTP 4xx（由 `BizErrorHandler` 出口） |
| **生成阶段** | `5001` `5002` | **已进 SSE 之后** | `event: error` 事件 + **HTTP 200** |

**一旦 `c.Writer.WriteHeader(200)` 写出 SSE 响应头，就再也回不去 `4043` 了。** 拿不到 `4043` 的越权请求会得到：HTTP 200 + 一个永远不结束的流 + 前端静默。

所以顺序**必须是**：

```
解析请求体 → 判人设归属（4043）→ 校验通过 → 才写 SSE 响应头 → 才开始调 AI
```

前端的 [`chat.ts:39`](../../../frontend/src/api/chat.ts) 正是按这个假设写的：`response.ok === false` → 抛 `ApiError`。**这个契约是前端已经依赖的，不是我们自由设计的部分。**

### 1.2 `\n\n` 分隔符（契约 §6 已钉死）

Go 与 Python **两侧**都必须输出 `\n\n`，**不使用** `\r\n\r\n`。

本地前后端同在 Windows，两种写法**都正常**；差异只在部署到 Linux 容器后才出现，那时的现象是「页面一个字都不出、控制台不报错」。
前端的切块方式是 `buffer.split('\n\n')`，不要求做归一化。

### 1.3 Go 不是纯管道 —— `done` 由 Go 补发

**Python 不知道 `messageId`**：它是 assistant 消息落库后的主键，而**落库是 Go 做的**（`message_repo.Create`，[chat-message spec §4](../chat-message/spec.md)；Python 不碰 DB）。

因此三段的分工是：

```
Python  ── delta × N ──►  end（内部终止事件，不带 messageId）
                                       │
Go      ── delta × N ──►  （吞掉 end）→ 落库 assistant → 发 done {"messageId": N}
```

- `end` 是**内部事件**，只存在于 Go ↔ Python 之间，**绝不透传给前端**（契约 §6 冻结了三种事件，多一种就是契约漂移）。
- `done` 由 Go 发，**落库成功之后**才发。落库失败 → 发 `error`（`5003`），不发 `done`。
- Go 转发 `delta` 时**逐事件转发**（不攒批），否则打字机效果会变成一段一段地蹦。

### 1.4 每次 write 后必须 Flush

响应头 `X-Accel-Buffering: no` + 每次写完 `c.Writer.Flush()`，**少任何一个**的表现都是「字全到了但最后一次性出现」。
这两项对应两个不同的缓冲层：`X-Accel-Buffering` 管 Nginx，`Flush()` 管 Go 自己的响应缓冲。**两个都要**。

### 1.5 落库事务边界：两次，不合并

| 事务 | 内容 |
|---|---|
| ① | `user message` 落库 + `personas.last_message_at` 更新 |
| ② | `assistant message` 落库 + `personas.last_message_at` 更新 |

**中间隔着几秒的 LLM 生成，绝不能拿一个长事务把生成包住**——那会把连接和行锁一起攥住几秒。

`last_message_at` 的写法见 [chat-message spec §4 不变量 2](../chat-message/spec.md)：**指定单列 `Update`**，**绝不能用 `Save(&persona)`**（会覆盖 `state.familiarity`，不可逆）。

### 1.6 内部接口用 snake_case

Go → Python 的请求体是 **snake_case**（`user_id` / `persona_id` / `message`），与对外契约的 camelCase **刻意不一致**。

理由不是风格：**能一眼区分「这是前端请求」和「这是内部调用」**。混淆两者是跨用户串号的经典入口——内部调用的 `user_id` 只能来自 Token，绝不能从请求体取（AGENTS §4.3）。

### 1.7 内部路径一律带 `/internal` 前缀

`/internal/chat/stream`、`/internal/emotion/analyze`、`/internal/memory/extract`、`/internal/schedule/parse`（后三个归 Week 3 / P1，本 spec 只定命名）。

前缀的作用是**让"只给 Go 调"这件事写在路径上**。与成员 3 的 `ai-moment/spec.md:615`（`/internal/moment/generate`）对齐。

> ⚠️ 这条与 [成员 1 任务书 §6](../../dev/MEMBER_1_BACKEND_AI.md) 现在的写法（`/chat/stream`）不一致，任务书需同步改。

---

## 2. 接口定义

### 2.1 Go 对外：`POST /api/v1/chat/stream`

**逐字以 [契约 §6](../../API_CONTRACT.md) 为准**，本 spec 不复制。要点：

- 请求体 `{personaId, content}`，挂在 `protected` 组下（需 `Authorization: Bearer <access token>`）
- 响应头四项 + 事件三型（`delta` / `done` / `error`）
- `delta.data` = `{"text":"..."}`；`done.data` = `{"messageId":N}`；`error.data` = `{"code":N,"message":"..."}`

### 2.2 Go → Python：`POST /internal/chat/stream`

**请求**

```json
{
  "user_id": 1,
  "persona_id": 1,
  "message": "今天上班好累啊"
}
```

**请求头**：`X-Internal-Token: <AI_SERVICE_TOKEN>`（两侧值必须一致，不一致恒 401 —— AGENTS §4.3）

**响应**：`text/event-stream`，事件同 §1.3 —— `delta` × N → `end`。

> 🚧 **待补细则**：`ChatRequest` 的完整字段（是否带人格描述 / 历史消息轮数 N）由 prompt 组装需要什么决定，见 §5 待确认 #1。

### 2.3 错误路径

| 场景 | 表现 |
|---|---|
| `personaId` 非本人 / 不存在 / 解析失败 | `4043`（**进 SSE 之前**，普通 JSON） |
| 请求体缺字段 / 格式错 | `4001`（进 SSE 之前） |
| 无 Token / Token 过期 | `4010` / `4011`（中间件产生） |
| Python 服务连不上 | `event: error` + `5002` |
| DeepSeek 报错 / 超时（重试 2 次后仍失败） | `event: error` + `5001` |
| assistant 落库失败 | `event: error` + `5003`（**不发 `done`**） |

> 🚧 **待补细则**：`event: error` 发出后 **user message 已经落库了**，前端点重试会再 POST 一次 → **库里会多一条一模一样的 user 消息**。这个是不是要处理，见 §5 待确认 #2。

---

## 3. 依赖与交接

### 3.1 依赖

| 依赖 | 提供方 | 现状 | 影响 |
|---|---|---|---|
| `pkg/errcode`（4001 / 4043 / 5001 / 5002 / 5003）、`pkg/response` | 成员 1 | ✅ 已合入 | 无 |
| `internal/middleware`（`JWTAuth` + `BizErrorHandler`） | 成员 1 | ✅ 已落地 | 无 |
| `persona_repo.FindOwned(ctx, userID, personaID)` | 成员 3 | ✅ 已落地 | 判归属 + 取人格字段（拼 prompt）一次拿齐 |
| `message_repo.Create(ctx, tx, m)` | 成员 3 | ❌ **未落地** | **硬前置**，见下 |
| `internal/dto/common_dto.go` | 成员 3 | ✅ 已落地 | 无 |
| 前端 `streamChat` / `consumeStream` | 成员 2 | ✅ 已合入 | 格式已冻结，本功能只能对齐 |

### 3.2 ⛔ 硬前置：`chat-message` 的仓储层还没落地

`dto/chat_dto.go`、`repository/message_repo.go`、`service/chat_service.go`、`handler/chat_handler.go` **四个文件都不存在**（2026-09-30 实测）。
`chat-message` spec 只交付了模型层。

**本功能必须在 `message_repo.Create` 落地后才能开始落库那一步**，否则 SSE 会顺手自己写一份 `INSERT`，两套写法必然对不上（尤其 `created_at` 与 `last_message_at` 的同步）——这正是 `chat-message` spec §0 存在的理由。

> 🚧 **待补细则**：这四行文件归属按 [chat-message spec §6.3 #1](../chat-message/spec.md) 是「待对齐」状态。本 spec 开工前必须先和成员 3 敲定：**`chat_service.go` / `chat_handler.go` 归我**（任务书 §0），`message_repo.go` / `chat_dto.go` 归谁。

### 3.3 交接

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `/internal/chat/stream` 契约（§2.2） | 成员 3（`ai-moment` 参照格式） | 内部接口命名与鉴权头的统一写法 |
| `AIClient` 接口（`internal/service/ai_client.go`） | Week 3 的 emotion / memory | 三条链路共用一个客户端抽象 |
| `done` 的补发逻辑（§1.3） | 成员 3（主动消息） | 主动消息落库后同样要发 `done`，**不要各写一份** |

---

## 4. 验收标准

**A · 分段联调（**这一节是生死线的判据**）**

按 [任务书 §3](../../dev/MEMBER_1_BACKEND_AI.md) 的四层顺序，**每加一层验证一次**，不要一次全接通再调：

- [ ] **第 4 步 · 直连 Python**：`curl -N -X POST http://localhost:8000/internal/chat/stream -H "X-Internal-Token: $AI_SERVICE_TOKEN" -d '{"user_id":1,"persona_id":1,"message":"你好"}'` → 看到 `event:` / `data:` **逐条**输出
- [ ] 第 5 步 · 经 Go：`curl -N -X POST http://localhost:8080/api/v1/chat/stream -H "Authorization: Bearer $TOKEN" -d '{"personaId":1,"content":"你好"}'` → 同样逐条
- [ ] 第 6 步 · 经 Nginx（部署后）
- [ ] 第 7 步 · 浏览器：**打字机效果**（字一个个出，不是一次性出现）

> **第 4 步不过就不要往上查 Go。** 跨进程排查成本集中在 SSE，四层里哪层坏了要能一眼定位。

**B · 契约与字段**

- [ ] 事件只有三种，**没有 `emotion` 事件**、**没有 `end` 透传**
- [ ] 响应头四项齐全（含 `X-Accel-Buffering: no`）
- [ ] 分隔符是 `\n\n`，**不是** `\r\n\r\n`（用 `curl -N ... | xxd | grep 0d` 验）
- [ ] `done.messageId` 是 **assistant** 消息的 id，且该行确实在库里
- [ ] 🚧 待补更多条目

**C · 越权与错误**

- [ ] 用 B 的 Token 打 A 的 `personaId` → **`4043`（普通 JSON，HTTP 404）**，**不是** 200 + 空流
- [ ] 同理 `4010` / `4001` 都不进 SSE
- [ ] 掐掉 Python 服务 → `event: error` + `5002`，HTTP 仍 200
- [ ] 🚧 待补更多条目

**D · 数据不变量**

- [ ] user 消息与 assistant 消息**都被落库**，`role` 正确
- [ ] `personas.last_message_at` 等于**最后一条消息**的 `created_at`
- [ ] `state.familiarity` / `state.self_note` **未被改动**（`Save(&persona)` 坑）
- [ ] `emotionLabel` / `emotionScore` 为 `null`（本阶段不写情绪）

**E · 代码层**

- [ ] grep 通过：无硬编码错误码、无 `response.Fail` in handler、无 `Save(`、无 secrets
- [ ] `ai-service/` 有 `.env.example`，且 `AI_SERVICE_TOKEN` 与 backend 侧同值

---

## 5. 待确认

| # | 问题 | 建议 |
|---|---|---|
| 1 | `ChatRequest` 的完整字段（要不要带人格描述 / 历史消息） | 由 [TECH_DESIGN §5.2](../../TECH_DESIGN.md) 的 prompt 结构反推：**人格字段与历史都要**，但历史取多少轮待定。Week 2 最小路径可先只带 `message`，人格与历史的拼接放 Python 侧（它需要自己查）——**但 Python 不碰 DB**，所以历史必须由 Go 传 |
| 2 | `error` 后前端重试 → 库里多一条重复 user 消息 | 见 §2.3。建议**本阶段不处理**，在 spec 里记为已知取舍；真要治需要客户端幂等键，属契约变更 |
| 3 | Python 的历史消息从哪来 | Go 传给 Python（`history: [{role, content}]`），还是 Python 回调 Go？**前者**——Python 不碰 DB 是既定架构 |
| 4 | 内部接口的 `/internal` 前缀 | 已在 §1.7 定为**加**。需同步改 [成员 1 任务书 §6](../../dev/MEMBER_1_BACKEND_AI.md)，并广播给成员 3（他的 `ai-moment` spec 已是这个写法） |

---

## 6. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-30 | v0.1 | 创建（骨架） | Week 2 生死线开工前的设计与验收基线；定下五条独有决策：归属校验先于响应头、`\n\n`、Go 补发 `done`、两次事务、`/internal` + snake_case |

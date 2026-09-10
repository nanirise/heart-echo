# HeartEcho 接口契约（API Contract）

> **这是准备期必须冻结的文件。** 前端照此写 Mock，后端照此实现，字段以本文为准。
> 派生自 [技术文档 §7](TECH_DESIGN.md#7-api-规范)；协作流程见 [协作文档 §3.4](COLLABORATION.md#34-接口契约先行准备期完成否则第-2-周必定互相等待)。
> 项目全貌：[项目概览](../README.md)。

| 项 | 值 |
|----|-----|
| 契约版本 | v1（冻结后任何变更必须升版本号并广播） |
| 冻结日期 | 待填（准备期结束前） |
| 确认人 | 成员 1：______ ｜ 成员 2：______ ｜ 成员 3：______ |

**冻结流程**：三人一起逐条过一遍本文 → 有异议当场改 → 全部确认后在下方「冻结签署」处签字 → **此后改字段必须在群里广播并更新本文的「变更记录」**。

**状态标记**：`⬜ 待确认` ｜ `✅ 已冻结` ｜ `🔒 已锁定`（锁定 = 已开始联调，改动成本高）

---

## 1. 全局约定

| 项 | 约定 |
|----|------|
| Base URL | `/api/v1` |
| 认证 | `Authorization: Bearer <accessToken>` |
| 免鉴权白名单 | 只有 `POST /auth/register`、`POST /auth/login`、`POST /auth/refresh`、`GET /health` 四个。**其余端点一律需要 Token**，缺失或过期返回 `4010` |
| 请求体 | `application/json` |
| 响应体 | 统一 `{ code, message, data, timestamp }` |
| 时间格式 | RFC3339，如 `2026-09-10T14:30:00+08:00` |
| 分页参数 | `page`（从 1 开始）、`pageSize`（默认 20，上限 100） |
| 分页响应 | `{ list, total, page, pageSize }` |
| URL 命名 | kebab-case 复数名词；路径参数用 camelCase 且与字段同名，如 `:personaId` |
| JSON 字段 | **camelCase**（前后端必须一致，这是最容易出 bug 的地方） |
| 数据边界 | **账号之间完全隔离**：任何接口都只能读到当前 Token 对应用户自己的数据，不存在跨用户的列表、动态或评论。这是个人产品，不是社区 |

**前端解包约定**：`src/api/request.ts` 在 `code === 200` 时**直接返回 `data` 部分**，因此组件里拿到的就是下文各端点的「响应 data」结构，组件中不写 `res.code` 判断。

---

## 2. 统一响应与错误码

**成功**

```json
{ "code": 200, "message": "success", "data": { }, "timestamp": 1789000000000 }
```

**失败**（HTTP 状态码与业务 code 分离，前端以 `code` 判断语义、`message` 直接展示）

```json
{ "code": 4013, "message": "用户名或密码错误", "data": null, "timestamp": 1789000000000 }
```

**错误码总表**（一个 code 严格对应一个 msg，不允许前端自定义文案）

| code | HTTP | message | 触发场景 |
|------|------|---------|----------|
| 200 | 200 | success | 成功 |
| 4001 | 400 | 参数校验失败 | 字段格式/长度不合法 |
| 4002 | 400 | 必填参数缺失 | 缺少必填字段 |
| 4003 | 400 | 该邮箱已被注册 | 注册邮箱冲突 |
| 4004 | 400 | 该用户名已被占用 | 注册用户名冲突 |
| 4010 | 401 | 未登录或登录已过期 | 请求未带 Token |
| 4011 | 401 | Token 无效 | Token 解析失败 / 用 refresh 访问业务接口 |
| 4012 | 401 | Token 已过期 | access token 过期 → 前端刷新后重放 |
| 4013 | 401 | 用户名或密码错误 | 登录失败（不区分用户不存在，防枚举） |
| 4014 | 401 | 刷新令牌无效，请重新登录 | refresh 失败 |
| 4015 | 401 | 原密码不正确 | 修改密码时旧密码校验失败 |
| 4030 | 403 | 无权限访问该资源 | 越权 |
| 4031 | 403 | 该人设不属于当前用户 | persona 归属校验失败 |
| 4040 | 404 | 资源不存在 | 通用 |
| 4041 | 404 | 用户不存在 | |
| ~~4042~~ | — | ~~会话不存在~~（已废弃：无会话实体，一人设一对话） | 保留占位，勿复用 |
| 4043 | 404 | 人设不存在 | 含「该人设的对话不存在」——人设不存在则对话不存在 |
| 5000 | 500 | 服务端内部错误 | panic / 未分类错误 |
| 5001 | 500 | AI 回复生成失败，请稍后重试 | LLM 调用失败 |
| 5002 | 500 | AI 服务暂时不可用 | AI 服务不可达 |
| 5003 | 500 | 数据库操作失败 | DB 错误 |

**前端错误处理约定**：`4012` → 刷新 token 后重放原请求；`4010` / `4011` / `4014` → 清空登录态跳登录页（**不重试**）；其余按 `message` 提示。

---

## 3. 认证与用户

### 3.1 `POST /auth/register` — 注册 ⬜

请求：

```json
{ "username": "xiaoming", "email": "xm@example.com", "password": "Passw0rd!" }
```

| 字段 | 类型 | 必填 | 校验 |
|------|------|:----:|------|
| username | string | ✅ | 3-20 位，字母数字 |
| email | string | ✅ | 合法邮箱 |
| password | string | ✅ | 8-32 位，**限 ASCII 可见字符**（避免 bcrypt 72 字节截断，见技术文档 §4.8） |

响应 data：

```json
{
  "accessToken": "eyJhbGciOi...",
  "refreshToken": "eyJhbGciOi...",
  "user": { "id": 1, "username": "xiaoming", "email": "xm@example.com", "avatarUrl": null }
}
```

错误码：`4001` `4003` `4004`

> **注册即登录**：本接口已返回 token，前端直接写入登录态并跳转 `/chat`，**不要**再让用户去登录页手动登录一次。

### 3.2 `POST /auth/login` — 登录 ⬜

请求 `{ "username": "...", "password": "..." }`，响应 data 同注册。
错误码：`4001` `4013`

### 3.3 `POST /auth/refresh` — 刷新令牌 ⬜

请求 `{ "refreshToken": "..." }`，响应 data `{ "accessToken": "...", "refreshToken": "..." }`。
错误码：`4014`

### 3.4 `GET /user/profile` — 当前用户信息 ⬜

响应 data：`{ "id": 1, "username": "xiaoming", "email": "xm@example.com", "avatarUrl": null, "createdAt": "2026-09-10T14:30:00+08:00" }`

### 3.5 `PUT /user/profile` — 更新用户信息 ⬜

请求 `{ "avatarUrl": "https://...", "username": "newname" }`（两个字段都可选，只传要改的），响应 data 同上。

| 字段 | 约束 |
|------|------|
| avatarUrl | 图片 URL，最长 255；**只存 URL，不做文件上传**。传 `null` 可清空，前端用用户名首字母色块兜底 |
| username | 3-20 位字母数字，需唯一；冲突返回 `4004` |

错误码：`4001` `4004`

### 3.6 `PUT /user/password` — 修改密码 ⬜

请求 `{ "oldPassword": "...", "newPassword": "..." }`，响应 data `null`。

| 字段 | 约束 |
|------|------|
| oldPassword | 必填，用 bcrypt 与库中哈希比对，不符返回 `4015` |
| newPassword | 8-32 位，与注册同规则（限 ASCII 可见字符） |

错误码：`4001` `4015`

> 修改成功后**服务端不主动失效旧 Token**，前端负责清空登录态并跳回登录页。所有其他设备上的 Token 仍然有效——这是本期接受的简化。

---

## 4. 人设（Persona）

**实体结构**

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

| 字段 | 说明 |
|------|------|
| `state` | 人格状态原始 JSONB，前端**不解析、不回传修改**，只在 `PUT` 时原样保留 |
| `familiarity` | 亲密度 0-100，随对话轮数累加（技术文档 §5.6）。**只读**，`PUT` 时忽略 |
| `lastMessageAt` | 该对话最后一条消息时间；**从未聊过为 `null`**。列表按它倒序 = 对话列表排序 |

> `familiarity` 字段**契约层面已备好**；是否在界面上展示（如对话页顶部的进度条）由你们后期决定，不影响后端实现。

| 端点 | 请求 | 响应 data | 错误码 |
|------|------|-----------|--------|
| `GET /personas` ⬜ | `page` `pageSize` | `PageResult<Persona>` | — |
| `POST /personas` ⬜ | `{name, personalityDesc, speakingStyle}` | `Persona` | 4001 |
| `PUT /personas/:id` ⬜ | 同上 | `Persona` | 4001 4031 4043 |
| `DELETE /personas/:id` ⬜ | — | `null` | 4031 4043 |

> 所有涉及 `:id` 的操作，服务端必须校验该人设属于当前登录用户（`user_id`），否则返回 `4031`。
> 删除人设会**级联删除**该人设的全部消息与记忆关联，前端必须二次确认后再调。

---

## 5. 对话（Chat）

**ChatMessage**

```json
{
  "id": 348,
  "personaId": 1,
  "role": "user",
  "content": "今天上班好累啊",
  "emotionLabel": "sadness",
  "emotionScore": 0.87,
  "isNudge": false,
  "createdAt": "2026-09-10T14:30:00+08:00"
}
```

| 字段 | 约束 |
|------|------|
| role | `"user"` \| `"assistant"` |
| emotionLabel | 8 类之一或 `null`：`joy` `sadness` `anger` `fear` `surprise` `disgust` `neutral` `love`。**内部信号，界面不展示**，保留字段供画像聚合使用 |
| emotionScore | `0.000` ~ `1.000` 或 `null`。同上，不展示 |
| isNudge | 是否为主动消息注入（`role` 仍为 `"user"`） |

| 端点 | 请求 | 响应 data | 错误码 |
|------|------|-----------|--------|
| `GET /chat/personas/:personaId/messages` ⬜ | `page` `pageSize` | `PageResult<ChatMessage>` | 4031 4043 |
| `POST /chat/stream` ⬜ | 见 §6 | SSE | 4001 4010 4031 4043 5001 5002 |

> **一个人设 = 一个对话**。没有会话实体，也就没有"新建对话/删除对话/会话列表"这三组接口——对话列表直接复用 `GET /personas`（见 §4，按 `lastMessageAt` 倒序），删人设即删对话（消息级联删除）。

---

## 6. SSE 契约（`POST /chat/stream`）🔒

**请求**

```json
{ "personaId": 1, "content": "今天上班好累啊" }
```

**响应头**

```
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive
X-Accel-Buffering: no
```

**事件流**

```
event: delta
data: {"text":"辛苦"}

event: delta
data: {"text":"了，今天发生"}

event: delta
data: {"text":"什么了吗？"}

event: done
data: {"messageId":348}
```

**错误事件**（HTTP 状态仍为 200，错误通过事件传递）

```
event: error
data: {"code":5001,"message":"AI 回复生成失败，请稍后重试"}
```

| event | 时机 | data | 前端动作 |
|-------|------|------|----------|
| `delta` | LLM 每生成一段 | `{text}` | 追加渲染（打字机） |
| `done` | 全量回复落库完成 | `{messageId}` | 结束流，标记完成 |
| `error` | 生成失败 | `{code, message}` | 显示错误提示 + 重试按钮 |

> **为什么没有 `emotion` 事件**：情绪标签是内部信号，不向用户展示（用于记忆写入与回复策略）。前端拿不到、也不需要它。情感分析的效果通过 **AI 回复策略的差异**体现——这是答辩演示该功能的方式。

> **契约纪律**：事件类型不可私自增删；新增必须先广播。前端必须处理 `error` 事件，不能静默。

---

## 7. 记忆与画像

**记忆与画像都是「一个人设一份」。**

| 端点 | 请求 | 响应 data | 错误码 |
|------|------|-----------|--------|
| `GET /memory` ⬜ | `personaId` `page` `pageSize` | `PageResult<MemoryItem>` | 4031 4043 |
| `GET /profile/portrait` ⬜ | `personaId` | `{ "personaId": 1, "profileData": {}, "updatedAt": "..." }` | 4031 4043 |

> 前端**必须传 `personaId`**（切到哪个伴侣就看谁的记忆/画像）；服务端除校验人设归属外还会带上 `user_id` 条件。**前端永远不传 `user_id`**——它从 Token 取。
> 本期记忆**只读**，不提供删除接口。

**MemoryItem**

```json
{ "id": 7, "personaId": 1, "memoryType": "fact", "content": "用户养了一只叫豆豆的猫", "importanceScore": 0.85, "createdAt": "2026-09-10T14:30:00+08:00" }
```

`memoryType`：`fact` \| `preference` \| `event`
（**没有 `emotion`**：情绪是内部信号，不作为长期记忆存储）

> `GET /emotion/trend` 为 **P2 预留端点，本期不实现**。
> **没有 `/emotion/diary`**：情绪标签是内部信号（见 §5），"情绪日记"本质是把标签呈现给用户，与全局约定冲突，因此**连预留路径都不留**。

---

## 8. AI 朋友圈（Moments）

**Moment**

```json
{
  "id": 3,
  "personaId": 1,
  "personaName": "小暖",
  "content": "今天下雨了，突然想喝热可可。",
  "likeCount": 2,
  "liked": true,
  "commentCount": 1,
  "createdAt": "2026-09-10T14:30:00+08:00"
}
```

| 字段 | 约束 |
|------|------|
| `likeCount` | 该动态的点赞总数 |
| `liked` | **当前登录用户**是否已点赞（决定按钮高亮） |
| `commentCount` | 评论数，**派生字段**（由 `moment_comments` 聚合，不是表上的列） |

> **没有 `emotionLabel`**：AI 发动态时的情绪只用于内部决定生成语气，**不进响应体**。前端拿不到，也就无从渲染成情绪角标。

**Comment**

```json
{ "id": 9, "momentId": 3, "personaId": 2, "userId": null, "authorName": "小星", "content": "我也喜欢热可可！", "createdAt": "2026-09-10T14:35:00+08:00" }
```

> `personaId` 与 `userId` **恰好一个非空**：AI 评论时 `personaId` 有值、`userId` 为 `null`；用户评论时相反。前端用 `authorName` 展示，不要自己判断。

| 端点 | 请求 | 响应 data | 错误码 |
|------|------|-----------|--------|
| `GET /moments` ⬜ | `page` `pageSize` | `PageResult<Moment>` | — |
| `POST /moments/:id/like` ⬜ | — | `{ "likeCount": 3, "liked": true }` | 4040 |
| `GET /moments/:id/comments` ⬜ | `page` `pageSize` | `PageResult<Comment>` | 4040 |
| `POST /moments/:id/comments` ⬜ | `{content}` | `Comment` | 4001 4040 |

> **可见范围是账号内**：只能看到自己创建的 AI 的动态。账号之间完全隔离，不存在跨用户的动态或评论。
> **没有 `POST /moments/generate`**：动态只由后台定时任务产生，不提供手动触发接口。演示依赖提前灌好的历史动态 + 调短的定时间隔（见技术文档 §5.5）。
> 用户**只能点赞和评论**，不能自己发布动态。
> **点赞是幂等的**：`moment_likes` 上有 `UNIQUE (user_id, moment_id)`，重复点击不会累加。本期**不支持取消点赞**（点第二次返回相同结果，不会减一）。

---

## 9. 主动消息（Proactive）

**Settings**

```json
{ "personaId": 1, "enabled": true, "intervalMin": 30, "intervalMax": 120, "dailyLimit": 3, "lastNudgeAt": null }
```

| 端点 | 请求 | 响应 data | 错误码 |
|------|------|-----------|--------|
| `GET /proactive/settings` ⬜ | `personaId` | `Settings` | 4031 4043 |
| `PUT /proactive/settings` ⬜ | `personaId` + 可改字段 | `Settings` | 4001 4031 4043 |
| `POST /proactive/trigger` 🚨 ⬜ | `{personaId}` | `{messageId, content, createdAt}` | 4031 4043 5001 |

**可改字段与约束**（前端三个控件，都要给）

| 字段 | 约束 | 前端控件 |
|------|------|----------|
| enabled | bool | 开关 |
| intervalMin | 5-1440，且 < intervalMax | 数字输入 |
| intervalMax | 5-1440，且 > intervalMin | 数字输入 |
| dailyLimit | 1-10 | 数字输入 |

> `lastNudgeAt` **只读**，`PUT` 时忽略。
> 🚨 `/proactive/trigger` **复用与定时任务完全相同的业务逻辑**，是答辩演示的硬性依赖，不是调试后门。朋友圈则没有对应的手动接口（见 §8）；日程提醒**有**（见 §10）。

---

## 10. 日程提醒（Schedules）· P1

> **P1**：排在 Week 2 生死线之后、Week 4 的余量里才做。砍功能顺序见 [总纲 §0.3](dev/MASTER.md#03-功能范围)。
> **不是第 4 个 Agent**：日程提醒复用主动消息的触发链路（注入 `[nudge]` → 走正常聊天链路），设计见 [技术文档 §5.7](TECH_DESIGN.md#57-日程提醒p1)。

**Schedule**

```json
{
  "id": 5,
  "personaId": 1,
  "content": "开会",
  "remindAt": "2026-09-11T15:00:00+08:00",
  "status": "pending",
  "createdAt": "2026-09-10T20:12:00+08:00"
}
```

| 字段 | 说明 |
|------|------|
| `content` | **用户原话里的事件描述**（"开会"），不是 AI 生成的提醒语 |
| `remindAt` | 绝对时间，RFC3339 带时区。规则解析出来的结果 |
| `status` | `pending` 待提醒 ｜ `sent` 已提醒 ｜ `cancelled` 已取消 |

| 端点 | 请求 | 响应 data | 错误码 |
|------|------|-----------|--------|
| `GET /schedules` ⬜ | `personaId`（**必带**）+ `page` `pageSize` | `PageResult<Schedule>` | 4001 4031 4043 |
| `DELETE /schedules/:id` ⬜ | — | `data: null` | 4031 4040 |
| `POST /schedules/:id/trigger` 🚨 ⬜ | — | `{messageId, content, createdAt}` | 4031 4040 5001 |

> **`personaId` 是必传的**：日程跟记忆一样是「一人设一份」，不带 `personaId` 无法确定查谁的日程。缺失返回 `4001`。
> **取消是软删除**：`DELETE` 把 `status` 置为 `cancelled`，不物理删行（保留可回溯）。重复删除同一条是幂等的。
> 🚨 `/schedules/:id/trigger` **复用与定时任务完全相同的业务逻辑**（与 §9 的 `/proactive/trigger` 同源），因为答辩现场不可能等到"明天下午三点"。
> **没有 `POST /schedules`**：日程**只能由对话中抽取产生**，不提供手工新建端点。用户在对话里说「明天三点提醒我开会」即可，不需要表单。

**未实现的兜底行为**（行为契约，不是错误码）：
> 用户在对话里表达了提醒意图，但**时间解析不出来**（如"过阵子提醒我"）时，AI **必须回问澄清**（"好的，但我没听准是哪天——你是说下周一吗？"），**不得静默丢弃**。
> **只有时间、没有提醒意图**的句子（如"我明天要开会"）**不建日程**——陈述句不是委托。

---

## 11. 健康检查

`GET /api/v1/health` ⬜ — 无需鉴权

```json
{ "status": "ok", "dependencies": { "database": "ok", "aiService": "ok" } }
```

供 Docker healthcheck 与部署核对使用。任一依赖异常时 `status` 为 `"degraded"`。

---

## 12. 变更记录

> 每条变更必须由改动者登记，并在群里广播。**默默改字段是团队协作中最容易引发返工的行为。**

| 日期 | 版本 | 变更内容 | 端点 | 改动人 | 已广播 |
|------|------|----------|------|--------|:------:|
| 2026-09-10 | v1 | 取消会话实体（一人设一对话）：删除 `ChatSession` 与 3 组会话接口；`ChatMessage.sessionId` → `personaId`；SSE 请求体 `sessionId` → `personaId`；`/proactive/trigger` 同上；废弃错误码 4042；`Persona` 新增 `lastMessageAt` | 见 §4 §5 §6 §9 | — | ⬜ |
| 2026-09-10 | v1 | 记忆与画像改为**一人设一份**：`/memory`、`/profile/portrait` 新增必传 `personaId`；`MemoryItem` 加 `personaId`；`memoryType` 去掉 `emotion` | §7 | — | ⬜ |
| 2026-09-10 | v1 | 新增 `PUT /user/password`（改密码）与新错误码 `4015`；`PUT /user/profile` 支持改 `username`（复用 4004）；`Persona` 新增只读 `familiarity` | §3 §4 | — | ⬜ |
| 2026-09-10 | v1 | 删除 `POST /moments/generate`：朋友圈动态只由定时任务产生；明确朋友圈**账号内可见**，用户只能点赞评论 | §8 | — | ⬜ |
| 2026-09-10 | v1 | **新增日程提醒（P1）**：`GET /schedules`（必带 `personaId`）、`DELETE /schedules/:id`（软删）、`POST /schedules/:id/trigger` 🚨；**无 `POST /schedules`**（只从对话抽取）；不新增错误码，复用 4001/4031/4040；**表数 9 → 10** | §10（新增，原 §10-12 顺延为 §11-13） | — | ⬜ |
| 2026-09-10 | v1 | 新增 `moment_likes` 表（`UNIQUE(user_id, moment_id)`）：点赞**幂等**，`POST /moments/:id/like` 返回改为 `{likeCount, liked}`，`Moment` 新增 `liked`。表数 8 → 9 | §8 | — | ⬜ |
| 2026-09-10 | v1 | `Moment` **移除 `emotionLabel`**：AI 发动态的情绪只用于内部生成语气，不进响应体（`ai_moments.emotion_label` 列保留）。与"情绪是内部信号"的全局约定对齐 | §8 | — | ⬜ |
| 2026-09-10 | v1 | 修正 `codeHTTPStatus` 漏登记 `4015`；`ChatMessage` TS 类型补 `isNudge`；SSE 前端类型补 `StreamChatPayload` / `DonePayload`，签名与成员 2 文档统一 | §5 §6 | — | ⬜ |
| 待填 | v1 | 初始冻结 | 全部 | — | — |

---

## 13. 冻结签署

- [ ] 成员 1 已确认全部端点与错误码（签名：______ 日期：______）
- [ ] 成员 2 已确认响应字段与分页结构，Mock 已按本文就位（签名：______ 日期：______）
- [ ] 成员 3 已确认人设 / 朋友圈 / 主动消息三组契约（签名：______ 日期：______）

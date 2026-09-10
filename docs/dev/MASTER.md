# HeartEcho 开发总纲

> 项目全貌：[项目概览](../../README.md)。
> 三份个人开发文档的**协调层**。分工边界、依赖交接、接口契约、集成顺序、止损决策都在这一份。
> 个人任务清单看自己那份：[成员 1](MEMBER_1_BACKEND_AI.md) ｜ [成员 2](MEMBER_2_FRONTEND.md) ｜ [成员 3](MEMBER_3_DATA_MOMENTS_DEPLOY.md)

**文档地图**

| 我想知道 | 看哪里 |
|----------|--------|
| 这个功能到底做不做、怎么做 | 本文 §0 功能决策记录 |
| 我自己这周干什么、怎么验收 | 我的个人开发文档 |
| 我要给别人什么东西、别人给我什么 | 本文 §3 交接清单 + 个人文档「依赖」节 |
| 接口字段长什么样 | [API_CONTRACT.md](../API_CONTRACT.md)（准备期冻结，字段以此为准） |
| 怎么把两个人的代码接起来 | 本文 §6 集成顺序 |
| 现在该不该砍功能 | 本文 §10 止损决策树 |
| 为什么这么设计 | [技术文档](../TECH_DESIGN.md) |
| Git / 提交 / 命名怎么做 | [团队协作文档](../COLLABORATION.md) |

---

## 0. 功能决策记录（已确认，不要私自改）

> 这一节是**产品决策的唯一记录**。下面每一条都经过明确确认，不是默认值也不是猜测。
> 如果你觉得哪条该改，**先在群里说，改完回来更新这一节**——不要直接改代码。

### 0.1 产品定位

| 项 | 结论 |
|----|------|
| 产品性质 | **个人产品**，不是社区、不是社交平台 |
| 账号关系 | 账号之间**完全隔离**。任何接口只能读到自己的数据，不存在跨用户的列表/动态/评论 |
| 跨用户 AI 互见 | **远期进阶选项，本期不做** |

### 0.2 数据归属

| 项 | 结论 |
|----|------|
| 人设与对话 | **一个人设 = 一个对话**，没有会话实体，不能新建/切换会话 |
| 对话历史 | 按人设隔离；**不可单独清空**，只能随人设一起删 |
| 记忆 | **一人设一份**（`user_memory.persona_id`）。换人设就换一套记忆 |
| 用户画像 | **一人设一份**（`user_profile.persona_id`），与记忆同粒度 |
| 记忆内容 | 只提取 `fact` / `preference` / `event` 三类，**不存情绪** |
| 记忆管理 | **只读展示**，不提供删除接口 |

### 0.3 功能范围

| 功能 | 结论 |
|------|------|
| 注册登录 | 用户名 + 密码；邮箱必填但本期**不做任何邮箱流程**；**注册成功即登录**，直接进对话页 |
| 头像 | **只填图片 URL**，不做文件上传；为空时前端用首字母色块 |
| 个人资料页 | 可改 **头像 URL + 用户名**（用户名唯一，复用 `4004`） |
| 修改密码 | **做**：登录态下 `PUT /user/password`，需原密码（`4015`） |
| 情感分析 | **内部信号，绝不展示**。效果靠 AI 回复策略的差异体现 |
| 人格演化 | 做**亲密度**一档（对话轮数驱动，注入 prompt）；不做"AI 改写自己人设" |
| 主动消息 | 配置**全部可调**（开关 + 间隔区间 + 每日上限）；消息出现在对话里 + 侧栏红点 |
| 日程提醒 | **P1，排在 Week 2 生死线之后**（Week 4 余量里做）。用户在对话里说「明天下午三点提醒我开会」，AI 记住并在到点后主动提醒。**不是第 4 个 Agent**，是主动消息管线的扩展（见 [技术文档 §5.7](../TECH_DESIGN.md#57-日程提醒p1)）。时间解析**只做规则档**，解析不出就**回问用户**，不静默丢弃 |
| 朋友圈 | 动态**只由定时任务产生，没有手动触发按钮**；用户**只能点赞和评论**，不能发动态；**只有自己的 AI 之间互动** |
| 重新生成回复 | **不做**（仅 SSE `error` 时要给重试提示） |
| 人设数量 | **不限制** |
| 空状态 | 没有任何人设时引导去创建 |
| 移动端 | 保证**主链路**（登录 → 对话 → 切人设）手机可用 |

### 0.4 技术范围

| 项 | 结论 |
|----|------|
| 阶段二 | **三项都做**（ONNX 情感模型 + ChromaDB 向量检索 + LLM 记忆提取），能推多少推多少，随时可回退 |
| 部署 | 国内云服务器 + **`http://公网IP` 直访**，不买域名、不做 HTTPS、**不备案** |
| P2 不做 | 语音对话、情绪趋势可视化、消息编辑撤回、主题切换 |

---


## 1. 三人边界总表

原则：**按功能模块切，不按技术栈切**。每人一条完整链路，前端 + 后端都有产出。

| 模块 | 成员 1 | 成员 2 | 成员 3 |
|------|:------:|:------:|:------:|
| Go 项目骨架 / 配置 / 日志 | ✅ | | |
| 统一响应 / 错误码 / JWT 中间件 | ✅ | | |
| 注册登录 API | ✅ | | |
| 全部 GORM 实体（`internal/model`） | | | ✅ |
| 数据库表设计与 AutoMigrate | | | ✅ |
| 用户 / 消息仓储层（`user_repo.go`、`message_repo.go`） | ✅ | | |
| 人设 CRUD（前后端，含 `persona_repo.go`） | | | ✅ |
| 流式对话后端（SSE） | ✅ | | |
| Python AI 服务（三 Agent） | ✅ | | |
| 前端骨架 / 路由 / 守卫 / Token 持久化 | | ✅ | |
| 登录注册页 / 聊天页 / 用户画像页 | | ✅ | |
| 人设管理页 / 朋友圈页 | | | ✅ |
| 主动消息（前后端 + 定时任务） | | | ✅ |
| 日程提醒（抽取与时间解析） | ✅ | | |
| 日程提醒（表 + 定时触发 + 前端列表） | | | ✅ |
| AI 朋友圈（前后端） | | | ✅ |
| Docker Compose / Nginx / 云部署 | | | ✅ |

**不 Own 的东西不要动**：发现队友文件里的 bug，先在群里说 / 提 Issue，由 Owner 改；紧急情况改了必须立刻告知（协作 §8.3）。

---

## 2. 依赖关系

```
                    准备期 (1-2d)
        ┌───────────────────────────────────────┐
        │ 三人共同：环境就绪 / 仓库可用 /        │
        │ 接口契约冻结 / DDL 冻结                │
        └───────────────────┬───────────────────┘
                            ▼
  成员 3 ──model/*.go──► 成员 1 ──pkg/{response,errcode,jwt}──► 成员 3
  (GORM 实体)           (框架与约定)                            (人设/朋友圈 handler)
                            │
                            └──pkg 约定 + 路由注册方式──► 成员 2
                                                          (前端全部)
        ▲                                                   │
        └────────── 成员 2 ⇄ 成员 3（双向）──────────────────┘
        成员 3 的人设接口 → 成员 2 写聊天/画像页消费它
        成员 2 的 request.ts / types/api.ts / 路由挂载点 → 成员 3 写人设与朋友圈页
```

**成员 1 是 Week 1 的瓶颈**——他产出的 `pkg/response`、`pkg/errcode`、`pkg/jwt`、路由注册方式、CORS 配置是另外两人的前置依赖。这些必须在 **Week 1 前半段冻结**，冻结后变更公共约定必须在群里广播（协作 §8.3）。

**成员 3 是 Week 1 的前置**——他产出的 `internal/model/*.go` 是成员 1 写 repository 的前置。所以 model 的字段定义要在**准备期就定死**（技术文档 §6.2 已有 DDL，成员 3 只是翻译成 Go struct）。

---

## 3. 准备期交接清单（P0，不完成不进 Week 1）

| # | 交接物 | 产出人 | 接收人 | 完成判据 |
|---|--------|--------|--------|----------|
| 1 | `internal/model/` 全部 10 个 GORM struct | 成员 3 | 成员 1、成员 3 | `go build ./...` 通过；字段与 §6.2 DDL 一一对应 |
| 2 | `pkg/response` + `pkg/errcode`（含 `errcode_test.go`） | 成员 1 | 全员 | 单测通过；`Fail()` 不接受 message 参数 |
| 3 | `pkg/jwt` + `internal/middleware/*` 空实现或可用实现 | 成员 1 | 成员 3 | 中间件链顺序与文档一致 |
| 4 | 路由注册方式约定（见 §4.5） | 成员 1 | 成员 2、成员 3 | 三人都能独立新增路由而不冲突 |
| 4b | `MOMENT_JOB_INTERVAL` / `PROACTIVE_JOB_INTERVAL` 写进 **`backend/.env.example`**（定时任务在 Go 侧）并在 `deploy/.env.example` + compose 中透传 | 成员 3 | 全员 | 本地开发时 Go 进程读的是 `backend/.env`，这两个变量**不在 ai-service 里**——放错服务会静默失效 |
| 4c | `SCHEDULE_PARSE_BACKEND` 写进 `ai-service/.env.example`，并在 `deploy/.env.example` + compose 中透传 | 成员 1（ai-service）/ 成员 3（deploy） | 全员 | 它是 P1 时间解析的开关键，属于 Python 侧 |
| 5 | [API_CONTRACT.md](../API_CONTRACT.md) 逐条确认并签署 | 三人共同 | — | 模板已就位，把每个端点的 `⬜` 改为 `✅`，三人在 §13 签字 |
| 6 | `deploy/docker-compose.dev.yml`（只起 PostgreSQL） | 成员 3 | 全员 | `docker compose up -d` 后 `psql` 可连 |
| 7 | 三份 `.env.example` + 各自本地 `.env` | 成员 1（backend/ai）/ 成员 2（frontend）/ 成员 3（deploy） | 全员 | 三服务本地能起来 |
| 8 | 前端 `types/api.ts` + `api/request.ts`（拦截器骨架） | 成员 2 | 成员 3 | 成员 3 能照着写 `api/persona.ts` |
| 9 | 三个 Dockerfile + `.dockerignore` + `docker compose build` 通过 | 成员 3 | 全员 | 部署问题不推迟到 Week 4 才暴露 |

> 契约模板已生成：[API_CONTRACT.md](../API_CONTRACT.md)。准备期三人逐条确认后签署，**改契约 = 群里广播 + 更新该文件「变更记录」**。本节的表是索引视图，字段细节以契约文件为准。

---

## 4. 接口契约冻结清单

### 4.1 全局约定

| 项 | 约定 |
|----|------|
| Base URL | `/api/v1` |
| 认证 | `Authorization: Bearer <access_token>` |
| 响应体 | `{ code, message, data, timestamp }` |
| JSON 字段 | **camelCase**（Go tag 与 TS interface 必须一致） |
| 时间 | RFC3339，如 `2026-09-10T14:30:00+08:00` |
| 分页 | `page`（从 1 开始）、`pageSize`（默认 20，上限 100） |
| 免鉴权白名单 | `POST /auth/register`、`POST /auth/login`、`POST /auth/refresh`、`GET /health`。**其余端点一律需要 `Authorization`**，由 `JWTAuth` 中间件拦截 |

**错误码分段**：200 成功 ｜ 4000-4009 参数/注册 ｜ 4010-4019 认证 ｜ 4030-4039 授权 ｜ 4040-4049 未找到 ｜ 5000-5009 服务端。完整表见 [技术文档 §7.4](../TECH_DESIGN.md#74-错误码总表)。**一个 code 严格对应一个 msg，`Fail()` 不接受自定义文案。**

### 4.2 端点归属表

| 端点 | 提供者 | 主要消费者 | 阶段 |
|------|:------:|:----------:|------|
| `POST /auth/register` | 成员 1 | 成员 2 | 一 |
| `POST /auth/login` | 成员 1 | 成员 2 | 一 |
| `POST /auth/refresh` | 成员 1 | 成员 2 | 一 |
| `GET/PUT /user/profile` | 成员 1 | 成员 2 | 一 |
| `PUT /user/password`（改密码，需原密码） | 成员 1 | 成员 2 | 一 |
| `GET/POST /personas`、`PUT/DELETE /personas/:id`（**同时就是对话列表**） | 成员 3 | 成员 2、成员 3 | 一 |
| `GET /chat/personas/:personaId/messages` | 成员 1 | 成员 2 | 一 |
| `POST /chat/stream`（SSE） | 成员 1 | 成员 2 | 一 |
| `GET /emotion/trend` | 成员 1 | — | **P2·不做**（仅预留路径）。**没有 `/emotion/diary`**：情绪标签不展示，连预留都不留 |
| `GET /memory`（**必带 `personaId`**） | 成员 1 | 成员 2 | 一 |
| `GET /profile/portrait`（**必带 `personaId`**） | 成员 1 | 成员 2 | 一 |
| `GET /moments`、`POST /moments/:id/like`、`GET/POST /moments/:id/comments` | 成员 3 | 成员 3 | 一 |
| `GET/PUT /proactive/settings` | 成员 3 | 成员 3 | 一 |
| `POST /proactive/trigger` 🚨 | 成员 3 | 成员 3 | 一 |
| `GET /schedules`（**必带 `personaId`**）、`DELETE /schedules/:id` | 成员 3 | 成员 3 | **P1** |
| `POST /schedules/:id/trigger` 🚨（演示用，与定时任务共用逻辑） | 成员 3 | 成员 3 | **P1** |
| `GET /health`（无需鉴权；本表路径均省略 §4.1 声明的 Base URL `/api/v1`） | 成员 1 | 成员 3（部署核对） | 一 |

> 🚨 `/proactive/trigger` 是**硬性要求**，不是调试后门：定时任务最短 30 分钟，答辩现场等不起。它必须复用与定时任务**完全相同**的业务逻辑。详见 [技术文档 §5.4](../TECH_DESIGN.md#54-主动消息实现)。
> **朋友圈没有对应的手动接口**（见 §0.3）：动态只由定时任务产生，靠提前灌数据 + 调短 `MOMENT_JOB_INTERVAL` 兜底。
> **日程提醒有手动接口**（P1）：`POST /schedules/:id/trigger` 🚨，与定时任务复用同一逻辑——理由和 `/proactive/trigger` 完全一样，现场等不到"明天下午三点"。

### 4.3 SSE 事件契约（成员 1 提供，成员 2 解析）

```
请求  POST /api/v1/chat/stream   { "personaId": 1, "content": "..." }

event: delta     data: {"text":"辛苦"}
event: delta     data: {"text":"了，今天发生"}
event: done      data: {"messageId":348}
event: error     data: {"code":5001,"message":"AI 回复生成失败，请稍后重试"}
```

响应头必须含 `Content-Type: text/event-stream`、`X-Accel-Buffering: no`。**事件类型不可私自增删**，要加先广播。

> **没有 `emotion` 事件**：情感分析结果写入消息表，并作为对话生成 Agent 的输入（决定共情策略）；**不下发独立事件**。历史消息里的 `emotionLabel` / `emotionScore` 只是回读字段，**界面一律不得渲染**。情感分析的效果通过 AI 回复策略的差异体现。

### 4.4 内存 / 部署契约

| 项 | 值 |
|----|-----|
| 前端 dev | `http://localhost:5173` |
| Go 后端 | `:8080` |
| Python AI 服务 | `:8000` |
| PostgreSQL | `:5432`（仅内网 / 本地） |
| 内部校验 | 后端调 AI 服务带 `AI_SERVICE_TOKEN`，两服务值必须一致 |
| 生产唯一对外端口 | Nginx `80` |

### 4.5 路由注册方式（成员 1 冻结，全员遵守）

每个模块在**自己的 handler 文件**里提供注册函数，`router.go` 只做汇总调用——避免三人同时改 `router.go` 冲突：

```go
// internal/handler/persona_handler.go（成员 3 的文件）
func RegisterPersonaRoutes(rg *gin.RouterGroup, h *PersonaHandler) {
    g := rg.Group("/personas")
    {
        g.GET("", h.List)
        g.POST("", h.Create)
        g.PUT("/:id", h.Update)
        g.DELETE("/:id", h.Delete)
    }
}
```

```go
// internal/handler/router.go（成员 1 的文件，只加一行）
api := r.Group("/api/v1")
RegisterAuthRoutes(api, authHandler)
RegisterChatRoutes(api, chatHandler)
RegisterPersonaRoutes(api, personaHandler)   // ← 成员 3 通知成员 1 加这一行
```

**约定**：新增模块时，Owner 写好 `RegisterXxxRoutes` 后**在群里说一声**，由成员 1 在 `router.go` 加一行（或 Owner 自行加，两人错开时间即可，不要同时改这个文件）。

---

## 5. 环境变量总表

完整清单见 [技术文档 §4.6](../TECH_DESIGN.md#46-环境变量清单)。三人必须知道的关键项：

| 变量 | 属于 | 谁维护 | 注意 |
|------|------|--------|------|
| `DB_*` | backend | 成员 1 | 本地指向 `localhost:5432`，容器内指向 `postgres` |
| `JWT_SECRET` | backend | 成员 1 | ≥32 字符，各环境不同，**禁止用示例值上线** |
| `AI_SERVICE_URL` | backend | 成员 1 | 本地 `http://localhost:8000`，容器内 `http://ai-service:8000` |
| `AI_SERVICE_TOKEN` | backend + ai-service | 成员 1 | **两边值必须一致**，不一致表现为 401 |
| `DEEPSEEK_API_KEY` | ai-service | 成员 1 提供 | 找成员 1 要，**绝不入库** |
| `EMOTION_BACKEND` / `MEMORY_BACKEND` / `MEMORY_EXTRACT_BACKEND` / `SCHEDULE_PARSE_BACKEND` | ai-service | 成员 1 | 阶段一/阶段二切换开关 |
| `MOMENT_JOB_INTERVAL` / `PROACTIVE_JOB_INTERVAL` | **backend** | 成员 3 | **定时任务跑在 Go 侧**；朋友圈没有手动触发接口，`MOMENT_JOB_INTERVAL` 是现场唯一兜底旋钮 |
| `VITE_API_BASE_URL` | frontend | 成员 2 | 本地 `http://localhost:8080/api/v1`；生产改同源 `/api/v1` |
| `POSTGRES_*` | deploy | 成员 3 | 生产用强密码；`.env` 权限 `600` |

> 🔴 **`.env` 永不入库**。仓库里只有 `.env.example`。一旦误提交，立刻轮换密钥并清理历史（协作 §10.4 红线 1）。

---

## 6. 集成顺序（按这个顺序联调，不要跳）

联调不是「大家写完了合起来跑」，而是**每接通一层就验证一层**。跨 Go / Python / Nginx / 浏览器四段的 SSE 链路尤其如此。

| 步 | 接通什么 | 谁做 | 验证命令 / 动作 | 通过判据 |
|----|----------|:----:|------------------|----------|
| 1 | PostgreSQL | 成员 3 | `docker compose -f deploy/docker-compose.dev.yml up -d` → `psql` 连接 | 能建表、能查 |
| 2 | Go 后端 ↔ DB | 成员 1 | `go run ./cmd/server`，调 `GET /api/v1/health` | 返回 `{"code":200,...}` |
| 3 | 认证链路 | 成员 1 → 成员 2 | `curl -X POST .../auth/register` → 登录页实测 | 前端能拿到 token，刷新页面仍登录 |
| 4 | Python AI 服务独立可用 | 成员 1 | `curl -N -X POST localhost:8000/chat/stream ...` | **直接看到 `data:` 逐条流出**（不经过 Go/Nginx） |
| 5 | Go ↔ Python | 成员 1 | 通过 Go 的 `/chat/stream` 用 `curl -N` | 同样能流式 |
| 6 | 前端 ↔ Go SSE | 成员 1 → 成员 2 | 浏览器 DevTools Network 看 EventStream | 打字机逐字出现，`delta` / `done` 逐条到达 |
| 7 | 人设链路 | 成员 3 → 成员 2 | 创建人设 → 对话 | 不同人设对话记录隔离 |
| 8 | 记忆往返 | 成员 1 → 成员 2 | 对人设 A 说「我养了只猫叫豆豆」→ 人设 A 问「我养了什么」答得出；切到人设 B 问 → 答不出（记忆按人设隔离） | 两边都对 |
| 9 | 主动消息 | 成员 3 | 点手动触发按钮 | 消息基于上下文，非模板 |
| 10 | 朋友圈 | 成员 3 | 打开朋友圈页（数据已提前灌好） | 动态 + 同账号其他 AI 评论 |
| 11 | 生产部署 | 成员 3 | 浏览器走完整演示脚本 | 手机 4G 可访问 |
| 12（**P1，可不做**） | 日程提醒 | 成员 1 → 成员 3 | 对人设说「1 分钟后提醒我喝水」→ 等 1 分钟（或点 `/schedules/:id/trigger`）→ 提醒作为消息出现在对话里 | 到点触发；解析不出时会回问而不是没反应 |

**第 4 步是最容易被跳过、也最不该跳过的一步。** 先用 `curl -N` 直连 Python 服务确认它本身能流式，再逐层往上加。一次性全接通再调，你无法判断是 Python、Go、Nginx 还是浏览器解析的问题。

---

## 7. 周检查点

| 节点 | 检查内容 | 不通过怎么办 |
|------|----------|-------------|
| 准备期末 | 环境就绪；契约冻结；仓库可用；model 已交付 | **不要带着未就绪的环境进 Week 1** |
| Week 1 末 | 注册 → 登录 → 刷新保持登录态，前后端联调成功 | Week 2 前 2 天继续补，同时压缩人设功能 |
| **Week 2 末** | **流式对话跑通（生死线）** | **立即砍掉记忆系统与主动消息**，Week 3-4 只保 1-4 项 + 部署 |
| Week 3 末 | 核心演示链路全通 | 砍 P1，全力保部署 |
| Week 4 主线完成 | 朋友圈 + 部署 + 阶段二做完 | **日程提醒不开工**（它排在 P1 队尾，没余量就不做） |
| Week 4 末 | 公网可访问 + 演示预演通过（≥2 次） | 启用备用演示录屏 |

**每日动作**（协作 §8.1）：开工在群里发 3 条（昨天做了 / 今天做什么 / 有没有卡住）；产出即 commit + push；阻塞超 30 分钟立刻说；收工确认已 push。

**每周动作**（协作 §8.2）：周一对齐目标；周三中期检查是否要砍功能；周日合并全部 feature 到 `develop`，跑通完整链路。

---

## 8. 阶段一 → 阶段二 切换清单

阶段一保证核心链路先跑通，阶段二在同一套接口下替换实现。**调用方一行不改**（技术文档 [§5.0](../TECH_DESIGN.md#50-实现分级与升级路径核心设计)）。

| 升级项 | 谁做 | 改什么文件 | 切换动作 | 前置条件 |
|--------|:----:|-----------|----------|----------|
| LLM 结构化记忆提取 | 成员 1 | `ai-service/app/agents/memory_agent.py` | `MEMORY_EXTRACT_BACKEND=llm` | 规则提取已跑通 + API 余额充足 |
| ChromaDB 向量检索 | 成员 1 + 成员 3 | `ai-service/app/memory/vector.py` + compose 启用 chromadb | `MEMORY_BACKEND=vector` | 记忆表已有真实数据；**metadata 存 `user_id` + `persona_id`，检索两个都要过滤** |
| ONNX 情感模型 | 成员 1 | `ai-service/app/classifiers/onnx.py` | `EMOTION_BACKEND=onnx` | 先单独跑通一次推理再集成 |
| 朋友圈 Agent 自主评论 | 成员 3 | 评论策略模块 | 服务内策略切换 | 朋友圈基础功能已上线 |
| 日程时间解析上 LLM | 成员 1 | `ai-service/app/schedules/llm_parser.py` | `SCHEDULE_PARSE_BACKEND=llm` | 规则档已跑通；能处理「下周三」这类相对表达（**P1 可选，做不动就保持规则档**） |

**推进纪律**：动手前确认当前版本已提交、合并、可演示；**每替换一项，当天收工前必须保证服务可运行**；遇阻塞立刻切回阶段一实现（改环境变量即可），继续推进其他项。

---

## 9. 部署分工

| 环节 | 负责人 | 协作 |
|------|:------:|------|
| 云服务器采购 / 安全组（只开 22/80） | 成员 3 | 三人确认厂商与规格 |
| 生产 `.env`（强密码，权限 600，永不入库） | 成员 3 | 成员 1 提供 `JWT_SECRET` / `DEEPSEEK_API_KEY` |
| `deploy/docker-compose.yml` | 成员 3 | 成员 1 确认 backend 环境变量名 |
| `deploy/Dockerfile`（前端 dist + Nginx 同镜像）、`backend/Dockerfile`、`ai-service/Dockerfile` | 成员 3 | 全员确认本地 `docker compose build` 通过 |
| `deploy/nginx.conf`（SSE 关缓冲 + history 模式） | 成员 3 | 成员 1 验证 `/api` 转发 |
| 构建产物 / 部署脚本验证 | 成员 3 | 全员跑演示脚本 |
| 演示账号 / 人设 / 历史消息准备 | 成员 2 | 成员 3 准备服务器环境 |
| 演示用的朋友圈历史动态灌数据 | 成员 3 | 成员 2 确认页面显示正常 |
| README / docs 维护 | 成员 1（总纲作者） | 三人都可提 Issue |
| Commit 规范抽查（周检查点） | 成员 1（约定冻结者） | 违反的当场在群里点名 |

部署检查清单见 [技术文档 §9.5](../TECH_DESIGN.md#95-部署流程github-actions-可选自动化)。

---

## 10. 止损决策树

**时间不够时，从上往下砍**（协作 §10.2）：

```
① 语音对话（本就不做，P2）
② 情绪趋势可视化（P2）
③ 人格演化（P1）
④ 用户画像页（P1）
⑤ 日程提醒（P1，Week 4 余量里才开工；没余量就不开工）
⑥ AI 朋友圈（P1）
⑦ 主动消息 / 记忆系统（仅当 Week 2 生死线未过才砍）
🚫 绝不砍：登录注册 / 人设 / 流式对话 / 情感分析
```

**核心演示链路**（跑通即成立）：

```
登录 → 创建人设 → 对话（流式 + 情感感知）→ 记住信息 → 主动消息 → 朋友圈互动
（P1 有余量再加一步：在对话里说「明天下午三点提醒我开会」→ 日程列表出现该条 → 点手动触发看到提醒）
```

**团队红线**（详见协作 §10.4）：

1. ❌ 提交 `.env` / API Key / 密码到仓库
2. ❌ 直接 push 到 `main` / `develop`
3. ❌ force push 共享分支
4. ❌ 跨用户 / 跨人设数据泄漏（记忆与画像查询不带 `user_id` 会跨用户，不带 `persona_id` 会串人设）
5. ❌ 明文 / 弱哈希存密码
6. ❌ 连续 3 天无提交且不沟通
7. ❌ 最后一周突击提交

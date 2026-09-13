# AGENTS.md

> 给 AI 编码代理（Claude Code / Cursor 等）的项目工作指南。
> 人类协作者的完整规范见 [docs/COLLABORATION.md](docs/COLLABORATION.md)；本文件是代理执行任务时的**速查与硬约束**。

## 0. 项目一句话

HeartEcho（心回响）：前后端分离的 AI 情感陪伴 Web 应用。Vue3 + TS 前端 → Go/Gin 业务后端 → Python FastAPI AI 服务 → PostgreSQL。核心是"像真人一样"的 AI 伴侣：有记忆、有情绪（内部信号）、能主动发消息、能发朋友圈。

**当前状态**：代码骨架正在逐步创建中，以仓库实际文件为准。任务若涉及新建代码，按 §3 的目标目录结构放置。

## 1. 权威文档与决策顺序

改动前先查，不要凭猜测实现：

| 想知道 | 看 |
|--------|-----|
| 某功能做不做、语义边界 | [docs/dev/MASTER.md](docs/dev/MASTER.md) §0 功能决策记录（**最高权威，不要私自改**） |
| 接口字段 / 错误码 / SSE 事件 | [docs/API_CONTRACT.md](docs/API_CONTRACT.md)（已冻结，字段以此为准） |
| 详细设计 / DDL / 部署 | [docs/TECH_DESIGN.md](docs/TECH_DESIGN.md) |
| Git / 命名 / Review | [docs/COLLABORATION.md](docs/COLLABORATION.md) |
| 项目全貌 / 排期 / 演示脚本 | [README.md](README.md) |

文档与代码冲突时：**契约 > 技术文档 > 代码现状**。改契约字段 = 群里广播 + 更新 `API_CONTRACT.md` 变更记录。

## 2. 常用命令

### 环境要求

Node ≥18（建议 20）、Go ≥1.21、Python ≥3.10、Docker Desktop、PostgreSQL 16（走 Docker，不装本机）。

### 首次准备

```bash
cp backend/.env.example backend/.env
cp ai-service/.env.example ai-service/.env
cp frontend/.env.example frontend/.env
cp deploy/.env.example deploy/.env
# DEEPSEEK_API_KEY 找成员 1 要，填进 ai-service/.env
docker compose -f deploy/docker-compose.dev.yml up -d   # 只起 PostgreSQL
```

### 启动三个服务（各开一个终端）

```bash
# Go 后端 :8080
cd backend && go run ./cmd/server

# Python AI 服务 :8000
cd ai-service && uvicorn app.main:app --reload --port 8000

# Vue 前端 :5173
cd frontend && npm install && npm run dev
```

浏览器访问 http://localhost:5173。

### 自测 / 构建（提交前必跑）

```bash
# Go：编译 + 全量测试（vet 建议一并跑）
cd backend && go build ./... && go test ./... && go vet ./...

# errcode 包单测（校验三个 map 的 key 集合与常量集合一致）
cd backend && go test ./pkg/errcode/

# 前端：类型零错误 + 构建
cd frontend && npx tsc --noEmit && npm run build

# AI 服务
cd ai-service && pytest

# 健康检查
curl localhost:8080/api/v1/health
```

### 分段验证 SSE（**联调必备**，见 §4.5）

```bash
# 先直连 Python 确认它自己能流式
curl -N -X POST http://localhost:8000/chat/stream -H 'Content-Type: application/json' \
  -d '{"personaId":1,"content":"你好"}'
# 再经 Go
curl -N -X POST http://localhost:8080/api/v1/chat/stream \
  -H 'Authorization: Bearer <token>' -H 'Content-Type: application/json' \
  -d '{"personaId":1,"content":"你好"}'
```

## 3. 目标目录结构

```
backend/     Go 后端：cmd/server、internal/{handler,service,repository,model,middleware,dto,config}、pkg/{response,errcode,jwt,logger}
ai-service/  Python：app/{agents,classifiers,memory,schedules,core,schemas,api}
frontend/    Vue3：src/{api,components,layouts,router,stores,types,utils,views}
deploy/      Dockerfile、docker-compose*.yml、nginx.conf、.env.example
docs/        TECH_DESIGN / COLLABORATION / API_CONTRACT / KICKOFF / dev/*
```

### 新代码放哪里

**后端**：接口 → `internal/handler/xxx_handler.go`；业务逻辑（事务/多表） → `internal/service/xxx_service.go`；单次查询 → `internal/repository/xxx_repo.go`；表实体 → `internal/model/xxx.go`；请求响应结构 → `internal/dto/xxx_dto.go`；跨项目可复用工具 → `pkg/xxx/`；中间件 → `internal/middleware/xxx.go`。

**前端**：页面 → `src/views/<模块>/XxxView.vue`；可复用组件 → `src/components/<模块>/Xxx.vue`；请求函数 → `src/api/<模块>.ts`；全局状态 → `src/stores/<模块>.ts`；类型 → `src/types/<模块>.ts`。

**纪律**：一个文件只做一件事；不新建 `utils.ts` / `common.ts` / `misc.go` 这类杂物间；组件不直接写 axios，必须走 `src/api/`；handler 不直接调 repository，必须走 service；新增目录前先在群里说一声。

## 4. 必须遵守的架构约定

### 4.1 统一响应与错误码（硬性要求）

- 响应体固定 `{ code, message, data, timestamp }`；HTTP 状态码与业务 code 分离。
- `response.Fail()` **只接受 `errcode.ErrorCode`，不接受自定义 message**——保证"一 code 一 msg"。
- 新增错误码：在 `pkg/errcode/errcode.go` 按段位分配常量，**同时**登记 `codeMessages` 与 `codeHTTPStatus` 两个 map，并跑 `errcode_test.go`（校验三者 key 集合一致）。
- `codeHTTPStatus` 漏登记会走兜底分支返回 HTTP 500（已知坑：`4015` 漏登记会让改密码失败变 500），所以测试必须校验三个集合一致。
- 段位：200 成功 ｜ 4000-4009 参数/注册 ｜ 4010-4019 认证 ｜ 4030-4039 授权 ｜ 4040-4049 未找到 ｜ 5000-5009 服务端。
- handler **不写** `response.Fail`，错误用 `_ = c.Error(err)` 上抛，由 `BizErrorHandler` 中间件统一出口。service 层返回 `errcode.New/Wrap`。
- panic 堆栈、原始 error 只进日志，**不返回前端**。登录失败统一 `4013`，不暴露"用户不存在"。

### 4.2 中间件链顺序（不可乱）

```
Recovery → RequestLogger → CORS → BizErrorHandler → JWTAuth → Handler
```

- 免鉴权白名单只有 4 个：`POST /auth/register`、`POST /auth/login`、`POST /auth/refresh`、`GET /health`（完整路径均带 Base URL `/api/v1`，见 [契约 §1](docs/API_CONTRACT.md)）。其余一律要 `Authorization: Bearer <accessToken>`。
- refresh token 不能用于访问业务接口（校验 `typ`）。
- CORS 只用于本地开发；生产走 Nginx 同源代理。

### 4.3 数据边界（最重要的正确性约束）

- **账号之间完全隔离**。任何接口只能读当前 Token 对应用户自己的数据，不存在跨用户列表/动态/评论。
- **一个人设 = 一个对话**，没有会话（session）实体。消息直接挂 `persona_id`；对话列表复用 `GET /personas`（按 `lastMessageAt` 倒序）；删人设级联删消息。
- **记忆与画像都是一人设一份**。查询必须同时带 `persona_id`（隔离人设）和 `user_id`（越权防线，从 Token 取，前端永不传）。只带 `persona_id` 会跨用户串号，只带 `user_id` 会跨人设。
- 涉及 `:id` 的人设操作必须校验归属，**失败按「人设不存在」返回 `4043`**——不返回 403：403 会暴露该 `persona_id` 存在，而它是全局自增的，可被顺序试号探测出系统内人设总数。`4030` 只用于**功能越权**（封禁用户、无权限的功能）。
- 阶段二 ChromaDB 检索同样两个过滤条件都要带（`persona_id` 是全局自增，别的用户同号人设会串号）。
- 外键统一 `ON DELETE CASCADE`（`source_message_id` 用 `SET NULL`）。
- 记忆只提取 `fact` / `preference` / `event` 三类，**不存情绪**；记忆**只读**，不提供删除接口。
- 密码必须 bcrypt（cost ≥ 10），严禁明文/MD5；密码限 8-32 位 **ASCII 可见字符**（bcrypt 只取前 72 字节，中文多字节会被静默截断）。
- JWT secret ≥32 字符、各环境不同、示例值禁止上线。
- `AI_SERVICE_TOKEN` 在 backend 与 ai-service **两边值必须一致**，不一致表现为内部调用 401。

### 4.4 情绪是内部信号，绝不展示

- 用户消息与 AI 回复**双向**做情感分析。8 类情绪：`joy` `sadness` `anger` `fear` `surprise` `disgust` `neutral` `love`。
- 情绪用于：写入消息表、驱动回复策略、供记忆提取打分。**界面一律不得渲染**，SSE **没有 `emotion` 事件**，`ai_moments.emotion_label` 不进响应体。
- 历史消息里的 `emotionLabel` / `emotionScore` 只是回读字段（供聚合），前端拿到了也不展示。
- 演示情感分析的方式是**两次回复策略的差异**（悲伤先共情不给建议，开心更活泼），不是展示标签。
- 没有 `/emotion/diary`，连预留路径都不留；`/emotion/trend` 是 P2 预留不实现。

### 4.5 SSE 契约（跨 Go / Python / Nginx / 浏览器四段）

- 事件类型固定三种，**不可私自增删**：`delta {text}`、`done {messageId}`、`error {code,message}`（HTTP 状态仍为 200）。
- 响应头必须含 `Content-Type: text/event-stream`、`X-Accel-Buffering: no`；Nginx 侧 `proxy_buffering off`。
- 前端不能用原生 `EventSource`（只支持 GET、不能带 Header），必须 `fetch` + `ReadableStream` 手动解析，按 `\n\n` 切分、残片留 buffer。
- **联调必须分段**：先 `curl -N` 直连 Python 确认流式 → 经 Go → 经 Nginx → 浏览器。不要一次性全接通再调。
- 前端必须处理 `error` 事件，不能静默。

### 4.6 三 Agent，且没有第 4 个

`ai-service/app/agents/` 下三个职责分离的模块（不是三个独立服务）：情感分析 Agent、对话生成 Agent、记忆管理 Agent。

- **日程提醒不是 Agent**，是主动消息管线的复用（到点注入 `[nudge]`）。解析器放 `app/schedules/`，**不要放 `agents/`**，也不要单开服务。同理 `classifiers/`、`memory/` 是可替换实现，不是 Agent。
- **朋友圈没有手动触发接口**，动态只由定时任务产生；演示靠提前灌数据 + 调短 `MOMENT_JOB_INTERVAL`。
- **`/proactive/trigger` 和 `/schedules/:id/trigger` 是硬性演示依赖**：必须复用定时任务的同一段 `TriggerNow` 逻辑，不是演示专用代码。

### 4.7 阶段一 / 阶段二，接口不变只换实现

阶段一先跑通（词典情感分析、PG 记忆检索、规则提取），阶段二在同一 HTTP 契约下替换（ONNX、ChromaDB、LLM 提取）。多数切换只改环境变量：

| 变量 | 归属服务 | 值 |
|------|---------|-----|
| `EMOTION_BACKEND` | ai-service | `lexicon` / `onnx` |
| `MEMORY_BACKEND` | ai-service | `recent` / `vector` |
| `MEMORY_EXTRACT_BACKEND` | ai-service | `rule` / `llm` |
| `SCHEDULE_PARSE_BACKEND` | ai-service | `rule` / `llm`（P1） |
| `MOMENT_JOB_INTERVAL` | **backend** | 定时任务在 Go 侧 |
| `PROACTIVE_JOB_INTERVAL` | **backend** | 主动消息 + 日程到期扫描共用 |

> 例外（不是只改变量）：`MEMORY_BACKEND=vector` 还需在 compose 启用 chromadb 容器；`EMOTION_BACKEND=onnx` 还需下载模型文件并配 `EMOTION_MODEL_PATH`（`*.onnx` 不入库）。

**升级纪律**：每替换一项，当天收工前必须保证服务可运行、可演示；遇阻塞立即切回阶段一环境变量。

### 4.8 定时任务与路由注册

- **所有定时任务跑在 Go 侧**（`robfig/cron`），不用 Python APScheduler。`MOMENT_JOB_INTERVAL` / `PROACTIVE_JOB_INTERVAL` 写在 `backend/.env.example`，放错到 ai-service 会静默失效。
- 路由注册：每个模块在自己 handler 文件里提供 `RegisterXxxRoutes(rg, h)`，`router.go` 只加一行汇总调用。**不要多人同时改 `router.go`**。

### 4.9 前端接口调用约定

- `src/api/request.ts` 在 `code === 200` 时**直接返回 `data` 部分**：组件里拿到的就是契约里的「响应 data」，**不要在组件里判断 `res.code` / `res.data`**。
- Mock 层与真实实现返回同样的 `data` 结构，靠 `VITE_USE_MOCK` 切换。
- 错误处理：`4012` → 刷新 token 后重放原请求；`4010` / `4011` / `4014` → 清空登录态跳登录页（**不重试**）；其余按 `message` 提示。
- 注册接口返回 token = **注册成功即登录**，直接进对话页，不要再跳登录页。
- 组件必须处理 loading / empty / error 三种状态。

### 4.10 部署要点

- `deploy/docker-compose.yml` 的构建上下文是**仓库根目录**（需访问 `frontend/`、`deploy/`）。
- `frontend/package-lock.json` **必须提交**——`deploy/Dockerfile` 用 `npm ci`，缺了构建直接失败。
- `VITE_API_BASE_URL` 在**构建时**注入，生产为同源 `/api/v1`（**不是** `localhost:8080`）；运行期改 `.env` 无效。
- 后端容器必须监听 `0.0.0.0`（不是 `127.0.0.1`），否则容器间连接被拒。
- 生产只暴露 Nginx `80`；PostgreSQL / ChromaDB / AI 服务端口只在 Docker 内网，不对外。
- 根目录 `.dockerignore` 必须排除 `.env`，避免密钥打进镜像。

## 5. 命名与代码规范

- **禁止拼音命名**；禁止 `any`（用 `unknown` + 类型收窄）；`strict: true` 必须零错误。
- 布尔值 `is/has/can/should` 开头；函数动词开头。
- Go：导出 PascalCase / 非导出 camelCase，缩写词全大写或全小写（`userID` 不是 `userId`），包名全小写单词，文件名小写下划线，接口 `-er` 后缀。
- TS：变量/函数 camelCase，常量 UPPER_SNAKE_CASE，类型/接口/组件 PascalCase，非组件文件 camelCase，CSS kebab-case。
- Python：变量/函数 snake_case，类 PascalCase。
- 数据库：表名小写下划线复数（例外 `user_memory` / `user_profile` 不可数用单数），字段小写下划线。
- URL：kebab-case 复数名词，路径参数 camelCase 且与字段同名（`:personaId`）。
- **JSON 字段一律 camelCase，前后端必须逐字一致**——这是最容易出 bug 的地方。前端 TS interface 字段用 camelCase，不要下划线。

## 6. Git 规范

- 分支：`feature/xxx`、`fix/xxx`、`hotfix/xxx`，全小写连字符，禁止中文/空格。**禁止直接 push `main` / `develop`**，一律走 PR（至少 1 人 Approve）。
- Commit 格式 `<type>(<scope>): <subject>`：subject ≤50 字符、祈使句、首字母小写、结尾无句号、第二行空行、body 每行 ≤72 字符、**统一英文**。
- type：`feat` `fix` `docs` `style` `refactor` `test` `chore`（+ `perf` `ci`）。
- scope 取值：`auth` `user` `model` `persona` `chat` `memory` `emotion` `moment` `proactive` `schedule` `deploy` `errcode`。
- **产出即提交**：写完一个函数/修完一个 bug/调通一个接口立刻 commit，一次只做一件事。禁止 `WIP`、禁止"今天的活"、禁止最后一周突击。
- 功能分支存活不超过 3 天；绝不 force push 共享分支。

## 7. 红线（不可触碰）

1. ❌ 提交 `.env`、API Key、密码、模型权重（`*.onnx` 等）到仓库——仓库只留 `.env.example`。
2. ❌ 直接 push `main` / `develop`，或 force push 共享分支。
3. ❌ 跨用户 / 跨人设数据泄漏（记忆与画像查询缺 `persona_id` 或 `user_id`）。
4. ❌ 明文 / 弱哈希存密码（必须 bcrypt）。
5. ❌ 给情绪加展示标签、给朋友圈加手动触发接口、给日程开 `POST /schedules` 新建端点——这些都违反 [MASTER §0](docs/dev/MASTER.md) 的功能决策与 [API_CONTRACT](docs/API_CONTRACT.md) 契约。
6. ❌ 硬编码错误码或错误文案；`Fail()` 传自定义 message。
7. ❌ 让 handler 直接操作 DB、让 service 依赖 `*gin.Context`、前端组件直接写 axios。
8. ❌ 生产环境执行 `DropTable` 等破坏性 DB 操作；禁止手写 `ALTER TABLE`（改 GORM struct + AutoMigrate）。

## 8. 止损与优先级（时间不够时）

砍功能顺序（从上往下）：语音对话 → 情绪趋势可视化 → 人格演化 → 用户画像页 → 日程提醒（P1 队尾，Week 4 主线没完就不开工）→ AI 朋友圈 → 主动消息/记忆系统（仅当 Week 2 生死线未过才砍）。

**绝不砍**：登录注册 / 人设 / 流式对话 / 情感分析。

**核心演示链路**：登录 → 创建人设 → 流式对话（情感感知）→ AI 记住信息 → 主动消息 → 朋友圈互动（P1 有余量再加日程提醒）。

## 9. CI 约定

CI 配置在 `.github/workflows/ci.yml`。每个顶层服务目录对应一个 job，job 的 `working-directory` 必须指向真实存在的目录。

改动目录结构时，同一次提交里同步更新 `ci.yml`：
- 新增顶层服务目录 → 新增对应 job
- 删除或重命名 → 同步修改 job
- `deploy/`、`docs/` 等非代码目录不纳入 CI

不确定某个目录是否该纳入 CI，问组长，不要自己判断。

AI 辅助时，若发现目录结构与 `ci.yml` 不一致，**提示我，不要自动修改**。
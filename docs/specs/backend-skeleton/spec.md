# spec · 后端骨架搭建

> 功能名：backend-skeleton ｜ 分支：`feature/backend-skeleton`
> 负责人：成员 1 ｜ 状态：草稿（待审核）
> 创建：2026-09-13 ｜ 最后更新：2026-09-13
> 关联：[开发总纲 §3 准备期交接清单](../../dev/MASTER.md#3-准备期交接清单p0不完成不进-week-1) ｜ [成员 1 开发文档 §1](../../dev/MEMBER_1_BACKEND_AI.md) ｜ [接口契约](../../API_CONTRACT.md) ｜ [技术文档 §4 / §8](../../TECH_DESIGN.md)

---

## 1. 背景与目标

成员 2 的 [前端骨架 spec §3.1](../frontend-skeleton/spec.md) 里，依赖我的三项状态均为「待确认」：统一响应结构、错误码表、SSE 事件契约。成员 3 要写 `persona_handler.go`，需要一个不会三人同时改的 `router.go` 挂载点。

按 [开发总纲 §2](../../dev/MASTER.md#2-依赖关系)，成员 1 是 Week 1 的瓶颈。本功能**不产出业务接口**（`/health` 除外），交付的是"别人能往上写代码"的工程约定。

**目标（一句话）**：让另两位队友拿到仓库后，能直接新增路由、写 handler、按统一格式返回响应与错误，不需要再问任何结构问题。

## 2. 范围

### 2.1 做什么（In Scope）

| 项 | 产物 |
|---|---|
| 目录骨架 | `backend/{cmd/server,internal/{config,middleware,dto,handler,service,repository},pkg}` |
| 配置加载 | `internal/config/config.go`（`caarlos0/env`） |
| 日志 | `pkg/logger/logger.go`（zap 封装，技术文档 §4.4 已定） |
| **统一响应** | `pkg/response/response.go`（`Success[T]` / `Fail(c, code)`） |
| **错误码** | `pkg/errcode/errcode.go` + `errcode_test.go`（覆盖技术文档 §7.4 全表） |
| JWT 基础包 | `pkg/jwt/jwt.go`（Access 2h / Refresh 7d，含 `TokenType`） |
| 中间件 | `internal/middleware/{recovery,logger,cors,biz_error,jwt}.go`，4 个可用 + JWTAuth 空壳 |
| 路由与数据库装配 | `internal/handler/router.go`、`cmd/server/main.go`（含 GORM 初始化） |
| 健康检查 | `GET /api/v1/health` |
| 环境变量模板 | `backend/.env.example` |

### 2.2 不做什么（Out of Scope）

| 不做的事 | 归属 |
|---|---|
| 注册 / 登录 / 刷新令牌 / 改密码、`GET/PUT /user/profile` | `feature/auth-login` |
| **JWTAuth 中间件的真实校验逻辑** | `feature/auth-login`（见 §2.3） |
| `internal/model/*.go` 全部 GORM 实体 + `migrate.go` | 成员 3 |
| 人设 / 朋友圈 / 主动消息 / 日程的一切 handler 与 service | 成员 3 |
| SSE 流式对话、`ai_client.go`、记忆与画像接口 | 后续 spec |
| `ai-service/` 骨架**及其 `.env.example`**；本轮**不建空目录** | 后续 spec |
| 前端一切 | 成员 2 |
| Dockerfile / docker-compose / Nginx | 成员 3 |

> 本功能范围内不出现业务 handler，`handler/` 目录只放 `router.go`。

### 2.3 一个刻意的不对称：JWTAuth 是空壳

| 中间件 | 本轮状态 | 原因 |
|---|---|---|
| Recovery / RequestLogger / CORS | 可用 | 纯框架能力，不依赖业务语义 |
| **BizErrorHandler** | 可用 | 别人写 handler 立刻要用——它是"一 code 一 msg"的唯一出口，没有它队友的错误无法上抛 |
| **JWTAuth** | **空壳**（占位 + 链上位置固定） | 真实语义要等 `pkg/jwt` 的 claims 结构与登录接口一起定；且**空壳状态下队友也测不了**他手里没有 token |

`pkg/jwt` 本身属于本轮（准备期交接清单 #3 原文即 "`pkg/jwt` + `internal/middleware/*`"），`auth-login` 只消费它。

> ⚠️ 空壳 = 放行 = 安全口子。**关闭它是 `auth-login` 的第一条验收项**。本 spec 的责任是把这件事写下来，不是默默留个坑。

## 3. 依赖与交接

### 3.1 我依赖谁

| 依赖 | 提供方 | 实况 |
|---|---|---|
| PostgreSQL 容器 | 成员 3 | `deploy/docker-compose.dev.yml` 已存在，**未验证能否起** |
| `internal/model/*.go`（10 个 struct） | 成员 3 | 只交付了 `user.go`。**本功能不依赖它，不构成阻塞** |
| 契约签署（[API_CONTRACT.md](../../API_CONTRACT.md) §13） | 三人共同 | **未完成**，端点全部 `⬜` |

### 3.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `pkg/response` + `pkg/errcode` | **成员 2**、成员 3 | 成员 2 的 spec 已标「待确认」；成员 3 的 handler 返回与错误出口 |
| `pkg/jwt` + 中间件链顺序 | 成员 3 | 受保护路由挂在 `JWTAuth` 之后 |
| `RegisterXxxRoutes` 约定 + `router.go` 挂载点 | 成员 2、成员 3 | 避免三人同时改 `router.go`（[总纲 §4.5](../../dev/MASTER.md#45-路由注册方式成员-1-冻结全员遵守)） |
| `backend/.env.example` | 成员 2、成员 3 | 本地起服务、成员 3 做 deploy 透传 |
| `GET /api/v1/health` | 成员 3 | [集成顺序](../../dev/MASTER.md#6-集成顺序按这个顺序联调不要跳)第 2 步的验证命令 |

**交付顺序由这张表决定**：`errcode` → `response` → 其余（`Fail(c, code errcode.ErrorCode)` 的类型依赖 `errcode`）。

### 3.3 交接边界：`backend/.env.example` 由两人共同写入

| 交付物 | 产出人 | 变量 |
|---|---|---|
| #7 | 成员 1 | `DB_*`、`JWT_SECRET`、`AI_SERVICE_URL`、`AI_SERVICE_TOKEN` |
| #4b | 成员 3 | `MOMENT_JOB_INTERVAL`、`PROACTIVE_JOB_INTERVAL` |

**约定**：本轮我只写入自己负责的 4 组变量，**不为成员 3 的变量留占位**；他后续自行添加。两人错开时间改同一文件，动手前在群里说一声。

## 4. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| 响应体固定 `{code, message, data, timestamp}`；HTTP 状态码与业务 code **分离** | [AGENTS.md §4.1](AGENTS.md) |
| `Fail()` **只接受 `errcode.ErrorCode`，不接受自定义 message** | AGENTS §4.1 |
| 新增错误码必须**同时**登记 `codeMessages` 与 `codeHTTPStatus` | AGENTS §4.1 |
| `errcode_test.go` 校验「常量集合 == `codeMessages` 键集合 == `codeHTTPStatus` 键集合」 | AGENTS §4.1 |
| 中间件链顺序 `Recovery → RequestLogger → CORS → BizErrorHandler → JWTAuth → Handler`，不可乱 | AGENTS §4.2 |
| 免鉴权白名单只有 4 个：`POST /auth/register`、`POST /auth/login`、`POST /auth/refresh`、`GET /health` | AGENTS §4.2 |
| handler 不写 `response.Fail`，错误用 `_ = c.Error(err)` 上抛 | AGENTS §4.1 |
| panic 堆栈、原始 error **只进日志，不返回前端** | AGENTS §4.1 |
| 错误码段位：200 ｜ 4000-4009 参数/注册 ｜ 4010-4019 认证 ｜ 4030-4039 授权 ｜ 4040-4049 未找到 ｜ 5000-5009 服务端 | AGENTS §4.1 |
| JSON 字段一律 camelCase | AGENTS §5 |
| 禁拼音命名；Go 缩写词全大写或全小写（`userID` 不是 `userId`）；文件名小写下划线 | AGENTS §5 |
| 不新建 `utils.go` / `common.go` / `misc.go`；一个文件只做一件事 | AGENTS §3 |
| `.env` 永不入库，仓库只留 `.env.example` | AGENTS §7 红线 1 |
| `JWT_SECRET` ≥32 字符、各环境不同、示例值禁止上线 | AGENTS §4.3 |
| `AI_SERVICE_TOKEN` 在 backend 与 ai-service 两边值必须一致，走 `X-Internal-Token` 头 | AGENTS §4.3 |

## 5. 验收标准

- [ ] `cd backend && go build ./...` 通过
- [ ] `cd backend && go vet ./...` 通过
- [ ] `cd backend && go test ./pkg/errcode/` 通过（三集合一致性校验生效）
- [ ] `go run ./cmd/server` 能起；`curl localhost:8080/api/v1/health` 返回 HTTP 200，body 为 `{"code":200,"message":...,"data":...,"timestamp":...}`
- [ ] 中间件链顺序与 AGENTS §4.2 一致
- [ ] 未鉴权访问任意非白名单路径 —— 本轮为**空壳预期行为**，`auth-login` 负责关闭
- [ ] `backend/.env.example` 已入库；`git check-ignore backend/.env` 有输出
- [ ] `pkg/response`、`pkg/errcode` 已推送，成员 2 / 成员 3 确认能 import
- [ ] `RegisterXxxRoutes` 注册方式已在群里广播，成员 3 确认按此写 `persona_handler.go`

## 6. 变更记录

| 日期 | 变更 | 原因 |
|---|---|---|
| 2026-09-13 | 创建（草稿） | 准备期基础设施，解锁成员 2 / 成员 3 并行开发 |

---

## 7. 待确认（定稿前清空）

**路由注册方式的广播时机**：[总纲 §4.5](../../dev/MASTER.md#45-路由注册方式成员-1-冻结全员遵守)要求由我冻结并告知另两人，但文档里已有示例代码，成员 3 可能已按示例动手。开工前需在群里确认一次：按文档示例为准，还是有调整。

# plan · 后端骨架搭建

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-skeleton`
> 负责人：成员 1 ｜ 最后更新：2026-09-13

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 状态 |
|---|---|---|---|---|
| 1 | 目录骨架 | `backend/{cmd,internal,pkg}` 下的包目录 | `go build ./...` 通过（空包也能编） | 🔶 部分：`pkg/{errcode,response}` 已建；`cmd/server`、`internal/{config,middleware,dto,handler,service,repository}`、`pkg/{logger,jwt}` 尚未创建 |
| 2 | **错误码（最高优先）** | `pkg/errcode/errcode.go`、`errcode_test.go` | `go test ./pkg/errcode/` 通过，三集合一致 | ✅ 已完成（PR #12 合入 develop） |
| 3 | **统一响应** | `pkg/response/response.go` | `Fail()` 签名不含 message 参数 | ✅ 已完成（PR #12 合入 develop） |
| 4 | 日志封装 | `pkg/logger/logger.go` | 提供 `*zap.Logger`，可被中间件注入 | 未开始 |
| 5 | JWT 基础包 | `pkg/jwt/jwt.go` | Access 2h / Refresh 7d，含 `TokenType` | 未开始 |
| 6 | 配置加载 | `internal/config/config.go` | 从环境变量读到 `DB_*` / `JWT_SECRET` / `AI_SERVICE_*` | 未开始 |
| 7 | 中间件 ×5 | `internal/middleware/*.go` | 4 个可用；JWTAuth 空壳且带 `TODO` 标记 | 未开始 |
| 8 | 装配与路由 | `cmd/server/main.go`、`handler/router.go` | GORM 初始化 + 中间件链装配 + `RegisterXxxRoutes` 挂载点 + `/health` | 未开始 |
| 9 | 环境变量模板 | `backend/.env.example` | 只含我负责的 4 组变量；`.env` 未入库 | 未开始 |
| 10 | 冒烟验收 | — | spec §5 全部勾上 | 未开始 |

**优先级说明**：**Step 2 → 3 优先于其余全部**。成员 2 的 spec 把 `pkg/response` 与 `pkg/errcode` 标为「待确认」，这是当前并行度最大的阻塞点；`pkg/response` 的类型又依赖 `pkg/errcode`，所以顺序不能倒。

**Step 8 放最后**：装配层要 import 到上面所有包，先写它会反复返工。

## 2. 文件清单

| 路径 | 作用 | 谁会用到 |
|---|---|---|
| `backend/pkg/errcode/errcode.go` | 错误码常量 + `codeMessages` + `codeHTTPStatus` | **成员 2、成员 3** |
| `backend/pkg/errcode/errcode_test.go` | 校验三个集合 key 一致 | 成员 1（提交前必跑） |
| `backend/pkg/response/response.go` | `Success[T]` / `Fail` | **成员 2、成员 3** |
| `backend/pkg/logger/logger.go` | zap 封装 | **成员 3**（他的 handler / service 要记日志） |
| `backend/pkg/jwt/jwt.go` | 令牌生成与解析 | **成员 3**（`JWTAuth` 的消费方） |
| `backend/internal/config/config.go` | 配置结构体 + 加载 | 成员 1、成员 3 |
| `backend/internal/middleware/biz_error.go` | 业务错误统一出口 | **成员 2、成员 3**（错误全靠它出） |
| `backend/internal/middleware/jwt.go` | 鉴权拦截（本轮空壳） | 成员 3 |
| `backend/internal/handler/router.go` | 路由汇总 + 中间件链 + 挂载点 | **成员 2、成员 3** |
| `backend/cmd/server/main.go` | 装配依赖、启动 HTTP | 成员 3（本地起服务） |
| `backend/.env.example` | 环境变量模板 | **成员 2、成员 3** |

## 3. 关键实现要点

### 3.1 `errcode` 的三集合一致性（Step 2）

三个东西必须同时存在、key 完全一致：

1. 常量集合（`ErrInvalidParam` 这类）
2. `codeMessages` 的键
3. `codeHTTPStatus` 的键

**已知坑**：`codeHTTPStatus` 漏登记会走兜底分支返回 **HTTP 500**——`4015`（改密码失败）就踩过。所以 `errcode_test.go` 不是形式主义，它拦的就是这个。

### 3.2 中间件链的装配位置（Step 8）

顺序 `Recovery → RequestLogger → CORS → BizErrorHandler → JWTAuth → Handler`。

其中 `BizErrorHandler` 的位置最容易放错：它要在路由组上、**在业务 handler 之前**，先 `c.Next()` 再检查 `c.Errors`。放在最外层则 `c.Errors` 还没被写入。

### 3.3 JWTAuth 空壳的写法要求（Step 7）

空壳必须**显式标记**，否则会变成隐形的安全漏洞：

- 函数体直接 `c.Next()` 放行
- 函数上方写 `TODO(auth-login): 实现 TokenType 校验与 4010/4011/4012 分发`
- 在 spec §2.3 与本节各留一处记录，保证 `auth-login` 不会漏掉

### 3.4 本轮的技术选型

| 项 | 值 | 说明 |
|---|---|---|
| Go | 1.27.1 | `go.mod` 已存在，module `github.com/nanirise/heart-echo/backend`，**不需要 `go mod init`** |
| 配置库 | `caarlos0/env` | 技术文档 §4.9 |
| 日志 | `zap` | 技术文档 §4.4 |
| ORM | `gorm` + `postgres` driver | 已在 `go.mod`；仅初始化连接，**不调 `AutoMigrate`**（那是成员 3 的 `model/migrate.go`） |
| `/health` 深度 | 只报进程存活 | 本轮决定，见 spec §2.1 |

## 4. 风险与对策

| 风险 | 对策 |
|---|---|
| **JWTAuth 空壳被遗忘，一直放行** | 三处留痕（代码 `TODO` + spec §2.3 + 本 plan §3.3）；`auth-login` 第一验收项 |
| `codeHTTPStatus` 漏登记 → 该 4xx 的错报 HTTP 500 | `errcode_test.go` 强制三集合一致 |
| 公共约定（响应结构 / 错误码 / 中间件顺序）冻结后被改 | 改 = 群里广播 + 更新 `API_CONTRACT.md`；这三个是本功能的核心交付物 |
| `backend/.env.example` 两人同时改 | 我只写自己的 4 组、不留占位（spec §3.3）；动手前群里说一声 |
| 本地没起 PostgreSQL 导致服务起不来 | `/health` 不 ping DB（本轮决定 1）；DB 连接失败的错误信息里带目标地址，便于排查 |
| 契约尚未签署就开工 | 不阻塞本功能（骨架不依赖字段细节）；但**准备期末门槛**，需在群里推进 |

## 5. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-13 | spec / plan 起草，等待审核 | 无（`model/*.go` 未交付但不阻塞本功能） |
| 2026-09-13 | Step 2 `pkg/errcode`、Step 3 `pkg/response` 完成，PR #12 合入 develop | 无 |
| 2026-09-13 | 错误码规则统一：`4031` 废弃、资源越权 `4043`、功能越权 `4030`；PR #13 修正 `4030` 文案 | 待成员 2、成员 3 同步各自文档 |
| 2026-09-13 | 按实际状态更新 Step 1-3 的状态列 | 无 |

# plan · 后端骨架搭建

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-bootstrap`
> 负责人：成员 1 ｜ 最后更新：2026-09-15

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 状态 |
|---|---|---|---|---|
| 1 | 目录骨架 | `backend/{cmd,internal,pkg}` 下的包目录 | `go build ./...` 通过（空包也能编） | 🔶 部分：`pkg/{errcode,response,logger,jwt}`、`internal/config` 已建；`cmd/server`、`internal/{middleware,dto,handler,service,repository}` 尚未创建 |
| 2 | **错误码（最高优先）** | `pkg/errcode/errcode.go`、`errcode_test.go` | `go test ./pkg/errcode/` 通过，三集合一致 | ✅ 已完成（PR #12 合入 develop） |
| 3 | **统一响应** | `pkg/response/response.go` | `Fail()` 签名不含 message 参数 | ✅ 已完成（PR #12 合入 develop） |
| 4 | 日志封装 | `pkg/logger/logger.go` | 提供 `*zap.Logger`，可被中间件注入 | ✅ 已完成（已合入 develop） |
| 5 | JWT 基础包 | `pkg/jwt/{jwt.go,jwt_test.go}` | Access 2h / Refresh 7d，含 `TokenType` | ✅ 已完成（已合入 develop） |
| 6 | 配置加载 | `internal/config/{config.go,config_test.go}` | 从环境变量读到 `DB_*` / `JWT_SECRET` / `AI_SERVICE_*` | ✅ 已完成（待 PR） |
| 7 | 中间件 ×5 | `internal/middleware/*.go` | 4 个可用；JWTAuth 空壳且带 `TODO` 标记 | ✅ 已完成（PR #23 合入 develop） |
| 8 | 装配与路由 | `cmd/server/main.go`、`handler/router.go` | GORM 初始化 + 中间件链装配 + `RegisterXxxRoutes` 挂载点 + `/health` | 未开始 |
| 9 | 环境变量模板 | `backend/.env.example` | 含 14 个变量（清单依据见 spec §3.3）；`.env` 未入库 | ✅ 已完成（随 Step 6 由 PR #20 合入） |
| 10 | 冒烟验收 | — | spec §5 全部勾上 | 未开始 |

**优先级说明**：**Step 2 → 3 优先于其余全部**。成员 2 的 spec 把 `pkg/response` 与 `pkg/errcode` 标为「待确认」，这是当前并行度最大的阻塞点；`pkg/response` 的类型又依赖 `pkg/errcode`，所以顺序不能倒。

**Step 8 放最后**：装配层要 import 到上面所有包，先写它会反复返工。

### 1.1 PR 划分

**一个 spec = 一个大功能，不是一次交付。** 大功能按「能独立验证、能单独 review」切成多个 PR，每个 PR 一次只做一件事（协作 §4.7、§3.3）。

| PR | 步骤 | 内容 | 状态 |
|---|---|---|---|
| #12 | Step 2–3 | `pkg/errcode` + `pkg/response` | ✅ 已合入 develop |
| — | Step 4–5 | `pkg/logger` + `pkg/jwt` | ✅ 已合入 develop（未单独开 PR，随其他合并进入） |
| #20 | Step 6 + 9 | `internal/config` + `backend/.env.example` | ✅ 已合入 develop |
| #23 | Step 7 | 中间件 ×5 | ✅ 已合入 develop |
| 待开 | Step 8–10 | 路由装配 + 冒烟验收 | 未开始 |

**2026-09-14 调整**：原表把 Step 6–7 合成一个 PR、`.env.example` 挂在 Step 8–10。现在改成 Step 6 单独一个 PR，并把 `.env.example` 并进它——理由是 `config.go` 读哪些变量和模板列哪些变量是同一件事的两面，分开写必然对不上。

**切分判据**：后续步骤只会「**使用**」而不会「**改动**」的部分，就可以先合。

- 可以提前合 —— `internal/config`：Step 7–10 全都只是消费它，已经定型
- 可以提前合 —— `pkg/logger` / `pkg/jwt`：同理
- 不能提前合 —— Step 8 的 `router.go`：它是所有包的汇合点，必须等中间件定稿

## 2. 文件清单

| 路径 | 作用 | 谁会用到 |
|---|---|---|
| `backend/pkg/errcode/errcode.go` | 错误码常量 + `codeMessages` + `codeHTTPStatus` | **成员 2、成员 3** |
| `backend/pkg/errcode/errcode_test.go` | 校验三个集合 key 一致 | 成员 1（提交前必跑） |
| `backend/pkg/response/response.go` | `Success[T]` / `Fail` | **成员 2、成员 3** |
| `backend/pkg/logger/logger.go` | zap 封装 | **成员 3**（他的 handler / service 要记日志） |
| `backend/pkg/jwt/jwt.go` | 令牌生成与解析 | **成员 3**（`JWTAuth` 的消费方） |
| `backend/internal/config/config.go` | 配置结构体 + 加载 | 成员 1、成员 3 |
| `backend/internal/config/config_test.go` | 校验 required 与密钥长度 | 成员 1（提交前必跑） |
| `backend/internal/middleware/recovery.go` | panic 捕获；包文档写在这里 | 成员 1（其余中间件都挂在它里面） |
| `backend/internal/middleware/logger.go` | 访问日志 + traceId；`ContextKey*` 常量定义在此 | **成员 2、成员 3**（取 userID 用常量，别手写字符串） |
| `backend/internal/middleware/cors.go` | 跨域 | 成员 2（前端本地联调） |
| `backend/internal/middleware/biz_error.go` | 业务错误统一出口 | **成员 2、成员 3**（错误全靠它出） |
| `backend/internal/middleware/jwt.go` | 鉴权拦截（本轮空壳） | 成员 3 |
| `backend/internal/handler/router.go` | 路由汇总 + 中间件链 + 挂载点 | **成员 2、成员 3** |
| `backend/cmd/server/main.go` | 装配依赖、启动 HTTP | 成员 3（本地起服务） |
| `backend/.env.example` | 环境变量模板（成员 3 后续自行追加 2 个 JOB 变量） | **成员 2、成员 3** |

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

**已落实**（2026-09-15）：三条都做到。`jwt.go` 的 `TODO` 里另附了 TECH_DESIGN §4.7 的五步实现清单和错误码对应关系，`auth-login` 照着填即可。

### 3.4 本轮的技术选型

| 项 | 值 | 说明 |
|---|---|---|
| Go | 1.27.1 | `go.mod` 已存在，module `github.com/nanirise/heart-echo/backend`，**不需要 `go mod init`** |
| 配置库 | `caarlos0/env/v11` | 技术文档 §4.9 |
| `.env` 加载 | `godotenv` | 技术文档 §4.9 只写了"`.env` 文件加载"没点库名。`caarlos0/env` **自己不读文件**，只认环境变量；而 AGENTS §2 的流程是 `cp ... .env` 后直接 `go run`，中间没人 `export`，所以必须有东西把文件灌成环境变量 |
| 日志 | `zap` | 技术文档 §4.4 |
| 跨域 | `gin-contrib/cors` | 技术文档 §4.5。`go get` 时会顺带升级一批间接依赖（`golang.org/x/crypto` 等），`go.mod` diff 偏大属正常 |
| Context 键名 | `ContextKey{TraceID,UserID,Username}` | 技术文档 §4.7 定的名字。成员 2、成员 3 的 spec 已按这些名字写，**不可改名** |
| ORM | `gorm` + `postgres` driver | 已在 `go.mod`；仅初始化连接，**不调 `AutoMigrate`**（那是成员 3 的 `model/migrate.go`） |
| `/health` 深度 | 只报进程存活 | 本轮决定，见 spec §2.1 |

### 3.5 `internal/config` 的四条设计决定（Step 6）

都是"看着可以那样写、但那样写会出问题"的地方，记下来免得以后被改回去。

| 决定 | 不这么做会怎样 |
|---|---|
| `godotenv.Load` 的错误**忽略** | 容器里没有 `.env`，报错会让服务起不来。生产靠 compose 注入环境变量，与本地走同一条代码路径 |
| `GIN_MODE` **不收进 Config** | gin 自己读、`pkg/logger.New()` 也自己读，再收进来就是**第三个事实来源**（同 `pkg/jwt` 拒绝写死有效期的理由） |
| 密钥长度校验写在 `validate()` | tag 里的 `required` 只保证非空、数不了长度。不兜住就漏到运行时 |
| `Load()` 只认环境变量一个来源 | 若让它直接读 `.env` 文件，生产环境得走另一条分支 |

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
| 2026-09-14 | 更正：Step 4–5（`pkg/logger`、`pkg/jwt`）早已提交并合入 `develop`，此前状态列写"未开始"与事实不符 | 无 |
| 2026-09-14 | Step 6 `internal/config` 完成（`config.go` + `config_test.go` 4 条用例，`build`/`vet`/`test` 全绿）；`backend/.env.example` 一并写入；新增依赖 `caarlos0/env/v11`、`godotenv`。PR 划分调整为 Step 6 单独一个 PR | 无 |
| 2026-09-14 | Step 6 + 9 由 **PR #20 合入 develop** | 无 |
| 2026-09-15 | Step 7 中间件 ×5 完成（`recovery` / `logger` / `cors` / `biz_error` / `jwt` 空壳），`build`/`vet` 全绿。新增依赖 `gin-contrib/cors` | 无 |
| 2026-09-15 | TECH_DESIGN §4.4 / §4.7 的示例代码与实际实现对不上（`jwtutil` → `pkg/jwt`、补 `traceId`、补 `Written()` 判断），已同步修正并在群里广播 | 无 |
| 2026-09-15 | Step 7 由 **PR #23 合入 develop**。`ContextKey*` 常量名按 TECH_DESIGN §4.7 对齐（原自拟的 `Ctx*` 作废） | 无 |

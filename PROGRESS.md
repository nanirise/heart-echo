# PROGRESS

最后更新：2026-09-15

## 成员1
- 在做：分支 `feature/backend-bootstrap`（spec: `docs/specs/backend-skeleton/`；spec 里写的 `feature/backend-skeleton` 已随 PR #12 合并，实际用的是 `feature/backend-bootstrap`）。plan 10 步**完成 8 步**——`pkg/{errcode,response,logger,jwt}`、`internal/config`、`backend/.env.example` 已合入 `develop`（PR #12 / #20、#13）；Step 7 中间件 ×5 已完成并**由 PR #23 合入 `develop`**（`feature/backend-bootstrap` 已合并）
- 下一步：① 从最新 `develop` 切新分支；② Step 8 `handler/router.go` + `cmd/server/main.go`（GORM 初始化 + 中间件链装配 + `/health` + `RegisterXxxRoutes` 挂载点）→ Step 10 冒烟验收
- 卡住：无
- 改了哪些文件：
  - 已合入 `develop`：`backend/pkg/{errcode,response,logger,jwt}/`（PR #12）、`backend/internal/config/` + `backend/.env.example`（PR #20）、`backend/internal/middleware/{recovery,logger,cors,biz_error,jwt}.go` + `gin-contrib/cors` 依赖（PR #23）
  - 全局契约：`AGENTS.md` §4.3、`docs/API_CONTRACT.md` §2/§4/§5/§7/§9/§10、`docs/TECH_DESIGN.md` §4.4/§4.7（示例代码与实际实现对不上，已修正）
  - 本功能文档：`docs/specs/backend-skeleton/{spec.md,plan.md}`

## 成员2
- 在做：分支 2「前端数据契约层」编码完成、等 PR —— `src/types/api.ts`、`src/types/errcode.ts`、`frontend/.env.example`，spec 与 plan 已提交（`docs/specs/frontend-api-types/`）
- 下一步：push 分支 → 开 PR → 等 AI 审计 → 合并后开分支 3（`request.ts` + 路由表 + `MainLayout.vue`）
- 卡住：无。风险：契约 §13 三人签署未完成，字段若再变，`types/*.ts` 需返工
- 改了哪些文件：本分支新增 `frontend/src/types/{api.ts,errcode.ts}`、`frontend/.env.example`、`docs/specs/frontend-api-types/{spec,plan}.md`、`PROGRESS.md`；分支 1 已合并 develop —— `frontend/{package.json,package-lock.json,tsconfig.json,vite.config.ts,index.html}`、`frontend/src/{main.ts,App.vue,vite-env.d.ts}`

## 成员3
- 在做：
- 下一步：
- 卡住：
- 改了哪些文件：

## 接口/数据结构变更
- 有。
  - 改了什么：错误码 `4031`（原「该人设不属于当前用户」）**废弃**。资源越权（访问他人 persona / 日程）统一返回 `4043`「人设不存在」+ HTTP 404；`4030` 收窄为**功能越权**专用，文案由「无权限访问该资源」改为「无权限使用该功能」
  - 涉及文件：`backend/pkg/errcode/errcode.go`、`docs/API_CONTRACT.md`、`AGENTS.md`、`docs/TECH_DESIGN.md`
  - 其他人要做什么：pull 最新 develop；归属校验**不要再返回 `4031`**；前端错误分支若按 `4031` 写过要改
- 有（2026-09-13 新增，**不是 HTTP 契约变更**，只影响 Go 内部调用方——主要是成员 3 的 `JWTAuth` 中间件）：
  - 改了什么：`backend/pkg/jwt` 对外只有两个函数：
    - `GenerateTokenPair(userID uint64, username, secret string, accessTTL, refreshTTL time.Duration) (*TokenPair, error)`
    - `ParseToken(tokenStr, secret string) (*Claims, error)`
    - 校验失败只返回本包的 `ErrTokenExpired` / `ErrTokenInvalid`，调用方用 `errors.Is` 判断，**不需要 import 底层 `golang-jwt` 库**
    - `Claims.UserID` 的类型从 TECH_DESIGN 原先写的 `uint` 改为 **`uint64`**，与 `internal/model.User.ID` 对齐（否则查库处处要转换）
  - 涉及文件：`backend/pkg/jwt/jwt.go`、`docs/TECH_DESIGN.md` §4.7
  - 其他人要做什么：写中间件时按上面签名调用即可，不用引 `golang-jwt`；**TECH_DESIGN §4.7 的中间件示例还是旧写法**（`jwtutil.ParseToken` + 底层库的 `jwt.ErrTokenExpired`），我会在写 Step 7 时一并改掉
- 有（2026-09-14 新增，**不是 HTTP 契约变更**，但成员 2、成员 3 本地要动手）：
  - 改了什么：新增 `backend/.env.example`，模板含 **14 个**变量：`SERVER_PORT`、`GIN_MODE`、`DB_HOST`/`DB_PORT`/`DB_USER`/`DB_PASSWORD`/`DB_NAME`/`DB_SSLMODE`、`JWT_SECRET`、`JWT_ACCESS_EXPIRE`、`JWT_REFRESH_EXPIRE`、`AI_SERVICE_URL`、`AI_SERVICE_TOKEN`、`CORS_ALLOW_ORIGINS`
  - 涉及文件：`backend/.env.example`（新增）、`backend/internal/config/config.go`（新增）
  - 其他人要做什么：本地 `cp backend/.env.example backend/.env` 后**必须填 `JWT_SECRET`（≥32 字符）和 `AI_SERVICE_TOKEN`**——这两个标了 `required`，缺了服务直接起不来；`AI_SERVICE_TOKEN` 要与 `ai-service` 侧的值一致
  - **成员 3**：`MOMENT_JOB_INTERVAL` / `PROACTIVE_JOB_INTERVAL` 请自行追加进 `backend/.env.example`，按 spec §3.3 的约定我**没有留占位行**
- 有（2026-09-15 新增，**不是 HTTP 契约变更**，是 Go 内部接口；**成员 2、成员 3 的阻塞解除了**）：
  - 改了什么：`backend/internal/middleware` 落地，对外可用的东西：
    - `middleware.ContextKeyUserID` = `"userId"`、`middleware.ContextKeyUsername` = `"username"`、`middleware.ContextKeyTraceID` = `"traceId"`
      —— 常量名按 TECH_DESIGN §4.7。**取 userID 一律用常量，别手写 `"userId"`**
    - `middleware.BizErrorHandler(logger *zap.Logger) gin.HandlerFunc` —— handler 里写 `_ = c.Error(err)` 上抛，响应由它统一出口
    - `middleware.JWTAuth(secret string) gin.HandlerFunc` —— ⚠️ **空壳，直接放行，等于没有鉴权**
  - 涉及文件：`backend/internal/middleware/*.go`、`docs/TECH_DESIGN.md` §4.4/§4.7
  - 其他人要做什么：
    - ① 写 handler 时错误只 `_ = c.Error(err)`，**不调 `response.Fail`**；成功才自己写响应
    - ② 越权防线取 userID 写 `c.GetUint64(middleware.ContextKeyUserID)`。⚠️ **本轮 JWTAuth 是空壳，取到的一律是 0**，别把它当成"已鉴权"
    - ③ 若你按 TECH_DESIGN §4.7 的**旧示例**写了 `jwtutil.ParseToken` 或 import 了 `golang-jwt`，改掉：用 `pkg/jwt` 的 `ParseToken` 和它暴露的 `ErrTokenExpired` / `ErrTokenInvalid`，配 `errors.Is` 判断

## 待确认
- 错误码规则已于 2026-09-13 定为：**资源越权 → `4043`**、**功能越权 → `4030`**、**`4031` 废弃**。两处文档均已同步：
- ✅ `docs/specs/persona-model/{spec.md,plan.md}`（PR #14 `ed0887d`，`4040`→`4043`、`4031`→`4030` 全量替换）
- ✅ `docs/dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md` 第 83、223 行（2026-09-14 复查确认，已改为 `4043`）
- ✅ 已于 2026-09-14 修订：`docs/specs/backend-skeleton/spec.md` §3.3 原写"只写入自己负责的 **4 组**变量"，与实际写进 `backend/.env.example` 的 **14 个**不一致。依据 [MASTER §5](docs/dev/MASTER.md) 明写"完整清单见技术文档 §4.6"，已按 §4.6 的 16 个减掉成员 3 的 2 个 JOB 变量，落定为 14 个
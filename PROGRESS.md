# PROGRESS

最后更新：2026-09-14

## 成员1
- 在做：分支 `feature/backend-bootstrap`（spec: `docs/specs/backend-skeleton/`；spec 里写的 `feature/backend-skeleton` 已随 PR #12 合并，实际用的是 `feature/backend-bootstrap`）。plan 10 步**完成 6 步**——`pkg/{errcode,response,logger,jwt}` 已合入 `develop`；Step 6 `internal/config` + `backend/.env.example` 已完成、本地验证通过（4 条用例），本分支待开 PR
- 下一步：① 提交 Step 6（`internal/config/` + `backend/.env.example` + `backend/go.{mod,sum}`）→ push → 开 PR；② 之后 Step 7 中间件 ×5 → Step 8 `router.go` + `cmd/server/main.go` → Step 10 冒烟验收
- 卡住：无
- 改了哪些文件：
  - 已合入 `develop`：`backend/pkg/{errcode,response,logger,jwt}/`
  - 本分支新增：`backend/internal/config/{config.go,config_test.go}`、`backend/.env.example`、`backend/go.{mod,sum}`（新增 `caarlos0/env/v11`、`godotenv`）
  - 全局契约：`AGENTS.md` §4.3、`docs/API_CONTRACT.md` §2/§4/§5/§7/§9/§10
  - 本功能文档：`docs/specs/backend-skeleton/{spec.md,plan.md}`

## 成员2
- 在做：
- 下一步：
- 卡住：
- 改了哪些文件：

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

## 待确认
- 错误码规则已于 2026-09-13 定为：**资源越权 → `4043`**、**功能越权 → `4030`**、**`4031` 废弃**。两处文档均已同步：
- ✅ `docs/specs/persona-model/{spec.md,plan.md}`（PR #14 `ed0887d`，`4040`→`4043`、`4031`→`4030` 全量替换）
- ✅ `docs/dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md` 第 83、223 行（2026-09-14 复查确认，已改为 `4043`）
- ⬜ **待修订**：`docs/specs/backend-skeleton/spec.md` §3.3 写的是"只写入自己负责的 **4 组**变量"，与实际写进 `backend/.env.example` 的 **14 个**不一致。依据：[MASTER §5](docs/dev/MASTER.md) 明写"完整清单见技术文档 §4.6"
# PROGRESS

最后更新：2026-09-14

## 成员1
- 在做：分支 `feature/backend-bootstrap`（spec: `docs/specs/backend-skeleton/`；spec 里写的 `feature/backend-skeleton` 已随 PR #12 合并，实际用的是 `feature/backend-bootstrap`）。plan 10 步**完成 4 步**——`pkg/errcode`、`pkg/response`、`pkg/logger` 已提交并 push；`pkg/jwt` 已写完且本地验证通过（7 条测试），**未提交**
- 下一步：① **收工提交**——`pkg/jwt/` + `backend/go.{mod,sum}` + `TECH_DESIGN.md` §4.7 + `plan.md` §1.1 → push → 开 **PR（Step 4–5：`pkg/logger` + `pkg/jwt`）**，切分依据见 `plan.md` §1.1「PR 划分」；② 之后 Step 6 `internal/config`（`caarlos0/env`）→ Step 7 中间件 ×5 → Step 8 `router.go` + `cmd/server/main.go` → Step 9 `.env.example` → Step 10 冒烟验收，合成第二个 PR
- 卡住：无。等成员 2、成员 3 确认错误码规则的同步（见下方「待确认」）
- 改了哪些文件：
  - 已提交并 push：`backend/pkg/errcode/{errcode.go,errcode_test.go}`、`backend/pkg/response/response.go`、`backend/pkg/logger/logger.go`
  - **本次未提交**：`backend/pkg/jwt/{jwt.go,jwt_test.go}`、`backend/go.{mod,sum}`（新增 `golang-jwt/v5`）、`docs/TECH_DESIGN.md` §4.7、`docs/specs/backend-skeleton/plan.md` §1.1
  - 全局契约：`AGENTS.md` §4.3、`docs/API_CONTRACT.md` §2/§4/§5/§7/§9/§10
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

## 待确认
- 错误码规则已于 2026-09-13 定为：**资源越权 → `4043`**、**功能越权 → `4030`**、**`4031` 废弃**。成员 3 的两处文档：
- ✅ **已同步**：`docs/specs/persona-model/{spec.md,plan.md}`（PR #14 `ed0887d`，`4040`→`4043`、`4031`→`4030` 全量替换）
- ⬜ **未同步**：`docs/dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md` 第 83、223 行仍写 `4031`
# PROGRESS

最后更新：2026-09-21

## 成员1
- 在做：**Week 1 完成**（2026-09-21 实测通过）。总纲 §7 给 Week 1 定的验收项「与成员 2 前后端联调成功」达成——PR B 合并后一直欠着的那一步补上了，五条都在浏览器里跑通：① 注册后直接进 `/chat` ② **F5 刷新仍保持登录态**（从 localStorage 的 `heart-echo-auth` 恢复，这是 Week 1 唯一的真验收项）③ 换隐身窗口走 `/login` 能进 ④ 密码错 → `4013`、重复用户名 → `4004` ⑤ 后端访问日志里能看到 `POST /api/v1/auth/register` 真的打进来
  联调环境（**本机，不入库**）：原生 **PostgreSQL 16**，不走 Docker。**库名、账号、密码三样都是 `heart`**；`backend/.env` / `frontend/.env` 由各自 `.env.example` 复制后改值——**注意本地 `DB_USER`/`DB_NAME` 是 `heart`，不是模板里的 `heart_echo`**。`.env` 不入库，各人本地自成一套，照抄模板去连库会连错
  踩到一个 PostgreSQL 15+ 的行为变化，记一笔：**普通角色在 `public` schema 建表会被拒（`SQLSTATE 42501`）**。Docker 的 postgres 镜像把 `POSTGRES_USER` 建成超级用户，所以撞不到；原生装法用的是普通角色，必须补一句 `ALTER SCHEMA public OWNER TO heart;`（在该库内执行）。**成员 3** 若也用原生 PG 跑 schema 核对，会卡在这一步
  **PR C 未开始**（`GET`/`PUT /user/profile`、`PUT /user/password`），可从 `develop` 切 `feature/auth-profile`
- 下一步：① 进 **Week 2 生死线：流式对话（SSE）**——`ai-service/` 目录**尚不存在**，`chat_service` / `message_repo` / `chat_handler` / Go 侧的 AI 客户端也一行没写，先从 `docs/specs/chat-message/` 的 spec 读起；② 之后再切 `feature/auth-profile` 做 PR C（repo 剩 2 个方法、service 剩 3 个、handler 剩 3 个 + `router.go` 1 行）
- ★ **遗留待补（2026-09-19 已知未做，下次改）**：
  1. `docs/specs/auth-login/spec.md` 未收尾：§5.1 的 8 个验收勾、§6 变更记录加一行、§7 待确认第 2 条（广播）、头部状态行补「PR A 已合并」
  2. `docs/API_CONTRACT.md` §12 的「已广播」列**仍全为 ⬜**、§13 三方冻结签署仍空白。**队长 2026-09-19 判定这两条「算通过」，文档尚未同步**——不改的话以后没人说得清
  3. `deploy/.env.example` 缺失；`backend/.env.example` 里 `MOMENT_JOB_INTERVAL` / `PROACTIVE_JOB_INTERVAL` 只有一行占位注释、变量本身没有（MASTER §3 #4b，成员 3 的活）
  4. `ai-service/` 目录**不存在**，`SCHEDULE_PARSE_BACKEND` 无从落地（MASTER §3 #4c）
  5. **本文件下面的成员 2 / 成员 3 段落已过期**：成员 2 写「在做前端数据契约层」但那是早已合并的 PR #21；成员 3 整段空白，而他实际交付了 8 个 model PR + persona CRUD 后端。这份表现在不能当排期依据（他们的段落不归我改）
  6. ~~`router.go` 第 40 行「本轮 JWTAuth 仍是空壳」的失效注释~~ —— **本次 PR 已修**
  7. **新发现（本 PR 不修）**：`gin-contrib/cors` 收到**空的 origin 列表会直接 panic**（`all origins disabled`）。生产靠 `internal/config` 里 `CORS_ALLOW_ORIGINS` 的 `envDefault` 兜着，但谁在 `.env` 里显式写一行 `CORS_ALLOW_ORIGINS=`（空串），服务会在**启动时 panic**，而不是报一条配置错误
  8. **新发现（2026-09-20，本 PR 不修，归全员）**：`gofmt -l .` 会在 `backend/` 下列出 30+ 个文件，逐个查过**都不是排版问题、是 CRLF 行尾**（`tr -d '\r'` 之后 gofmt 差异为空）。已有的文件是 CRLF、本次新增的是 LF。现在没人管这条，但**若 CI 将来加 `gofmt -l` 检查会全线报红**。要治就统一行尾（`.gitattributes` 或 `core.autocrlf`），是全局决定，不该我单独改一批文件
  9. **新发现（2026-09-20，本 PR 不修）**：`encoding/json` 匹配结构体字段时**优先精确匹配、匹配不上会退化成大小写不敏感匹配**。所以契约 §3.1 的 `username` 写成 `userName` 也能落进 `RegisterRequest.Username`——请求体这侧的 json tag 名**在 Go 这层钉不住**（`internal/dto/auth_dto_test.go` 里留了一条断言放行的用例记录这件事）。真正被钉住的是**响应体**的字段名（`TestUserResponseJSONTags` 逐字比对）。**成员 2**：前端按契约写 `username`，别依赖这个宽容
  10. **`SCHEMA_CHECK_DSN` 与 `DB_*` 两套连接配置并存**（2026-09-21 发现）：`cmd/migrate` 用前者、`cmd/server` 用后者，且前者不加载 `.env`、不在 `.env.example` 里。属**跨成员约定**，要队长拍板是否统一（`docs/specs/persona-model/plan.md:161` 已记为待定）。本轮不改代码，只在跑 migrate 时临时 export
  11. **bcrypt 72 字节的说法在 6 处仍是错的**（2026-09-21 更正，见下方「接口/数据结构变更」）：`docs/specs/auth-login/spec.md:116`（归我，待改）；`AGENTS.md:138`、`docs/API_CONTRACT.md:97`、`docs/TECH_DESIGN.md:853`、`docs/TECH_DESIGN.md:1079`（**共享文件，要广播**）；`docs/specs/frontend-auth-pages/plan.md:112`（**成员 2 的文件**，不自己动）。结论（必须限制密码长度）不变，只是理由从「截断」改成「报 `ErrPasswordTooLong`」。**建议凑一次广播一起改**——反正契约 §12 的「已广播」列本来就全空，欠着
- 卡住：**无**。此前「本机无 PostgreSQL」的阻塞已解，Week 1 端到端验证跑通。
  仍**未决**（不阻塞，但欠着）：`cmd/migrate` 读的是**裸 `os.Getenv("SCHEMA_CHECK_DSN")`**，不加载 `.env`、也不在 `.env.example` 里；而 `cmd/server` 走的是 `DB_*` 系列。两套连接参数各说各的。**「该不该统一」待队长拍板**（记录在 `docs/specs/persona-model/plan.md:161`）。眼下跑迁移时临时 `export` 一个绕开，**没改代码**
  **Docker 推迟到 Week 2 流式对话做完再装**——Week 4 的部署/交付才真正需要它
- 改了哪些文件：
  - 已合入 `develop`：`backend/pkg/{errcode,response,logger,jwt}/`（PR #12）、`backend/internal/config/` + `backend/.env.example`（PR #20）、`backend/internal/middleware/{recovery,logger,cors,biz_error,jwt}.go` + `gin-contrib/cors` 依赖（PR #23）；`backend/internal/handler/{router.go,health_handler.go}`、`backend/cmd/server/main.go`（Step 8–10）
  - PR A 新增（PR #34）：`docs/specs/auth-login/{spec.md,plan.md}`、`backend/internal/middleware/jwt_test.go`
  - PR A 修改（PR #34）：`backend/internal/middleware/jwt.go`（空壳 → 真实实现）、`backend/internal/middleware/logger.go`（access log 的 `userId` 由 `GetString` 改 `GetUint64`——类型不符时 gin 静默取到空串，表现是「鉴权生效了但日志里 userId 一直是空的」）、`backend/pkg/jwt/jwt_test.go` + `backend/pkg/jwt/jwt.go`（修偶发失败与注释，见下「接口/数据结构变更」）
  - persona 路由 PR 新增（`feature/backend-persona-routes`）：`backend/internal/handler/router_test.go`（259 行）
  - persona 路由 PR 修改（`feature/backend-persona-routes`）：`backend/internal/handler/router.go`（装配 4 条 persona 路由 + 删掉失效的空壳注释）、`PROGRESS.md`、`docs/specs/auth-login/plan.md`
  - 全局契约与总纲：`AGENTS.md` §4.3、`docs/dev/MASTER.md` §4.5（路由示例拆成 `api` / `protected` 双组）、`docs/API_CONTRACT.md` §2/§4/§5/§7/§9/§10/§11、`docs/TECH_DESIGN.md` §4.4/§4.7（示例代码与实际实现对不上，已修正）
  - 任务书：`docs/dev/MEMBER_1_BACKEND_AI.md` §1（准备期清单 9 项按实际进度重标）
  - 本功能文档：`docs/specs/backend-skeleton/{spec.md,plan.md}`、`.learn/2026-09-16-router-assembly.md`（本地未追踪）
  - auth PR B 新增（`feature/auth-register-login`）：`backend/internal/dto/auth_dto.go`、`backend/internal/dto/auth_dto_test.go`、`backend/internal/repository/user_repo.go`、`backend/internal/repository/user_repo_test.go`、`backend/internal/service/auth_service.go`、`backend/internal/service/auth_service_test.go`、`backend/internal/handler/auth_handler.go`
  - auth PR B 修改：`backend/internal/handler/router.go`（3 条 auth 路由挂到**免鉴权的 `api` 组** + 更新白名单注释）、`backend/internal/handler/router_test.go`（加 `TestAuthRoutesAreNotBehindJWTAuth`）、`backend/go.mod` + `go.sum`（`golang.org/x/crypto`、`github.com/jackc/pgx/v5` 由 indirect 变 direct）、`docs/specs/auth-login/plan.md`、`PROGRESS.md`

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
- 有（2026-09-16 新增，**Go 内部接口，影响成员 3 挂路由**）：
  - 改了什么：`internal/handler/router.go` 落地，对外分两个路由组：
    - `api`（=`/api/v1`）= **免鉴权区**：现有 `GET /health`，另 3 个 auth 端点由 `feature/auth-login` 挂这里
    - `protected`（`api` 的子组，多一道 `JWTAuth`）= **业务路由全挂这里**
    - `NewRouter(cfg *config.Config, logger *zap.Logger, db *gorm.DB) *gin.Engine`
  - ⏳ **待广播**：`docs/dev/MASTER.md` §4.5 已更新（路由示例拆成 `api` / `protected` 双组），群公告文案已备。**发出后由我勾上 `docs/API_CONTRACT.md` §12 的「已广播」**（当前仍是 ⬜）
  - 涉及文件：`backend/internal/handler/{router.go,health_handler.go}`、`backend/cmd/server/main.go`、`docs/dev/MASTER.md` §4.5
  - 其他人要做什么：
    - **成员 3**：写好的 `RegisterPersonaRoutes(rg, h)` 签名不变，只是要挂在 `protected` 上。注册函数**不要自己 `r.Group(...)` 建组**，用调用方传进来的 `rg`。在群里说一声，由我在 `router.go` 加那一行（MASTER §4.5 约定：不要两人同时改这个文件）
    - **成员 3**：你三份 spec 的「路由挂载」行还是旧写法 `RegisterXxxRoutes(api, ...)`——请改成 `protected`：`docs/specs/persona-model/spec.md` 第 36 行、`docs/specs/chat-message/spec.md` 第 36 行、`docs/specs/user-memory/spec.md` 第 48 行（你的文件我不动）
    - **成员 3**：`internal/model/migrate.go` 的注释写「由 `cmd/server` 启动时调用」**与实际不符**——`main.go` 刻意**不调** `AutoMigrate`，建表只走 `cmd/migrate`。该文件归你，请改一下注释（我不动别人的文件）
    - **成员 2**：无影响（`/health` 后端自查用，前端不消费）
- 有（2026-09-16 新增，**HTTP 契约补充，不影响已完成的前端代码**）：
  - 改了什么：`GET /health` 补全取值定义——`status` = `"ok"` / `"degraded"`，`dependencies.database` 与 `dependencies.aiService` = `"ok"` / `"down"`（原先只定义了 `"ok"`）。并明确**依赖异常时仍返回 HTTP 200 + 业务 code 200**
  - 涉及文件：`docs/API_CONTRACT.md` §11/§12、`backend/internal/handler/health_handler.go`
  - 其他人要做什么：**成员 3** 若在 compose 里按「非 200 即不健康」写 healthcheck，注意依赖挂了它照样回 200，要判 `data.status == "degraded"`
- 有（2026-09-18 新增，**不是 HTTP 契约变更**，但**成员 3 必须知道**——`protected` 组从 PR A 合并起真的拦人了）：
  - 改了什么：`middleware.JWTAuth` 从空壳改为真实校验（`feature/auth-middleware`，auth-login 的 PR A）。行为变化：
    - `protected` 组下的路由**必须带 `Authorization: Bearer <access token>`**：没带 → `4010`；access token 过期 → `4012`；签名不对 / 格式错 / 拿 refresh token 当 access 用 → `4011`
    - `c.GetUint64(middleware.ContextKeyUserID)` **从合并起真的等于签发时的 userID**——此前空壳不写 Context，取到的一律是 `0`。谁按它做归属校验，此前的防线是空的，现在才真正生效
    - `c.GetString(middleware.ContextKeyUsername)` 同理
    - 中间件失败时只做 `c.Error()` + `c.Abort()`，响应仍由 `BizErrorHandler` 出口，**没有新增任何错误码**
  - ⚠️ **契约本身没变**：`API_CONTRACT.md` §1 早就写了「其余端点一律需要 Token，缺失或过期返回 `4010`」，这次是实现在补上，所以 §12 **不新增行**
  - 涉及文件：`backend/internal/middleware/{jwt.go,jwt_test.go,logger.go}`、`backend/pkg/jwt/{jwt_test.go,jwt.go}`
  - 其他人要做什么：
    - **成员 3**：`router.go` 的 `protected` 组下**目前一条路由都还没有**（我只留了挂载点注释）。你的 `RegisterPersonaRoutes` 已在 `persona_handler.go` 第 26 行，但那一行挂载还没加——等你确认后我加（MASTER §4.5：这个文件不要两人同时改）
    - **成员 2**：`request.ts` 的 `4012` → 刷新重放分支、`4010` / `4011` → 登出分支，合并后会真的被触发
  - ★ **待广播**：本条要连同下面 2026-09-16 那条「路由挂载方式」一起发——**两条至今都是 ⬜**，`docs/API_CONTRACT.md` §12 的「已广播」列全空
- 有（2026-09-18 新增，**测试修复，不是接口变更**，但 CI 会红所以记一笔）：
  - 改了什么：`pkg/jwt/jwt_test.go` 的 `TestParseRejectsTamperedToken` 原先靠改签名的**最后一个字符**来模拟篡改，这个写法是错的。HS256 签名 32 字节 → base64url 编码 43 字符，末位字符只有**高 4 位**是有效数据，低 2 位是填充位、解码时被丢弃；字母表里 `U`(20)=`010100` 与 `X`(23)=`010111` 高 4 位相同，所以末位恰好是 `U` 时（概率 **1/16**）把 `U` 改成 `X`，解出的 32 字节一个比特都没变——签名依然有效，篡改根本没发生，测试就红了
  - 这就是 CI 上 `Backend (push)` 红、`Backend (pull_request)` 绿的原因：**两份跑的是同一份代码**，只是签令牌落在了不同的秒。不是环境问题
  - 改为改签名段的**首字符**（6 位全是有效数据，改它必然改变签名字节），并对原字符做避让（撞上原字符等于没改，概率 1/64）。20 万个真随机签名对照实验：旧写法 12451 次篡改无效（6.23%），新写法 0 次
  - 涉及文件：`backend/pkg/jwt/jwt_test.go`、`backend/internal/middleware/jwt_test.go`、`backend/pkg/jwt/jwt.go`（注释里「过期 → 4011、其余一律 → 4010」订正为 4012 / 4011，与契约 §2 对齐）
  - 其他人要做什么：无。但**写测试时注意**：base64 / JWT 这类「改末位字符」的篡改手法都可能因填充位而失效，改中间的字符更可靠
- 有（2026-09-21 新增，**不是接口变更，是文档订正 + 待广播**）：
  - 改了什么：bcrypt 在 72 字节处的行为，**文档一直写错了**。原文各处都写「只看前 72 字节，超出部分静默失效（截断）」，而 `golang.org/x/crypto@v0.55.0` 的 `bcrypt/bcrypt.go:96` 是 `if len(password) > 72 { return nil, ErrPasswordTooLong }`——**是拒绝，不是丢弃**。
  - **结论完全不变**（密码必须限长，`binding:"...max=32,printascii"` 照旧），**变的只是理由**：不是「超出部分白设」，而是「超长密码会让 `hashPassword` 返回错误 → 用户拿到一个改不了也重试不了的 `5000`」。谁维护这块时按旧理由推演，会推出「那把上限提到 100 也没事」这种错结论。
  - 顺带一处不对称：`CompareHashAndPassword`（登录方向）**没有**这个长度检查。生成方向报错、比对方向不报错。
  - 涉及文件：**已改** `backend/internal/dto/auth_dto.go`（注释）、`docs/specs/auth-login/plan.md` §3.5
  - ⏳ **待广播（6 处仍是错的）**：`docs/specs/auth-login/spec.md:116`（我的，待改）；`AGENTS.md:138`、`docs/API_CONTRACT.md:97`、`docs/TECH_DESIGN.md:853`、`docs/TECH_DESIGN.md:1079`（**共享文件，需广播**）；`docs/specs/frontend-auth-pages/plan.md:112`（**成员 2 的文件，我不动**）
  - 其他人要做什么：**成员 2** 如见到 `docs/specs/frontend-auth-pages/plan.md:112` 的同款说法，可顺手订正（那是你的文件）；其余共享文件等广播后一起改。**不改也不影响任何代码行为**——只是别拿旧说法做新推导
- 有（2026-09-21 新增，**不是接口变更，是范围冻结，但成员 3 要看**）：
  - 决定了什么：**流式对话做真流式（SSE），不做伪流式**。`POST /chat/stream` 按 `docs/API_CONTRACT.md` §6 实现（那份契约已冻结，前端成员 2 早已按它写好了消费端）。**负责人：成员 1**（`docs/specs/chat-message/spec.md` 里这条本就分给我）
  - 为什么现在定：`chat_service` / `message_repo` / `chat_handler` 和 `ai-service/` **一行都还没写**——这是最后能低成本改口的时刻；再往后就是返工。且总纲 §10「🚫 绝不砍」清单里就有「流式对话」
  - 涉及文件（**将来要新增，现在都还不存在**）：`backend/internal/handler/chat_handler.go`、`backend/internal/service/chat_service.go`、`backend/internal/repository/message_repo.go`、`backend/internal/client/ai_client.go`、`ai-service/`（整个目录尚未创建）
  - 其他人要做什么：**成员 3**——`chat_messages` 表是你建的，若你原计划里有「伪流式」的中间态（比如先整段返回再切片）请停手，通道语义要对齐 SSE；**成员 2**——无动作，前端按契约 §6 写的实现就是对的，这次是后端跟上

## 待确认
- 错误码规则已于 2026-09-13 定为：**资源越权 → `4043`**、**功能越权 → `4030`**、**`4031` 废弃**。两处文档均已同步：
- ✅ `docs/specs/persona-model/{spec.md,plan.md}`（PR #14 `ed0887d`，`4040`→`4043`、`4031`→`4030` 全量替换）
- ✅ `docs/dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md` 第 83、223 行（2026-09-14 复查确认，已改为 `4043`）
- ✅ 已于 2026-09-14 修订：`docs/specs/backend-skeleton/spec.md` §3.3 原写"只写入自己负责的 **4 组**变量"，与实际写进 `backend/.env.example` 的 **14 个**不一致。依据 [MASTER §5](docs/dev/MASTER.md) 明写"完整清单见技术文档 §4.6"，已按 §4.6 的 16 个减掉成员 3 的 2 个 JOB 变量，落定为 14 个
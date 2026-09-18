# spec · 认证与用户（auth-login）

> 功能名：auth-login ｜ 分支：`feature/auth-middleware` → `feature/auth-register-login` → `feature/auth-profile`（跨多个分支交付，PR 划分见 [plan §1.1](plan.md#11-pr-划分)）
> 负责人：成员 1 ｜ 状态：草稿（待审核）
> 创建：2026-09-18 ｜ 最后更新：2026-09-18
> 关联：[接口契约 §3](../../API_CONTRACT.md) ｜ [开发总纲 §3 / §4.2](../../dev/MASTER.md) ｜ [成员 1 开发文档 §2](../../dev/MEMBER_1_BACKEND_AI.md) ｜ [技术文档 §4.7 / §4.8](../../TECH_DESIGN.md) ｜ [backend-skeleton spec §2.2](../backend-skeleton/spec.md)

---

## 1. 背景与目标

[backend-skeleton spec §2.2](../backend-skeleton/spec.md) 明确把这三件事留给了本功能：

1. `POST /auth/register`、`POST /auth/login`、`POST /auth/refresh`
2. `GET` / `PUT /user/profile`、`PUT /user/password`
3. **`JWTAuth` 中间件的真实校验逻辑**

其中第 3 条是**安全口子**：当前 [jwt.go](../../../backend/internal/middleware/jwt.go) 直接 `c.Next()` 放行，`protected` 组形同虚设，任何人不带 token 都能访问业务路由。这是准备期刻意留下的不对称，不是遗漏——但必须在本功能关闭。

成员 2 的 `feature/frontend-auth-request` 已完成（`request.ts` 的 4012 自动刷新重放、`auth.ts` 的 `login` / `register` / `logout` 都写完了），合并后它会真实调用本功能的 4 个端点。**他是本功能的直接消费方。**

**目标（一句话）**：注册 → 登录 → 刷新页面保持登录态这条链路真跑通，且 `protected` 组真正拦得住没 token 的请求。

这是 [开发总纲 §8.2](../../COLLABORATION.md) 的 **Week 1 末里程碑**。

## 2. 范围

### 2.1 做什么（In Scope）

| # | 端点 | 鉴权 | 错误码 | 产物 |
|---|---|---|---|---|
| 1 | `POST /api/v1/auth/register` | 免鉴权 | `4001` `4003` `4004` | `auth_handler.go` + `auth_service.go` |
| 2 | `POST /api/v1/auth/login` | 免鉴权 | `4001` `4013` | 同上 |
| 3 | `POST /api/v1/auth/refresh` | 免鉴权 | `4014` | 同上 |
| 4 | `GET /api/v1/user/profile` | 需 access token | `4010` | 同上 |
| 5 | `PUT /api/v1/user/profile` | 需 access token | `4001` `4004` | 同上 |
| 6 | `PUT /api/v1/user/password` | 需 access token | `4001` `4015` | 同上 |

| 项 | 产物 |
|---|---|
| **JWTAuth 真实实现** | `internal/middleware/jwt.go` + `jwt_test.go`（关闭空壳，见 §2.3） |
| 请求/响应 DTO | `internal/dto/auth_dto.go` |
| 用户仓储 | `internal/repository/user_repo.go` |
| 认证业务逻辑 | `internal/service/auth_service.go`（bcrypt、令牌签发） |
| 路由挂载 | `internal/handler/router.go` 增加 6 行挂载 |

### 2.2 不做什么（Out of Scope）

| 不做的事 | 归属 / 原因 |
|---|---|
| **封禁用户拦截** | `internal/model/user.go` **没有** status / banned 字段，数据库层就没有这个概念。本期不实现，也不预留字段——预留了没人写、没人维护，比没有更糟。将来要做要先改表（成员 3） |
| 登出接口 `POST /auth/logout` | 契约 §3 无此端点。无状态 JWT 下"登出"= 前端清空 localStorage（成员 2 已实现） |
| Token 服务端失效 / 黑名单 | 契约 §3.6 明写"修改成功后服务端不主动失效旧 Token，所有其他设备上的 Token 仍然有效——这是本期接受的简化" |
| 邮箱验证、验证码、找回密码 | 契约 §3 无此端点 |
| OAuth / 第三方登录 | README 无此需求 |
| 头像**文件上传** | 契约 §3.5：`avatarUrl` 只存 URL，不做文件上传 |
| 用户注销、删除账号 | 契约 §3 无此端点 |
| `pkg/jwt` 本身的改动 | 已在 backend-skeleton 冻结（`GenerateTokenPair` / `ParseToken`），本功能只**消费** |
| 前端一切 | 成员 2（`frontend-auth-request` 已完成待合并） |

### 2.3 本功能第一条验收项：关闭 JWTAuth 空壳

[backend-skeleton spec §2.3](../backend-skeleton/spec.md) 把这件事写明为 `auth-login` 的第一条验收项，本 spec 承接：

| 中间件 | 当前状态 | 本功能交付后 |
|---|---|---|
| Recovery / RequestLogger / CORS / BizErrorHandler | 可用 | 不变 |
| **JWTAuth** | **空壳——直接放行，等于没有鉴权** | 真实校验，见 §5 |

⚠️ 在 PR 1 合并之前，`protected` 组下的所有路由都是**公开的**。若今天只提 PR 1，这个口子当天关闭；若推迟，必须知道它开着的每一天都有风险。

## 3. 依赖与交接

### 3.1 我依赖谁

| 依赖 | 提供方 | 实况 |
|---|---|---|
| `internal/model/user.go` | 成员 3 | ✅ 已交付。字段：`ID uint64` / `Username` / `Email` / `PasswordHash`（`json:"-"`）/ `AvatarURL *string` / `CreatedAt` / `UpdatedAt` |
| `pkg/jwt`（`GenerateTokenPair` / `ParseToken` / `ErrTokenExpired` / `ErrTokenInvalid`） | 成员 1（已冻结） | ✅ 合入 `develop` |
| `pkg/errcode` / `pkg/response` / 中间件链 / `router.go` 挂载点 | 成员 1（已冻结） | ✅ 合入 `develop` |
| `users` 表（`cmd/migrate` 建表） | 成员 3 | ⚠️ `model` 已交付，但**本机无 PostgreSQL**，见 §3.3 |
| `bcrypt` | `golang.org/x/crypto` | ⚠️ 当前在 `go.mod` 里是 **indirect**，本功能开始引用后需 `go mod tidy` 转为 direct |
| 契约 §3 的字段与错误码 | 三人共同 | ✅ 字段已定；§13 冻结签署仍为空 |

### 3.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| 4 个认证端点的真实实现 | **成员 2** | `frontend-auth-request` 已按契约写完调用方，合并后直接联调 |
| `4010` / `4011` / `4012` / `4013` / `4014` / `4015` 的真实语义 | 成员 2 | `request.ts` 已按这些码写了分支（4012 → 刷新重放；4010/4011/4014 → 登出） |
| `JWTAuth` 可用 | **成员 3** | 他的 persona / moment / proactive 路由挂在 `protected` 上，之前是裸的 |
| `ContextKeyUserID` 真实有值 | 成员 3 | 越权防线取 userID 做归属校验；本轮之前**取到的一律是 0** |

### 3.3 本功能最大的现实约束：本机没有数据库

| 项 | 实况 |
|---|---|
| `backend/.env` | 不存在 |
| Docker Desktop | 本机未安装（`docker` 命令不存在），今日不打算装 |
| PostgreSQL | 无 |

**后果**：端点 1–6 全都读写在 `users` 表上，**本机没有端到端验证手段**。能做的只有：

- `go build` / `go vet` / `go test` 全绿
- DTO 校验、service 逻辑（用桩仓储）的表驱动单测
- JWTAuth 的完整验证——**它不碰数据库**（用 `pkg/jwt` 造 token + `httptest`），所以端点 1–6 里唯一能完整验证的就是这一块

**约定**：验收标准里凡需要真库的条目，一律标注"待 PostgreSQL 就位后复验"，**不勾 ✅**。宁可留空，不把没跑过的算成通过。

## 4. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| 密码用 **bcrypt**，cost ≥ 10；**严禁明文 / MD5 / SHA1** | 协作 §10.4 红线 5 |
| 登录失败统一返回 `4013`，**不区分**「用户不存在」与「密码错误」 | 成员 1 文档 §2（防用户名枚举） |
| 密码长度 8–32 位，**限 ASCII 可见字符** | 契约 §3.1（bcrypt 在 72 字节处截断，超长部分静默失效） |
| 响应体里**不得出现 `passwordHash`** | `model.User` 已标 `json:"-"`，DTO 不得另起字段绕过它 |
| handler 里**不写** `response.Fail`，错误一律 `_ = c.Error(err)` 上抛给 BizErrorHandler | AGENTS §4.1 |
| 未命中路由的 404 走 gin 默认响应，不经统一响应体 | backend-skeleton plan §3.2 已记录的能力边界 |
| `GET` / `PUT /user/profile`、`PUT /user/password` 的**目标用户只能来自 token**，不得接受请求参数传 `userId` | 协作 §10.4 红线 4（越权）。这是本功能最容易被写错的一点 |
| 取 userID 用 `middleware.ContextKeyUserID` 常量，不手写 `"userId"` | TECH_DESIGN §4.7 |
| 新增错误码必须**同时**登记 `codeMessages` 与 `codeHTTPStatus` | AGENTS §4.1。本功能**预期不新增任何错误码**，6 个端点全部复用现有码 |
| `JWT_SECRET` 从 `internal/config` 读，不硬编码 | AGENTS §7 红线 1 |
| `.env` 永不入库 | 同上 |
| JSON 字段 camelCase；Go 缩写词全大写或全小写（`userID` 不是 `userId`）；文件名小写下划线 | AGENTS §5 |

## 5. 验收标准

### 5.1 JWTAuth（不需要数据库，可完整验证）

- [ ] 无 `Authorization` 头 → `4010`
- [ ] `Authorization` 无 `Bearer ` 前缀 → `4010`
- [ ] access token 已过期 → `4012`
- [ ] **refresh token 当 access 用** → `4011`（`claims.TokenType != "access"`）
- [ ] 签名被篡改 / 密钥不对 → `4011`
- [ ] 合法 access token → 放行，且 `c.GetUint64(ContextKeyUserID)` 等于签发时的 `userID`、`c.GetString(ContextKeyUsername)` 等于用户名
- [ ] 任一失败路径都调用了 `c.Abort()`，业务 handler **未被执行**
- [ ] 失败响应的 body 是 `{code, message, data, timestamp}` 结构，`message` 与 `errcode` 一一对应

### 5.2 端点 1–6（需要 PostgreSQL，本机未验）

- [ ] `POST /auth/register` 返回 token pair + user 对象（**注册即登录**，契约 §3.1）
- [ ] 邮箱重复 → `4003`；用户名重复 → `4004`
- [ ] `POST /auth/login` 成功返回 token pair；失败（用户不存在 **或** 密码错）一律 `4013`
- [ ] `POST /auth/refresh` 返回新的 token pair；refresh token 无效 → `4014`
- [ ] `GET /user/profile` 带 token 可读；不带 token → `4010`
- [ ] `PUT /user/profile` 能改 `avatarUrl` / `username`；用户名冲突 → `4004`
- [ ] `PUT /user/password` 旧密码错 → `4015`；成功后新密码可登录
- [ ] 库中存的是 bcrypt 哈希，不是明文
- [ ] 任一响应体中不出现 `passwordHash`

### 5.3 工程项

- [ ] `cd backend && go build ./...` / `go vet ./...` / `go test ./...` 全绿
- [ ] `go.mod` 中 `golang.org/x/crypto` 由 indirect 转 direct
- [ ] 6 个端点的鉴权归属正确：1–3 挂 `api` 组，4–6 挂 `protected` 组
- [ ] 未新增错误码；若确需新增，三集合一致性测试通过

> **本 spec 的验收不完整是已知的、被接受的**：§5.2 全部条目依赖 PostgreSQL，本机无法验证。这份清单在 PG 就位前不勾 ✅。

## 6. 变更记录

| 日期 | 变更 | 原因 |
|---|---|---|
| 2026-09-18 | 创建（草稿） | 承接 backend-skeleton §2.2 的移交；Week 1 里程碑 |

## 7. 待确认（定稿前清空）

1. **契约 §13 冻结签署**仍是三方空白 —— 本功能把 §3 的 6 个端点从"纸面"变成"实现"，签署越晚，返工代价越大。三人共同事项。
2. **路由挂载方式的群广播**：[API_CONTRACT.md](../../API_CONTRACT.md) §12 的「已广播」列**所有行仍为 ⬜**（含 2026-09-16 已发过的 `/health` 那条）。核实是漏勾还是漏发。
3. **端到端验证的替代手段**：本机无 Docker / PostgreSQL。是否向成员 3 要一个可连的库（他的 `deploy/docker-compose.dev.yml` 本地可能起得来），待定。

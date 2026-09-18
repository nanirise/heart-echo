# plan · 认证与用户（auth-login）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/auth-middleware` → `feature/auth-register-login` → `feature/auth-profile`
> 负责人：成员 1 ｜ 最后更新：2026-09-18

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 状态 |
|---|---|---|---|---|
| 1 | **关闭 JWTAuth 空壳** | `internal/middleware/jwt.go`、`jwt_test.go` | spec §5.1 全部 8 条勾上；`go test ./internal/middleware/` 通过 | ⬜ 未开始 |
| 2 | 请求 / 响应 DTO | `internal/dto/auth_dto.go` | 5 个结构体的 binding tag 与契约 §3 的校验规则逐条对应，表驱动单测通过 | ⬜ 未开始 |
| 3 | 用户仓储 | `internal/repository/user_repo.go` | 6 个方法：`Create` / `FindByUsername` / `FindByEmail` / `FindByID` / `UpdateProfile` / `UpdatePassword` | ⬜ 未开始 |
| 4 | 认证业务逻辑 | `internal/service/auth_service.go` | `Register` / `Login` / `Refresh` / `GetProfile` / `UpdateProfile` / `ChangePassword`；bcrypt；令牌签发 | ⬜ 未开始 |
| 5 | HTTP 层 | `internal/handler/auth_handler.go` | 6 个 handler，薄到只做"绑定 → 调 service → 写响应" | ⬜ 未开始 |
| 6 | 路由挂载 | `internal/handler/router.go` | 前 3 个挂 `api` 组，后 3 个挂 `protected` 组 | ⬜ 未开始 |
| 7 | 冒烟验收 | — | spec §5 全部勾上（**§5.2 依赖 PostgreSQL，本机无法完成**） | ⬜ 未开始 |

**顺序不可颠倒**：Step 2 → 3 → 4 → 5 是"类型从内向外逐层依赖"（handler 的入参类型来自 dto、service 的返回来自 repo），先写外层会反复返工。Step 1 独立，可以先做——它不碰 dto / repo / service 任何一个。

**Step 1 优先做**：它是当前开着的安全口子（spec §2.3），且**不受"本机没有数据库"的约束**，是整条链路上唯一今天能完整验证的部分。

### 1.1 PR 划分

| PR | 分支 | 步骤 | 内容 | 状态 |
|---|---|---|---|---|
| **A** | `feature/auth-middleware` | spec + Step 1 | 本 spec/plan + JWTAuth 实装 + 单测 | ⬜ 今日目标 |
| **B** | `feature/auth-register-login` | Step 2–4、6（部分） | register / login / refresh 三个端点的 dto + repo + service + handler + 挂载 | ⬜ 今日目标（缺端到端验证） |
| **C** | `feature/auth-profile` | Step 5、6（其余） | `GET`/`PUT /user/profile`、`PUT /user/password` | ⬜ 待 B 合并后 |

**切分判据**：一个 PR = 一件**能独立验证、能单独 review** 的事。

- PR A 独立：不依赖数据库，也不依赖 B、C 的任何文件
- PR B 是一个完整功能点（"能登录了"），三个端点共用同一套 dto/repo/service 骨架，拆开是重复样板
- PR C 在 B 的文件上**追加**方法，从 B 合并后的 `develop` 切出，不冲突

> ⚠️ **PR B 是"提上去验不了端到端"的 PR**（spec §3.3）。它只有单测覆盖。这样做的理由是成员 2 的 `frontend-auth-request` 在等这 3 个端点，代码早写出来，PG 一就位就能立刻验，不必等到那时候才开始写。**PR 描述里必须写明"未做端到端验证"**，不能含糊。

## 2. 文件清单

| 路径 | 作用 | 谁会用到 | PR |
|---|---|---|---|
| `backend/internal/middleware/jwt.go` | JWTAuth 真实实现（**改现有文件**） | 成员 3（他的路由挂在 `protected` 上） | A |
| `backend/internal/middleware/jwt_test.go` | 中间件单测（新建） | 成员 1 | A |
| `backend/internal/dto/auth_dto.go` | 5 个请求/响应结构体 + binding tag | 成员 2（对照前端调用） | B |
| `backend/internal/repository/user_repo.go` | `users` 表的 6 个查询/写入 | 成员 1 | B |
| `backend/internal/service/auth_service.go` | 业务逻辑 + bcrypt + 令牌签发 | 成员 1 | B（C 追加） |
| `backend/internal/handler/auth_handler.go` | 6 个 handler | 成员 2、成员 3（写法模板） | B（C 追加） |
| `backend/internal/handler/router.go` | 6 行挂载（**改现有文件**） | 全员 | B、C |
| `backend/go.mod` / `go.sum` | `golang.org/x/crypto` indirect → direct | 全员 | B |

## 3. 关键实现要点

都是"看着可以那样写、但那样写会出问题"的地方，记下来免得以后被改回去。

### 3.1 JWTAuth 的错误出口必须接上 BizErrorHandler（Step 1）

`JWTAuth` 自己**不写响应**，失败时 `c.Error(errcode.New(...))` + `c.Abort()`，响应由 `BizErrorHandler` 出口——这是 AGENTS §4.1 定的统一出口。

**单测的坑**：只挂 `JWTAuth` 是测不出响应的，`c.Errors` 没人消费。测试里必须把 `BizErrorHandler` 一起挂上（或在测试 harness 里复刻 `router.go` 的链），断言 HTTP 状态码与 body。**这反过来是最好的回归测试**——它同时验证了中间件链的装配顺序没被改坏。

### 3.2 `PUT /user/profile`：区分"字段没传"和"传了 null"（Step 2）

契约 §3.5：`avatarUrl` **传 `null` 可清空**，而两个字段都可选（只传要改的）。

Go 里 `*string` 字段无法区分这两种情况——未传和传 `null` 都是 `nil`。用 `json.RawMessage`：

```go
type UpdateProfileReq struct {
    AvatarURL json.RawMessage `json:"avatarUrl"` // nil=没传；null=清空；否则解析成 string
    Username  *string         `json:"username"`  // nil=没传
}
```

**不这么写会怎样**：传 `null` 清空头像会被当成"没传"，用户点了清空头像没反应，且这个 bug 在只测 `PUT {"username":"x"}` 时看不出来。

### 3.3 唯一冲突要靠数据库兜底，不能只靠"先查后插"（Step 3–4）

`4003`（邮箱重复）/ `4004`（用户名重复）用 `FindByEmail` / `FindByUsername` 先查一次是必要的——但它**拦不住并发**：两个请求同时查到"不存在"，然后同时插入，一个会撞上唯一索引报 500。

正确做法是两层：先查（给出友好错误）→ 插入时捕获 PostgreSQL 的 unique violation（**SQLSTATE `23505`**），按约束名映射成 `4003` / `4004`。

**不这么写会怎样**：并发注册同一用户名时接口返回 `5003 数据库操作失败` 而不是 `4004`，前端分支走错。

### 3.4 登录防枚举要连**时间**一起防（Step 4）

契约与成员 1 文档都要求"登录失败统一 `4013`，不区分用户不存在与密码错误"。但如果用户不存在时**直接返回**、跳过 bcrypt 比对，响应时间会明显短于"用户存在但密码错"（bcrypt cost 10 约 50–100ms），攻击者靠计时就能枚举出哪些用户名存在。**返回码统一了，侧信道没堵上。**

做法：用户不存在时，对一个固定的假哈希照样跑一次 `CompareHashAndPassword`，让两条路径耗时相近。

### 3.5 bcrypt 在 72 字节处截断（Step 2）

契约 §3.1 要求密码"8–32 位，限 ASCII 可见字符"就是为了这个：bcrypt 只看前 72 字节，超长部分**静默失效**。32 个 ASCII 字符最多 32 字节，留足余量。binding tag 要同时限制长度和字符集（`alphanum` 不够——`Passw0rd!` 里有 `!`）。

### 3.6 目标用户只能来自 token（Step 4–5）

`GET`/`PUT /user/profile`、`PUT /user/password` 三个端点，service 方法的签名**不接收** `userID` 参数以外的用户标识，handler 从 `c.GetUint64(middleware.ContextKeyUserID)` 取。

**不做的事**：不提供 `GET /user/profile?userId=2` 这种形式，哪怕是"方便调试"。一旦存在，越权防线就多了一个入口，而协作 §10.4 红线 4 是功能性问题不是风格问题。

⚠️ 在 Step 1 落地前，这个值**恒为 0**（空壳不写 Context）。所以 Step 4 的归属校验**必须在 Step 1 之后才谈得上正确**。

## 4. 风险与对策

| 风险 | 对策 |
|---|---|
| **JWTAuth 空壳被遗忘，一直放行** | 三处留痕（`jwt.go` 的 `TODO` + backend-skeleton spec §2.3 + 本 spec §2.3）；列为第一条验收项；Step 1 今天就做 |
| **本机无 PostgreSQL，端点 1–6 无法端到端验证** | 验收标准里如实标注，不勾 ✅；PR 描述写明"未端到端验证"；向成员 3 要可连的库（spec §7 第 3 条） |
| 并发注册撞唯一索引返回 500 而非 4003/4004 | 捕获 SQLSTATE `23505`（§3.3） |
| 登录接口可被计时枚举用户名 | 用户不存在时也跑一次假哈希比对（§3.4） |
| `ContextKeyUserID` 取到 0 导致归属校验形同虚设 | Step 1 与 Step 4 的**顺序**写进本 plan；单测断言 Context 里写进去的确实是签发时的 userID |
| bcrypt cost 过高拖慢接口 | cost 固定 10（技术文档 §4.8）；不为了"更安全"调到 14——登录是高频接口 |
| 响应体泄漏 `passwordHash` | `model.User` 已标 `json:"-"`；handler 返回 DTO 而非 model，单测断言 body 不含该字段 |
| 契约 §13 三方签署未完成，§3 字段若再变 | 本功能阶段契约已定稿，返工面可控；但签署越晚代价越大（spec §7 第 1 条） |

## 5. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-18 | spec / plan 起草，等待审核 | 本机无 Docker / PostgreSQL，端点 1–6 无端到端验证手段 |

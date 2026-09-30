# plan · 认证与用户（auth-login）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/auth-middleware` → `feature/auth-register-login` → `feature/auth-profile`
> 负责人：成员 1 ｜ 最后更新：2026-09-30

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 状态 |
|---|---|---|---|---|
| 1 | **关闭 JWTAuth 空壳** | `internal/middleware/jwt.go`、`jwt_test.go` | spec §5.1 全部 8 条勾上；`go test ./internal/middleware/` 通过 | ✅ 已完成（PR A，PR #34 已合并） |
| 2 | 请求 / 响应 DTO | `internal/dto/auth_dto.go` | 5 个结构体的 binding tag 与契约 §3 的校验规则逐条对应，表驱动单测通过 | ✅ 已完成（PR B） |
| 3 | 用户仓储 | `internal/repository/user_repo.go` | 6 个方法：`Create` / `FindByUsername` / `FindByEmail` / `FindByID` / `UpdateProfile` / `UpdatePassword` | ✅ 6/6（PR B 前 4 个，PR C 补 `UpdateProfile` / `UpdatePassword`） |
| 4 | 认证业务逻辑 | `internal/service/auth_service.go` | `Register` / `Login` / `Refresh` / `GetProfile` / `UpdateProfile` / `ChangePassword`；bcrypt；令牌签发 | ✅ 6/6（PR B 前 3 个 + bcrypt + 令牌签发，PR C 补 profile / 改密三件套） |
| 5 | HTTP 层 | `internal/handler/auth_handler.go` | 6 个 handler，薄到只做"绑定 → 调 service → 写响应" | ✅ 6/6（PR B 前 3 个，PR C 补后 3 个 + `RegisterUserRoutes`） |
| 6 | 路由挂载 | `internal/handler/router.go` | 前 3 个挂 `api` 组，后 3 个挂 `protected` 组 | ✅ 前 3 个挂 `api`（PR B），后 3 个挂 `protected`（PR C） |
| 7 | 冒烟验收 | — | spec §5 全部勾上 | ✅ 已完成（2026-09-30，本机原生 PG 16 + 真后端，spec §5 全勾） |

**顺序不可颠倒**：Step 2 → 3 → 4 → 5 是"类型从内向外逐层依赖"（handler 的入参类型来自 dto、service 的返回来自 repo），先写外层会反复返工。Step 1 独立，可以先做——它不碰 dto / repo / service 任何一个。

**Step 1 优先做**：它是当前开着的安全口子（spec §2.3），且**不受"本机没有数据库"的约束**，是整条链路上唯一今天能完整验证的部分。

### 1.1 PR 划分

| PR | 分支 | 步骤 | 内容 | 状态 |
|---|---|---|---|---|
| **A** | `feature/auth-middleware` | spec + Step 1 | 本 spec/plan + JWTAuth 实装 + 单测 | ✅ **已合并（PR #34）** |
| **B** | `feature/auth-register-login` | Step 2–4、6（部分） | register / login / refresh 三个端点的 dto + repo + service + handler + 挂载 | ✅ **已合并（PR #45）** |
| **C** | `feature/auth-profile` | Step 3–6（其余） | `GET`/`PUT /user/profile`、`PUT /user/password` | ✅ **已合并（PR #50，2026-09-30）** |

**切分判据**：一个 PR = 一件**能独立验证、能单独 review** 的事。

- PR A 独立：不依赖数据库，也不依赖 B、C 的任何文件
- PR B 是一个完整功能点（"能登录了"），三个端点共用同一套 dto/repo/service 骨架，拆开是重复样板
- PR C 在 B 的文件上**追加**方法，从 B 合并后的 `develop` 切出，不冲突

> ⚠️ **PR B 是"提上去验不了端到端"的 PR**（spec §3.3）。它只有单测覆盖。这样做的理由是成员 2 的 `frontend-auth-request` 在等这 3 个端点，代码早写出来，PG 一就位就能立刻验，不必等到那时候才开始写。**PR 描述里必须写明"未做端到端验证"**，不能含糊。
>
> ✅ **2026-09-30 已补上**：本机 PG 就位后，PR C 的冒烟把 PR B 的 3 个端点一并端到端打了（register / login / refresh 都在 §5.2 里）。这段"欠着"到此为止——**留着它是为了让后来人知道当时为什么敢在没验的情况下合并**，不是因为还欠着。

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
| `backend/internal/handler/auth_handler_test.go` | handler 的**请求体处理**测试（新建，PR C） | 成员 1 | C |

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

### 3.5 bcrypt 在 72 字节处**报错**，不是截断（Step 2）

> 更正（2026-09-21）：本节原写"只看前 72 字节、超出部分静默失效"，与所用版本的行为不符。
> `golang.org/x/crypto@v0.55.0` 的 `bcrypt/bcrypt.go:96`：`GenerateFromPassword` 在
> `len(password) > 72` 时 `return nil, ErrPasswordTooLong`。**行为是拒绝，不是丢弃。**
> 结论（必须限制密码长度）不变，理由变了。

契约 §3.1 要求密码"8–32 位，限 ASCII 可见字符"就是为了这个：不给上限的话，超长密码会让 `hashPassword` 失败，`Register` 走 `errcode.Wrap(errcode.ErrInternal, err)`，用户拿到的是 `5000 服务端内部错误`——一个他改不了、重试也没用的错。32 个 ASCII 字符最多 32 字节，离 72 有大余量。binding tag 要同时限制长度和字符集（`alphanum` 不够——`Passw0rd!` 里有 `!`）。

顺带一处不对称：`CompareHashAndPassword`（登录方向，`bcrypt.go:108-125`）**没有**这个长度检查。生成方向报错，比对方向不报错。

### 3.6 `PUT /user/profile` 要**严格解码**，不能默认忽略多余字段（Step 5）

> 2026-09-30 新增。契约 §3.5 同步加了同样的说明。

`c.ShouldBindJSON` 走的是 `encoding/json` 的默认解码：请求体里有对不上结构体字段的键，**不报错、直接丢掉**。

对别的端点无所谓，对这一个不行。它是全项目唯一的"部分更新"接口——两个字段都可选，所以 `{}` 是**合法**请求。于是这两种情况返回的响应一模一样：

- 调用方发 `{}` —— 什么都没想改，`200` 正确
- 调用方把 `username` 拼成 `user_name` —— 键被丢掉，什么都没改，也是 `200`

第二种人看到"保存成功"，只能自己猜。改用 `json.Decoder.DisallowUnknownFields()`，多个不认识的键就返 `4001`。

**两个坑**：

1. **`ValidateStruct` 必须显式补上**。`ShouldBindJSON` 是"解码 + 跑 binding tag"两件事，换成手写 `Decoder` 只剩第一件。漏了它 `min=3` / `alphanum` 会**静默失效**——不报错，只是不再拦人。测试里那条"太短的用户名"就是专门钉这个的。
2. **别用 gin 的全局开关** `binding.EnableDecoderDisallowUnknownFields`。它是一个包级 bool，一改全项目所有端点都变成严格模式，包括别人的。这里要的只是单端点。

**前端的雷**：把整个 user 对象 PUT 上来（`{id, username, email, avatarUrl, createdAt}`）会 `4001`。store 里正好有整个对象，顺手 PUT 过去是很自然的写法，所以契约 §3.5 专门写了警示。

### 3.6 目标用户只能来自 token（Step 4–5）

`GET`/`PUT /user/profile`、`PUT /user/password` 三个端点，service 方法的签名**不接收** `userID` 参数以外的用户标识，handler 从 `c.GetUint64(middleware.ContextKeyUserID)` 取。

**不做的事**：不提供 `GET /user/profile?userId=2` 这种形式，哪怕是"方便调试"。一旦存在，越权防线就多了一个入口，而协作 §10.4 红线 4 是功能性问题不是风格问题。

⚠️ 在 Step 1 落地前，这个值**恒为 0**（空壳不写 Context）。所以 Step 4 的归属校验**必须在 Step 1 之后才谈得上正确**。

## 4. 风险与对策

| 风险 | 对策 |
|---|---|
| **JWTAuth 空壳被遗忘，一直放行** | 三处留痕（`jwt.go` 的 `TODO` + backend-skeleton spec §2.3 + 本 spec §2.3）；列为第一条验收项；Step 1 今天就做 |
| ~~**本机无 PostgreSQL，端点 1–6 无法端到端验证**~~ | **2026-09-30 已解除**：本机原生装了 PG 16，六个端点全部端到端打过（spec §5.2）。当时的对策（如实标注、PR 描述写明、向成员 3 借库）已不再需要 |
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
| 2026-09-18 | 审核通过，范围收敛为**只做 PR A**；Step 1 完成——`JWTAuth` 实装，单测 10 条子用例（8 拒绝 + 2 放行）全绿 | 同上 |
| 2026-09-18 | 顺带修复 `pkg/jwt/jwt_test.go` 的偶发失败测试（base64url 末位填充位导致篡改无效，1/16 概率），CI 恢复绿 | 同上 |
| 2026-09-18 | 已知未做：**路由挂载方式的群广播**（`API_CONTRACT.md` §12「已广播」列仍全空）、**契约 §13 三方冻结签署**仍空白 | 两条都需要三人到场 |
| 2026-09-20 | **PR B 代码完成**：Step 2 dto（+ 30 条子用例）、Step 3 repo 4 个方法、Step 4 service 3 个方法、Step 5 前 3 个 handler、Step 6 挂到 `api` 组。`go build` / `go vet` / `go test ./...` 全绿；新测试全部做过反向注入（削弱 tag、把路由挂到 `protected` 上），确认拦得住 | **端点仍无法端到端验证**（本机无 Docker / PostgreSQL），PR 描述已如实标注 |
| 2026-09-20 | 与决策 ③「repo 只做 3 个方法」的偏差：决策 ①（refresh 要回查库）需要 `FindByID`，实际 4 个。偏差已并入 PR B，PR C 的 repo 剩 2 个方法 | — |
| 2026-09-30 | **PR C 代码完成**：Step 3 补 `UpdateProfile` / `UpdatePassword`、Step 4 补 profile / 改密三件套、Step 5 补后 3 个 handler + `RegisterUserRoutes`、Step 6 挂 `protected` 组（`router.go`，与成员 3 的 persona 路由同组）。新增 `internal/handler/auth_handler_test.go` 承接「handler 拿到请求之后怎么解析」这一类测试（`router_test.go` 只管路由装配，两者的分工写进了各自的文件头） | — |
| 2026-09-30 | **端到端冒烟通过**，Step 7 完成，spec §5 全勾。本机原生 PG 16 + `go run ./cmd/server`，约 30 条请求含 20 多条负例，日志 0 ERROR / 0 500。**PR B 当时欠的"未端到端验证"补上了** | 无（阻塞已解） |
| 2026-09-30 | Step 5 的偏差：`PUT /user/profile` 改用严格解码（`bindStrictJSON`），请求体出现 `username` / `avatarUrl` 以外的键返回 `4001`。原因见契约 §3.5——它是全项目唯一的"部分更新"接口，宽松解码会让"拼错字段名"和"什么都没改"返回同样的 `200 保存成功` | 契约行为变更，**待广播** |
| 2026-09-30 | **PR C 已合并（PR #50）**，本 plan 的 Step 1–7 全部结束，auth-login 整条链路交付完毕 | 无 |

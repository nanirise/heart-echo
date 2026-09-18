# 登录与注册页（frontend-auth-pages）· 实施计划

> 配套合同：[spec.md](spec.md)。本文只记录**怎么做**与**为什么这么做**。
> 注释纪律见 [AGENTS §5.1](../../../AGENTS.md)：代码里只留「为什么」，讲解留在本文。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-auth-pages` |
| 状态 | 🚧 进行中 |
| 依赖 | `feature/frontend-router-layout`（已合并 PR #31） |
| 规模 | 3 个源文件（2 新增 + 1 修改）+ 2 篇文档 |

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 状态 |
|---|------|------|:----:|
| 1 | 写 `spec.md` 与 `plan.md` | 2 篇文档 | 🚧 |
| 2 | `views/auth/LoginView.vue` | 新增 | ⏳ |
| 3 | `views/auth/RegisterView.vue` | 新增 | ⏳ |
| 4 | 路由表换 `component`（2 行） | 改 `router/index.ts` | ⏳ |
| 5 | 自查：`typecheck` / `build` / 浏览器逐条过 spec §5.1 | 证据 | ⏳ |
| 6 | 提交：文档一笔、代码一笔 | 2 笔 | ⏳ |

**顺序理由**：视图必须先于路由改动落地——路由表引用的文件不存在时 `vue-tsc` 报 TS2307、构建直接失败（3-B 踩过同一个坑，见其 `plan.md` §3.2）。所以第 4 步一定在第 2、3 步之后。

---

## 2. 文件清单

| 文件 | 类型 | 说明 |
|------|------|------|
| `frontend/src/views/auth/LoginView.vue` | 新增 | 登录表单；消费 `?redirect=` |
| `frontend/src/views/auth/RegisterView.vue` | 新增 | 注册表单；成功后直接进应用 |
| `frontend/src/router/index.ts` | 修改 | `/login`、`/register` 的 `component` 由 `Placeholder` 换成 `() => import(...)` |
| `docs/specs/frontend-auth-pages/spec.md` | 新增 | 合同 |
| `docs/specs/frontend-auth-pages/plan.md` | 新增 | 本文 |

`views/auth/` 目录本支首次创建；`api/` 目录不新增文件（理由见 §3.5）。

---

## 3. 关键实现要点

### 3.1 `?redirect=` 必须防开放重定向（本支唯一的安全红线）

3-B 的守卫在拦截未登录访问时会写 `{ name: 'Login', query: { redirect: to.fullPath } }`，本支要把它读回来。**直接 `router.replace(route.query.redirect)` 是错的**：攻击者可以构造

```
http://你的站点/login?redirect=https://evil.com
```

用户以为是在自己站点登录，登录成功后被送去钓鱼页——而**地址栏里那条链接确实是你发出去的域名**，这比普通钓鱼更有说服力。同理 `//evil.com`（协议相对 URL）也会跳出站外。

**判定规则**：只接受**站内绝对路径**。最稳的写法不是字符串前缀判断，而是交给 `URL` 解析后**比 origin**：

```ts
function resolveRedirect(raw: unknown): string {
  if (typeof raw !== 'string') return '/chat'
  const url = new URL(raw, window.location.origin)
  // 只有 origin 完全一致才认；pathname + search 重新拼，避免把 origin 带进去
  if (url.origin !== window.location.origin) return '/chat'
  return url.pathname + url.search
}
```

比 origin 顺手把 `//evil.com`、`/\evil.com`、`https://evil.com` 三种变体一次挡掉，比手写前缀判断更难写漏。`typeof raw !== 'string'` 这一句也是必需的：`route.query.redirect` 的类型是 `string | string[] | null`，重复传参会变成数组。

> 这条对应 spec §5.1 的最后两项验收。

### 3.2 错误处理：`unknown` 收窄 + 兜底文案

`request.ts` 把后端业务错误统一抛成 `ApiError`（`code` + `message`），网络不通时抛 `ApiError(-1, ...)`。视图侧要做的就是收窄后取 `message`：

```ts
try {
  await authStore.login({ username, password })
} catch (error) {
  errorMessage.value = error instanceof ApiError ? error.message : '登录失败，请稍后重试'
}
```

**为什么不能写 `catch (error: any)`**：TS `strict` 下禁止 `any`（[AGENTS §4.9](../../../AGENTS.md) / spec §4.3），而且 `instanceof` 收窄本来就更准确——它顺带证明了「这个错误确实是我们自己的 ApiError，身上一定有 `message`」。

不额外做「把 `code` 映射成中文」的表格：后端「一 code 一 msg」，`message` 已经是权威文案，前端再映射一遍就有两份真相（[AGENTS §4.9](../../../AGENTS.md) 也这么要求）。

### 3.3 校验规则为什么要在前后端各写一遍

前端校验**不是**安全措施，只是「省一次网络往返」的体验优化：本地能立刻告诉用户「用户名要 3-20 位」，不用等一个 RTT 再被后端拒。**正确性始终在后端**——前端拦得住的是普通用户的手误，拦不住构造请求的人。所以 spec §4.2 明确写了「两者冲突时以后端为准」，前端也不因为本地校验通过就假设一定成功。

### 3.4 「ASCII 可见字符」是密码校验里最容易写错的一条

契约 §3.1 要求密码 8-32 位且**限 ASCII 可见字符**（理由见契约与技术文档 §4.8：bcrypt 在 72 字节处截断）。两个常见错法：

- 用 `.length` 判长度 → 中文、全角字符也能通过（`'中文中文'` 长度是 4，但它不是 ASCII）；
- 只判「不含空格」→ 制表符、emoji 都能过。

正确做法是一条字符集正则：`/^[\x21-\x7E]{8,32}$/`（`\x21`–`\x7E` 正是「可打印的 ASCII 去掉空格」）。用户名同理：`/^[A-Za-z0-9]{3,20}$/`。

### 3.5 为什么不建 `api/auth.ts`

`stores/auth.ts` 里的 `login` / `register` / `refresh` **已经是**「认证接口的调用层」：它们调 `request.post<AuthResult>('/auth/login', payload)`，并把结果写进 state。再包一层 `api/auth.ts` 只会得到两个名字不同、职责重合的模块，调用方要记「注册页是调 api 还是调 store」。

**边界原则**：`api/` 放**跨模块共用的请求基础设施与端点封装**（目前只有 `request.ts`）；**只有一个 store 用的接口，就写在那个 store 里**。等某个端点被两个以上 store / 组件用到，再往上提。

### 3.6 为什么本支还不做 Mock（`VITE_USE_MOCK`）

[AGENTS §4.9](../../../AGENTS.md) 确实提到了「Mock 层与真实实现返回同样的 `data` 结构，靠 `VITE_USE_MOCK` 切换」，3-A 也留了口子。本支**仍然不做**，理由三条：

1. **范围**：本支的合同是「两个页面」，不是「前端能自证登录链路」。加 mock 等于把一支 UI 分支变成 UI + 假后端两支的合体。
2. **会制造两份真相**：mock 的响应结构与后端实现是并行演化的。真正危险的不是「mock 没写对」，而是**mock 先通过了**——等 PR B 落地才发现字段名/层级对不上，那时假的那份已经给了虚假的信心。
3. **如实标注比好看的通过率可靠**：spec §5.2 把「成功路径」诚实地留空、写明等 PR B 复验，比让它「本地跑绿」更符合项目一贯口径（auth-login spec §3.3 也是这么处理的）。

> **但这不是永久决定**：如果 PR B 拖延超过一周，mock 的性价比会反转（没有它，前端就无法验证「写登录态 → 消费 redirect → 跳转」这一段）。到那时单开一支做，并让 mock 与后端共用同一份类型定义来避免两份真相。

### 3.7 两个视图的 markup 为什么不先抽公共组件

注册页和登录页结构确实像（标题 + 表单 + 提交按钮 + 错误区），但**字段数、校验规则、成功后的去向都不同**。现在抽出来只会把「两个具体的页面」变成「一个抽象组件 + 两个配置参数」，抽象边界是在只有两处用例时猜的。等**第三处**复用出现（改密码页，也在同一个卡片外壳里），边界才看得清，那时再抽。这是「先等三个，再抽象」的常规做法。

---

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| 后端 PR B（`/auth/{register,login}`）未交付 | 「提交成功」链路无法验证 | spec §5.2 如实留空不勾 ✅；本支只宣布 §5.1 完成 |
| **全队无 PostgreSQL**（成员 1 无 Docker/PG，成员 2 本机亦无） | 即使 PR B 落地，仍无人能端到端验证 | 在群里挑明这是本周里程碑的头号风险；仓库已有 `deploy/docker-compose.dev.yml`，谁装 Docker 谁把库起来 |
| `?redirect=` 写成开放重定向 | 用户被送到钓鱼页 | §3.1 的 origin 比对 + spec §5.1 两条专项验收 |
| 前端校验被误当成安全措施 | 后端若漏校验则成漏洞 | spec §4.2 写明正确性在后端；前端只做提前拦截 |
| 表单值进日志 / 进 URL | 密码泄露 | 不打印表单值；提交走后端 POST body，绝不把密码放 query |
| `catch` 里用 `any` | 破坏 `strict`，且掩盖真实类型 | §3.2 用 `instanceof ApiError` 收窄 |

---

## 5. 进度记录

| 日期 | 步骤 | 说明 |
|------|------|------|
| 2026-09-18 | 1 | 写 `spec.md` + `plan.md`；分支自 `develop`（`8c15c00`）切出 |

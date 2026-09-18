# 登录与注册页（frontend-auth-pages）· 规格说明

> 本文件是**合同**：定义本分支做什么、做到什么算完成。合并后即冻结，行为变更须升版本并在群里广播。
> 按 [AGENTS §5.2](../../../AGENTS.md) 要求，本文只写**本功能独有**的内容；通用约定一律给链接，不重复描述。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-auth-pages` |
| 状态 | 🚧 进行中 |
| 依赖分支 | `feature/frontend-router-layout`（已合并，PR #31） |
| 关联契约 | [API_CONTRACT](../../API_CONTRACT.md) §3.1 / §3.2 |
| 关联设计 | [TECH_DESIGN](../../TECH_DESIGN.md) §3 |
| 后端依赖 | `auth-login` 的 **PR B**（`/auth/{register,login}`）——**未交付** |

---

## 1. 背景与目标

前三支把「数据长什么样」（`types/`）、「怎么发请求、怎么记住登录」（`api/request.ts` + `stores/auth.ts`）、「哪个地址显示哪个页面」（`router/` + `MainLayout`）都铺好了，但**全站没有一个能输入账号密码的地方**：`/login`、`/register` 两条路由在 3-B 时指向内联占位组件，打开只有「待实现」三个字。

本支把这两页做出来，让「注册 → 进入应用」和「登录 → 进入应用」这两条**人的入口**可用。

**同时还一笔债**：3-B 的守卫在拦截时会写 `?redirect=<原地址>`，但当时没有任何页面消费它。本支的 `LoginView` 必须把它读回来并跳过去，否则这个参数形同虚设（3-B `plan.md` §3.4 已记下）。

**目标（可验证）**：未登录访问 `/chat` → 落到 `/login?redirect=%2Fchat` → 登录成功后**回到 `/chat`**，而不是无脑跳首页。

---

## 2. 范围

### 2.1 做什么（In Scope）

| 文件 | 职责 |
|------|------|
| `frontend/src/views/auth/LoginView.vue` | 新增 · 登录表单 + 校验 + 提交 + 错误提示 + 消费 `?redirect=` |
| `frontend/src/views/auth/RegisterView.vue` | 新增 · 注册表单 + 校验 + 提交 + 错误提示；注册成功即登录 |
| `frontend/src/router/index.ts` | 修改 · `/login`、`/register` 两条路由的 `component` 由 `Placeholder` 换成真实懒加载 import |
| `docs/specs/frontend-auth-pages/{spec,plan}.md` | 新增 · 本文与实施计划 |

合计 **3 个源文件**（2 新增 + 1 修改）+ 2 篇文档。

### 2.2 不做什么（明确排除，防止范围蔓延）

- ❌ `views/profile/{ProfileView,PasswordView}.vue` → 依赖 `auth-login` 的 **PR C**（`/user/profile`、`/user/password`），另开一支
- ❌ `src/api/auth.ts` → **不需要建**。`stores/auth.ts` 的 `login` / `register` / `refresh` 已经是这一层的全部内容，理由见 `plan.md` §3.5
- ❌ **Mock 层**（`VITE_USE_MOCK`）→ [AGENTS §4.9](../../../AGENTS.md) 提到过这个开关，但 `frontend-auth-request` 已显式推迟，本支同样不实现，理由见 `plan.md` §3.6
- ❌ 忘记密码 / 邮箱验证 / 验证码 / 第三方登录 → 契约 §3 无此端点
- ❌ 头像**文件上传** → 契约 §3.5 只存 URL
- ❌ 视觉打磨（品牌插画、动效、窄屏适配）→ Week 4 UI 分支
- ❌ `stores/auth.ts`、`api/request.ts`、`router/guards.ts` 的改动 → 均已合并，本支只**消费**

---

## 3. 依赖与交接

### 3.1 我依赖谁

| 依赖 | 状态 |
|------|------|
| `stores/auth.ts` 的 `login` / `register` / `user` / `isLogin` | ✅ 已合并（PR #24） |
| `request.ts` 的解包行为与 `ApiError` 语义 | ✅ 已合并（PR #24） |
| `/login`、`/register` 路由与守卫写的 `?redirect=` | ✅ 已合并（PR #31） |
| 后端 `POST /auth/register`、`POST /auth/login` | ❌ **未交付**（`auth-login` 的 PR B）→ 见 §5 与 `plan.md` §3.6 |

### 3.2 谁依赖我

| 依赖方 | 用我的什么 |
|--------|-----------|
| 成员 1（联调） | 有了可提交的表单，`/auth/login` 落地后可直接对着页面调 |
| 后续登录态分支（画像页 / 改密码页） | `?redirect=` 的消费范式 |
| 答辩演示 | 「注册 → 进应用」这条主链路的前半段 |

---

## 4. 硬性约束

1. **必须消费 `?redirect=`，且只接受站内绝对路径** —— 以 `/` 开头、**且不以 `//` 开头**。否则 `?redirect=https://evil.com` 会变成**开放重定向**：地址栏还是你自己发出去的链接，人却已经被送到攻击者的钓鱼页。判定规则见 `plan.md` §3.1。
2. **注册页的校验规则与 [API_CONTRACT §3.1](../../API_CONTRACT.md) 的校验列保持一致**，但**前端不承担正确性**——它只是「省一次往返」的提前拦截，后端仍会校验并返回 `4001`。两者冲突时以后端为准。**登录页只校验「非空」，不重复格式规则**，理由见 `plan.md` §3.8。
3. **不许出现 `any`**：`catch` 到的是 `unknown`，必须 `instanceof ApiError` 收窄后才能读 `message`。
4. **三态齐全**：loading（提交中禁用按钮 + 文案变化）/ error（错误提示）/ empty（表单未通过校验时不可提交）。见 [AGENTS §4.9](../../../AGENTS.md)。
5. **注册即登录**：`register()` 成功后直接进应用，**不得**再把用户丢回登录页登一次（契约 §3.1 的显式要求）。
6. **视图里不判断 `res.code`** —— `request.ts` 已解包，组件拿到的就是契约里的「响应 data」。见 [AGENTS §4.9](../../../AGENTS.md)。
7. **错误文案优先用后端 `message`**，仅在网络不通（`NETWORK_ERROR_CODE`）这类没有 message 的场景用兜底文案。
8. **不引入新依赖**，表单沿用 Element Plus。
9. 注释遵守 [AGENTS §5.1](../../../AGENTS.md)。

---

## 5. 验收标准

> ⚠️ 后端 `/auth/{register,login}` 尚未交付（`auth-login` PR B）。**「提交成功」那一段链路本支无法真实验证**，一律标注「待 PR B 就位后复验」，**不勾 ✅**。宁可留空，不把没跑过的算成通过（沿用 auth-login spec §3.3 的口径）。

### 5.1 本机可完整验证

- [ ] `npm run typecheck`（`vue-tsc --noEmit`）零错误
- [ ] `npm run build` 通过
- [ ] 访问 `/login` 渲染出用户名 / 密码输入框与提交按钮（不再是「待实现」）
- [ ] 访问 `/register` 渲染出用户名 / 邮箱 / 密码输入框与提交按钮
- [ ] 登录页：用户名或密码为空 → 提交按钮处于禁用状态（empty 态），点不动、不发请求
- [ ] 登录页：**不做**密码格式校验——非法值（7 位 / 含中文）照常提交给后端，由后端返回 `4001` / `4013`（理由见 `plan.md` §3.8）
- [ ] 注册页：用户名 2 位 / 邮箱无 `@` / 密码 7 位 / 密码含中文 → 逐条给出校验提示，**不发请求**
- [ ] 提交中：按钮禁用并显示 loading，无法重复提交
- [ ] 后端不可达时（当前必然如此）→ 展示兜底错误文案，页面不崩、`console` 无未捕获异常
- [ ] 未登录访问 `/chat` → 地址栏变为 `/login?redirect=%2Fchat`
- [ ] 已登录访问 `/login` → 被守卫弹回 `/chat`（3-B 行为，本支不得回归）。造登录态的手法：Console 执行 `localStorage.setItem('heart-echo-auth', JSON.stringify({ accessToken: 'fake', refreshToken: 'fake', user: { id: 1, username: 'x', email: 'x@x.com', avatarUrl: null } }))` 后刷新
- [ ] `resolveRedirect` 对 `https://evil.com`、`//evil.com`、`/\evil.com` 一律返回 `/chat`，对站内路径保留 `pathname + search`（逻辑层验证，函数体与 4 组输入见 `plan.md` §3.1）
- [ ] 本支共 3 个源文件 + 2 篇文档，`views/profile/` 目录不存在

### 5.2 待后端 PR B 就位后复验

- [ ] 登录成功 → 写入登录态（`isLogin === true`、`user` 有值）→ 跳 `?redirect=` 指定页或 `/chat`
- [ ] 携带 `?redirect=` 登录成功后**真的落在目标页**；`?redirect=https://evil.com` 时落在 `/chat`（端到端跳转需要真实登录态，本机走不通）
- [ ] 登录失败（`4013`）→ 展示后端文案「用户名或密码错误」
- [ ] 注册成功 → 直接进应用（**注册即登录**），不回登录页
- [ ] 邮箱重复 `4003` / 用户名重复 `4004` → 展示对应文案

---

## 6. 变更记录

| 日期 | 版本 | 变更内容 | 改动人 |
|------|------|----------|--------|
| 2026-09-18 | v1 | 初始版本 | 成员 2 |
| 2026-09-18 | v1.1 | 修正 §4.2 与 §5.1：登录页**不做**密码格式校验（只校验非空），格式规则只属于注册页。原因见 `plan.md` §3.8 | 成员 2 |
| 2026-09-18 | v1.2 | 修正 §5.1 分类错误：原「`?redirect=` → **登录后**落在 `/chat`」一项含「登录后」，而登录链路本机走不通（§5 开头已自述），与本节自相矛盾。现本机只留 `resolveRedirect` 的逻辑层验证，端到端那半移入 §5.2；§5.1 另补「手工造登录态」的验证手法 | 成员 2 |

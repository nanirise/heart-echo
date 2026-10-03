# 账号资料与改密（frontend-profile-settings）· 规格说明

> 本文件是**合同**：定义本分支做什么、做到什么算完成。合并后即冻结，行为变更须升版本并在群里广播。
> 按 [AGENTS §5.2](../../../AGENTS.md) 要求，本文只写**本功能独有**的内容；通用约定一律给链接，不重复描述。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-profile-settings` |
| 状态 | 🚧 进行中 |
| 依赖分支 | 无（自 `develop` `09a0e7b` 直接切出） |
| 关联契约 | [API_CONTRACT](../../API_CONTRACT.md) §3.4 / §3.5 / §3.6 |
| 关联设计 | [TECH_DESIGN](../../TECH_DESIGN.md) §3 |
| 后端依赖 | `auth-login` 的 **PR C** —— ✅ **已合并**（PR #50，合并点 `40fc87a`，2026-09-30） |

---

## 1. 背景与目标

`/profile` 是 3-B 预留的路由，一直指向内联占位组件，打开只有「待实现」三个字。它同时也是 [MASTER §1](../../dev/MASTER.md) 边界表里成员 2 的**最后一行**。

前三支一直欠着它，是因为后端没有端点。`auth-login` 的 PR C 于 2026-09-30 合并（PR #50）：`GET /user/profile`、`PUT /user/profile`、`PUT /user/password` 三条就位，并已通过端到端冒烟（30 余条请求含 20 余条负例，0 ERROR / 0 500）。依赖解除。

**目标（可验证）**：登录后访问 `/profile`，能改自己的用户名与头像链接、能改密码；改密成功后回到登录页重新登录。

### 1.1 为什么本支只做「账号资料」，不做「AI 画像」

`views/profile/` 这一个目录下装的是**两件不同的事**：

| | 数据来源 | 本支能否做 |
|---|---------|-----------|
| 账号资料 | `GET/PUT /user/profile`、`PUT /user/password` | ✅ 契约完整、后端已实现 |
| AI 画像 + 记忆 | `GET /profile/portrait?personaId=`、`GET /memory?personaId=` | ❌ 做不了 |

后者做不了，**不是「等后端排期」那么软**。[契约 §7](../../API_CONTRACT.md) 里它的响应是：

```json
{ "personaId": 1, "profileData": {}, "updatedAt": "..." }
```

`profileData` 是**空对象**，字段一个都没约定。前端不知道要渲染什么；Mock 也编不出可信的结构——编出来的必然与后端将来给的不一致，届时整块返工。**等契约 §7 把 `profileData` 的结构补齐后再另开支**，本支不做。

---

## 2. 范围

### 2.1 做什么（In Scope）

| 文件 | 职责 |
|------|------|
| `frontend/src/views/profile/ProfileView.vue` | 新增 · 账号资料卡片：头像预览 + 用户名输入 + 保存；按钮打开改密对话框 |
| `frontend/src/views/profile/PasswordDialog.vue` | 新增 · 改密对话框：原密码 + 新密码 + 确认；成功后清登录态并跳登录页 |
| `frontend/src/stores/auth.ts` | 修改 · 新增 `fetchProfile` / `updateProfile` / `changePassword` 三个 action 与 `ProfilePatch` 类型 |
| `frontend/src/router/index.ts` | 修改 · `/profile` 的 `component` 由 `Placeholder` 换成真实懒加载 import（1 行） |
| `docs/specs/frontend-profile-settings/{spec,plan}.md` | 新增 · 本文与实施计划 |

合计 **4 个源文件**（2 新增 + 2 修改）+ 2 篇文档。`views/profile/` 目录本支首次创建。

**不改路由结构**：不新增路由条目、不动 `children` 的层级与 `meta`。

### 2.2 不做什么（明确排除，防止范围蔓延）

- ❌ **AI 画像展示**与**记忆查看** → 见 §1.1
- ❌ 头像**文件上传** → [契约 §3.5](../../API_CONTRACT.md) 只存 URL
- ❌ **邮箱修改** → 契约 §3.5 的请求体只接受 `username` / `avatarUrl` 两个键，传 `email` 是 `4001`
- ❌ 忘记密码 / 重置密码 / 邮箱验证 / 验证码 → 契约 §3 无此端点
- ❌ `request.ts` / `router/guards.ts` 的改造（含 4012 刷新重放）→ 均已合并，本支只**消费**
- ❌ 视觉打磨（品牌插画、动效、窄屏适配）→ Week 4 UI 分支
- ❌ 给 `auth` 通道做 Mock（`VITE_USE_MOCK`）→ 理由见 [`plan.md` §3.6](plan.md)

---

## 3. 依赖与交接

### 3.1 我依赖谁

| 依赖 | 状态 |
|------|------|
| 后端 `GET /user/profile`、`PUT /user/profile`、`PUT /user/password` | ✅ 已交付（PR #50，合并点 `40fc87a`） |
| `stores/auth.ts` 的 `user` / `setUser` / `clearAuth` | ✅ 已合并（PR #24） |
| `api/request.ts` 的解包行为与 `ApiError` 语义 | ✅ 已合并（PR #24） |
| `api/error.ts` 的 `toErrorMessage` | ✅ 已合并（PR #49） |
| `/profile` 路由与 `MainLayout` 外壳 | ✅ 已合并（PR #31 / #39） |
| **一个能连上的后端实例** | ⚠️ **本机没有**（无 Go / Docker / PostgreSQL）→ 见 §5 |

### 3.2 谁依赖我

| 依赖方 | 用我的什么 |
|--------|-----------|
| 成员 1（联调） | 有了可提交的资料表单与改密对话框，可对着页面复验 PR C 的前端侧 |
| 将来的画像支 | `/profile` 页面的外壳与「账号资料」区块 |
| 答辩演示 | 「改密码 → 重新登录」这条安全链路的前端一半 |

---

## 4. 硬性约束

1. **`PUT /user/profile` 是严格解码端点**（[契约 §3.5](../../API_CONTRACT.md)）：请求体里出现 `username` / `avatarUrl` 以外的键，一律 `4001`，不忽略。**绝不能把 `authStore.user` 整个对象 PUT 上去**——`id` / `email` / `createdAt` 在那里都是「不认识的键」。只发**改动过的**字段。
2. **`avatarUrl: null` 是一个有意义的值**（清空头像），与「不传这个键」（不要动它）语义完全不同。判「用户改没改」必须用**存在性判断**（`!== undefined` 一类），**不能用 `if (avatarUrl)` 一类的真值判断**——那样「清空头像」会被静默丢掉。
3. **`patch` 的键集合由类型锁死**：`updateProfile` 的入参类型是 `ProfilePatch`（只有 `username` / `avatarUrl` 两个可选键），让「只发这两个键」由编译器保证，而不是靠人记得。
4. **改密成功后必须清登录态并跳登录页**——[契约 §3.6](../../API_CONTRACT.md) 明写服务端**不主动失效旧 Token**，这半件事由前端负责。跳转放在视图层（与 `logout()` 只清状态、不管跳转的既有分工一致）。
5. **三态齐全**：loading（提交中按钮禁用 + 文案变化）/ error（错误提示）/ empty（表单未通过校验时不可提交）。见 [AGENTS §4.9](../../../AGENTS.md)。
6. **视图里不判断 `res.code`** —— `request.ts` 已解包，组件拿到的就是契约里的「响应 data」。
7. **错误文案优先用后端 `message`**，只在请求没到后端时用兜底文案——复用 `api/error.ts` 的 `toErrorMessage`，不要自己再写一份。
8. **不许出现 `any`**：`catch` 到的是 `unknown`，收窄后才读 `message`。
9. **不改路由结构**：本支只把 `/profile` 的 `component` 换掉。
10. 注释遵守 [AGENTS §5.1](../../../AGENTS.md)。

---

## 5. 验收标准

> 与上一支不同：这次**后端已交付**，但**本机连不上**（无 Go / Docker / PostgreSQL）。所以按「本机可验」与「需联调复验」分开列，后者**不勾 ✅**——沿用 `auth-login` spec §3.3 与 `frontend-auth-pages` spec §5 的口径：宁可留空，不把没跑过的算成通过。

### 5.1 本机可完整验证

- [ ] `npm run typecheck`（`vue-tsc --noEmit`）零错误
- [ ] `npm run build` 通过
- [ ] 访问 `/profile` 渲染出资料表单（不再是「待实现」）
- [ ] 未登录访问 `/profile` → 落到 `/login?redirect=%2Fprofile`
- [ ] 头像预览：`avatarUrl` 为空时显示用户名首字色块，与左侧栏的兜底一致
- [ ] 点「修改密码」→ 对话框打开；原密码或新密码为空时提交按钮禁用（empty 态）
- [ ] `newPassword` 7 位 / 含中文 / 含全角 → 逐条给出校验提示，**不发请求**
- [ ] 新密码与确认框不一致 → 给出提示，**不发请求**
- [ ] 后端不可达时 → 展示兜底文案，页面不崩、`console` 无未捕获异常
- [ ] **`PUT /user/profile` 的 payload 只含改动过的键**（DevTools › Network › Payload）：只改用户名时是 `{"username":"..."}`，**不含 `id` / `email` / `createdAt`** ★
- [ ] **清空头像时 payload 是 `{"avatarUrl":null}`** —— 不是 `{}`、不是空字符串 ★
- [ ] 一个字段都没改时点保存 → **不发请求**，或明确提示「没有需要保存的改动」
- [ ] 本支共 4 个源文件 + 2 篇文档，`router/index.ts` 未新增路由条目

> ★ 这两条**即使后端连不上也能验**：请求虽然会失败（`net::ERR_CONNECTION_REFUSED`），但 DevTools 里该请求的 Payload 仍然看得到。本支最大的风险是「顺手把整个 user 对象 PUT 上去」，这两条正是钉它的。
>
> 造登录态的手法沿用前支（[`frontend-auth-pages/spec.md`](../frontend-auth-pages/spec.md) §5.1）：Console 执行
> ```js
> localStorage.setItem('heart-echo-auth', JSON.stringify({ accessToken: 'fake', refreshToken: 'fake', user: { id: 1, username: 'x', email: 'x@x.com', avatarUrl: null } }))
> ```
> 后刷新。存储 key 与 `stores/auth.ts` 的 `persist` 配置一致，写错会静默无效。
> ⚠️ `VITE_USE_MOCK` **不覆盖 `auth`**，所以本机只能靠这一招进页面。

### 5.2 需连上后端后复验（不勾 ✅）

- [ ] 改用户名成功 → `200`，表单回填新用户名、左侧栏同步更新
- [ ] 用户名撞已有账号 → `4004`，展示「用户名被占用」类文案
- [ ] 新用户名 2 位 → `4001`
- [ ] 改头像 URL 成功 → 预览换成新图，刷新后仍在
- [ ] 清空头像成功 → 预览回到首字色块
- [ ] 改密：原密码错 → `4015`，展示「原密码不正确」
- [ ] 改密成功 → 清登录态并落到 `/login`；**新密码**能登录、**老密码**返回 `4013`（证明真的落库了）
- [ ] 带着失效 token 访问 `/profile` → 被拦截器按 `4010` / `4011` 清登录态并跳登录页

---

## 6. 变更记录

| 日期 | 版本 | 变更内容 | 改动人 |
|------|------|----------|--------|
| 2026-10-02 | v1 | 初始版本 | 成员 2 |

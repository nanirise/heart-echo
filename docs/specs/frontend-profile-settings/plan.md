# 账号资料与改密（frontend-profile-settings）· 实施计划

> 配套合同：[spec.md](spec.md)。本文只记录**怎么做**与**为什么这么做**。
> 注释纪律见 [AGENTS §5.1](../../../AGENTS.md)：代码里只留「为什么」，讲解留在本文。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-profile-settings` |
| 状态 | ✅ 已完成（待合并） |
| 依赖 | `auth-login` PR C（已合并，PR #50） |
| 规模 | 4 个源文件（2 新增 + 2 修改）+ 2 篇文档 |

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 状态 |
|---|------|------|:----:|
| 1 | 写 `spec.md` 与 `plan.md` | 2 篇文档 | ✅ |
| 2 | `stores/auth.ts` 加 3 个 action + `ProfilePatch` | 修改 | ✅ |
| 3 | `views/profile/ProfileView.vue` | 新增 | ✅ |
| 4 | `views/profile/PasswordDialog.vue` | 新增 | ✅ |
| 5 | 路由表换 `component`（1 行） | 改 `router/index.ts` | ✅ |
| 6 | 自查：`typecheck` / `build` / 浏览器逐条过 spec §5.1 | 证据 | ✅ 本机项见 spec §5.1；需联调的 7 项按约定留空 |
| 7 | 提交：文档两笔、代码一笔 | 3 笔 | ✅ |

**顺序理由**（两条都是踩过的）：

1. **第 5 步必须在第 3 步之后**——路由表引用的文件不存在时 `vue-tsc` 报 TS2307、构建直接失败。`frontend-auth-pages/plan.md` §1 记过同一个坑。
2. **第 2 步必须在第 3、4 步之前**——两个视图都要用 `updateProfile` / `changePassword` 与 `ProfilePatch` 类型。先定接口，视图才有东西可调；反过来写会出现「视图里先手搓一个 fetch，回头再搬进 store」的返工。

---

## 2. 文件清单

| 文件 | 类型 | 说明 |
|------|------|------|
| `frontend/src/stores/auth.ts` | 修改 | 新增 `ProfilePatch` / `ChangePasswordPayload` 类型与 `fetchProfile` / `updateProfile` / `changePassword` 三个 action |
| `frontend/src/views/profile/ProfileView.vue` | 新增 | 账号资料卡片；只提交改动过的字段 |
| `frontend/src/views/profile/PasswordDialog.vue` | 新增 | 改密对话框 |
| `frontend/src/router/index.ts` | 修改 | `/profile` 的 `component` 由 `Placeholder` 换成 `() => import(...)` |
| `docs/specs/frontend-profile-settings/spec.md` | 新增 | 合同 |
| `docs/specs/frontend-profile-settings/plan.md` | 新增 | 本文 |

`views/profile/` 目录本支首次创建。**不新增 `api/profile.ts`**（理由见 §3.2），**不新增 `types/profile.ts`**（理由见 §3.8）。

---

## 3. 关键实现要点

### 3.1 「这个字段用户改了吗」有三个答案，不是两个（本支最容易写错的地方）

后端只接受两种状态：**不要动这个字段**、**把这个字段改成某个值**。但「改成某个值」里面还藏着一个特例——**清空**（`avatarUrl` 从有到无）。于是实际是三种：

| 意图 | 请求体里应该长什么样 |
|------|---------------------|
| 不要动头像 | `avatarUrl` 这个**键根本不出现** |
| 把头像改成某个 URL | `{"avatarUrl": "https://..."}` |
| 把头像**清空** | `{"avatarUrl": null}` |

对应到 JS 侧：

```
undefined（或键不存在） → 序列化时被整个丢掉 → 后端「不传即不改」✅
null                    → 序列化成 null        → 后端「清空」✅
''（空字符串）          → 序列化成 ""          → 后端当成非法 URL → 4001 ❌
```

两个必须注意的点：

1. **`JSON.stringify` 会丢掉值为 `undefined` 的键**——`JSON.stringify({ a: undefined })` 得到 `{}`。这正好是后端要的语义，但也意味着**一旦把 `null` 误写成 `undefined`，「清空头像」就变成了「不改头像」，而且不报错、不提示**。用户点保存、看到「保存成功」、刷新一下头像又回来了。
2. **输入框清空得到的是空字符串，不是 `null`**。`<input v-model="form.avatarUrl">` 里删光内容是 `''`，直接塞进 patch 会变成 `{"avatarUrl": ""}`，被后端判非法（契约 §3.5 只定义了 URL 与 `null` 两态）。所以必须写一层转换。

**推荐的判定写法**：

```ts
const patch: ProfilePatch = {}
const current = authStore.user

if (current) {
  const username = form.username.trim()
  if (username !== current.username) patch.username = username

  const avatarUrl = form.avatarUrl.trim()
  // ?? '' 是为了把 store 里的 null 与输入框的 '' 归一到同一种「空」，否则没改也会被判成改过
  if (avatarUrl !== (current.avatarUrl ?? '')) {
    patch.avatarUrl = avatarUrl === '' ? null : avatarUrl
  }
}

if (Object.keys(patch).length === 0) return
```

两个细节都不是可选的：

- `?? ''`：store 里 `avatarUrl` 可能是 `null`，而输入框的值是 `''`。不归一化的话，**用户什么都没动，程序也会认为「从 null 变成了空字符串」而发出一次 `{"avatarUrl":null}`**——请求看着无害，但它凭实力证明了「我的比较逻辑是错的」，将来一旦这个字段的语义变了就会出真问题。
- 空对象直接 return：`{}` 是**合法**请求（契约 §3.5 明写），但发它是白跑一次往返。更重要的是，用户点「保存」却什么都没改时，应该得到「没有改动」这个反馈，而不是一个来自后端的 `200`。

> 这条对应 spec §4.1 / §4.2 与 §5.1 的两条 ★ 验收。

### 3.2 为什么 `updateProfile` 写在 store 里，而不是新建 `api/profile.ts`

`/user/profile` 操作的对象是「**当前登录用户**」，而「当前登录用户是谁」的唯一权威就是 `stores/auth.ts`。如果建 `api/profile.ts`，它要么反过来 import store（层依赖倒挂），要么让视图先读 store 再拼给 api——**「拼接身份」这件事就漏到视图层了**，正是要避免的。

而且响应 data 就是新的 `UserInfo`，更新完必须写回 store，否则左侧栏的用户名不会同步。「发请求 + 写回状态」这个组合动作天然属于 store。

按 `frontend-auth-pages/plan.md` §3.5 定下的边界：`api/` 放**跨模块共用的请求基础设施与端点封装**；**只有一个 store 用的接口，就写在那个 store 里**。本支照此办理。

### 3.3 为什么进页面还要重新 `GET /user/profile`

store 里已经有 `user` 了（登录时写入，并 persist 到 localStorage）。理论上直接渲染就行，但仍然拉一次，三个理由：

1. **本地那份可能过期**。persist 的内容是**上次登录那一刻**的快照。用户若在别的设备上改过名，本地显示的就是旧的——而这是一个「查看并编辑自己资料」的页面，显示旧值是最难解释的一种 bug。
2. **契约 §3.4 存在就是为了这个**。前端若永远只读本地缓存，这个端点在前端侧就是死代码。
3. **它是本支第一个「进页面就要鉴权的读接口」**，`4010` / `4011` 的登出分支会在这里第一次真的被跑到。顺带说明一处容易被误连的结论：本支三个请求都走 `request.ts`，所以 **`4012` 的刷新重放是自动生效的**——这与 [`frontend-chat-view/plan.md`](../frontend-chat-view/plan.md) §3.7 记的那个缺口不同，那里是因为 `streamChat` 为了真流式而**绕开了** `request.ts`，属于流式请求特有的问题，普通请求不受影响。

**代价与保护**：多一次往返，且要处理 loading。**拉取失败不阻塞渲染**——用 store 里的缓存兜底 + 一条提示，否则后端挂了会导致设置页整个打不开。

### 3.4 改密为什么做成对话框，不做独立页面

1. 任务书 [MEMBER_2_FRONTEND.md §4](../../dev/MEMBER_2_FRONTEND.md) 原文就是「`PasswordView.vue`（**或弹窗**）」，两种都在授权范围内。
2. 独立页面要新增路由条目，而路由表是**共享文件**——[MASTER §4.5](../../dev/MASTER.md) 明写「不要两人同时改这个文件」。本支对路由表只有 1 行改动，风险面最小。
3. 改密是低频短流程（两个输入框 + 一个按钮），对话框足够装。
4. 先例：成员 3 的 `views/persona/PersonaFormDialog.vue` 就是放在 `views/<模块>/` 下的对话框组件，命名与位置一致。

### 3.5 「确认新密码」绝不能进请求体（一个现在不报错、将来会炸的坑）

契约 §3.6 的请求体只有 `oldPassword` / `newPassword` 两个键。确认框是**纯前端**字段，只用于防手误。

危险写法：

```ts
// ❌ form 里含 confirmPassword
await authStore.changePassword(form)
```

后端会收到 `{"oldPassword":..., "newPassword":..., "confirmPassword":...}`。而 §3.6 **不是**严格解码端点（严格解码目前只加在 §3.5 的 `PUT /user/profile`），所以这个多余的键会被**静默忽略、返回 200**——看起来一切正常，实际上前端一直在发一个不该发的字段。等哪天有人给改密也加上严格解码，这一支立刻全线 `4001`，而且从现象上完全看不出与今天的关联。

**对策**：显式挑字段构造 payload，并且让 store 的入参类型**只声明两个键**——由类型挡住，而不是靠人记得。

```ts
await authStore.changePassword({
  oldPassword: form.oldPassword,
  newPassword: form.newPassword,
})
```

### 3.6 为什么本支仍然不做 Mock

`frontend-auth-pages/plan.md` §3.6 当时列了三条不做 Mock 的理由。本支情况**变了**（后端已交付），所以要重新审一遍，不能照抄结论：

**支持做 Mock 的两条新理由（如实列出）**：

1. 契约 §3.4 / §3.5 / §3.6 的响应结构**是完整的**——不像 §7 的 `profileData` 是个空对象。Mock 不会编错结构。
2. 本机没有 PostgreSQL，跑不了真后端。

**但仍然不做**，两条理由：

1. **机制上做不了，且代价超出本支范围**。`VITE_USE_MOCK` 目前只覆盖 `chatApi` 与 `listPersonas`，走的是「`api/mock/index.ts` 按开关二选一导出」这条通道。而 `stores/auth.ts` 是**直接调 `request.post`** 的，没有可替换的间接层——要 Mock 就得先给 auth 抽一层 `api/auth.ts`，**那是重构，不是本支范围**；而且 `frontend-auth-pages/plan.md` §3.5 已经明确论证过「不建 `api/auth.ts`」，本支不该推翻它。
2. **本支最大的风险本机已经能验**。本支最怕的是「把多余的键 PUT 上去」，而 DevTools 的 Request Payload 在后端连不上时**照样看得见**（spec §5.1 那两条 ★）。Mock 能多证明的只有「成功后的回填」——那条等一次联调就能结。

→ 结论：如实把成功路径留在 spec §5.2 不勾 ✅，比造一条假通道更符合项目一贯口径。

### 3.7 头像的兜底色块 —— 契约要求，但项目里还没有先例

契约 §3.5 明写「传 `null` 可清空，**前端用用户名首字母色块兜底**」，所以这不是可选项，本支必须做。

但项目现状是：**全项目没有一处渲染过 `avatarUrl`**——`MainLayout.vue` 的导航侧栏只显示用户名文本，`ChatView.vue` 的人设侧栏只显示名字与亲密度（那里唯一的圆形是未读红点，不是头像）。所以本支是**第一次**定义头像的展示方式。

因此这里不存在「抽不抽公共组件」的问题：没有可复用的实现，也没有第二处用例。等真出现第二个使用者（比如人设头像、消息气泡头像）再谈抽象。

**取首字用 `charAt(0)` 是安全的**：契约 §3.1 把 username 限死为「3-20 位字母数字」，纯 ASCII，不存在 emoji 或其它非 BMP 字符取到半个代理对的问题。换个字段（比如将来允许 emoji 的昵称）就不能这么写了。

### 3.8 为什么不建 `types/profile.ts`

本支要用的两个类型是 `UserInfo`（已由 `stores/auth.ts` 导出）与 `ProfilePatch`（本支新增，**只有 `updateProfile` 一个消费者**）。

`types/` 的定位是**契约冻结层的镜像**——`types/api.ts` / `errcode.ts` / `persona.ts` / `chat.ts` 都是「后端 JSON 长什么样」的逐字映射，改动要跟着契约走。而 `ProfilePatch` 是**请求侧的入参约型**，不是响应结构，且只服务一个 action；放进 `types/` 会让「契约镜像」这个定位变模糊。

**规则**：只有一个消费者的类型，跟它的消费者放一起（`UserInfo`、`LoginPayload`、`RegisterPayload` 在 `stores/auth.ts` 里都是这么放的，本支延续）。等出现第二个消费者再往上提。

---

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| 顺手 PUT 整个 user 对象 | 每次保存都 `4001`，现象像「后端坏了」 | §3.1 的类型锁 + spec §5.1 两条 ★ 验收 |
| 清空头像被静默丢掉（`null` 写成 `undefined`） | 用户以为清掉了，刷新又回来，且全程无报错 | §3.1 存在性判断 + 空串转 `null` |
| 输入框空串直接进 payload | `{"avatarUrl":""}` → `4001` | §3.1 的 `?? ''` 归一化与三态转换 |
| 「确认密码」混进请求体 | 现在静默通过，将来加严后全线 4001，且看不出关联 | §3.5 显式挑字段 + store 入参类型只声明两个键 |
| 本机无后端 → 真实验证缺通路 | 成功路径无法勾 ✅ | spec §5.1 / §5.2 分开，后者留空不勾 |
| 改密后忘记清登录态 | 用户拿旧 token 继续用，以为改密没生效 | spec §4.4 契约依据 + §5.2 专项验收 |
| `catch` 里用 `any` | 破坏 `strict`，掩盖真实类型 | 复用 `toErrorMessage`，或 `instanceof ApiError` 收窄 |
| 拉取资料失败导致页面打不开 | 后端一挂设置页就白屏 | §3.3 失败不阻塞渲染，用 store 缓存兜底 |

---

## 5. 进度记录

| 日期 | 步骤 | 说明 |
|------|------|------|
| 2026-10-02 | 1 | 写 `spec.md` + `plan.md`；分支自 `develop`（`09a0e7b`）切出 |
| 2026-10-03 | 2–4 | 三个源文件落地：`stores/auth.ts`（3 个 action + `ProfilePatch` / `ChangePasswordPayload`）、`ProfileView.vue`、`PasswordDialog.vue` |
| 2026-10-03 | 5 | `router/index.ts` 仅改 1 行：`component` 由 `Placeholder` 换成 `() => import('@/views/profile/ProfileView.vue')` |
| 2026-10-03 | 6 | `vue-tsc --noEmit` 0 错误；`vite build` 通过（21.87s），产物含独立 `ProfileView-*.js` chunk，懒加载分割生效 |
| 2026-10-03 | — | 本文 §3.7 修正：原文称「`MainLayout.vue` 侧栏已有头像兜底色块」，核实后全项目没有任何 `avatarUrl` 渲染，已重写 |
| 2026-10-03 | 7 | 三笔提交：`0713ec8`（spec+plan）／`3b68ad0`（本文 §3.7 修正）／`4e632e9`（代码） |

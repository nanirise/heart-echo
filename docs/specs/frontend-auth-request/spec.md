# 前端请求层与登录态（frontend-auth-request）· 规格说明

> 本文件是**合同**：定义本分支做什么、做到什么算完成。
> 合并后即冻结，任何字段或行为变更必须升版本并在群里广播。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-auth-request` |
| 状态 | ⬜ 进行中 |
| 依赖分支 | `feature/frontend-api-types`（已合并 40e9105） |
| 关联契约 | `docs/API_CONTRACT.md` §1 §2 §3 |

---

## 1. 背景与目标

上一分支（`frontend-api-types`）只解决了「数据长什么样」——定义了 `ApiResponse<T>`、`PageResult<T>` 与 19 个错误码常量，但**没有任何一行代码真的发过请求**。

本分支补上这条链路的两端：

- **怎么发请求** —— 统一的 axios 实例、自动附加认证头、自动解包 `data`、统一错误处理
- **怎么记住登录** —— accessToken / refreshToken / 用户信息的存放与持久化

完成后，后续所有业务页面只需 `import` 请求方法即可调用后端，不必各自处理 token 与错误码。

**目标（可验证）**：写一个调用 `GET /user/profile` 的组件，不需要手动传 token、不需要判断 `code`，直接拿到用户对象。

---

## 2. 范围

### 2.1 做什么

| 文件 | 职责 |
|------|------|
| `src/api/request.ts` | axios 实例；请求拦截器附加 `Authorization`；响应拦截器解包与错误分发；`4012` 刷新重放 + 并发锁 |
| `src/stores/auth.ts` | Pinia store；保存 `accessToken` / `refreshToken` / `user`；提供 `login` / `register` / `logout` / `refreshToken` 动作与 `isLogin` 计算属性；持久化到 localStorage |

### 2.2 不做什么（明确排除，防止范围蔓延）

- ❌ **路由**（`src/router/index.ts`、`guards.ts`）→ 分支 3-B
- ❌ **布局**（`src/layouts/MainLayout.vue`）→ 分支 3-B
- ❌ **任何页面或组件**（登录页、聊天页等）→ 后续分支
- ❌ **SSE 流式请求**（`POST /chat/stream`）→ 聊天分支，且必须用 `fetch` 而非 axios
- ❌ 不新增任何类型定义，只消费上一分支的 `types/api.ts`、`types/errcode.ts`

---

## 3. 依赖与交接

### 3.1 我依赖谁

| 依赖 | 状态 |
|------|------|
| `src/types/api.ts`（`ApiResponse<T>`、`PageResult<T>`） | ✅ 已合并（40e9105） |
| `src/types/errcode.ts`（`ErrorCode`、`ErrorCodeMessages`） | ✅ 已合并（40e9105） |
| `docs/API_CONTRACT.md` §1 全局约定 | ✅ 已冻结字段（签署待补） |
| `docs/API_CONTRACT.md` §2 错误码与前端处理约定 | ✅ |
| `docs/API_CONTRACT.md` §3.1–3.3 三个认证端点 | ✅ |
| `axios`、`pinia-plugin-persistedstate@3.x` | 已在 `package.json` |

### 3.2 谁依赖我

| 依赖方 | 用我的什么 |
|--------|-----------|
| 分支 3-B 路由守卫 | `auth.ts` 的 `isLogin`，判断放行或跳登录页 |
| 分支 3-B 布局 | `auth.ts` 的 `user`（显示用户名/头像）、`logout` |
| 后续所有业务分支 | `request.ts` 导出的请求方法 |

> ⚠️ **交接约定**：本分支合并后，`auth.ts` 的对外 API（`isLogin` / `user` / `login` / `logout`）即视为稳定，改动需在群里广播。

---

## 4. 硬性约束

1. **响应解包**：仅当 `code === 200` 时返回 `data`；非 200 一律 `throw`，错误对象含 `code` 与 `message`。这是 `ApiResponse.data: T`（不含 `null`）类型成立的前提，见契约 §1。
2. **认证头**：所有请求自动附加 `Authorization: Bearer <accessToken>`；**免鉴权端点**（`/auth/login`、`/auth/register`、`/auth/refresh`）不附加。
3. **4012 处理**：access token 过期时自动调用 `/auth/refresh`，成功后**重放原请求**；并发场景下**只发起一次 refresh**（并发锁）。
4. **登出条件**：遇到 `4010` / `4011` / `4014` 时清空登录态并跳转登录页，**不重试**（契约 §2）。
5. **token 轮换**：refresh 成功后必须同时更新 `accessToken` 和 `refreshToken`，不可只更新其一。
6. **持久化**：登录态存 localStorage，刷新页面后保持；使用 `pinia-plugin-persistedstate` **3.x 语法**。
7. **禁止 `any`**：TS `strict` 模式下零 `any`，错误对象用 `unknown` + 类型收窄。
8. **不泄露敏感信息**：`user` 中只存 `id` / `username` / `email` / `avatarUrl`，不存密码。
9. **错误边界**：refresh 本身失败（返回 `4014`）时直接登出，不得陷入「刷新失败 → 再刷新」的死循环。

---

## 5. 验收标准

- [ ] `request.ts` 导出统一的请求方法，业务代码调用时无需手动传 token
- [ ] `code === 200` 时返回 `data` 字段，调用方**不需要**判断 `code`
- [ ] 非 200 时抛出错误，错误对象包含 `code` 与 `message`
- [ ] 请求自动附加 `Authorization: Bearer <accessToken>` 头
- [ ] 免鉴权端点（login / register / refresh）不附加 token 头
- [ ] 遇到 `4012` 时自动调用 refresh 并**重放**原请求
- [ ] 多个请求同时遇到 `4012` 时，refresh **只发起一次**（并发锁）
- [ ] refresh 成功后 `accessToken` 与 `refreshToken` **两者都更新**
- [ ] 遇到 `4010` / `4011` / `4014` 时清空登录态并跳转登录页，且**不重试**
- [ ] `auth.ts` 提供 `login` / `register` / `logout` / `refreshToken` 与 `isLogin`
- [ ] 注册成功后直接进入登录态（契约 §3.1「注册即登录」，不再走登录页）
- [ ] 登录态持久化，手动刷新浏览器后仍保持登录
- [ ] `npx tsc --noEmit` 零错误
- [ ] `npm run build` 通过

---

## 6. 变更记录

| 日期 | 版本 | 变更内容 | 改动人 |
|------|------|----------|--------|
| 2026-09-15 | v1 | 初始版本 | 成员 2 |

# 前端请求层与登录态（frontend-auth-request）· 实施计划

> 本文件是**活文档**：记录怎么做、分几步、每步状态。
> 与 `spec.md` 的分工：spec 说做什么，plan 说怎么做。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-auth-request` |
| 关联规格 | `./spec.md` |
| 更新方式 | 每完成一步，把状态列从 ⬜ 改为 ✅ |

---

## 1. 步骤拆解

| # | 步骤 | 产出 | 状态 |
|---|------|------|:----:|
| 1 | 写 `stores/auth.ts` 骨架：state / getters / 动作签名 | 可被 import，无请求逻辑 | ✅ |
| 2 | 写 `api/request.ts`：axios 实例 + 请求拦截器（附加 token） | 能带 token 发请求 | ⬜ |
| 3 | 写响应拦截器：`code === 200` 解包，否则抛错 | 返回值就是业务数据 | ⬜ |
| 4 | 实现 `4012` 刷新重放 + 并发锁 | 过期后无感续期 | ⬜ |
| 5 | 实现 `4010` / `4011` / `4014` 登出分支 | 彻底失效即登出 | ⬜ |
| 6 | 补全 `auth.ts` 的 login / register / logout / refreshToken 动作，接上 `request.ts` | 完整闭环 | ⬜ |
| 7 | 配置持久化（`pinia-plugin-persistedstate` 3.x），本地自查 | 刷新页面仍登录 | ⬜ |

> 为什么 1 在 2 前面：`request.ts` 的刷新逻辑要调 `auth.ts`，先立骨架避免来回改。

---

## 2. 文件清单

| 文件 | 操作 |
|------|------|
| `frontend/src/stores/auth.ts` | 新增 |
| `frontend/src/api/request.ts` | 新增 |
| `frontend/src/main.ts` | 修改（挂载 pinia 持久化插件） |
| `frontend/src/api/index.ts` | 新增（可选：统一导出，便于业务层 import） |
| `docs/specs/frontend-auth-request/spec.md` | 新增 |
| `docs/specs/frontend-auth-request/plan.md` | 新增 |
| `PROGRESS.md` | 修改（成员 2 段） |

---

## 3. 关键实现要点

### 3.1 模块依赖关系

```
stores/auth.ts  ──调用──→  api/request.ts  （发 login/refresh 请求）
api/request.ts  ──调用──→  stores/auth.ts  （读 token、写新 token）

⚠️ 循环依赖。解决办法：request.ts 内部**延迟获取** store 实例
（在拦截器函数体内调用 useAuthStore()，而不是模块顶层 import 后立即执行），
因为 Pinia 的 store 必须先 activate 才能用。
```

### 3.2 并发锁设计

```ts
// 用模块级变量缓存「正在进行的刷新」
let refreshPromise: Promise<string> | null = null

// 第一个 4012 触发刷新并赋值，后续请求直接 await 同一个 Promise
// 刷新结束后置回 null
```

要点：
- 后续请求**复用同一个 Promise**，而不是各自发起
- 无论成功失败都要在 `finally` 里把变量清空，否则后续刷新全被卡住
- 刷新失败 → 清空登录态 + 跳登录页，并把该 Promise reject 给所有等待者

### 3.3 免鉴权白名单判断

```ts
const WHITE_LIST = ['/auth/login', '/auth/register', '/auth/refresh']
// 判断 config.url 是否白名单，是则不加 Authorization 头
// 注意：refresh 请求本身不能再触发 4012 刷新（否则死循环）
```

### 3.4 持久化（3.x 语法）

`pinia-plugin-persistedstate` 3.x 的 `persist` 写法与 4.x 不同，本机安装的是 3.x（因项目锁 pinia 2）。要点：

- store 的 `persist` 选项里用 `key` 与 `paths`（**不是** 4.x 的 `pick`）
- 只持久化 `accessToken` / `refreshToken` / `user`，不持久化临时状态
- 插件在 `main.ts` 里 `app.use(piniaPluginPersistedstate)`

### 3.5 与上一分支的衔接

- `ApiResponse<T>` 与 `ErrorCode` 直接 `import`，不重复定义
- 抛错时用 `ErrorCode` 常量比较（如 `ErrorCode.ErrTokenExpired`），不写魔法数字 `4012`
- `VITE_API_BASE_URL` 取环境变量（含 `/api/v1` 后缀）
- 兑现 `VITE_USE_MOCK`：为 `true` 时不发真实请求（Mock 策略见 §4）

---

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| `request.ts` 与 `auth.ts` 循环依赖 | 运行时报 store 未初始化 | 拦截器内延迟调用 `useAuthStore()` |
| 并发锁未清空 | 刷新一次后永久卡死 | `finally` 中置 `null` |
| refresh 请求自身走拦截器 | 死循环刷新 | 白名单跳过 + 标记跳过刷新 |
| 持久化语法用错版本 | 编译报错 | 按 3.x 的 `key` / `paths` 写 |
| 契约 §13 三人签署未完成 | 字段若变，token 结构返工 | 本分支只依赖 §3，变更风险低 |
| 登录页尚未存在 | 登出后无页可跳 | 本分支只做 `router.push('/login')`，页面留分支 3-B/后续 |

---

## 5. 进度记录

| 日期 | 进展 |
|------|------|
| 2026-09-15 | 建立 spec 与 plan，确定方案 B（拆两支），本支负责 auth + request |
| 2026-09-15 | 完成 spec/plan 提交（e1fe8e7）；auth.ts 骨架完成并通过 tsc（4bfc19e） |


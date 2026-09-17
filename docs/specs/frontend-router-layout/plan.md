# 路由表与主布局（frontend-router-layout）· 实施计划

> 本文件是**活文档**：记录怎么做、分几步、每步状态。
> 与 `spec.md` 的分工：spec 说做什么，plan 说怎么做。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-router-layout` |
| 关联规格 | `./spec.md` |
| 更新方式 | 每完成一步，把状态列从 ⬜ 改为 ✅ |

---

## 1. 步骤拆解

| # | 步骤 | 产出 | 状态 |
|---|------|------|:----:|
| 1 | 写 `types/router.d.ts`：扩展 `RouteMeta` | 编辑器能提示 `to.meta.title` | ✅ |
| 2 | 写 `layouts/MainLayout.vue`：侧边导航 + 用户名 + 退出登录 + 出口 | 页面外壳可用 | ✅ |
| 3 | 写 `router/index.ts`：路由表 + 内联占位组件 | 5 条页面路由 + 404 兜底 | ✅ |
| 4 | 写 `router/guards.ts`：前置守卫（登录态 + `?redirect=`） | 未登录被弹回登录页 | ✅ |
| 5 | 补后置守卫：`document.title` 跟随 `meta.title` | 标签页标题正确 | ✅ |
| 6 | 改 `main.ts` 注册 router；改 `App.vue` 为 `<router-view />` | 应用真的跑路由 | ✅ |
| 7 | 本地自查：`typecheck` / `build` / `dev` 逐条对验收 | 10 条验收全过 | ✅ |

> 为什么 1 在 2 前面：`router.d.ts` 是纯类型声明，先落它，后面写路由表与守卫时编辑器才有 `meta.title` 的提示和检查。
> 为什么 `MainLayout.vue` 排在路由表之前：路由表要 `import('@/layouts/MainLayout.vue')`，文件不存在时 `vue-tsc` 直接报 TS2307、构建失败。**先有零件，再组装。**

---

## 2. 文件清单

| 文件 | 操作 |
|------|------|
| `frontend/src/router/index.ts` | 新增 |
| `frontend/src/router/guards.ts` | 新增 |
| `frontend/src/layouts/MainLayout.vue` | 新增 |
| `frontend/src/types/router.d.ts` | 新增 |
| `frontend/src/main.ts` | 修改（注册 router） |
| `frontend/src/App.vue` | 修改（改为 `<router-view />`） |
| `docs/specs/frontend-router-layout/spec.md` | 新增 |
| `docs/specs/frontend-router-layout/plan.md` | 新增 |

> 共 **6 个源文件**（4 新增 + 2 修改）+ 2 个文档，落在「一支 3-8 文件」的粒度内。
> **无 `views/**`** —— 页面由内联占位组件顶替，理由见 §3.2。

---

## 3. 关键实现要点

### 3.1 为什么守卫不写 `await authStore.restore()`

`TECH_DESIGN.md` §3.3 的示例守卫里有两行：

```ts
if (!authStore.initialized) { await authStore.restore() }
```

**本支未采用**，因为本仓库的 `auth.ts` 走的是 `pinia-plugin-persistedstate`，不是手写 `localStorage`：

- 插件在 `main.ts` 的 `pinia.use(piniaPluginPersistedstate)` 时注册；**每个 store 第一次被 `useAuthStore()` 取用时，就地同步地从 `localStorage` 恢复**。
- 守卫里那次 `useAuthStore()` 恰好是全应用第一次取该 store → 等它返回时 `accessToken` 已被插件填好，`isLogin` 立刻是准的。**不存在「守卫比持久化先跑」的时间差。**
- 所以既不需要 `initialized` 标志，也不需要 `restore()` 方法 —— 本仓库的 `auth.ts` 里本来就没有这两个东西。它的真实对外 API 是 `isLogin` / `user` / `login` / `register` / `logout` / `refresh`。

**结论**：守卫直接用 `authStore.isLogin`。

> 这也是对 `TECH_DESIGN.md` §3.3 / §3.4 示例的一处**有意偏离**：那两节的示例代码基于另一套组合式写法（手写 `localStorage` + `restore()`），与本仓库已合并的实现不一致。本支以**实际代码**为准。

### 3.2 为什么用内联占位组件，而不是占位页文件

路由表里写下的页面组件**必须真实存在**，否则 `vue-tsc` 报 `TS2307`（找不到模块）、`vite build` 直接失败。

本支决定**不创建 `views/**` 占位文件**，而是在 `router/index.ts` 里就地定义一个渲染函数组件：

```ts
import { defineComponent, h } from 'vue'

const Placeholder = defineComponent({
  name: 'RoutePlaceholder',
  render: () => h('div', '待实现'),
})
```

- 5 个页面路由全部先指向 `Placeholder`，本支 **0 个额外文件**，且 `build` / `typecheck` 都通过。
- 之后各功能分支把 `component: Placeholder` 换成 `component: () => import('@/views/...')` **一行即可**，路径 / `name` / `meta` 都不动。

两个备选方案为什么不用：

| 备选 | 为什么不用 |
|------|-----------|
| 建 7 个极简占位页文件 | 文件数冲到 14 个，超粒度上限，且这些文件很快会被真实页面替换，属于一次性垃圾 |
| 把没页面的路由注释掉 | **行不通**：守卫跳转到 `/login` 时该路由若被注释，用户直接撞 404，守卫本身失效 |

> ⚠️ 两个细节：
> 1. 用 `h()` 而**不是** `{ template: '<div>...</div>' }` —— Vite 默认引入 Vue 的 **runtime-only** 构建，`template` 选项需要运行时模板编译器，会报错。
> 2. 用 `defineComponent()` 包一层 —— 裸对象字面量传给 `RouteRecordRaw.component` 时，TS 对 `render` 选项的推断不够精确，容易报类型错误。

### 3.3 依赖方向：守卫依赖 store，但 `request.ts` 不依赖路由

- `router/guards.ts` import `stores/auth.ts` → **正常**（路由层 → 状态层，上层依赖下层）。
- `api/request.ts` **不** import 路由层 → 它用 `window.location.assign('/login')` 整页跳转（见分支 3-A 的 `plan.md` §3.6）。若让它 import `router`，就成了「底层模块反向依赖上层」，并与 `router → guards → store → request.ts` 连成一个环。

**口诀：依赖只能从上往下，不能从下往上。**

### 3.4 404 页为什么必须 `requiresAuth: false`

守卫规则是「`meta.requiresAuth !== false` 且未登录 → 跳登录页」。

若 404 路由不带这个标记，未登录用户随便敲一个不存在的地址（如 `/abc`）会被弹到 `/login`，**永远看不到 404 页**，排查问题时会非常迷惑。所以 404 显式声明 `requiresAuth: false`。

### 3.5 为什么用全局前置守卫，而不是组件内钩子

| 方案 | 评价 |
|------|------|
| 全局 `router.beforeEach` | ✅ 只有一处，新增页面**自动**受保护，漏不了 |
| 组件内 `onBeforeRouteEnter` | ❌ 每个页面重复写一遍，**漏写不报错**——这是最危险的失败模式 |

### 3.6 history 模式的两个后果（提前知道）

- **开发环境**：Vite 内置 SPA fallback，刷新子路由正常。
- **生产环境**：需要 Nginx `try_files $uri $uri/ /index.html`，否则刷新 `/chat` 会 404。这是成员 3 的部署工作，本支负责在合并后提醒并在联调时验证。

### 3.7 后端登录接口尚未就绪，怎么验

成员 1 的 `feature/auth-login` 还没合并，`/auth/login` 不可用。本支验收**不依赖后端**，用手写 `localStorage` 模拟登录态：

```js
// 浏览器 DevTools → Console
localStorage.setItem('heart-echo-auth', JSON.stringify({
  accessToken: 'fake-access-token',
  refreshToken: 'fake-refresh-token',
  user: { id: 1, username: 'lin', email: 'lin@example.com', avatarUrl: null },
}))
location.reload()
```

清除登录态（模拟未登录）：`localStorage.removeItem('heart-echo-auth')` 后刷新。

> 存储的 key 与字段名来自 `stores/auth.ts` 的 `persist` 配置（`key: 'heart-echo-auth'`，`paths: ['accessToken','refreshToken','user']`），**手写时字段名必须逐字一致**，否则插件读不到。

---

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| 路由指向不存在的组件 | `vue-tsc` TS2307 / build 失败 | 本支统一用内联 `Placeholder`，不引用任何 `views/**`（§3.2） |
| 守卫里用了 `auth.ts` 不存在的 API | 类型报错或运行时报错 | 只用 `isLogin` / `user` / `logout`（§3.1） |
| 404 页被守卫拦到登录页 | 无法验证不存在路径 | 404 显式 `requiresAuth: false`（§3.4） |
| `App.vue` 忘了改成 `<router-view />` | 页面白屏、路由不生效 | 与 `main.ts` 同一笔提交一起改 |
| `main.ts` 漏了 `app.use(router)` | 守卫完全不执行，且无报错 | 自查清单第 1 项；`dev` 下访问 `/chat` 立刻能发现 |
| 后端登录接口未实现 | 走不了真机「登录 → 跳转」 | 本支验收全部用 `localStorage` 模拟（§3.7）；真联调留到 `feature/auth-login` 合并后 |
| 生产 history 刷新 404 | 部署后才暴露 | 已列入 §3.6，合并后提醒成员 3 配 Nginx |
| 契约 §13 三人签署未完成 | 字段若变需返工 | 本支不消费任何接口数据，仅用 `user.username`，风险极低 |

---

## 5. 进度记录

| 日期 | 进展 |
|------|------|
| 2026-09-17 | 建立 spec 与 plan；确定「占位页拆支 + 内联占位组件」方案 |
| 2026-09-17 | 完成 `types/router.d.ts` 与 `layouts/MainLayout.vue`；步骤 2/3 顺序对调——路由表要引用布局文件，先有零件再组装 |
| 2026-09-17 | 完成 `router/index.ts`（路由表 + 内联占位）与 `router/guards.ts`（前后置守卫）；改 `main.ts` 挂 router、`App.vue` 改为 `<router-view />` |
| 2026-09-17 | 自查：`vue-tsc --noEmit` 与 `vite build` 均 EXIT 0；浏览器实测 10 条验收全过 |

# 路由表与主布局（frontend-router-layout）· 规格说明

> 本文件是**合同**：定义本分支做什么、做到什么算完成。
> 合并后即冻结，任何字段或行为变更必须升版本并在群里广播。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-router-layout` |
| 状态 | 🚧 进行中 |
| 依赖分支 | `feature/frontend-auth-request`（已合并 `4d7ff63`） |
| 关联契约 | `docs/API_CONTRACT.md` §1（免鉴权白名单 4 个） |
| 关联设计 | `docs/TECH_DESIGN.md` §3.3（路由管理与路由守卫） |

---

## 1. 背景与目标

前两个分支解决了「数据长什么样」（`types/`）与「怎么发请求、怎么记住登录」（`api/request.ts` + `stores/auth.ts`），但整个前端**至今只有一个页面**：

- `App.vue` 还是脚手架的自检页（标题 + 计数器按钮）
- `main.ts` 里没有 `app.use(router)`
- 没有任何地址能访问到业务页面

本分支补上前端地基的**第三块**：

- **哪个地址显示哪个页面** —— 路由表 + history 模式
- **这个人能不能看** —— 全局前置守卫，未登录弹回登录页并记住来源
- **页面的公共外壳** —— `MainLayout.vue`（侧边导航 + 退出登录 + `<router-view>` 出口）

完成后，成员 3 拿到 `/personas`、`/moments`、`/schedules` 三个挂载点，只需替换组件即可开工，不必改动路由结构。

**目标（可验证）**：未登录时访问 `/chat`，地址栏被改写成 `/login?redirect=/chat`。

---

## 2. 范围

### 2.1 做什么

| 文件 | 职责 |
|------|------|
| `src/router/index.ts` | 路由表：`/login`、`/register`、`/`（`MainLayout` 嵌套 5 个子路由）、404 兜底 |
| `src/router/guards.ts` | 前置守卫（登录态校验 + `?redirect=`）、后置守卫（`document.title`） |
| `src/layouts/MainLayout.vue` | 主布局：侧边导航、当前用户名、退出登录、`<router-view>` 出口 |
| `src/types/router.d.ts` | `RouteMeta` 类型扩展（`requiresAuth` / `title`） |
| `src/main.ts` | 修改：注册 router |
| `src/App.vue` | 修改：改为 `<router-view />` |

### 2.2 不做什么（明确排除，防止范围蔓延）

- ❌ **任何真实页面**（登录页、注册页、聊天页、画像页……）→ 后续分支
- ❌ **`views/**` 占位页文件** → 本支用**内联占位组件**顶替，理由见 `plan.md` §3.2
- ❌ 请求层与登录态（`request.ts` / `auth.ts` / `types/{api,errcode}.ts`）→ 已完成，本支不动
- ❌ 侧边导航的**视觉打磨**（图标、窄屏收起、动画）→ Week 4 UI 打磨分支
- ❌ `views/schedule/` 的实现（P1 功能，砍功能时优先砍）
- ❌ 后端未实现端点的联调（登录接口由成员 1 的 `feature/auth-login` 提供）

---

## 3. 依赖与交接

### 3.1 我依赖谁

| 依赖 | 状态 |
|------|------|
| `stores/auth.ts` 的 `isLogin` / `user` / `logout` | ✅ 已合并（`4d7ff63`） |
| `vue-router@^4.4.5` | 已在 `package.json` |
| `docs/API_CONTRACT.md` §1 免鉴权白名单（4 个端点） | ✅ 已冻结 |
| 后端 `GET /api/v1/health`（联调时确认后端活着） | ✅ 已合并（`af2f857`，成员 1 的 Step 8） |

### 3.2 谁依赖我

| 依赖方 | 用我的什么 |
|--------|-----------|
| 成员 3（人设 / 朋友圈 / 日程页） | `/personas`、`/moments`、`/schedules` 三个路由位置，以及 `MainLayout` 的 `<router-view>` 出口 |
| 后续所有页面分支 | 路由表结构、`meta.requiresAuth` 与 `meta.title` 约定 |
| 成员 1（联调） | 登录成功后跳转的落地页 `/chat` |

> ⚠️ **挂载约定**：成员 3 只需把路由表里对应那条的 `component` 从 `Placeholder` 换成 `() => import('@/views/persona/PersonaView.vue')`，**路径、`name`、`meta` 一概不改**。这是 `MASTER §4.5` 与 `MEMBER_2_FRONTEND.md` §6 已约定的交接方式。

---

## 4. 硬性约束

1. **动态 import**：页面组件一律用 `() => import(...)` 懒加载，不静态 import —— 首屏不必下载全部页面代码。
2. **`meta` 驱动**：是否需登录由 `meta.requiresAuth` 声明，**不在守卫里写 `if (to.path === '/chat')` 这类硬编码判断**——新增页面会漏改。
3. **免鉴权页面必须显式声明**：`/login`、`/register` 与 404 页都要 `requiresAuth: false`。**404 页尤其不能漏**：否则未登录访问不存在的路径会被拦到登录页，永远看不到 404。
4. **禁止 `any`**：TS `strict` 模式零 `any`。
5. **history 模式**：用 `createWebHistory()`，不用 hash 模式。
6. **不引入新依赖**：`vue-router` 已在 `package.json`。
7. **`noUnusedLocals` / `noUnusedParameters` 已开**：不留未使用的 import 与函数参数，否则 `vue-tsc` 报 TS6133。
8. **依赖方向单向**：`router/guards.ts` 可以 import `stores/auth.ts`（上层依赖下层）；`api/request.ts` **不得** import 路由层（底层不依赖上层），它保持 `window.location.assign('/login')`。

---

## 5. 验收标准

- [ ] `npm run typecheck`（`vue-tsc --noEmit`）零错误
- [ ] `npm run build` 通过
- [ ] `npm run dev` 起得来，访问 `/` 自动重定向到 `/chat`
- [ ] 未登录访问 `/chat` → 跳转 `/login?redirect=/chat`
- [ ] 未登录访问不存在的路径（如 `/xxx`）→ 显示 404 页，**不被拦到登录页**
- [ ] 已登录访问 `/login` → 自动弹回 `/chat`
- [ ] `document.title` 随 `meta.title` 变化（如「对话 · HeartEcho」）
- [ ] `MainLayout` 显示当前用户名（`authStore.user`），点退出登录后清空登录态并跳 `/login`
- [ ] `/personas`、`/moments`、`/schedules` 三条路由存在且可访问（显示占位内容），成员 3 可原地替换组件
- [ ] 本支共 6 个源文件（4 新增 + 2 修改）+ 2 个文档，**无 `views/**` 文件**

---

## 6. 变更记录

| 日期 | 版本 | 变更内容 | 改动人 |
|------|------|----------|--------|
| 2026-09-17 | v1 | 初始版本 | 成员 2 |

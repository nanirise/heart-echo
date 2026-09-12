# plan · 前端骨架搭建

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/frontend-skeleton`
> 负责人：成员 2 ｜ 最后更新：2026-09-12

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 状态 |
|---|---|---|---|---|
| 1 | 初始化工程清单 | `frontend/package.json` | JSON 有效，依赖覆盖 Vue 全家桶 | 完成 |
| 2 | 严格 TS 配置 | `frontend/tsconfig.json` | `strict` 开启，`@/*` 别名已声明 | 完成 |
| 3 | 构建与页面入口 | `vite.config.ts`、`index.html` | 别名两侧一致，端口 5173 | 完成 |
| 3.5 | 安装依赖 | `node_modules/`、`package-lock.json` | `npm install` 无 ERESOLVE | 阻塞（环境问题） |
| 4 | **数据契约** | `src/types/api.ts` | 与后端 JSON tag 逐字段对齐 | 准备中 |
| 5 | 错误码类型 | `src/types/errcode.ts` | 覆盖全部约定错误码，禁硬编码数字 | 未开始 |
| 6 | 登录态持久化 | `src/stores/auth.ts` | 刷新页面登录态不丢，登出可清空 | 未开始 |
| 7 | **请求层（核心）**| `src/api/request.ts` | 拦截器 + 4012 刷新重放 + 并发只刷一次 | 未开始 |
| 8 | 路由与守卫 | `router/index.ts`、`router/guards.ts` | 未登录跳转带 `redirect` | 未开始 |
| 9 | 应用外壳 | `main.ts`、`App.vue`、`layouts/MainLayout.vue` | 页面能渲染，非白屏 | 未开始 |
| 10 | 冒烟验收 | — | `tsc --noEmit` 与 `build` 双通过 | 未开始 |

**优先级说明**：**Step 4 优先于 Step 5-9**。`types/api.ts` 解锁成员 3，越早交付并行度越高。宁可后面几步慢一点，也要先把它推上去。

## 2. 文件清单

| 路径 | 作用 | 谁会用到 |
|---|---|---|
| `frontend/package.json` | 依赖清单与脚本入口 | 全员 |
| `frontend/tsconfig.json` | 类型检查规则、`@/*` 别名 | 全员 |
| `frontend/vite.config.ts` | 构建配置、开发端口、别名 | 全员 |
| `frontend/index.html` | 唯一 HTML，`#app` 挂载点 | — |
| `frontend/src/types/api.ts` | 前后端数据契约 | **成员 3**、成员 1 |
| `frontend/src/types/errcode.ts` | 业务错误码常量与类型 | 成员 1 核对 |
| `frontend/src/api/request.ts` | axios 实例、拦截器、刷新重放 | **成员 3** |
| `frontend/src/stores/auth.ts` | token 与登录态 | 成员 3 |
| `frontend/src/router/index.ts` | 路由表 | **成员 3** |
| `frontend/src/router/guards.ts` | 路由守卫 | 全员 |
| `frontend/src/layouts/MainLayout.vue` | 主布局与 `<router-view>` | **成员 3** |
| `frontend/src/main.ts` | 应用入口 | — |

## 3. 关键实现要点

### 3.1 请求层 4012 刷新重放（Step 7）

1. 拦截到 4012（token 过期）→ 若已有刷新请求在途，**排队等待**，不重复发起
2. 否则发起刷新；成功后**重放**原请求（带上新 token）
3. 刷新失败 → 清空本地登录态 → 跳转登录页并带 `redirect`

**并发锁是这里最容易写错的地方**：同时 3 个请求都返回 4012 时，必须**只刷新一次**，否则会互相覆盖 token 导致全部失败。

token 的读写统一走 `stores/auth.ts`，`request.ts` 不直接操作 localStorage。

### 3.2 环境变量

| 变量 | 用途 | 开发默认值 |
|---|---|---|
| `VITE_API_BASE_URL` | 后端接口地址 | `http://localhost:8080` |

> `EMOTION_BACKEND` / `MEMORY_BACKEND` 是后端与 AI 服务的阶段切换开关，前端**无感、不感知**。

## 4. 风险与对策

| 风险 | 对策 |
|---|---|
| npm 依赖版本漂移（`^` 会拉到更新的小版本） | 提交 `package-lock.json` 锚定版本；依赖变更后在群里广播 |
| 本机 npm 无法调用 `cmd.exe`，esbuild 装不上 | 环境问题、与代码无关；按诊断步骤单独处理，**不阻塞 Step 4** |
| 前后端字段不一致（最易出 bug） | `types/api.ts` 与 `API_CONTRACT.md` 逐字段核对，联调优先查这里 |
| 契约变更未广播 | 任何字段变更必须发群 + 登记进 `API_CONTRACT.md` |
| 情绪字段被误用展示 | review 清单已含此项；前端应把它当作不存在 |

## 5. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-12 | Step 1-3 完成并推送；spec/plan 补记 | `npm install` 环境问题（esbuild 调 cmd.exe 失败） |

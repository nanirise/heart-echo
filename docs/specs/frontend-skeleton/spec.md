# spec · 前端骨架搭建

> 功能名：frontend-skeleton ｜ 分支：`feature/frontend-skeleton`
> 负责人：成员 2（前端） ｜ 状态：进行中
> 创建：2026-09-12 ｜ 最后更新：2026-09-12
> 关联：[协作文档](../../COLLABORATION.md) ｜ [接口契约](../../API_CONTRACT.md) ｜ [技术文档](../../TECH_DESIGN.md)

---

## 1. 背景与目标

成员 3 的页面（人设、朋友圈、日程）需要一个挂载点；成员 1 联调聊天页需要请求层与类型定义。
本功能**不产出任何业务页面**，只交付「别人能往上写代码」的工程骨架。

**目标（一句话）**：让另两位队友拿到仓库后，能直接 `npm run dev` 起服务、按既有约定新增页面与接口调用，不需要再问任何结构问题。

## 2. 范围

### 2.1 做什么（In Scope）

| 项 | 产物 |
|---|---|
| 工程配置 | `package.json`、`tsconfig.json`、`vite.config.ts`、`index.html` |
| 数据契约 | `src/types/api.ts`、`src/types/errcode.ts` |
| 请求层 | `src/api/request.ts`（axios 实例 + 拦截器 + 4012 刷新重放） |
| 状态与路由 | `src/stores/auth.ts`、`src/router/index.ts`、`src/router/guards.ts` |
| 应用外壳 | `src/main.ts`、`src/App.vue`、`src/layouts/MainLayout.vue` |
| 目录骨架 | `src/{api,assets,components,layouts,router,stores,types,utils,views}` |

### 2.2 不做什么（Out of Scope）

| 不做的事 | 归属功能 |
|---|---|
| 登录页 / 注册页 / 改密码页 | `feature/auth-login` |
| 聊天界面、SSE 前端解析、对话侧栏 | `feature/chat-stream` |
| 用户画像页 | `feature/user-profile` |
| 人设、朋友圈、日程相关一切 | 成员 3 负责 |

> 本功能范围内**不出现业务 UI**。路由表只放占位路由，供守卫跳转使用。

## 3. 依赖与交接

### 3.1 我依赖谁

| 依赖 | 提供方 | 状态 |
|---|---|---|
| 统一响应结构 `Response[T]` | 成员 1 | 待确认 |
| 错误码表（4001 / 4003 / 4012 / 4013 …） | 成员 1 | 待确认 |
| SSE 事件契约（`delta` / `done` / `error`） | 成员 1 | 待确认 |

### 3.2 谁依赖我（本功能优先级最高的原因）

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `src/types/api.ts` | 成员 3、成员 1 | 成员 3 据此翻译 Go struct，成员 1 据此核对 JSON tag |
| `src/api/request.ts` | 成员 3 | 他的页面直接调用，避免重复造请求层 |
| `src/router/index.ts` + `guards.ts` | 成员 3 | 他的新路由往这里挂 |
| `src/layouts/MainLayout.vue` | 成员 3 | 他的页面挂在布局的 `<router-view>` 内 |

## 4. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| TypeScript `strict`，**禁止 `any`**（用 `unknown` + 类型收窄） | 项目硬性要求 |
| `npx tsc --noEmit` 零错误、`npm run build` 通过 | 成员 2 自查清单 |
| 字段名**全 camelCase**，与后端 JSON tag 完全一致 | 协作文档 §6.4 |
| SSE 必须用 `fetch` + `ReadableStream`，**禁止 `EventSource`**（带不了 Authorization 头） | 成员 2 任务书 |
| `/memory`、`/profile/portrait`、`/schedules` **必须传 `personaId`**，永不传 `user_id` | 成员 2 任务书 |
| 情绪为内部信号，前端**不展示、不接收** `emotionLabel` / `emotionScore` | 开发总纲 §0 |
| 头像只支持填图片 URL，**不做文件上传** | 开发总纲 §0 |
| 未登录访问受保护路由需重定向，并带 `redirect` 参数 | 成员 2 自查清单 |
| `.env`、`node_modules/`、`dist/` 永不入库 | 团队红线 |
| 开发端口固定 `5173`；`@/` 别名须在 vite 与 tsconfig 两侧同时声明 | 技术文档 |

## 5. 验收标准

- [ ] `npm install` 成功，生成 `package-lock.json` 并入库
- [ ] `npm run dev` 在 `5173` 启动，浏览器能看到外壳页面（非白屏）
- [ ] `npx tsc --noEmit` 零错误，全项目无 `any`
- [ ] `npm run build` 通过
- [ ] `@/` 别名可用（`import type { User } from '@/types/api'` 不报错）
- [ ] 路由守卫：未登录访问受保护路由 → 重定向并带 `redirect` 参数
- [ ] `types/api.ts` 已推送到分支，成员 3 确认可读

## 6. 变更记录

| 日期 | 变更 | 原因 |
|---|---|---|
| 2026-09-12 | 创建 | 补记：Step 1-3 已先行完成，本文件为事后补写的设计记录 |

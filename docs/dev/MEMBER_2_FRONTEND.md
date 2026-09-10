# 成员 2 开发文档 · 前端框架 + 聊天界面 + 用户画像

> 你负责整个前端的地基和两条最核心的用户路径：登录 → 聊天。你的 `request.ts` 和 `types/` 是成员 3 写页面的前置。
> 项目全貌：[项目概览](../../README.md)。
> 协调规则与集成顺序见 [开发总纲](MASTER.md)，设计细节见 [技术文档 §3](../TECH_DESIGN.md#3-前端设计)。

## 0. 快速定位

| 项 | 内容 |
|----|------|
| 你负责 | Vue 骨架、路由表与守卫、Token 持久化、登录/注册页、聊天界面、SSE 前端解析、对话侧栏、用户画像页 |
| 你的主要文件 | `frontend/src/{router,stores/auth.ts,stores/chat.ts,api/request.ts,api/auth.ts,api/chat.ts,api/memory.ts,api/profile.ts,types/**}`、`views/auth/`、`views/chat/`、`views/profile/`、`components/chat/`、`layouts/MainLayout.vue`、`App.vue`、`main.ts`、`tsconfig.json`、`vite.config.ts` |
| 你不负责 | `views/persona/`、`views/moments/`、`api/persona.ts`、`api/moment.ts`、`stores/persona.ts`（成员 3）；后端与 AI 服务（成员 1）；部署（成员 3） |
| 你的 commit scope | `auth` `user` `chat`（前端不做情绪相关功能，不要用 `emotion` scope） |
| 你依赖 | 成员 1 的 `Response[T]` 结构、错误码表、SSE 事件契约；成员 3 的人设接口用于聊天页联调 |
| 谁依赖你 | **成员 3**——`api/request.ts`、`types/api.ts`、路由与布局是他的页面挂载点；**成员 1**——联调时消费你的聊天页与 SSE 解析 |
| 你的硬性要求 | ① Vue 全家桶 + Route 路由管理 + TypeScript + 路由守卫；③ 前端登录态持久化 |

**第一原则**：路由表和 `request.ts` 拦截器要在准备期搭好，让成员 3 能并行写他的页面。

---

## 1. 准备期（1-2 天）

- [ ] 本地环境：Node ≥18（建议 20）、Docker Desktop
- [ ] `npm create vite@latest frontend -- --template vue-ts`，装 Pinia、Vue Router 4、Element Plus、Axios
- [ ] `tsconfig.json` 开 `strict: true`、`noImplicitAny: true`、`paths: { "@/*": ["./src/*"] }`
- [ ] `cp frontend/.env.example frontend/.env`（`VITE_API_BASE_URL=http://localhost:8080/api/v1`）
- [ ] 写 `src/types/api.ts`（`ApiResponse<T>` / `PageResult<T>`）——**交接物 #8**
- [ ] 写 `src/api/request.ts`：Axios 实例 + 请求带 Token + 响应解包 + 4012 自动刷新后重放（4010/4011/4014 直接清空登录态跳登录页，**不重试**）
- [ ] 写路由表骨架与 `MainLayout`，给成员 3 预留 `personas` / `moments` / `schedules` 路由位置
- [ ] 参与契约评审，确认字段名（**camelCase，与后端 JSON 完全一致**）

**准备期末判据**：`npx tsc --noEmit` 零错误；`npm run dev` 起得来；未登录访问 `/chat` 被弹回 `/login?redirect=/chat`。

---

## 2. Week 1：认证链路

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 路由守卫 | `src/router/guards.ts` | `meta.requiresAuth` 生效，带 `?redirect=` |
| Token 持久化 | `src/stores/auth.ts` + `pinia-plugin-persistedstate` | 刷新 / 重开浏览器仍登录 |
| Axios 拦截器 | `src/api/request.ts` | 自动带 Token；4012 自动刷新后重放原请求；刷新失败跳登录 |
| 登录页 | `src/views/auth/LoginView.vue` | 表单校验；错误码 4013 显示「用户名或密码错误」 |
| 注册页 | `src/views/auth/RegisterView.vue` | 4003/4004 分别提示邮箱/用户名冲突 |
| 注册即登录 | `RegisterView.vue` | **注册成功直接写入登录态跳 `/chat`**，不要再跳登录页 |
| 修改密码 | `src/views/profile/PasswordView.vue`（或弹窗） | `PUT /user/password`；旧密码错显示 4015；成功后清登录态跳登录页 |
| 主布局 | `src/layouts/MainLayout.vue` | 侧边导航 + 退出登录 |
| 类型定义 | `src/types/{user,errcode}.ts` | 与后端错误码表一致 |

**关键代码要求**

- **禁止 `any`**。未知类型用 `unknown` 再做类型收窄。
- 响应拦截器统一解包：`code === 200` 时返回 `data`，否则按 `code` 抛业务错误；**组件里不写 `if (res.code === 200)`**。
- Token 刷新要有并发保护：多个请求同时 4012 时只发起一次 refresh，其余排队。
- 路由守卫首次进入先 `await authStore.restore()`，避免刷新瞬间误判未登录。

**Week 1 末里程碑**：注册 → 登录 → 刷新页面保持登录态 → 与成员 1 前后端联调成功。

---

## 3. Week 2：流式对话（生死线）

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| SSE 解析 | `src/api/chat.ts` | 用 `fetch` + `ReadableStream`，逐事件回调 |
| 聊天界面 | `src/views/chat/ChatView.vue` | 消息列表 + 输入框 + 发送 |
| 打字机 | `stores/chat.ts` | `delta` 逐段拼接渲染 |
| 消息气泡 | `components/chat/{MessageBubble,ChatInput}.vue` | Markdown 渲染 + 代码高亮 |
| 等待态 | `components/chat/TypingIndicator.vue` | 发送后到首个 `delta` 到达前显示 |
| 对话侧栏 | `ChatView` 侧栏 / `stores/chat.ts` | 列出人设，点击切换到该人设的对话（**人设的增删改在成员 3 的人设页，你只管切换**） |
| 未读红点 | `stores/chat.ts` + 侧栏 | 该人设有未读的主动消息时，名字旁显示红点，打开后清除（**日程提醒到点后也是一条普通消息，红点逻辑直接复用，不用为它加分支**） |
| 空状态 | `ChatView.vue` | 一个人设都没有时，显示引导 + 按钮跳 `/personas` |
| Mock 开发 | `src/api/mock/*.ts` + `VITE_USE_MOCK` | 后端未就绪时页面可跑 |

**SSE 关键实现**

- **不用 `EventSource`**：它只支持 GET 且不能自定义 `Authorization` 头。用 `fetch` + `ReadableStream` 手动解析。
- 解析要处理跨 chunk 的半行：维护 buffer，按 `\n\n` 切事件块，再按 `event:` / `data:` 取值。
- 三种事件分别处理，**`error` 事件要落到 UI 提示**，不能静默。
- 用户中途离开页面要 `AbortController` 中断流。
- **没有情绪事件**：情绪是后端内部信号，不会推给前端，**不要为它预留 handler 或类型**。

```ts
// src/api/chat.ts 关键结构
export async function streamChat(
  payload: StreamChatPayload,
  handlers: { onDelta: (text: string) => void
              onDone: (r: DonePayload) => void
              onError: (e: { code: number; message: string }) => void },
  signal?: AbortSignal,
) { /* fetch + ReadableStream 解析，事件名与契约一致 */ }
```

**Week 2 末生死线自测**：DevTools → Network → `stream` 请求 → EventStream 标签页，能看到 `delta` / `done` 逐条到达；页面上回复逐字出现。

> 若此时后端还没跑通，先用 Mock 把界面与解析逻辑做完并自测通过——**你的部分不能成为 Week 2 生死线的原因**。同时立即在群里说。

---

## 4. Week 3：用户画像

| 任务 | 产出文件 | 验收 |
|------|----------|------|
| 用户画像页 | `views/profile/ProfileView.vue` | 按人设展示 `profileData`（**camelCase，后端 JSON 就是这个名字**），页面上有伴侣切换 |
| 记忆查看 | 画像页内 / 独立区块 | 展示 `GET /memory?personaId=` 结果 |
| 资料编辑 | `views/profile/ProfileView.vue` | 改头像 URL（图片链接）与用户名；用户名冲突提示 4004 |

> **本周没有"情绪标签展示"这项工作**：情绪标签是后端内部信号（用于记忆与回复策略），前端不展示、不接收。`ChatMessage` 里虽然带着 `emotionLabel` / `emotionScore` 字段，但**不要渲染它们**——页面刷新后重新拉历史消息时同样忽略即可。

**记忆和画像都是「一个人设一份」，所以两个接口都必须传 `personaId`**：

```ts
// ✅ 切到哪个伴侣，就看谁的记忆与画像
const memory = await request.get<PageResult<MemoryItem>>('/memory', { params: { personaId } })
const portrait = await request.get<Portrait>('/profile/portrait', { params: { personaId } })
```

- `personaId` **必须传**，不传后端无法判断要看哪个人设的数据。
- `user_id` **永远不要传**——后端从 Token 取。前端自己拼 `user_id` 是越权隐患。
- 记忆是**只读**的，没有删除接口，不要做删除按钮。

> **亲密度**：`Persona` 上带了只读字段 `familiarity`（0-100）。契约层面已经备好，**是否在界面上展示（比如对话页顶部的进度条）你们后期再定**，不做也不影响任何功能。

---

## 5. Week 4：朋友圈联调 + UI 打磨

- [ ] **朋友圈页面（`views/moments/`）由成员 3 实现**，你负责它的路由挂载、组件复用（`MessageBubble`）与 UI 打磨
- [ ] 若成员 3 有余量做日程提醒（P1）：他只加 `views/schedule/ScheduleView.vue`，路由与布局是你的，**不要为它改路由结构**
- [ ] UI 统一：间距、颜色、加载态、空状态、错误提示
- [ ] 响应式：演示会用手机 4G 打开，**至少保证主链路（登录 → 对话 → 切人设）在移动端可用**；侧栏在窄屏下要能收起来
- [ ] 配合成员 1 排查 SSE 在生产环境的表现
- [ ] 配合成员 3 跑通完整演示脚本，准备演示账号与历史数据

---

## 6. 你的接口契约（对外发布）

你发布的契约，成员 3 依赖：

| 契约 | 内容 | 冻结时间 |
|------|------|----------|
| `ApiResponse<T>` | 与后端 `Response[T]` 一一对应 | 准备期 |
| `request.ts` 行为 | 返回解包后的 `data`；抛出的错误含 `code` + `message` | 准备期 |
| 路由 meta | `requiresAuth` / `title` | 准备期 |
| 目录放置规则 | `views/<模块>/XxxView.vue`、`api/<模块>.ts`、`types/<模块>.ts` | 准备期 |
| 可用组件 | `MessageBubble`、`TypingIndicator`、`EmptyState` 的 props | Week 2 末 |

**给成员 3 的挂载约定**：`/personas`、`/moments`、`/schedules`（P1）已在路由表 `MainLayout` 的 children 中预留，成员 3 只需替换 `component` 为空实现改为真实页面，不用改路由结构。

---

## 7. 常见坑

| 坑 | 现象 | 处理 |
|----|------|------|
| 字段名不一致 | 取到 `undefined` | 后端 JSON 是 camelCase，TS interface 也必须 camelCase（协作 §6.4） |
| 用 EventSource | 无法带 Authorization，401 | 改用 `fetch` + `ReadableStream` |
| 半行解析 | 偶发 JSON.parse 报错 | buffer 按 `\n\n` 切块，不完整的留到下一 chunk |
| 忘传 `personaId` | 记忆/画像接口报错或返回错的数据 | `/memory`、`/profile/portrait`、`/schedules` 都必须带 `personaId` |
| 给提醒做特殊消息类型 | 多出一套气泡样式与后端字段 | 到点的提醒就是一条普通 AI 消息，**复用 `MessageBubble`，不要新增 `type: 'reminder'`** |
| 想给头像做上传 | 实现到一半发现要文件存储 | **本期只支持填图片 URL**，不要写 `<input type="file">`（[总纲 §0.3](MASTER.md#03-功能范围)） |
| 刷新并发 | 多个 401 触发多次 refresh，后到的 token 失效 | refresh 加锁，排队重放 |
| history 模式 404 | 生产刷新子路由白屏 | Nginx `try_files $uri $uri/ /index.html`（成员 3 负责，你负责验证） |
| `any` 泛滥 | `tsc --noEmit` 报错 / 类型失效 | 用 `unknown` + 收窄；`ApiResponse<T>` 泛型化 |
| 中文路径 | 路由或文件名带中文 | 全英文命名（协作 §6.1） |

---

## 8. 每周自检

- [ ] `npx tsc --noEmit` 零错误，无 `any`
- [ ] `npm run build` 通过
- [ ] 我的提交都 push 了，commit 符合 `<type>(<scope>): <subject>`
- [ ] 没有把 `.env`、`node_modules/`、`dist/` 提交进仓库
- [ ] SSE 的 `error` 事件在 UI 上有可见提示
- [ ] 未登录访问受保护路由会被正确重定向，且带 `redirect` 参数
- [ ] 注册成功后是**自动登录直接进对话页**，没有多余的登录步骤
- [ ] 改密码时旧密码错误显示「原密码不正确」（4015），成功后清登录态跳登录页

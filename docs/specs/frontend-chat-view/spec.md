# 聊天页（frontend-chat-view）· 规格说明

> 本文件是**合同**：定义本分支做什么、做到什么算完成。合并后即冻结，行为变更须升版本并在群里广播。
> 按 [AGENTS §5.2](../../../AGENTS.md) 要求，本文只写**本功能独有**的内容；通用约定一律给链接，不重复描述。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-chat-view` |
| 状态 | 🚧 进行中 |
| 依赖分支 | 无（`develop` `b05a311` 切出） |
| 关联契约 | [API_CONTRACT](../../API_CONTRACT.md) §5 / **§6（🔒 已冻结）** / §4 |
| 关联设计 | [TECH_DESIGN](../../TECH_DESIGN.md) §3 |
| 后端依赖 | `chat-message` 的 SSE 实现 —— **未交付**（`ai-service/` 目录尚不存在） |
| 里程碑 | **Week 2 生死线**（[MASTER §7](dev/MASTER.md)） |

---

## 1. 背景与目标

Week 1 交付了「人的入口」（登录/注册页），但**进来之后没地方去**：`/chat` 这条路由从 3-B 起就指向内联占位组件，打开只有「待实现」三个字。而它是**核心演示链路的第一站**——`MASTER §10` 画的链路是「登录 → 创建人设 → **对话** → 记住信息 → 主动消息 → 朋友圈」，其中「流式对话」被列为 🚫 **绝不砍**。

本支把聊天页做出来，让「选一个人设 → 说话 → 看着回复逐字出现」这条链路在前端可用。

**本支的特殊之处：它是 Week 2 的生死线组件，但后端尚未开工。** 成员 1 在 2026-09-21 的进度里明确 `chat_service` / `message_repo` / `chat_handler` / `ai-service/` **一行都没写**。而 [MEMBER_2_FRONTEND §3](../../dev/MEMBER_2_FRONTEND.md) 的原话是：

> 若此时后端还没跑通，先用 Mock 把界面与解析逻辑做完并自测通过——**你的部分不能成为 Week 2 生死线的原因**。

所以本支**必须走 Mock 通道**把界面与 SSE 解析逻辑做完并自测通过，不等后端。

**目标（可验证）**：`VITE_USE_MOCK=true` 时，打开 `/chat` 选中一个人设、输入文字发送，能在页面上看到回复**逐字出现**，流结束后消息留在列表里；DevTools → Network 能观察到 `delta` / `done` 事件逐条到达（Mock 通道同样走真实的 `fetch` + `ReadableStream` 解析路径）。

---

## 2. 范围

### 2.1 做什么（In Scope）

| 文件 | 职责 |
|------|------|
| `frontend/src/api/chat.ts` | 新增 · `streamChat()`（SSE 解析）+ `getMessages()`（历史消息） |
| `frontend/src/api/mock/chat.ts` | 新增 · 与 `chat.ts` **同签名**的 Mock 实现 |
| `frontend/src/api/mock/index.ts` | 新增 · 按 `VITE_USE_MOCK` 决定导出真实实现还是 Mock |
| `frontend/src/stores/chat.ts` | 新增 · 消息列表、发送、打字机拼接、未读红点 |
| `frontend/src/views/chat/ChatView.vue` | 新增 · 对话页（人设侧栏 + 消息区 + 输入框） |
| `frontend/src/components/chat/MessageBubble.vue` | 新增 · 消息气泡（区分 user / assistant / nudge） |
| `frontend/src/components/chat/ChatInput.vue` | 新增 · 输入框 + 发送（含 Enter 发送 / Shift+Enter 换行） |
| `frontend/src/components/chat/TypingIndicator.vue` | 新增 · 首个 `delta` 到达前的等待态 |
| `frontend/src/types/chat.ts` | 新增 · `ChatMessage` / `StreamChatPayload` 等本模块类型 |
| `frontend/src/router/index.ts` | 修改 · `chat/:personaId?` 的 `component` 由 `Placeholder` 换成真实懒加载 import |
| `docs/specs/frontend-chat-view/{spec,plan}.md` | 新增 · 本文与实施计划 |

合计 **10 个源文件**（9 新增 + 1 修改）+ 2 篇文档。

> ⚠️ 本支规模**超出** [AGENTS §6](../../../AGENTS.md) 约定的「3-8 文件」。理由与拆分考量见 `plan.md` §3.10；结论是**不拆**（拆分会让「能跑通」这个判据失去意义）。

### 2.2 不做什么（明确排除，防止范围蔓延）

- ❌ **人设的增删改** → 成员 3 的人设页。本支侧栏**只负责列出人设与切换**，一个新增/删除按钮都不放
- ❌ `api/persona.ts` / `stores/persona.ts` → 人设接口归成员 3（[MEMBER_2_FRONTEND §0](../../dev/MEMBER_2_FRONTEND.md) 边界表）
- ❌ **Markdown 渲染与代码高亮** → 需引入新依赖（`marked` + `highlight.js`），而本支要求「不引入新依赖」。先用纯文本 + `white-space: pre-wrap` 展示，Markdown 化留到 Week 4 UI 分支
- ❌ 情绪相关任何处理 → 契约 §5 明确 `emotionLabel` / `emotionScore` 是**内部信号**，UI 不展示；本支**连类型都不为它们留字段**
- ❌ 日程提醒的特殊消息类型 → [MEMBER_2_FRONTEND §7](../../dev/MEMBER_2_FRONTEND.md)：到点的提醒就是一条普通 AI 消息，**不要新增 `type: 'reminder'`**
- ❌ 头像文件上传 → 契约只存 URL
- ❌ 移动端适配 / 动效 / 视觉打磨 → Week 4
- ❌ 后端的任何代码 → 成员 1

---

## 3. 依赖与交接

### 3.1 我依赖谁

| 依赖 | 状态 |
|------|------|
| SSE 契约 §6（三事件 `delta` / `done` / `error`） | ✅ **🔒 已冻结**，可直接照写 |
| `GET /chat/personas/:personaId/messages` 契约（§5） | ✅ 已定义 |
| `GET /personas` 契约（§4，侧栏用） | ✅ 已定义 |
| `request.ts`（带 token 的请求基础设施） | ✅ 已合并（PR #24） |
| `stores/auth.ts` | ✅ 已合并（PR #24） |
| 路由 `chat/:personaId?` 占位与 `MainLayout` | ✅ 已合并（PR #31） |
| `VITE_USE_MOCK` 的 TS 声明 | ✅ 已在 `vite-env.d.ts:13` |
| 后端 `POST /chat/stream`、`GET /chat/.../messages` | ❌ **未交付**（`ai-service/` 不存在）→ 见 §5 |
| 后端 `GET /personas` | ⚠️ 代码已合并，但**本机无 PG，跑不起来** → 侧栏同样走 Mock |

### 3.2 谁依赖我

| 依赖方 | 用我的什么 |
|--------|-----------|
| 成员 1（Week 2 联调） | `streamChat()` 的解析逻辑就是「契约 §6 的可执行说明书」；他实现 SSE 时对着它调 |
| 成员 3（人设页 → 对话页跳转） | 「点击人设进入对话」的落点；`ChatView` 的 `personaId` 从路由参数取 |
| 成员 3（未读红点） | 主动消息到达后的红点逻辑（`stores/chat.ts` 提供） |
| 答辩演示 | 核心链路第 3 站，也是「流式」这一亮点的唯一呈现处 |

---

## 4. 硬性约束

1. **禁用 `EventSource`** —— 它只能发 GET 且**不能自定义 `Authorization` 头**，而 `/chat/stream` 在 `protected` 组下、必须带 token。一律用 `fetch` + `ReadableStream`。见 [MEMBER_2_FRONTEND §3](../../dev/MEMBER_2_FRONTEND.md)。
2. **必须处理跨 chunk 的半行** —— `delta` 事件的字节可能被切在任意位置。维护 buffer，按 `\n\n` 切事件块，**不完整的尾巴留到下一个 chunk**。这是本支最容易写错、也最容易在 Mock 下「看起来没问题」的一点，见 `plan.md` §3.2。
3. **`error` 事件必须落到 UI** —— 契约 §6 原文：「前端必须处理 `error` 事件，不能静默」。且此时 **HTTP 状态仍是 200**，不能靠 `response.ok` 判断。
4. **事件类型不可私自增删** —— 只认 `delta` / `done` / `error` 三种。**未知事件名一律忽略**（向后兼容），不得报错。
5. **三种状态齐全** —— loading（等待首个 delta）/ error（可重试）/ empty（无人设时引导去 `/personas`）。见 [AGENTS §4.9](../../../AGENTS.md)。
6. **离开页面必须中断流** —— `AbortController`，否则组件卸载后回调仍在改已销毁的 reactive 数据。
7. **`getMessages()` 的返回值是倒序的** —— 契约 §5 明确「最新在前」，**前端自己反转成「旧→新」**再渲染。空对话是 `200 + []`，**不是** 4043。
8. **Mock 与真实实现同签名、同 `data` 结构** —— [AGENTS §4.9](../../../AGENTS.md)。切换只靠 `VITE_USE_MOCK`，**调用方代码一行不改**。
9. **不引入新依赖** —— 纯 Vue + Element Plus + 原生 `fetch`。
10. **不出现 `any`** —— `catch` 到的是 `unknown`，必须收窄；Mock 与真实实现共用同一套类型。
11. **不传 `user_id`** —— 后端从 Token 取。只传 `personaId`。
12. 注释遵守 [AGENTS §5.1](../../../AGENTS.md)。

---

## 5. 验收标准

> ⚠️ 后端 `POST /chat/stream` 与 `GET /chat/personas/:personaId/messages` **均未交付**（`ai-service/` 目录都不存在）。
> **本支全部验证都在 Mock 通道下完成**，一律标注「待后端就位后复验真实通道」，**不勾 ✅**。
> 沿用 auth-login spec §3.3 与 frontend-auth-pages spec §5 的口径：宁可留空，不把没跑过的算成通过。

### 5.1 Mock 通道可完整验证

- [ ] `npm run typecheck`（`vue-tsc --noEmit`）零错误
- [ ] `npm run build` 通过
- [ ] `VITE_USE_MOCK=true` + `npm run dev` 启动，访问 `/chat` 不再是「待实现」
- [ ] 侧栏列出人设（Mock 提供 3 个），点击可切换；切换后消息区内容随之改变
- [ ] 输入文字 + Enter → 用户消息立即上屏（乐观更新），随后 AI 回复**逐字出现**
- [ ] 首个 `delta` 到达前显示 `TypingIndicator` 等待态
- [ ] 流结束后消息留在列表，输入框恢复可用
- [ ] 发送中再次 Enter **不会**重复发送（或明确被忽略）
- [ ] **人为往 Mock 的流里插一个 `error` 事件** → 页面上出现可见的错误提示 + 重试按钮，**不是静默失败**
- [ ] **把一个 `delta` 事件的 JSON 从中间切开**（Mock 支持分片配置）→ 解析仍正确，回复内容完整无缺字
- [ ] 发送后立刻切走路由（如点侧栏另一个人设）→ 流被中断，console 无「组件已卸载仍更新」警告
- [ ] 一个人设都没有时 → 显示空状态 + 按钮跳 `/personas`
- [ ] 历史消息：Mock 返回**倒序**数组 → 页面上显示顺序是**旧在上、新在下**
- [ ] 空对话（返回 `[]`）→ 正常显示空对话态，**不报错、不显示 4043**

### 5.2 待后端就位后复验

- [ ] `VITE_USE_MOCK=false` 时同样能逐字出现（真实 SSE）
- [ ] DevTools → Network → `stream` 请求 → **EventStream 标签页能看到 `delta` / `done` 逐条到达**（[MASTER §6](dev/MASTER.md) 第 6 步的通过判据）
- [ ] 真实 `4043`（人设不属于本人）→ 页面提示后返回或引导
- [ ] 未登录访问 `/chat` → 被守卫弹回 `/login`（既有行为，本支不得回归）
- [ ] 主动消息（`isNudge=true`）到达 → **渲染为 AI 侧**，且侧栏显示未读红点

---

## 6. 变更记录

| 日期 | 版本 | 变更内容 | 改动人 |
|------|------|----------|--------|
| 2026-09-22 | v1 | 初始版本 | 成员 2 |

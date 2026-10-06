# 聊天页（frontend-chat-view）· 实施计划

> 配套合同：[spec.md](spec.md)。本文只记录**怎么做**与**为什么这么做**。
> 注释纪律见 [AGENTS §5.1](../../../AGENTS.md)：代码里只留「为什么」，讲解留在本文。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-chat-view` |
| 状态 | 🚧 进行中 |
| 依赖 | 无（`develop` `b05a311` 切出） |
| 规模 | 10 个源文件（9 新增 + 1 修改）+ 2 篇文档 |

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 状态 |
|---|------|------|:----:|
| 1 | 写 `spec.md` 与 `plan.md` | 2 篇文档 | ✅ |
| 2 | `types/chat.ts`（消息与流事件的类型） | 新增 | ⏳ |
| 3 | `api/chat.ts`（SSE 解析 + 历史消息） | 新增 | ⏳ |
| 4 | `api/mock/chat.ts` + `api/mock/index.ts`（同签名 Mock + 开关） | 新增 | ⏳ |
| 5 | `stores/chat.ts`（列表、发送、打字机、红点） | 新增 | ⏳ |
| 6 | `components/chat/` 三个组件 | 新增 | ⏳ |
| 7 | `views/chat/ChatView.vue` | 新增 | ⏳ |
| 8 | 路由表换 `component`（1 行） | 改 `router/index.ts` | ⏳ |
| 9 | 自查：`typecheck` / `build` / 浏览器逐条过 spec §5.1 | 证据 | ⏳ |
| 10 | 提交：文档一笔、代码一笔 | 2 笔 | ⏳ |
| 11 | **v4 增量**：回前台刷新（store 新 action + `visibilitychange` / `focus` 监听） | 改 2 个源文件 | ⏳ |

**顺序理由**：类型 → 请求 → Mock → store → 组件 → 视图 → 路由，自下而上。最关键是**第 8 步必须最后做**：路由表引用的文件不存在时 `vue-tsc` 报 TS2307、构建直接失败（3-B 与 frontend-auth-pages 都踩过同一个坑）。

---

## 2. 文件清单

| 文件 | 类型 | 说明 |
|------|------|------|
| `frontend/src/types/chat.ts` | 新增 | `ChatMessage` / `StreamChatPayload` / 三种事件 payload |
| `frontend/src/api/chat.ts` | 新增 | `streamChat()` / `getMessages()` |
| `frontend/src/api/mock/chat.ts` | 新增 | 与上面**同签名**的 Mock |
| `frontend/src/api/mock/index.ts` | 新增 | `VITE_USE_MOCK` 分流出口 |
| `frontend/src/stores/chat.ts` | 新增 | 会话状态与动作 |
| `frontend/src/views/chat/ChatView.vue` | 新增 | 对话页骨架 |
| `frontend/src/components/chat/MessageBubble.vue` | 新增 | 消息气泡 |
| `frontend/src/components/chat/ChatInput.vue` | 新增 | 输入框 |
| `frontend/src/components/chat/TypingIndicator.vue` | 新增 | 等待态 |
| `frontend/src/router/index.ts` | 修改 | `chat/:personaId?` 的 `component` 换成真实懒加载 import |
| `docs/specs/frontend-chat-view/spec.md` | 新增 | 合同 |
| `docs/specs/frontend-chat-view/plan.md` | 新增 | 本文 |

`views/chat/`、`components/chat/`、`api/mock/` 三个目录本支首次创建。

---

## 3. 关键实现要点

### 3.1 为什么 `api/chat.ts` 不走 `request.ts` 的 axios 实例

`request.ts` 是我们**所有普通接口**的统一出口（自动带 token、统一解包、4012 自动刷新重放）。但 `streamChat()` **必须绕开它**，直接用原生 `fetch`：

1. **axios 会把整个响应缓冲完再交给你。** 它的 `response.data` 是一个「完整的、已解析」的对象——而 SSE 的全部价值在于**边收边用**。等 axios 返回时流已经结束了，打字机效果无从谈起。
2. **要读的是字节流，不是 JSON。** `fetch` 给的 `response.body` 是 `ReadableStream<Uint8Array>`，`getReader()` 能一段一段地读。这是浏览器唯一能逐块消费响应的接口。

**但这样就丢掉了 `request.ts` 的三件事**，必须手工补回来（这是本支最容易漏的部分）：

| `request.ts` 原本负责 | `streamChat()` 要手工做 |
|---|---|
| 请求拦截器自动加 `Authorization` | 自己从 `authStore` 取 `accessToken` 拼头 |
| 响应拦截器解包 / 抛 `ApiError` | 自己判断 `response.ok`，构造 `ApiError` |
| 4012 自动刷新后重放 | **本支不做**（见 §3.7） |

> 代码里要给这三件事写一句「为什么」注释：说明「此处刻意不复用 request.ts，因为 axios 会缓冲整个响应、破坏流式」。

### 3.2 半行解析：本支最容易写错的一点

SSE 的字节流**可以在任意位置被切开**。服务端发的是：

```
event: delta\ndata: {"text":"辛苦"}\n\nevent: delta\ndata: {"text":"了"}\n\n
```

但浏览器给你的 chunk 可能长这样（`|` 是切点）：

```
chunk 1:  event: delta\ndata: {"tex|      ← 事件块没结束
chunk 2:  t":"辛苦"}\n\nevent: delta\nda|    ← 补完上一个，又切下一个
chunk 3:  ta: {"text":"了"}\n\n              ← 补完
```

**错误做法**：每个 chunk 直接 `JSON.parse` → 必然在 chunk 1 和 2 上崩，且报的是「Unexpected end of JSON input」这种和真实原因毫不相干的错。

**正确做法**——维护一个跨 chunk 存活的 buffer：

```ts
let buffer = ''

while (true) {
  const { done, value } = await reader.read()
  if (done) break

  // 用 { stream: true } 让解码器也保留跨 chunk 的半截多字节字符
  buffer += decoder.decode(value, { stream: true })

  // 按空行切事件块；最后一段可能不完整，留在 buffer 里
  const blocks = buffer.split('\n\n')
  buffer = blocks.pop() ?? ''

  for (const block of blocks) {
    // 解析 event: / data: 两行
  }
}
```

两个容易漏的细节：

- **`decoder.decode(value, { stream: true })` 的 `{ stream: true }` 是必需的**。中文一个字符占 3 字节 UTF-8，同样可能被切在中间。不加这个参数，`TextDecoder` 会在 chunk 边界把半个汉字解成 `` ——表现是**偶发乱码**，而且只在中文回复长到跨 chunk 时才出现，极难复现。
- **`buffer = blocks.pop() ?? ''` 这一句要写 `?? ''`**。`pop()` 在空数组上返回 `undefined`，TS `strict` 下不写 `?? ''` 会报类型不匹配（这正是想要的效果——逼你处理它）。

> `.split('\n\n')` 里的 `\n\n` 是 SSE 的**事件分隔符**（规范里 CRLF 也合法，但契约 §6 的例子用的是 `\n`；为稳妥，解析前可先统一 `\r\n` → `\n`）。

### 3.3 三种事件的处理，以及为什么未知事件要静默忽略

契约 §6 冻结了三种事件，各自的动作是：

| event | data | 动作 |
|---|---|---|
| `delta` | `{text}` | 追加到当前 AI 消息的 `content` 末尾（打字机） |
| `done` | `{messageId}` | 结束流；把临时消息的 id 补成真实 id |
| `error` | `{code, message}` | 结束流；**在 UI 上显示错误 + 重试按钮** |

**未知事件名必须静默忽略，不能抛错。** 理由：契约 §6 写着「事件类型不可私自增删」，那是约束**后端**不许随手加；但**前端**面对一个不认识的 `event: ping` 时，正确反应是跳过它继续读，而不是整个流中断。这是流协议的通行做法——**对扩展宽容，对内容严格**。

**`error` 事件的特殊性**：它出现时 **HTTP 状态码仍然是 200**（契约 §6 原文：「HTTP 状态仍为 200，错误通过事件传递」）。所以**绝不能**靠 `response.ok === false` 来判断失败——请求从 HTTP 角度看是成功的，失败信息藏在事件里。这个设计的好处是：连接建立之后才发生的错误（比如 LLM 中途挂了）也能用同一个通道告诉你，而不用去改已经发出去的 HTTP 状态码。

### 3.4 `done` 事件为什么不用它带的内容

`done` 只带 `messageId`，**不带全文**。所以本地那条 AI 消息的 `content` 是**我们自己一个 delta 一个 delta 拼出来的**。

这里有个容易被忽略的后果：**拼接结果与后端落库的内容可能有细微差异**（我们丢过一个未知事件，或最后一个 delta 没读完）。所以 `done` 到达时，**不做「用后端全文覆盖本地文本」的操作**（契约也没给全文），只把 `messageId` 补上。若将来发现内容不一致，正确做法是**重新拉一次历史消息**，而不是假设本地拼的一定对。

### 3.5 Mock 怎么做才是「同签名、同结构」

[MEMBER_2_FRONTEND §3](../../dev/MEMBER_2_FRONTEND.md) 要求 Mock「后端未就绪时页面可跑」，[AGENTS §4.9](../../../AGENTS.md) 要求「与真实实现返回同样的 `data` 结构」。做法：

**`api/mock/index.ts` 只做一件事——分流**：

```ts
import * as real from '@/api/chat'
import * as mock from '@/api/mock/chat'

// 只在这里判断开关，业务代码永远 import 这个出口
export const chatApi = import.meta.env.VITE_USE_MOCK === 'true' ? mock : real
```

调用方一律 `import { chatApi } from '@/api/mock'`，**永远不出现 `if (USE_MOCK)` 的分支**。这样切开关时调用方代码一行不改（这正是 spec §4.8 的要求）。

**Mock 也要走真实的解析路径**——这是本支 Mock 和普通「假数据」最大的区别：

Mock 的 `streamChat()` **不直接回调**，而是构造一个真正的 `ReadableStream`，把 SSE 文本按分片 `enqueue` 进去，然后**复用与真实实现完全相同的解析函数**。收益有两条：

1. **§3.2 的半行解析逻辑在 Mock 下就被真实检验了。** 如果 Mock 直接 `onDelta('辛苦')`，那解析代码一行都没跑过，等后端就位才发现切分写错——那就是 Mock 最坏的用法（给了虚假信心，[frontend-auth-pages plan.md §3.6](../frontend-auth-pages/plan.md) 已经点过这个风险）。
2. **能给 §5.1 的验收项提供手段**：Mock 支持一个「分片大小」参数，设成 1 字节就能把每个事件切得七零八落；再加一个「注入 error 事件」开关，用来验错误路径。

Mock 数据本身要覆盖几种边界：**3 个人设**（侧栏切换）、**一段倒序的历史消息**（验反转）、**一个空对话的人设**（验空态）。

### 3.6 历史消息为什么要自己反转

契约 §5 明确 `GET /chat/personas/:personaId/messages` **倒序返回（最新在前）**。后端这么做是为了配合**分页**——第一页就是最新的 20 条，越往后翻越旧，符合「先看最近的」这个真实需求。

但**界面的阅读顺序是「旧在上、新在下」**（微信、iMessage 都是这样）。所以 `getMessages()` 拿到数组后要 `reverse()` 再交给 store 渲染。

**这个反转放在 `api/chat.ts` 里做，不放在组件里。** 理由：这是**接口语义与界面语义的翻译**，属于请求层的职责；放在组件里，将来第二个消费点（比如历史消息弹窗）就要记得再反转一次，忘一次就显示反了。

两个配套的坑：

- **空对话返回 `200 + []`，不是 `4043`**。契约 §5 写得很清楚。`[]` 反转还是 `[]`，正常显示空对话态；**不要**把空数组当错误处理。
- **`page` / `pageSize` 会被后端钳制**（默认 20、上限 100），**且非法值不返回 `4001`**——你传 `pageSize=99999` 它默默给你 100。所以前端**不要**指望「传了就按传的来」，也不要做「先查最大值」这种多余的事。

### 3.7 `streamChat` 不做 4012 刷新重放（本支唯一的功能缺口）

`request.ts` 对普通请求有一套完整的令牌续期：4012 → 刷新 → 重放原请求。`streamChat()` **不复用这套**，因为：

1. **重放一条已经在流的请求语义可疑**：已经吐出去的 delta 怎么办？重放会让用户看到重复内容。
2. **实现成本高**：要在 `fetch` 层重做「暂停流 → 刷新 → 用新 token 重新发起 → 丢弃旧流的后续数据」。这是一整套状态机，不是本支该顺手塞进来的。
3. **它的发生概率在本支是零**：Mock 通道下根本不存在 token 校验。

**本支的处理**：`streamChat()` 在 `response.ok === false` 时抛 `ApiError`（`code = ErrInternal`，message 含 HTTP 状态码），由 store 的 `catch` 落到页面错误条 —— **不对 4010 / 4011 / 4012 做分流**，也不触发 `request.ts` 的 `forceLogout()`。**如实记为缺口**，写进 spec §5.2 待复验项。

> ⚠️ **按实现订正**：本节早先写「由 `ChatView` 提示『登录已失效，请重新登录』」，但实现未按状态码分流，提示是通用的「对话请求失败（HTTP 4xx）」+ store 的兜底文案。**保持现状**——Mock 通道下该分支不可达，等后端 SSE 就位后连同「刷新重放」一起设计，现在改提示文案等于猜。

> 真要做的时间点是**后端 SSE 就位之后**：那时才知道刷新请求能否与进行中的流并存，现在设计等于猜。

### 3.8 打字机的「临时消息」怎么管理

流式渲染有个状态管理问题：**AI 的回复不是一次性到来的**。做法是：

1. 用户点发送 → 立刻**乐观更新**：往列表 push 一条 `role: 'user'` 的消息（不等后端，否则输入后有一段无反馈的空窗）。
2. 同时 push 一条**占位的 AI 消息**，`content: ''`，并记下它的 id（用本地生成的**临时负数 id**，见下）。
3. 每个 `delta` 到达 → 找到那条占位消息，`content += text`。Vue 的响应式会自动重渲染。
4. `done` 到达 → 把占位消息的临时 id 换成后端给的 `messageId`。

**临时 id 为什么是「负数」而不是字符串**：契约里 `ChatMessage.id` 是 `number`，写成 `temp-<时间戳>` 会直接和类型冲突。改用**模块级递减计数器取负**（`-1` / `-2` / …）：真实 id 由数据库自增、必定为正，于是「id 为负 = 还没落库」这条判据**不需要额外字段**，契约类型也不用动。
> ⚠️ 最初打算用 `-Date.now()`，但**同一毫秒内连造两条消息**（用户消息 + AI 占位）**会撞 id**，所以换成计数器。

**流结束时的三件收尾（都在 `finally` 里）**：
- **删掉「一个字都没流出来」的占位**（`content === ''`）——只有一开始就失败、或被直接中断时成立。已经流出一部分的**保留**：用户已经看到的字不该凭空消失（因此本支的中断**不做回滚**）。
- **只有「我还是当前那条流」才复位 `isStreaming`**——比对本次的 `AbortController` 引用。否则旧流结束时会把用户已经切走、并新开的那条流一起解禁，输入框提前可用。
- **占位 id 要用可变引用记住**——`done` 会把它换成真实 `messageId`，收尾时的清理必须跟上换过之后的 id。

**为什么不用一个 `streamingText` 变量、结束后再 push 成消息**：那样消息列表与「正在流的内容」是两套东西，滚动定位、气泡样式、回车禁用都要写两遍。放进同一个列表里，**渲染逻辑只有一套**。

**`isNudge` 的处理**（2026-10-06 变更）：注入行**不进消息列表**——过滤在 store 层（`loadMessages` 赋值处 `list.filter((item) => item.isNudge === false)`），气泡因此只需按 `role` 判断「我发的」。原方案「当普通 AI 消息渲染 + `isNudge` 视觉区分」已废弃：注入行的 `content` 就是字面 `[nudge]`（[proactive-message §1](../proactive-message/spec.md)），渲染出来是机器标记而非对话内容；留在 `messages` 里还会干扰 `retry()` 按 `role` 摘尾巴的判据（注入行的 `role` 也是 `user`）。`readAt` 校准仍用**过滤前**的原始数组（`lastMessageAt` 口径含注入行）。同步见 spec §5.2 第 5 条。

### 3.9 未读红点为什么不做成「独立功能」

[MEMBER_2_FRONTEND §3](../../dev/MEMBER_2_FRONTEND.md) 讲得很直接：

> 该人设有未读的主动消息时，名字旁显示红点，打开后清除（**日程提醒到点后也是一条普通消息，红点逻辑直接复用，不用为它加分支**）

所以红点是 **store 里的一个字段**（`Map<personaId, boolean>` 或给每个人设加 `hasUnread`），不是一套消息类型系统。要抵制的诱惑：给它设计 `type: 'unread'`、给它单独的事件、给它独立的接口——**都不需要**。本支只做「有/无」两态。

> 主动消息的后端（`/proactive/trigger`、定时任务）本支完全不碰，红点由 `POST /proactive/trigger` 或定时任务触发均可（PR #64 后）。

### 3.10 为什么这支不拆成两支（对 AGENTS §6 的偏离说明）

[AGENTS §6](../../../AGENTS.md) 约定「一支 = 一功能 = 3-8 文件」，本支 10 个源文件**超了**。为什么不拆：

**能拆的方案**：先一支做「SSE 解析 + Mock + store」（无 UI），再一支做「界面」。**不采纳**，三条理由：

1. **第一支没有可验证的判据。** 拆出来的「解析层」只能靠单测证明——而本项目前端**没有测试框架**（`package.json` 里没有 vitest/jest，只有 `vue-tsc` 和 `vite build`）。那么第一支的验收只能是「typecheck 通过」，这等于什么都没验。
2. **解析逻辑的正确性只能在界面上看出来。** 「逐字出现」是唯一有说服力的证据；`delta` 拼接、半行切分、`error` 落 UI——这些缺陷全都是**视觉可见、类型不可见**的。分开做会得到一个「类型全对但行为全错」的中间态。
3. **Week 2 生死线就在本周。** 今天是 Tuesday，Week 2 只剩 5 天（[MASTER §7](dev/MASTER.md)）。拆成两支要走两轮 PR + AI 审计，时间成本换不到任何质量收益。

**补偿措施**：把规模换来的风险用**更细的自查清单**抵掉——spec §5.1 列了 15 条可本机验证的项，其中「人为注入 error」「把 delta 从中间切开」两条是本支专属的**破坏性验证**，专门用来覆盖最容易偷工减料的地方。

### 3.11 回前台为什么不能直接「重拉一次」

**问题**：主动消息由后端触发并落库，前端不知道。停在对话页不动看不到；**切走再回来也看不到**——`selectPersona` 的早退分支（PR #68）只在**路由参数变化**时执行，而「切标签页」「切窗口」不改路由。

**做法**：`ChatView` 监听 `document` 的 `visibilitychange` 与 `window` 的 `focus`，回前台时交给 store 的一个新 action（`refreshActiveConversation()`）去 `loadPersonas()` + `loadMessages(currentPersonaId)`。

**为什么收口到 store，而不是直接写在组件里**：判据是 `isStreaming`——它是 **store 的 state**。组件读它做业务决策，等于把「什么时候不能刷」这条规则散到视图层；收口到 action 之后，将来任何新的刷新入口（轮询、手动刷新按钮）复用同一个判据，不会各自漏一次。这也是 PR #69 定下的「store 是唯一收口」。

**⚠️ 必须加的三道守卫：**

1. **`isStreaming` 时整体跳过。** `loadMessages` 是**整份替换** `this.messages`，而流式的占位消息是本地造的**负数 id**（§3.8），服务端列表里根本没有它。替换掉之后：

   ```
   占位消失 → onDelta 的 messages.find(id === placeholderId) 返回 undefined
            → 每个 delta 静默丢弃（onDone / finally 也找不到占位）
            → 用户看到：一个字都没出来就没了
   ```

   而且 `isStreaming` 会**正常复位、不报错**——界面上看不出任何异常。这是本次改动里唯一会造成**数据静默丢失**的点，必须挡在第一步。

   > 更彻底的做法是「合并」而不是「整份替换」（保留本地负数 id 的消息）。**不采纳**：合并要处理同 id 覆盖、顺序、去重，复杂度远超收益；一次流通常只持续几秒，跳过这一次刷新没有可感知的代价。

2. **`focus` 与 `visibilitychange` 都要监听，但要防重入。** 一次「切走 → 切回」可能连着触发两个事件：

   | 场景 | `visibilitychange` | `window.focus` |
   |---|---|---|
   | 切到别的标签页再切回 | ✅ | ✅ |
   | 点别的窗口再点回（浏览器**未被完全遮挡**） | ❌ **不触发** | ✅ |
   | 浏览器被完全遮挡 | ✅ | ✅ |

   第 2 行是**演示场景**（去 Postman / curl 触发主动消息再切回），只监听 `visibilitychange` 会整个漏掉。但两个都监听就会重复请求 → 用一个 **in-flight 标志**防重入（第二次事件在第一次请求未完成时直接返回）。

   > **不用时间节流**：演示时「切走 → 触发 → 切回」可能只隔几秒，节流窗口设大了会把演示本身挡掉；设小了又挡不住双触发。in-flight 标志天然对齐「同一件事不并发做两次」这个真实约束。

3. **`onBeforeUnmount` 里解绑。** 组件卸载（切去 `/personas` 等）后监听器若还挂着，之后每次切窗口都会对**已卸载的组件**发请求。`ChatView` 已有 `onBeforeUnmount`（绑着 `stopStreaming`），解绑放在同一个 hook 里。

**顺带把一个口径钉死**：回前台刷新**只覆盖「离开又回来」**。用户一直盯着页面不动，主动消息到了仍然看不到——要真「实时」得轮询或长连接，而 `POST /chat/stream` 是一次性请求流、不是常驻订阅。这条已写进 spec §2.2 的「不做什么」，免得被读成「做完就实时了」。

**已知残余竞态（P3，本支不修）**：两道守卫都只在 `refreshActiveConversation` 的入口判。
`loadPersonas()` 返回后复判 `isStreaming` 已挡掉主场景（用户在列表往返期间发消息，
尤其「切走前已打好字、切回直接按回车」——`ChatInput` 的 `draft` 是组件本地状态，
所以这个场景比想像中更容易触发），但 `loadMessages` 内部还有一次 `getMessages` 往返：
若用户恰好在这期间发出消息，整份替换仍会洗掉流式占位、导致 delta 静默丢弃（毫秒级窗口）。
更彻底的做法是把守卫下沉到 `loadMessages` 里替换 `this.messages` 之前 —— 那会同时覆盖
`selectPersona` 早退分支的同款隐患；但它动的是 #69 刚定稿的 `loadMessages`，且顺带修的是
另一支引入的问题，按「一支一功能」另开 issue 处理。

---

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| 后端 SSE 未交付，`ai-service/` 目录都不存在 | 真实通道完全无法验证 | 走 Mock 通道；spec §5.1（本机可验）与 §5.2（待后端）**分列，不混勾** |
| **Mock 通过 ≠ 真实能通** | 最坏情况：Mock 给了虚假信心，后端就位才发现字段/事件名对不上 | §3.5：Mock **复用真实解析函数**、走真 `ReadableStream`；Mock 的 `data` 结构直接照契约 §6 抄，不自创 |
| 半行解析写错 | 偶发乱码 / JSON 解析崩溃，且难以复现 | §3.2 的 buffer + `{ stream: true }`；Mock 提供**分片配置**做破坏性验证（spec §5.1） |
| `error` 事件被静默吞掉 | 用户看到「回复卡住不动」，无任何提示 | spec §4.3 硬约束 + §5.1 专项验证（人为注入 `error`） |
| 把 `emotionLabel` 渲染出来 | 违反契约 §5 的「内部信号不上界面」，答辩时被追问会很尴尬 | spec §2.2 明确排除；**类型定义里干脆不留这两个字段**（从源头杜绝） |
| 越过边界改了成员 3 的文件 | 与 `views/persona/` 等产生冲突 | spec §2.2 列出不属于本支的目录；侧栏只读人设、不提供增删改 |
| 未登录时流的 401 处理不完整 | 用户卡在一个永远不动的流上 | §3.7 如实记为缺口，spec §5.2 待复验；先做「提示 + 引导重新登录」 |
| 本支文件数超 AGENTS §6 约定 | 审计时被质疑范围失控 | §3.10 写明偏离理由 + 补偿措施，主动在 PR 描述里说明 |
| `VITE_USE_MOCK` 开关误留 `true` 提交 | 生产环境跑 Mock 数据 | `.env.example` 里默认写 `false`；开关只写进 `.env`（不入库） |
| **回前台刷新打断进行中的流** | **每个 `delta` 静默丢弃**，用户看到「回复凭空消失」，且不报错、`isStreaming` 正常复位 → 既难复现也难定位 | §3.11 守卫 1（`isStreaming` 时整体跳过）；spec §4.13 立为硬约束 |
| `focus` + `visibilitychange` 双触发 | 一次切回发两轮请求（`/personas` 与 `/messages` 各 2 次） | §3.11 守卫 2（in-flight 标志防重入） |

---

## 5. 进度记录

| 日期 | 步骤 | 说明 |
|------|------|------|
| 2026-09-22 | 1 | 写 `spec.md` + `plan.md`；分支自 `develop`（`b05a311`）切出 |
| 2026-09-27 | — | `types/chat.ts`、`api/chat.ts`（抽出 `consumeStream` / `toChronological`）、`api/mock/{chat,index}.ts` 完成 |
| 2026-09-29 | — | rebase 到 `751e8c3`（PR #47 合并后）；`api/mock/persona.ts` 取代本地 `MockPersona`；`stores/chat.ts` 完成，§3.8 临时 id 方案订正为负数计数器 |
| 2026-09-29 | 5 | `components/chat/{TypingIndicator,MessageBubble,ChatInput}.vue`、`views/chat/ChatView.vue` 完成；`router/index.ts` L35 接入 `ChatView`；本机经 Mock 通道实测**跑通** |
| 2026-10-06 | 11 | v4 增量：新增「离开又回来时刷新」（store `refreshActiveConversation()` + `ChatView` 监听 `visibilitychange` / `focus`）；同步 spec §2.2 / §4 第 13 条 / §5.3 |

> 本机实测使用**手动写入的假登录态**（浏览器 Console 写 localStorage，key `heart-echo-auth`）——原因是本机无后端在跑（`localhost:8080` 无监听），而 `VITE_USE_MOCK` 只覆盖 `chatApi` 与 `listPersonas` 两个出口，**不含 `auth`**。这是本机调试手段，不代表跳过鉴权；验收清单中依赖真实响应头的项（spec §5.2）不受此影响。

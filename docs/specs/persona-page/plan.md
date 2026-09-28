# 人设管理页面（persona-page）· 实施计划

> 配套合同：[spec.md](spec.md)。本文只记录**怎么做**与**为什么**这么做。
> 注释纪律见 [AGENTS §5.1](../../../AGENTS.md)：代码里只留「为什么」，讲解留在本文与 `.learn/`。

| 项 | 值 |
|----|-----|
| 分支 | `feature/frontend-persona-page`（自 `b05a311`，与 `develop` 同点切出） |
| 状态 | ✅ 实现完成、读数已回填（spec §6 A/B 全绿，C 组未开始），**待人工审查 + 开 PR** |
| 后端依赖 | persona CRUD 4 个端点 **已合并且已实测可用**（无阻塞） |
| 规模 | 4 个源文件新增 + `router/index.ts` 改 1 处（实测 `6+/1-`，拆行原因见 spec §2.1）+ 2 篇文档 + 1 篇 `.learn/` |

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 状态 |
|---|------|------|:----:|
| 1 | 探针实测 4 个端点，确认响应形状与边界行为 | spec §5 的 9 条读数 | ✅ |
| 2 | 写 `spec.md` 与 `plan.md` | 2 篇文档 | ✅ |
| 3 | `types/persona.ts` | 新增 | ✅ |
| 4 | `api/persona.ts` | 新增 | ✅ |
| 5 | `views/persona/PersonaFormDialog.vue` | 新增 | ✅ |
| 6 | `views/persona/PersonaView.vue` | 新增 | ✅ |
| 7 | 路由换 `component` | 改 `router/index.ts`（`6+/1-`） | ✅ |
| 8 | 自查：`typecheck` / `build` / 浏览器逐条过 spec §6 | 读数已回填 spec §6 | ✅ |
| 9 | `.learn/persona-page.md` | 学习笔记（不入库） | ✅ |

**顺序理由**：与 `frontend-auth-pages` 同因——路由表引用的文件不存在时 `vue-tsc` 报 **TS2307**、构建直接失败。所以第 7 步必须在第 6 步之后。第 3 步先于第 4、5、6 步：类型是另外三者的公共依赖。

---

## 2. 文件清单

| 文件 | 类型 | 说明 |
|------|------|------|
| `frontend/src/types/persona.ts` | 新增 | `Persona`（8 字段，契约 §4 逐字）、`PersonaPayload`（3 字段） |
| `frontend/src/api/persona.ts` | 新增 | `listPersonas` / `createPersona` / `updatePersona` / `deletePersona` |
| `frontend/src/views/persona/PersonaView.vue` | 新增 | 页面本体 |
| `frontend/src/views/persona/PersonaFormDialog.vue` | 新增 | 新建/编辑共用弹窗 |
| `frontend/src/router/index.ts` | 修改 | 1 处（实测 `6+/1-`：语义上只换 `component`，另 5 行是拆行） |
| `docs/specs/persona-page/{spec,plan}.md` | 新增 | 合同与本文 |
| `.learn/persona-page.md` | 新增 | 学习笔记（`gitignore:75`，不进仓库） |

**不新增目录**：`views/persona/` 是本支唯一新建的目录（`views/` 下已有 `auth/`，属同类页面目录，无需群里再确认）。`types/`、`api/` 均为已有目录追加文件。

---

## 3. 关键实现要点

### 3.1 为什么本支不做 Mock 层

`AGENTS §4.9` 与 `frontend/.env.example` 都提到 `VITE_USE_MOCK`，未合并的 `feature/frontend-chat-view` 也已经建了 `src/api/mock/{index,chat}.ts`，把开关出口放在 `api/mock/index.ts`。本支**仍然不做**，三条理由：

1. **后端已经可用**：4 个端点已合并，本机实测通过（spec §5）。Mock 的价值是「后端不存在时也能开发」，这个前提不成立。
2. **会撞车**：`api/mock/index.ts` 是那支新建的**同一个文件**。两支同点切出、都没合并，我再往这个文件里加 `personaApi` 出口，谁后合并谁解决冲突（AGENTS §4.8「不要两人同时改一个文件」）。跨分支的开关出口设计应当**由先合并的那支负责扩展**。
3. **会制造第二份真相**：mock 的 Persona 与 true 后端的 Persona 并行演化，最危险的不是「mock 写错」，而是「mock 先通过了」——等接真后端才发现字段名对不上，那时假的信心已经发下去了。

> 这不是永久决定。若 chat-view 先合并，本支的收尾动作是：**在它已有的 `api/mock/index.ts` 里补一行 persona 出口**，而不是另起一套。列为广播项（spec §6 分组 C）。

### 3.2 三态是一个互斥状态，不是三个布尔

`loading` / `error` / `empty` / `有数据` 四者互斥，用**求值顺序**表达，不用 `v-if` 互相打架：

```
loading  → 骨架屏
error    → 错误提示 + 重试按钮
空列表   → 空态 + 创建入口
否则     → 卡片栅格（+ 分页器）
```

两个容易写错的地方：

- **空态判据是 `list.length === 0`，不是 `total === 0`**。后端 `page` 超界时返回「空列表 + **真实 total**」（`persona-crud/spec.md` §4.2 第 5 条）。用 `total === 0` 判空，会在超界页上算出「总数 25 但显示空态」这种自相矛盾的画面——而它恰好在删除最后一页最后一项时出现。
- **`loading` 期间不清空 `errorMessage` 之外的旧数据**：重试时先 `loading = true`，请求回来再整体替换，避免闪成空态再闪回列表。

### 3.3 删除：`ElMessageBox` 的「取消」是 reject，不是错误

```ts
try {
  await ElMessageBox.confirm(...)
} catch {
  return   // 用户点取消/关闭：reject 表示"没确认"，不是失败
}
```

若不 catch，点取消会在 console 里留下一条 unhandled rejection——页面功能正常，但演示时开发者工具一开就见红。**这条必须有注释**（它是非显然的 API 语义，属 AGENTS §5.1 第 ①类「非显然约定」）。

确认文案**必须写明级联**（契约 §4 明确要求前端二次确认）：

> 删除「<名字>」后，它的全部对话与记忆都会一并删除，且无法恢复。

（上面这行是**实现后的实际字符串**，spec §6 分组 B #8 有浏览器实测读数；标题为「确认删除这个人设吗？」）

`confirmButtonClass: 'el-button--danger'` 让危险动作用红色按钮；`type: 'warning'` 给图标。

### 3.4 变更后**重新拉取**，不在本地改数组

新建 / 编辑 / 删除成功后一律 `await load(目标页)`，不做本地 `push` / `splice` / 就地赋值。

理由是**排序权在后端**：列表按 `last_message_at DESC NULLS LAST, id DESC`（spec §5 第 9 条实测）。本地插入要自己算出「这条该排第几」，而它取决于该用户有多少条「聊过」的人设——算错的表现是「刚建的人设在这一屏，刷新后跳走了」。重新拉取永远和后端一致，代价是多一次请求（卡片量级下可忽略）。

**建完为什么跳回第 1 页**：新的人设 `lastMessageAt` 为 `null`，按 `NULLS LAST` 排在「聊过的人设」之后。因此它**不一定在第一屏**——这是契约排序的必然结果，不是 bug。用成功提示（`ElMessage.success`）保证用户知道建成了，不额外做「跳到最后一页」的猜测。

### 3.5 分页器与「删掉本页最后一项」

`pageSize` 用 **20**（契约默认值，且 ≤ 上限 100）；`total > 20` 时才渲染 `el-pagination`（只有一页时分页器是噪音）。

删除成功后的页码规则：

```
删除前本页只有 1 项 且 page > 1  →  回到 page - 1
否则                             →  留在本页
```

没有这条规则会怎样：停在超界页 → 后端返回 `200` + 空列表 + 真实 `total` → 页面显示空态，而分页器上明明还有上几页。**这是「不报错、只失灵」的典型形态**（后端行为正确，前端把它渲染成了错误结论）。

### 3.6 表单：新建与编辑**共用一个弹窗**，但**复制字段而不是引用**

共用的理由不是「省代码」，是它们本来就是**同一个动作的两种入口**：同样 3 个必填字段、同一套校验、同一个 `PUT`/`POST` 语义（`PUT` 是整体替换，不是部分更新，所以「编辑」本质上就是「用新值整体提交」）。这与 `frontend-auth-pages/plan.md` §3.7「先等三个再抽象」不冲突——那里是**两个不同页面**，这里**是一个表单**。

复制而引用：打开编辑时必须 `form.name = persona.name` 逐字段拷贝，**不能把 `form` 指向列表里那条对象**。否则用户在弹窗里打字时，卡片上的文字会跟着变——请求还没发，界面已经「显示成功」了；若用户点取消，列表上还留着改动。

实测佐证（spec §6 分组 B 补充读数）：弹窗里改成 `小暖改` 时卡片仍是 `小暖`；编辑过 `小暖改` 后再点「新建人设」，三框字数计数全为 `0 / N`，没有残留。

**重置时机用 `watch(() => props.modelValue)`，不用 `@closed`**：`@closed` 要等关闭动画结束才触发，而「关掉再立刻打开」的连点场景下，第二次打开可能早于上一次的 `@closed`，留下上一次的红字与残值。在 `modelValue` 变 `true` 的那一刻（打开前）逐字段复制并 `nextTick(clearValidate())`，与打开动作同序，不依赖动画时序。

### 3.7 `4043` 要单独处理

`4043` 只有一个含义：**这个 id 现在不属于你，或者不存在**。它在两种真实场景下会出现：

- 用户在另一个标签页删掉了这个人设，本页再编辑/删除它；
- 列表停留在旧数据上，实际已被清空。

处理方式是**提示 + 刷新列表**（而不是当普通错误弹一下就完了）——用户的界面状态已经过期，只弹提示会让他反复点同一个不存在的卡片。后端文案「人设不存在」直接展示（一 code 一 msg，`AGENTS §4.9`）；网络错误（`NETWORK_ERROR_CODE`，即请求没到后端）才用兜底文案，因为那时 `message` 是 axios 的英文原文。

这个「取 `message` 还是取兜底」的判断收在 `api/persona.ts` 的 `toErrorMessage(error, fallback)` 里，**视图不 import `ApiError`**（spec §4 约束 3 要求组件不碰 `@/api/request`，而 `ApiError` 正是从那里导出的）。兜底文案按场景传，实测三条：

| 场景 | 兜底文案 | 实测（spec §6 补充读数） |
|---|---|---|
| 列表加载失败 | `人设列表加载失败，请稍后重试` | 停后端刷新时页面显示的就是这句 ✅ |
| 保存失败 | `保存失败，请稍后重试` | route 中断 `POST` 时弹窗里显示的就是这句 ✅ |
| 删除失败 | `删除失败，请稍后重试` | 未单独构造（同一函数、同一条分支） |

业务错误走 `error.message` 也有实测：用已删除的账号发 `POST` 拿到 500，弹窗里显示的是后端原文 `数据库操作失败`，不是兜底文案。

### 3.8 时间格式化先留在视图内

`lastMessageAt` / `createdAt` 都是 RFC3339 且**带 6 位小数**（`2026-09-28T13:53:16.172057+08:00`，spec §5 第 4 条实测）。所以格式化必须交给 `Date` 解析，**不能按字符串切片**（切 `slice(0,16)` 得到 `2026-09-28T13:53`，恰好能显示，但一旦后端改精度或改成 UTC，切片就静默错位）。

函数就放在 `PersonaView.vue` 内，**不建 `src/utils/`**：`AGENTS §3` 要求新增目录前群里说一声，而目前只有这一处用。等第三处出现（画像页 / 朋友圈页 / 日程页都要显示时间）再抽 `src/utils/datetime.ts`——与 `frontend-auth-pages/plan.md` §3.7 同一条规矩。

### 3.9 前端 trim：比后端严一点点

后端 `binding:"required"` 对字符串的判据是「非零值」，**一串空格不算零值**——也就是说 `"   "` 能建出一个名字看不见的人设卡片。前端在提交前 `trim()` 三个字段，并让必填校验的判据也变成「trim 后非空」，使「只输入空格」在本地就被拦下。

落地方式：`requiredText(label)` 生成一条 `required: true` + 自定义 `validator`（`value.trim() === ''` 即 `callback(new Error(...))`）的规则，**判据与文案写在同一处**（`请输入人设名` 里的「人设名」由参数传入）。

选 `validator` 而不是 `transform: v => v.trim()` 的理由是**文案要按字段不同**，不是「`transform` 会改输入框」——后者是个常见误解，实际**不会**：`async-validator@4.2.5` 的 `dist-node/index.js:1118-1122` 在写入 transform 结果之前先 `source = _extends({}, source)` 拷贝了一份；而 Element Plus 又是**逐字段**校验、另建包装对象传进去（`form-item.vue_vue_type_script_setup_true_lang.mjs:132` 的 `.validate({ [modelName]: fieldValue.value })`），`form` 被隔了两层。两种写法都拦得住空格，只是 `transform` 只剩下一条 `required` 的默认文案可用。（详细推导见 `.learn/persona-page.md` §3.3）

代价与边界要说清楚：**这是体验优化，不是正确性**。前端拦不住构造请求的人，后端也仍然接受空格名（本支不改后端）；若 reviewer 认为不该比后端严，把三处 `requiredText(...)` 换回 `{ required: true }` 即可，不影响其余逻辑。

实测（spec §6 分组 B #4/#5）：三框全空 → 三条错误、0 条 `POST`；隔离用例（只有名字是 `"   "`）→ 唯一错误 `请输入人设名`、0 条 `POST`。

### 3.10 `state` 的 TS 类型与「不读」纪律

契约要求前端**不解析、不回传** `state`。类型写成 `state: Record<string, unknown>`（诚实反映「一个不透明的 JSON 对象」），并且**代码里一次都不读它**：不显示、不判断、不进请求体。

不写 `any`：`AGENTS §5` 明令禁止；`Record<string, unknown>` 既禁止了误用，也不需要类型断言。

---

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|------|------|------|
| `views/persona/PersonaView.vue` 先提交、路由后改 | 构建失败（TS2307）——3-B 与 auth-pages 都踩过 | §1 的顺序：视图先落地，路由最后改 |
| 用了 Element Plus 图标（`<Plus />`） | 编译失败：`@element-plus/icons-vue` **不在依赖里** | 按钮一律文字；spec §3.1 已写明 |
| 空态用 `total === 0` 判 | 超界页显示「总数 N 但空列表」的矛盾画面 | §3.2：判据是 `list.length === 0` |
| `ElMessageBox` 取消未 catch | console 里 unhandled rejection | §3.3 的 `try/catch { return }` |
| 编辑时把表单指向列表对象 | 未提交先改卡片；取消后列表留着脏值 | §3.6：逐字段复制 |
| 本地插排、不重拉 | 后端排序（`NULLS LAST, id DESC`）与本地算出的位置不一致 | §3.4：一律重拉 |
| 与 chat-view 分支在 `api/mock/index.ts` 撞车 | 合并冲突、两人同改一个文件 | §3.1：本支不碰该文件，列为广播项 |
| 把 `familiarity` 当成情绪展示 | 触碰 `AGENTS §4.4` 红线（情绪不得渲染） | spec §2.1：`familiarity` 是人格状态，与情绪无关；本页不展示任何情绪字段 |
| 网络错误直接显示 axios 英文 `message` | 用户看到 `Network Error` | §3.7：`NETWORK_ERROR_CODE` 走中文兜底文案 |

---

## 5. 进度记录与实测读数

### 5.1 探针原始读数（2026-09-28，本机真跑）

命令：`cd backend && go run ./cmd/server`；探针脚本用 `urllib` 逐个端点真发请求，注册 `probe0928` 后建 2 改 1 删 1；**跑完已级联删除该用户**（`persona=1→0`、`proactive_setting=1→0`）。探针脚本与清理脚本都在临时目录，**均不入库**。

```
register:        200  {"code":200,...,"data":{"accessToken":"..."}}
no-token list:   401  {"code":4010,"message":"未登录或登录已过期","data":null}
empty list:      200  {"list":[],"total":0,"page":1,"pageSize":20}
create:          200  {"id":2,...,"state":{"familiarity":0},"familiarity":0,"lastMessageAt":null,
                       "createdAt":"2026-09-28T13:53:16.172057+08:00"}
create:          200  {"id":3,...}
list(paged):     200  {"list":[{"id":3,...},{"id":2,...}],"total":2,"page":1,"pageSize":100}   ← pageSize 1000 → 100
update:          200  {"id":2,...,"state":{"familiarity":0},"familiarity":0}   ← body 里的 state/familiarity 被忽略
update missing:  4043
update bad id:   4043                                                ← /personas/abc
create missing:  4001                                                ← 只传了 name
delete:          200  {"code":200,"data":null}
delete again:    4043
final list:      200  {"list":[{"id":2,"name":"小暖2",...}],"total":1,...}
```

结论逐条见 spec §5 的「对前端的含义」列。

### 5.2 步骤进度

| 日期 | 步骤 | 说明 |
|------|------|------|
| 2026-09-28 | 1 | 探针实测 4 个端点；发现并记录「`pageSize` 收敛」「`PUT` 忽略 `state`」「删除不幂等」三条与直觉不同的读数 |
| 2026-09-28 | 2 | `spec.md` + `plan.md` 落地 |
| 2026-09-28 | 3–7 | 4 个源文件 + 路由落地；`typecheck` exit 0、`build` exit 0（`PersonaView` 单独成 chunk） |
| 2026-09-28 | 8 | 浏览器逐条过 spec §6 B 组 13 项，全部拿到读数；A2 的 9 条锚定检查在真实产物上跑过（含一次**裸词失真**的实拍，见 spec §6 A2 #8）。过程中真停过一次后端测 error 态，随后重启 |
| 2026-09-28 | 9 | `.learn/persona-page.md`（不入库） |

### 5.3 验证过程中的两个方法论教训（写给下一个人）

1. **route 拦截会让请求 `ERR_ABORTED`**：用 Playwright 的 `page.route` 延迟 `GET`/`POST` 来观察 loading 态时，延迟到位后 `continue()` 偶发让请求以 `ERR_ABORTED` 结束（本文写作时 GET、POST 各遇到一次）。**这是拦截器的副作用，不是应用缺陷** —— 同一请求在无拦截时稳定 200。若用它来判定「请求失败」，会得出反向结论。
2. **合成点击 ≠ 用户点击**：`element.click()` 对**已关闭（隐藏）弹窗**里的按钮照样生效，而真人点不到。本文写作时用「两次 `click()` 间隔 100ms」测重复提交，一度读出 2 条 `POST` —— 复查发现第一次提交成功后弹窗已关闭，第二次点击落在隐藏按钮上，属**不可达路径**。改用真实 `dblclick` 在**可见按钮**上重测，读数是 **1 条 `POST`**（spec §6 分组 B #9）。判定「能不能重复提交」这类问题，必须以可达路径为准。

---

## 6. 广播项（合并前发群里）

| # | 内容 | 收件人 |
|---|------|--------|
| 1 | **`types/persona.ts` 是 `GET /personas` 响应类型的唯一一份**。chat-view 分支若自带 `MockPersona`，字段需与本文件对齐，不要两份并行 | 成员 2 |
| 2 | **Mock 出口的分工**：`api/mock/index.ts` 由先合并的那支负责扩展；本支不做 mock（理由见 §3.1）。谁先合并说一声，另一支收尾时补出口 | 成员 2 |
| 3 | `GET /personas` 返回的 `list` 在无人设时是 `[]` 而不是 `null`，且**没有错误码**——空是合法状态，前端做空态引导，不要用错误态渲染 | 全员（成员 3 = 后端 owner，已确认） |

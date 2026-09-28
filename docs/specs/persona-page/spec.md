# spec · 人设管理页面（persona-page）· 前端

> 功能名：persona-page ｜ 分支：`feature/frontend-persona-page`
> 负责人：成员 3（人设模块 owner，[MASTER §1](../../dev/MASTER.md)「人设管理页」一行）
> 状态：✅ 实现完成、验收 A/B 两组读数已回填，**待人工审查 + 开 PR**（C 组未开始）｜ 创建：2026-09-28 ｜ 最后更新：2026-09-28（v2）
> 关联契约：[API_CONTRACT.md §4](../../API_CONTRACT.md)（`GET/POST /personas`、`PUT/DELETE /personas/:id`）
> 上游文档：[persona-crud/spec.md](../persona-crud/spec.md)（接口层，**已合并**）
> 通用约定一律给链接，不重复描述：[AGENTS §4.9](../../../AGENTS.md)（前端接口调用约定）、[§5.1](../../../AGENTS.md)（注释纪律）、[§5.2](../../../AGENTS.md)（文档引用优先）

---

## 1. 背景与目标

`persona-crud` 的 4 个端点已经落地并挂进 `router.go:49` 的 `protected` 组，但**前端 `/personas` 这条路由至今指向 `router/index.ts` 里的内联占位组件**，点开只有「待实现」三个字。也就是说：后端已能建人设，界面上却没有地方能建。

**目标（一句话）**：登录后打开 `/personas`，能**新建 / 编辑 / 删除**人设，三条链路都在真实后端上跑通；页面同时承担「对话列表」职责（契约 §4：无会话实体，列表即对话列表）。

**为什么它在关键路径上**：人设是全部子数据（消息 / 记忆 / 画像 / 主动消息 / 日程）的挂载点，而演示主链路「登录 → **创建人设** → 流式对话」中间那一环正是本页——没有它，聊天页无从选择对话对象。

---

## 2. 范围

### 2.1 做什么（In Scope）

| 文件 | 类型 | 职责 |
|------|------|------|
| `frontend/src/types/persona.ts` | 新增 | `Persona`、`PersonaPayload` 类型；契约 §4 字段逐字对齐 |
| `frontend/src/api/persona.ts` | 新增 | 4 个端点的封装；**全应用访问 `/personas` 的唯一入口** |
| `frontend/src/views/persona/PersonaView.vue` | 新增 | 页面本体：卡片列表 + loading/empty/error 三态 + 删除二次确认 + 分页 |
| `frontend/src/views/persona/PersonaFormDialog.vue` | 新增 | 新建 / 编辑共用的表单弹窗（三个必填字段 + 校验 + 提交） |
| `frontend/src/router/index.ts` | 修改 | **语义上只换 `component`**：`/personas` 由 `Placeholder` 换成 `() => import('@/views/persona/PersonaView.vue')`；`path` / `name` / `meta` 不动。实测 `git diff --numstat` = **`6 insertions(+), 1 deletion(-)`** —— 多出的 5 行是把这一条 route 从单行拆成多行：改成动态 import 后单行长度 **129 字符**，与该文件已有的多行条目（`/login`、`/register`）体例不一致，故拆行，**无第二条 route 被动过**（判据见 §6 A2 #4） |
| `docs/specs/persona-page/{spec,plan}.md` | 新增 | 本文与实施计划 |
| `.learn/persona-page.md` | 新增 | 学习笔记（`.learn/` 在 `.gitignore:75`，**不进仓库**） |

合计 **4 个源文件新增 + 1 个源文件改 1 处（6+/1-）** + 2 篇文档 + 1 篇学习笔记。

**两处明确写出来的取舍**（reviewer 可以据此砍掉，砍掉不影响其余部分）：

1. **卡片上的「去对话」链接**：`RouterLink → { name: 'Chat', params: { personaId } }`。契约把本列表定义为对话列表，从此处进对话是最短路径；`/chat/:personaId` 目前仍是占位页（属 chat-view 分支），点进去显示「待实现」。
2. **卡片上的 `familiarity` / `lastMessageAt` / `createdAt` 只读展示**：三者都是契约 §4 暴露的读字段，管理页不展示就只能靠猜。**注意 `familiarity` 是人格状态、不是情绪**（[AGENTS §4.4](../../../AGENTS.md) 禁的是情绪标签，两者无关）。

### 2.2 不做什么（明确排除，防止范围蔓延）

| 不做的事 | 原因 |
|---|---|
| **Mock 层**（`api/mock/persona.ts` + `VITE_USE_MOCK` 出口） | ① 后端 4 个端点**已可用**，本页能真联调，mock 没有存在理由；② 未合并的 `feature/frontend-chat-view` 分支已建 `src/api/mock/index.ts` 作为开关出口，本支再改同一文件 = **两人同时改一个文件**（AGENTS §4.8）；③ 会制造第二份真相（同 `frontend-auth-pages/plan.md` §3.6 的结论）。**见 plan §3.1，并列为广播项** |
| 聊天页 `/chat/:personaId` | 属 `feature/frontend-chat-view` |
| `MainLayout.vue` 的侧栏 | 已含「人设」项且指向 `/personas`（`MainLayout.vue:11`），无需改 |
| `request.ts` / `types/api.ts` / `stores/auth.ts` / `router/guards.ts` | 均已合并，本支只**消费** |
| 人设数量上限 / 人设名去重 | 后端明确不做（`persona-crud/spec.md` §2.2：`name` 无 UNIQUE，**重名合法**）。前端不得自作主张加校验 |
| `state` 的解析与展示 | 契约 §4：`state` 前端**不解析、不回传修改** |
| 情绪标签 / 情绪趋势 | [AGENTS §4.4](../../../AGENTS.md) |
| 主动消息设置入口 | `GET/PUT /proactive/settings` 是后续模块，尚未实现 |
| 头像 / 文件上传 / 富文本 | 契约 §4 只有三个文本字段 |
| 视觉打磨、窄屏适配、主题 | Week 4 |

---

## 3. 依赖与交接

### 3.1 我依赖谁（全部已就绪，本支无阻塞）

| 依赖 | 状态 |
|------|------|
| 4 个 persona 端点（`persona_handler.go:26` 注册，挂在 `protected` 组） | ✅ 已合并，**2026-09-28 本机实测通过**（读数见 §5） |
| `request.ts` 的解包 / 自动附 token / `4012` 刷新重放 / `4010` 清登录态 | ✅ 已合并（PR #24） |
| 路由 `/personas` 与守卫（`requiresAuth` 缺省为需登录） | ✅ 已合并（PR #31） |
| Element Plus 全局注册（`main.ts` 全量引入） | ✅ `el-*` 组件模板内直接可用 |

> ⚠️ **未装 `@element-plus/icons-vue`**（不在 `package.json` 的 dependencies 里）：模板里**不得**使用 `<el-icon><Plus /></el-icon>` 这类写法，会直接编译失败。按钮一律用文字。

### 3.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| `types/persona.ts` 的 `Persona` / `PersonaPayload` | **聊天页（chat-view 分支）** | `GET /personas` 的对话列表侧栏；**若该支自带一份 persona 类型（其 mock 里已有 `MockPersona`），需按本文件对齐字段，不要两份** |
| `api/persona.ts` 的 4 个函数 | 后续任意页面 | 人设是消息 / 记忆 / 画像的外键载体，任何需要「选一个人设」的页面都从这里取列表 |
| 三态 + 二次确认的写法 | 画像页 / 朋友圈页 / 日程页 | 同构的「列表 + 增删改」页面照抄，避免四人四种写法 |

---

## 4. 硬性约束（违反即不通过）

| # | 约束 | 来源 |
|---|---|---|
| 1 | 路由**只换 `component`**：`path: 'personas'`、`name: 'Persona'`、`meta.title: '人设管理'` 三者逐字不动 | 任务书要求 1 |
| 2 | 页面组件放 `views/persona/`，**不新建平行目录**（不出现 `views/personas/`、`views/user-persona/`） | 任务书要求 2 |
| 3 | 组件**不出现 `axios`**、也**不直接 import `@/api/request`**：一切请求经 `api/persona.ts` | 任务书要求 3 / [AGENTS §3](../../../AGENTS.md)「组件不直接写 axios」 |
| 4 | **三个必填字段**：`name` / `personalityDesc` / `speakingStyle`；字段名 camelCase 逐字对齐契约 §4 | 任务书要求 5 / [AGENTS §5](../../../AGENTS.md) |
| 5 | 列表**卡片式**展示；删除**必须二次确认**；编辑**用表单**（不是原地编辑行内文本） | 任务书要求 6 |
| 6 | **三态齐全**：loading / empty / error，且三者互斥 | 任务书要求 6 / [AGENTS §4.9](../../../AGENTS.md) |
| 7 | **空态不是错误态**：`GET /personas` 业务错误码集合为空，`list: []` 是 `200`（§5 实测） | 契约 §4 / persona-crud spec §4.2 |
| 8 | 删除确认文案**必须写明级联后果**（消息与记忆一并删除、不可恢复） | 契约 §4 末尾要求 |
| 9 | **不发送** `state` / `familiarity` / `userId`；`PUT` 只发三个字段（整体替换，不是部分更新） | 契约 §4 / persona-crud spec §4.6 |
| 10 | 无 `any`：`catch` 到的是 `unknown`，必须 `instanceof ApiError` 收窄后才能读 `message` | [AGENTS §5](../../../AGENTS.md) |
| 11 | 错误文案**优先用后端 `message`**（一 code 一 msg）；仅网络不通时用兜底文案 | [AGENTS §4.9](../../../AGENTS.md) |
| 12 | 不引入新依赖 | 任务书 |
| 13 | 注释只写必要三类；**学习内容进 `.learn/`，不进 `.vue`/`.ts`** | 任务书要求 7 / [AGENTS §5.1](../../../AGENTS.md) |

---

## 5. 本支依赖的契约事实（2026-09-28 实测，非推演）

> **口径**：本机 `go run ./cmd/server`（`:8080`）+ 原生 PostgreSQL 16（`:5432`）。探针脚本临时注册用户 `probe0928`，逐个端点真发请求；**跑完已把该用户级联删除**（`persona=1 → 0`、`proactive_setting=1 → 0`）。
> **为什么写这一节**：本节是验收表的判据来源。凡「我以为后端会怎样」的说法，一律不写进验收（`persona-crud/spec.md` §8.5 的立规）。

| # | 输入 | 实测读数 | 对前端的含义 |
|---|---|---|---|
| 1 | `GET /personas` 不带 Token | HTTP **401**、`code: 4010` | `request.ts` 已有清登录态 + 跳登录页分支，页面**不需要**为它写额外逻辑 |
| 2 | `GET /personas`（该用户无人设） | `{list: [], total: 0, page: 1, pageSize: 20}` | **空 = 空**：不是错误、不是 404。空态判据只能用 `list.length === 0` |
| 3 | `GET /personas?page=0&pageSize=1000` | `page: 1, pageSize: 100` | 分页参数**只收敛不报错**：前端传超界值不会收到 `4001`，也不必自己夹 |
| 4 | `POST /personas`（三个字段） | HTTP **200**（不是 201）；data 为完整 Persona：`state: {"familiarity": 0}`、`familiarity: 0`、`lastMessageAt: null`、`createdAt` 带微秒（`2026-09-28T13:53:16.172057+08:00`） | 新建后**不需要**回读；时间字符串含 6 位小数，格式化必须走 `new Date()` 而不是字符串切片 |
| 5 | `PUT` 的 body 里**多传** `state: {"hacked": true}` 与 `familiarity: 999` | 响应里 `state` 仍是 `{"familiarity": 0}`、`familiarity` 仍是 `0` | 契约「静默忽略」落实了：前端**不传**这两项，且**不需要**「先读后写保留 state」 |
| 6 | `PUT /personas/999999`（不存在）/ `PUT /personas/abc`（非数字） | 两者都是 `4043` | 前端只需处理**一个**码；非数字 id 也不会收到 `4001` |
| 7 | `POST` 只传 `{name}` | `4001` | 前端校验只是提前拦截；漏网的由后端兜底 |
| 8 | `DELETE /personas/:id` | HTTP 200、`data: null`；**再删一次 → `4043`** | 删除**不是幂等成功**：重复提交（或另一标签页已删）会看到「人设不存在」→ 见 plan §3.7 的处理 |
| 9 | 两条 `lastMessageAt` 均为 `null` 的人设，按列表返回 | 返回 `[id=3, id=2]`（**新在前**），即 `ORDER BY last_message_at DESC NULLS LAST, id DESC` 生效 | 排序权威在后端；**新建的人设在「有聊天记录的人设」之后**（NULLS LAST），所以建完不一定落在第一屏——见 plan §3.4 |

---

## 6. 验收标准

> **规则**（沿用 `persona-crud/spec.md` §8）：禁止类与必须类**成对**写；**每条检查都要在真实产物上跑过并写下读数**，没有读数的检查项不写进表。
> **读数口径**：2026-09-28，本机 `go run ./cmd/server`（`:8080`）+ 原生 PostgreSQL 16 + `vite dev`（`:5173`），浏览器 Chromium（Playwright 驱动）。B 组的探针账号 `probeload` 及其 3 条人设**跑完已级联删除**，库已回到验证前的 3 个账号（详见 §6 末尾「残留」）。

### 分组 A · 本机可验证（不经浏览器）

**A1 · 静态**（工作目录 `frontend/`）

| 检查 | 期望 | 实测 |
|---|---|---|
| `npm run typecheck`（`vue-tsc --noEmit`） | 零错误 | **exit 0、无输出** ✅ |
| `npm run build`（`vue-tsc --noEmit && vite build`） | 通过 | **exit 0、`✓ built in 6.57s`** ✅；产物里 `PersonaView-D1LT7SQq.js` 6.78 kB + `PersonaView-NyBp4DSz.css` 1.14 kB 单独成 chunk，佐证路由的懒加载生效。**遗留**：`index-*.js` 1,111.02 kB 触发 `>500 kB` 警告 —— 全量引入 Element Plus 导致，**本支未新增依赖，非本支引入** |

**A2 · 代码级锚定检查**（锚定到 `import` / `from`，**不用裸词**——裸词会命中注释，`persona-crud/spec.md` §8.5 已栽过 6 次）

| # | 检查 | 期望 | 实测 |
|---|---|---|---|
| 1 | `grep -cE "^import .*from 'axios'" frontend/src/views/persona/*.vue` | **0**（禁止类） | **PersonaView:0 / PersonaFormDialog:0** ✅ |
| 2 | `grep -cE "^import .*from '@/api/request'" frontend/src/views/persona/*.vue` | **0**（禁止类；组件不许碰 request） | **PersonaView:0 / PersonaFormDialog:0** ✅ |
| 3 | `grep -c "from '@/api/persona'" frontend/src/views/persona/PersonaView.vue` | **1**（必须类，与 #1/#2 配对） | **1** ✅（`PersonaFormDialog.vue` 同为 **1**，同一条规则同样成立） |
| 4 | `grep -cE "component: Placeholder" frontend/src/router/index.ts` | **5**（实现前是 6：本支只换掉 personas 那一处） | **5** ✅（`git show HEAD:frontend/src/router/index.ts` 计数为 6） |
| 5 | `grep -c "views/persona/PersonaView.vue" frontend/src/router/index.ts` | **1** | **1** ✅ |
| 6 | `ls frontend/src/views/` | 只有 `auth` 与 `persona`（无平行目录） | **`auth`、`persona`** ✅ |
| 7 | `grep -c "state" frontend/src/views/persona/*.vue` | **0**（禁止类：页面不读不写 state） | **PersonaView:0 / PersonaFormDialog:0** ✅ |
| 8 | `grep -cE "(familiarity\|state):" frontend/src/api/persona.ts` | **0**（禁止类：请求体里不出现非三字段） | **0** ✅ |
| 9 | `grep -c "ElMessageBox.confirm" frontend/src/views/persona/PersonaView.vue` | **1**（必须类：二次确认只有一处） | **1** ✅ |

> **#8 的锚定不是形式主义，本支就踩到了**：同文件里裸词 `grep -c "state"` 读数是 **1**，命中的是 `api/persona.ts:25` 的注释行（`编辑人设：三个字段整体替换，\`state\` / \`familiarity\` 不进请求体`）——与代码行为无关，却是典型的「检查失真」。带 `:` 的锚定式读数 0 才说明得了问题。
> **#7 读数为 0 的前提是**不把 `state` 写进注释——注释里出现 `state` 会让它变成 1（内容同上）。本支两处 view 注释都没提它，读数为 0。

### 分组 B · 真实后端联调（浏览器逐条，后端 + PG 已就绪）

| # | 检查 | 实测读数 |
|---|---|---|
| 1 | 未登录访问 `/personas` → 被守卫拦到 `/login?redirect=%2Fpersonas`（既有行为，本支不得回归） | 未登录访问 `/personas` → 落地 URL `/login?redirect=%2Fpersonas` ✅ |
| 2 | 登录后进入 `/personas`：该账号无人设时显示**空态**（不是错误态），且空态里有创建入口 | 新账号 `probeload` 首访：`.el-empty__description` = `还没有人设`、按钮 = `创建第一个 AI 伴侣`；`.page-error` / `.el-skeleton` 均不存在 ✅ |
| 3 | 新建：填三个字段 → 卡片出现在列表；**网络面板里请求体只有三个字段**（无 `state` / `familiarity`） | `POST /api/v1/personas` 200、卡片出现、提示 `人设已创建` ✅。请求体逐字节实测（浏览器网络面板）：`{"name":"请求体复核","personalityDesc":"只为一个读数","speakingStyle":"简短"}` —— 只有三个字段，**没有 `state` / `familiarity` / `userId`** ✅ |
| 4 | 表单校验：三个字段任一为空 → 提交被拦、**不发请求** | 三框全空点保存：`.el-form-item__error` 三条 = `请输入人设名` / `请输入性格描述` / `请输入说话风格`，按钮**无 `is-loading`**（说明在 `submitting=true` 之前就返回了）；网络列表里 **0 条 `POST`** ✅ |
| 5 | 表单校验：只输入空格的名字 → 同样被拦（前端 trim；详见 plan §3.9） | 隔离用例：`name` 填纯空格 `"   "`、另两框填合法值 → 唯一错误 `请输入人设名`、弹窗不关、**0 条 `POST`** ✅（对照：非隔离用例下三框俱空时第三条错误也在，属预期） |
| 6 | 编辑：表单**预填当前值**（不是空白）；只改 `name` 保存 → 卡片更新，且 `familiarity` / `lastMessageAt` **没变**（证明没碰 `state`） | 点编辑：三框分别为 `小暖` / `温柔、耐心，喜欢倾听` / `语气轻柔，偶尔用颜文字`（非空）；改名为 `小暖改` 保存后卡片标题变 `小暖改`，而卡片上 `亲密度 0`、`最近对话 还没聊过`、`创建于 2026-09-28 14:13` **三项逐字未变**；`PUT /personas/4` 请求体同样只有三字段 ✅ |
| 7 | 删除：点删除先弹二次确认；**点取消不发请求**、卡片仍在；确认后卡片消失 | 点删除 → 弹窗标题 `确认删除这个人设吗？`、正文含人设名；点 `取消` 后网络列表里 **0 条 DELETE**、卡片仍在、确认框已消失；再点删除并确认 → `DELETE /personas/5` 200，该卡片消失、另一张仍在 ✅ |
| 8 | 删除确认文案里**含级联说明**（消息与记忆一并删除、不可恢复） | 正文实测：`删除「小暖改」后，它的全部对话与记忆都会一并删除，且无法恢复。` ✅ |
| 9 | loading 态可见（首屏骨架 / 按钮 loading），且提交中**不能重复提交** | **首屏**：用 Playwright route 给 `GET /personas` 加 1500ms 延迟，加载期间 `.el-skeleton` 存在（4 个骨架块、`is-animated`），且卡片 / 空态 / 错误态**三者都不存在** ✅。**提交中**：给 `POST` 加 900ms 延迟，点保存后 +300ms 采样 → 按钮文案 `保存中…`、`disabled: true`、`class 含 is-loading`；此时再点一次，`POST` 记录**仍为 1 条** ✅。**真实双击**：在可见按钮上 `dblclick`（Playwright 原生双击），`POST` 记录 **1 条**、只生成 1 张卡片 ✅ —— 双击的第二下被 `is-loading` 的 disabled 态吞掉 |
| 10 | error 态：停掉后端再刷新 → 显示错误态 + 重试按钮；页面不崩、console **无未捕获异常** | `taskkill` 掉 `server.exe`（PID 36088）后刷新：`.page-error` 显示 `人设列表加载失败，请稍后重试` + `重试` 按钮；卡片 / 空态 / 骨架均不在；console 唯一一条错误是 `net::ERR_CONNECTION_REFUSED`（预期内的网络失败），**无未捕获 Promise、无 Vue 警告**；重启后端后点 `重试` → 20 张卡片 + 分页器恢复 ✅ |
| 11 | `ElMessageBox` 点取消后 console **无 unhandled rejection** | 点 `取消` 前后 console 条目数不变（1 条，且是 `favicon.ico` 404，与本支无关）✅ |
| 12 | 列表超过一页时出现分页控件；**删掉某页最后一项后不会停在空页**（plan §3.5） | 21 条人设时：`total=21`、第 1 页 20 张卡、分页器出现且共 2 页；点第 2 页 → 1 张卡（`.el-pager .is-active` = 2）；删掉这 1 张 → 自动回到第 1 页并显示 20 张卡，分页器因 `total=20` 隐藏，**没有出现「空页 + 还有数据」的自相矛盾画面** ✅ |
| 13 | 整个流程结束后 `git status` 里 `frontend/src/types/persona.ts` 等 5 个源文件**未提交**（任务书：留工作区待人工审查） | `git status --porcelain -uall` 逐文件为：` M frontend/src/router/index.ts`、`?? frontend/src/api/persona.ts`、`?? frontend/src/types/persona.ts`、`?? frontend/src/views/persona/PersonaView.vue`、`?? frontend/src/views/persona/PersonaFormDialog.vue`、`?? docs/specs/persona-page/{spec,plan}.md` —— **7 个文件、全部未提交**，HEAD 仍是 `b05a311` ✅ |

**B 组补充读数（不在原表内，实现时顺带测到，一并留档）**

| 场景 | 读数 |
|---|---|
| 中文往返 | 界面创建中文人设后，卡片标题的码点实测为 `U+4E2D U+6587 U+540D U+6821 U+9A8C`（中文名校验）、描述为 `U+6E29 U+67D4`（温柔）；**无 U+FFFD** → 写入 / 读取 / 渲染三段都不掉编码 |
| 表单「复制而非引用」 | 弹窗里把名字改成 `小暖改`（列表对象值仍是 `小暖`）后立即采样：卡片 `小暖`、弹窗 `小暖改` → 两者不是同一个对象，**编辑途中列表不会被就地改动** |
| 二次打开弹窗 | 编辑过 `小暖改` 后再点「新建人设」：标题 `新建人设`、三框字数计数全为 `0 / N`（**不是残留的 小暖改**）→ `watch(modelValue)` 的逐字段复制在新建方向同样生效 |
| 新建 / 编辑后不本地插排 | `POST` 后紧跟一条 `GET`（`POST → GET`）、`PUT` 后同样（`PUT → GET`）：保存后一律重拉，与 plan §3.4「排序权在后端」一致 |
| 保存失败分支（网络类） | 用 route 把 `POST` 延迟到 1500ms（触发了 route 自身的中断，请求 `ERR_ABORTED`）：弹窗**不关闭**、内嵌 `el-alert` 显示 `保存失败，请稍后重试`（网络类失败走兜底文案）、按钮回到 `保存` 且可再点、无 toast、**未创建任何人设** ✅ |
| 保存失败分支（业务类） | 用已被删除的探针账号再发一次 `POST`（后端外键拒绝）：HTTP **500**、`code` 为数据库错误码，弹窗内嵌 `el-alert` 显示的是**后端原文 `数据库操作失败`**、不是兜底文案 → 约束 #11「错误文案优先用后端 `message`」在业务错误上成立 ✅（该请求**未在库里留下任何行**，见下方「残留」） |
| **不可达路径的反例**（方法论，务必读完） | 用两次 `element.click()`（间隔 100ms）测重复提交，一度读出 **2 条 `POST` / 2 张卡片**。复查：第一次提交成功后弹窗**已关闭**（`取消` 按钮 Playwright 报 `element is not visible`），第二次点击落在**隐藏弹窗**的按钮上 —— 真人点不到这个按钮，属**不可达路径**，**不是缺陷**。改用真实 `dblclick` 在可见按钮上重测，读数为 1 条 `POST`（见 #9）。**教训**：合成点击不受可见性约束，用它下结论前必须先确认该路径用户可达 |

> **口径备注**：B#9 的 loading 读数是**用 route 延迟请求**观测到的，不是「后端真的慢」；B#10 用 `taskkill` 真停后端，是真实断网。两处的差异已如实分开写。
> 另：route 延迟到位后 `continue()` 会偶发让请求以 `ERR_ABORTED` 结束（GET、POST 各遇到一次），**这是 Playwright 拦截的副作用、不是应用缺陷**——不加拦截时同一请求均 200（见 B#12 的 21 条数据全绿）。

### 分组 C · 流程（阻塞合并）

> **状态：未开始**。任务书要求「写完留工作区不提交，先人工审查」，故本节全部未执行；下面的框在人工审查通过、提交并开 PR 后才可能打勾。

- [ ] PR 已开、至少 1 人 Approve；commit 符合 `<type>(<scope>): <subject>`（scope `persona`）
- [ ] 广播项已发：① mock 层与 chat-view 分支的分歧（plan §3.1）② `types/persona.ts` 是 `GET /personas` 类型的**唯一一份**（§3.2）

### 残留（人工审查前请知悉）

- **库**：验证用的 3 个探针账号（`probe0928` / `personaprobe` / `probeload`）及其人设、`proactive_settings` **已全部级联删除**，库回到验证前的 3 个账号（`test1` / `test2` / `JMX2`），仅 `test1` 名下 1 条人设（**非本支产生**）。
- **进程**：`:8080` 后端与 `:5173` vite dev 仍在跑（本次验证为停后端测 error 态重启过一次）；不需要时请自行停掉。
- **`.playwright-mcp/`**：浏览器工具会在仓库根写入快照 / console 日志（**未被 `.gitignore` 覆盖**，会出现在 `git status` 里）。本次产生的已删除，工作区现在只剩上表那 7 个文件；下次谁再用浏览器工具验证，记得收尾删掉它（或考虑加进 `.gitignore`——**本支没改 `.gitignore`**，因为那会超出本支范围）。
- **`frontend/dist/`**：A1 的 `npm run build` 产物，已被 `.gitignore:11` 覆盖，不进入审查范围。

---

## 7. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|------|------|------|------|
| 2026-09-28 | v1 | 创建。范围 = 4 个源文件新增 + 路由 1 行；§5 写入 9 条本机实测契约读数作为验收判据；§6 A 组 9 条锚定检查（待回填读数） | 人设管理页开工前的合同与验收基线 |
| 2026-09-28 | v2 | 实现完成，§6 两组读数**全部回填**（A1 2 条、A2 9 条、B 13 条 + 补充 7 条）；新增「残留」小节；§2.1 路由改动量由「1 行」改为实测 `6+/1-` 并说明拆行原因；A2 #8 补上**本支真实的裸词失真实例**；C 组标注未开始 | 任务书要求「写完留工作区不提交，先人工审查」——审查需要可核对的读数，而不是「应该没问题」 |

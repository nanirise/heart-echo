# spec · 主动消息（Proactive Message）

> 本文件是**合同**：定义本功能做什么、做到什么算完成。合并后即冻结，任何字段或行为变更必须升版本并在群里广播。
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：DDL、契约字段、错误码表、通用约定一律**不复制**，只给链接；
> 本文件只写**本功能独有**的内容——独有陷阱、独有决策、独有验收项。

| 项 | 值 |
|----|-----|
| 功能名 | `proactive-message` |
| 版本 | v1.5 |
| 分支 | `feature/backend-proactive-trigger`（已切，本分支 3 个 commit，未推） |
| 状态 | 🟡 联调中（settings 两端 + 前端已交付；触发链路 TriggerNow / `POST /proactive/trigger` 已落盘，待装配合并） |
| **本功能范围** | 触发判定 `TriggerNow` + 定时扫描 `proactive_job.go` + `GET/PUT /proactive/settings` + `POST /proactive/trigger` + 前端设置页与侧栏红点 |
| 负责人 | 成员 3 |
| 关联任务书 | [成员 3 任务书 §4](../../dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md)（Week 3 主动消息任务表）、§8（自检） |
| 关联契约 | [API_CONTRACT §9](../../API_CONTRACT.md)（`Settings` + 3 个端点）、[§2](../../API_CONTRACT.md)（错误码总表） |
| 关联设计 | [TECH_DESIGN §5.4](../../TECH_DESIGN.md)（主动消息实现）、[§4.6](../../TECH_DESIGN.md)（环境变量）、[MASTER §4.2](../../dev/MASTER.md)（端点归属）、[§4.5](../../dev/MASTER.md)（路由注册） |
| 关联红线 | [AGENTS §4.3](../../../AGENTS.md)（数据边界 = 协作 §10.4 红线 4）、[§4.6](../../../AGENTS.md)（三 Agent）、[§4.8](../../../AGENTS.md)（定时任务与路由）、[§7](../../../AGENTS.md)（红线 5） |
| 关联 spec | [proactive-setting](../proactive-setting/spec.md)（settings 仓储**已交付**；§2 是本功能的端点预写规格）、[chat-message](../chat-message/spec.md)（[nudge] 写入路径归属，§7.3）、[schedule](../schedule/spec.md)（P1 复用同款注入） |

---

## 1. 目标与边界

**一句话**：主动消息**只负责制造一次对话机会**——向该人设的对话注入一条 `role='user'`、`is_nudge=true` 的 `[nudge]` 消息，随后**走原本的聊天链路**生成回复（[TECH_DESIGN §5.4](../../TECH_DESIGN.md)：主动触发与正常回复共用一条链路）。它**不负责生成**：不为主动消息单独写生成逻辑、不做模板消息——回复该长什么样由聊天链路决定。

**已经交付的（本功能的前置，勿重复实现）**：

- `proactive_settings` 的表结构、播种（创建人设时同事务写默认行）、仓储 `Get` / `UpdateOwned`：见 [proactive-setting §3](../proactive-setting/spec.md)。`Get` 一条查询同时完成「取配置」与「验归属」（配置行自带 `user_id` + `persona_id` 两个键）。
- settings 两个端点的绑定规则、校验分界、错误码口径：已由 [proactive-setting §2](../proactive-setting/spec.md) **预写**——照写，不要临场发挥。

**本功能要交付的**：触发判定（§3）、两个入口——定时扫描与手动触发（§4）、settings 两端的 HTTP 层、前端设置页与侧栏红点（见 [plan](plan.md) 步 3–6）。

**边界上最容易含混的一点**：`[nudge]` 消息要**写进 `chat_messages`** 并通过聊天链路触发生成，而 `chat_messages` 的写入路径（`message_repo.go` / `chat_service.go`）按任务书归成员 1，且 PROGRESS.md 显示尚无一行。**注入由谁执行、经由什么入口，是 §7 第 1 条（已选 A：调成员 1 的 `ChatService`，签名待定）**——本 spec 只定「注入的内容形态」，不定「`ChatService` 的方法签名」。

---

## 2. 本功能独有约束

1. **查询双条件**：对 `proactive_settings` 的任何查询必须**同时**带 `user_id` 与 `persona_id`，缺一即越权——漏 `persona_id` 会一次读到该用户**所有人设**的配置。`user_id` 只有两个合法来源：手动端点从 Token 取、定时任务从扫描行取；**永不从请求体 / query 读**。（[AGENTS §4.3](../../../AGENTS.md)）
2. **`personaId` 必传**：`GET /proactive/settings`、`POST /proactive/trigger` 缺 `personaId` 一律 `4001`，**在参数层拦下**——缺参时还没有任何归属可供校验，不能当 `personaId = 0` 查库（会命中空集，把调用方的 bug 伪装成正常态）。口径来自契约 §9 的 2026-09-20 变更记录。
3. **资源越权 → `4043`**：人设不是自己的 / 不存在 / 播种缺失，三者一律 `4043`「人设不存在」，**同码同文案**；不返回 403（会暴露该 `persona_id` 存在，而它是全局自增的、可被顺序试号探测），也不返回 `4030`（那是功能越权，本模块无此场景）。（[AGENTS §4.3](../../../AGENTS.md)）
4. **`lastNudgeAt` 只读**：`PUT` 忽略、前端不写；**唯一的写入方是本功能的触发链路**（注入成功后更新 `last_nudge_at`）。它是记录列，不参与 §3 的判定（② 看的是 `personas.last_message_at`）。（契约 §9；[proactive-setting §1.5](../proactive-setting/spec.md)）
5. **定时任务跑在 Go 侧**：`proactive_job.go` 在 backend 内按 `PROACTIVE_JOB_INTERVAL` 周期扫描（默认 5m，[TECH_DESIGN §4.6](../../TECH_DESIGN.md)），**不放 ai-service**（Python APScheduler 一律不用）——放错服务会静默失效（[AGENTS §4.8](../../../AGENTS.md)）。该变量**已落地**，见 §7 第 2 条。

> 以上之外的内容（DTO 指针、`intervalMin < intervalMax` 等跨字段校验、`RowsAffected == 0 → 4043`）已在 [proactive-setting §1–§2](../proactive-setting/spec.md) 定过，本文件不重复。

---

## 3. 触发判定顺序（`TriggerNow` 核心）

`TriggerNow` 是触发链路的**唯一入口**——手动端点与定时任务都调它（§4）。按序判定，**任一不通过即返回「未触发」，不报错**：

| # | 判定 | 不通过 |
|---|------|--------|
| ① | `enabled == true` | 跳过 |
| ② | 空闲已达阈值：`now - personas.last_message_at` **达到/超过** [intervalMin, intervalMax] 区间内的随机阈值（[TECH_DESIGN §5.4](../../TECH_DESIGN.md)） | 跳过 |
| ③ | 当日已发条数 < `dailyLimit` | 跳过 |
| ④ | 该人设最近 1 小时内**没有**用户消息 | 跳过 |
| ⑤ | 全通过 → 注入 `[nudge]`（写入路径归属见 §7 第 1 条）→ 走聊天链路生成 → 更新 `last_nudge_at` | —— |

**五条落地口径（都会踩，逐条写清）**：

a. **「不报错」的边界**：判定不通过是**正常态**（每 5 分钟扫一次，绝大多数人设都不触发），不产生错误码、不发 ERROR 日志。但**基础设施错误不算「不通过」**——DB 查询失败、生成调用失败照常上抛（手动端点按错误码口径出口，生成失败为 `5001`）。把 DB 故障吞成「未触发」，会让演示时「点了没反应」变成常态。
b. **② 的语义是「达到/超过」不是「落在区间内」**：静默更久的人设同样触发，频率由 ③ 的日上限兜底；阈值的具体随机口径（每个空闲段取一次 / 每次扫描取）归步 2 实现，本 spec 只钉判定式。`last_message_at IS NULL`（从未聊过）→ **不触发**（本 spec 定：没有空闲起点；SQL 里 `NULL > 阈值` 天然为假，实现与语义一致）。
c. **④ 必须排除 `is_nudge = true` 的行**：注入的 `[nudge]` 自身就是 `role='user'`——不排除的话，上一次主动消息会把自己的注入当成「用户回复了」，误伤下一次判定。
d. **③ 的计数口径**：`chat_messages` 中 `persona_id` 匹配、`is_nudge = true`、`created_at` 落在**服务器本地当日**（零点起）的条数，与 `daily_limit` 比较。**数的是注入的 nudge 条数，不只本模块**：P1 日程提醒复用同一条注入路径，同样计入——两套链路共用一个日上限，否则「防骚扰」会出现两个口径。（`chat_messages` 没有来源列，靠 `is_nudge` 一列可数。）（**已确认**：日上限共用，成员 1 / 队长 2026-10-02 定）
e. **⑤ 之后空闲计时会自然重置**：注入的 `[nudge]` 与生成的回复都是该对话的新消息（聊天链路的既有行为），`personas.last_message_at` 随之推进，② 不会连着触发。

**每一层不通过都留一条 debug 级日志**（带 `persona_id` 与判定名）——演示前排障全靠它，否则只能看到「没触发」三个字。

---

## 4. 手动触发与定时任务同源

- **两个入口、同一段逻辑**：`POST /proactive/trigger`（手动）与 `service/proactive_job.go`（按 `PROACTIVE_JOB_INTERVAL` 扫描）**都调同一个 `TriggerNow`**——不复制、不变体。这是 [MASTER §4.2](../../dev/MASTER.md) 的 🚨 硬性要求：定时任务最短 30 分钟起步，答辩现场等不起；「同一个」的含义是**演示通过 = 线上逻辑通过**（[MEMBER_3 §4](../../dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md)：不是作弊，是标准可测试性设计）。
- **差异只在「谁来点名」**：定时任务对全部 `enabled = true` 的人设逐个判；手动对**一个** `personaId` 判。判定与注入不分叉。
- **手动端点的出入口**（契约 §9）：请求 `{personaId}`（必传 → `4001`；归属 → `4043`；生成失败 → `5001`）；响应 `{messageId, content, createdAt}` 对应**生成的回复**（不是 `[nudge]` 注入行——与日程触发的返回同构，见 [schedule §4.1](../schedule/spec.md)）。
- ✅ **已定（2026-10-05 群内确认）**：判定未通过时手动端点返回 **200 + 空结果**，即 `{messageId: 0, content: "", createdAt: null}`。实现口径：`TriggerNow` 返回 `(*NudgeResult, error)`，**`nil + nil` 即判定未通过**（不是错误），handler 据此返回空结果，前端按 `messageId === 0` 区分。不新增表达的原因：契约 §9 只给了成功形态与 `4001 / 4043 / 5001`，复用 `4043` 会让「人设不是你的」与「今天到限额了」不可区分，新增错误码则违反 §5「不做的事」。
- **演示依赖**（引 [成员 3 任务书 §8](../../dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md) 自检「演示前实测可用」）：演示的人设必须**满足 ② 与 ④**——现场刚聊完就点按钮会被自己的防骚扰拦下；用演示前灌好的、静置超阈值且 1 小时内无用户消息的人设（或把 interval 调到最小值后静置）。
- **前端入口**按 [TECH_DESIGN §5.4](../../TECH_DESIGN.md) 的形态：对话页**隐藏按钮**（连点标题 5 次 / 仅开发环境显示），不做常规 UI 按钮——它服务于演示与自测，不是用户功能。

---

## 5. 不做的事

| 不做 | 去向 / 原因 |
|---|---|
| 为主动消息单独写一套生成逻辑、做模板消息 | §1：注入后走聊天链路（[TECH_DESIGN §5.4](../../TECH_DESIGN.md)） |
| 前端写 `lastNudgeAt`、或让 `PUT` 能改它 | 契约 §9 只读；唯一写入方是触发链路（[proactive-setting §1.5](../proactive-setting/spec.md)） |
| 给朋友圈加手动触发接口 | [AGENTS §7 红线 5](../../../AGENTS.md)：动态只由定时任务产生 |
| 开 `POST /schedules` 类新端点 | 契约 §10：日程只能从对话抽取，不提供手工新建 |
| 新增错误码 | 复用 `4001` / `4043` / `5001`（契约 §2，一 code 一 msg） |

---

## 6. 变更记录

| 日期 | 版本 | 变更 | 已广播 |
|---|---|---|---|
| 待填 | v1.0 | 初版 | ⬜ |
| 2026-10-05 | v1.5 | ① §4 判定未通过时的返回（`TriggerNow` 返回类型）由「待对齐」改为已定：`nil + nil` = 未通过，handler 返 200 + 空结果 `{messageId: 0, content: "", createdAt: null}`；② §7 第 1 条由「待方法签名」改为「已选 A + 签名已落地」；③ §7 第 2 条（含 §2 第 5 条指引）由「未落地」改为「已落地」 | ⬜ |

---

## 7. 待确认

1. **[nudge] 注入归谁**：**已选 A（成员 1，2026-10-02）**：`TriggerNow` 调 `ChatService.InjectNudge(ctx, NudgeInput) (*NudgeResult, error)`（`[nudge]` 写入语义单点真相在 `ChatService`）。签名 2026-10-05 已落地于 [chat_service.go](../../../backend/internal/service/chat_service.go)；返回的 `NudgeResult{MessageID, Content, CreatedAt}` 与契约 §9 的 `{messageId, content, createdAt}` 一一对应。
2. **`PROACTIVE_JOB_INTERVAL` 落地**：**✅ 已结案（2026-10-05）**——`backend/.env.example:41` 与 `deploy/.env.example:26` 均有 `PROACTIVE_JOB_INTERVAL=5m`（含 deploy 透传）。[MASTER §3 #4b](../../dev/MASTER.md) 列为成员 3 交接物，交接完成。
3. **bigserial 现状核实**：`grep backend/internal/model/` 确认 10 个 model 均已按 `type:bigint` + `autoIncrement` 写（引胶囊「外键禁止自增」条目；同款机制见 [persona-model §1.3](../persona-model/spec.md)）。

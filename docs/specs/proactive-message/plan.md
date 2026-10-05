# plan · 主动消息（Proactive Message）

> spec 见 [spec.md](spec.md)。本文件只管**怎么落地**——步骤、前置、验收、顺序纪律。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-proactive-trigger`（已切，本分支 3 个 commit，未推） |
| 负责人 | 成员 3 |
| 关联任务书 | [成员 3 任务书 §4](../../dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md)（Week 3 任务表）、§8（自检） |
| 关联契约 | [API_CONTRACT §9](../../API_CONTRACT.md)（3 个端点） |
| 关联红线 | [AGENTS §4.5](../../../AGENTS.md)（SSE 不适用本模块，红点不加服务端未读状态）、[§4.8](../../../AGENTS.md)（路由注册 / 定时任务在 Go 侧） |
| 前置已交付 | [proactive-setting](../proactive-setting/spec.md)（表 + 播种 + 仓储 `Get` / `UpdateOwned`，提交 `08c002c` 在 develop 上）；`pkg/response` / `pkg/errcode`（成员 1）；前端 `request.ts`（成员 2） |
| 分支纪律 | 存活 ≤3 天（[AGENTS §6](../../../AGENTS.md)）；按步拆 PR，一次只做一件事 |

---

## 1. 步骤拆解

| 步 | 产物 | 前置 | 验收 | 状态 |
|----|------|------|------|------|
| 1 | `internal/repository/proactive_repo.go`：新增三个**触发侧**方法——`CountTodayNudges`（查 `chat_messages`，只数注入行）/ `UpdateLastNudgeAt`（独立方法，不并入 `UpdateOwned`）/ `ListEnabledForScan`（JOIN `personas`）；签名与口径见下方注 1 | `model/proactive_setting.go` 已交付；`Get` / `UpdateOwned` / `CreateDefaultSettings` 已交付勿重写 | `go test ./internal/repository/`：计数口径（只数 `is_nudge = true`、+08:00 本地当日零点起、跨日边界）与扫描过滤（`enabled = true`、`last_message_at` 可 NULL）各有单测 | ✅ 已在 develop 交付（commit 973e207） |
| 2 | **新建** `internal/service/proactive_service.go`（service 层目前不存在）：实现 `TriggerNow` + settings 读写封装，调用 `ProactiveRepo` 的既有方法与步 1 新增的触发侧方法（settings 两端照 [proactive-setting §2](../proactive-setting/spec.md) 预写） | 步 1；注入已选 A：`TriggerNow` 调 `ChatService.InjectNudge`，签名 2026-10-05 已落地（[spec §7 第 1 条](spec.md)） | 服务层单测可直接调 `TriggerNow`：五层判定各自的跳过路径 + 全通过路径 | 🟡 已落盘，待装配验证 |
| 3 | `internal/handler/proactive_handler.go` + `RegisterProactiveRoutes(rg, h)` | 步 2 | 三个端点可达；缺 `personaId` → `4001`、越权 → `4043`（与 [proactive-setting §2](../proactive-setting/spec.md) 预写逐条比对） | 🟡 已落盘，待 router.go 挂载 |
| 4 | `internal/service/proactive_job.go`：按 `PROACTIVE_JOB_INTERVAL` 周期扫描，逐个调步 2 的 `TriggerNow` | 步 2；`backend/.env.example` 补上变量（spec §7 第 2 条）✅ 已满足（2026-10-05） | 日志可见每轮扫描与各判定跳过原因；把 interval 调成 1m 实测一个静置人设被触发 | 🟡 已落盘，待装配验证 |
| 5 | `router.go`：加一行挂载步 3 的 `RegisterProactiveRoutes` 到 **protected** 组 | 步 3 | 带 Token 可访问、免 Token `4010` | ⬜ |
| 6 | 前端：`src/api/proactive.ts`（复用 `request.ts`）+ 设置页（3 控件）+ 侧栏红点 + 触发按钮隐藏入口 | 成员 2 的 `request.ts` / 路由挂载点；步 3、5 | 浏览器实测：改 3 控件并持久化、刷新后回读一致；侧栏红点在有未触发机会时出现、进入对话后消失 | 🟡 设置页 / 红点已落盘（Mock 实测通过）；触发按钮未做——待触发链路合并后做 |

**表下注（落地时必须遵守的四点）**：

- **步 1 只补触发侧**：`CreateDefaultSettings` / `Get` / `UpdateOwned` 已随 proactive-setting 支交付在 develop 上，**不改已有签名（`Get` / `UpdateOwned` / `CreateDefaultSettings` 三签名不动）**；本步只加三个触发侧方法：
  1. `CountTodayNudges(ctx, userID, personaID uint64) (int64, error)`——**查 `chat_messages` 表**（`is_nudge` 列在该表，不在 `proactive_settings`）：`WHERE persona_id=? AND user_id=? AND is_nudge = true AND created_at >= 本地当日零点（+08:00，不依赖进程 TZ）`。只数注入行、**不数 assistant 回复**（[spec §3.d](spec.md)）。**时区口径 +08:00——不得静默用 `time.Now()`**；取值方式（硬编码常量 / 从 config 读）在方法注释写明。
  2. `UpdateLastNudgeAt(ctx, tx *gorm.DB, userID, personaID uint64) error`——**独立方法**，不并入 `UpdateOwned`（后者 map 键集合已冻结为 4 个可改列，不含 `last_nudge_at`，见 `proactive_repo.go` 第 55 行注释）。
  3. `ListEnabledForScan(ctx context.Context) ([]ScanRow, error)——定时任务专用：JOIN `personas` 拿 `last_message_at`，筛 `enabled = true` 的配置行；返回至少 `userID, personaID, intervalMin, intervalMax, dailyLimit, lastMessageAt`（可 nil）。**本方法为跨用户扫描，不带 `user_id` 条件**——与 [spec §2](spec.md) 的双条件纪律不冲突；在方法注释写明「定时任务专用，返回行不落库不越权」。
- **步 2 的签名底线**：`TriggerNow` 需要两个入参——`userID` 与 `personaID`（手动端点 `userID` 从 Token 取、定时任务从扫描行取）。`user_id` 从哪来可以讨论，**从请求体读是红线**（[AGENTS §4.3](../../../AGENTS.md)）。注入调用按 [spec §7 第 1 条](spec.md) 已选 A：`TriggerNow` 调成员 1 的 `ChatService.InjectNudge(ctx, NudgeInput) (*NudgeResult, error)`，签名 2026-10-05 已落地。
- **步 4 与手动端点必须复用步 2 的同一个 `TriggerNow`**（[MASTER §4.2](../../dev/MASTER.md) 🚨；[spec §4](spec.md)）——步 2 一旦定下，谁都不许复制第二份判定逻辑。「演示不是作弊」的立论全押在这条上。
- **步 6 的前端两处口径**：① `api/proactive.ts` 复用成员 2 的 `request.ts`——`code === 200` 时它直接返回 `data`，组件里**不判 `res.code`**（[AGENTS §4.9](../../../AGENTS.md)）；② 侧栏红点**不加服务端未读状态、不加新端点**——红点数据从既有 `GET /personas` 的 `lastMessageAt` 与客户端已读位比对得出；触发按钮按 [TECH_DESIGN §5.4](../../TECH_DESIGN.md) 做隐藏入口（连点标题 5 次 / 仅开发环境），否则 🚨 端点没有演示路径。

**PR 拆分建议**：步 1–2 一个 PR（仓储 + 服务，含单测）→ 步 3+5 一个 PR（HTTP 层 + 挂载行，成员 1 执行挂载）→ 步 4 一个 PR（定时任务）→ 步 6 一个 PR（前端）。

---

## 2. 人工审查 grep 清单（落代码前对 spec / plan 跑）

```bash
grep -n "TriggerNow"  docs/specs/proactive-message/spec.md docs/specs/proactive-message/plan.md
grep -n "nudge"       docs/specs/proactive-message/spec.md docs/specs/proactive-message/plan.md
grep -n "user_id"     docs/specs/proactive-message/spec.md docs/specs/proactive-message/plan.md
grep -n "daily_limit\|enabled" docs/specs/proactive-message/spec.md docs/specs/proactive-message/plan.md
```

四个词各须有**语义落点**（不是只划过标题或链接）：

| 词 | 必须能回答 |
|----|-----------|
| `TriggerNow` | 唯一入口是谁的两个调用方；不可复制的纪律写在哪个文件第几节；签名底线（`userID` 来源） |
| `nudge` | 注入的消息形态（`role='user'` + `is_nudge=true`）；当日计数只数它；④ 判定必须排除它；注入归谁写在 §7 |
| `user_id` | 双条件查询；两个合法来源；永不从请求体读 |
| `daily_limit` / `enabled` | 五层判定里各自的位置；计数口径（本地当日零点起、仅 nudge）；`enabled` 是扫描过滤条件 |

> 教训沿用 [proactive-setting 的 plan](../proactive-setting/plan.md) 同款清单——§0.1 引的每个关键词都必须在正文有独立落点，否则改 spec 时容易删掉判定语义、只留引用。

---

## 3. 顺序纪律

1. **步 5 由成员 1 执行**：`router.go` 只加一行挂载（[MASTER §4.5](../../dev/MASTER.md)）——勿两人同时改 `router.go`（历史事故来源）。
2. **步 2 定下即冻结**：`TriggerNow` 的判定顺序与签名一旦合并，步 4 与步 3 的手动端点只能**调用**，不能拷贝（[spec §4](spec.md)）。
3. **前端复用而不重写**：`api/proactive.ts` 走成员 2 的 `request.ts`；组件不直接写 axios（[AGENTS §3](../../../AGENTS.md) 纪律）。
4. **每步跑全量自测**：`cd backend && go build ./... && go test ./... && go vet ./...`；前端 `npx tsc --noEmit && npm run build`（[AGENTS §2](../../../AGENTS.md)）。
5. **产出即提交**：每步一个 commit，`<type>(proactive): <subject>`（[AGENTS §6](../../../AGENTS.md)），不攒到最后。

---

## 4. 待确认

1. **[nudge] 注入归谁**（**已选 A（成员 1，2026-10-02），签名已落地（2026-10-05）**）：同 [spec §7 第 1 条](spec.md)——`TriggerNow` 调成员 1 的 `ChatService.InjectNudge(ctx, NudgeInput) (*NudgeResult, error)` 注入；签名已落地于 [chat_service.go](../../../backend/internal/service/chat_service.go)，步 2 不再留占位。
2. **`PROACTIVE_JOB_INTERVAL` 落地**（**✅ 已结案（2026-10-05）**）：同 [spec §7 第 2 条](spec.md)——`backend/.env.example:41` 与 `deploy/.env.example:26` 均有 `PROACTIVE_JOB_INTERVAL=5m`。
3. **bigserial 现状核实**：同 [spec §7 第 3 条](spec.md)——已预跑 grep，10 个 model 均无 `bigserial`。

---

## 5. 进度记录

| 日期 | 事件 |
|------|------|
| 2026-10-02 | spec / plan 初版落盘（草稿）；分支待切，代码未开工 |
| 2026-10-02 | 分支已切（未 commit）；按成员 1 回复修订：步 1 方法化（`CountTodayNudges` / `UpdateLastNudgeAt` / `ListEnabledForScan`）、步 2 措辞、spec §3.d 已确认、spec §7 第 1 条已选 A·待签名 |
| 2026-10-04 | 步 6 前端（Mock 先行）落盘于 `feature/frontend-proactive-settings`（未提交）：`types/proactive.ts`、`api/proactive.ts` + `api/mock/proactive.ts`（`VITE_USE_MOCK` 切换）、`views/proactive/ProactiveSettingsView.vue`（3 控件 + 人设切换 + 三态）、侧栏红点（`lastMessageAt` 与本地已读位比对，localStorage 持久化）。`vue-tsc` / `npm run build` 通过，Mock 浏览器实测通过。挂载点：前端 `router/index.ts` 与 `MainLayout.vue` 各加一行（成员 2 的文件，PR 里请留意）。真接口待 `router.go` 挂载后联调 |
| 2026-10-04 | **触发按钮未做**（步 6 的隐藏入口项）：`POST /proactive/trigger` 尚未挂载（步 3、5 未完成），待触发链路合并后按 [TECH_DESIGN §5.4](../../TECH_DESIGN.md#54-主动消息实现) 补 |
| 2026-10-05 | TriggerNow + /proactive/trigger 落盘，待成员 1 同 PR 补 main.go / router.go 装配 |

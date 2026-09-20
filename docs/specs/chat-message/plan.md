# plan · 聊天记录（Chat Message）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-chat-message-model`
> 负责人：成员 3 ｜ 创建：2026-09-14 ｜ 最后更新：2026-09-20
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：步骤产物与代码写法以 [spec](spec.md) 为准，本文件**不重复**；
> 这里只写**计划独有**的内容——步骤顺序、审查方法、打回标准、进度记录。
>
> ⚠️ **先读 spec §2**：本功能名里的「CRUD」只有 **R** 是端点。C 由服务端落库（三条链路复用同一个 `Create`），U / D 在 DDL 与契约两层都不存在——**不该写的代码比该写的更重要**。

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 前置 | 状态 |
|---|---|---|---|---|---|
| 0 | 通用分页结构 | `internal/dto/common_dto.go` | `PageResult[T]` + `PageSizeDefault/Max` + `ClampPage()` 只有这一份；persona 分支若已落地则**直接消费** | 无 | 未开始 |
| 1 | **翻译 Struct** | `internal/model/chat_message.go` | 9 列 + 2 个外键关联 + `role` 的 CHECK 全被声明；字段对照 spec §1 逐行对齐 | 无 | ✅ 已落地 |
| 2 | 建表 | `internal/model/migrate.go` | `AutoMigrate` 追加 `&ChatMessage{}`，顺序在 `&Persona{}` 之后；`\d chat_messages` 有 2 个 CASCADE 外键 + CHECK + 2 个索引 | 步骤 1 | ✅ 已落地 |
| 3 | 请求 / 响应结构 | `internal/dto/chat_dto.go` | `ChatMessageResponse`（**8 字段**）+ 构造器 + `MessageListQuery`；camelCase 对齐契约 §5 | 步骤 0、1 | 未开始 |
| 4 | 归属判定 | `internal/repository/persona_repo.go` 的 `ExistsOwnedByUser` | 一条查询、两个条件；**只回 bool**，调用方拿 false 一律 `4043` | 无 | 未开始 |
| 5 | **仓储层** | `internal/repository/message_repo.go` | `ListByPersona`（含 `total`）与 `Create`；签名强制带 `userID` 与事务句柄；**不存在**任何不带 `user_id` 的业务查询、不存在删除方法 | 步骤 1 | 未开始 |
| 6 | **业务层** | `internal/service/chat_service.go` | **先判归属再查消息**（spec §3.1）；未命中一律 `4043`；分页钳制在此；不依赖 `*gin.Context` | 步骤 3、4、5 | 未开始 |
| 7 | **HTTP 层** | `internal/handler/chat_handler.go` | 薄；含 `RegisterChatRoutes`；错误 `_ = c.Error(err)` 上抛；`personaId` 解析失败传 `0` | 步骤 6 | 未开始 |
| 8 | 路由挂载 | `router.go`（成员 1 的文件） | 群里同步后加一行，**不要与成员 1 同时改**；`POST /chat/stream` 由他那一步在同一个 `RegisterChatRoutes` 里追加，**现在不要写空壳路由** | 步骤 7 | 未开始 |
| 9 | 越权 / 分页 / 级联专项验证 | 本文件 §3.3 的命令 | spec §5 的验收项逐条打勾 | 步骤 8 | 未开始 |
| 10 | **人工审查 + PR** | `docs/dev_notes/chat_message_notes.md`、PR | §3 的审查清单全过；至少 1 人 Approve | 步骤 9 | 未开始 |

**优先级**：步骤 0-5 是**流式对话（Week 2 生死线）的前置**——SSE 链路要 `Create` 才能落库，所以仓储层先于端点交付。等待公共层期间，用 §3.3 的 SQL 直接对 PG 验证仓储层行为。

## 2. 文件清单

| 路径 | 作用 | 谁还会用到 |
|---|---|---|
| `internal/dto/common_dto.go` | `PageResult[T]` + 分页常量 + `ClampPage`，**五处分页共用** | **成员 1**（messages / memory）、成员 3（personas / moments / schedules）——**别写两份** |
| `internal/model/chat_message.go` | `chat_messages` 的 GORM 实体 | **成员 1**（SSE 落库）、成员 3（主动消息 / 日程） |
| `internal/dto/chat_dto.go` | 消息响应结构 + 构造器 + 列表查询参数 | **成员 2**（据此写 `types/chat.ts` 与 Mock） |
| `internal/repository/persona_repo.go` | 增 `ExistsOwnedByUser`（**人设模块的文件，本分支一并落地**） | **成员 1**（SSE 链路判归属；后续会再要 `GetOwned`） |
| `internal/repository/message_repo.go` | `ListByPersona` / `Create` | **成员 1 + 成员 3**（三条写入链路共用 `Create`） |
| `internal/service/chat_service.go` | 归属校验 + 分页 + DTO 转换 | 成员 1（SSE 的 service 与它同包） |
| `internal/handler/chat_handler.go` | 1 个端点 + `RegisterChatRoutes` | 成员 1（他往里加 `POST /chat/stream`） |
| `docs/dev_notes/chat_message_notes.md` | **我的审查笔记**（不是给别人的文档，**不入库**） | 只有我；与 `user_model_notes.md` 同级 |

## 3. 我手动审查 AI 代码的计划

> 前提：AI 生成的代码**默认不可信**，尤其是「越权防线」「分页边界」「不该写的代码有没有被顺手写上」这三处——它们写错了照样能跑通、照样能演示，只在特定输入下才暴露。审查的**目标不是"能跑"，是"我能逐行解释它为什么安全"**。

### 3.1 审查方法（三遍）

| 遍 | 做什么 | 判据 |
|---|---|---|
| **第一遍 · 逐行重写注释** | 把 AI 产出的每个文件抄进 `docs/dev_notes/chat_message_notes.md`，**用自己的话**加注释 | **写不出注释的那一行，就是我没看懂的地方** |
| **第二遍 · 对照核对** | 拿 DDL + 契约 §5 + spec 全文逐字段/逐端点核对 | 每个 JSON 字段名都能在契约里指到来源；**响应里多出来的字段一律删掉**（尤其 `userId`） |
| **第三遍 · 对抗验证** | 主动构造能打破代码的输入（跨账号、空对话、不存在的 id、`pageSize=100000`、`page=abc`、`personaId=abc`、同秒消息、越界页），按 §3.3 实跑 | 每个高危点都有一次实跑记录 |

**第三遍是核心**：前两遍只能证明"代码符合我的预期"，第三遍才能证明"代码在我不期望的输入下不泄漏、不崩、不静默返回错东西"。

### 3.2 七个高危点

| # | 高危点 | 为什么会错 | 怎么验 |
|---|---|---|---|
| 1 | **越权被伪装成空对话** | "只查消息表、查不到就返回空列表"是最自然、也最像对的写法——**但它把"看别人的对话"变成了 `200 + []`**。本功能头号风险，且完全静默 | 用**另一个账号的 Token** 打 A 的 `personaId`：必须 `4043`；再拿一个**自己没聊过的人设**打：必须 `200 + []`。两次结果**必须不同**，这才证明归属判定真的生效了 |
| 2 | **归属判定返回三态 / 多一个错误码** | 容易写 `if !exists { 4043 } else if !owned { 4043 }` 这类"看起来更精确"的分支，或给非法 `personaId` 返回 `4001` | grep 确认 service 里只有**一处** `4043` 出口；各种非法输入实跑一遍，响应里**只出现 `4043` / `4010` / `200`** |
| 3 | **分页不设上限 / `total` 口径错** | `pageSize` 无上限 = 一次拉全表；`COUNT(*)` 全表 = **把全站消息量级泄漏给任意用户** | `pageSize=100000` → 最多 100 条、响应 `pageSize=100`；两个账号各造消息，互相核对 `total` 只是**自己那个人设**的消息数 |
| 4 | **排序不稳** | 只写 `created_at DESC`，同秒消息顺序不定 → 翻页重复或漏项，**但页面看着"能滚动"** | 手工把两条消息的 `created_at` 改成完全相同，`pageSize=1` 逐页拉，逐条核对不重不漏 |
| 5 | **空列表是 `null`** | Go 的 nil slice 序列化成 `null`，前端 `.map` 崩；而在后端看响应体之外的地方一切正常 | 构造"人设属于我但没消息" → 响应体里必须是 `"list":[]`；越界页同样 |
| 6 | **不该写的代码被顺手写上** | 最容易出现的是：给消息加 `DELETE`（违反总纲 §0）、加 `Save()`、给 `migrate.go` 顺手加 `updated_at`、或给 SSE 预埋一个 `emotion` 事件 | `grep -rnE "DELETE FROM chat_messages\|func.*Delete\|updated_at\|deleted_at\|emotion\"" internal/` 逐条确认命中都合理 |
| 7 | **写入路径的 `last_message_at` / `state`** | 落库后对话列表不刷新（漏更新），或用 `Save(&persona)` 把 `state` 覆盖成零值（`familiarity` 直接归零，**不可逆**） | 走一次 `Create` + 更新，`psql` 里核对 `last_message_at` 等于消息时间、`state` 一字未变（先手工塞 `{"familiarity":42,"self_note":"x"}` 再验） |

**其中三条是"只看 tag / SQL 就能判死"的，审查时逐条回看**：

1. **`ChatMessage.ID` 严禁出现 `bigserial`**，必须 `type:bigint` + `autoIncrement`——它被 `user_memory.source_message_id` 引用，写错会让下游外键列长出 `nextval` 默认值，**今天看不出来，写记忆表那天才炸**。
2. **`total` 的查询条件必须带 `user_id` + `persona_id`**，与 `list` 逐字相同——防全站数据量级泄漏。
3. **`GET` 接口必须"先判归属、后查消息"**，越权一律 `4043`——顺序反了就是把越权伪装成空对话。

### 3.3 必须亲眼看到的证据（不接受"我觉得没问题"）

```bash
# 0) 编译 + 静态检查（提交前必跑）
cd backend && go build ./... && go vet ./... && go test ./...

# 1) 表结构真的建对了吗（命令里不写密码，红线 1）
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c '\d chat_messages'
#    要看到：两个 ON DELETE CASCADE 外键、CHECK (role IN ('user','assistant'))、
#            idx_messages_persona_time / idx_messages_user_id，且【没有】updated_at / deleted_at
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c '\ds'
#    要看到：只有 chat_messages_id_seq（多出 persona_id/user_id 的序列 = 关联复制坑）

# 2) 越权 vs 空对话 vs 不存在（三条必须给出两种不同结果）
curl -s "localhost:8080/api/v1/chat/personas/<A的人设>/messages" -H "Authorization: Bearer $TOKEN_B"
#    期望 4043
curl -s "localhost:8080/api/v1/chat/personas/<A的人设>/messages" -H "Authorization: Bearer $TOKEN_A"
#    期望 200 + list 有内容；再拿 A 的「没聊过的人设」打 → 200 + []
curl -s "localhost:8080/api/v1/chat/personas/999999/messages" -H "Authorization: Bearer $TOKEN_A"
#    期望 4043，且 message 与第 1 条【逐字相同】

# 3) 非法输入不产生 4001 / 4030 / 4040
curl -s "localhost:8080/api/v1/chat/personas/abc/messages" -H "Authorization: Bearer $TOKEN_A"
curl -s "localhost:8080/api/v1/chat/personas/1/messages?page=abc&pageSize=100000" -H "Authorization: Bearer $TOKEN_A"
#    期望：非数字 personaId → 4043；分页参数非法 → 200 且 pageSize 回填 100
curl -s "localhost:8080/api/v1/chat/personas/1/messages"   # 无 Token → 4010

# 4) 排序与分页稳定性（同秒消息）
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c \
  "UPDATE chat_messages SET created_at = '2026-09-10T14:30:00+08:00' WHERE persona_id = <id>;"
curl -s "localhost:8080/api/v1/chat/personas/<id>/messages?page=1&pageSize=1" -H "Authorization: Bearer $TOKEN"
#    逐页翻完，核对不重不漏（id DESC 兜底生效）

# 5) 情绪字段与空值序列化
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c \
  "UPDATE chat_messages SET emotion_label = NULL, emotion_score = NULL WHERE id = <id>;
   UPDATE chat_messages SET emotion_label = 'sadness', emotion_score = 0.870 WHERE id = <id2>;"
#    期望：null / null（不是 "" 和 0）；0.87 读出来是 0.87（NUMERIC ↔ *float64 往返）

# 6) code 层面 grep（都要无输出；⚠️ 期望 0 的 grep 不要用 && 串联，无匹配时退出码为 1 会断链）
grep -rn "response.Fail"        internal/handler/chat_handler.go
grep -rnE "Delete\(|Save\("    internal/repository/message_repo.go
grep -rnE "\b(400[0-9]|403[0-9]|404[0-9])\b" internal/service/chat_service.go
grep -rn "gin"                  internal/service/chat_service.go
grep -rn "type PageResult"      internal/dto/     # 期望只有 1 行

# 7) 红线 1 自查：不许有密钥进入本次改动
git status --short && git diff --cached --name-only | grep -E '\.env$|\.key$|\.pem$'
```

> ⚠️ **别用 `&&` 串联**：第 6 组里期望"无输出"的 grep 在无匹配时退出码为 1，链条会在第一条就断掉，后面的检查**静默不执行**——"没输出"会被误读成"全过了"。逐条跑。

> `$TOKEN` / `$TOKEN_A` / `$TOKEN_B` 从登录接口拿，**不要写死在脚本或 `_test.go` 里**（红线 1）。

### 3.4 拒绝标准（出现任一条就打回重写，不做"小修小补"）

| 打回条件 | 为什么不能只小修 |
|---|---|
| **越权请求返回 `200 + []`**（没判归属，或判了没用上） | 本功能最严重的错误：把"读别人的对话"伪装成"对话是空的"。防线必须是结构性的，逐处打补丁会漏 |
| 仓储层存在**不带 `user_id`** 的业务查询 | 最后一道闸门失守 |
| service 里 `4043` 有多处出口 / 出现 `4030` / 出现 `4001` / 出现 `4040` | 出口一多就会有分支漏改；多出的错误码 = 可观测差异 = 泄漏存在性 |
| `total` 用了不带 `persona_id` + `user_id` 条件的 `COUNT` | 数字错 + 泄漏全站量级 |
| `ListByPersona` 排序缺 `id DESC` | 翻页重复/漏项，且**页面看着正常**，最难排查 |
| 空列表返回 `null` | 前端直接崩 |
| 出现 `Delete` / `Update` / `Save` 的消息写法 | 违反总纲 §0（不可单独清空）；`Save` 覆盖 `state` 是不可逆的数据损坏 |
| `Create` 里有 `tx == nil` 的兜底分支 | 会让"忘了传事务"静默变成"不在事务里"，事务白做 |
| `ChatMessage.ID` 写成 `bigserial` | 下游 `user_memory.source_message_id` 长出 `nextval`，**今天看不出来，写记忆表那天才炸** |
| 多出一个 `POST /chat/messages` / `DELETE /chat/messages/:id` 之类的空壳路由 | 预留路径会被后来的人当成"已规划的功能"实现出来，与总纲 §0 冲突 |
| service 里出现 `*gin.Context` | 破坏分层（红线 7），后续无法单测 |
| `PageResult` 出现第二份定义 | 两份同名类型在联调时会出现"字段对不上"的诡异问题，最难排查 |

**审查通过的唯一标准**：§3.3 的命令全部实跑过，且 §3.2 每个文件我都能逐行解释。**审查笔记落到 `docs/dev_notes/chat_message_notes.md`**（该目录被 git 忽略，所以要留痕的结论必须回填进 spec / plan）。

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| **越权被伪装成空对话** | 读别人的聊天记录，且**完全静默**——没有报错、没有异常日志，只有内容不对 | spec §3.1 的三步顺序 + §3.2 #1 的"两种结果必须不同"实跑；这是本功能唯一必须演给 review 看的证据 |
| **两人同时写 `chat_*.go`** | `router.go` / `chat_handler.go` 都是两人交汇点，同时改必冲突（AGENTS §4.8） | 步骤 10 前先在群里对齐（spec §6.3 #1）；`RegisterChatRoutes` 由成员 1 追加 `POST /chat/stream` 那一行，**同一时间只有一个人动这个文件** |
| `PageResult` 出现两份（persona 分支与本分支各写一份） | 联调时字段对不上，最难排查 | 本功能第一个提交就把 `common_dto.go` 落地并在群里说一声（spec §6.3 #2）；后落地的一方直接消费 |
| `emotion_score` 的 pgx 编码问题 | SSE 落库时才发现写不进去 | 本功能先定 `*float64`；**在 SSE 那一步写入前**用一条真实 INSERT 验一次（spec §6.3 #3），不行就换 `driver.Valuer` |
| 写入方漏更新 `last_message_at` | 对话列表顺序不刷新、主动消息空闲判定永远算不出来（表现为"AI 永远不主动发消息"） | spec §4 不变量 2 写进 `Create` 的注释 + 交接给成员 1 / 成员 3；验收里专门有一条 |
| `[nudge]` 内容格式 / `isNudge` 渲染方式未对齐 | 两个模块各存一份格式、前端可能把 `[nudge]` 原文当用户的话显示出来 | spec §6.2 已列为交接项，与成员 2 单独对齐（后端不参与） |

## 5. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-14 | spec / plan 第 1 版落地；确认契约 §5 该端点错误码已收敛为 `4043`（无需改全局文件、不阻塞合并） | 成员 1 的 `internal/middleware` / `router.go` 未落地（仅阻塞步骤 7-8）；文件归属待群里对齐 |
| 2026-09-20 | **按 AGENTS §5.2 引用优先精简**：删除与 spec 重复的实现要点、复制粘贴的通用约定与红线自查表；**把原表格下方三行游离的审查要点并入 §3.2**（原本未加标记、紧贴在步骤表后面）；步骤 1-2 状态按实际落地情况回填（401 → 169 行，行数按 `wc -l` 含空行） | 步骤 0、3-10 未开始；公共层已就位，可开工 |

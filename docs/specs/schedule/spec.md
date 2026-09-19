# spec · 日程提醒（Schedule）· 数据模型

> 本文件是**合同**：定义本分支做什么、做到什么算完成，以及下游分支必须遵守的预写设计。
> 合并后即冻结，任何字段或行为变更必须升版本并在群里广播。
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：DDL 原文、契约字段、错误码表、通用约定一律**不复制**，只给链接；
> 本文件只写**本表独有**的内容——独有陷阱、独有决策、独有验收项。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-schedule-model` |
| 状态 | v1 待人工审查（**未过审不动手写代码**） |
| **本支范围** | **模型层：`Schedule` struct + `migrate.go` 一行**（§0.1） |
| 负责人 | 成员 3（`JMX2033`） |
| 依赖分支 | `feature/backend-moment-like-model` 及其余 9 张表的 model 分支（**均已合并进 main**） |
| 关联契约 | [API_CONTRACT §10](../../API_CONTRACT.md#10-日程提醒schedules-p1)（`Schedule` 实体 + 三个端点） |
| 关联设计 | [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl)（`schedules` DDL，含字段取值理由）、[§5.7](../../TECH_DESIGN.md#57-日程提醒p1)（业务流程与解析器）、[§4.6](../../../AGENTS.md)（不是第 4 个 Agent） |
| 关联 spec | [moment-like/spec.md](../moment-like/spec.md)（上一支；**它把"序列收口"交棒给本支**）、[user-memory/spec.md](../user-memory/spec.md)（**同形三外键 + 同一处 `SET NULL` 事故**）、[ai-moment/spec.md](../ai-moment/spec.md)（§1.1 序列污染原理、§4.6 反例） |
| 关联红线 | [AGENTS §4.3](../../../AGENTS.md)（数据边界）、§4.6（日程不是 Agent）、§5.1（注释纪律）、[§7](../../../AGENTS.md)（红线 5：**不许给日程开 `POST /schedules`**） |

---

## 0. 范围声明（**先读这一节**）

### 0.1 本支只做四件事

| # | 文件 | 动作 | 内容 |
|---|------|------|------|
| 1 | `backend/internal/model/schedule.go` | 新增 | `ScheduleStatus` 类型 + 三个常量（§1.4）+ `Schedule` struct（§3）+ `TableName()` |
| 2 | `backend/internal/model/migrate.go` | 修改 | 追加一行 `&Schedule{}`（**第 10 行，全项目最后一张表**） |
| 3 | `backend/internal/model/schedule_test.go` | 新增 | JSON 键集断言（§5 分组 C） |
| 4 | `.learn/schedule-model.md` | 新增（**不入库**） | 学习笔记：三外键形态、触发链路的越权推演、排序兜底推演、named type + `default:` 的实验 |

### 0.2 本支不做什么（**其余全部交接，去向逐个标明**）

| 不做的事 | 去向 |
|---------|------|
| `repository/schedule_repo.go`：落库 / 列表 / 取消三条 SQL（§4.1） | `feature/backend-schedule-api` |
| `service` / `handler` / `dto` / `RegisterScheduleRoutes` | `feature/backend-schedule-api` |
| 到期扫描定时任务（`schedule_job`，复用 `PROACTIVE_JOB_INTERVAL`） | `feature/backend-schedule-job`；触发逻辑与 `TriggerNow` 同源 |
| 时间解析器 `ai-service/app/schedules/**`（规则档 / LLM 档） | ai-service（成员 1） |
| `router.go` 的那一行 | **成员 1**（我不碰这个文件） |
| 前端 `api/schedule.ts`、`views/schedule/ScheduleView.vue` | 前端分支（归属成员 3，但**不在本支**） |

> ⚠️ **本支不含任何 SQL**。§4 的写入路径是**预写设计**，供 `feature/backend-schedule-api` 落地；
> 配套的**运行时**验收（跨账号 `4040`、触发往别人对话写消息）登记在 §5 分组 E，不在本支跑。

---

## 1. 本表独有的硬性要求（违反任何一条都不要提交）

### 1.0 三个外键、两种 `ON DELETE` —— 要求里说的"外键"只是其中一条

开工前的口头要求是「外键指向 `personas(id)` 带 `constraint:OnDelete:CASCADE`」。
这句话**作为 `persona_id` 那一列的描述是准确的**，但**按单数去理解整张表会漏掉两条外键**。
权威 DDL 上有 **3 条**外键，`ON DELETE` 分**两种**：

| DDL 列 | 指向 | `ON DELETE` | Go 类型 | 语义 |
|--------|------|-------------|---------|------|
| `user_id` | `users(id)` | **CASCADE** | `uint64`（值） | 删账号 → 清其全部日程 |
| `persona_id` | `personas(id)` | **CASCADE** | `uint64`（值） | 删人设 → 清该人设的日程 |
| `source_message_id` | `chat_messages(id)` | **SET NULL** | **`*uint64`（指针）** | 删消息 → 只断开溯源，**日程留下** |

三处独立来源一致（[TECH_DESIGN §6.2 DDL](../../TECH_DESIGN.md#62-核心表-ddl)、[§6.3 设计说明](../../TECH_DESIGN.md#63-设计说明) 的
"外键统一 `ON DELETE CASCADE`（`source_message_id` 用 `SET NULL`）"、[§5.7 流程 3](../../TECH_DESIGN.md#57-日程提醒p1)）。

**两个错法，一个是"静默"、一个是"响的"，都要防**：

| # | 错法 | 后果 | 会不会报错 |
|---|------|------|:---:|
| ① | 只写 `persona_id` 一条外键，`user_id` / `source_message_id` 不写关联字段 | 「删账号级联清日程」「删消息断溯源」两条验收**都不成立** | **不会**——`go build` / `vet` 全过，只是库里少了外键 |
| ② | 把 `constraint:OnDelete:CASCADE` **照要求推广到 `source_message_id`** | 删一条消息**顺手删掉一条日程**——与"溯源"语义、与 DDL 直接冲突 | **不会**——建表成功，症状只在删消息时出现 |

> 错法 ② 是本表独有的一处"要求比 DDL 更窄，但照着抄会写宽"的地方：
> 上一支 `moment_likes` 的两个外键**都是** CASCADE，从那里带过来的肌肉记忆正好是错的。

**为什么 `source_message_id` 是 `SET NULL`**：[TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl) 已定稿——
"它只是'这条提醒从哪儿来的'的溯源，丢了不影响提醒本身"。与 [user_memory 的同名列](../user-memory/spec.md) 同一个理由，**引用不重复**。

### 1.1 三个外键列的 Default 都必须为空（**头号验收项**，比前两支多一列）

> 来源：**本表是全项目第三张有三个外键列的表**（前两张是 `user_memory` 与 `moment_comments`），
> 且是本支的头号验收对象——三列都要查，比前两支都多。
> 原理见 [ai-moment §1.1](../ai-moment/spec.md)（关联建表时 GORM 会把被引用列的 `DataType` **复制**到外键列上），**本支不重复推导**。

`schedules` 的 3 个外键列 `user_id` / `persona_id` / `source_message_id`，**三个都不能带默认值**。
`source_message_id` 尤其危险：它**可空**、且是**唯一一处 `SET NULL`**，看起来"本来就该有个默认值"。

**两个产生机制**（[ai-moment §1.1](../ai-moment/spec.md)）：

| # | 机制 | 本支状态 |
|---|------|---------|
| ① | 上游 `users.id` / `personas.id` / `chat_messages.id` 写成 `bigserial`，GORM 把 `DataType` 复制到本列 | 三张表**都**已用 `type:bigint` 切断（历史 fix 提交）；**本支确认它真的生效** |
| ② | 本字段自己误加 `autoIncrement` | 本支的 tag 纪律（§2 对照表 + §5 分组 A） |

**验收（§5 分组 A）**：`\d schedules` 里三行的 Default 列**都为空**；`\ds`（或 `pg_sequences`）里
`schedules_user_id_seq` / `schedules_persona_id_seq` / `schedules_source_message_id_seq` **各 0 结果**。

**正反两面（缺一不可）**：

- **不许有**：上面三个名字 → **各 0 结果**
- **必须有**：`schedules_id_seq` **恰好 1 个**（来自 `id` 的 `autoIncrement`）。
  只查"没有多余序列"是**半套标准**——把 `ID` 的 `autoIncrement` 一起删掉同样能过，代价是本表再也插不进数据。

> **要求里那条命令的精度修正**：原话是"`\ds` 里搜 `schedules_*_id_seq`（除 `schedules_id_seq`）应 0 结果"。
> 这条**有判别力**（不像 [moment-like §1.1](../moment-like/spec.md) 那条恒不存在的名字），两点补充：
> **①"除 `schedules_id_seq`"是多余的一句**——该名字**不在**这个 pattern 的命中范围内（`schedules_` 之后还需要一段 `*_id_seq`），
> 写不写都不影响结果，别误以为它会被搜到。
> **② `\ds` + 通配符是人读版**；机器版用 [user-memory spec 的既定形式](../user-memory/spec.md)：
> `SELECT sequencename FROM pg_sequences WHERE schemaname='public'` 配**显式三个名字**，不靠模糊匹配。

**全库序列的最终收口（本支**第一次**跑得动）**：
`schedules` 是 `migrate.go` 里的**第 10 行 = 全项目最后一张表**（契约变更记录："表数 9 → 10"）。
前面 9 张表落地时，每次只能验"本表 + 相邻表"的局部结论；
**本支合并后，全库序列可以一次性对齐**：`pg_sequences` 里**恰好 10 个** `*_id_seq`，即
`users_id_seq` `personas_id_seq` `chat_messages_id_seq` `user_memory_id_seq` `user_profile_id_seq`
`ai_moments_id_seq` `moment_comments_id_seq` `moment_likes_id_seq` `proactive_settings_id_seq` `schedules_id_seq`，
且**没有任何** `<表名>_<外键列名>_seq` 形态的序列。

> 这条同时**关闭**了 [moment-like §5 分组 E](../moment-like/spec.md) 留下的那一项
> （"本次已达成的'朋友圈三序列干净'结论不随新表回归——新表落地后重跑"）。

> **若验收失败怎么办**：三个外键里任何一个长出序列，说明**上游那张表的 `ID` tag 被改坏了**（不是本支的代码）。
> 修复要改上游 model 并**另开 `fix/` 分支**，不要在本支顺手改别人已合并的文件。
> 注意 GORM `AutoMigrate` **不会**移除已存在列的 DEFAULT——改完 tag 后需**重建该表**（仅限开发库，[AGENTS 红线 8](../../../AGENTS.md)）。

### 1.2 越权防线：本表**有** `user_id`，走**直接过滤**——但"直接过滤"**不**等于"只过滤 `user_id`"

**分支判定（要求 3 的定案）**：`schedules` **有** `user_id` 列 → 走**直接过滤**，
**不**走 `JOIN personas`（那是给"只挂 `persona_id`"的表准备的分支）。本表不需要为了拿 `user_id` 而联表。

> ⚠️ **但这一步只回答了"怎么拿到 `user_id`"，没有回答"要过滤几个条件"。** 这两件事很容易被读成一件。
> 本表与 `user_memory` / `user_profile` 同为「**一人设一份**」，[AGENTS §4.3](../../../AGENTS.md) 对这类表的要求是
> **同时带 `persona_id` 与 `user_id`**；契约 §10 又把 `personaId` 定为**必传**（缺 → `4001`）。
> 于是列表查询固定为 **`WHERE persona_id = ? AND user_id = ?`**，两个条件缺一不可：

| 漏掉哪个 | 症状 | 定性 |
|---------|------|------|
| 只带 `persona_id` | 查到别人的日程 | **越权**（[AGENTS 红线 3](../../../AGENTS.md)） |
| 只带 `user_id` | **同一账号**下 A 人设的日程出现在 B 人设的列表里 | 跨人设串号——不是跨用户，但同样错 |

**三个端点的归属条件（预写，下游照此落）**：

| 端点 | 归属条件 | 说明 |
|------|---------|------|
| `GET /schedules` | `persona_id = ? AND user_id = ?` | 人设归属校验失败按 **`4043`**（契约 §10；[AGENTS §4.3](../../../AGENTS.md)：不返回 403，以免探测人设是否存在） |
| `DELETE /schedules/:id` | `id = ? AND user_id = ?` | 本表**自己就是归属载体**，不需要经父表两层；未命中 → `4040` |
| `POST /schedules/:id/trigger` | ⚠️ **见下方"触发链路是例外"** | 未命中 → `4040`；生成失败 → `5001` |

**本表与 `moment_likes` 的差别**（[moment-like §1.4](../moment-like/spec.md) 那条"归属要走三层"的规约**不适用**于本表的读/取消路径）：
`moment_likes` 的 `user_id` 只证明"这行是你点的"，归属还得经 `ai_moments → personas` 两层；
`schedules` 的 `user_id` **就是所有权本身**（行直接属于用户 + 人设），所以单列即可判定。

#### ⚠️ 触发链路是例外：这条**写**路径必须校验 `persona` 归属

`POST /schedules/:id/trigger` 不是"读本表"，而是**以本表为跳板往另一个表写**：
提醒触发要**注入 `[nudge]` 并生成一条 `chat_messages` 行**（[TECH_DESIGN §5.7 流程 5](../../TECH_DESIGN.md#57-日程提醒p1)）。
写入的那行带 **`(user_id, persona_id)` 两列**——若只按 `schedules.user_id` 放行，
一行 `user_id = A` 而 `persona_id` 属于 B 的日程，会让触发器**把消息写进 B 的对话里**。

**为什么不能靠"写入时不会写坏"当防线**（同 [moment-like §1.4](../moment-like/spec.md) 的论证）：
本表上**没有任何 schema 约束**把 `schedules.user_id` 绑到 `personas.user_id`；
定时任务、补偿脚本、一次数据修复都能造出这种行。防线不能建立在"别的分支不会写坏"之上。

**结论（登记给 `feature/backend-schedule-api`，§5 分组 E）**：
触发路径的归属校验**必须覆盖 `persona`**——`JOIN personas p ON p.id = s.persona_id WHERE s.id = ? AND s.user_id = ? AND p.user_id = ?`，
或复用 `TriggerNow` 内部已有的那次人设归属校验（**不许**在调用前"假定"它做过）。
**列表与取消走直接过滤，触发走联表**——两种形态并存是**有意的**，不是不一致。

#### 错误码口径（两处待群里对齐，别在代码里私自定）

契约 §10 的端点行与正文共给出三个码，其中两处与 §2 总表**不完全一致**，登记如下（**本支不改契约**）：

| 场景 | §10 端点行 / 正文 | §2 总表（第 58–59 行） | 处理 |
|------|------------------|----------------------|------|
| 缺 `personaId` | `4001`（§10 正文："缺失返回 `4001`"） | `4002` = 必填参数缺失 | **待群里对齐**；`errcode` 里两个码都存在，API 分支按对齐结果落，**不要自己挑一个** |
| 访问别人的 `personaId` | `4043` | `4043` = 人设不存在（含资源越权） | 一致，照办 |
| `:id` 不存在 / 不是自己的 | `4040` | `4040` = 资源不存在 | 一致；**两种情况必须同码同文案**，不做区分 |
| 契约变更记录 | 该行仍写着 `4031` | 第 69 行：`4031` **已废弃，保留占位勿复用** | 以**端点行**（`4001 4043`）为准，别把变更记录里的过期码抄进代码 |

> `4030` / `4031` 两个码在日程功能里**一次都不该出现**——资源越权按"不存在"处理是本项目定下的防探测口径。

### 1.3 `remind_at` 的排序必须带 `id` 兜底（要求 6 的定案）

**定案写法**（`GET /schedules`，以及任何按时间取日程的查询）：

```sql
ORDER BY remind_at ASC, id DESC
```

- 契约 §10 **没有规定排序**（端点行只有 `personaId` + 分页），所以这是**本 spec 定的**——
  API 分支与前端**必须照此执行**，前端**不得**在自己那边重排（两边各排一次必然对不上）。
- **"除 `schedules_id_seq`"式的半套验收在这条上同样成立**：只写 `ORDER BY remind_at` 在功能上"看起来对"，
  只有同值行才会暴露，而**同值行在本表是常态，不是巧合**（见下）。

**为什么本表比别处更容易撞车**：`remind_at` **不是数据库生成的**——它不像 `created_at` 由 `NOW()` 给，
而是**规则解析器的输出**（[TECH_DESIGN §5.7 时间解析表](../../TECH_DESIGN.md#57-日程提醒p1)）。两个来源：

| # | 机制 | 例子 |
|---|------|------|
| ① | 相对表达按**解析时刻**计算，精度到秒/分 | 同一秒里的两次解析 → `NOW() + 30 分钟` 得到**完全相同**的时间戳 |
| ② | 取整点惯例 + 秒级截断 | "明天下午三点""明早九点"——两个不同人设、不同用户的日程会落在**同一个整点** |

> ① 与 ② 是**推理出来的机制**（不是实测读数），但结论不依赖它们中的哪一个：
> 只要 `remind_at` 允许相等，排序就必须确定性。项目里已有的同类判断见
> [user_memory 的 `created_at` 说明](../user-memory/spec.md)（批量写入同一事务 → 时间戳完全相同）
> 与 [ai-moment §9 分组 E](../ai-moment/spec.md) 的"分页稳定：两条 `created_at` 相同时 `pageSize=1` 逐页拉**不重不漏**"。

**一个兜底列同时兜住三件事**：

1. **分页不重不漏**：同 `remind_at` 的行顺序固定，翻页不会重复或跳过；
2. **列表渲染稳定**：同一份数据两次请求顺序一致，前端不会"刷新一下顺序变了"；
3. **到期扫描可分批**：定时任务的扫描（`WHERE status = 'pending' AND remind_at <= NOW()`）若带批量上限（`LIMIT`），
   同样必须带兜底列，否则同刻的一批可能被**重复取**或**长期饿死**（登记给 `feature/backend-schedule-job`）——**本支不含扫描代码**。

**诚实记两条边界**：

- `idx_schedules_due` 的列是 `(status, remind_at)`，**不含 `id`** → 兜底列不是索引覆盖的，同 `remind_at` 的组内会多一次排序。
  **不要为了"用上索引"把兜底列删掉**：正确性优先于这点开销，且这是本 spec 定的写法。
- 兜底列的**方向不承载语义**（`id DESC` 与 `id ASC` 都能保证确定性），定成 `id DESC` 是为与全项目既有写法
  （`created_at DESC, id DESC`）一致。**审查时不要"为了与 ASC 对称"改成 `id ASC`**——改了不算错，
  但会让前端与分页在联调期对不上，且要求里已经定死。

**列表没有状态过滤参数**：契约 §10 的 `GET /schedules` 只有 `personaId` + `page` `pageSize`，
所以"待提醒"与"历史提醒"同列一页，顺序就是上面那一条。**不要自作主张加 `status` 查询参数**——那是改契约。

### 1.4 本表**没有** CHECK 约束（要求 5 的答案）；`status` 的防线在 **Go 侧**

DDL 上 `status` 写的是 `VARCHAR(10) NOT NULL DEFAULT 'pending'   -- pending | sent | cancelled`——
**注释列了取值，但没有 CHECK**。按 [ai-moment §7 已定的判据](../ai-moment/spec.md)：
"**判据永远是 DDL 上有没有，不是加了更安全**"。

| 表.列 | DDL 上有 CHECK 吗 | 本项目的做法 |
|------|:---:|------|
| `chat_messages.role` | **有**（`CHECK (role IN ('user','assistant'))`） | 照抄（`check:` tag，已合并） |
| `moment_comments`（作者二选一） | **有**（具名 `chk_comment_author`） | 照抄（已合并） |
| `user_memory.memory_type` | **没有** | **不加**（已合并，同 [user_memory spec](../user-memory/spec.md)） |
| **`schedules.status`** | **没有** | **不加**（本支） |

**但"数据库不拦"不等于"不设防线"**：本表照 `MemoryType` / `MessageRole` 的手法，
在 Go 侧声明 `type ScheduleStatus string` + 三个常量（§3），写入方只能取常量，拼错在**编译期**暴露。

> ⚠️ **必须分清两层**（这是最容易自我安慰的一处）：
> **常量层**（Go 编译期）**与 `check:` tag 层**（数据库约束）是**两件独立的事**，本列**只有前者**。
> 手工 `INSERT`、脚本、将来的补偿任务**能塞进任意字符串**，数据库不会拦——代价与
> [user_memory 同源](../user-memory/spec.md)，此处不重复展开。
> **绝不能**因为"已经有常量了"就顺手加一条 `check:`——那是改 schema（加约束同样要广播、要能说出 DDL 依据）。

**`status` 的其它 tag 纪律**：

- `default:'pending'` 的**单引号不能省**：GORM 把 `default:` 后面的内容直接交给 dialector 拼进 DDL，
  不带引号会渲染成 `DEFAULT pending`（未加引号的标识符），**建表直接失败**。同一手法见 `user_memory.embedding_status`。
- **值类型，不是指针**：DDL 是 `NOT NULL`，用 `*ScheduleStatus` 反而允许写入 `NULL`。
- ⚠️ **本支是全项目第一处「named string type + `default:` tag」的组合**——既有的 `MemoryType`（有类型无默认值）、
  `MessageRole`（有 `check:` 无默认值）、`EmbeddingStatus`（有默认值但类型是裸 `string`）**都只覆盖了一半**。
  落码后**必须用 DryRun 实测**该组合是否渲染出 `"status" varchar(10) NOT NULL DEFAULT 'pending'`（§2.1）；
  渲染不出就**退回裸 `string` + 常量**并在变更记录里记一笔，**不要**为了保住自定义类型而丢掉 DB 默认值（那会偏离权威 DDL）。

### 1.5 `source_message_id` 必须是指针 + `SET NULL`，且关联字段的三个 tag 段一个都不能省

**两个必须**：

- **指针 `*uint64`**：`SET NULL` 的前提是列可空。值类型会让 `AutoMigrate` 建出 `NOT NULL`，
  删消息时**直接报错**（不是静默失败）——同 [user_memory 的同名列](../user-memory/spec.md)，不重复。
- **关联字段 `SourceMessage` 必须是 `*ChatMessage`（指针）**：值类型会被 GORM 当成"必有行"，
  与 `SET NULL` 的语义冲突，且零值 `ChatMessage` 会被当成一个待保存的实体。

**为什么单独成节：本表是**第二处**出现该形态的地方，而上一处出过真实事故。**
[user_memory 的那次事故](../../../backend/internal/model/user_memory.go)：`SourceMessage` 的 tag 被截断成
`gorm:"...;constraint:OnDelete:SET NULL"`，**`foreignKey` / `references` 两个 key 丢失**，
**`go build` / `go vet` / `gofmt` 和当时全部六条 grep 全过**——tag 是字符串字面量，编译器不看内容。

因此本支的验收**必须包含"该有的 key 是否齐全"**（不是只查"有没有违规"）：

| 检查 | 期望 | 抓什么 |
|------|:---:|------|
| `grep -c 'foreignKey:' internal/model/schedule.go` | **3** | 任一关联字段的 tag 被截断 |
| `grep -c 'references:' internal/model/schedule.go` | **3** | 同上 |
| `grep -c 'constraint:OnDelete:CASCADE'` | **2** | 少一条 CASCADE，或多算进 `SET NULL` 那条 |
| `grep -c 'constraint:OnDelete:SET NULL'` | **1** | `SET NULL` 被写成 `CASCADE` / `SET_NULL` |
| `\d schedules` | **3 条外键**，`ON DELETE` 依次为 CASCADE / CASCADE / **SET NULL** | 缺外键（静默）或 `SET NULL` 被写成 CASCADE |

> `SET NULL` **中间的空格是安全的**：GORM 按 `;` → `,` → `:` 三级切分后会把冒号后的多段值拼回，
> 拼 DDL 时是 `" ON DELETE " + OnDelete`。**不要写成 `SET_NULL`**（[user_memory 已验证](../../../backend/internal/model/user_memory.go)）。
>
> ⚠️ **两条计数都要在注释**不写**这些 tag key 的前提下才有判别力**——
> 这是 [ai-moment §3.2 定下的惯例](../ai-moment/spec.md)：**被禁值只出现在 spec 与 `.learn/` 里**。
> §3 的定稿注释因此写成"三个 tag 段缺一不可（spec §1.5）"，**不写 key 的字面量**。

### 1.6 注释只写必要的（[AGENTS §5.1](../../../AGENTS.md)）

与 [moment-like §1.6](../moment-like/spec.md) 同一纪律，不重复。
**目标密度：全文件 ≤ 15 行注释**（§3 设计稿计 **11 行**，**落码后复测回填**——设计稿的行数不算读数）。
上限才是要防的东西，为凑行数加注释本身就违反 §5.1。

> **风格冲突沿用既有决策**：既有 model 文件的注释远超 §5.1，**本支不清理它们**；新文件从简。
> 审查时**不以"与既有文件风格不一致"为退回理由**。

---

## 2. 字段对照表（DDL ↔ Go ↔ tag ↔ JSON）

> DDL 原文见 [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl)，**此处不复制**，只做翻译对照。
> **字段声明顺序 = DDL 列顺序**（GORM 按字段顺序渲染列），不要重排。

| DDL 列 | Go 字段 | GORM tag 关键段 | JSON 键 |
|--------|---------|----------------|---------|
| `id BIGSERIAL PRIMARY KEY` | `ID uint64` | `type:bigint;primaryKey;autoIncrement` | `id` |
| `user_id BIGINT NOT NULL REFERENCES users(id)` | `UserID uint64` | `type:bigint;not null`（**无索引**，见下） | **`-`**（契约无 `userId`） |
| `persona_id BIGINT NOT NULL REFERENCES personas(id)` | `PersonaID uint64` | `type:bigint;not null;index:idx_schedules_persona` | `personaId` |
| `content TEXT NOT NULL` | `Content string` | `type:text;not null` | `content` |
| `remind_at TIMESTAMPTZ NOT NULL` | `RemindAt time.Time` | `type:timestamptz;not null;index:idx_schedules_due,priority:2` | `remindAt` |
| `status VARCHAR(10) NOT NULL DEFAULT 'pending'` | `Status ScheduleStatus` | `type:varchar(10);not null;default:'pending';index:idx_schedules_due,priority:1` | `status` |
| `source_message_id BIGINT REFERENCES chat_messages(id)` | `SourceMessageID *uint64` | `type:bigint`（**指针 + 无索引**） | **`-`**（契约无此字段） |
| `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()` | `CreatedAt time.Time` | `type:timestamptz;not null;default:now();autoCreateTime` | `createdAt` |

**五条翻译纪律**：

- **只有 `ID` 带 `autoIncrement`，三个外键列都不带** → 直接决定 §1.1 能不能过。
  **三条都要查**，漏查一条就漏一个 `nextval`。
- **JSON 键的判据是"契约的 `Schedule` 实体里有没有这个键"**，不是"看别的表怎么写"——
  仓库里**两种写法都有**：`moment_like.go` 的 `UserID` 是 `json:"userId"`（契约要它），
  `user_memory.go` 的 `UserID` 是 `json:"-"`（契约不要它）。**照抄哪一张都可能错**。
  契约 §10 的 `Schedule` 恰好 **6 个键**：`id` `personaId` `content` `remindAt` `status` `createdAt`
  → `UserID` 与 `SourceMessageID` **都是 `json:"-"`**。
- **`user_id` 上没有索引，这是照 DDL 的结果，不要"顺手加一个"**：
  DDL 只给了两个索引，`user_id` **不在其中**。列表查询走 `idx_schedules_persona` 拿到该人设的行之后
  再做行内过滤（一个人设下的日程量级很小）。加索引属于改 schema（先广播、拿依据）。
- **索引恰好 2 个具名对象，列序不能反**：
  - `idx_schedules_persona` —— `(persona_id)`
  - `idx_schedules_due` —— **`(status, remind_at)`**，`status` 在前。
    扫描固定为 `WHERE status = 'pending' AND remind_at <= NOW()`，**列序反转后这个索引对扫描就无用了**。
- **字段顺序与索引列序方向相反是正常的**：struct 里 `RemindAt` 声明在 `Status` 之前（照 DDL 列序），
  而 `idx_schedules_due` 里 `status` 在前（`priority:1` 在 `Status` 上）。**这不是笔误**，
  两处各自对齐各自的权威顺序（DDL 列序 / DDL 索引列序）。

### 2.1 预期渲染结果（**落码后实测回填，禁止把预期当读数**）

```sql
CREATE TABLE "schedules" (
  "id"                bigserial,
  "user_id"           bigint NOT NULL,        -- ← 无 DEFAULT：§1.1 头号验收项之一
  "persona_id"        bigint NOT NULL,        -- ← 无 DEFAULT：另一个
  "content"           text NOT NULL,
  "remind_at"         timestamptz NOT NULL,
  "status"            varchar(10) NOT NULL DEFAULT 'pending',
  "source_message_id" bigint,                 -- ← 无 DEFAULT 且可空（§1.1 第三个 + §1.5）
  "created_at"        timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "fk_schedules_user"           FOREIGN KEY ("user_id")           REFERENCES "users"("id")         ON DELETE CASCADE,
  CONSTRAINT "fk_schedules_persona"        FOREIGN KEY ("persona_id")        REFERENCES "personas"("id")      ON DELETE CASCADE,
  CONSTRAINT "fk_schedules_source_message" FOREIGN KEY ("source_message_id") REFERENCES "chat_messages"("id") ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS "idx_schedules_persona" ON "schedules" ("persona_id");
CREATE INDEX IF NOT EXISTS "idx_schedules_due"     ON "schedules" ("status","remind_at");
```

**落码后要回填的读数**（现在全部是 **⬜ 待跑**，写法与 [moment-like §2.1](../moment-like/spec.md) 一致：
把 §3 的代码本身放进 `package model`，`go build` 过之后用 `gorm.Config{DryRun: true}` 捕获语句——**不连库、不改表**）：

| 检查 | 命令 / 方法 | 期望读数 |
|------|------------|:---:|
| §1.1 头号项 | `schema.Parse` → `UserID` / `PersonaID` / `SourceMessageID` 的 `DataType` / `HasDefault` / `AutoIncrement` | `bigint` / **false** / **false** ×3 |
| （对照） | 同上 `ID` | `bigint` / `true` / `true`（正常） |
| §1.4 新组合 | DryRun 渲染出的 `status` 列 | `varchar(10) NOT NULL DEFAULT 'pending'` |
| 分组 A | `schema.ParseIndexes()` | **2** 个：`idx_schedules_due` / `idx_schedules_persona` |
| 分组 A | `grep -c 'bigserial'` | **0** |
| 分组 A | `grep -c 'gorm:"[^"]*autoIncrement'` | **1**（只在 `ID`；`UserID` 的注释里有这个词，所以必须限定在 `gorm:"` 内） |
| 分组 A | `grep -c 'gorm:"[^"]*check:'` | **0**（§1.4） |
| 分组 A/C | `grep -c 'foreignKey:'` / `'references:'` | **3 / 3**（§1.5） |
| 分组 C | 序列化键集 | **恰好 6**：`id` `personaId` `content` `remindAt` `status` `createdAt` |
| 分组 C | `grep -c 'json:"userId"\|json:"sourceMessageId"'` | **0**（§2 第二条纪律） |
| 分组 D | 注释行数 | **11**（≤ 15，§1.6） |
| — | `go build ./... && go vet ./...` | 通过 |

**设计稿层面的静态读数（2026-09-19 已跑，非推断）**：

> ⚠️ **口径要说清楚**：下表的读数是拿 **§3 的代码块原文**（抽成临时文件、**未落进仓库**）跑出来的，
> 验的是**设计稿的静态自洽**——它证明这几条 grep 在正确写法下取值是多少、**不是**仓库文件的读数，也**不是**真库读数。
> 落码后**必须对 `internal/model/schedule.go` 本体重测**（步骤 3/4），真库读数仍全部 ⬜ 待跑。

| 检查 | 期望 | 设计稿读数 |
|------|:---:|:---:|
| 注释行数（§1.6） | ≤ 15 | **11** ✓ |
| `grep -c 'bigserial'` | 0 | **0** ✓ |
| `grep -c 'gorm:"[^"]*autoIncrement'` | 1 | **1** ✓ |
| **裸** `grep -c 'autoIncrement'` | — | **2** ← **"必须限定在 `gorm:"` 内"的实证**：多出来的 1 次来自注释 |
| `grep -c 'gorm:"[^"]*check:'` | 0 | **0** ✓ |
| `grep -c 'foreignKey:'` / `'references:'` | 3 / 3 | **3 / 3** ✓（§1.5） |
| `grep -c 'constraint:OnDelete:CASCADE'` | 2 | **2** ✓ |
| `grep -c 'constraint:OnDelete:SET NULL'` | 1 | **1** ✓ |
| `grep -c 'json:"userId"\|json:"sourceMessageId"'` | 0 | **0** ✓ |
| 字段级探针（PersonaName 等 5 个） | 0 | **0** ✓ |

> 裸 grep 那行（2 ≠ 1）就是**为什么分组 A 那条必须写成 `gorm:"[^"]*autoIncrement`**：
> 本表的注释按 §5.1 描述了"外键列不能带自增标记"这件事，裸 grep 会把注释算进去。
> 这与 [moment-like 的读法 1](../moment-like/spec.md) 是同一条纪律——**不是本表写错了注释，而是 grep 要限定范围**。

**三个真库口径别混用**（差 1 的来源是主键）：

| 口径 | 命令 | 期望 |
|------|------|:---:|
| GORM 声明 | `schema.ParseIndexes()` | **2** |
| 真库全部索引 | `SELECT indexname FROM pg_indexes WHERE tablename='schedules'` | **3**（多一个 `schedules_pkey`） |
| 真库序列 | `SELECT sequencename FROM pg_sequences WHERE schemaname='public'` | `schedules_id_seq` **恰好 1** |

---

## 3. 定稿代码（**逐字对照，不要临场发挥**）

`backend/internal/model/schedule.go`：

```go
package model

import "time"

// ScheduleStatus 日程状态：pending 待提醒 ｜ sent 已提醒 ｜ cancelled 已取消。
// 与 MemoryType 同一手法；本列数据库侧没有 CHECK 约束（spec §1.4），常量是它唯一的防线。
type ScheduleStatus string

const (
	ScheduleStatusPending   ScheduleStatus = "pending"
	ScheduleStatusSent      ScheduleStatus = "sent"
	ScheduleStatusCancelled ScheduleStatus = "cancelled"
)

// Schedule 日程提醒，对应数据库表 schedules。一个人设一份，删账号 / 删人设时级联删除。
type Schedule struct {
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

	// 越权防线载体，契约 §10 的 Schedule 没有 userId，不进响应体。
	// ⛔ 外键列，绝不能带 autoIncrement：带上会让本列长出数据库自增默认值（spec §1.1）。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null" json:"-"`

	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_schedules_persona" json:"personaId"`

	Content string `gorm:"column:content;type:text;not null" json:"content"`

	// 到期扫描是 WHERE status='pending' AND remind_at <= NOW()，故组合索引 status 在前。
	// ⚠️ 取日程必须写成 ORDER BY remind_at ASC, id DESC：兜底列钉死同值顺序（spec §1.3）。
	RemindAt time.Time `gorm:"column:remind_at;type:timestamptz;not null;index:idx_schedules_due,priority:2" json:"remindAt"`

	Status ScheduleStatus `gorm:"column:status;type:varchar(10);not null;default:'pending';index:idx_schedules_due,priority:1" json:"status"`

	// 指针是 SET NULL 的前提（值类型会建出 NOT NULL，删消息时直接报错）；溯源断链不删日程本身。
	SourceMessageID *uint64 `gorm:"column:source_message_id;type:bigint" json:"-"`

	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null;default:now();autoCreateTime" json:"createdAt"`

	// 仅供 GORM 生成外键约束用，不参与序列化，Create 时不要赋值（否则会连带写 users / personas / chat_messages）。
	// 三个关联字段缺一不可，缺哪个就没有哪个 ON DELETE（spec §1.0 / §1.5）。
	User          User         `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Persona       Persona      `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	SourceMessage *ChatMessage `gorm:"foreignKey:SourceMessageID;references:ID;constraint:OnDelete:SET NULL" json:"-"`
}

// TableName 显式指定表名。
func (Schedule) TableName() string { return "schedules" }
```

`backend/internal/model/migrate.go`——**只加一行**，位置在 `&MomentLike{}` 之后（**全表最后一行**）：

```go
			&Schedule{},         // 引用 users(id) / personas(id) / chat_messages(id)，三张表都要先建好
```

> 顺序理由与 `migrate.go` 顶部注释一致：被引用的表必须排在前面。
> 本行是**第 10 行**——`AutoMigrate` 的列表从此与 [TECH_DESIGN §6.4](../../TECH_DESIGN.md#64-建表方式gorm-automigrate) 的十张表完全对齐。

> **`status` 用常量而不是裸字符串**：写入方取 `model.ScheduleStatusPending` / `Sent` / `Cancelled`，
> 拼错在编译期报错（数据库**不会**拦，§1.4）。**不要**为了"顺手"把裸字符串 `"pending"` 写进 repo——
> 那正是常量存在的理由。

---

## 4. 写入路径（**预写，本支不实现**，由 `feature/backend-schedule-api` / `-job` 落地）

### 4.1 四条路径的形态（每条一句话，细节归下游 spec）

| # | 路径 | 形态要点 |
|---|------|---------|
| 1 | **落库**（解析成功后） | `status` 取 `ScheduleStatusPending`；`user_id` 从 Token 取（**前端永不传**）；`persona_id` 必须**校验归属**——把归属条件焊进写入语句（`INSERT ... SELECT ... FROM personas WHERE id = ? AND user_id = ?`，与 [ai-moment §4.4③](../ai-moment/spec.md) 同一手法），"先查后插"漏掉那个 `if` 会**静默越权**。**没有 `POST /schedules`**（[AGENTS 红线 5](../../../AGENTS.md)） |
| 2 | **列表** `GET /schedules` | `WHERE persona_id = ? AND user_id = ?`；`ORDER BY remind_at ASC, id DESC`（§1.3）；分页 `page` 从 1、`pageSize` 默认 20 上限 100；**空列表返回 `[]` 不是 `null`**（Go 的 nil slice 会序列化成 `null`，前端 `v-for` 直接崩——[user_memory spec 已定](../user-memory/spec.md)） |
| 3 | **取消** `DELETE /schedules/:id` | `UPDATE schedules SET status = ? WHERE id = ? AND user_id = ?`（值传 `ScheduleStatusCancelled`，**不用 `Save()` 写回整行**）；`RowsAffected = 0` → `4040`；重复取消 → `200` **幂等**（契约 §10）。⚠️ 用 `Updates` 时值必须在**结构体非零**或用 `map`（[proactive_setting.go 的 ⛔ 注释](../../../backend/internal/model/proactive_setting.go) 记过这条静默空操作） |
| 4 | **触发** `POST /schedules/:id/trigger` | **必须复用定时任务的同一段 `TriggerNow` 逻辑**（[TECH_DESIGN §5.7 末段](../../TECH_DESIGN.md#57-日程提醒p1)：既是演示硬性依赖，也是"不是演示专用代码"的纪律）；归属校验**必须覆盖 `persona`**（§1.2 例外）；成功后 `status = 'sent'` 并返回 `{messageId, content, createdAt}`——那是**新建的 chat 消息**，生成失败 → `5001` |

### 4.2 一处未定义行为（**待群里对齐，不要在代码里私自定**）

| 场景 | 契约原文 | 状态 |
|------|---------|------|
| 对一条 `status = 'sent'` 的日程再 `DELETE` | 契约只写了"把 `status` 置为 `cancelled`；重复删除同一条是**幂等**的" | **未定义**：是否允许把"已提醒"改回"已取消"？**待对齐**。下游落地前先问，别自己挑一种（选错会让"历史提醒"列表变得可被抹掉） |

### 4.3 本表反例清单（下游 AI 最容易写出来的错法）

| # | 错法 | 后果 | 归属 |
|---|------|------|------|
| 1 | 列表只 `WHERE persona_id = ?` | **跨用户**读到别人的日程（[红线 3](../../../AGENTS.md)） | API 分支 |
| 2 | 列表只 `WHERE user_id = ?` | **跨人设**串号（同账号，但语义错） | API 分支 |
| 3 | `ORDER BY remind_at` 不带 `id` 兜底 | 同值行顺序不定 → 翻页**重复或漏项**（§1.3） | API 分支 |
| 4 | 取消 / 触发只按 `id` 不带 `user_id` | 凭全局自增 id **顺序试号**即可操作别人的日程 | API 分支 |
| 5 | 触发不校验 `persona` 归属 | **往别人的对话里写消息**（§1.2 例外，本表独有） | API 分支 |
| 6 | 取消用 `Save()` / `Updates(model.Schedule{Status:...})` | `Save()` 覆盖整行；`Updates` 传零值**静默空操作**（`err = nil`、`RowsAffected = 0`） | API 分支 |
| 7 | `status` 写裸字符串、或写第四态（如 `failed` / `read`） | 拼错**静默落库**（§1.4 数据库没有 CHECK 兜底）；第四态与 DDL 三态冲突（§1.5 / [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl)） | API 分支 |
| 8 | 新增 `POST /schedules` 或给列表加 `status` 参数 | 改契约（[红线 5](../../../AGENTS.md) / §1.3） | API 分支 |
| 9 | 触发端点在 `INSERT chat_messages` 时把 `user_id` / `persona_id` 取错来源 | 写进别人的对话 → 越权（与 #5 同源，但更隐蔽：SQL 本身"没错"） | API 分支 |
| 10 | 扫描任务（`schedule_job`）带 `LIMIT` 却不带兜底列 | 同刻一批被重复取 / 长期饿死（§1.3 第 3 条） | job 分支 |
| 11 | 把 `source_message_id` 写成 CASCADE，或把 `SourceMessageID` 改成值类型 | 删消息删日程；或建成 `NOT NULL` 后删消息**报错**（§1.5） | 本支（静态）+ API 分支（行为） |
| 12 | 给 model 加 `PersonaName` / `LastNudgeAt` / `EmotionLabel` 之类的派生字段 | `AutoMigrate` 在**本表建出真列**（[ai-moment §3.5](../ai-moment/spec.md) 的派生字段陷阱，静默） | 本支（§5 分组 C 钉死） |

---

## 5. 验收标准

> **正反两面都写**（沿用 [ai-moment §9](../ai-moment/spec.md) 的既定格式）：
> 每条"不许有什么"都配一条"必须有什么"，否则"写漏"类缺陷会全部静默通过。

### 分组 A · 建表与结构（**本支，必须全绿**）

- [ ] `AutoMigrate` 追加 `&Schedule{}` 后，`\d schedules` 与 [TECH_DESIGN §6.2](../../TECH_DESIGN.md#62-核心表-ddl) **逐列一致**
  - [ ] **必须恰好 8 列**：`id` `user_id` `persona_id` `content` `remind_at` `status` `source_message_id` `created_at`
  - [ ] **多出的列必须是零**（§4.3 反例 12 的派生字段陷阱）
  - [ ] **列宽/类型逐个看**，不能只看列名：三个 id 列 + `source_message_id` 都是 `bigint`，
        `content` 是 `text`，`status` 是 `character varying(10)`，`remind_at` / `created_at` 是 `timestamp with time zone`
  - [ ] **`user_id` / `persona_id` / `source_message_id` 三列 Default 都必须为空**（§1.1，**头号验收项**）
  - [ ] `user_id` / `persona_id` **不是可空列**；`source_message_id` **是**可空列（§1.5）
  - [ ] `status` 的 Default 是 **`'pending'`**（`\d` 里带引号显示）
- [ ] **索引恰好 2 个具名对象**（口径见 §2.1）：
  - [ ] `idx_schedules_persona` —— `(persona_id)`
  - [ ] `idx_schedules_due` —— **`(status, remind_at)`**，`status` 在前
  - [ ] **多出的索引是零**——特别是**没有** `idx_schedules_user`（§2 第三条纪律：DDL 上没有）
- [ ] **序列（`\ds` / `pg_sequences`）——正反两面**：
  - [ ] 搜 `schedules_user_id_seq` / `schedules_persona_id_seq` / `schedules_source_message_id_seq` → **各 0 结果**（§1.1）
  - [ ] `schedules_id_seq` **恰好 1 个**（来自 `id` 的 `autoIncrement`）
  - [ ] **全库收口（本支第一次跑得动）**：`pg_sequences` 里 `*_id_seq` **恰好 10 个**，
        名单与 §1.1 列出的十个逐字一致；且**没有任何** `<表名>_<外键列名>_seq`
        （同时关闭 [moment-like §5 分组 E](../moment-like/spec.md) 的遗留项）
- [ ] **三条外键全部存在**，`ON DELETE` 依次是 **CASCADE / CASCADE / SET NULL**（§1.5；`\d` 里逐条看）
- [ ] **tag 静态检查**（grep 一律针对 `internal/model/schedule.go`）：
  - [ ] `grep -n "bigserial"` → **零命中**
  - [ ] `grep -c 'gorm:"[^"]*autoIncrement'` → **恰好 1 行**（只在 `ID`）
        > 必须限定在 `gorm:"..."` 内：`UserID` 的注释在描述"不带 autoIncrement"，裸 grep 会误命中。
  - [ ] `grep -c 'gorm:"[^"]*check:'` → **0**（§1.4）
  - [ ] `grep -c 'foreignKey:'` → **3**、`grep -c 'references:'` → **3**（§1.5 的 tag 截断检查）
  - [ ] `grep -c 'constraint:OnDelete:CASCADE'` → **2**、`grep -c 'constraint:OnDelete:SET NULL'` → **1**
  - [ ] `grep -c 'json:"userId"\|json:"sourceMessageId"'` → **0**（§2 第二条纪律）
- [ ] `go build ./... && go vet ./... && go test ./...` 全过

### 分组 B · 级联与断链（**本支，可跑**）

> 三条外键的三种行为各验**正反两面**——"清空了"可以被"把全表清了"蒙混过关，所以每一步都要留一条**不该被动的行**。

- [ ] **删账号（CASCADE）**：A、B 两账号各有人设与日程 → 删 A → A 的日程清零，**B 的日程仍在**
- [ ] **删人设（CASCADE）**：同一账号两个人设各带日程 → 删其一 → 该人设日程清零，**另一人设的仍在**
- [ ] **删消息（SET NULL）**：一条日程带 `source_message_id` → 删那条 `chat_messages` 行 →
      **日程行还在**，且该列变成 `NULL`（不是 `0`，不是级联删除）
- [ ] **反向验证**：删消息**没有**触发任何日程删除（`SELECT count(*) FROM schedules` 前后一致）

### 分组 C · 序列化与静态探针（**本支**）

> 本表**有**契约实体（`Schedule`，契约 §10），所以这条测试是**契约校验**——
> 与 [moment_likes（无实体、纯探针）](../moment-like/spec.md) 性质不同，**但键集必须"恰好相等"这一点相同**。

- [ ] **第一条：产物检查**（`TestScheduleJSONKeys`）——`internal/model/schedule_test.go`：marshal 一个 `Schedule`，断言键集**恰好**是
      `{id, personaId, content, remindAt, status, createdAt}`（**6 个**，与契约 §10 逐字一致）
  - [ ] 键集里**没有** `userId` / `sourceMessageId`（§2 第二条纪律，本表最容易抄错的两个）
  - [ ] 键集里**没有** `personaName` / `lastNudgeAt` / 任何 `emotion` 字样（[AGENTS §4.4](../../../AGENTS.md)）
- [ ] **字段级静态检查**（不依赖注释措辞）：
      `grep -cE '^[[:space:]]+(PersonaName|LastNudgeAt|EmotionLabel|EmotionScore|Username)[[:space:]]' internal/model/schedule.go` → **0**
      > 它匹配的是 **struct 字段声明行**，注释整行以 `//` 开头、不会命中。
      > 与键集断言是**两道独立的闸门**：键集验"序列化出来多了什么"，这条验"struct 里多了什么"——
      > 后者才是 AutoMigrate 建列的直接原因。
- [ ] **第二条：tag 声明级检查**（`TestScheduleJSONTags`，**必须与上面那条同时存在**，缺一不可）：
      遍历 `reflect.TypeFor[Schedule]().Fields()`，把每个字段的 `json` tag 名收成 `map[json 名][]Go 字段名`，双向校验：
  - [ ] **名字集合 == 契约集合**（不多不少，与键集断言同一份数据）
  - [ ] **没有任何两个字段共用同一个 json 名**
  - [ ] 契约键集在测试文件里**只写一份**（包级 `scheduleJSONKeys`，两条测试共用；两处各写一份必然漂移）
  > **为什么产物检查不够**（本支实测撞出来的，不是设计的）：`encoding/json` 遇到**同名**字段会
  > **静默丢弃全部、且不报错**。把 `UserID` 的 `json:"-"` 改成 `json:"userId"`、**同时**把关联字段
  > `User` 也写成 `json:"userId"` 时，序列化输出与合规时**逐字节一致**（六个键一个不少），
  > 上面那条 marshal 断言**照样绿**——它看的是产物，而产物恰恰是"被丢掉之后"的样子。
  > 这是**第三道闸门**：键集验"序列化出来多了什么"，字段级 grep 验"struct 里多了什么"，
  > 这条验"**tag 声明了什么**"。三者都会瞎的地方不同，所以三条都要在。
  > 同族事故见 [user_memory 的 tag 截断](../../../backend/internal/model/user_memory.go)（tag 是字符串字面量，编译器和 `vet` 都不看内容）。
- [ ] **反向验证（注入缺陷，确认检查会报红）**：

  | # | 注入的缺陷 | 期望哪条报红 | 实际读数 |
  |---|-----------|-------------|---------|
  | 1 | `UserID` 的 tag 加 `autoIncrement` | 分组 A 的 `autoIncrement` 恰好 1 + `\ds` 的 `schedules_user_id_seq` | ⬜ 待跑 |
  | 2 | `PersonaID` 的 tag 加 `autoIncrement` | 同上（**必须与 1 分开跑**） | ⬜ 待跑 |
  | 3 | `SourceMessageID` 的 tag 加 `autoIncrement` | 同上（**本表有三个外键列，三条要各跑一遍**） | ⬜ 待跑 |
  | 4 | `SourceMessage` 的 tag 截断成只剩 `constraint:OnDelete:SET NULL`（**重放 user_memory 事故**） | 分组 A 的 `foreignKey:` / `references:` 恰好 3 + `\d` 缺该外键 | ⬜ 待跑 |
  | 5 | `SourceMessageID` 改成值类型 `uint64` | `\d` 里该列变 `NOT NULL` + 分组 B 的"删消息"报错 | ⬜ 待跑 |
  | 6 | `SourceMessage` 的 `OnDelete` 改成 `CASCADE` | 分组 B 的"日程行还在" | ⬜ 待跑 |
  | 7 | 给 `Status` 加 `check:status IN (...)` | 分组 A 的 `check:` 零命中 | ⬜ 待跑 |
  | 8 | `Status` 的 `default:'pending'` 去掉单引号 | 建表失败（§1.4） | ⬜ 待跑 |
  | 9 | `idx_schedules_due` 两处 `priority` 对调 | `\d` 里列序变 `(remind_at, status)` → 扫描用不上索引 | ⬜ 待跑 |
  | 10 | 给 `UserID` 加 `index:idx_schedules_user` | 分组 A 的"索引恰好 2 个" | ⬜ 待跑 |
  | 11 | 给 struct 加 `PersonaName string \`json:"personaName"\`` | 分组 A 的"恰好 8 列" + 分组 C 的**字段级 grep** + 键集 | 分组 C **两条测试均报红** ✅（2026-09-19，口径 `go test ./internal/model/ -run TestScheduleJSON`，非真库）；分组 A 的"恰好 8 列" ⬜ 待真库复跑 |
  | 12 | 删掉 `User User` 关联字段 | 分组 A 的 `user_id → users` 外键消失 | ⬜ 待跑 |
  | 13 | `ID` 的 tag 改成 `type:bigserial` | 分组 A 的 `bigserial` 零命中 | ⬜ 待跑 |
  | 14 | `UserID` 与关联字段 `User` **同时**写成 `json:"userId"`（**撞名**） | **只有** tag 声明级检查 `TestScheduleJSONTags`；marshal 键集断言**看不见**（见下） | ✅ 已实测：声明检查**红**、产物检查**绿**（2026-09-19，口径 `go test ./internal/model/ -run TestScheduleJSON`，非真库）——**本条是实测中撞出来的，不是设计出来的** |

  **任何一条注入后检查仍然"通过"，说明那条验收是假的**——先修验收，再改代码。读数连同日期提交。

  > **注入 1/2/3 必须分开跑**：本表有**三个**外键列（全项目第三张，前两张是 `user_memory` / `moment_comments`），
  > 只测其中一个会漏掉另外两个的 `nextval`。
  > **注入 4 是本支唯一"重放真实事故"的一条**：它不是假想，[user_memory 真的这么坏过一次](../../../backend/internal/model/user_memory.go)，
  > 而当时的六条 grep 全是"查有没有违规"，没有一条查"该有的 key 是否齐全"——本支的 §1.5 就是补的那一半。

  > **注入 14 是"验收本身有盲区"的实证，而它不在原始设计里**：本条是**做注入 11 时意外撞出来的**——
  > 第一次的替换命令没加锚点，把 struct 里**六处** `json:"-"` 一起改成了同一个名字，等于自己制造出撞名，
  > 产物测试于是"该红而红不了"。逐条重做后才发现：初稿的 marshal 断言在注入 1/2/11 上都正常报红，
  > 它**不是假标准，是覆盖不全**——对"字段被静默丢弃"这一类完全瞎。据此才补了 tag 声明级检查。
  > **过程教训**：反向验证必须**逐条注入**。一次改多个字段会自己制造撞名，
  > 把"验收失效"掩盖成"我在故意测盲区"，两者在现象上无法区分（都是"红了"或都是"绿了"）。

### 分组 D · 注释与流程（**本支**）

- [ ] `schedule.go` 注释 **≤ 15 行**（§1.6）——设计稿计 **11 行**，落码后复测回填
- [ ] 注释里**不出现** `foreignKey` / `references` / `check` 这类**被字面 grep 计数**的 tag key（§1.5 惯例）
      > `autoIncrement` 是**例外**：分组 A 那条 grep 已限定在 `gorm:"` 内，注释里描述它可以照写
      > （§3 的定稿注释里就有，与 [moment-like 的处理](../moment-like/spec.md) 一致）。
      > 判据是"**注释会不会命中同一条 grep**"，不是"这个词能不能出现"。
- [ ] 无 DDL 逐字对照 / 长篇学习性解释 / 反例分析（内容已挪到 `.learn/`）
- [ ] `.learn/schedule-model.md` 已写；`git status` 里**不出现**它（`.gitignore:75` 已覆盖）
- [ ] 未改 `router.go`、`go.mod`、他人的 model 文件
- [ ] `git diff --name-only` 只含本支 4 个文件（含 `.learn/` 则为异常）
- [ ] commit 符合 `<type>(<scope>): <subject>`，scope 用 `schedule`；PR 已开、至少 1 人 Approve

### 分组 E · 交接给下游的验收（**不在本支**，登记以免断档）

**给 `feature/backend-schedule-api`**（§1.2 / §1.3 / §4）：

- [ ] **列表必带两个条件**：A、B 两账号各有人设与日程
  - [ ] A 的 Token + **B 的 `personaId`** → `4043`（不是 403、不是空列表）
  - [ ] 缺 `personaId` → 按 §1.2 对齐结果（`4001` 或 `4002`）——**对齐之前先问**
  - [ ] A 自己的两个人设各带日程 → 各自列表**只出现自己的**（跨人设不串号）
- [ ] **`:id` 端点同码同文案**：`DELETE` / `trigger` 打 A 自己的、别人的、不存在的 id →
      别人的与不存在的**都是 `4040`**，且文案逐字相同（不做区分，防探测）
- [ ] **取消幂等**：同一条连删 3 次 → 每次 `200`，库里该行 **只有 1 行**且 `status = 'cancelled'`（不物理删）
- [ ] **排序与分页**：造 **3 条 `remind_at` 完全相同**的日程 → `pageSize=1` 逐页拉 3 页 →
      **不重不漏**，且三次请求顺序一致（§1.3）
- [ ] **触发复用**：`POST /schedules/:id/trigger` 与到期扫描走**同一段 `TriggerNow`**；
      触发后 `status = 'sent'`，且 `chat_messages` 里确实多了一条该人设的消息（响应里的 `messageId` 能在库里查到）
- [ ] **越权触发（本表独有）**：手工造一行 `user_id = A` 而 `persona_id` 属于 B 的日程
      → A 触发它**必须失败**（`4040`），且在 B 的对话里**没有**新增消息（§1.2 例外）
- [ ] **不产生 `4030` / `4031`**：任意输入组合下只出现 `200` / `4001`(或 `4002`) / `4010` / `4040` / `4043` / `5001`
- [ ] **没有 `POST /schedules`**（`grep` 零命中，[红线 5](../../../AGENTS.md)）
- [ ] 空列表返回 `[]` 而不是 `null`

**给 `feature/backend-schedule-job`**：

- [ ] 扫描 `WHERE status = 'pending' AND remind_at <= NOW()` 走 `idx_schedules_due`；
      若带批量上限，则排序带 `id` 兜底（§1.3）

**给前端分支**：

- [ ] 列表**不在前端重排**（顺序由后端那一条 `ORDER BY` 决定，§1.3）；`status` 三态渲染，不出现第四态

**给 `ai-service`（时间解析器）**：

- [ ] 解析器**不碰数据库**（[TECH_DESIGN §5.7 流程 3](../../TECH_DESIGN.md#57-日程提醒p1)），只返回 `{content, remind_at}` 或 `{need_clarify: true}`；
      **没有独立提醒意图的陈述句不建日程**、解析不出**必须回问澄清**（契约 §10 "未实现的兜底行为"）

---

## 6. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|------|------|------|------|
| 2026-09-19 | v1 | 创建 | 日程提醒模型开工前的设计与验收基线。**§1.0 记下"要求只提 `persona_id`、DDL 有三条外键且其中一条是 `SET NULL`"的落差**；§1.1 把要求 4 的序列验收补齐**正面项**并扩展到本表**三个外键列**，同时把"全库 10 个序列"的收口挂在本支（它才第一次跑得动）；§1.2 定案走**直接过滤**分支，并单独指出**触发链路必须额外校验 persona 归属**（本表独有）；§1.3 定死 `remind_at ASC, id DESC` 并说明本表同值撞车为何是**结构性**的；§1.4 定案**不加 CHECK**并把 `status` 的防线放到 Go 常量；§1.5 把 `source_message_id` 的指针 + `SET NULL` + tag 三段齐全立为独立验收（重放 user_memory 事故） | 设计阶段先证明验收项可触发；两处契约口径不一致（`4001`/`4002`、变更记录里的过期 `4031`）留痕待群里对齐 |
| 2026-09-19 | v1（同日补） | §2.1 新增**设计稿层面的静态读数**表：用 §3 的代码块原文（未落库）跑出 10 项读数，全部与期望一致；其中**裸 `grep -c 'autoIncrement'` = 2** 是"必须限定在 `gorm:"` 内"的实证。§1.1 修正"唯一一张三外键表"的错述（**实为第三张**：`user_memory` / `moment_comments` 在前）；§1.6 与分组 D 把"定稿实测 11 行"改为"设计稿计 11 行、落码后复测"（**设计稿行数不算读数**）；分组 D 的注释禁令补上 `autoIncrement` 例外 | 自查时发现三处"预期被写成读数"与一处事实性错述——按 [AGENTS §5.2](../../../AGENTS.md) 与本项目"假验收"的既有教训当日修正 |
| 2026-09-19 | v1.0.1（落码同步） | §3 定稿注释一处**笔误**："等值类型" → "**值类型**"（与 `schedule.go` 同步；仅注释措辞，tag / 字段 / 行为均未变） | 逐字对照落码时发现；为保住"代码 ↔ §3 逐字一致"这条可审查性，两处同时改为正确措辞 |
| 2026-09-19 | v1.0.2（验收补强） | §5 分组 C 新增**第三道闸门**"tag 声明级检查"（`TestScheduleJSONTags`：tag 名集合 == 契约集合，且无两个字段共用一名），注入表新增**第 14 条**（撞名注入）并回填注入 11 的实测读数 | **反向验证时实测撞出来的真盲区**：`encoding/json` 对同名字段**静默丢弃全部且不报错**，初稿的 marshal 键集断言在该状态下**照样绿**。原 13 条注入里没有一条能覆盖它——验收标准缺了一半，补齐后 14 条 |

# spec · 用户记忆（User Memory）

> 功能名：user-memory ｜ 分支：`feature/backend-user-memory-model`
> 负责人：成员 3（数据 + 人设 + 部署） ｜ 状态：**模型层设计完成，待实现**；接口层待与成员 1 对齐
> 创建：2026-09-15 ｜ 最后更新：2026-09-15（**第 2 版**：并入决策 1-3——分工边界 / 只做模型层验证 / 契约空白只记录，见 §7.3）
> 关联：[AGENTS.md](../../../AGENTS.md) ｜ [接口契约 §7](../../API_CONTRACT.md) ｜ [技术文档 §6.2 / §5.3 / §4.3](../TECH_DESIGN.md) ｜ [总纲 §0](../dev/MASTER.md) ｜ [成员 1 任务书 §4](../dev/MEMBER_1_BACKEND_AI.md)

> ✅ **分工边界已定（2026-09-15 决策 1）**：按 [总纲 §4.2](../dev/MASTER.md) 的端点归属表，**`GET /memory` 的提供者是成员 1**；`internal/model/*` 属成员 3。**本分支只做三件事**：① `internal/model/user_memory.go`；② `internal/model/migrate.go` 追加一行；③ 本 spec / plan 文档。**`repository/` / `service/` / `handler/` / `router.go` 一律等与成员 1 对齐后再动**（避免两人同时改同一文件，AGENTS §4.8）。因此 **§4 / §5 是「交给成员 1 的设计约定」，不是本分支要落的代码**——文档先推进。
> 🚧 **阻塞：接口层端到端验收依赖成员 1 的 `internal/middleware`**（`JWTAuth` 目前是空壳，从不 `Set(ContextKeyUserID)`）。**本分支只做模型层验证**：`AutoMigrate` 跑通、表结构 / 外键 / 索引 / 序列与 DDL 一致，**不做任何接口层测试**（§7.1、§8 分组 A/B）。
> ⚠️ **记忆是只读的**：总纲 §0「记忆管理：只读展示，**不提供删除接口**」。**不要发明 `DELETE /memory/:id`**，也不要为了"测试方便"在仓储层留一个删除方法——见 §4.7。
> ⚠️ **本功能不新增任何错误码**：复用 `4001` / `4043` / `5003` / `4010`（§6）。

---

## 1. 背景与目标

记忆是"像真人一样"的 AI 伴侣的**第一支撑物**：对话链路每轮结束都要用它、画像页（P1）要展示它、主动消息要基于它开口。它挂在 `persona_id` 上——**一个人设一份记忆，换人设就换一套**（总纲 §0.2），因为"记得你的猫叫豆豆"这件事是**这个伴侣**记得，不是"系统"记得。

**目标（一句话）**：交付 `user_memory` 的 GORM 实体、只读仓储与写入方法、`GET /memory` 分页读端点，字段与契约 §7 逐字一致；并且**任何接口都不可能读到别人的记忆，也不可能把 A 人设的记忆串到 B 人设**。

**为什么它是核心演示链路的一环**：演示脚本第 8 项「记忆往返」——对人设 A 说「我养了只猫叫豆豆」→ A 答得出；切到人设 B 问 → 答不出（[总纲 §6](../dev/MASTER.md)）。前半句靠**写入 + 检索注入**，后半句靠**双条件隔离**。前半句错了功能不好看，后半句错了是红线 3（跨用户 / 跨人设泄漏）。

## 2. 范围

### 2.1 做什么（In Scope）

**A. 本分支落地（模型层，不等任何人）**

| 项 | 产物 |
|---|---|
| 数据模型 | `internal/model/user_memory.go`（GORM 实体，含**三个**外键声明——两个 CASCADE + 一个 SET NULL） |
| 建表 | `internal/model/migrate.go` 的 `AutoMigrate` 追加 `&UserMemory{}`（顺序在 `&ChatMessage{}` 之后，见 §3.4） |
| 设计文档 | 本 `spec.md` 与 [plan.md](plan.md) |

**B. 交接给成员 1（本分支不落地，等对齐后再动）**

> 下表是**设计约定**，写在这里是为了让接口层有唯一一份可依据的规格；**本分支不写这些文件**（决策 1）。成员 1 若已在写，直接按本 spec 对齐即可；若要本分支接，先群里对齐。

| 项 | 产物 | 关键约定 |
|---|---|---|
| 通用分页结构 | `internal/dto/common_dto.go`（`PageResult[T]` + 分页常量 + `ClampPage`，**全项目五个分页端点共用**） | persona / chat-message / user-memory **谁先落地谁建，后来者消费，不要写第二份** |
| 请求 / 响应结构 | `internal/dto/memory_dto.go` | `MemoryItemResponse` **恰好 6 字段**；`form:"personaId"`（§4.5） |
| 归属判定 | `internal/repository/persona_repo.go` 增 `ExistsOwnedByUser` | **一条查询、两个条件、只回 bool**（§5.2）；chat-message 分支也声明了它，**同一份** |
| 数据访问（读） | `internal/repository/memory_repo.go`：`ListByPersona`（含 `total`） | 签名强制带 `userID` **与** `personaID`；`Count` 与列表条件逐字相同（§5.1 ②） |
| 数据访问（写） | 同上：`CreateBatch` | **供成员 1 的记忆提取链路落库**，四个交接约定见 §4.6 |
| 业务逻辑 | `internal/service/memory_service.go` | **先判归属、后查数据**；未命中一律 `4043`；分页钳制在此（§5.2） |
| HTTP 接口 | `internal/handler/memory_handler.go` | `GET /memory` + `RegisterMemoryRoutes`；错误 `_ = c.Error(err)` 上抛 |
| 路由挂载 | `router.go` 加一行 `RegisterMemoryRoutes(api, memoryHandler)` | **不要与成员 1 同时改** |

### 2.2 不做什么（Out of Scope）

| 不做的事 | 归属 / 原因 |
|---|---|
| **记忆提取链路**（`ai-service/app/agents/memory_agent.py`、规则正则、打分阈值 0.6） | **成员 1**。本功能**只提供落库方法**（§4.6），不实现正则、不实现打分、不决定"什么时候提取" |
| **`AIClient.ExtractMemories` 与 `POST /memory/extract` 的调用时机** | 成员 1。技术文档 §5.1 时序：**SSE 的 `done` 发出之后**异步调一次，**不要内联进流式生成过程**（理由与 `/schedule/parse` 相同，技术文档 §5.7 旁注） |
| **记忆检索与注入 Prompt**（`app/memory/recent.py`、`RecentMemoryRetriever`） | 成员 1。阶段一走 PG、阶段二走 ChromaDB，**接口不变只换实现**（技术文档 §5.3）。本功能不管检索 |
| **ChromaDB 写入与双写一致性** | 阶段二。`embedding_id` / `embedding_status` 两列**本期只保证类型正确、默认值正确**，不写任何索引同步代码（§3.3 第 3 条） |
| **`GET /profile/portrait`（用户画像）** | 契约 §7 的另一半，属 **`user_profile`** 表，是**另一个功能**（P1）。两张表同粒度（都挂 `persona_id`）但生命周期不同，**不要在本功能里顺手把它也做了** |
| **删除 / 编辑记忆的端点** | 总纲 §0：**只读**。不提供端点，**连预留路径都不留**（与 `/emotion/diary` 同一处理方式） |
| **前端画像页 / 记忆页、`api/memory.ts`、`types/memory.ts`** | **成员 2**。本功能只做后端，但要把「前端必须传 `personaId`」与「空态是 `[]` 不是 `null`」对齐清楚（§4.2） |
| **按记忆类型筛选**（`?memoryType=fact`） | 契约 §7 的请求参数**只有** `personaId` `page` `pageSize`。**不要顺手加**——那是契约漂移。`idx_memory_persona_type` 是给将来/别处用的索引，不代表端点要暴露这个参数 |
| **情绪记忆** | `memoryType` 只有 `fact` / `preference` / `event`，**没有 `emotion`**（总纲 §0：记忆不存情绪）。情绪只是提取时的打分输入 |
| **记忆数量上限、去重 / 合并** | 契约与 DDL 都没有。**不要自作主张加"同一内容不重复写入"的校验**——重复由提取侧（阶段二 LLM）自行抑制 |
| 新增错误码 | 本功能**一个都不需要新增**，复用 `4001` / `4043` / `5003` / `4010` |
| 手写 `ALTER TABLE` / `DropTable` | 红线 8：改结构 = 改 struct + AutoMigrate |

## 3. 数据模型（`user_memory`）

### 3.1 权威 DDL（技术文档 §6.2，照抄待翻译）

```sql
-- 记忆表（一人设一份）
CREATE TABLE user_memory (
    id                BIGSERIAL PRIMARY KEY,
    user_id           BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id        BIGINT       NOT NULL REFERENCES personas(id) ON DELETE CASCADE,
    memory_type       VARCHAR(20)  NOT NULL,       -- fact | preference | event
    content           TEXT         NOT NULL,
    embedding_id      VARCHAR(64),                 -- 对应 ChromaDB 中的向量 id
    embedding_status  VARCHAR(10)  NOT NULL DEFAULT 'pending', -- pending | synced | failed
    importance_score  NUMERIC(4,3) NOT NULL,       -- 0.000 ~ 1.000
    source_message_id BIGINT REFERENCES chat_messages(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_memory_persona_type ON user_memory(persona_id, memory_type);
CREATE INDEX idx_memory_user_id ON user_memory(user_id);
CREATE INDEX idx_memory_embedding_status ON user_memory(embedding_status); -- 补偿任务扫描
```

### 3.2 字段对照表（DDL ↔ GORM ↔ JSON）

| DDL 列 | Go 类型 | GORM tag 要点 | JSON 字段 | 契约来源 |
|---|---|---|---|---|
| `id BIGSERIAL PK` | `uint64` | `type:bigint;primaryKey;autoIncrement` | `id` | §7 |
| `user_id BIGINT NOT NULL` | `uint64` | `type:bigint;not null;index:idx_memory_user_id` | **`"-"`** | ⚠️ 契约 MemoryItem 里**没有** `userId` |
| `persona_id BIGINT NOT NULL` | `uint64` | `type:bigint;not null;index:idx_memory_persona_type,priority:1` | `personaId` | §7（v1 新增） |
| `memory_type VARCHAR(20) NOT NULL` | `model.MemoryType` | `type:varchar(20);not null;index:idx_memory_persona_type,priority:2` | `memoryType` | §7 |
| `content TEXT NOT NULL` | `string` | `type:text;not null` | `content` | §7 |
| `embedding_id VARCHAR(64)`（可空） | `*string` | `type:varchar(64)` | **`"-"`** | 契约**无**此字段 |
| `embedding_status VARCHAR(10) NOT NULL` | `string` | `type:varchar(10);not null;default:'pending';index:idx_memory_embedding_status` | **`"-"`** | 契约**无**此字段 |
| `importance_score NUMERIC(4,3) NOT NULL` | `float64` | `type:numeric(4,3);not null` | `importanceScore` | §7 |
| `source_message_id BIGINT`（可空） | `*uint64` | `type:bigint` | **`"-"`** | 契约**无**此字段 |
| `created_at TIMESTAMPTZ NOT NULL` | `time.Time` | `type:timestamptz;not null;default:now();autoCreateTime` | `createdAt` | §7 |

**所有 ID 类型统一 `uint64`**（`id` / `user_id` / `persona_id` / `source_message_id` 的指针基类型），与 `user.go` / `persona.go` / `chat_message.go` 保持一致，不做 `uint` / `int64` 的来回转换。

**五条必须记住的结论：**

1. **`user_id` 与 `embedding_*`、`source_message_id` 都不进响应体**：契约 §7 的 MemoryItem 只有 **6 个字段**（`id` `personaId` `memoryType` `content` `importanceScore` `createdAt`）——逐个数一遍，**没有 `userId`**。用 `json:"-"` 关掉（与 `User.PasswordHash` / `Persona.UserID` 同一手法）。`user_id` 是越权防线的载体，不是给前端看的；`embedding_*` 是阶段二的内部状态，**前端物理上拿不到，从结构上杜绝误渲染**（同 `ai_moments.emotion_label` 的处理）。
2. **`ID` 的 tag 是 `type:bigint` + `autoIncrement`，不是 `type:bigserial`**：这是**全项目已经踩过两次的坑**（`user.go` / `persona.go` 一轮，`chat_message.go` 一轮）。GORM 建关联时会把**被引用主键的 `DataType`** 复制到外键列上（`schema/relationship.go` 不抄 `autoIncrement`），写成 `bigserial` 会让下游外键长出 `DEFAULT nextval(...)`。**本表是这条链的末端**（`user_memory.source_message_id` 引用 `chat_messages.id`），但**将来仍可能有人引用 `user_memory.id`**（阶段二 `embedding_id = memory_id` 就把主键语义抬到了 ChromaDB），所以**照抄正确写法，不要开倒车**。
3. **`embedding_id` 与 `embedding_status` 本期是"空跑"列**：阶段一全部为 `NULL` / `'pending'`（技术文档 §6.3：保留这两列是为了阶段二接入 ChromaDB 时**无需改表**）。
   - `embedding_id` 用 `*string`（可空），**不要用 `string`**——值类型会把 `null` 序列化成 `""`，阶段二的补偿任务就分不清"还没索引"与"索引 id 是空串"。
   - `embedding_status` 是 `NOT NULL DEFAULT 'pending'`，用 `string` + `default:'pending'`。**写入方不需要显式赋值**（留零值即可，GORM 会补）。⚠️ **但别说成"靠 DDL 默认值"**：string 类型的 `default:` 会被 GORM 解析进 `DefaultValueInterface`，于是 `Create` 时这一列被**显式写进 INSERT**、**不走数据库默认值**（GORM 还会把值回写进内存结构体）——DDL 里那条 `DEFAULT 'pending'` 只是同一个值的**第二份副本**，改默认值时**两处要一起改**。GORM 侧行为已由源码确认并**实测复现**（§8 分组 A）。另：**读侧不要假设它一定是三态之一**（§3.5）。
4. **`source_message_id` 是 `*uint64` + `ON DELETE SET NULL`，不是 CASCADE**：这是**本表与 `chat_messages` / `personas` 两处最不一样的地方**。技术文档 §6.2 明确：删消息不该连带删记忆——"这条记忆是从哪句话来的"只是溯源，**句子没了，记忆本身还在**。用值类型会让 `SET NULL` 写不进去（`NOT NULL` 违例）；用 CASCADE 会让"删一条消息顺手删掉一条记忆"，与"记忆是长期事实"的定位直接矛盾。
5. **`importance_score` 用 `float64`，且**它**是本表最可能触发 pgx 类型问题的一列**：`chat_message.go` 已经在 `emotion_score NUMERIC(4,3)` 上留过同一条备注——pgx 往 `numeric` 编 `float64` 若实测报类型不匹配，**退到自写 `driver.Valuer` 返回字符串**（零新增依赖），**不要为此引入 `decimal` 包，也不要为此改列类型**。区别在于：`emotion_score` 可空、`importance_score` **NOT NULL 且是必写字段**，这条路径一定会被走到。

### 3.3 `memory_type`：为什么 Go 层加常量，DB 层却不加 CHECK

**DDL 上 `memory_type` 没有 CHECK 约束**（`chat_messages.role` 有 `CHECK (role IN (...))`，这里是 `VARCHAR(20)` 加一行注释而已）。这**不是**技术文档漏写，所以：

| 层 | 做法 | 理由 |
|---|---|---|
| Go | 定义 `type MemoryType string` + 三个常量 `MemoryTypeFact` / `MemoryTypePreference` / `MemoryTypeEvent` | 编译期挡住拼写错误（`"presonal"`）。与 `MessageRole` 同一手法 |
| DB | **不加 `check:` tag** | DDL 没有这条约束，加了就是**偏离权威 DDL**（AGENTS §1：契约 > 技术文档 > 代码现状；DDL 是技术文档的一部分）。真要加，属于改 schema，得群里广播后统一改 DDL——**不是本分支能顺手做的事** |

> **代价（诚实记录）**：数据库这层没有兜底，绕过 Go 的写入（手工 INSERT、脚本）可以塞进任意 `memory_type`，而前端按 `fact` / `preference` / `event` 三分类渲染时会掉进"未知分类"分支。
> **对策不在本表**：写入方**只有一个**（§4.6 的 `CreateBatch`，由成员 1 的提取链路调用），且它接受的是 `model.MemoryType` 而不是裸 `string`；阶段二 LLM 返回的 `memory_type` 必须在**落库前**校验 / 归一（非法值丢弃并记日志，**不要原样落库**）。

### 3.4 外键与索引

**三个外键必须由 GORM 建出来**，否则"删人设级联清空记忆"与"删消息保留记忆"两条验收项都不成立。只写标量字段时 GORM **不会创建任何外键**——必须额外声明 belongs-to 关联：

```go
// 以下三个关联字段仅供 GORM 生成外键约束用，不参与序列化。
// Create 时不要给它们赋值（保持零值），否则 GORM 会尝试连带写入 users / personas / chat_messages 表。
User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
// ⚠️ 唯一一个 SET NULL：指针类型 + 可空列，删消息只断溯源、不删记忆
SourceMessage *ChatMessage `gorm:"foreignKey:SourceMessageID;references:ID;constraint:OnDelete:SET NULL" json:"-"`
```

> ⚠️ **`SourceMessage` 必须是 `*ChatMessage`（指针）**：关联对象用值类型时 GORM 会把它当"必有行"，与 `SET NULL` 的语义冲突，且零值 `ChatMessage` 会被当成一个待保存的实体。
> ⚠️ **`SET NULL` 要求列可空**，所以 `SourceMessageID *uint64` 与它必须成对出现——两者任一写错，`AutoMigrate` 要么报错、要么建出一列 `NOT NULL` 让删消息直接失败。

| 索引 | 用途 | 写法 |
|---|---|---|
| `idx_memory_persona_type` | `(persona_id, memory_type)`。列表查询 `WHERE persona_id = ?` 走**前缀**；将来按类型分组也用得上 | 两个字段共用一个 `index:` 名，`persona_id` priority 1、`memory_type` priority 2 |
| `idx_memory_user_id` | `user_id` 单列索引 | `index:idx_memory_user_id` |
| `idx_memory_embedding_status` | **补偿任务扫描**（阶段二）`WHERE embedding_status = 'pending'` | `index:idx_memory_embedding_status` |

> **`AutoMigrate` 顺序**：`&UserMemory{}` 必须排在 `&User{}` / `&Persona{}` / `&ChatMessage{}` **之后**——它同时引用三张表，任何一张没建好都会在建外键时失败。
> **它也是 `ChatMessage` 那个坑的下游**：`chat_message.go` 的注释里已经写明 `ID` 用 `bigint` 正是为了本表的 `source_message_id`。写本表时**不要反过来**把 `ChatMessage.ID` 改成 `bigserial`。

## 4. 接口定义

### 4.1 通用约定（契约 §1 / §2）

| 项 | 约定 |
|---|---|
| Base URL | `/api/v1` |
| 鉴权 | **必须**带 `Authorization: Bearer <accessToken>`；缺失/过期 → `4010`。**不在 4 个免鉴权白名单里**（AGENTS §4.2） |
| 响应体 | `{ code, message, data, timestamp }`，由成员 1 的 `pkg/response` 统一产出 |
| 分页 | `page` 从 1 开始；`pageSize` 默认 20、上限 100 |
| 分页响应 | `{ list, total, page, pageSize }`，类型 `PageResult[T]`，**定义在 `internal/dto/common_dto.go`** |
| 时间格式 | RFC3339 |
| 错误出口 | handler 一律 `_ = c.Error(err)` 上抛，由 `BizErrorHandler` 中间件统一出口；**handler 里不出现 `response.Fail`** |

**`PageResult[T]` 的位置（已定）**：`backend/internal/dto/common_dto.go`。personas / messages / memory / moments / schedules **五个分页端点共用这一个**——成员 1、成员 3 都从这里引，**不要各自再定义一份**（两份同名类型在联调时会出现字段对不上的诡异问题）。**本功能是它的第三个声明方**：若 persona / chat-message 分支已落地，**直接消费，不要重建**。

### 4.2 `GET /memory` — 某个人设的记忆列表

| 项 | 内容 |
|---|---|
| 请求 | query：**`personaId`（必传）**、`page`、`pageSize`（后两个可选） |
| 响应 data | `PageResult<MemoryItem>` = `{ list, total, page, pageSize }` |
| 错误码 | `4001`（缺 / 非法 `personaId`，见 §4.3）**`4043`**（人设不属于当前用户，或人设不存在，**同一个码**，见 §5.2） |

**MemoryItem（契约 §7 冻结，6 个字段）**

```json
{ "id": 7, "personaId": 1, "memoryType": "fact", "content": "用户养了一只叫豆豆的猫", "importanceScore": 0.85, "createdAt": "2026-09-10T14:30:00+08:00" }
```

**五条容易写错的要求：**

1. **`user_id` 只能来自 Token**，绝不从 query / body 取。请求 DTO 里**连这个字段都不该存在**——字段不存在，就没有被前端传进来的可能（结构性防御，比"记得校验"可靠）。契约 §7 已写明"前端永远不传 `user_id`"。
2. **查询必须同时带 `persona_id` 和 `user_id` 两个条件**（技术文档 §4.3）：
   - **只带 `persona_id` 会跨用户串号**：`persona_id` 是全局自增，**别的用户的同号人设会命中**。
   - **只带 `user_id` 会跨人设**：把该用户**所有人设**的记忆混在一起返回，演示脚本第 8 项当场翻车（切到人设 B 还能答出人设 A 的事）。
   - 这条**不是代码风格，是红线 3**。
3. **空列表必须返回 `[]` 而不是 `null`**：Go 的 nil slice 会序列化成 `null`，前端 `v-for` / `.map` 直接崩。切片必须初始化（`list := make([]MemoryItemResponse, 0)`）。`page` 超出范围时返回空列表 + 真实 `total`，仍是 `200`。
4. **`total` 必须用同两个条件统计**，不能只 `COUNT(*)` 全表，也不能只按 `persona_id` 统计（后者会把别人的记忆算进你的 `total`——**这是个只泄漏数量、不泄漏内容的口子，比全量泄漏更隐蔽**）。
5. **"人设属于你但一条记忆都没有"是 `200` + 空列表，不是 `4043`**。这两种"空"必须分得清清楚楚：**归属判定单独做一次**，不能靠"记忆查不到就 4043"——否则新伴侣的记忆页会"打不开"（§5.2）。

### 4.3 缺 `personaId` 时返回什么（契约空白，需广播）

**问题**：契约 §7 的错误码列**只写了 `4043`**。但 `personaId` 缺失时没有任何归属可校验，"查不到 → 4043"在语义上不成立（没有东西"不存在"）。

| 方案 | 后果 | 评价 |
|---|---|---|
| 当作 `personaId = 0` 查库 → 命中空集 → `200` + 空列表 | 前端漏传参数时看到"这个人设没有任何记忆"，**把调用方的 bug 伪装成正常空态** | ❌ 静默降级，与 `AGENTS §4.3`「取不到 `userID` 不许退化成 `0`」同一条原则相悖 |
| 返回 **`4001`**（`errcode.ErrInvalidParams`） | 参数缺失归 4000-4009 段，与契约分段规则一致；**§10 的 `GET /schedules` 已有先例**（同为「必带 `personaId`」的分页端点，错误码列写的是 `4001 4043`，旁注"缺失返回 `4001`"） | ✅ **本功能的做法** |

**动作（与本分支的代码分开）**：契约 §7 的错误码列应由 `4043` 改为 `4001 4043`（`GET /memory` 一行），并按契约纪律**登记 §12 变更记录 + 群里广播**。`API_CONTRACT.md` 是**全局文件，本分支不直接改**——交出去由队长 / 成员 1 / 契约负责人统一处理（§7.2 交接项）。

> **`binding` 失败一律 `4001`**，不要细分：`personaId` 缺失、传了非数字、传了 `0` 或负数，**统一 `ErrInvalidParams`**。**不要**因为"缺必填字段"就改成 `4002`（`ErrParamMissing`）——那是契约里别的端点的语义，这里擅自细化就是契约漂移（同 persona spec §4.3 第 4 条）。

### 4.4 排序与分页

**DDL 没规定列表顺序，契约也没写**，所以要显式定一条（§7.3 待确认 #2）：

```sql
ORDER BY created_at DESC, id DESC
```

**三条理由：**

1. **`created_at DESC`（新的在前）** 符合"最近记住的事"的直觉，也与 `app/memory/recent.py` 的召回排序方向一致（技术文档 §5.3：按 `importance_score` + `created_at` 排序）。
2. **`id DESC` 兜底不是可选项**：Postgres 的 `NOW()` 是**事务开始时间**（`chat_message.go` 已记录同一条坑），而**记忆提取是批量写入**——一轮对话提取出的 3 条记忆在**同一个事务**里插入，`created_at` **完全相同**。只按 `created_at` 排序时同值行顺序不确定，**翻页会出现"第 2 页冒出第 1 页的记忆"或漏项**。
3. **不要用 `importance_score` 当主排序**：它是 `NUMERIC(4,3)`，阶段一规则档**所有记忆都是同一个固定分 0.8**（技术文档 §5.3），排序完全无区分度；且它不满足"稳定顺序"的需求（同分太常见）。**类型分组的活儿留给前端**——`memoryType` 已经在响应里了，前端自己 `groupBy` 即可，**不需要服务端排序来帮它**。

> **索引与排序不一致是正常的**：`idx_memory_persona_type` 是 `(persona_id, memory_type)`，用不上 `created_at` 的排序。本项目每人设的记忆量级是**几十条**（阈值 0.6 + 每轮最多提取 2-3 条），排序开销可以忽略——**不要为了排序顺手加索引**（那是改 schema，得走 DDL 广播）。

### 4.5 请求 / 响应结构

| 结构 | 定义位置 | 字段 | 说明 |
|---|---|---|---|
| `PageResult[T]` | **`dto/common_dto.go`** | `list` `total` `page` `pageSize` | **共用，不重建** |
| `MemoryListQuery` | `dto/memory_dto.go` | `personaId`（`form` tag，**必须逐字 camelCase**）`page` `pageSize` | `personaId` 必填 → 缺失走 `4001` |
| `MemoryItemResponse` | 同上 | `id` `personaId` `memoryType` `content` `importanceScore` `createdAt` | **6 个字段，一个不多一个不少** |
| `NewMemoryItemResponse(m *model.UserMemory) MemoryItemResponse` | 同上 | — | 转换函数放 DTO 层，**不要写在 model 上**（model 不依赖 dto） |

> ⚠️ **query 参数的 tag 必须是 `form:"personaId"`**（不是 `json:"personaId"`，也不是 `form:"persona_id"`）。Gin 的 `ShouldBindQuery` 读的是 `form` tag，且**区分大小写**——写成 `form:"persona_id"` 时 Gin 找不到匹配字段、**不报错**，`PersonaID` 静默为 `0`，然后被 §4.3 的 `4001` 拦住。表现为"前端明明传了 `personaId`，接口却说缺参数"，极难查。

### 4.6 写入方法（`CreateBatch`，供记忆提取链路，**不是端点**）

记忆的**写入不由本功能的端点触发**——总纲 §0 明确记忆只读，没有 `POST /memory`。唯一的写入方是**成员 1 的记忆管理 Agent 链路**（技术文档 §5.1 时序 ⑤）：SSE 的 `done` 之后异步提取 → 超 0.6 的候选 → 落库。

**本功能提供这条链路要用的低层方法**（与 chat-message 分支提供 `Create` 给 SSE 链路是同一件事）：

```go
// 一个事务里写入一轮提取的全部记忆；memories 为空直接返回 nil（不写空事务）
CreateBatch(ctx context.Context, tx *gorm.DB, memories []*model.UserMemory) error
```

**四条交接约定（写给成员 1，也是本方法的使用说明）：**

1. **`user_id` 与 `persona_id` 由 Go 侧赋值，不采信 AI 服务的返回体**。`/memory/extract` 的响应里只有 `{memory_type, content, importance_score}`（技术文档 §5.3 的 LLM 输出契约）——**没有归属字段**，也**不该有**。归属来自本轮对话所属的人设（已通过归属校验），这是数据来源可追溯性的一部分。
2. **`source_message_id` 填本轮的用户消息 id**，可空。填了就有溯源，且删消息时外键会自动置 `NULL`（§3.4）——**不要为了"保住溯源"改成 CASCADE 或自己写清理逻辑**。
3. **`embedding_id` / `embedding_status` 不要赋值**（留零值即可，GORM 会按 tag 补上 `'pending'`）。⚠️ 别理解成"交给数据库默认值"：GORM 会把 tag 里的默认值**显式写进 INSERT**，DDL 的 `DEFAULT 'pending'` 只是同值的第二份副本（§3.2 第 3 条，已实测）。阶段二接入 ChromaDB 时按技术文档 §5.3 的双轨设计：先写 PG 拿 `memory_id` → 再写 ChromaDB（`embedding_id = memory_id`）→ **ChromaDB 失败不回滚 PG**，只标 `embedding_status = 'pending'` 交给补偿任务。
4. **`memory_type` 必须在落库前校验**：阶段一规则档只会产出三个合法值；**阶段二 LLM 可能返回别的东西**（§3.3）。非法值**丢弃并记日志**，不要原样写进库，也不要静默改成 `fact`。

> **本功能不决定"什么时候调用它"**：调用时机在 SSE 的 `done` 之后、异步执行（技术文档 §5.1 / §5.7 旁注），属成员 1 的 `chat_service` / `ai_client`。本功能只保证"给对了参数，这一批就原子地落进去"。

### 4.7 明确不做的端点（**不该写的代码比该写的更重要**）

| 不写的端点 | 原因 |
|---|---|
| `POST /memory` | 记忆只能由对话提取产生，**不提供手工新建**（同"没有 `POST /schedules`"的逻辑，契约 §10） |
| `DELETE /memory/:id` | 总纲 §0：**只读展示，不提供删除接口** |
| `PUT /memory/:id` | 同上；记忆是长期事实，不由用户编辑 |
| `GET /memory/:id` | 契约没有单条详情端点；**也刻意不要在仓储层留 `GetByID`**（§5.1 ②） |
| `GET /memory?personaId=` 之外的任何列表（全站 / 跨人设） | 账号之间完全隔离（AGENTS §4.3），**不存在跨用户列表** |
| `GET /emotion/diary` 之类 | 情绪不作为记忆存储，且"情绪日记"违反"情绪是内部信号"（AGENTS §4.4）——**连预留路径都不留** |

## 5. 越权防线（本功能最重要的正确性约束）

> 对应红线 3：**❌ 跨用户 / 跨人设数据泄漏**。技术文档 §4.3 把这张表点名为重灾区："记忆与画像都是一人设一份。查询必须同时带 `persona_id`（隔离人设）和 `user_id`（越权防线，从 Token 取，前端永不传）。只带 `persona_id` 会跨用户串号，只带 `user_id` 会跨人设。"

> 📌 **本节的落地范围（决策 1）**：这些防线里**只有数据层（§5.1 ③ 的外键 + 索引）由本分支落地**——它写在 `model/user_memory.go` 里。**入口层与仓储层的代码属成员 1**（`handler` / `repository` / `service`），本节因此是**交接给成员 1 的设计约定**与**审查清单**，不是本分支要写的代码。**唯一写入方与本表的直接查询方都不止一个**（Go 的读端点、Go 的提取落库、ai-service 的检索器），所以这份约定要同时约束三处。

### 5.1 三层防线

**① 入口层（Handler）——`user_id` 只有一个来源**

- `user_id` 只能从 JWT 声明取（成员 1 的 `JWTAuth` 中间件写入 `gin.Context`，键名 `middleware.ContextKeyUserID` = `"userId"`），**永不从 body / query / path / header 读**。
- **类型是 `uint64`**（已定，与 `user.go` 一致）；用 `c.GetUint64(middleware.ContextKeyUserID)` 读，**必须检查第二个返回值 `ok`**。
- 取不到（`ok == false` 或值为 `0`）→ **返回 `4010`，直接中断**，绝不退化成 `userID = 0` 继续查。因为 `WHERE user_id = 0` 会返回空集——**看起来没泄漏，实际是把鉴权失败伪装成了空列表**，这种"静默降级"比报错危险得多。
- 请求 DTO 里**不声明** `userId` 字段：字段不存在就无法被传入。

> 🚧 **这一层目前不可用（决策 2）**：`internal/middleware/jwt.go` 的 `JWTAuth` **是空壳**（函数体只有 `c.Next()`，`ContextKeyUserID` 从未被 `Set`）。也就是说 `c.GetUint64(ContextKeyUserID)` 现在**必然返回 `ok == false`**。后果与处理：
> - 按上面第 3 条，接口会稳定返回 `4010`——**这是"没实现"而不是"防住了"，不要把它当成越权防线生效的证据**。
> - **端到端验收做不了**，本分支**只做模型层验证**（§8 分组 A）。**不要为了跑通自己实现一份 `JWTAuth`**——那是成员 1 的文件（红线：不 Own 的东西不要动）。
> - 与成员 1 对齐接口层时，**第一条要确认的就是它**：`jwt.go` 的函数体里必须有一句 `c.Set(ContextKeyUserID, claims.UserID)`。好消息是类型已经对得上——`pkg/jwt.Claims.UserID` 就是 `uint64`（与 `model.User.ID` 一致），**所以 `c.GetUint64` 能直接取到，不需要任何转换**。要防的是落地时被写成 `c.Set(ContextKeyUserID, int(claims.UserID))` 之类：那样 `ok` 会恒为 `false`，表现为"登录了却说未登录"。

**② 仓储层（Repository）——让"不安全的查询"在结构上不存在**

- 方法签名**强制**带 `userID uint64` **和** `personaID uint64`：`ListByPersona(ctx, userID, personaID, offset, limit)`。调用方没有"忘了传"的选项。
- 过滤条件**下沉到 SQL 的 `WHERE` 里**，而不是"先把该人设的记忆全查出来、再在 Go 里 `if m.UserID != userID`"。后者有两个问题：多查了不该查的数据（**已经在内存里了，日志/panic 都会带出去**）；以及**忘写那个 `if` 就静默越权**——这类 bug 在 code review 里也容易被漏掉，因为它看起来"逻辑完整"。
- **仓储层不提供任何"按 id 单查记忆"或"按 persona_id 单查"的方法**。§4.7 已定"不提供详情端点"，那"单查"这个能力在代码里**根本不需要出现**——少一个方法就少一处泄漏面。
- **也不提供任何删除方法**（§4.7）。仓储层多一个 `Delete`，将来就有被误调的可能。
- 事务句柄作为显式参数传入（`tx *gorm.DB`），事务边界由调用方（service / 提取链路）决定。

**③ 数据层（DB）与外层**

- `user_memory.user_id` / `persona_id` 上的外键 + 三个索引（§3.4）。
- 删人设 → 外键级联清空其记忆（验收项）；删账号 → 同理。**不要手写多表清理代码**。
- **本表还有第二个查询方**：`ai-service/app/memory/recent.py` 的 `RecentMemoryRetriever.retrieve(user_id, persona_id, query, top_k)`——**它也必须带两个条件**（技术文档 §5.3 的注释已写明：`WHERE persona_id = ? AND user_id = ?`）。这不是本仓库的代码，但**阶段二换 ChromaDB 时同样是硬要求**：`persona_memories` 集合的 `metadata` 要存 `{user_id, persona_id, ...}`，检索时**两个过滤条件都带**后再做向量排序（技术文档 §6.3）。只按 `persona_id` 过滤不够——它是全局自增，**别的用户的同号人设会串号**。

### 5.2 归属校验：`4043` 与"空列表"的分界线（最容易写错的一处）

`GET /memory` 是一次"先判归属、再查数据"的**两步**操作，因为返回码取决于**人设**而不是**记忆**：

| 情形 | 返回 | 为什么 |
|---|---|---|
| 人设属于当前用户，有 N 条记忆 | `200` + `list` 有 N 条 | 正常 |
| **人设属于当前用户，0 条记忆** | **`200` + `list: []`** | 合法空态（新伴侣还没记住任何事）。**绝不能返回 `4043`**——那样新伴侣的记忆页会"打不开" |
| 人设**不属于**当前用户 | **`4043`** | 资源越权。**不返回 `4030`**（§5.3） |
| 人设**不存在** | **`4043`** | 与上一行**同码同文案**，外部无法区分（这正是要的效果） |
| `personaId` 缺失 / 非法 | `4001` | §4.3 |

**实现上的两条硬要求：**

1. **归属判定必须单独查一次**，用 `persona_repo.ExistsOwnedByUser(ctx, userID, personaID) (bool, error)`——**一条查询、两个条件**（`WHERE id = ? AND user_id = ?`）。**不要**从记忆查询的结果反推归属：`len(list) == 0` 既可能是"不是你的"也可能是"还没有记忆"，**这两种情况在结果集上长得一模一样**，用它做判断必然误伤空态。
2. **记忆查询本身仍然要带两个条件**（§5.1 ②）。归属判定是"为了让错误码正确"，不是"因为记忆查询不安全"——**两道防线各司其职，不要用前者替代后者**（否则 A 人设的记忆会被判成"归属 OK"，然后按 `persona_id` 单条件查出来）。

> **这一步是"资源越权"，不是"功能越权"**：按 [总纲 §4 的通用错误码规则](../dev/MASTER.md)（队长 2026-09-13）：**资源越权 → `4043`**（隐藏资源存在性）、**功能越权 → `4030`**。`GET /memory` 全程属资源越权侧，**不产生 `4030`**。
> 契约 §7 对 `GET /memory` 的错误码**已经是 `4043`**（`4030` 已在早前的规则统一中清掉），**本功能不需要申请契约变更**——唯一的空白是 §4.3 的 `4001`。

### 5.3 "不存在"与"不属于你"必须同码同文案

`persona_id` 是 `BIGSERIAL` 全局自增。若"不属于你"回 A 码、"不存在"回 B 码，调用方拿同一个 id 各打一次就能判断这个 id **到底存不存在**——隐藏存在性的目的当场失效。

所以**同一个端点内这两种情形只有一个答案**：`errcode.New(errcode.ErrPersonaNotFound)`（`4043`，"人设不存在"）。**结构上就只有一个 `if !ok` 分支**，不需要"先按 id 探针二分再判码"的两步判定——那两步的唯一目的就是区分这两个码，现在不需要了。

**代价（诚实记录）**：调试时"看别人人设的记忆"与"看不存在的人设"返回同一个码，排查略麻烦——靠服务端日志区分（`BizErrorHandler` 会为 4xxx 记 warn 日志，含 `traceId`）。

### 5.4 反例清单（AI 最容易写出来的错法）

| ❌ 错法 | 后果 | ✅ 正确 |
|---|---|---|
| 只 `WHERE persona_id = ?` | **跨用户泄漏**：`persona_id` 全局自增，别的用户同号人设的记忆全被读出来 | `WHERE persona_id = ? AND user_id = ?` |
| 只 `WHERE user_id = ?` | **跨人设串号**：把该用户所有人设的记忆混在一起，演示第 8 项当场翻车 | 同上，两个条件都要 |
| `COUNT(*)` 不带条件 / 只带一个条件 | `total` 泄漏别人记忆的**数量**（内容没漏，但口子已经开了） | `total` 与列表用**同一组条件** |
| `db.First(&m, id)` 再在 Go 里比 `UserID` | 忘写比较就静默越权；数据已经进了内存 | 仓储层不提供单查方法 |
| 用 `len(list) == 0` 判越权 / 不存在 | **把"还没记住任何事"误报成 `4043`**，新伴侣的记忆页打不开 | 归属单独查一次（§5.2） |
| 未命中时返回 `4030` | 违反通用规则（`4030` 是**功能越权**的码，本模块无该场景）；且 `4030`/`4043` 并存即可被二分探测出 id 是否存在 | 统一 `4043` |
| 从 query / body 读 `userId` | 攻击者传谁的 id 就看谁的数据 | 只从 JWT 上下文取 |
| 取不到 `userID` 时用 `0` 兜底继续查 | 鉴权失败伪装成空列表 | 返回 `4010` 并中断 |
| 缺 `personaId` 时按 `0` 查库、返回空列表 | 前端漏传参数被伪装成正常空态，bug 藏到联调后期 | `4001`（§4.3） |
| `list := make([]MemoryItemResponse, 0)` 写成 `var list []MemoryItemResponse` | 空态序列化成 `null`，前端 `v-for` 崩 | 显式初始化切片 |
| 未命中时返回成功（`200`） | 越权读取被伪装成成功，比报错危险 | 未命中就是 `4043` |
| 顺手加 `DELETE /memory/:id` | 违反总纲 §0「只读」；且删除路径同样要防越权，白增一处泄漏面 | 不写（§4.7） |
| 把 `embedding_id` / `embedding_status` 加进响应体 | 阶段二的内部状态泄漏到前端，且契约没有这两个字段 | `json:"-"`（§3.2 第 1 条） |
| `SourceMessageID` 用值类型或 `OnDelete:CASCADE` | 值类型 → `SET NULL` 写不进去（NOT NULL 违例）；CASCADE → 删一条消息顺手删掉一条长期记忆 | `*uint64` + `SET NULL`（§3.4） |
| 排序只写 `created_at DESC` | 批量写入的同一批记忆时间戳完全相同，翻页重复 / 漏项 | `created_at DESC, id DESC` |

## 6. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| 字段名**全 camelCase**，与契约 §7 逐字一致（`memoryType` 不是 `memory_type`，`importanceScore` 不是 `importance_score`） | 协作规范 §5 / 契约 §1 |
| **查询必须同时带 `persona_id` 与 `user_id`**；`user_id` 只从 Token 取 | **红线 3** / AGENTS §4.3 / 技术文档 §4.3 |
| **`memoryType` 只有 `fact` / `preference` / `event`，没有 `emotion`** | 总纲 §0 / 契约 §7 |
| **不提供记忆的删除 / 编辑 / 新建端点**（连预留路径都不留） | 总纲 §0 / 契约 §7 |
| **不新增错误码**；本功能只用 `ErrInvalidParams`(4001) / `ErrPersonaNotFound`(4043) / `ErrDBFailed`(5003) / `ErrUnauthorized`(4010) | 技术文档 §4.3 分段规则 |
| **资源越权 → `4043`**，本模块**不产生 `4030`**；`userID == 0` 一律 `4010` | 队长规则 2026-09-13 / §5.2 |
| **禁止硬编码错误码数字或文案**；`Fail()` 只接受 `ErrorCode` | **红线 6** |
| handler **不写** `response.Fail`，错误 `_ = c.Error(err)` 上抛 | 技术文档 §4.4 |
| handler 不直接操作 DB；service **不依赖 `*gin.Context`**（只收 `context.Context`） | **红线 7** / 技术文档 §4.1 |
| 记忆的批量写入事务边界在**调用方**（service / 提取链路），repo 只接受事务句柄 `tx` | 技术文档 §4.1 / 红线 7 |
| **`ID` 的 tag 是 `type:bigint` + `autoIncrement`**，不是 `type:bigserial` | §3.2 第 2 条 |
| `source_message_id` 是 `*uint64` + `constraint:OnDelete:SET NULL`；另两个外键 `ON DELETE CASCADE` | 技术文档 §6.2 / §6.3 |
| `embedding_id` / `embedding_status` / `source_message_id` / `user_id` **不进响应体**（`json:"-"`） | 契约 §7 |
| `PageResult[T]` 只有 `dto/common_dto.go` **一份**定义 | 协作规范 §3 / spec §4.1 |
| 改表结构只改 struct + `AutoMigrate`，**禁止手写 `ALTER TABLE` / `DropTable`**；`AutoMigrate` 顺序在 `&ChatMessage{}` 之后 | **红线 8** / §3.4 |
| 表名 `user_memory`（**不可数、单数**，AGENTS §4.3 的例外）、列名小写下划线；Go 缩写词全大写（`UserID` 不是 `userId`）；ID 一律 `uint64` | 协作规范 §5 |
| 分支 `feature/backend-user-memory-model`，走 PR，**禁止直推 `main` / `develop`**；commit 格式 `<type>(<scope>): <subject>`，scope 用 `memory` | **红线 2** / 协作规范 §6 |
| **不提交任何密钥**：本功能不新增任何 Key/Token/密码；禁止把 DSN/密码写进代码或 `_test.go`；`AI_SERVICE_TOKEN` 只在 `.env.example` | **红线 1** |
| **契约变更（§4.3 的 `4001`）未广播、`API_CONTRACT.md` 未更新前，不得合并 PR** | 契约纪律 |

**红线自查（AGENTS.md §7 逐条对照）**

| 红线 | 本功能是否涉及 |
|---|---|
| ① 提交 `.env` / Key / 密码 / 模型权重 | 不涉及新增。**测试代码里也不许出现密码**；AI 服务的 `AI_SERVICE_TOKEN` 只出现在两个 `.env.example` |
| ② 直推 `main` / `develop` / force push | 流程约束：本分支走 PR，至少 1 人 Approve |
| ③ 跨用户 / 跨人设泄漏 | **主要战场**，见 §5（本表是整个项目的第二个重灾区，仅次于人设） |
| ④ 明文 / 弱哈希存密码 | 不涉及（不碰密码） |
| ⑤ 情绪展示标签 / 朋友圈手动触发 / 日程新建端点 | **不涉及，且本表是"不存情绪"的落点**：`memoryType` 没有 `emotion`；**不要**因为"情绪也值得记"就加第四个类型 |
| ⑥ 硬编码错误码或文案；`Fail()` 传自定义 message | 见 §6 上表 |
| ⑦ handler 直接操作 DB；service 依赖 `*gin.Context`；组件写 axios | 见 §6 上表 |
| ⑧ 生产 `DropTable`；手写 `ALTER TABLE` | 见 §6 上表（`embedding_*` 两列**已在 DDL 里**，阶段二接入时**不需要改表**——这正是它们现在就被翻译进来的原因） |

## 7. 依赖与交接

### 7.1 我依赖谁

**A. 本分支（模型层）——依赖已全部就绪，可以立即开工**

| 依赖 | 提供方 | 现状（2026-09-15） | 影响 |
|---|---|---|---|
| `gorm.io/gorm` + `gorm.io/driver/postgres` | — | ✅ 已进 `go.mod`（persona 分支落地时引入） | `model/` 与 `cmd/migrate` 可编译 |
| `internal/model/jsonb.go`、`migrate.go`、`chat_message.go` | 成员 3 / 成员 1（`chat_message.go` 已合入） | ✅ **已落地** | `UserMemory` 的三个外键关联分别引用 `User` / `Persona` / `ChatMessage`，**三个都已在仓库里**，不需要等任何人 |
| `cmd/migrate`（独立的建表命令，读 `SCHEMA_CHECK_DSN`） | 成员 3 | ✅ **已落地** | 模型层验证靠它跑 `AutoMigrate` |

**B. 接口层（交接给成员 1，本分支不落地）**

| 依赖 | 提供方 | 现状 | 影响 |
|---|---|---|---|
| **`middleware.JWTAuth`** | 成员 1 | 🚧 **是空壳**（`jwt.go` 只有 `c.Next()`，`ContextKeyUserID` 从未被 `Set`） | **越权防线入口层取不到 `userID`**：handler 会稳定走 `4010` 分支。**接口层端到端验收（§8 分组 B）在它实现前做不了**——这是本功能唯一的硬阻塞 |
| `internal/config`、`pkg/logger`、`middleware`（除 JWT）、`pkg/errcode`、`pkg/response` | 成员 1 | ✅ 已落地 | service / handler 可编译（一旦决定落地它们） |
| `dto/common_dto.go` 的 `PageResult[T]` | 三个分支之一先落地 | ⚠️ **尚未落地** | 三个分支共用一份；**谁先合谁建，后来者消费，不要重建** |
| `repository/persona_repo.go`（`ExistsOwnedByUser`） | chat-message 分支已声明 | ⚠️ **尚未落地** | **同一文件三个分支都会改**（§5 风险），群里确认谁先合 |
| `router.go` 的挂载点 | 成员 1 | ⚠️ **尚未落地**（仓库中无 `router.go` / `cmd/server`） | 端点不可达；**等它落地后加一行，不要与成员 1 同时改** |

> **本分支能做的与不能做的（决策 1 + 2）**：
> - ✅ **能做**：`model/user_memory.go` + `migrate.go` 追加一行 + 文档；**在真库上验证表结构**（`AutoMigrate` 跑通、3 个外键的 `ON DELETE` 行为、3 个索引、序列清单）——`cmd/migrate` 已就绪，**不需要任何接口层**。
> - ❌ **不做**：`dto/` / `repository/` / `service/` / `handler/` 的任何代码（表格 B 的六项）。**接口层测试也不做**（决策 2）。
> - ⛔ **不要为了"跑起来"自己造一份 `router.go`、`JWTAuth`、`pkg/errcode` 或 `PageResult`**——那四样都是别人的文件（红线：不 Own 的东西不要动；造了必冲突）。

### 7.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| **`internal/model/user_memory.go`（本分支实际交付的唯一代码文件）** | **成员 1** | 记忆提取链路落库要引 `UserMemory`；`model/migrate.go` 已把表建好 |
| **接口层的完整设计**（`dto/memory_dto.go` + `repository/memory_repo.go` + `service/memory_service.go` + `handler/memory_handler.go` 的字段、方法签名、错误码、判定顺序） | **成员 1** | spec §4 / §5 + plan §3 **就是给这四份文件写的规格**。本分支**不写这些代码**（决策 1）——他若已在写，按此对齐；若要本分支接手，先群里对齐 |
| `CreateBatch(ctx, tx, memories)` 的设计（**方法本身待成员 1 落地**） | **成员 1** | SSE `done` 之后的异步提取落库（技术文档 §5.1 时序 ⑤），而且是记忆表**唯一的写入入口**。**四个交接约定见 §4.6** |
| `SourceMessageID *uint64` + `SET NULL` | **成员 1** | 填本轮用户消息 id 做溯源；删消息时外键自动置 `NULL`，**不需要他写任何清理逻辑** |
| `embedding_id` / `embedding_status` 两列 | **成员 1 + 成员 3（阶段二）** | 接入 ChromaDB 时**无需改表**（这两列已在 DDL 里）；双写一致性按技术文档 §5.3 |
| `GET /memory` 的响应结构（`MemoryItem` 6 字段） | **成员 2** | 写 `types/memory.ts` 与 Mock；**字段名逐字对齐 camelCase**。⚠️ 特别提醒：**没有 `userId` / `embeddingId` / `embeddingStatus` / `sourceMessageId`**，Mock 里不要自作主张加 |
| `GET /memory` 的空态与缺参约定 | **成员 2** | `list` 是 `[]` 不是 `null`；缺 `personaId` 是 `4001` 不是空列表；切人设必须重新请求（记忆按人设隔离） |
| ⚠️ **契约 §7 需补 `4001`（阻塞合并）** | **队长 / 契约负责人** | §4.3 的契约空白（`GET /memory` 错误码列 `4043` → `4001 4043`），需登记 §12 变更记录 + 广播后由队长拍板。**本分支不直接改全局文件**（决策 3） |

### 7.3 决策记录与剩余待确认

**已定（2026-09-15 三项决策，**先于**下面的设计决策）：**

| # | 决策 | 落点 |
|---|---|---|
| **D1** | **分工边界：不越界。** 本分支只做 `internal/model/user_memory.go`、`migrate.go` 追加一行、spec/plan 文档。**`repository/` / `service/` / `handler/` 等与成员 1 对齐后再动**，避免同时改同一文件。**文档先推进** | §2.1（A/B 分表）、§5 开头、§7.1、§7.2 |
| **D2** | **JWTAuth 空壳 → 只做模型层验证。** spec 里标注 `🚧 阻塞：依赖成员 1 的 middleware`；**本地只验证 `AutoMigrate` 跑通、表结构正确、外键 / 索引正确，不做接口层测试** | §5.1 ① 注、§7.1 B、§8 分组 A/B |
| **D3** | **契约空白：只记录，不改全局文件。** 遵守 `4001`（参数错误）作为设计决定，**不动 `API_CONTRACT.md`**；把它列为**阻塞合并步骤** + 群里广播让队长拍板 | §4.3、§7.2 末行、§8 分组 C |

**设计决策（本版并入）：**

| # | 决策 | 落点 |
|---|---|---|
| 1 | 排序 `created_at DESC, id DESC`（`id` 兜底不可省） | §4.4 |
| 2 | 缺 / 非法 `personaId` → `4001`（不静默当 `0` 查） | §4.3 |
| 3 | 归属判定单独查一次（`ExistsOwnedByUser`），**不用 `len(list)` 反推** | §5.2 |
| 4 | `source_message_id` 用 `*uint64` + `ON DELETE SET NULL` | §3.4 |
| 5 | `memory_type` 用 Go 常量 + `MemoryType` 类型，**不加 DB CHECK**（DDL 没有） | §3.3 |
| 6 | `CreateBatch` 进记忆表的仓储层（与 chat-message 提供 `Create` 同一逻辑）——**作为设计交给成员 1，本分支不落地** | §4.6 |

**剩余待确认（接口层开工前在群里问清，不要自己拍板）：**

| # | 问题 | 建议 |
|---|---|---|
| 1 | **接口层由谁落地**：`memory_repo.go` / `memory_service.go` / `memory_handler.go` 按总纲 §4.2 属**成员 1** | 本分支不写（D1）。spec §4 / §5 就是给他的规格；**他若已在写，按此对齐即可**；若要本分支接手，先群里说定 |
| 2 | 列表排序契约里没写，是否就按 `created_at DESC, id DESC`？ | 按 §4.4 三条理由实施；若要按 `memoryType` 分组由前端做，**服务端不加参数** |
| 3 | **`GET /memory` 的错误码 `4001` 是否补进契约？** | 建议补（§4.3 有 `GET /schedules` 先例），**但本分支不动 `API_CONTRACT.md`**（D3）——广播后由队长拍板、契约负责人统一改 |
| 4 | **`ExtractRequest` 的确切字段**（技术文档只给了 `AIClient` 的方法签名） | 属成员 1 的 `ai_client`；`CreateBatch` 只要求 `user_id` / `persona_id` 由 Go 侧赋值（§4.6 约定 1） |
| 5 | 写入编排（"提取结果 → `CreateBatch`"）放 `memory_service.go` 还是 `chat_service.go`？ | 技术文档 §9 的目录树里有 `memory_service.go`，而成员 1 任务书的文件清单里**没有**（只有 `memory_repo.go` / `memory_handler.go`）——**两处不一致，需成员 1 定**（§5 风险） |
| 6 | `importance_score` 的 pgx 编码 | 先按 `float64` + `type:numeric(4,3)` 实现；**实测报类型不匹配**再退到自写 `driver.Valuer`（§3.2 第 5 条），**不要引入 `decimal` 包** |
| 7 | 提取阈值 0.6 与规则档固定分 0.8 | 属提取链路（成员 1）。本功能**不校验 range**（DDL 是 `NUMERIC(4,3)`，超出会由 DB 报错）——若将来要防御性校验，加在 service 而不是 repo |

## 8. 验收标准

> **按决策 1 + 2 分三组**：**A 组是本分支的验收范围**（模型层，不依赖任何人，现在就能全做完）；**B 组是接口层**，`🚧 阻塞：依赖成员 1 的 `middleware` + 端点落地`，**本分支不做**；**C 组是流程项**。
> **A 组里的外键行为（级联 / `SET NULL` / 默认值）能用纯 SQL 验证**——`AutoMigrate` 建完表，直接用 `psql` INSERT / DELETE 就能看出约束对不对，**不需要写任何 Go 代码，也不需要接口层**。这正是 persona 分支当初的做法。

### 分组 A · 模型层（✅ 本分支的验收范围，不依赖任何人）

- [x] `cd backend && go build ./... && go vet ./... && go test ./...` 全通过
- [x] **`AutoMigrate` 跑通**：`cmd/migrate` 在一个库上执行成功，`user_memory` 表被建出来（重复执行幂等）
- [x] **表结构逐列对齐 DDL**：`\d user_memory` 的 10 列，类型 / 可空性 / 默认值与 spec §3.1 逐行一致
  - `embedding_id` 可空、`embedding_status` 默认 `'pending'`、`source_message_id` **可空**、`importance_score` 是 `numeric(4,3)` 且 `NOT NULL`
- [x] **3 个外键都在，且 `ON DELETE` 各不相同**：`\d user_memory` 能看到
  - `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`
  - `FOREIGN KEY (persona_id) REFERENCES personas(id) ON DELETE CASCADE`
  - `FOREIGN KEY (source_message_id) REFERENCES chat_messages(id) ON DELETE **SET NULL**` ← **最容易漏/最容易写成 CASCADE 的一个**
- [x] **3 个索引都在**：`idx_memory_persona_type`（`persona_id, memory_type` 两列）、`idx_memory_user_id`、`idx_memory_embedding_status`
- [x] **`ID` 的 DataType 是 `bigint`**：`\d` 看到 `id bigint ... DEFAULT nextval('user_memory_id_seq')` 就是**正确**结果——Postgres 从不显示 `bigserial` 这个字（它只是 `bigint + 序列默认值` 的简写）。**真正的失败信号是别的列长出 `nextval`**，或序列清单里多出条目（见下一条）。`type:bigint` 的意义不在自己那一列，而在 `field.DataType`——GORM 建关联时只把它复制到外键列
- [x] **没有多余的序列**（关联复制污染的反面验证）：`SELECT sequencename FROM pg_sequences WHERE schemaname='public';` 里**不存在** `user_memory_user_id_seq` / `user_memory_persona_id_seq` / `user_memory_source_message_id_seq`
- [x] **级联删除（SQL 级）**：手工插一行 `user_memory` → `DELETE FROM personas WHERE id = <该行 persona_id>` → **记忆行消失**
- [x] **`SET NULL`（SQL 级，方向与上一条相反）**：手工插一行带 `source_message_id` 的记忆 → `DELETE FROM chat_messages WHERE id = <该消息>` → **记忆行还在**，且 `source_message_id IS NULL`
- [x] **`embedding_*` 默认值（SQL 级）**：插入时不写 `embedding_id` / `embedding_status` → 读出 `embedding_id IS NULL`、`embedding_status = 'pending'`
- [x] **`importance_score` 可写可读**：插一个 `0.850` 读回来还是 `0.850`（pgx `float64` → `numeric` 的编码验证，spec §3.2 第 5 条）
- [x] `migrate.go` 的 `AutoMigrate` 里 `&UserMemory{}` 排在 `&ChatMessage{}` **之后**
- [x] **代码层面**（⚠️ 每个检查都必须**针对 tag**，不能裸 grep 单词）：`user_memory.go` 的注释里**故意**写着 `bigserial` / `json:"-"` / `emotion` 来解释"为什么不能这么写"，裸 grep 会把这些**正确**的解释误判成违规——本分支实测时三条裸 grep 全部误报，所以下面全部改成锚定 tag。**下面的命令要逐条跑，不要用 `&&` 串成一条**：期望 0 的检查匹配 0 行时 `grep` 退出码为 1，链条会在第二条断掉、后四条静默不执行：
  - `grep -cE 'type:bigserial' internal/model/user_memory.go` = **0**
  - `grep -cE 'gorm:"[^"]*check:' internal/model/user_memory.go` = **0**（DDL 没有 CHECK，代码里也不能有）
  - `grep -cE '^\s*MemoryType +MemoryType +`' internal/model/user_memory.go` = **1**（字段声明为自定义 `MemoryType`；注意**不能**用 `grep "MemoryType string"`——那会命中 `type MemoryType string` 这行类型声明本身）
  - `grep -c 'gorm:"column:' internal/model/user_memory.go` = **10**
  - `grep -cE 'json:"-.{0,2}$' internal/model/user_memory.go` = **7** = **4 个数据列**（`UserID` / `EmbeddingID` / `EmbeddingStatus` / `SourceMessageID`）**+ 3 个关联字段**（`User` / `Persona` / `SourceMessage`）。⚠️ 不是 4——三个关联字段同样带 `json:"-"`
  - 从 `gorm:"column:"` 行里抽出的**对外** JSON 名恰好 **6 个**：`id` `personaId` `memoryType` `content` `importanceScore` `createdAt`。必须写成两步，**不能只写第一步**：
    ```bash
    # 第一步单独跑会输出 10 行（10 个数据列里 4 个是 json:"-"），看到 10 不等于失败
    grep 'gorm:"column:' internal/model/user_memory.go | grep -oE 'json:"[^"]*"' | grep -v 'json:"-"' | wc -l   # 期望 6
    ```
- [x] **（本分支补充验证）零值关联字段不会连带写库**（实测）：GORM `Create` 只赋 5 个标量（`UserID` / `PersonaID` / `MemoryType` / `Content` / `ImportanceScore`），三个关联字段保持零值 → `users` / `personas` / `chat_messages` 行数**不变**。这验证了 `CreateBatch` 的实际调用形态是安全的
- [x] **（本分支补充验证）pgx 的 `float64` → `numeric(4,3)` 编码可用**（实测）：经 GORM `Create` 写 `0.85`、读回 `0.85`，**不需要** §3.2 第 5 条准备的 `driver.Valuer` 兜底——阶段一保持 `float64` 即可，那条兜底留作备选
- [x] **（本分支补充验证）表名不是 `user_memories`**：`\dt` 里只有 `user_memory`。GORM 会把 `UserMemory` 复数化成 `user_memories`，`TableName()` 是**必需**的（不像另三张表那样恰好一致）
- [x] **红线 1 自查**：`git status --short` 无 `.env` / `.key` / `.pem`；验证用的 DSN 用环境变量传入，**没有写进代码或 `_test.go`**

### 分组 B · 接口层（🚧 阻塞：依赖成员 1 的 `middleware` + 端点落地，**本分支不做**）

> 交接给成员 1 的验收清单。**`JWTAuth` 落地前这一组全部无法执行**（spec §7.1 B）。

- [ ] `GET /memory?personaId=<自己的>` → `200`，`data.list` 的元素**只有 6 个字段**：没有 `userId` / `embeddingId` / `embeddingStatus` / `sourceMessageId`（`jq '.data.list[0] | keys | length'` = 6）
- [ ] **空态**：人设属于自己但一条记忆都没有 → `200` + `data.list` 是 `[]`（**不是 `null`，也不是 `4043`**）
- [ ] **越权（跨用户）**：用 B 的 Token 打 A 的 `personaId` → **`4043`**
- [ ] **不存在**：不存在的 `personaId` → **`4043`**，且 `message` 与上一条**逐字相同**（外部无法区分——这是隐藏存在性的效果）
- [ ] **不产生 `4030`**：任意输入组合（越权 / 不存在 / 缺参 / 无 Token / 超范围分页）**都不出现 `4030`**
- [ ] **缺 `personaId`** → `4001`（**不是** `200` + 空列表）
- [ ] **跨人设隔离**：同一账号两个不同人设各写记忆，各自只查到自己那份；`total` 也只算自己那份
- [ ] **`total` 正确性**：A 的 `personaId` 有 2 条时 `total` = 2（**不是全表行数**、不是别人记忆也被算进来）
- [ ] 不带 Token / Token 无效 → `4010` / `4011`（由中间件产生）
- [ ] **分页稳定**：同一批写入的 N 条记忆（`created_at` 相同）翻页不重复、不漏项（`id DESC` 兜底生效）
- [ ] `pageSize` 超上限被钳到 100；`page` 超范围返回空列表 + 真实 `total`，仍 `200`
- [ ] **唯一写入方**：全仓库 `grep -rn "user_memory" --include=*.go` 命中的**只有** `model/user_memory.go`、`repository/memory_repo.go` 与迁移/测试文件——没有第二处自己拼 INSERT 的地方
- [ ] **不该写的代码不存在**：全仓库无 `DELETE FROM user_memory`、无 `POST /memory`、无 `PUT /memory` 路由
- [ ] code 评审 grep 通过：无硬编码错误码数字/文案、无 `response.Fail` in handler、无 `"emotion"` 作为 memoryType、无 secrets

### 分组 C · 流程（阻塞合并）

- [ ] **契约空白已广播 + 由队长拍板**（决策 3）：`API_CONTRACT.md` §7 的 `GET /memory` 错误码列补 `4001`（→ `4001 4043`）、§12 登记变更记录。**本分支不直接改全局文件**——改由队长 / 契约负责人执行
- [ ] **与成员 1 的接口层分工已说定**（决策 1）：他写 / 本分支接手 / 一起写，三选一在群里留个话，避免同时改同一批文件
- [ ] PR 已开、至少 1 人 Approve；commit 格式 `<type>(<scope>): <subject>`（scope `memory`）

> **本分支的合并门槛就是「分组 A 全绿 + 分组 C 前两项有结论」**——接口层（B 组）不在本分支的验收范围内，**没有它本分支照样能合并**（它是模型层交付）。

## 9. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-15 | v1 | 创建 | 用户记忆开工前的设计与验收基线；含越权防线（双条件 + `4043` 与空态的分界）与契约空白记录（§4.3 的 `4001`） |
| 2026-09-15 | v2 | **并入三项决策（D1-D3）**：① §2.1 拆成「A 本分支落地（模型层）/ B 交接成员 1（dto·repo·service·handler）」两张表，§5 与 §7 同步标明落地范围；② §5.1 ① 加 `🚧 依赖成员 1 的 middleware`，§7.1 拆成「模型层依赖（已就绪）/ 接口层依赖（阻塞）」，**§8 验收拆成 A 模型层 / B 接口层（阻塞，本分支不做）/ C 流程 三组**，并把外键行为验证改成 SQL 级（不需要接口层）；③ §4.3 明确**只记录不改 `API_CONTRACT.md`**，列为阻塞合并步骤交队长拍板 | 用户决策 1-3：不越界、只做模型层验证、不自己动全局文件 |
| 2026-09-15 | v3 | **模型层落地 + 分组 A 全绿（17/17）**。修正了**六处会误伤正确代码的验收写法**——这些是照着文档做反而会"验证失败"的坑，全部由实跑暴露：① 三条裸 grep（`bigserial` / `json:"-"` / `emotion`）会命中注释里的解释性文字，改为锚定 tag；② `json:"-"` 的正确数量是 **7**（4 数据列 + 3 关联字段）而非 4；③ `grep "MemoryType string"` 会命中 `type MemoryType string` 类型声明本身；④ `ID` 的 DataType 判读（`\d` 显示 `bigint` + `nextval` 即**正确**，Postgres 不显示 `bigserial` 这个字）；⑤ 「从 `gorm:"column:"` 行抽对外 JSON 名 = 6」必须写成三步并排除 `json:"-"`——只跑前两步输出 **10** 行，看到 10 不等于失败；⑥ 期望 0 的 grep **不要用 `&&` 串联**：无匹配时退出码为 1，链条会在第二条断掉、后四条静默不执行，"没输出"会被误读成"全过了"。**修正一处机制写反**：`embedding_status` 的默认值是 GORM 从 tag **显式写进 INSERT**，不经 DB 默认值（§3.2 第 3 条 / §4.6 约定 3）。新增三条本分支的补充验证（零值关联不连带写库、pgx `float64→numeric` 可用、表名非复数） | 实跑结论回填：把"我以为的验收标准"改成"实际能判别的验收标准"，避免下一个人照文档做却拿到假失败 |

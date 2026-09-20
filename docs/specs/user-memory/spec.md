# spec · 用户记忆（User Memory）

> 本文件是**合同**：定义本功能做什么、做到什么算完成。合并后即冻结，任何字段或行为变更必须升版本并在群里广播。
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：`user_memory` DDL、契约字段、错误码表、通用约定一律**不复制**，只给链接；
> 本文件只写**本功能独有**的内容——独有陷阱、独有决策、独有验收项。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-user-memory-model` |
| 状态 | **模型层已落地（验收 A 组 17/17 全绿）**；接口层待与成员 1 对齐 |
| **本功能范围** | ① `internal/model/user_memory.go`；② `migrate.go` 追加一行；③ 本 spec / plan。**`repository/` / `service/` / `handler/` / `router.go` 等与成员 1 对齐后再动**（§6） |
| 负责人 | 成员 3（数据 + 人设 + 部署） |
| 关联契约 | [API_CONTRACT §7](../../API_CONTRACT.md)（`GET /memory` + MemoryItem 实体） |
| 关联设计 | [TECH_DESIGN §4.3 / §5.1 / §5.3 / §6.2 / §6.3](../../TECH_DESIGN.md)、[总纲 §0 / §6](../../dev/MASTER.md)、[成员 1 任务书 §4](../../dev/MEMBER_1_BACKEND_AI.md) |
| 关联红线 | [AGENTS §4.3](../../../AGENTS.md)（数据边界）、§4.4（情绪内部信号）、§5.1 |

---

## 0. 范围声明（先读这一节）

记忆是"像真人一样"的 AI 伴侣的**第一支撑物**：对话链路每轮结束都要用它、画像页要展示它、主动消息要基于它开口。它挂在 `persona_id` 上——**一个人设一份记忆，换人设就换一套**，因为"记得你的猫叫豆豆"是**这个伴侣**记得，不是"系统"记得。

它是核心演示链路的一环：对 A 说「我养了只猫叫豆豆」→ A 答得出；切到 B 问 → 答不出。前半句靠**写入 + 检索注入**，后半句靠**双条件隔离**。前半句错了功能不好看，后半句错了是红线 3。

### 0.1 说什么**不给**

⚠️ **记忆是只读的**：总纲 §0「记忆管理：只读展示，**不提供删除接口**」。**不要发明 `DELETE /memory/:id`**，也不要为了"测试方便"在仓储层留一个删除方法。同理没有 `POST /memory`（记忆只能由对话提取产生）、没有 `PUT /memory/:id`、没有 `GET /memory/:id`，**连预留路径都不留**。

| 不做的事 | 原因 / 去向 |
|---|---|
| **记忆提取链路**（正则、打分阈值 0.6、`memory_agent.py`） | **成员 1**。本功能只提供落库方法（§4.4），不实现正则、不实现打分、不决定"什么时候提取" |
| **`ExtractMemories` 与 `POST /memory/extract` 的调用时机** | 成员 1。**SSE 的 `done` 发出之后**异步调一次，**不要内联进流式生成过程** |
| **记忆检索与注入 Prompt**（`recent.py`） | 成员 1。阶段一走 PG、阶段二走 ChromaDB，**接口不变只换实现**。本功能不管检索 |
| **ChromaDB 写入与双写一致性** | 阶段二。`embedding_*` 两列**本期只保证类型正确、默认值正确**，不写任何索引同步代码 |
| **`GET /profile/portrait`** | 契约 §7 的另一半，属 **`user_profile`** 表，是**另一个功能**。两张表同粒度但生命周期不同，**不要顺手把它也做了** |
| **按记忆类型筛选**（`?memoryType=fact`） | 契约 §7 的请求参数**只有** `personaId` `page` `pageSize`。**不要顺手加**——那是契约漂移。`idx_memory_persona_type` 是给将来用的索引，不代表端点要暴露这个参数 |
| **情绪记忆** | `memoryType` 只有 `fact` / `preference` / `event`，**没有 `emotion`**（总纲 §0：记忆不存情绪）。情绪只是提取时的打分输入 |
| **记忆数量上限、去重 / 合并** | 契约与 DDL 都没有。**不要自作主张加"同一内容不重复写入"的校验**——重复由提取侧抑制 |
| 新增错误码 | 一个都不需要，复用 `4001` / `4043` / `5003` / `4010` |
| 手写 `ALTER TABLE` / `DropTable` | 红线 8：改结构 = 改 struct + `AutoMigrate` |

### 0.2 落地范围（分工边界）

| 组 | 内容 | 谁做 |
|---|---|---|
| **A** | `model/user_memory.go`、`migrate.go` 追加一行、本 spec/plan | **本分支**（不等任何人） |
| **B** | `dto/memory_dto.go`、`repository/memory_repo.go`、`repository/persona_repo.go` 的归属方法、`service/memory_service.go`、`handler/memory_handler.go` | **成员 1**。§4 / §5 是**交给他的设计约定**，不是本分支要落的代码 |
| **C** | `dto/common_dto.go` 的 `PageResult[T]`（**五处分页共用**） | 三个分支谁先落地谁建，后来者消费 |

---

## 1. 本表独有的硬性要求

### 1.1 三个外键，`ON DELETE` **各不相同**（最容易照抄上一份文件写错的一处）

```go
// 仅供 GORM 生成外键约束；Create 时不要赋值（保持零值），否则会连带写 users / personas / chat_messages
User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
// ⚠️ 唯一一个 SET NULL：指针关联 + 可空列成对出现
SourceMessage *ChatMessage `gorm:"foreignKey:SourceMessageID;references:ID;constraint:OnDelete:SET NULL" json:"-"`
```

只写标量字段时 GORM **一个外键都不建**——"删人设级联清空记忆"与"删消息保留记忆"两条验收项直接不成立。

**`source_message_id` 必须是 `*uint64` + `SET NULL`，不是 CASCADE**：这是本表与 `chat_messages` / `personas` 最不一样的地方。删消息不该连带删记忆——"这条记忆是从哪句话来的"只是溯源，**句子没了，记忆本身还在**。用值类型会让 `SET NULL` 写不进去（`NOT NULL` 违例）；用 CASCADE 会让"删一条消息顺手删掉一条长期记忆"，与"记忆是长期事实"的定位直接矛盾。

> ⚠️ **`ID` 的 tag 是 `type:bigint` + `autoIncrement`，不是 `type:bigserial`**——全项目已踩三次的关联复制坑（详见 [persona spec §1.3](../persona-model/spec.md)）。本表虽是这条链的末端，但阶段二 `embedding_id = memory_id` 会把主键语义抬到 ChromaDB，**照抄正确写法，不要开倒车**。

**`AutoMigrate` 顺序**：`&UserMemory{}` 必须排在 `&User{}` / `&Persona{}` / `&ChatMessage{}` **之后**——它同时引用三张表。**不能有 `persona_id`**：本表挂人设，但**不挂 `moment_id` 之类的上游**；也不要反过来把 `ChatMessage.ID` 改成 `bigserial`。

### 1.2 `memory_type`：Go 层加常量，DB 层**不加** CHECK

**DDL 上 `memory_type` 没有 CHECK 约束**（`chat_messages.role` 才有）。这**不是**技术文档漏写：

| 层 | 做法 | 理由 |
|---|---|---|
| Go | `type MemoryType string` + `MemoryTypeFact` / `MemoryTypePreference` / `MemoryTypeEvent` | 编译期挡住拼写错误（`"presonal"`）。与 `MessageRole` 同一手法 |
| DB | **不加 `check:` tag** | DDL 没有这条约束，加了就是**偏离权威 DDL**。真要加属于改 schema，得群里广播后统一改——**不是本分支能顺手做的事** |

> **代价（诚实记录）**：数据库这层没有兜底。实测手工 `INSERT` 一个 `memory_type='emotion'` **能成功**——绕过 Go 的写入可以塞进任意字符串，而前端按三分类渲染时会掉进"未知分类"分支。
> **对策不在本表**：写入方**只有一个**（§4.4 的 `CreateBatch`，由成员 1 的提取链路调用），且它接受的是 `model.MemoryType` 而不是裸 `string`；阶段二 LLM 返回的 `memory_type` 必须在**落库前**校验 / 归一（非法值丢弃并记日志，**不要原样落库**）。

### 1.3 `embedding_*` 两列本期是"空跑"列

阶段一全部为 `NULL` / `'pending'`（TECH_DESIGN §6.3：保留这两列是为了阶段二接入 ChromaDB 时**无需改表**）。

- **`embedding_id` 用 `*string`**，不要用 `string`——值类型会把 `null` 序列化成 `""`，阶段二的补偿任务就分不清"还没索引"与"索引 id 是空串"。
- **`embedding_status` 用 `string` + `default:'pending'`**，写入方**不需要显式赋值**（留零值即可）。
  ⚠️ **但别说成"靠 DDL 默认值"**：string 类型的 `default:` 会被 GORM 解析进 `DefaultValueInterface`，于是 `Create` 时这一列被**显式写进 INSERT**、**不走数据库默认值**（GORM 还会把值回写进内存结构体）。DDL 里那条 `DEFAULT 'pending'` 只是同一个值的**第二份副本**，改默认值时**两处要一起改**。GORM 侧行为已由源码确认并**实测复现**。
- 两列都 `json:"-"`：**前端物理上拿不到，从结构上杜绝误渲染**。

### 1.4 `importance_score` 是用值类型的 `float64`

它 `NOT NULL` 且是必写字段，不需要"零值 = 未设置"的语义。**它也是本表最可能触发 pgx 类型问题的一列**：若实测报类型不匹配，退到自写 `driver.Valuer` 返回字符串（零新增依赖）——**不要为此引入 `decimal` 包，也不要为此改列类型**。（实测：`Create` 写 `0.85` 读回 `0.85`，兜底方案**暂时用不上**，留着备选。）

### 1.5 表名 `user_memory`（不可数）

GORM 会把 `UserMemory` 复数化成 `user_memories`，所以 **`TableName()` 在这里是必需的**（不像另三张表那样恰好一致）。少了它会在 `AutoMigrate` 时**静默建错表**。

---

## 2. 越权防线（本功能最重要的正确性约束）

> 对应红线 3。TECH_DESIGN §4.3 把这张表点名为重灾区："记忆与画像都是一人设一份。查询必须同时带 `persona_id` 和 `user_id`。只带 `persona_id` 会跨用户串号，只带 `user_id` 会跨人设。"

**① 入口层**：`user_id` **只能从 JWT 声明取**，永不从 body / query / path / header 读。用 `c.GetUint64(middleware.ContextKeyUserID)` 读，**必须检查第二个返回值 `ok`**。取不到（`ok == false` 或值为 `0`）→ **返回 `4010` 并中断**，绝不退化成 `userID = 0` 继续查——`WHERE user_id = 0` 返回空集，**看起来没泄漏，实际是把鉴权失败伪装成了空列表**。请求 DTO 里**不声明** `userId` 字段。

**② 仓储层**：方法签名**强制**带 `userID uint64` **和** `personaID uint64`，过滤条件下沉到 SQL 的 `WHERE` 里，而不是"先全查出来再在 Go 里 `if m.UserID != userID`"——后者**多查了不该查的数据**（已经在内存里了，日志 / panic 都会带出去），且忘写那个 `if` 就静默越权。**仓储层不提供任何"按 id 单查记忆"或"按 persona_id 单查"的方法**，也不提供任何删除方法（§0.1）。

**③ 数据层**：`user_id` / `persona_id` 上的外键 + 三个索引。删人设 / 删账号的清理由外键 CASCADE 完成，**不要手写多表清理代码**。

> **本表还有第二个查询方**：`ai-service/app/memory/recent.py` 的检索器——**它也必须带两个条件**（TECH_DESIGN §5.3 已写明）。这不是本仓库的代码，但**阶段二换 ChromaDB 时同样是硬要求**：集合的 `metadata` 要存 `{user_id, persona_id, ...}`，检索时**两个过滤条件都带**后再做向量排序。只按 `persona_id` 过滤不够——它是全局自增。

### 2.1 归属校验：必须**单独查一次**，不能用 `len(list) == 0` 反推

| 情形 | 返回 |
|---|---|
| 人设属于当前用户，0 条记忆 | **`200` + `list: []`**（合法空态，新伴侣还没记住任何事）。**绝不能返回 `4043`**——那样新伴侣的记忆页会"打不开" |
| 人设**不属于**当前用户 | **`4043`** |
| 人设**不存在** | **`4043`**，与上一行**同码同文案**，外部无法区分 |

用 `persona_repo.ExistsOwnedByUser(ctx, userID, personaID) (bool, error)`——**一条查询、两个条件**（`WHERE id = ? AND user_id = ?`）。**不要**从记忆查询的结果反推归属：`len(list) == 0` 既可能是"不是你的"也可能是"还没有记忆"，**这两种情况在结果集上长得一模一样**，用它做判断必然误伤空态。

**记忆查询本身仍然要带两个条件**：归属判定是"为了让错误码正确"，不是"因为记忆查询不安全"——**两道防线各司其职，不要用前者替代后者**（否则 A 人设的记忆会被判成"归属 OK"，然后按 `persona_id` 单条件查出来）。

> **这一步是"资源越权"，不是"功能越权"**：**资源越权 → `4043`**（隐藏资源存在性）、**功能越权 → `4030`**。`GET /memory` 全程属资源越权侧，**不产生 `4030`**，也不产生 `4040`（"不存在"与"不属于你"必须同码，理由见 [persona spec §2.1](../persona-model/spec.md)）。**结构上只有一个 `if !ok` 分支**，不需要"先按 id 探针二分再判码"的两步判定。

### 2.2 缺 `personaId` → `4001`（契约空白，需广播）

契约 §7 的错误码列**只写了 `4043`**，但 `personaId` 缺失时没有任何归属可校验，"查不到 → `4043`"在语义上不成立（没有东西"不存在"）。

- ❌ **不能当作 `personaId = 0` 查库**：命中空集 → `200` + 空列表，把调用方的 bug **伪装成正常空态**（与"取不到 `userID` 不许退化成 `0`"同一条原则）。
- ✅ **返回 `4001`**（`errcode.ErrInvalidParams`）：参数缺失归 4000-4009 段；**契约 §10 的 `GET /schedules` 已有先例**（同为"必带 `personaId`"的分页端点，错误码列写的是 `4001 4043`）。

`binding` 失败**一律 `4001`**，不要细分：缺失、非数字、`0` 或负数**统一 `ErrInvalidParams`**。**不要**改成 `4002`（那是别的端点的语义）。

**动作（与本分支的代码分开）**：契约 §7 该端点的错误码列应由 `4043` 改为 `4001 4043`，并按契约纪律**登记 §12 变更记录 + 群里广播**。`API_CONTRACT.md` 是**全局文件，本分支不直接改**。

### 2.3 排序：`created_at DESC, id DESC`

DDL 没规定列表顺序，契约也没写，所以要显式定一条：

1. `created_at DESC`（新的在前）符合"最近记住的事"的直觉，也与召回排序方向一致。
2. **`id DESC` 兜底不是可选项**：`NOW()` 是**事务开始时间**，而**记忆提取是批量写入**——一轮对话提取出的 3 条记忆在**同一个事务**里插入，`created_at` **完全相同**。只按它排序时同值行顺序不确定，**翻页会出现"第 2 页冒出第 1 页的记忆"或漏项**。
3. **不要用 `importance_score` 当主排序**：阶段一规则档**所有记忆都是同一个固定分 0.8**，排序完全无区分度，且它不满足"稳定顺序"的需求。**类型分组的活儿留给前端**——`memoryType` 已经在响应里了，前端自己 `groupBy` 即可。

> **索引与排序不一致是正常的**：`idx_memory_persona_type` 是 `(persona_id, memory_type)`，用不上 `created_at` 排序。本项目每人设的记忆量级是**几十条**，排序开销可以忽略——**不要为了排序顺手加索引**（那是改 schema，得走 DDL 广播）。

---

## 3. 接口定义

### 3.1 `GET /memory`

| 项 | 内容 |
|---|---|
| 请求 | query：**`personaId`（必传）**、`page`、`pageSize`（后两个可选） |
| 响应 data | `PageResult<MemoryItem>` |
| 错误码 | `4001`（缺 / 非法 `personaId`，§2.2）**`4043`**（人设不属于当前用户，或不存在，**同一个码**，§2.1） |

**五条容易写错的要求：**

1. **`user_id` 只能来自 Token**，绝不从 query / body 取。请求 DTO 里**连这个字段都不该存在**。
2. **查询必须同时带 `persona_id` 和 `user_id`**（§2）。
3. **空列表返回 `[]` 而不是 `null`**（Go 的 nil slice 序列化成 `null`，前端 `v-for` 直接崩）。`page` 越界返回空列表 + 真实 `total`，仍是 `200`。
4. **`total` 必须用同两个条件统计**。不能只 `COUNT(*)` 全表，也不能只按 `persona_id` 统计（后者会把别人的记忆算进你的 `total`——**这是个只泄漏数量、不泄漏内容的口子，比全量泄漏更隐蔽**）。
5. **"人设属于你但一条记忆都没有"是 `200` + 空列表，不是 `4043`**（§2.1）。

> ⚠️ **query 参数的 tag 必须是 `form:"personaId"`**（不是 `json:`，也不是 `form:"persona_id"`）。Gin 的 `ShouldBindQuery` 读的是 `form` tag 且**区分大小写**——写成 `form:"persona_id"` 时 Gin **找不到匹配字段、不报错**，`PersonaID` 静默为 `0`，然后被 §2.2 的 `4001` 拦住。表现为"前端明明传了 `personaId`，接口却说缺参数"，**极难查**。

### 3.2 请求 / 响应结构

`MemoryItemResponse` **恰好 6 个字段**（`id` `personaId` `memoryType` `content` `importanceScore` `createdAt`），**没有** `userId` / `embeddingId` / `embeddingStatus` / `sourceMessageId`——契约 §7 冻结。转换函数 `NewMemoryItemResponse` 放 DTO 层，**不要写在 model 上**（model 不依赖 dto）。

### 3.3 写入方法 `CreateBatch`（**不是端点**）

记忆的写入不由本功能的端点触发——总纲 §0 明确记忆只读。唯一的写入方是**成员 1 的记忆管理 Agent 链路**：SSE 的 `done` 之后异步提取 → 超 0.6 的候选 → 落库。

```go
// 一个事务里写入一轮提取的全部记忆；memories 为空直接返回 nil（不写空事务）
CreateBatch(ctx context.Context, tx *gorm.DB, memories []*model.UserMemory) error
```

**四条交接约定：**

1. **`user_id` 与 `persona_id` 由 Go 侧赋值，不采信 AI 服务的返回体**。`/memory/extract` 的响应里只有 `{memory_type, content, importance_score}`——**没有归属字段**，也**不该有**。归属来自本轮对话所属的人设（已通过归属校验）。
2. **`source_message_id` 填本轮的用户消息 id**，可空。填了就有溯源，且删消息时外键会自动置 `NULL`——**不要为了"保住溯源"改成 CASCADE 或自己写清理逻辑**。
3. **`embedding_id` / `embedding_status` 不要赋值**（留零值，GORM 会按 tag 补上 `'pending'`；别理解成"交给数据库默认值"，见 §1.3）。阶段二接 ChromaDB 时按双轨设计：先写 PG 拿 `memory_id` → 再写 ChromaDB → **ChromaDB 失败不回滚 PG**，只标 `embedding_status = 'pending'` 交给补偿任务。
4. **`memory_type` 必须在落库前校验**（§1.2）：阶段一规则档只产出三个合法值，**阶段二 LLM 可能返回别的东西**。非法值**丢弃并记日志**，不要原样写库，也不要静默改成 `fact`。

> **本功能不决定"什么时候调用它"**：调用时机在 SSE 的 `done` 之后、异步执行，属成员 1 的 `chat_service` / `ai_client`。本功能只保证"给对了参数，这一批就原子地落进去"。

---

## 4. 验收标准

> **A 组是本分支的验收范围**（不依赖任何人，已全绿）；**B 组是接口层**，交接给成员 1；**C 组是流程项**。
> A 组的外键行为（级联 / `SET NULL` / 默认值）**能用纯 SQL 验证**——`AutoMigrate` 建完表，直接用 `psql` INSERT / DELETE 就能看出约束对不对，**不需要写任何 Go 业务代码，也不需要接口层**。

### 分组 A · 模型层（✅ 本分支，17/17 全绿）

- [x] `cd backend && go build ./... && go vet ./... && go test ./...` 全通过
- [x] **`AutoMigrate` 跑通**（`cmd/migrate` 在临时库上执行成功，重复执行幂等）
- [x] **表结构逐列对齐 DDL**：`\d user_memory` 的 10 列，类型 / 可空性 / 默认值逐行一致
- [x] **3 个外键都在，且 `ON DELETE` 各不相同**：两个 CASCADE + **一个 `SET NULL`**（最容易漏 / 最容易写成 CASCADE 的一个）
- [x] **3 个索引都在**：`idx_memory_persona_type`（**两列**，顺序不能反）、`idx_memory_user_id`、`idx_memory_embedding_status`
- [x] **没有多余的序列**：`pg_sequences` 里**不存在** `user_memory_user_id_seq` / `_persona_id_seq` / `_source_message_id_seq`（关联复制污染的反面验证）
- [x] **级联删除（SQL 级）**：插一行记忆 → `DELETE FROM personas WHERE id = <其 persona_id>` → **记忆行消失**
- [x] **`SET NULL`（SQL 级，方向与上一条相反）**：插一行带 `source_message_id` 的记忆 → `DELETE FROM chat_messages WHERE id = <该消息>` → **记忆行还在**，且 `source_message_id IS NULL`
- [x] **`embedding_*` 默认值（SQL 级）**：不写这两列插入 → 读出 `embedding_id IS NULL`、`embedding_status = 'pending'`
- [x] **`importance_score` 可写可读**：插 `0.850` 读回 `0.850`（pgx `float64` → `numeric` 编码验证）
- [x] **`&UserMemory{}` 排在 `&ChatMessage{}` 之后**
- [x] **表名不是 `user_memories`**：`\dt` 里只有 `user_memory`
- [x] **零值关联字段不会连带写库**：`Create` 只赋 5 个标量 → `users` / `personas` / `chat_messages` 行数**不变**
- [x] **代码层 tag 检查**（⚠️ **每条都必须锚定 tag**，不能用裸 `grep` 单词——注释里**故意**写着 `bigserial` / `json:"-"` / `emotion` 来解释"为什么不能这么写"，裸 grep 会把这些**正确**的解释误判成违规，本分支实测三条全误报。命令见 plan §3.3）

  | 检查 | 期望 |
  |---|---|
  | `type:bigserial` | **0** |
  | `gorm:"[^"]*check:` | **0**（DDL 没有 CHECK，代码里也不能有） |
  | 字段声明为自定义 `MemoryType` | **1**（**不能**用 `grep "MemoryType string"`——那会命中 `type MemoryType string` 类型声明本身） |
  | `gorm:"column:` | **10** |
  | **三个关联 tag 的三个 key 齐全**（`foreignKey` + `references` + `constraint:OnDelete:`） | **3** |
  | `json:"-"`（锚定行尾） | **7** = **4 个数据列** + **3 个关联字段**（⚠️ 不是 4） |
  | 从 `gorm:"column:"` 行抽**对外** JSON 名（排除 `json:"-"`） | **6** |

  > ⚠️ **`go build` / `go vet` / `gofmt` 都查不出 tag 被截断**（tag 是字符串字面量）。少了 `foreignKey` / `references`，GORM 会退回按约定推断关联、外键可能建错或建不出来，**而编译毫无反应**。本项由一次实际事故补上：某提交里 `SourceMessage` 的 tag 被截断成 `gorm:"...;constraint:OnDelete:SET NULL"`，CI 只报了同一提交里的语法错误，这个静默缺陷是核对 `\d` 时才发现的。已实测该检查能报出缺陷（修复版 = 3 / 损坏版 = 2）。
  > ⚠️ **"抽对外 JSON 名 = 6"必须写成三步并 `grep -v 'json:"-"'`**：只跑前两步会输出 **10 行**（10 个数据列里 4 个是 `json:"-"`），看到 10 **不等于失败**。
  > ⚠️ **期望 0 的 grep 不要用 `&&` 串联**：无匹配时退出码为 1，链条会在第一条断掉、后面的检查**静默不执行**，"没输出"会被误读成"全过了"。

- [x] **红线 1 自查**：`git status --short` 无 `.env` / `.key` / `.pem`；验证用的 DSN 走环境变量，**没有写进代码或 `_test.go`**

### 分组 B · 接口层（交接给成员 1）

- [ ] `GET /memory?personaId=<自己的>` → `200`，`data.list` 的元素**只有 6 个字段**（`jq '.data.list[0] | keys | length'` = 6）
- [ ] **空态**：人设属于自己但一条记忆都没有 → `200` + `data.list` 是 `[]`（**不是 `null`，也不是 `4043`**）
- [ ] **越权（跨用户）**：用 B 的 Token 打 A 的 `personaId` → **`4043`**
- [ ] **不存在**：不存在的 `personaId` → **`4043`**，且 `message` 与上一条**逐字相同**
- [ ] **不产生 `4030` / `4040`**：任意输入组合（越权 / 不存在 / 缺参 / 无 Token / 超范围分页）都不出现这两个码
- [ ] **缺 `personaId`** → `4001`（**不是** `200` + 空列表）
- [ ] **跨人设隔离**：同一账号两个人设各写记忆，各自只查到自己那份；`total` 也只算自己那份
- [ ] **分页稳定**：同一批写入的 N 条记忆（`created_at` 相同）翻页不重复、不漏项
- [ ] `pageSize` 超上限被钳到 100；`page` 超范围返回空列表 + 真实 `total`，仍 `200`
- [ ] **唯一写入方**：全仓库 `grep -rn "user_memory" --include=*.go` 命中的**只有** model / repo / 迁移与测试文件——没有第二处自己拼 INSERT 的地方
- [ ] **不该写的代码不存在**：全仓库无 `DELETE FROM user_memory`、无 `POST/PUT /memory` 路由
- [ ] code 评审 grep 通过：无硬编码错误码数字/文案、无 `response.Fail` in handler、无 `"emotion"` 作为 `memoryType`、无 secrets

### 分组 C · 流程（阻塞合并）

- [ ] **契约空白已广播 + 由队长拍板**（§2.2 的 `4001`）：`API_CONTRACT.md` §7 错误码列补 `4001`、§12 登记变更记录。**本分支不直接改全局文件**
- [ ] **与成员 1 的接口层分工已说定**：他写 / 本分支接手 / 一起写，三选一在群里留个话
- [ ] PR 已开、至少 1 人 Approve；commit 格式 `<type>(<scope>): <subject>`（scope `memory`）

> **本分支的合并门槛 = 「分组 A 全绿 + 分组 C 前两项有结论」**——接口层（B 组）不在本分支的验收范围内，**没有它本分支照样能合并**（它是模型层交付）。

---

## 5. 交接

| 交接物 | 接收方 | 用途 |
|---|---|---|
| **`internal/model/user_memory.go`**（本分支实际交付的唯一代码文件） | **成员 1** | 记忆提取链路落库要引 `UserMemory`；`migrate.go` 已把表建好 |
| **接口层的完整设计**（字段、方法签名、错误码、判定顺序） | **成员 1** | spec §2 / §3 **就是给那四份文件写的规格**（审查要点见 [plan §3.2](plan.md) B 段与 §3.4）。本分支**不写这些代码**——他若已在写，按此对齐；若要本分支接手，先群里对齐 |
| `CreateBatch(ctx, tx, memories)` 的设计（**方法本身待成员 1 落地**） | **成员 1** | SSE `done` 之后的异步提取落库，而且是记忆表**唯一的写入入口**。四个交接约定见 §3.3 |
| `SourceMessageID *uint64` + `SET NULL` | **成员 1** | 填本轮用户消息 id 做溯源；删消息时外键自动置 `NULL`，**不需要他写任何清理逻辑** |
| `embedding_id` / `embedding_status` 两列 | **成员 1 + 成员 3（阶段二）** | 接入 ChromaDB 时**无需改表**（两列已在 DDL 里） |
| `GET /memory` 的响应结构（6 字段） | **成员 2** | 写 `types/memory.ts` 与 Mock，**字段名逐字对齐 camelCase**。⚠️ **没有 `userId` / `embeddingId` / `embeddingStatus` / `sourceMessageId`**，Mock 里不要自作主张加 |
| `GET /memory` 的空态与缺参约定 | **成员 2** | `list` 是 `[]` 不是 `null`；缺 `personaId` 是 `4001` 不是空列表；**切人设必须重新请求**（记忆按人设隔离） |
| ⚠️ **契约 §7 需补 `4001`（阻塞合并）** | **队长 / 契约负责人** | §2.2 的契约空白，需登记 §12 变更记录 + 广播后由队长拍板。**本分支不直接改全局文件** |

### 5.1 待确认（接口层开工前在群里问清）

| # | 问题 | 建议 |
|---|---|---|
| 1 | **接口层由谁落地**：`memory_repo.go` / `memory_service.go` / `memory_handler.go` 按总纲属**成员 1** | 本分支不写。spec §3 / §2 就是给他的规格；**他若已在写，按此对齐即可** |
| 2 | 列表排序契约里没写，是否就按 `created_at DESC, id DESC`？ | 按 §2.3 实施；分组由前端做，**服务端不加参数** |
| 3 | **`GET /memory` 的错误码 `4001` 是否补进契约？** | 建议补（有 `GET /schedules` 先例），**但本分支不动 `API_CONTRACT.md`**——广播后由队长拍板 |
| 4 | **`ExtractRequest` 的确切字段** | 属成员 1 的 `ai_client`；`CreateBatch` 只要求 `user_id` / `persona_id` 由 Go 侧赋值（§3.3 约定 1） |
| 5 | 写入编排放 `memory_service.go` 还是 `chat_service.go`？ | 技术文档 §9 的目录树里有 `memory_service.go`，而成员 1 任务书的文件清单里**没有**——**两处不一致，需成员 1 定** |
| 6 | 提取阈值 0.6 与规则档固定分 0.8 | 属提取链路（成员 1）。本功能**不校验 range**（DDL 是 `NUMERIC(4,3)`，超出由 DB 报错） |

---

## 6. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-15 | v1 | 创建 | 用户记忆开工前的设计与验收基线；含越权防线（双条件 + `4043` 与空态的分界）与契约空白记录 |
| 2026-09-15 | v2 | **并入三项决策**：① 范围收窄到模型层（§0.2 的 A/B 分组、§5 同步标明落地范围）；② 只做模型层验证，外键行为改成 **SQL 级**验证；③ 契约空白**只记录不改全局文件** | 用户决策：不越界、只做模型层验证、不自己动全局文件 |
| 2026-09-15 | v3 | **模型层落地 + 分组 A 全绿（17/17）**。修正**六处会误伤正确代码的验收写法**（裸 grep 误报、`json:"-"` 数量是 7 不是 4、`MemoryType string` 会命中类型声明本身、`\d` 显示 `bigint + nextval` 即正确、"抽 6 个 JSON 名"必须三步、期望 0 的 grep 不能 `&&` 串联）；**修正一处机制写反**：`embedding_status` 的默认值由 GORM 显式写进 INSERT，不走 DB 默认值。新增三条补充验证（零值关联不连带写库、pgx `float64→numeric` 可用、表名非复数） | 实跑结论回填：把"我以为的验收标准"改成"实际能判别的验收标准"，避免下一个人照文档做却拿到假失败 |
| 2026-09-15 | v4 | **分组 A 增补一条 tag 完整性检查** | 实际出过事故：`SourceMessage` 的 gorm tag 被截断，`foreignKey` / `references` 丢失，而 `go build` / `vet` / `gofmt` 全部照过。**原检查组全是"查有没有违规"，没有一条查"tag 是否完整"**——而缺 key 恰恰不产生任何报错 |
| 2026-09-20 | v5 | **按 AGENTS §5.2 引用优先精简**：删除从 TECH_DESIGN / API_CONTRACT 复制粘贴的 DDL 原文、字段对照表、通用约定表与红线自查表，改为链接；分组 A 的 7 条命令收成一张表（完整命令移入 plan §3.3）（532 → 294 行，行数按 `wc -l` 含空行）。**修正一处过期事实**：原文称 `JWTAuth` 是空壳、接口层端到端验收做不了——现已确认 `middleware/jwt.go` 的 `c.Set(ContextKeyUserID, claims.UserID)` 已落地，该阻塞解除 | 文档膨胀 + 依赖状态已变化 |

# spec · 用户画像（User Profile）

> 功能名：user-profile ｜ 分支：`feature/backend-user-profile-model`
> 负责人：成员 3（数据 + 人设 + 部署） ｜ 状态：**设计完成，模型层待实现**；接口层待与成员 1 对齐
> 创建：2026-09-16 ｜ 最后更新：2026-09-16（第 1 版）
> 关联：[AGENTS.md](../../../AGENTS.md) ｜ [接口契约 §7](../../API_CONTRACT.md) ｜ [技术文档 §6.2 / §5.2 / §5.3 / §4.3](../TECH_DESIGN.md) ｜ [总纲 §0 / §4.2 / §10](../dev/MASTER.md) ｜ [成员 1 任务书 §4](../dev/MEMBER_1_BACKEND_AI.md)

> ✅ **分工边界（按 [总纲 §4.2 端点归属表](../dev/MASTER.md)）**：`GET /profile/portrait` 的**提供者是成员 1**；`internal/model/*` 属成员 3。**本分支只做三件事**：① `internal/model/user_profile.go`；② `internal/model/migrate.go` 追加一行；③ 本 spec / plan 文档。**`dto/` / `repository/` / `service/` / `handler/` / `router.go` 一律等与成员 1 对齐后再动**（避免两人同时改同一文件，AGENTS §4.8）。因此 **§4 / §5 是「交给成员 1 的设计约定」，不是本分支要落的代码**——文档先推进。与 [user-memory spec](../user-memory/spec.md) 的决策 D1 同一处理方式。
> 🚧 **阻塞：接口层端到端验收依赖成员 1 的 `internal/middleware`**（`JWTAuth` 目前是空壳，从不 `Set(ContextKeyUserID)`，见 [user-memory spec §5.1 ①](../user-memory/spec.md)）。**本分支只做模型层验证**：`AutoMigrate` 跑通、表结构 / 唯一约束 / 外键与 DDL 一致，**不做任何接口层测试**（§8 分组 A/B）。
> ⚠️ **阶段一没有任何东西会写这张表**：画像的行由 `POST /memory/extract` 的 `profile_updates` 产生，而**阶段一的规则档提取器不产出这个字段**（技术文档 §5.3 的 `RuleMemoryExtractor` 只返回 memories，`profile_updates` 出现在 §5.3 末的「LLM 提取输出契约」里，属阶段二）。**后果：画像页在阶段一默认永远是空的**。这必须在演示前解决，见 §7.3 #1——**本功能最大的风险不在代码，在这里**。
> ⚠️ **画像页是 P1，砍功能序第 ④ 位**（[总纲 §10](../dev/MASTER.md)）。但**模型层要先落**：`internal/model/*` 是成员 1 写仓储层的前置（总纲 §2）。
> ⚠️ **本功能不新增任何错误码**：复用 `4001` / `4043` / `5003` / `4010`（§6）。
> ⚠️ **画像页是只读展示**：**不提供 `PUT` / `POST` / `DELETE /profile/portrait`**。画像不是用户填的表单，是 AI 对用户的认知（§4.7）。「资料编辑」是另一个端点 `PUT /user/profile`（账号资料：头像 / 用户名），与本功能**无关**——两个路径名字很像，别搞混（§4.1 末注）。

---

## 1. 背景与目标

画像与记忆是同一件事的两个面：**记忆是"这个人说过什么"，画像是"我从这些话里总结出他是个什么样的人"**。它同样挂在 `persona_id` 上——**一个人设一份画像，换人设就换一套**（总纲 §0.2）：小暖眼里的你和小星眼里的你可以不一样，因为它们各自陪你聊过不同的内容。

**目标（一句话）**：交付 `user_profile` 的 GORM 实体，字段与契约 §7 的 `Portrait` 逐字一致；并且**任何接口都不可能读到别人的画像，也不可能把 A 人设的画像串到 B 人设**——写入路径也一样（§5.4，本表独有的、比读泄漏更危险的一处）。

**它与其他表最关键的两点不同：**

| # | 不同点 | 后果 |
|---|---|---|
| 1 | **`persona_id` 上有 `UNIQUE`**（10 张表里唯一一处） | 「一人设一份」是**数据库级**保证，不是 Go 侧的约定；也让本表成为**唯一有 UPSERT 语义**的表 |
| 2 | **它会被 UPDATE，不只是 INSERT** | 记忆是"只写只读的长期事实"，画像会被**反复覆盖**。写路径的越权不再只是"多写一行自己的数据"，而是**改写别人的行**（§5.4） |

> **为什么它不像记忆那样"只读"**：技术文档 §6.3 明确记忆是**只读的长期事实**，由抽取器写入、不予删除；而画像的定位是**AI 认知的当前快照**——`profile_updates` 每轮对话都可能刷新它。所以两张表同粒度、**生命周期却完全不同**，不要把它们合并（技术文档 §6.3 末条已就此立过规矩）。

## 2. 范围

### 2.1 做什么（In Scope）

**A. 本分支落地（模型层，不等任何人）**

| 项 | 产物 |
|---|---|
| 数据模型 | `internal/model/user_profile.go`（GORM 实体，含 **2 个外键**与 **`persona_id` 的唯一性**、**必需的 `TableName()`**） |
| 建表 | `internal/model/migrate.go` 的 `AutoMigrate` 追加 `&UserProfile{}`（顺序在 `&User{}` / `&Persona{}` 之后，见 §3.6） |
| 设计文档 | 本 `spec.md` 与 [plan.md](plan.md) |

**B. 交接给成员 1（本分支不落地，等对齐后再动）**

> 下表是**设计约定**。按 [技术文档 §8 目录树](../TECH_DESIGN.md) 的标注，画像的查询**不是新建 `profile_*.go`**，而是**落在记忆那套文件里**（`memory_handler.go # /memory 与 /profile/portrait`、`memory_repo.go # 记忆与画像查询`）——**与 user-memory 分支同一批文件**，这是本功能最大的协作风险（§5 风险表）。

| 项 | 产物 | 关键约定 |
|---|---|---|
| 请求 / 响应结构 | `internal/dto/memory_dto.go`（与记忆同文件，见 §4.5 待确认 #1） | `PortraitResponse` **恰好 3 字段**；`form:"personaId"` |
| 归属判定 | `internal/repository/persona_repo.go` 增 `ExistsOwnedByUser` | **一条查询、两个条件、只回 bool**（§5.2）；persona / chat-message / user-memory **已经三个分支声明过它**，本功能是**第四个消费方，不要再写第四份** |
| 数据访问（读） | `internal/repository/memory_repo.go` 增 `FindPortrait` | 签名强制带 `userID` **与** `personaID`；**查不到行返回 `(nil, nil)` 而不是 `gorm.ErrRecordNotFound`**（§5.2 ②） |
| 数据访问（写） | 同上增 `UpsertPortrait` | 供成员 1 的提取链路落库；**四个交接约定见 §4.6**，其中 `DO UPDATE ... WHERE user_id = ?` 是**越权兜底，不是可选项** |
| 业务逻辑 | `internal/service/memory_service.go` 增画像方法 | **先判归属、后查数据**；未命中一律 `4043`；**无行 → `200` + `{}`**（不是错）；`userID == 0` → `4010` |
| HTTP 接口 | `internal/handler/memory_handler.go` 增 `GetPortrait` | 挂 `GET /profile/portrait`；错误 `_ = c.Error(err)` 上抛 |
| 路由挂载 | `router.go` 加一行 | **不要与成员 1 同时改** |
| **画像的写入编排** | `chat_service.go` / `ai_client.go` | **阶段一是空白**（§7.3 #1）；阶段二按技术文档 §5.3 在 `/memory/extract` 的响应里带上 `profile_updates` |

### 2.2 不做什么（Out of Scope）

| 不做的事 | 归属 / 原因 |
|---|---|
| **`profile_data` 的键集合定义** | **不做**。DDL 明说"字段不固定、演进频繁"（技术文档 §6.3），键由**提取链路**决定。**⛔ 不要在 Go 侧定义 `Occupation` / `Interests` 这类固定字段的 struct**——那会让未知键在读写往返中**静默丢失**（与 `personas.state` 保留 `self_note` 同一道理，见 `jsonb.go` 的注释）。§3.4 |
| **用户手改画像的端点**（`PUT /profile/portrait`） | 契约 §7 没有；MASTER §0 也没有。画像是**AI 的认知**，不是用户的表单。**连预留路径都不留**（§4.7） |
| **画像的删除 / 重置端点** | 同上。行的生命周期只跟随账号 / 人设级联删除 |
| **画像写进 ChromaDB / 建 embedding 列** | 技术文档 §5.3 的双轨设计**只针对记忆**。画像不参与向量检索，**不要给它加 `embedding_*` 列**（那是改 schema） |
| **`GET /profile/portrait` 的分页** | 契约 §7 的请求参数只有 `personaId`，**没有 `page` / `pageSize`**。一人设一份 → 单对象端点。**不要顺手加 `PageResult`**（那是契约漂移） |
| **`profile_data` 的 GIN 索引 / 任何索引** | DDL 一条 `CREATE INDEX` 都没有（§3.5）。技术文档 §6.3 说 PG 支持 GIN，**但没说本表要建**——加了就是改 schema |
| **画像与记忆的自动合并 / 去重规则** | 属提取链路（成员 1）。本功能只提供 `UpsertPortrait`，**不决定"什么时候更新、更新哪些键"** |
| **按人设聚合情绪进画像**（如"最近情绪偏 sad"） | 情绪是内部信号（AGENTS §4.4）。**⛔ 画像里不许出现情绪标签**——它一样会被前端渲染 |
| **本功能的前端**（`api/profile.ts`、`types/profile.ts`、`views/profile/ProfileView.vue`） | **成员 2**。本功能只做后端，但要把「必传 `personaId`」「空态是 `{}` 不是 `null`」「`updatedAt` 可能为 `null`」三条对齐清楚（§4.2 / §4.4） |
| 新增错误码 | **一个都不需要新增**，复用 `4001` / `4043` / `5003` / `4010` |
| 手写 `ALTER TABLE` / `DropTable` | 红线 8：改结构 = 改 struct + AutoMigrate |

## 3. 数据模型（`user_profile`）

### 3.1 权威 DDL（技术文档 §6.2，照抄待翻译）

```sql
-- 画像表（一人设一份，与记忆同粒度）
CREATE TABLE user_profile (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    persona_id   BIGINT      NOT NULL UNIQUE REFERENCES personas(id) ON DELETE CASCADE,
    profile_data JSONB       NOT NULL DEFAULT '{}',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**注意这张 DDL 的三个"没有"**——每一处都是刻意的，不是漏写：

1. **没有 `CREATE INDEX` 语句**（10 张表里只有它和 `moment_likes` 之外少数几张没有显式索引，且 `user_id` 上**没有索引**）。见 §3.5。
2. **没有 `created_at`**。行是 upsert 出来的，"第一次写入的时间"**没有被记录**。契约也只给 `updatedAt`。
3. **没有 `memory_type` / `embedding_*` / `source_message_id` 之类的字段**。别从记忆表"顺手继承"。

### 3.2 字段对照表（DDL ↔ GORM ↔ JSON）

| DDL 列 | Go 类型 | GORM tag 要点 | JSON 字段 | 契约来源 |
|---|---|---|---|---|
| `id BIGSERIAL PK` | `uint64` | `type:bigint;primaryKey;autoIncrement` | **`"-"`** | ⚠️ 契约 Portrait **没有** `id` |
| `user_id BIGINT NOT NULL` | `uint64` | `type:bigint;not null`（**⛔ 无 index、无 autoIncrement**） | **`"-"`** | ⚠️ 契约 Portrait **没有** `userId` |
| `persona_id BIGINT NOT NULL UNIQUE` | `uint64` | `type:bigint;not null;`**`uniqueIndex`** | `personaId` | §7 |
| `profile_data JSONB NOT NULL DEFAULT '{}'` | `model.JSONB` | `type:jsonb;not null;default:'{}'`（**单引号不能省**） | `profileData` | §7 |
| `updated_at TIMESTAMPTZ NOT NULL` | `time.Time` | `type:timestamptz;not null;default:now();`**`autoUpdateTime`** | `updatedAt` | §7 |

**所有 ID 类型统一 `uint64`**（`id` / `user_id` / `persona_id`），与 `user.go` / `persona.go` / `chat_message.go` / `user_memory.go` 保持一致，不做 `uint` / `int64` 的来回转换。

**五条必须记住的结论：**

1. **响应体只有 3 个字段**：`personaId` `profileData` `updatedAt`——逐个数一遍，**没有 `id`、没有 `userId`、没有 `createdAt`**。用 `json:"-"` 关掉（与 `User.PasswordHash` / `Persona.UserID` / `UserMemory.UserID` 同一手法）。`user_id` 是越权防线的载体，不是给前端看的字段。
   - ⚠️ **`ID` 也带 `json:"-"`，这与 `user_memory` 不一样**（那里契约有 `id`）。本表的"身份"是 `personaId`（一人设一份 → `persona_id` 唯一），`id` 只是内部代理主键。**"顺手"给它补一个 `json:"id"` 就是契约漂移**——前端会多出一个它没定义的字段。
   - 于是 `json:"-"` 共 **4 处**：`ID` / `UserID` 两个数据列 + `User` / `Persona` 两个关联字段。**不是 2 处**——关联字段同样带 `json:"-"`（`user_memory` 在这里栽过一次：按 4 数过，实际 7）。
2. **`ID` 的 tag 是 `type:bigint` + `autoIncrement`，不是 `type:bigserial`**：**全项目已经踩过三次的坑**（`user.go` / `persona.go` 一轮，`chat_message.go` 一轮，`user_memory` 的注释里又复述了一轮）。GORM 建关联时会把**被引用主键的 `DataType`** 复制到外键列上（`schema/relationship.go` 只抄 `DataType` / `GORMDataType` / `Size`，不抄 `autoIncrement`），写成 `bigserial` 会让**下游**引用本表的外键长出 `DEFAULT nextval(...)`。本表目前处在引用链末端（没人引用 `user_profile.id`），**但照抄正确写法，不要开倒车**。
3. **`user_id` 上不加 `autoIncrement`**：它是外键、不是主键。带上自增会让每行自动拿到一个与用户无关的 id，**越权防线当场失效**——与 `chat_messages.user_id` / `user_memory.user_id` 同一处陷阱。
4. **`updated_at` 用 `autoUpdateTime`，不是 `autoCreateTime`**：本表会被反复更新，`updated_at` 的语义是"画像最后一次刷新的时间"。契约里它就叫 `updatedAt`，没有 `createdAt`。
   - `default:now()` 照留：DDL 里有，去掉就偏离权威 DDL。
   - ⚠️ **写入方不要手写 `UpdatedAt`**（留零值，GORM 会填）。**但 UPSERT 路径要显式给值**——`clause.OnConflict` 的 `DoUpdates` 里的字段**不走 `autoUpdateTime` 的自动填充**，靠 `.Updates()` / `.Save()` 才有。§4.6 约定 3。
5. **`profile_data` 用 `model.JSONB`（`jsonb.go`），不是 `map[string]any`、不是固定 struct**：理由与 `personas.state` 完全一致——**读写一次就会静默丢掉未知键**。§3.4。
   - ⚠️ **`model.JSONB` 的空值 `MarshalJSON` 返回 `null`**（`jsonb.go` 第 71-76 行）。"这个人设还没有画像"时若返回零值 `JSONB`，响应体会是 `"profileData": null`，前端 `Object.keys(null)` 直接崩——**必须显式给 `model.JSONB("{}")`**。这是 §4.4 的核心，也是与记忆的「空列表必须是 `[]` 而不是 `null`」**同构**的一条（那条已被证明是真会踩的坑）。

### 3.3 `persona_id` 上的 `UNIQUE`：一人设一份的**数据库级**保证

这是本表最重要的一行 tag，也是**最容易整行丢掉的一行**：`DROP` 掉它，`go build` / `go vet` / `gofmt` / CI **全部照过**，而"一人设一份"从"数据库级约束"退化成"Go 侧的约定"——并发的两次 upsert 可以建出两行，`GET` 时 `First()` 随机返回其中一行，表现为**画像时不时"回退"到旧内容**，无法解释。

**GORM 怎么写**：`gorm:"...;uniqueIndex"`（**已定案，见 §7.3 决策 2**）。`AutoMigrate` 建出一个唯一索引 `idx_user_profile_persona_id`。

**落点与 DDL 不一致是正常的，不要把"长什么样"写死进检查项**：

| 来源 | Postgres 里的对象 |
|---|---|
| DDL 的**内联** `UNIQUE` | Postgres 自动命名 `user_profile_persona_id_key`（**Constraints** 段） |
| GORM 的 `uniqueIndex` tag（**本功能采用**） | `idx_user_profile_persona_id`，`UNIQUE, btree (persona_id)`（**Indexes** 段） |
| GORM 的 `unique` tag（未采用） | `uni_user_profile_persona_id`（**Constraints** 段） |

DDL **没有给这个唯一性命名**，所以两种写法都满足 DDL 的语义。**验收只查"唯一性是否存在"，不查名字、也不查它落在哪一段**——查名字（或查错段）会让一个**正确实现**验收失败。已在真库实测确认本写法落点（见下方"实测记录"）。

> **两种写法的真实差别（记住就行，别纠结）**：`unique` 是约束、`uniqueIndex` 是索引；对 `ON CONFLICT (persona_id)` 的推断、对查询提速、对 `duplicate key` 的报错，**三者等价**（实测：唯一索引形态下报的是 `duplicate key value violates unique constraint "idx_user_profile_persona_id"` —— 索引也叫 constraint，措辞不必纠结）。选 `uniqueIndex` 的理由是它更明确地表达"这是给 upsert 用的唯一索引"。

> **实测记录（2026-09-16，一次性 Postgres 16 容器）**：`\d user_profile` 输出为
> ```
> Indexes:
>     "user_profile_pkey" PRIMARY KEY, btree (id)
>     "idx_user_profile_persona_id" UNIQUE, btree (persona_id)
> Foreign-key constraints:
>     "fk_user_profile_persona" FOREIGN KEY (persona_id) REFERENCES personas(id) ON DELETE CASCADE
>     "fk_user_profile_user" FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
> ```
> ⚠️ **外键名是 GORM 的 `fk_user_profile_<字段>`，不是 Postgres 默认的 `<表>_<列>_fkey`**——检查外键时同样**别按名字查**（写文档时我并不知道这个名字，是实测出来的）。
> ⚠️ **唯一性在这里是"索引"不是"约束"**：若按"Constraints 段必须有 UNIQUE"去验收，本实现会被判失败。

### 3.4 `profile_data` 是开放键的 JSONB

**键不固定**：技术文档 §5.3 只给了一个示例 `{"occupation": "程序员", "interests": ["跑步", "猫"]}`，**没有定义过完整键集合**；技术文档 §5.2 把它当作一段「摘要」拼进 prompt。

**三条硬要求：**

1. **Go 侧任何地方都不许把它解成固定 struct**。解一次就丢一次未知键：下一轮对话把画像读出来、合并新键、写回去，原来的键就没了——而且**悄悄没了，没有报错**。
2. **需要合并时，在 SQL 侧用 `jsonb ||` 合并，不要"读-改-写"**：读出来在 Go 里改再写回，既有丢键风险，也有**丢更新竞态**（两次提取并发时后写的那次会覆盖前一次的键）。§4.6 给了合并写法。
3. **它不参与任何内部计算**。画像只在两处出现：拼 prompt（技术文档 §5.2）与 `GET /profile/portrait` 的响应。**不要拿它做筛选、排序、聚合**——那是记忆表的事。

> **与 `personas.state` 的区别**：`state` 里的 `familiarity` 是 Go 侧要**读**出来做人格演化的（技术文档 §5.6），所以它是"开放 JSON + 一个已知可读的键"；`profile_data` 则是**纯透传**——Go 侧从头到尾一行都不用解析它。**这反而更简单**：任何一处出现 `json.Unmarshal(profile_data...)` 都值得问一句为什么。

### 3.5 外键与索引

**两个外键，且都必须是 `ON DELETE CASCADE`**，否则"删账号 / 删人设即清空其画像"两条验收项不成立。只写标量字段时 GORM **不会创建任何外键**——必须额外声明 belongs-to 关联：

```go
// 以下两个关联字段仅供 GORM 生成外键约束用，不参与序列化。
// Create 时不要给它们赋值（保持零值），否则 GORM 会尝试连带写入 users / personas 表。
User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
```

> ⚠️ **本表没有 `SET NULL`**（`user_memory` 有一个，别照抄过来）：两个外键的语义都是"宿主没了，画像就没有存在意义"。画像不是可以脱离人设独立存在的长期事实。
> ⚠️ **`foreignKey` / `references` 两个 key 一个都不能省**：少了它们 GORM 会退回按约定推断关联，可能指向别的字段或干脆建不出这条外键，而 `go build` 照样通过——**静默失败**（`user_memory` 的 `62d10fa` 正是这么坏过一次）。

**索引：一条都不加。**

| DDL 里没有的索引 | 为什么不该加 |
|---|---|
| `user_profile(user_id)` | 查询永远是 `WHERE persona_id = ? AND user_id = ?`。`persona_id` 上的**唯一索引**把候选集缩到 ≤ 1 行；`user_id` 只是**行内过滤**，加索引是纯粹的写放大。与记忆表不同——记忆是按 `persona_id` 查**多行**，且 `idx_memory_user_id` 是 DDL 里就有的 |
| `user_profile(updated_at)` | 没有任何按时间扫画像的场景（不是列表端点） |
| `profile_data` 的 GIN 索引 | 技术文档 §6.3 提到 PG 支持 GIN，**但本表没有任何按键查询的路径**。加了就是改 schema |
| `persona_id` 再建普通索引 | **重复**：`uniqueIndex` 已经有索引了，再加一个是白占空间 |

> **一句话**：本表的索引**只有** `pkey` 与 `persona_id` 上的唯一索引，`\d user_profile` 的 Indexes 段里**除 `user_profile_pkey` 和 `idx_user_profile_persona_id` 之外出现任何东西都是多余的**（实测输出见 §3.3）。

### 3.6 建表顺序与表名

- **`AutoMigrate` 顺序**：`&UserProfile{}` 必须排在 `&User{}` / `&Persona{}` **之后**——它同时引用这两张表。与 `&UserMemory{}` 之间**没有依赖**，追加在其后（最小 diff，不动别人的行）。
- **`TableName()` 是必需的**：GORM 默认把 `UserProfile` 复数化成 **`user_profiles`**，与权威 DDL 的 `user_profile` 不一致——不像 `users` / `personas` / `chat_messages` 那样恰好一致（**与 `user_memory` 同一情况**）。少了这个方法，`AutoMigrate` 会默默建出另一张表 `user_profiles`，接口层查 `user_profile` 时才报"表不存在"，而且从代码上完全看不出哪里错了。
- **表名不可数、用单数**：`user_profile`（协作规范 §5 的明确例外，与 `user_memory` 同级）。

## 4. 接口定义

### 4.1 通用约定（契约 §1 / §2）

| 项 | 约定 |
|---|---|
| Base URL | `/api/v1` |
| 鉴权 | **必须**带 `Authorization: Bearer <accessToken>`；缺失/过期 → `4010`。**不在 4 个免鉴权白名单里**（AGENTS §4.2） |
| 响应体 | `{ code, message, data, timestamp }`，由成员 1 的 `pkg/response` 统一产出 |
| 时间格式 | RFC3339 |
| 错误出口 | handler 一律 `_ = c.Error(err)` 上抛，由 `BizErrorHandler` 中间件统一出口；**handler 里不出现 `response.Fail`** |
| 分页 | **不适用**（单对象端点，无 `page` / `pageSize`） |

> ⚠️ **别和 `GET /user/profile` 搞混**（用户信息 / 账号资料）：`/user/profile` 与 `PUT /user/profile` 是**账号**的头像与用户名（成员 1，契约 §3.4 / §3.5）；`/profile/portrait` 是**人设**的画像。前端 `api/profile.ts` 同时承载两者（成员 2 任务书 §4），所以两个端点的字段名、错误码、参数**都不一样**——`/user/profile` **没有** `personaId`，`/profile/portrait` **必带** `personaId`。

### 4.2 `GET /profile/portrait` — 某个人设的画像

| 项 | 内容 |
|---|---|
| 请求 | query：**`personaId`（必传）**。**没有** `page` / `pageSize` |
| 响应 data | `{ "personaId": 1, "profileData": {}, "updatedAt": "..." }`（**恰好 3 字段**） |
| 错误码 | `4001`（缺 / 非法 `personaId`，见 §4.3）**`4043`**（人设不属于当前用户，或人设不存在，**同一个码**，见 §5.2） |

**Portrait（契约 §7 冻结）**

```json
{ "personaId": 1, "profileData": {}, "updatedAt": "2026-09-10T14:30:00+08:00" }
```

**六条容易写错的要求：**

1. **`user_id` 只能来自 Token**，绝不从 query / body 取。请求 DTO 里**连这个字段都不该存在**——字段不存在，就没有被前端传进来的可能（结构性防御，比"记得校验"可靠）。
2. **查询必须同时带 `persona_id` 和 `user_id` 两个条件**（技术文档 §4.3）：只带 `persona_id` 会**跨用户串号**（`persona_id` 全局自增，别的用户的同号人设会命中）；只带 `user_id` 在这里**恰好不会串人设**（一人设一份），但**仍然要带**——防线的写法必须与记忆一致，不能"这张表看着安全就少写一个条件"（§5.1 ②）。
3. **空画像必须返回 `{}` 而不是 `null`**，且**是 `200` 而不是 `4040` / `4043`**（§4.4）。
4. **响应体恰好 3 个字段**：不许出现 `id` / `userId` / `createdAt` / `embedding*` / `memoryType`。`jq '.data | keys | length'` = 3。
5. **"人设属于你但还没有画像行"是 `200` + `{}`，不是 `4043`**。与记忆的空态同一条纪律：**归属判定单独做一次**，不能靠"画像查不到就 4043"——否则新伴侣的画像页会"打不开"（§5.2）。
6. **单对象端点，不要套 `PageResult`**：契约里这个端点没有 `list` / `total`。

### 4.3 缺 `personaId` 时返回什么（契约空白 #1，需广播）

**问题**：契约 §7 的错误码列**只写了 `4043`**。但 `personaId` 缺失时没有任何归属可校验，"查不到 → 4043"在语义上不成立（没有东西"不存在"）。

| 方案 | 后果 | 评价 |
|---|---|---|
| 当作 `personaId = 0` 查库 → 命中空集 → `200` + `{}` | 前端漏传参数时看到"这个人设还没有画像"，**把调用方的 bug 伪装成正常空态** | ❌ 与 AGENTS §4.3「取不到 `userID` 不许退化成 `0`」同一条原则相悖 |
| 返回 **`4001`**（`errcode.ErrInvalidParams`） | 参数缺失归 4000-4009 段；**§10 的 `GET /schedules` 已有先例**（同为「必带 `personaId`」的端点，错误码列写的是 `4001 4043`） | ✅ **本功能的做法** |

**动作（与本分支的代码分开）**：契约 §7 的错误码列应由 `4043` 改为 `4001 4043`（`GET /profile/portrait` 一行），并按契约纪律**登记 §12 变更记录 + 群里广播**。`API_CONTRACT.md` 是**全局文件，本分支不直接改**——与 [user-memory spec §4.3](../user-memory/spec.md) 同一处理方式（那条至今未广播，**两条可以合并成一次广播**）。

> **`binding` 失败一律 `4001`**，不要细分：`personaId` 缺失、传了非数字、传了 `0` 或负数，**统一 `ErrInvalidParams`**。**不要**改成 `4002`（`ErrParamMissing`）——那是别的端点的语义，擅自细化就是契约漂移。

### 4.4 空画像：`{}` 与 `updatedAt: null`（契约空白 #2，需广播）

**"这个人设还没有画像"是一个合法状态，而且它是阶段一的默认状态**（§7.3 #1：阶段一没有写入方）。它必须能正常打开页面：

| 情形 | 返回 | 为什么 |
|---|---|---|
| 人设属于当前用户，有画像行 | `200` + `profileData` 为库里的内容 + `updatedAt` 有值 | 正常 |
| **人设属于当前用户，没有画像行** | **`200` + `profileData: {}` + `updatedAt: null`** | 合法空态。**绝不能返回 `4040` / `4043`**——那样新伴侣的画像页会"打不开" |
| 人设**不属于**当前用户 / **不存在** | `4043` | §5.2 |

**两条实现上的硬要求：**

1. **`profileData` 必须显式给 `model.JSONB("{}")`，不能是零值**。`model.JSONB` 的 `MarshalJSON` 对空值返回 `null`（`jsonb.go` 第 71-76 行），零值会让响应体变成 `"profileData": null`。**它照样能编译、能跑、能"演示"**，只在看响应体的那一刻才暴露，而前端 `Object.keys(null)` 直接崩。这与记忆那条「`list` 必须是 `make(..., 0)` 而不是 `var list []T`」是**同一个坑的两种形态**。
2. **`updatedAt` 需要可空**（Go 侧 `*time.Time` → JSON `null`）。契约 §7 给的示例是一个字符串，**没写它可能为 `null`**——这是**契约空白 #2**：需广播（见 §7.2 末行）。

> **为什么不"惰性建行"**（读的时候顺手 `INSERT ... ON CONFLICT DO NOTHING` 补一行）：
> ① GET 有写副作用，端点语义变浑；② 它会与提取链路的 upsert 形成并发写同一行；③ 建出来的空行与"从未提取过"在库里长得一模一样，**反而丢掉了"这个伴侣还什么都没总结过"这个信息**；④ 完全没必要——DDL 的 `DEFAULT '{}'` 让"没有行"与"空画像"在响应上可以统一成 `{}`，前端一个分支都不用多写。

### 4.5 请求 / 响应结构

| 结构 | 定义位置 | 字段 | 说明 |
|---|---|---|---|
| `PortraitQuery` | `dto/memory_dto.go` | `personaId`（`form` tag，**必须逐字 camelCase**） | 必填 → 缺失走 `4001` |
| `PortraitResponse` | 同上 | `personaId` `profileData` `updatedAt` | **3 个字段，一个不多一个不少** |
| `NewPortraitResponse(p *model.UserProfile, personaID uint64) PortraitResponse` | 同上 | — | 转换函数放 DTO 层，**不要写在 model 上**（model 不依赖 dto）；**空行分支在它这里统一兜底成 `{}` / `nil`** |

> ⚠️ **query 参数的 tag 必须是 `form:"personaId"`**（不是 `json:"personaId"`，也不是 `form:"persona_id"`）。Gin 的 `ShouldBindQuery` 读的是 `form` tag，且**区分大小写**——写成 `form:"persona_id"` 时 Gin 找不到匹配字段、**不报错**，`PersonaID` 静默为 `0`，然后被 §4.3 的 `4001` 拦住。表现为"前端明明传了 `personaId`，接口却说缺参数"。
> ⚠️ **`updatedAt` 用 `*time.Time`**，不要用值类型：值类型会把"没有画像"序列化成 `"0001-01-01T00:00:00Z"`（`persona.go` 的 `LastMessageAt` 同一条坑）。
> ⚠️ **`profileData` 的 Go 字段类型是 `model.JSONB`**，不是 `map[string]any`：`map` 会把数字变成 `float64`、把大整数精度吃掉，且空 `map` 与 `nil` 的序列化行为都要额外操心。`JSONB` 原样透传字节。

### 4.6 写入方法（`UpsertPortrait`，供画像刷新链路，**不是端点**）

画像的**写入不由本功能的端点触发**——契约 §7 没有写端点，画像由**对话提取链路**刷新（技术文档 §5.3 的 `profile_updates`）。

**签名（交接设计）**：

```go
// 按 persona_id 插入或合并；updates 为空直接返回 nil（不写空更新）
UpsertPortrait(ctx context.Context, tx *gorm.DB, userID, personaID uint64, updates model.JSONB) error
```

**四条交接约定（写给成员 1，也是本方法的使用说明）：**

1. **`user_id` 与 `persona_id` 由 Go 侧赋值，不采信 AI 服务的返回体**。`/memory/extract` 的响应里只有 `{memories, profile_updates}`——`profile_updates` 是**一堆画像键**（`{"occupation": "程序员"}`），**没有归属字段**，也**不该有**。归属来自本轮对话所属的人设（已通过归属校验）。
2. **`updates` 为空 / `{}` 时直接 `return nil`**：不要为了"保持行存在"而 upsert 一个空对象。空对象会把 `updated_at` 刷新，让"画像最近更新于"变成假信息。
3. **`updated_at` 要在 UPSERT 里显式赋值**：`clause.OnConflict` 的 `DoUpdates` **不走 `autoUpdateTime` 的自动填充**（那条路径只服务 `.Updates()` / `.Save()`）。写法见下。
4. **`DO UPDATE` 必须带 `WHERE user_profile.user_id = ?`**：这不是可选优化，是**越权兜底**——理由见 §5.4。落库方应检查 `RowsAffected`，为 `0` 时**记日志**（不返回给前端），那意味着归属校验被绕过了。

**合并式 upsert 的写法（在 SQL 侧合并，不要在 Go 里读-改-写，§3.4）**：

```go
// 一个语句同时完成"建行"与"并键"：
//   - 建行：persona_id 走 UNIQUE 推断，冲突时进入 DO UPDATE
//   - 并键：profile_data || EXCLUDED.profile_data —— jsonb 的 || 是"右边的键覆盖左边的同名键"
//   - 越权兜底：DO UPDATE ... WHERE user_profile.user_id = ? —— 不匹配则静默跳过（0 行），绝不改别人的行
//   - updated_at 显式赋值（约定 3）
err := tx.WithContext(ctx).Exec(`
    INSERT INTO user_profile (user_id, persona_id, profile_data, updated_at)
    VALUES (?, ?, ?::jsonb, NOW())
    ON CONFLICT (persona_id) DO UPDATE
       SET profile_data = user_profile.profile_data || EXCLUDED.profile_data,
           updated_at   = NOW()
     WHERE user_profile.user_id = ?`,
    userID, personaID, string(updates), userID).Error
```

> **两条命门**：① `?::jsonb` 这个转换**不能省**——用参数绑定的字符串插入 jsonb 列时 PG 需要显式转换（`jsonb.go` 的 `Value()` 返回 string 正是同一原因）；② `DO UPDATE ... WHERE` 里那个 `userID` 必须与 `INSERT` 的同一个值，**都来自 Token**。
> **事务句柄由调用方传**（`tx *gorm.DB`），与记忆的 `CreateBatch` 同一约定：一轮提取的产物（记忆 + 画像）**要么都落、要么都不落**，事务边界在 service / 提取链路。

### 4.7 明确不做的端点（**不该写的代码比该写的更重要**）

| 不写的端点 | 原因 |
|---|---|
| `PUT /profile/portrait` | 画像不是用户填的表单，是 AI 的认知（§2.2） |
| `POST /profile/portrait` | 同上；行只能由提取链路 upsert 出来 |
| `DELETE /profile/portrait` | 同上。行的生命周期只跟随账号 / 人设级联删除 |
| `GET /profile/portrait/:id` | 契约没有单条详情端点；本表按 `personaId` 定位，**不需要第二个入口** |
| 接受 `page` / `pageSize` 的分页形态 | 一人设一份 → 单对象（§4.5） |
| 任何 `/emotion/*` 相关的画像端点 | 情绪是内部信号（AGENTS §4.4）；"情绪日记"连预留路径都不留 |
| `PUT /user/profile` 的改动 | **另一个功能**（账号资料，成员 1）。路径相邻，但**不属于本功能** |

## 5. 越权防线（本功能最重要的正确性约束）

> 对应红线 3：**❌ 跨用户 / 跨人设数据泄漏**。技术文档 §4.3 把这张表点名为重灾区："记忆与画像都是一人设一份。查询必须同时带 `persona_id`（隔离人设）和 `user_id`（越权防线，从 Token 取，前端永不传）。只带 `persona_id` 会跨用户串号，只带 `user_id` 会跨人设。"

> 📌 **本节的落地范围**：这些防线里**只有数据层（§3.5 的 `persona_id` 唯一约束 + §3.5 的两个外键）由本分支落地**——它写在 `model/user_profile.go` 里。**入口层与仓储层的代码属成员 1**，本节因此是**交接给成员 1 的设计约定**与**审查清单**。

### 5.1 三层防线

**① 入口层（Handler）——`user_id` 只有一个来源**

- `user_id` 只能从 JWT 声明取（成员 1 的 `JWTAuth` 中间件写入 `gin.Context`，键名 `middleware.ContextKeyUserID` = `"userId"`），**永不从 body / query / path / header 读**。
- **类型是 `uint64`**；用 `c.GetUint64(middleware.ContextKeyUserID)` 读，**必须检查第二个返回值 `ok`**。
- 取不到（`ok == false` 或值为 `0`）→ **返回 `4010`，直接中断**，绝不退化成 `userID = 0` 继续查。`WHERE user_id = 0` 会返回空集——**看起来没泄漏，实际是把鉴权失败伪装成了空画像**。
- 请求 DTO 里**不声明** `userId` 字段：字段不存在就无法被传入。

> 🚧 **这一层目前不可用**：`internal/middleware/jwt.go` 的 `JWTAuth` **是空壳**（函数体只有 `c.Next()`）。按上一条，接口会稳定返回 `4010`——**这是"没实现"而不是"防住了"，不要把它当成防线生效的证据**。与成员 1 对齐时第一条要确认的就是 `c.Set(ContextKeyUserID, claims.UserID)`（`pkg/jwt.Claims.UserID` 已经是 `uint64`，`c.GetUint64` 能直接取到，**不要写成 `int(...)`**）。

**② 仓储层（Repository）——让"不安全的查询"在结构上不存在**

- 方法签名**强制**带 `userID uint64` **和** `personaID uint64`：`FindPortrait(ctx, userID, personaID)`。调用方没有"忘了传"的选项。
- 过滤条件**下沉到 SQL 的 `WHERE` 里**，而不是"先按 `persona_id` 查出来、再在 Go 里 `if p.UserID != userID`"。后者有两个问题：多查了不该查的数据（**已经在内存里了，日志/panic 都会带出去**）；以及**忘写那个 `if` 就静默越权**。
- **查不到行返回 `(nil, nil)`，不要返回 `gorm.ErrRecordNotFound`**：本表"查不到"是**合法空态**（§4.4），把它当错误抛出去，service 就得靠 `errors.Is` 反推——那条路上任何一处漏判都会把空态变成 `5003`。**仓储层直接把它翻译成"没有"**。
- **仓储层不提供任何"按 id 单查画像"或"按 persona_id 单查（不带 user_id）"的方法**。少一个方法就少一处泄漏面。**尤其不能有 `FindByPersonaID`**——那个名字天生只带一个条件，写出来早晚有人用。
- **写路径同理**：`UpsertPortrait` 的签名强制带 `userID`，且 SQL 里 `DO UPDATE ... WHERE user_id = ?`（§4.6 约定 4）。
- 事务句柄作为显式参数传入（`tx *gorm.DB`），事务边界由调用方决定。

**③ 数据层（DB）与外层**

- `persona_id` 的 `UNIQUE`（一人设一份的**数据库级**保证，也顺带成为读路径的索引）与两个 `ON DELETE CASCADE` 外键（§3.3 / §3.5）。
- 删人设 → 外键级联清空其画像（验收项）；删账号 → 同理。**不要手写多表清理代码**。
- **本表还有第二个访问方**：成员 1 的提取链路（写）。它不是端点，但**同样必须带两个条件**——写路径的越权比读泄漏更危险（§5.4）。

### 5.2 归属校验：`4043` 与「空画像」的分界线（最容易写错的一处）

`GET /profile/portrait` 是一次"先判归属、再查数据"的**两步**操作，因为返回码取决于**人设**而不是**画像行**：

| 情形 | 返回 | 为什么 |
|---|---|---|
| 人设属于当前用户，有画像行 | `200` + 该行内容 | 正常 |
| **人设属于当前用户，没有画像行** | **`200` + `{}` + `updatedAt: null`** | 合法空态。**绝不能返回 `4043`** |
| 人设**不属于**当前用户 | **`4043`** | 资源越权。**不返回 `4030`**（§5.3） |
| 人设**不存在** | **`4043`** | 与上一行**同码同文案**，外部无法区分（这正是要的效果） |
| `personaId` 缺失 / 非法 | `4001` | §4.3 |

**实现上的两条硬要求：**

1. **归属判定必须单独查一次**，用 `persona_repo.ExistsOwnedByUser(ctx, userID, personaID) (bool, error)`——**一条查询、两个条件**（`WHERE id = ? AND user_id = ?`）。**不要**从画像查询的结果反推归属：行查不到既可能是"不是你的"也可能是"还没总结过"，**这两种情况在结果集上长得一模一样**，用它做判断必然误伤空态。
   > ⚠️ **本表比记忆更隐蔽**：记忆那版至少有 `total` 这个额外信号；画像连"条数"都没有，**空态与越权在返回值上是完全同一件事**（都是"没查到行"）。所以这一步是**强制**的，不是"更稳妥些"。
2. **画像查询本身仍然要带两个条件**（§5.1 ②）。归属判定是"为了让错误码正确"，不是"因为画像查询不安全"——**两道防线各司其职，不要用前者替代后者**。

> **这一步是"资源越权"，不是"功能越权"**：按 [总纲 §4 的通用错误码规则](../dev/MASTER.md)（队长 2026-09-13）：**资源越权 → `4043`**（隐藏资源存在性）、**功能越权 → `4030`**。本功能全程属资源越权侧，**不产生 `4030`**。

### 5.3 "不存在"与"不属于你"必须同码同文案

`persona_id` 是 `BIGSERIAL` 全局自增。若"不属于你"回 A 码、"不存在"回 B 码，调用方拿同一个 id 各打一次就能判断这个 id **到底存不存在**——隐藏存在性的目的当场失效。

所以**同一个端点内这两种情形只有一个答案**：`errcode.New(errcode.ErrPersonaNotFound)`（`4043`，"人设不存在"）。**结构上就只有一个 `if !ok` 分支**。

**代价（诚实记录）**：调试时"看别人人设的画像"与"看不存在的"返回同一个码，排查略麻烦——靠服务端日志区分（`BizErrorHandler` 会为 4xxx 记 warn 日志，含 `traceId`）。

### 5.4 写路径的越权：**本表独有的一处，也是本功能最危险的一处**

**这是画像与记忆最本质的差别，也是写这份 spec 最想说清的一件事。**

记忆的写入是 `INSERT`：最坏情况是"多写了一行"，而且归属字段由 Go 侧赋值、还经过了归属校验。

**画像的写入是 `UPSERT`（`ON CONFLICT (persona_id) DO UPDATE`），冲突时会命中一行已存在的记录并改写它。** 如果归属校验被绕过、写错、或被将来某次重构改坏，后果不是"多读到一条数据"，而是：

> **别人的画像被覆盖，且原来的内容不可恢复**（`profile_data` 是整体合并覆盖，没有历史版本）。

**四个必须理解的要点：**

1. **`persona_id` 上的 `UNIQUE` 不提供任何越权保护**。它是"一人设一份"的保证，不是"只能写自己的"的保证——`user_id` **不在唯一键里**（DDL 就是这样），所以冲突判定只看 `persona_id`。**保护完全来自 WHERE 条件与归属校验**，没有第二道隐式防线可依赖。
2. **`user_id` 只能来自 Token**（§5.1 ①）。如果它来自请求体，攻击者传 `{personaId: 别人的, userId: 别人的}` 就能直接改写别人的画像——而**响应还是 `200`**，因为 upsert 成功了。
3. **`DO UPDATE ... WHERE user_profile.user_id = ?` 是第二层兜底**（§4.6 约定 4）。它保证"即使归属校验被改坏，SQL 也不会命中别人的行"——不匹配时**静默跳过（0 行受影响）**，失败方向是"没写进去"，不是"写错了地方"。落库方检查 `RowsAffected == 0` 并记日志。
4. **不要用"先按 id 查出来、在 Go 里比 `UserID`、再 `Save`"的写法**：既有 TOCTOU（查与写之间别人可能改了行），也有"忘写那个比较就静默越权"的问题——而且这条路径下**它看起来逻辑完整**，code review 很容易放过。

> **为什么值得单独写一节**：读泄漏是"看错了"，写覆盖是"**改坏了**"。风险等级不同，防线的写法也不同——记忆那份 spec 只能写"两个条件"，本表还要多写这一层。**如果这次只照抄记忆的检查清单，这一条会被整个漏掉。**

### 5.5 反例清单（AI 最容易写出来的错法）

| ❌ 错法 | 后果 | ✅ 正确 |
|---|---|---|
| 只 `WHERE persona_id = ?` | **跨用户泄漏**：`persona_id` 全局自增，别的用户同号人设的画像被读出来 | `WHERE persona_id = ? AND user_id = ?` |
| `persona_id` 上少写 `uniqueIndex` tag | 「一人设一份」退化成 Go 侧约定，并发 upsert 可建出两行，`First()` 随机返回一行 → 画像"时不时回退"，且**没有任何编译/静态检查会报警**。**已实测**：删掉该索引后，换一个 `user_id` 就能给同一 `persona_id` 插进第二行（`GROUP BY persona_id HAVING count(*)>1` 出现 count=2） | `gorm:"...;not null;uniqueIndex"`（§3.3） |
| `UpsertPortrait` 的 `DO UPDATE` 不带 `WHERE user_id = ?` | 归属校验一旦被绕过就**覆盖别人的画像**（§5.4） | 兜底 WHERE + 检查 `RowsAffected` |
| `DO UPDATE` 里漏了 `updated_at = NOW()` | 以为 `autoUpdateTime` 会管——它**只管 `.Updates()` / `.Save()`，不管 `OnConflict`**。表现为"画像变了但 `updatedAt` 还是旧的" | 显式赋值（§4.6 约定 3） |
| `db.First(&p, id)` 再在 Go 里比 `UserID` | 忘写比较就静默越权；数据已经进了内存 | 仓储层不提供单查方法 |
| 用 `len(...) == 0` / "查不到行" 判越权 | 把"还没总结过"误报成 `4043`，新伴侣的画像页打不开。**本表比记忆更隐蔽——它连条数都没有** | 归属单独查一次（§5.2） |
| 未命中时返回 `4030` | 违反通用规则（`4030` 是**功能越权**的码，本模块无该场景）；且 `4030`/`4043` 并存即可被二分探测出 id 是否存在 | 统一 `4043` |
| 从 query / body 读 `userId` | 攻击者传谁的 id 就看谁、**写谁**的数据 | 只从 JWT 上下文取 |
| 取不到 `userID` 时用 `0` 兜底继续查 | 鉴权失败伪装成空画像 | 返回 `4010` 并中断 |
| 缺 `personaId` 时按 `0` 查库、返回 `200` + `{}` | 前端漏传参数被伪装成正常空态，bug 藏到联调后期 | `4001`（§4.3） |
| 空画像返回零值 `model.JSONB` | **`"profileData": null`**，前端 `Object.keys(null)` 崩（`JSONB.MarshalJSON` 对空值返回 `null`） | 显式 `model.JSONB("{}")`（§4.4） |
| 空画像返回 `4040` / `4043` | 新伴侣的画像页打不开（阶段一**所有**人设都走这条路径） | `200` + `{}` |
| `updatedAt` 用 `time.Time` 值类型 | 没有画像时序列化成 `"0001-01-01T00:00:00Z"`，前端显示"0001 年" | `*time.Time` |
| 把 `profile_data` 解成固定 struct 或 `map[string]any` | 未知键在读写往返中**静默丢失**；`map` 还会吃掉大整数精度 | `model.JSONB` 原样透传（§3.4） |
| 在 Go 里"读-改-写"合并画像 | 丢键 + 丢更新竞态（并发提取时后写的覆盖前写的键） | SQL 侧 `jsonb \|\|`（§4.6） |
| `user_id` 上加了 `index:` tag | 偏离权威 DDL（DDL 没有这条索引），且 `AutoMigrate` 会**真的把索引建出来**——"代码与文档不符"变成"库里多了个对象" | 不加（§3.5） |
| `ID` 上补了 `json:"id"` | 契约漂移：响应多一个前端没定义的字段 | `json:"-"`（§3.2 第 1 条） |
| 顺手加 `PUT` / `DELETE /profile/portrait` | 违反总纲 §0 的定位（画像是 AI 的认知，不是用户的表单）；写路径还要再防一次越权，白增泄漏面 | 不写（§4.7） |
| 画像里塞情绪标签（`mood` / `recentEmotion`） | 违反"情绪是内部信号"（AGENTS §4.4），且会被前端渲染出来 | 键由提取链路决定，**不含情绪**（§2.2） |
| 把画像写进 ChromaDB / 加 `embedding_*` 列 | 技术文档 §5.3 的双轨**只针对记忆**；加列是改 schema | 不写（§2.2） |

## 6. 硬性约束（违反即不通过）

| 约束 | 来源 |
|---|---|
| 字段名**全 camelCase**，与契约 §7 逐字一致（`profileData` 不是 `profile_data`，`updatedAt` 不是 `updated_at`） | 协作规范 §5 / 契约 §1 |
| **响应体恰好 3 字段**：`personaId` `profileData` `updatedAt`；`id` / `userId` 都不进响应体 | 契约 §7 |
| **查询与写入都必须同时带 `persona_id` 与 `user_id`**；`user_id` 只从 Token 取 | **红线 3** / AGENTS §4.3 / 技术文档 §4.3 |
| **`persona_id` 上的 `uniqueIndex` tag 必须在**（一人设一份的 DB 级保证，也是 upsert 冲突推断的依据） | 技术文档 §6.2 / §3.3 |
| **`UpsertPortrait` 的 `DO UPDATE` 必须带 `WHERE user_profile.user_id = ?`** | §5.4 |
| **不提供画像的 `PUT` / `POST` / `DELETE` 端点**（连预留路径都不留） | 总纲 §0 / 契约 §7 |
| **不新增错误码**；本功能只用 `ErrInvalidParams`(4001) / `ErrPersonaNotFound`(4043) / `ErrDBFailed`(5003) / `ErrUnauthorized`(4010) | 技术文档 §4.3 分段规则 |
| **资源越权 → `4043`**，本模块**不产生 `4030`**；`userID == 0` 一律 `4010` | 队长规则 2026-09-13 / §5.2 |
| **禁止硬编码错误码数字或文案**；`Fail()` 只接受 `ErrorCode` | **红线 6** |
| handler **不写** `response.Fail`，错误 `_ = c.Error(err)` 上抛 | 技术文档 §4.4 |
| handler 不直接操作 DB；service **不依赖 `*gin.Context`**（只收 `context.Context`） | **红线 7** / 技术文档 §4.1 |
| 写入的事务边界在**调用方**（service / 提取链路），repo 只接受事务句柄 `tx` | 技术文档 §4.1 / 红线 7 |
| **`ID` 的 tag 是 `type:bigint` + `autoIncrement`**，不是 `type:bigserial` | §3.2 第 2 条 |
| **`user_id` 上不加 `index:`**、不加 `autoIncrement`；**本表不加任何 DDL 之外的索引** | §3.5 |
| `profile_data` 用 `model.JSONB`，**不加 `check:`**、不定义固定 struct、不做读-改-写合并 | §3.4 |
| `updated_at` 用 `autoUpdateTime`（不是 `autoCreateTime`），类型 `time.Time`；**UPSERT 里显式赋值** | §3.2 第 4 条 / §4.6 |
| `updatedAt` 在 Go DTO 里是 `*time.Time`；`profileData` 空态必须是 `{}` 而不是 `null` | §4.4 |
| **不新增 `created_at` 列**（DDL 没有） | §3.1 |
| 两个外键都是 `ON DELETE CASCADE`（**本表没有 `SET NULL`**） | 技术文档 §6.2 |
| 改表结构只改 struct + `AutoMigrate`，**禁止手写 `ALTER TABLE` / `DropTable`**；`AutoMigrate` 顺序在 `&Persona{}` 之后 | **红线 8** / §3.6 |
| 表名 `user_profile`（**不可数、单数**）、列名小写下划线；Go 缩写词全大写（`UserID` 不是 `userId`）；ID 一律 `uint64` | 协作规范 §5 |
| 分支 `feature/backend-user-profile-model`，走 PR，**禁止直推 `main` / `develop`**；commit 格式 `<type>(<scope>): <subject>`，scope 用 **`user`**（见下方说明） | **红线 2** / 协作规范 §6 |
| **不提交任何密钥**：本功能不新增任何 Key/Token/密码；禁止把 DSN/密码写进代码或 `_test.go` | **红线 1** |
| **契约空白（§4.3 的 `4001`、§4.4 的 `updatedAt: null`）未广播前，不得合并 PR** | 契约纪律 |

> **commit scope 的说明**：AGENTS §6 的 scope 取值表里没有 `profile`，最贴近的是 **`user`**（账号与画像同属用户维度），其次是 `model`（只改 model 层时）。**用 `user`**，不要自己发明 `profile`——scope 表是约定，不是自动补全。

**红线自查（AGENTS.md §7 逐条对照）**

| 红线 | 本功能是否涉及 |
|---|---|
| ① 提交 `.env` / Key / 密码 / 模型权重 | 不涉及新增。**测试代码里也不许出现密码** |
| ② 直推 `main` / `develop` / force push | 流程约束：本分支走 PR，至少 1 人 Approve |
| ③ 跨用户 / 跨人设泄漏 | **主要战场**，见 §5。**并且本表多一个写路径覆盖风险（§5.4）** |
| ④ 明文 / 弱哈希存密码 | 不涉及（不碰密码） |
| ⑤ 情绪展示标签 / 朋友圈手动触发 / 日程新建端点 | **不涉及，且本表是"画像不含情绪"的落点**：`profile_data` 里不许出现情绪键（§2.2） |
| ⑥ 硬编码错误码或文案；`Fail()` 传自定义 message | 见 §6 上表 |
| ⑦ handler 直接操作 DB；service 依赖 `*gin.Context`；组件写 axios | 见 §6 上表 |
| ⑧ 生产 `DropTable`；手写 `ALTER TABLE` | 见 §6 上表 |

## 7. 依赖与交接

### 7.1 我依赖谁

**A. 本分支（模型层）——依赖已全部就绪，可以立即开工**

| 依赖 | 提供方 | 现状（2026-09-16） | 影响 |
|---|---|---|---|
| `gorm.io/gorm` + `gorm.io/driver/postgres` | — | ✅ 已进 `go.mod` | `model/` 与 `cmd/migrate` 可编译 |
| `internal/model/jsonb.go` | 成员 3 | ✅ **已落地** | `ProfileData model.JSONB` 直接复用，零新增依赖 |
| `internal/model/user.go`、`persona.go`、`migrate.go` | 成员 3 / 成员 1 | ✅ **已落地** | 两个外键关联引用 `User` / `Persona`，**都在仓库里**，不需要等任何人 |
| `cmd/migrate`（独立的建表命令，读 `SCHEMA_CHECK_DSN`） | 成员 3 | ✅ **已落地** | 模型层验证靠它跑 `AutoMigrate` |
| `internal/model/user_memory.go` | 成员 3 | ✅ **已落地**（`f3c7d08`） | 无代码依赖，但它的 spec 是本 spec 的写法基准与**教训来源**（§8 分组 A 的多条检查直接沿用） |

**B. 接口层（交接给成员 1，本分支不落地）**

| 依赖 | 提供方 | 现状 | 影响 |
|---|---|---|---|
| **`middleware.JWTAuth`** | 成员 1 | 🚧 **是空壳**（`jwt.go` 只有 `c.Next()`） | **越权防线入口层取不到 `userID`**：handler 会稳定走 `4010` 分支。**接口层端到端验收（§8 分组 B）在它实现前做不了**——与 user-memory 分支同一个硬阻塞 |
| `pkg/errcode`、`pkg/response`、`internal/config`、`pkg/logger` | 成员 1 | ✅ 已落地 | service / handler 可编译（一旦决定落地它们） |
| **`repository/memory_repo.go`** | 成员 1（user-memory 分支已声明） | ⚠️ **尚未落地** | **画像的查询要加进这个文件**（技术文档 §8）。它同时是 user-memory 分支的目标文件——**两个分支改同一个文件**，见 §7.3 #2 |
| `repository/persona_repo.go` 的 `ExistsOwnedByUser` | 成员 1（三个分支已声明） | ⚠️ **尚未落地** | 本功能是**第四个消费方**，**直接消费，不要写第四份** |
| `router.go` 的挂载点 | 成员 1 | ⚠️ **尚未落地**（仓库中无 `router.go` / `cmd/server`） | 端点不可达；等它落地后加一行，**不要与成员 1 同时改** |
| **`/memory/extract` 的 `profile_updates`** | 成员 1（`ai_client` + ai-service） | ⛔ **阶段一不存在** | **画像这张表在阶段一没有任何写入方**——这是本功能最大的风险，见 §7.3 #1 |

> **本分支能做的与不能做的**：
> - ✅ **能做**：`model/user_profile.go` + `migrate.go` 追加一行 + 文档；**在真库上验证表结构**（`AutoMigrate` 跑通、`persona_id` 唯一性、2 个外键的 `CASCADE`、`DEFAULT '{}'`）、**用纯 SQL 验证 UPSERT 的合并与越权兜底**（`ON CONFLICT ... DO UPDATE ... WHERE` 不需要任何 Go 业务代码，也不需要接口层——`user_memory` 分支已经证明这条路走得通）。
> - ❌ **不做**：`dto/` / `repository/` / `service/` / `handler/` 的任何代码（表格 B）。
> - ⛔ **不要为了"跑起来"自己造一份 `router.go`、`JWTAuth`、`pkg/errcode` 或 `PageResult`**——那几样都是别人的文件。

### 7.2 谁依赖我

| 交接物 | 接收方 | 用途 |
|---|---|---|
| **`internal/model/user_profile.go`（本分支实际交付的唯一代码文件）** | **成员 1** | 画像的读 / 写仓储都要引 `UserProfile`；`model/migrate.go` 已把表建好 |
| **接口层的完整设计**（`PortraitResponse` / `FindPortrait` / `UpsertPortrait` / 归属判定顺序 / 错误码） | **成员 1** | spec §4 / §5 + plan §3 **就是给这几份文件写的规格** |
| **`UpsertPortrait` 的四条约定 + `DO UPDATE ... WHERE user_id = ?`** | **成员 1** | 画像表**唯一的写入入口**，且是**唯一有覆盖风险**的一处（§5.4） |
| **`persona_id` 的唯一索引与 `ON CONFLICT (persona_id)` 的搭配** | **成员 1** | upsert 的冲突推断依赖它；**删了它，upsert 会退化成"每次插一行"** |
| `GET /profile/portrait` 的响应结构（`Portrait` 3 字段） | **成员 2** | 写 `types/profile.ts` 与 Mock；**字段名逐字对齐 camelCase**。⚠️ 特别提醒：**没有 `id` / `userId` / `createdAt`**，Mock 里不要自作主张加 |
| `GET /profile/portrait` 的空态约定 | **成员 2** | ① `profileData` 是 `{}` 不是 `null`；② **`updatedAt` 可能是 `null`**（还没有画像时），页面要显示"还没有总结出画像"而不是"1970 年"；③ **切人设必须重新请求**（画像按人设隔离） |
| ⚠️ **契约 §7 需补两处（阻塞合并）** | **队长 / 契约负责人** | ① `GET /profile/portrait` 错误码列 `4043` → `4001 4043`；② 明确 `updatedAt` 可为 `null`。需登记 §12 变更记录 + 广播。**本分支不直接改全局文件**（§4.3 / §4.4） |
| ⚠️ **阶段一画像的演示兜底方案** | **队长 + 成员 1** | §7.3 #1。**这是产品决策，不是代码问题** |

### 7.3 决策记录与剩余待确认

**设计决策（本版）：**

| # | 决策 | 落点 |
|---|---|---|
| 1 | **响应体恰好 3 字段**，`ID` 也带 `json:"-"`（与 `user_memory` 不同——那里契约有 `id`） | §3.2 第 1 条 |
| 2 | **`persona_id` 用 `uniqueIndex` tag**（**2026-09-16 定案**：明确表达"给 upsert 用的唯一索引"）；验收只查"唯一性是否存在"，**不查名字、也不查它落在 Constraints 段还是 Indexes 段** | §3.3 |
| 3 | **`profile_data` 用 `model.JSONB` 透传**，Go 侧不解析、不定义键 | §3.4 |
| 4 | **不加任何索引**（`user_id` 也不加），理由是唯一约束已把候选集缩到 ≤1 行 | §3.5 |
| 5 | **空画像 → `200` + `{}` + `updatedAt: null`**，**不做惰性建行** | §4.4 |
| 6 | **缺 / 非法 `personaId` → `4001`**（不静默当 `0` 查） | §4.3 |
| 7 | **归属判定单独查一次**（`ExistsOwnedByUser`），不用"查不到行"反推 | §5.2 |
| 8 | **`UpsertPortrait` 在 SQL 侧合并（`jsonb \|\|`）+ `DO UPDATE ... WHERE user_id = ?` 兜底** | §4.6 / §5.4 |
| 9 | **画像**（读 / 写）**落在 `memory_repo.go` / `memory_handler.go`**，不新建 `profile_*.go` | §2.1 B / [技术文档 §8](../TECH_DESIGN.md) |

**剩余待确认（接口层开工前在群里问清，不要自己拍板）：**

| # | 问题 | 建议 |
|---|---|---|
| **1** | **`profile_data` 在阶段一由谁写？** 阶段一的规则档提取器不产出 `profile_updates`（技术文档 §5.3），**画像页在阶段一恒为空** | **本功能最大的风险，必须有人拍板**。三个选项：① **演示兜底灌数据**（与朋友圈同一个手法，技术文档 §5.5"演示前灌好一批历史动态作为兜底"）——**成本最低，推荐**；② 让规则档顺带产出 `profile_updates`（把 `FACT_PATTERNS` 命中的槽位映射成画像键，属 ai-service / 成员 1，约十几行）；③ 阶段一不解决，画像页接受"演示时是空的"。**注意：画像页在砍功能序第 ④ 位（总纲 §10），如果时间紧它可能本来就不演示**——那么 ③ 其实是可接受的答案。**⛔ 唯一不能接受的选项是"在 Go 侧从记忆里现编一份画像"**——那是伪造数据，且与"画像由 AI 总结"的定位矛盾 |
| 2 | **画像的读 / 写加进 `memory_repo.go` 与 `memory_handler.go`（技术文档 §8 如此标注），但这两个文件同时是 user-memory 分支的目标文件** | **群里确认谁先合、谁后加**（AGENTS §4.8：不要多人同时改同一文件）。若 user-memory 分支先合，本功能在它之上追加方法即可；若并行，**必须错开时间并先同步各自的函数名单** |
| 3 | **画像的 DTO 放哪儿**：技术文档 §8 的 `dto/` 目录树里既没有 `memory_dto.go` 也没有 `profile_dto.go` | 建议**放 `dto/memory_dto.go`**（与 repo / handler 同源，三处共享同一套归属判定与错误码）。但**三处必须是同一个决定**——不能 repo 在 `memory_repo.go` 而 dto 在 `profile_dto.go` |
| 4 | **`GET /profile/portrait` 的错误码 `4001` 与 `updatedAt: null` 是否补进契约？** | 建议补（§4.3 有 `GET /schedules` 先例）。**本分支不动 `API_CONTRACT.md`**——广播后由队长拍板、契约负责人统一改。可与 user-memory 那条 `4001` **合并成一次广播** |
| 5 | **`/memory/extract` 的响应结构由谁定**（要加 `profile_updates`） | 属成员 1 的 `ai_client` + ai-service。`UpsertPortrait` 只要求归属字段由 Go 侧赋值（§4.6 约定 1），**不要求 AI 返回归属** |
| 6 | **记忆与画像是否同一个事务** | 建议**同一个**：一轮提取的产物（记忆 + 画像）要么都落、要么都不落，避免"记住了但画像没更新"这种无法解释的半批状态。但这是成员 1 的编排决定（§4.6 末注） |
| 7 | **`profile_data` 的键有没有上限 / 是否要校验** | DDL 是 `JSONB`，没有长度约束；`NUMERIC` 那样的精度问题也不存在。**建议不加校验**——键由提取链路决定，本功能只负责透传 |

## 8. 验收标准

> **分三组**：**A 组是本分支的验收范围**（模型层，不依赖任何人，现在就能全做完）；**B 组是接口层**，`🚧 阻塞：依赖成员 1 的 `middleware` + 端点落地`，**本分支不做**；**C 组是流程项**。
> **A 组里的唯一约束 / 外键行为 / UPSERT 语义全都能用纯 SQL 验证**——`AutoMigrate` 建完表，直接用 `psql` INSERT / DELETE / UPSERT 就能看出行为对不对，**不需要写任何 Go 业务代码，也不需要接口层**。
> **⛔ 禁止类与必须类成对**：下面的检查一律写成成对的——「不许出现 X」旁边一定有一条「Y 必须在」。理由是本项目已经吃过一次亏：**`62d10fa` 的 `SourceMessage` tag 被截断，`go build` / `go vet` / `gofmt` / 当时全部六条既有 grep 全部照过**，因为那六条查的全是"有没有违规"，**没有一条查"该有的 key 是否齐全"**。每写完一条禁止类检查，回头问一句：**「如果我把这行整个删掉，有哪条检查会响？」** 答不上来的地方就是缺了一条必须类检查。

### 分组 A · 模型层（✅ 本分支的验收范围，不依赖任何人）

- [ ] `cd backend && go build ./... && go vet ./... && go test ./...` 全通过
- [ ] **`AutoMigrate` 跑通**：`cmd/migrate` 在一个临时库上执行成功，`user_profile` 表被建出来（重复执行幂等）
- [ ] **表结构逐列对齐 DDL**：`\d user_profile` 的 5 列，类型 / 可空性 / 默认值与 §3.1 逐行一致
  - `profile_data` 是 `jsonb`、`NOT NULL`、默认 `'{}'`
  - `updated_at` 是 `timestamptz`、`NOT NULL`、默认 `now()`
  - `user_id` / `persona_id` **无默认值**（`nextval` 污染的反向验证）
- [ ] **`persona_id` 的唯一性真的存在**（**必须类，本表最重要的一条**）：`\d user_profile` 的 **Indexes 段**里能看到
  - `"idx_user_profile_persona_id" UNIQUE, btree (persona_id)`（GORM 的 `uniqueIndex` tag）
  - ⚠️ **只查"唯一性存在"，不查名字、不查段落**：DDL 没有给这个唯一性命名，正确实现在 `\d` 里显示的名字与 DDL 手写版本**本来就不一样**（§3.3）。**换用 `unique` tag 会落成 Constraints 段的 `uni_user_profile_persona_id`——同样合格**。**按"Constraints 段必须有 UNIQUE"去查，本实现会被误判为失败**
- [ ] **唯一性用 SQL 反向验证（最关键的一条实测）**：
  - `INSERT INTO user_profile (user_id, persona_id, profile_data) VALUES (1,1,'{}');` → **`INSERT 0 1`**（成功）
  - **再插一行同 `persona_id`、不同 `user_id`** → **必须报 `ERROR: duplicate key value violates unique constraint "idx_user_profile_persona_id"`**
  - ⚠️ **这一条是"删掉 `uniqueIndex` tag 后唯一会响的检查**（`go build` / `vet` / `gofmt` / CI 全都不响）。**必须实测**：删掉/`DROP INDEX` 后重跑，确认第二行**真的插进去了**（`GROUP BY persona_id HAVING count(*)>1` 出现 count=2），再恢复——**已按此做过一次，确认这条防护是真的**（§3.3 实测记录）
  - ⚠️ **判据是 `INSERT 0 1` 之外必须出现 `ERROR`**：若用子查询（`SELECT ... FROM personas JOIN users WHERE ...`）造冲突行，条件写错时会静默得到 **`INSERT 0 0`（一行都没插）**——**不报错也不冲突，看起来"没有重复"，其实唯一性根本没被考验到**。**这是本轮亲自踩过的假验证**（详见 plan §4.4）。**冲突行的 `persona_id` 用字面值或确认非空的子查询**
- [ ] **2 个外键都在，且都是 `ON DELETE CASCADE`**：
  - `FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`
  - `FOREIGN KEY (persona_id) REFERENCES personas(id) ON DELETE CASCADE`
  - ⚠️ **本表不该出现 `SET NULL`**（那是 `user_memory` 的 `source_message_id`，别照抄）
- [ ] **索引恰好两条**：`\d user_profile` 的 Indexes 段里**只有** `user_profile_pkey` 与 `idx_user_profile_persona_id`；**不存在** `idx_user_profile_user_id`（DDL 没有这条）
- [ ] **2 个外键的约束名不必对上 DDL**（实测是 GORM 的 `fk_user_profile_user` / `fk_user_profile_persona`，不是 Postgres 默认的 `<表>_<列>_fkey`）——**查 `ON DELETE CASCADE` 这个语义，不查名字**
- [ ] **没有多余的序列**：`SELECT sequencename FROM pg_sequences WHERE schemaname='public';` 里**不存在** `user_profile_user_id_seq` / `user_profile_persona_id_seq`
- [ ] **级联删除（SQL 级）**：手工插一行 `user_profile` → `DELETE FROM personas WHERE id = <该行 persona_id>` → **画像行消失**；再验一次删 `users`
- [ ] **`DEFAULT '{}'` 真的生效（SQL 级）**：插入时不写 `profile_data` → **读出 `{}`**（不是 NULL）
- [ ] **JSONB 往返一致（SQL 级）**：插 `{"occupation":"程序员","interests":["跑步","猫"]}` → 读回**逐字一致**（含中文与数组）
- [ ] **UPSERT 合并语义（SQL 级，本表独有的一条）**：
  - 先插 `{"occupation":"程序员"}`，再 upsert `{"interests":["跑步"]}`
  - → 合并后**两个键都在**（验证 `profile_data || EXCLUDED.profile_data` 的并键行为）
  - 再 upsert `{"occupation":"设计师"}` → **同名键被覆盖、`interests` 保留**
- [ ] **UPSERT 的越权兜底（SQL 级，本表独有的一条）**：
  - 用**另一个 `user_id`** 去 upsert 同一 `persona_id` → **`UPDATE 0`**，且**库里的内容与 `updated_at` 都没变**
  - ⚠️ 这一条验的是 §5.4 的第二层防线。**它不报错是设计（静默跳过），所以必须靠"数据没变"来判**，不能靠"有没有报错"
- [ ] **`updated_at` 的更新行为（SQL 级）**：upsert 之后 `updated_at` **必须变大**（验 §4.6 约定 3——`OnConflict` 不会自动填它）
- [ ] **`migrate.go` 的 `AutoMigrate` 里 `&UserProfile{}` 排在 `&Persona{}` 之后**，且 `git diff migrate.go` **只有 1 行新增**（没动别人的行）
- [ ] **`\dt` 里是 `user_profile` 不是 `user_profiles`**（`TableName()` 是必需方法，少了它会静默建错表）
- [ ] **代码层面**（⚠️ 每条都必须**锚定 tag**，不能裸 grep 单词：注释里**故意**写着 `bigserial` / `json:"-"` / `check:` / `index:` / `autoUpdateTime` 来解释"为什么不能这么写"，裸 grep 会把这些**正确**的解释误判成违规。**这个坑在本项目是复发而不是新发现**：[user-memory/spec.md 的 v3 变更记录](../user-memory/spec.md) 已经记过一条「三条裸 grep（`bigserial` / `json:"-"` / `emotion`）会命中注释里的解释性文字，改为锚定 tag」，而本轮在 user-profile 上**又犯了一次**——所以这不是"知道就行"，是**每条检查都要真的跑一遍**才算数。今日重测：裸 `grep -c 'bigserial'` 打在**已经落地且正确**的 `user_memory.go` 上读到 **2**（全是注释），锚定后 **0**）。**下面的命令逐条跑，不要用 `&&` 串成一条**（期望 0 的检查匹配 0 行时 `grep` 退出码为 1，链条会断掉、后面几条静默不执行）：
  | # | 类型 | 命令（`internal/model/user_profile.go`） | 期望 |
  |---|---|---|---|
  | A-1 | 必须 | `grep -c 'gorm:"column:'` | **5** = 5 个数据列 |
  | A-2 | 必须 | `grep 'gorm:"column:' \| grep -c 'json:"-"'` | **2** = `ID` + `UserID` |
  | A-3 | 必须 | `grep 'gorm:"column:' \| grep -oE 'json:"[^"]*"' \| grep -v 'json:"-"' \| wc -l` | **3** = `personaId` `profileData` `updatedAt` |
  | A-4 | 必须 | `grep -cE 'gorm:"foreignKey:[^"]*;references:[^"]*;constraint:OnDelete:'` | **2**（两个关联字段的三个 key 必须齐全） |
  | A-5 | 必须 | `grep -cE 'gorm:"[^"]*unique'` | **1**（`persona_id`，§3.3） |
  | A-6 | 必须 | `grep -cE 'return "user_profile"'` | **1**（`TableName()` 返回单数；返回复数会变 0） |
  | A-7 | 必须 | `grep -cE 'ProfileData +JSONB +'` | **1**（用 `model.JSONB`，不是裸 `[]byte` / `map`） |
  | A-8 | 必须 | `gofmt -l internal/model/user_profile.go` | **无输出** |
  | A-9 | 禁止 | `grep -cE 'gorm:"[^"]*type:bigserial'` | **0** |
  | A-10 | 禁止 | `grep -cE 'gorm:"[^"]*check:'` | **0**（DDL 没有 CHECK） |
  | A-11 | 禁止 | `grep -cE 'gorm:"[^"]*index:'` | **0**（DDL 一条索引都没有，§3.5） |
  | A-12 | 禁止 | `grep -cE 'gorm:"[^"]*autoCreateTime'` | **0**（`updated_at` 用 `autoUpdateTime`） |
  | A-13 | 禁止 | `grep 'gorm:"column:' \| grep -c 'json:"userId"'` / 同前 `'json:"id"'` | **0 / 0**（契约没有这两个字段） |
  | A-14 | 必须 | `grep -cE 'gorm:"[^"]*autoUpdateTime'` | **1**（`updated_at` **必须有** `autoUpdateTime`） |
  - ⚠️ **A-2 / A-3 / A-6 / A-7 / A-9 / A-13 / A-14 七条已用真实缺陷反向验证过**（做法与实测结果见 plan §4.4）。**两条已发现的假检查，不要复用**：
    ① **`user_memory` spec 的 `grep -cE 'json:"-.{0,2}$'`**——它要求 tag 在**行尾**，字段后面一旦跟了行尾注释（本文件**计划就要写**这类注释）就会**静默少算到 0**（实测：正常 4 → 加行尾注释 0）。
    ② **任何裸 grep 单词的形式**（`grep -c 'type:bigserial'`、`grep -c 'autoUpdateTime'`、`grep -c 'json:"id"'`）——本文件**故意**在注释里写着 `type:bigserial` / `json:"id"` / `autoUpdateTime` 来解释"为什么不能这么写"，裸形式会把这些**正确**的解释**误判成违规**（实测，对着**目标文件的真实注释**跑：A-9 0→**1**、A-13 0→**1**、A-14 1→**2**，三条全部误报成失败）。**上面表里的形式已全部锚定**——要么锚定 `gorm:"` 前缀，要么锚定 `gorm:"column:` 行 / `return` 语句。**A-6 / A-9 / A-13 / A-14 四条是本轮实测后从裸形式改过来的**，改完复测：正常版读数正确、损坏版照样能响（见 plan §4.4）
  - ⚠️ **A-14 是 A-12 的配对项**：A-12 只能证明"没有写错成 `autoCreateTime`"，**不能证明"真的写了 `autoUpdateTime`"**——两条都漏写时 A-12 同样得 0（实测：正常版与损坏版 A-12 读数一样）。**禁止类检查旁边必须有必须类**，这条正是"成对"原则的现场实例（§8 开头的说明）。
- [ ] **红线 1 自查**：`git status --short` 无 `.env` / `.key` / `.pem`；验证用的 DSN 用环境变量传入，**没有写进代码或 `_test.go`**

### 分组 B · 接口层（🚧 阻塞：依赖成员 1 的 `middleware` + 端点落地，**本分支不做**）

> 交接给成员 1 的验收清单。**`JWTAuth` 落地前这一组全部无法执行**（§7.1 B）。

- [ ] `GET /profile/portrait?personaId=<自己的>` → `200`，`data` 的键**恰好 3 个**：`jq '.data | keys | length'` = 3
- [ ] **响应体不含** `id` / `userId` / `createdAt` / `embedding*`（契约 §7 没有）
- [ ] **空态**：人设属于自己但没有任何画像行 → `200` + `data.profileData == {}` + `data.updatedAt == null`
  - ⚠️ **`jq -r '.data.profileData'` 必须是 `{}` 而不是 `null`**——这是 §4.4 那条坑的实测判据
- [ ] **越权（跨用户）**：用 B 的 Token 打 A 的 `personaId` → **`4043`**
- [ ] **不存在**：不存在的 `personaId` → **`4043`**，且 `message` 与上一条**逐字相同**
- [ ] **不产生 `4030`**：越权 / 不存在 / 缺参 / 无 Token 四种输入**都不出现 `4030`**
- [ ] **缺 `personaId`** → `4001`（**不是** `200` + `{}`）
- [ ] **跨人设隔离**：同一账号两个不同人设各有画像，各自只查到自己那份（`personaId` 不串）
- [ ] **写入不覆盖别人的行**（**本表独有的高危项**，§5.4）：用小号身份或直接 SQL，对别人的 `personaId` 调 upsert 路径 → **对方画像内容与 `updated_at` 均不变**
- [ ] 不带 Token / Token 无效 → `4010` / `4011`（由中间件产生）
- [ ] **唯一写入方**：全仓库 `grep -rn "user_profile" --include=*.go` 命中的**只有** `model/user_profile.go`、`repository/memory_repo.go` 与迁移/测试文件——没有第二处自己拼 INSERT 的地方
- [ ] **不该写的代码不存在**：全仓库无 `POST` / `PUT` / `DELETE` 的 `/profile/portrait` 路由
- [ ] code 评审 grep 通过：无硬编码错误码数字/文案、无 `response.Fail` in handler、无 `profile_data` 的 `Unmarshal`、无 secrets

### 分组 C · 流程（阻塞合并）

- [ ] **契约空白已广播 + 由队长拍板**：`API_CONTRACT.md` §7 的 `GET /profile/portrait` 错误码列补 `4001`、明确 `updatedAt` 可为 `null`、§12 登记变更记录。**本分支不直接改全局文件**——改由队长 / 契约负责人执行。**可与 user-memory 那条 `4001` 合并成一次广播**
- [ ] **阶段一的画像兜底方案有结论**（§7.3 #1）：灌数据 / 规则档补产出 / 接受为空，三选一在群里留个话
- [ ] **与成员 1 的文件分工已说定**（§7.3 #2）：画像的读 / 写加进 `memory_repo.go` / `memory_handler.go`，而这两个文件同时也是 user-memory 分支的目标——**谁先合、谁后加，说定再动**
- [ ] PR 已开、至少 1 人 Approve；commit 格式 `<type>(<scope>): <subject>`（scope 用 `user`）

> **本分支的合并门槛就是「分组 A 全绿 + 分组 C 前三项有结论」**——接口层（B 组）不在本分支的验收范围内，**没有它本分支照样能合并**（它是模型层交付）。

## 9. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-16 | v1 | 创建 | 用户画像开工前的设计与验收基线。含数据模型（`persona_id` 的 DB 级唯一性、`profile_data` 的开放键）、接口定义（3 字段响应、空画像 `{}`、缺参 `4001`）、**越权防线（读的双条件 + 本表独有的写路径覆盖风险 §5.4）**，以及两条契约空白的记录（§4.3 / §4.4）。验收标准按"禁止类与必须类成对"编写，A-2 / A-3 / A-4 / A-5 / A-6 / A-7 / A-9 / A-13 / A-14 九条**已用真实缺陷双向验证**（plan §4.4）。 |
| 2026-09-16 | v1.1 | 修订 | §8 分组 A 的检查表：**A-6 / A-9 / A-13 / A-14 四条命令改为锚定形式**，并补上 A-14（`autoUpdateTime` 必须存在）作为 A-12 的配对项。原因：对着目标文件（含其在步骤 A1 里的真实注释）实测发现，**裸 grep 单词的形式会被注释里"为什么不能这么写"的反例说明误报成失败**（A-9 0→1、A-13 0→1、A-14 1→2 三条全部误报）；改锚定后复测，正常版读数正确、损坏版照样能响。另记入 plan §4.4"假检查 ②"。**注：同一类错误 [user-memory/spec.md](../user-memory/spec.md) 的 v3 已经记过一次——本轮是复发**，说明"知道这个坑"不能替代"每条检查都实跑一遍"。
| 2026-09-16 | v1.2 | 修订 + **模型层落地** | ① **`persona_id` 的唯一性改用 `uniqueIndex` tag**（原定 `unique`）——落点是 **Indexes 段的 `idx_user_profile_persona_id`**，不是 Constraints 段。§3.3 / §3.2 / §3.5 / §5.5 / §6 / §7.2 / §7.3 决策 2 已同步；**A 组的唯一性验收项从"Constraints 段"改为"Indexes 段"**——不改的话正确实现会被判假失败。② **A 组已在一次性容器里全绿实测**（§3.3 实测记录 + plan §4.4）：表结构 5 列逐项对齐、唯一索引与 2 个 `CASCADE` 外键、`DEFAULT '{}'`、JSONB 中文/数组往返逐字一致、**UPSERT 的合并 / 同名覆盖 / 越权兜底（`INSERT 0 0` 且内容与 `updated_at` 逐字未变）**、两层级联删除（删账号 → 删人设 → 删画像）、以及**索引反向验证**（`DROP INDEX` 后同一 `persona_id` 真的插进第二行）。③ 记入两条**假验证**：子查询造冲突行时条件写错会静默得到 `INSERT 0 0`（唯一性根本没被考验）；用新人设测"重复 persona_id"等于没测。

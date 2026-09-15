# plan · 用户记忆（User Memory）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-user-memory-model`
> 负责人：成员 3 ｜ 创建：2026-09-15 ｜ 最后更新：2026-09-15（**第 2 版**：按决策 1-3 把范围收窄到模型层，接口层改为交接设计）

> ✅ **本分支只做模型层（决策 1）**：`internal/model/user_memory.go` + `migrate.go` 追加一行 + spec/plan 文档。**`dto/` / `repository/` / `service/` / `handler/` / `router.go` 一律等与成员 1 对齐后再动**——§3.2-§3.5 是**交给接口层的设计约定**（文档先推进），不是本分支要落的代码。
> 🚧 **接口层验收阻塞在成员 1 的 `middleware`**（`JWTAuth` 是空壳，从不 `Set(ContextKeyUserID)`）。**本分支只做模型层验证**：`AutoMigrate` 跑通、表结构 / 外键 / 索引 / 序列正确，**不做接口层测试**（决策 2）。
> ⚠️ **先读 spec §5**：本功能未来最大的风险是**一行 `WHERE`**（`persona_id` + `user_id` 两个条件）与**一次归属判定**。这两处写错都能编译、能跑、能演示，只在"换个账号 / 换个心态"时才暴露——所以 §4 的审查计划不是形式，是本 plan 的核心。
> ⚠️ **外键行为现在就能验**：`AutoMigrate` 建完表后，级联 / `SET NULL` / 默认值都能用**纯 SQL** 验证（`psql` 里 INSERT + DELETE），**不需要任何 Go 业务代码，也不需要接口层**——这正是本分支 A 组验收的主体。

---

## 1. 步骤拆解

### 1.1 本分支（模型层，不等任何人）

| # | 步骤 | 产物 | 完成标准 | 前置 | 状态 |
|---|---|---|---|---|---|
| 1 | **翻译 Struct** | `internal/model/user_memory.go` | 10 列 + **3 个外键关联**（2×CASCADE + 1×**SET NULL**）+ `MemoryType` 常量全被声明 + **`TableName()` 返回 `user_memory`**（这里**必需**，GORM 默认会复数成 `user_memories`）；字段对照 spec §3.2 逐行对齐；每字段带注释 | 无 | ✅ **已完成** |
| 2 | 建表 | `internal/model/migrate.go` | `AutoMigrate` 追加 `&UserMemory{}`，顺序在 `&ChatMessage{}` **之后** | 步骤 1 | ✅ **已完成**（`git diff` 恰好 +1 行，未动别人的行） |
| 3 | **模型层实库验证** | 临时库上的实跑记录（原始输出已回填 §6） | spec §8 **分组 A 全绿（17/17）**：`AutoMigrate` 跑通且**幂等**、3 个 FK 的 `ON DELETE` 各自正确、3 个索引在、序列恰好 4 条、级联与 `SET NULL` 用 SQL 各验一次、`float64→numeric` 往返一致 | 步骤 1、2 | ✅ **已完成** |
| 4 | **人工审查** | 本文件 §4 的清单 + §6 的原始输出 | §4.1 前两遍 + §4.4 的 **A 组命令**全过 | 步骤 3 | ✅ **已完成**（A0-A10 全过；逐条对照 §4.3 的 8 个高危点自查通过）。**提交与 PR 暂缓**——用户指定代码先留在工作区 |
| 5 | **与成员 1 对齐接口层分工（阻塞合并）** | 群里的话 | 三选一说定：他写 / 本分支接手 / 一起写。**没结论就不合并**（spec §7.3 #1） | 步骤 4 | 未开始 |
| 6 | **契约空白广播（阻塞合并）** | 群里的话 | spec §4.3 的 `4001` 已广播、由队长拍板；**`API_CONTRACT.md` 不由本分支改**（决策 3） | 步骤 4 | 未开始 |

### 1.2 交接给成员 1（本分支**不做**，等对齐后再定）

| # | 步骤 | 产物 | 完成标准 | 何时做 |
|---|---|---|---|---|
| H0 | 通用分页结构 | `internal/dto/common_dto.go` | `PageResult[T]` + 分页常量 + `ClampPage` **只有这一份**；persona / chat-message 分支若已落地则**直接消费** | 接口层开工时 |
| H1 | 请求 / 响应结构 | `internal/dto/memory_dto.go` | `MemoryItemResponse`（**恰好 6 字段**）+ `MemoryListQuery`（`form:"personaId"`） | 同上 |
| H2 | 归属判定 | `internal/repository/persona_repo.go` 的 `ExistsOwnedByUser` | **一条查询、两个条件；只回 bool** | 同上（与 chat-message 分支共用，**别写两份**） |
| H3 | **仓储层** | `internal/repository/memory_repo.go` | `ListByPersona`（含 `total`）+ `CreateBatch`；签名强制带 `userID` **与** `personaID`；**不存在**单条件查询、不存在删除方法 | 同上 |
| H4 | **业务层** | `internal/service/memory_service.go` | **先判归属、后查数据**；未命中一律 `4043`；`userID == 0` → `4010`；`personaID == 0` → `4001`；不依赖 `*gin.Context` | 同上 |
| H5 | **HTTP 层** | `internal/handler/memory_handler.go` | 薄；含 `RegisterMemoryRoutes`；错误 `_ = c.Error(err)` 上抛 | 同上 |
| H6 | 路由挂载 | `router.go` | 加一行，**不要与成员 1 同时改** | 同上 |
| H7 | 接口层验证 | — | spec §8 **分组 B**（🚧 依赖 `JWTAuth` 实现） | `JWTAuth` 落地后 |

**必查三件事（写进步骤 1 的完成标准，review 时逐条回看）：**

1. **`UserMemory.ID` 严禁出现 `bigserial`**，必须是 `type:bigint` + `autoIncrement`（spec §3.2 第 2 条，全项目已踩两次）。
2. **三个外键关联一个都不能少，且第三个必须是 `*ChatMessage` + `OnDelete:SET NULL`**（spec §3.4）——这是本表与另两张表唯一不同的地方，也是本步骤最容易整行丢掉的东西。
3. **`json:"-"` 有七处**（4 个数据列 `UserID` / `EmbeddingID` / `EmbeddingStatus` / `SourceMessageID`，**加 3 个关联字段** `User` / `Persona` / `SourceMessage`）：契约的 MemoryItem 只有 6 个字段，多一个就是契约漂移 + 内部状态泄漏。⚠️ 不是四个——三个关联字段同样带 `json:"-"`。

**优先级**：步骤 1-3 **今天就能全做完**——`cmd/migrate` 已就绪，`User` / `Persona` / `ChatMessage` 三张表都已在仓库里，**模型层不依赖任何人的任何东西**（spec §7.1 A）。

## 2. 文件清单

**A. 本分支产出**

| 路径 | 作用 | 谁还会用到 |
|---|---|---|
| `backend/internal/model/user_memory.go` | `user_memory` 的 GORM 实体（含 3 个外键，其中 1 个 SET NULL） | **成员 1**（提取链路落库）、成员 2（阶段二无关，但画像页要按人设查记忆） |
| `backend/internal/model/migrate.go` | 建表入口（追加一行） | 成员 1（`main.go` 调用） |
| `docs/specs/user-memory/spec.md` / `plan.md` | 本功能的规格与计划 | **成员 1**（接口层的规格）、成员 2（响应结构） |
| `docs/dev_notes/user_memory_notes.md` | **我的审查笔记**（不是给别人的文档），与既有的 `user_model_notes.md` 同级 | 只有我。⚠️ **`docs/dev_notes/` 被 git 忽略**（`.git/info/exclude`，与 `docs/agent_logs/`、`deploy/.env` 同一份排除表），所以它**不进 PR、不入库**——想进 PR 的内容必须写进 `spec.md` / `plan.md` |

**B. 交接给成员 1（本分支不动）**

| 路径 | 作用 | 别踩的坑 |
|---|---|---|
| `backend/internal/dto/common_dto.go` | `PageResult[T]` + 分页常量 + `ClampPage`，**全项目五处分页共用** | persona / chat-message / user-memory **谁先落地谁建** |
| `backend/internal/dto/memory_dto.go` | 记忆响应结构 + 构造器 + 列表查询参数 | `form:"personaId"`（spec §4.5 的 ⚠️） |
| `backend/internal/repository/persona_repo.go` | 增 `ExistsOwnedByUser` | **三个分支都会改这个文件** |
| `backend/internal/repository/memory_repo.go` | `ListByPersona`（读）/ `CreateBatch`（写） | 唯一写入入口；**不要有 `ListByUser`** |
| `backend/internal/service/memory_service.go` | 归属校验 + 分页 + DTO 转换 | 顺序：先判归属后查数据 |
| `backend/internal/handler/memory_handler.go` | 1 个端点 + `RegisterMemoryRoutes` | — |

## 3. 关键实现要点

### 3.1 翻译 Struct（步骤 1）—— **本分支的核心工作**

**手法沿用 `docs/dev_notes/user_model_notes.md`**：每个字段一行注释，把对应的 DDL 原文贴在 GORM tag 上方。理由很实际——`user.go` / `chat_message.go` 那种带注释的版本才是能看懂、能改的版本。

> **实际执行时改了流程（记录一下）**：原计划是"先写带全注释的版本进 `dev_notes`（git 忽略），再精简落 `user_memory.go`"。落地时改成**直接把完整注释写进 `user_memory.go`**——因为 `dev_notes` 不入库，注释放那儿等于**给 PR 审查者看不见**，而这份注释恰恰是最该被审的东西。所以现在：注释在**要入库的代码里**（审查者看得到），`docs/dev_notes/user_memory_notes.md` 只放**验证证据与判断依据**（哪些是我实测的、哪些是我核源码确认的、哪些没验）。

**四处最容易翻车**（完整对照表见 spec §3.2）：

1. **`ID` 的 tag 是 `type:bigint` + `autoIncrement`，不是 `type:bigserial`**：`user.go` / `persona.go` / `chat_message.go` 已在同一条坑上摔过。本表是这条链的**末端**（`source_message_id` 引用 `chat_messages.id`），但阶段二 `embedding_id = memory_id` 会把主键语义抬到 ChromaDB——**照抄正确写法，不要开倒车**。
2. **三个外键关联一个都不能少，且第 3 个是 `SET NULL`**：

   ```go
   // 仅供 GORM 生成外键约束；Create 时不要赋值（保持零值），否则会连带写 users / personas / chat_messages
   User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
   Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
   // ⚠️ 唯一一个 SET NULL：指针关联 + 可空列成对出现，删消息只断溯源、不删记忆
   SourceMessage *ChatMessage `gorm:"foreignKey:SourceMessageID;references:ID;constraint:OnDelete:SET NULL" json:"-"`
   ```

   只写标量 `UserID` / `PersonaID` / `SourceMessageID` 时 GORM **一个外键都不建**——"删人设级联清空记忆"与"删消息保留记忆"两条验收项直接不成立。
   **`SourceMessageID` 必须是 `*uint64`**：`SET NULL` 语义要求列可空，值类型会让 `AutoMigrate` 建出 `NOT NULL`，删消息时直接失败。
3. **`json:"-"` 有七处**（4 个数据列：`UserID`（契约 MemoryItem 没有 `userId`）、`EmbeddingID`、`EmbeddingStatus`、`SourceMessageID`；**加 3 个关联字段** `User` / `Persona` / `SourceMessage`）。**响应体只有 6 个字段**，多一个就是契约漂移 + 内部状态泄漏。
4. **`MemoryType` 用自定义类型 + 三个常量，但不加 `check:` tag**（spec §3.3）：DDL 上 `memory_type` **没有 CHECK**（不像 `chat_messages.role`），加 CHECK 就是偏离权威 DDL，属改 schema、得走广播。**Go 层常量是唯一的编译期防线**，所以落库前的类型校验必须在提取链路（spec §4.6 约定 4）。

```go
// 对应 SQL：memory_type VARCHAR(20) NOT NULL   -- fact | preference | event
//   DDL 上没有 CHECK（与 chat_messages.role 不同），所以这里不加 check: tag——
//   加了就是偏离权威 DDL。编译期安全由自定义类型 + 常量提供，非法值的拦截在落库前。
//   ⛔ 没有 emotion：情绪是内部信号，不作为长期记忆存储（总纲 §0）。
type MemoryType string

const (
	MemoryTypeFact       MemoryType = "fact"
	MemoryTypePreference MemoryType = "preference"
	MemoryTypeEvent      MemoryType = "event"
)

MemoryType MemoryType `gorm:"column:memory_type;type:varchar(20);not null;index:idx_memory_persona_type,priority:2" json:"memoryType"`
```

**索引 tag 的写法**（三个索引，其中一个跨两个字段）：

```go
UserID    uint64 `gorm:"column:user_id;type:bigint;not null;index:idx_memory_user_id" json:"-"`
PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_memory_persona_type,priority:1" json:"personaId"`
EmbeddingStatus string `gorm:"column:embedding_status;type:varchar(10);not null;default:'pending';index:idx_memory_embedding_status" json:"-"`
```

> **`importance_score` 的 Go 类型用 `float64`**（`NOT NULL`，不是指针）。它是本表最可能触发 pgx 类型问题的一列——`chat_message.go` 已在 `emotion_score` 上留过同一条备注：**实测报类型不匹配就退到自写 `driver.Valuer` 返回字符串**（零新增依赖），**不要引入 `decimal` 包，也不要为此改列类型**。

### 3.2 Repository（**交接设计**，spec §5.1 ②）

> 📌 下面三节（3.2 / 3.3 / 3.4）是**给成员 1 的接口层规格**，本分支不落地（决策 1）。写在这里是为了让接口层有唯一一份可依据的约定；[spec §4 / §5](spec.md) 是它的正文。

**核心是"让不安全的查询不存在"**，而不是"记得每次都带两个条件"：

```go
// memory_repo.go —— 签名强制带 userID 与 personaID，调用方没有"忘了传"的选项
ListByPersona(ctx context.Context, userID, personaID uint64, offset, limit int) ([]model.UserMemory, int64, error)
CreateBatch(ctx context.Context, tx *gorm.DB, memories []*model.UserMemory) error
```

> **刻意没有** `GetByID` / `FindByID` / `ExistsByID` / `Delete*` / `ListByUser`：
> - 没有单条详情端点（契约 §7），所以"单查记忆"这个能力**不需要存在**；
> - **尤其不能有 `ListByUser`**——那个方法名天生只带一个条件，写出来早晚有人用，用了就跨人设串号（spec §5.4）。**要按人设查，方法名里就写死 `ByPersona`**。
> - 记忆只读（总纲 §0），删除方法一个都不留。

**列表查询**（两个条件、`total` 同条件、排序带 `id` 兜底，理由见 spec §4.2 / §4.4）：

```go
err := db.WithContext(ctx).Model(&model.UserMemory{}).
    Where("persona_id = ? AND user_id = ?", personaID, userID).   // 两个条件，缺一不可
    Count(&total).Error
// ...
err = db.WithContext(ctx).
    Where("persona_id = ? AND user_id = ?", personaID, userID).   // 与 Count 逐字相同
    Order("created_at DESC, id DESC").                            // id DESC 不是可选项
    Offset(offset).Limit(limit).Find(&list).Error
```

> **不要写 `Where(&model.UserMemory{PersonaID: personaID, UserID: userID})` 这种 struct 条件**：它把条件表达成"字段等于零值则忽略"，**`userID` 传 0 时那个条件会被静默丢掉**——恰好和 spec §5.1 ① 要防的"`user_id = 0` 兜底"是同一类事故。用字符串条件，写死两个参数。

**批量写入**：

```go
func (r *memoryRepo) CreateBatch(ctx context.Context, tx *gorm.DB, memories []*model.UserMemory) error {
    if len(memories) == 0 {
        return nil   // 空批次直接返回，不要开一个空事务
    }
    return tx.WithContext(ctx).Create(&memories).Error   // 用 tx，不要用包级 db
}
```

> ⚠️ **必须用 `tx.WithContext(ctx)`**：用包级 `db` 就等于脱离了事务，`CreateBatch` 报错时**前面的插入不会被回滚**（与 persona 播种那条坑同一个形态）。
> ⚠️ **不要给 `User` / `Persona` / `SourceMessage` 三个关联字段赋值**：GORM 会尝试连带保存它们。落库时**只赋 `UserID` / `PersonaID` / `SourceMessageID` 三个标量**（spec §4.6）。
> ⚠️ **不要显式写 `EmbeddingStatus`**：留零值，GORM 会按 tag 补上 `'pending'`。但**别以为这是"靠 DDL 默认值"**——string 类型的 `default:` 会被 GORM 解析进 `DefaultValueInterface`，`Create` 时这一列被**显式写进 INSERT**（GORM 还会把值回写进内存结构体），**不走数据库默认值**。DDL 里那条 `DEFAULT 'pending'` 只是同值的第二份副本，所以改默认值时 **tag 与 DDL 两处要一起改**。此结论来自 GORM 源码 + 本分支实测（spec §8 分组 A）。

### 3.3 Service（**交接设计**，spec §5.2）

- 签名收 `context.Context`，**不出现 `*gin.Context`**（红线 7）。
- **顺序不能反**：先 `ExistsOwnedByUser` 判人设归属，再查记忆。**不要用 `len(list) == 0` 反推归属**（spec §5.2）——那会把"还没记住任何事"误报成 `4043`。
- **未命中一律 `errcode.New(errcode.ErrPersonaNotFound)`（4043）**，不区分"不属于你"与"不存在"。**不要**写存在性探针，**也永远不要返回 `4030`**。
- **`userID == 0` 要当错误拦住**：`0` 说明上游鉴权失败（`JWTAuth` 未实现 / Token 缺失），**不能拿它去查库**，返回 `4010`（spec §5.1 ①）。
- 错误一律 `errcode.New` / `errcode.Wrap`，**不拼接自定义文案**（红线 6）。
- 分页钳制在这里，不在 handler、也不在 repo。

```go
func (s *memoryService) ListByPersona(ctx context.Context, userID, personaID uint64, page, pageSize int) (*dto.PageResult[dto.MemoryItemResponse], error) {
    if userID == 0 {
        return nil, errcode.New(errcode.ErrUnauthorized)    // 鉴权失败不许静默降级
    }
    if personaID == 0 {
        return nil, errcode.New(errcode.ErrInvalidParams)   // 缺参：4001，不是空列表（spec §4.3）
    }

    // ① 归属判定单独做一次：决定"4043 还是数据"，与"有没有记忆"无关
    owned, err := s.personaRepo.ExistsOwnedByUser(ctx, userID, personaID)
    if err != nil {
        return nil, errcode.Wrap(errcode.ErrDBFailed, err)
    }
    if !owned {
        return nil, errcode.New(errcode.ErrPersonaNotFound) // 不属于你 / 不存在 —— 同码同文案
    }

    // ② 查数据：两个条件仍然都要带（防线各司其职，不是二选一）
    //    注：ClampPage 的确切签名以 common_dto.go 实际落地的那份为准（三个分支共用一个文件），
    //    这里只示意"钳制 + 算 offset"发生在 service 层，不在 handler、也不在 repo
    page, pageSize, offset, limit := dto.ClampPage(page, pageSize)
    rows, total, err := s.memoryRepo.ListByPersona(ctx, userID, personaID, offset, limit)
    if err != nil {
        return nil, errcode.Wrap(errcode.ErrDBFailed, err)
    }

    list := make([]dto.MemoryItemResponse, 0, len(rows))   // ⚠️ 必须初始化：nil slice 会序列化成 null
    for i := range rows {
        list = append(list, dto.NewMemoryItemResponse(&rows[i]))
    }
    return &dto.PageResult[dto.MemoryItemResponse]{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}
```

### 3.4 DTO 与 Handler（**交接设计**，spec §4.5）

- `common_dto.go` 只放 `PageResult[T]` + 分页常量 + `ClampPage`，**不放别的东西**（AGENTS.md：不新建杂物间文件）。
- `MemoryListQuery` 的 `PersonaID` tag **必须是 `form:"personaId"`**（spec §4.5 的 ⚠️）：写成 `persona_id` 或 `json:` 会让 Gin 静默绑不上，`PersonaID` 恒为 `0`，表现为"前端明明传了却说缺参数"。
- `MemoryItemResponse` **只有 6 个字段**；转换函数 `NewMemoryItemResponse` 放 DTO 层，**不要写在 model 上**（model 不依赖 dto）。
- Handler 只做三件事：绑定 → 调 service → `response.Success`；出错 `_ = c.Error(err)` 后 `return`。**不写 `response.Fail`**。
- **取 `userID` 必须检查 `ok`**：

  ```go
  userID, ok := c.GetUint64(middleware.ContextKeyUserID)
  if !ok {
      _ = c.Error(errcode.New(errcode.ErrUnauthorized))
      return
  }
  ```

- `RegisterMemoryRoutes` 照总纲 §4.5 的样式写（`rg.GET("/memory", h.List)`），`router.go` 由成员 1 加一行。

### 3.5 写入链路的事务边界（**交接设计**，spec §4.6）

`CreateBatch` 的事务边界在**调用方**（成员 1 的提取链路），repo 只接受 `tx`。三条约定：

1. **一轮提取的全部记忆在一个事务里**：要么都落、要么都不落。半批落库会让"AI 记住了 3 件事里的 1 件"这种现象无法解释。
2. **`user_id` / `persona_id` 由 Go 侧赋值**，不采信 AI 服务返回体（`/memory/extract` 的响应里只有 `memory_type` / `content` / `importance_score`，**没有也不该有归属字段**）。
3. **`source_message_id` 填本轮用户消息 id**，删消息时外键自动置 `NULL`——**不要自己写清理逻辑**。

---

## 4. 我手动审查 AI 代码的计划

> 前提：AI 生成的代码**默认不可信**。本分支的产物只有两个文件，但它们是**下游一切的基座**——`AutoMigrate` 建出来的表长什么样，直接决定接口层能不能"改对了"，也决定阶段二接 ChromaDB 时要不要改表。所以审查的**目标不是"能编译"，是"我能逐行解释 `\d user_memory` 为什么长这样"**。
> **审查分两段**：**A 段（§4.2-§4.4）是本分支现在就做的**，对象是 `user_memory.go` / `migrate.go`；**B 段（§4.5）是给接口层的**，等与成员 1 对齐、有人开始写 `repo` / `service` / `handler` 时再启用（**本分支不产出这些代码，但我是这两个文件的设计者，规格是我写的，我仍然要能解释它**）。

### 4.1 审查方法（A 段用两遍 + 实跑，比 persona 那轮少一遍"接口对抗"）

| 遍 | 做什么 | 判据 |
|---|---|---|
| **第一遍 · 逐行重写注释** | 每个字段一行注释、DDL 原文贴在 tag 上方。**实际落在要入库的 `user_memory.go` 里**（不是 `dev_notes`——见 §3.1 的流程说明），用我自己的话写 | **写不出注释的那一行，就是我没看懂的地方** → 去查 GORM 文档或问，不放过 |
| **第二遍 · 对照 DDL 核对** | 拿 spec §3.1 的 DDL 逐列核对：类型 / 可空性 / 默认值 / 外键的 `ON DELETE` / 索引列与顺序，填成表格 | 每一列都能在 DDL 里指到来源；**DDL 里没有的东西一律不加**（`memory_type` 的 CHECK 就是这么被排除的，spec §3.3） |
| **第三遍 · 实库对抗验证** | `AutoMigrate` 到一个**干净的库**上，然后按 §4.4 的 A 组命令逐条实跑：`\d` 看结构、`pg_sequences` 看序列、**手工 INSERT + DELETE 验级联与 `SET NULL`** | 每条约束都有一次实跑记录，不是"读代码觉得没问题" |

**第三遍是本段的核心，而且这一遍现在就能做全**：外键行为是**数据库层**的事，`AutoMigrate` 一跑完就能用纯 SQL 验证——**不需要接口层，也不需要 `JWTAuth`**。另外两遍只能证明"代码符合我的预期"，第三遍才能证明"`\d` 出来的东西真的是 DDL 描述的那个表"。

> **B 段（人的对抗验证）等接口层**：跨用户 / 跨人设串号这两条必须用"另一个账号的 Token"实跑，那要等 `repo` / `service` / `handler` 与 `JWTAuth` 都就位——见 §4.5。

### 4.2 A 段 · 逐文件审查清单（本分支）

| 文件 | 我要盯的点 | 看懂的标准 |
|---|---|---|
| `model/user_memory.go` | `ID` 是否 `type:bigint`（**不是 bigserial**）；`json:"-"` 是否**七处**（4 数据列 + 3 关联字段）；`SourceMessageID` 是否 `*uint64`；**三个**关联字段是否都在、`SourceMessage` 是否**指针**且 `OnDelete:SET NULL`；`MemoryType` 是否自定义类型；`importance_score` 是否 `float64`；两个索引 tag 的 `priority` 是否正确（`persona_id`=1、`memory_type`=2）；**`TableName()` 是否返回 `user_memory`**（GORM 默认复数化是 `user_memories`，少了这个方法会静默建错表） | 我能说出每个 tag 去掉之后会发生什么；`\d user_memory` 看到的 3 个 FK / 3 个索引与代码逐条对得上 |
| `model/migrate.go` | `&UserMemory{}` 是否排在 `&User{}` / `&Persona{}` / `&ChatMessage{}` **之后**（它同时引用三张表）；**有没有顺手改动别人的行** | `AutoMigrate` 一次跑过，无依赖顺序报错；`git diff migrate.go` **只有 1 行新增** |
| `docs/dev_notes/user_memory_notes.md` | 里面的**验证证据**是否可追溯：每条"已验"都能指到一次实跑的原始输出，每条"没验"都写明了卡在哪个依赖上 | 出现"我不确定 / 这条没验"的批注也是合格的——那正是审查的价值。⚠️ 这份笔记**不入库**（git 忽略），所以**它不能是唯一载体**：要留痕的结论都已回填进 `spec.md` §8 / `plan.md` §6 |

### 4.3 A 段 · 九个高危点专项审查

| # | 高危点 | 为什么会错 | 我怎么验 |
|---|---|---|---|
| 1 | **`SourceMessage` 不是 `SET NULL`**（写成 CASCADE，或整行漏掉） | 漏掉 → GORM **一个外键都不建**；写成 CASCADE → **删一条消息顺手删掉一条长期记忆**，与"记忆是长期事实"直接矛盾。**这是本表与另两张表唯一不同的地方，也是 AI 最容易照抄上一份文件写错的一处** | `\d user_memory` 看第三个 FK 是否 `ON DELETE SET NULL`；**再实删一条消息**，确认记忆还在且 `source_message_id IS NULL` |
| 2 | **`SourceMessageID` 用了值类型** | `SET NULL` 要求列可空；值类型会让 `AutoMigrate` 建出 `NOT NULL`，**删消息时直接报错**（而不是静默）——但这只有在"真的删了一次消息"时才暴露 | `\d user_memory` 看 `source_message_id` 是否可空；§4.4 A3 实删一次 |
| 3 | **`ID` 写成 `bigserial`** | GORM 会把被引用主键的 `DataType` 复制到外键列上，让下游列长出 `nextval` 默认值。**已踩三次的坑，却是最容易"顺手写回去"的一处** | `grep -cE 'type:bigserial' user_memory.go` = **0**（⚠️ **不能**用裸 `grep -n "bigserial"`——注释里写着"不要写成 bigserial"来解释这个坑，裸 grep 会命中它，本分支实测误报过）；`pg_sequences` 里**不存在** `user_memory_*_id_seq`（期望恰好 4 条序列） |
| 4 | **`memory_type` 被加了 `check:` tag** | 「DDL 里别的表有 CHECK，这里也加一个」是很自然的联想，但这会让 **struct 与权威 DDL 不一致**（spec §3.3）——而 `AutoMigrate` 会**真的把这条约束建出来**，于是"代码与文档不符"变成了"库里多了一条约束" | 第二遍对照 DDL 核对：DDL 没有 CHECK，代码里就不能有。`\d user_memory` 里**不应**出现 `chk_user_memory_memory_type` |
| 5 | **`json:"-"` 少写一个 / 多写一个** | 少写 → `embeddingStatus` / `sourceMessageId` / `userId` 漏进响应体（内部状态泄漏 + 契约漂移）；多写 → 该给的字段没给 | `grep -cE 'json:"-.{0,2}$' user_memory.go` = **7**（4 数据列 + 3 关联字段）；再从**真实字段行**（`grep 'gorm:"column:'`）抽 `json:` 名、并**排除 `json:"-"`** = **恰好 6 个**（`id` `personaId` `memoryType` `content` `importanceScore` `createdAt`）：`grep 'gorm:"column:' user_memory.go \| grep -oE 'json:"[^"]*"' \| grep -v 'json:"-"' \| wc -l` = 6。⚠️ 两处陷阱都实测过：裸 `grep -c 'json:"-"'` 会把注释里的说明数进去（得 10 不是 7）；而**只跑到中间那步**会输出 10 行（10 个数据列里 4 个是 `json:"-"`），看到 10 **不等于**失败——必须带上 `grep -v` |
| 6 | **`MemoryType` 用了裸 `string`** | 拼错 `"preference"` 在编译期发现不了，会落成脏数据，前端三分类渲染掉进"未知"分支 | `grep -cE '^\s*MemoryType +MemoryType +`' user_memory.go` = **1**（字段声明为自定义类型）。⚠️ **不能**用 `grep "MemoryType string"`——那会命中 `type MemoryType string` 这行**类型声明本身**（本分支实测误报过）；三个常量都在 |
| 7 | **`importance_score` 的类型 / 编解码** | pgx 往 `numeric` 编 `float64` 若类型不匹配，落库直接失败（该列 **NOT NULL**，必写） | A5：插 `0.850` 读回 `0.850`；报错再退 `driver.Valuer`（§3.1 末注），**不要引入 `decimal`** |
| 8 | **`migrate.go` 动了别人的行** | 多人共用一个文件，顺手"整理"一下就会与别的分支冲突 | `git diff` 只有 1 行新增 |
| 9 | **关联 tag 被截断 / 结构体没闭合**（`62d10fa` 实际发生过） | 两处损坏同时出现在一个提交里：① 结构体闭合 `}` 丢失 → CI 报 `147:1 expected '}' found 'func'`，**能报出来**；② `SourceMessage` 的 tag 变成 `gorm:"...;constraint:OnDelete:SET NULL"`，`foreignKey` / `references` 一起没了 → **`go build` / `vet` / `gofmt` 全部照过**，但 GORM 退回按约定推断关联，这条外键可能建错或建不出来——而它恰恰是本表唯一与另两张表不同的地方 | ① `gofmt -e user_memory.go` 退出码 = 0；`grep -cE '^\}$'` = 2（结构体 + 方法各一个）；② `grep -cE 'gorm:"foreignKey:[^"]*;references:[^"]*;constraint:OnDelete:'` = **3**（已实测：修复版 3 / 损坏版 2，能报出缺陷）；③ 最终还是靠 `\d user_memory` 的 3 个 FK 兜底 |

### 4.4 必须亲眼看到的证据（A 段 · 现在就能全跑）

```bash
# ---- 准备：临时库。凭据从 deploy/.env 读，不写进代码 / 不写进文档（红线 1）----
# 注：cmd/migrate 读的是 SCHEMA_CHECK_DSN（裸 os.Getenv），与 backend/.env.example 的 DB_*
#     不是同一套变量，必须自己 export；compose 的 POSTGRES_USER 与 .env.example 的 DB_USER 也不同值
set -a; . deploy/.env; set +a
docker compose -f deploy/docker-compose.dev.yml exec -T postgres \
  sh -c 'createdb -U "$POSTGRES_USER" user_memory_schemacheck'
export SCHEMA_CHECK_DSN="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5432/user_memory_schemacheck?sslmode=disable"

# A0) 编译 + 静态检查
cd backend && go build ./... && go vet ./... && go test ./...

# A1) 建表（打临时库，不动 heart_echo）
go run ./cmd/migrate                      # 期望输出：AutoMigrate 完成

# A2) 表结构 / 外键 / 索引 —— 本分支最重要的一眼
docker compose -f deploy/docker-compose.dev.yml exec -T postgres \
  sh -c 'psql -U "$POSTGRES_USER" -d user_memory_schemacheck -c "\d user_memory"'
#    要看到（括号里是实测结果）：
#      fk_user_memory_user           FOREIGN KEY (user_id)           REFERENCES users(id)         ON DELETE CASCADE
#      fk_user_memory_persona        FOREIGN KEY (persona_id)        REFERENCES personas(id)      ON DELETE CASCADE
#      fk_user_memory_source_message FOREIGN KEY (source_message_id) REFERENCES chat_messages(id) ON DELETE SET NULL  ← 关键
#      idx_memory_persona_type btree (persona_id, memory_type)   ← 必须是【两列】，顺序不能反
#      idx_memory_user_id / idx_memory_embedding_status
#      embedding_id 可空、embedding_status DEFAULT 'pending'、source_message_id 【可空】
#      importance_score numeric(4,3) NOT NULL
#      id bigint DEFAULT nextval('user_memory_id_seq')  ← 这就是【正确】结果：PG 从不显示 bigserial 这个字，
#                                                        它只是 bigint + 序列默认值的简写（见 §4.3 #3）
#    并且【不应】出现 chk_user_memory_memory_type（DDL 没有这条 CHECK，见 §4.3 #4）
#    另外 `\dt` 里应只有 user_memory ——【不是】 user_memories

# A3) 序列清单：多一个都不行（bigserial 污染的反向验证）
docker compose -f deploy/docker-compose.dev.yml exec -T postgres sh -c \
 'psql -U "$POSTGRES_USER" -d user_memory_schemacheck -c \
  "SELECT sequencename FROM pg_sequences WHERE schemaname=(SELECT current_schema()) ORDER BY 1;"'
#    期望【恰好四条】：chat_messages_id_seq / personas_id_seq / user_memory_id_seq / users_id_seq
#    出现 user_memory_user_id_seq / _persona_id_seq / _source_message_id_seq = 关联复制污染，打回

# A4-A6) 级联 / SET NULL / 默认值 / numeric 往返 —— 纯 SQL，一次 heredoc 跑完
#        这段能跑起来正说明「外键行为是数据库层的事，不需要接口层、不需要 JWTAuth」
docker compose -f deploy/docker-compose.dev.yml exec -T postgres \
  sh -c 'psql -U "$POSTGRES_USER" -d user_memory_schemacheck -v ON_ERROR_STOP=1 -q' <<'SQL'
INSERT INTO users (id, username, email, password_hash) VALUES (1,'t_u','t@example.com','x');
INSERT INTO personas (id, user_id, name, personality_desc, speaking_style) VALUES (1,1,'p1','d','s'),(2,1,'p2','d','s');
INSERT INTO chat_messages (id, user_id, persona_id, role, content) VALUES (1,1,2,'user','我养了只猫叫豆豆');

-- A4) 级联：记忆挂 persona 1，删 persona 1 → 记忆必须消失
INSERT INTO user_memory (user_id, persona_id, memory_type, content, importance_score) VALUES (1,1,'fact','A 的记忆',0.5);
SELECT count(*) AS "删前persona1记忆数" FROM user_memory WHERE persona_id=1;   -- 期望 1
DELETE FROM personas WHERE id=1;
SELECT count(*) AS "删persona1后残留"   FROM user_memory WHERE persona_id=1;   -- 期望 0

-- A5) 默认值 + numeric 往返 + 溯源
INSERT INTO user_memory (user_id, persona_id, memory_type, content, importance_score, source_message_id)
  VALUES (1,2,'preference','用户喜欢跑步',0.850,1);
SELECT memory_type, importance_score, embedding_id IS NULL AS "embedding_id为NULL", embedding_status, source_message_id
  FROM user_memory WHERE persona_id=2;   -- 期望 0.850 / t / pending / 1

-- A6) SET NULL（方向与 A4 相反）：删消息 → 记忆【必须还在】、溯源变 NULL
DELETE FROM chat_messages WHERE id=1;
SELECT memory_type, content, importance_score, source_message_id IS NULL AS "溯源已断开"
  FROM user_memory WHERE persona_id=2;   -- 期望：行还在、0.850、t
SQL

# A7) Go 级插入 —— 只有它能验这两件事（临时写个 cmd/tmpcheck，跑完即删，不进仓库）
#     ① pgx 的 float64 → numeric(4,3) 能不能编码  ② 关联字段留零值会不会连带写 users / personas
#     实测：写 0.85 读回 0.85；users=1 / personas=1 / chat_messages=0（行数不变）
#     → CreateBatch 的调用形态安全，且【不需要】spec §3.2 第 5 条准备的 driver.Valuer 兜底

# A8) 幂等：同一份 schema 再跑一次 AutoMigrate，必须无报错、无副作用
go run ./cmd/migrate
#    再查一次应为 fk数=3 / 索引数=4（3 个显式索引 + pkey）/ 列数=10，与首次完全一致

# A9) code 层面 grep —— ⚠️ 每个都必须【锚定 tag】：注释里故意写着 bigserial / json:"-" / emotion
#     来解释"为什么不能这么写"，裸 grep 单词会把这些【正确】的解释误判成违规（实测三条全误报）
#     ⛔ 不要用 && 把这六行串成一条链：期望 0 的检查（前两条）匹配 0 行时 grep 退出码是 1，
#        链条会在第二行就断掉，后面四条根本不执行——"没输出"会被误读成"后面都过了"。逐行跑。
grep -cE 'type:bigserial'                internal/model/user_memory.go   # 期望 0
grep -cE 'gorm:"[^"]*check:'              internal/model/user_memory.go   # 期望 0
grep -cE 'json:"-.{0,2}$'                 internal/model/user_memory.go   # 期望 7（4 数据列 + 3 关联字段）
grep -cE '^\s*MemoryType +MemoryType +`'  internal/model/user_memory.go   # 期望 1（不能 grep "MemoryType string"）
grep -c  'gorm:"column:'                  internal/model/user_memory.go   # 期望 10
# 三个关联 tag 的三个 key 必须齐全（少一个是静默失败：build/vet/gofmt 全部照过）
grep -cE 'gorm:"foreignKey:[^"]*;references:[^"]*;constraint:OnDelete:' internal/model/user_memory.go   # 期望 3
#    ⚠️ 上一步只数数据列。要验"对外字段恰好 6 个"必须再排除 json:"-"，且【三步都要写】：
#    只跑前两步会输出 10 行（10 个数据列里 4 个是 json:"-"），看到 10 不等于失败
grep 'gorm:"column:' internal/model/user_memory.go | grep -oE 'json:"[^"]*"' | grep -v 'json:"-"' | wc -l   # 期望 6
git diff internal/model/migrate.go                                        # 只有 1 行新增

# A10) 红线 1 自查 + 收尾
git status --short | grep -E '\.env$|\.key$|\.pem$'      # 期望无输出
docker compose -f deploy/docker-compose.dev.yml exec -T postgres \
  sh -c 'dropdb -U "$POSTGRES_USER" user_memory_schemacheck'   # 不留临时库，不碰 heart_echo
```

> **实测记录**：A0-A10 已完整跑过一遍，spec §8 分组 A **17/17 全绿**。逐条原始输出见 §6 进度记录。
> **`-T` 不能省**：本分支实跑时每处 `docker compose exec` 都带 `-T`（不分配 TTY）。不加时 `psql` 的输出里可能混入 TTY 控制字符，`\d` 的表格会很难读。
> **为什么用临时库**：A4-A6 会往库里插测试数据（`t_u` 用户、`p1`/`p2` 人设、两条记忆）。留在开发库 `heart_echo` 里会混进演示数据——总纲 §6 的演示脚本本来就要灌记忆，混着"测试记忆"很难看。临时库跑完直接 `dropdb`，**开发库一行都没碰**。

### 4.5 B 段 · 接口层审查（**等与成员 1 对齐后再启用**，本分支不产出这些文件）

> 这四份文件（`dto` / `repository` / `service` / `handler`）的解与验收**在 spec 里已经写全**（§4 / §5 / §8 分组 B），本节只列"我作为规格作者会重点盯什么"。等有人开始写时，**规格是我写的，我仍然要能解释它**——这也是 D1 说"动手前对齐"的原因。

| 文件 | 我要盯的点 | 看懂的标准 |
|---|---|---|
| `dto/memory_dto.go` | 响应结构是否**恰好 6 个字段**；`form:"personaId"` 是否逐字 camelCase | 与契约 §7 的 JSON 示例并排比对，一个字母都不差 |
| `repository/memory_repo.go` | **每个方法签名是否都带 `userID` 与 `personaID`**；`Count` 与 `Find` 的条件是否**逐字相同**；有没有 `First(&m, id)` 形态的裸查询；有没有 `ListByUser` / `ExistsByID` / `Delete`；`CreateBatch` 是否用 `tx` | `grep -nE "First\(&|ExistsBy|Delete|ListByUser" memory_repo.go` 无业务命中 |
| `repository/persona_repo.go` | `ExistsOwnedByUser` 是否**一条查询、两个条件**、只回 `bool`；有没有顺手改到别的函数 | `git diff` 只有新增、没有改动既有行 |
| `service/memory_service.go` | **先判归属后查数据**（顺序）；`userID == 0` → `4010`；`personaID == 0` → `4001`；未命中统一 `4043`（**不出现 `4030`**）；`list` 是否 `make(..., 0)` 初始化；是否引了 `gin`；是否出现裸数字错误码 | `grep -n "gin"` 无结果；`grep -nE "\b(400[0-9]\|403[0-9]\|404[0-9]\|500[0-9])\b"` 无结果 |
| `handler/memory_handler.go` | 是否 `_ = c.Error(err)` 上抛；有没有 `response.Fail`；`userID` 是否只从 `c.GetUint64(ContextKeyUserID)` 取**且检查了 `ok`** | `grep -n "response.Fail"` 无结果 |

**B 段的两个高危点（必须用另一个账号实跑，不能只读代码）：**

| # | 高危点 | 为什么会错 | 我怎么验 |
|---|---|---|---|
| B1 | **跨用户泄漏**（只带 `persona_id`） | `persona_id` 全局自增，只按它查就是别人的记忆。AI 常把 `user_id` 当"可选优化"省掉 | 用 **B 的 Token** 打 A 的 `personaId` → 必须 `4043`；再直接用 SQL 跑**去掉 `user_id`** 的那条查询，亲眼看它**返回了别人的行**（证明这个条件必需，不是装饰） |
| B2 | **跨人设串号**（只带 `user_id`） | 把该用户所有人设的记忆混在一起。**响应看起来完全正常**（有数据、有条数、能翻页），只有对着"切人设问同一个问题"才暴露——**最隐蔽的一条** | 同一账号建**两个**人设、各写不同记忆 → 分别查，`list` 与 `total` 都必须只含自己那份；再直接 SQL 跑**去掉 `persona_id`** 的查询，看它把两个人设的记忆一起返回 |
| B3 | **空态与 `4043` 的分界** | 用 `len(list) == 0` 反推归属，会把"还没记住任何事"误报成 `4043` | 建一个**从未聊过**的人设，查它的记忆 → 必须 `200` + `list: []` |

### 4.6 拒绝标准（出现任一条就打回重写，不做"小修小补"）

**A 段（本分支，现在就适用）：**

| 打回条件 | 为什么不能只小修 |
|---|---|
| `SourceMessageID` 用值类型，或关联写成 `OnDelete:CASCADE` / 整行漏掉 | 值类型 → `SET NULL` 建不出来、删消息直接失败；CASCADE → **删一条消息顺手删掉一条长期记忆**，属数据损坏；漏掉 → 三个外键一个都不建 |
| `ID` 出现 `bigserial` | 静默污染下游外键列的默认值，**已踩三次** |
| `memory_type` 加了 `check:` tag | struct 与权威 DDL 不一致，且 `AutoMigrate` 会真的把约束建出来 |
| 响应体相关字段的 `json:"-"` 少了任意一个 | 内部状态泄漏 + 契约漂移；前端会照着 Mock 把它们渲染出来 |
| `MemoryType` 用裸 `string` | 编译期防线消失，脏数据落库 |
| `migrate.go` 动了别人的行 | 多人共用一个文件，冲突成本高、review 范围不清 |
| 出现裸数字错误码 / 中文错误文案字面量 | 破坏"一 code 一 msg"（红线 6），一处放纵会蔓延 |

**B 段（接口层，成员 1 开工时适用）：**

| 打回条件 | 为什么不能只小修 |
|---|---|
| 仓储层存在**不带 `user_id`** 或**不带 `persona_id`** 的业务查询 | 防线是结构性的，逐处打补丁会漏；**尤其 `ListByUser` 这种方法名本身就是漏洞的形状** |
| `Count` 与列表查询的条件**不一致** | `total` 会算进别人的记忆（或漏算自己的），且**只看响应体几乎发现不了** |
| 用 `len(list) == 0` 判越权 / 不存在 | 把"还没有记忆"误报成 `4043`，新伴侣的记忆页打不开 |
| 未命中返回 `4030` | 违反队长规则（**资源越权 → `4043`**）；`4030` 是功能越权的码、本模块无此场景 |
| 缺 `personaId` 时返回 `200` + 空列表 | 静默降级：把调用方的 bug 伪装成正常空态 |
| 响应体出现 `userId` / `embeddingId` / `embeddingStatus` / `sourceMessageId` | 契约漂移 + 内部状态泄漏 |
| 出现 `Delete*` / `Update*` 记忆的方法或端点 | 违反总纲 §0「记忆只读」；删除路径还要再防一次越权，白增泄漏面 |
| `memoryType` 里出现 `emotion` | 违反"情绪是内部信号"（AGENTS §4.4 / 总纲 §0） |
| 排序只写 `created_at DESC`（无 `id` 兜底） | 批量写入的同批记忆时间戳相同，翻页重复 / 漏项 |
| service 里出现 `*gin.Context` | 破坏分层（红线 7），后续无法单测 |
| `CreateBatch` 用包级 `db` 而不是 `tx` | 事务不生效，半批落库；演示时表现为"AI 只记住了一部分" |
| `PageResult` 出现第二份定义 | 两份同名类型在联调时会出现"字段对不上"的诡异问题，最难排查 |

**审查通过的唯一标准**：§4.4 的 A 组命令全部实跑过，且 §4.2 每个文件我都能逐行解释。**注释落在要入库的 `user_memory.go` 里**（审查者在 PR 里直接看得到），**验证证据落到 `docs/dev_notes/user_memory_notes.md`** 并在 §6 留原始输出。

## 5. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| **接口层分工未定**（spec §7.3 #1） | 两人同时写 `memory_repo.go` / `memory_service.go` / `memory_handler.go`（AGENTS §4.8 明令避免） | 步骤 5：**合并前必须有结论**（他写 / 本分支接手 / 一起写）；本分支先只交模型层，**不碰那些文件** |
| **接口层端到端验收做不了**（`JWTAuth` 是空壳，决策 2） | spec §8 分组 B 全部无法执行 | 本分支**只做模型层验证**（分组 A），并用 **SQL 级**验证覆盖外键 / 级联 / `SET NULL` / 默认值——这些是数据库层的事，**不需要接口层**。`JWTAuth` 是成员 1 的文件，**不要自己实现** |
| **三个分支都会改 `persona_repo.go` 与 `common_dto.go`**（persona / chat-message / user-memory） | 合并冲突，或各自写一份同名函数 | 群里确认**谁先合谁建**；谁用到时先 `grep -rn "type PageResult" internal/`，有就直接消费，**不重建** |
| 契约 §7 的 `4001` 未广播就合并（决策 3） | 前端按"只会收到 4043"写错误分支，缺参时会走到 `message` 兜底分支 | 步骤 6：**只广播、不改全局文件**；由队长拍板后交契约负责人统一改。**未同步不得合并** |
| ~~验证时把测试数据留在开发库~~ | ~~演示脚本要灌记忆数据，混着"测试记忆"会很难看~~ | ✅ **已消除**：全程在临时库 `user_memory_schemacheck` 上验证，跑完 `dropdb`，**开发库 `heart_echo` 一行未碰** |
| ~~`importance_score` 的 pgx 编码报类型不匹配~~ | ~~落库直接失败（该列 NOT NULL，是必写字段）~~ | ✅ **已排除**：实测 GORM `Create` 写 `0.85` 读回 `0.85`，pgx 能编码 `float64` → `numeric(4,3)`。**阶段一不需要** `driver.Valuer` 兜底（spec §3.2 第 5 条那条备选留着，别提前引入）。仍**不要引入 `decimal` 包** |
| 阶段二 ChromaDB 的 `metadata` 只存 `persona_id` | **跨用户串号**（技术文档 §6.3 点名的风险） | spec §5.1 ③ 已写明：`metadata` 存 `{user_id, persona_id, memory_type, importance_score}`，检索两个过滤条件都带。阶段二开工时按这段落地 |
| 阶段二 LLM 返回非法 `memory_type` | 落库脏数据，前端三分类渲染掉进"未知"分支 | spec §3.3 已写明：**落库前校验 / 归一，非法值丢弃并记日志**；这条属提取链路（成员 1），交付时口头确认一句 |
| 写入编排的归属两处不一致（技术文档 §9 有 `memory_service.go`，成员 1 任务书没有） | 落地时互相等 / 都写一份 | spec §7.3 #5；**模型层只保证 `CreateBatch` 的签名与约定稳定**，谁编排都接得上 |
| 排序契约没写、实现各来一套 | 记忆页顺序前后端不一致（前端自己再排一次，两边打架） | spec §4.4 已定为 `created_at DESC, id DESC` 并写明理由；**接口层落地后同步给成员 2**，让他不要在 Mock 里另排一遍 |

## 6. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-15 | spec / plan 第 1 版落地。定下 6 项设计决策（排序含 `id` 兜底、缺参 `4001`、归属单独判定、`SET NULL`、`MemoryType` 不加 DB CHECK、`CreateBatch` 进仓储层），记录 7 项待确认与 1 项契约空白 | 分工未定；`JWTAuth` 是空壳 |
| 2026-09-15 | **第 2 版：并入三项决策（D1-D3）** —— ① **范围收窄到模型层**：§1 拆成「本分支 6 步 / 交接成员 1 的 H0-H7」，§2 文件清单分 A/B，§3.2-3.5 标为「交接设计」；② **只做模型层验证**：§4 审查计划拆成 A 段（现在就做，本分支）/ B 段（接口层启用），**A 段的外键行为验证改成 SQL 级**（不需要接口层），§5 风险同步；③ **契约空白只记录**：步骤 6 只广播、不动全局文件 | **无阻塞**——步骤 1-3（模型层）今天就能全做完 |
| 2026-09-15 | **第 3 版：模型层落地 + 分组 A 全绿（17 项）** —— `internal/model/user_memory.go` 新增（10 列 + 3 关联字段 + `MemoryType`/3 常量 + **必需的 `TableName()`**），`migrate.go` 追加 1 行；§4.4 的证据命令全部换成**实跑过的版本**（凭据走 `deploy/.env`、grep 锚定 tag、临时库隔离）；修掉 **6 处会误伤正确代码的验收写法** + **1 处机制写反**（`embedding_status` 的默认值走 GORM 而非 DB）。**代码留在工作区，未提交**（用户指定） | **无阻塞**——模型层交付完毕。接口层（H0-H7）待与成员 1 对齐；契约空白广播待发 |
| 2026-09-15 | **第 4 版：修复 CI 报错 + 补一条静默缺陷检查** —— `62d10fa`（非本分支产出，已推送）里的 `user_memory.go` 有**两处损坏**：① 结构体闭合 `}` 丢失 → CI 报 `147:1 expected '}' found 'func'`；② `SourceMessage` 的 gorm tag 被截断成 `gorm:"...;constraint:OnDelete:SET NULL"`，`foreignKey` / `references` 丢失 → **`go build` / `vet` / `gofmt` 全部照过**。修复提交 `f3c7d08` 已推送（fast-forward，未 force）。**新增 §4.3 高危点 9** + §4.4 的 tag 完整性检查（已实测：修复版 = 3 / 损坏版 = 2，确实能报出缺陷）。修复后在临时库重跑了建表验证，3 个 FK 与 SET NULL 行为全部正确 | 暴露了一个验收盲区：原 A9 全部是"查有没有违规"，**没有一条查 tag 是否完整**，而缺 key 恰恰不报错。补上后这类损坏在 code 层就能拦住，不必等到 `\d` |

**实测原始输出**（§4.4 的 A0-A10，逐条）：

```text
A1) go run ./cmd/migrate        → 2026/09/15 20:25:34 AutoMigrate 完成

A2) \d user_memory（10 列 + 3 索引 + 3 外键）
     id                | bigint                   | not null | nextval('user_memory_id_seq'::regclass)
     user_id           | bigint                   | not null |                      ← 无默认值 ✅ 未被污染
     persona_id        | bigint                   | not null |                      ← 无默认值 ✅
     memory_type       | character varying(20)    | not null |                      ← 无 CHECK ✅
     content           | text                     | not null |
     embedding_id      | character varying(64)    |          |                      ← 可空 ✅
     embedding_status  | character varying(10)    | not null | 'pending'::character varying  ✅
     importance_score  | numeric(4,3)             | not null |                      ✅
     source_message_id | bigint                   |          |                      ← 可空 ✅（SET NULL 的前提）
     created_at        | timestamp with time zone | not null | now()                ✅
     "user_memory_pkey" PRIMARY KEY, btree (id)
     "idx_memory_embedding_status" btree (embedding_status)
     "idx_memory_persona_type" btree (persona_id, memory_type)   ← 两列、顺序正确 ✅
     "idx_memory_user_id" btree (user_id)
     "fk_user_memory_persona"        FOREIGN KEY (persona_id)        REFERENCES personas(id)      ON DELETE CASCADE
     "fk_user_memory_source_message" FOREIGN KEY (source_message_id) REFERENCES chat_messages(id) ON DELETE SET NULL  ✅
     "fk_user_memory_user"           FOREIGN KEY (user_id)           REFERENCES users(id)         ON DELETE CASCADE
     表清单：只有 user_memory（不是 user_memories）✅     CHECK 约束：0 条 ✅

A3) pg_sequences → 恰好 4 条：chat_messages_id_seq / personas_id_seq / user_memory_id_seq / users_id_seq
     → 没有 user_memory_user_id_seq 之类的污染 ✅

A4) 级联      ：删前 persona1 记忆数 = 1 → DELETE FROM personas → 残留 = 0                    ✅
A5) 默认值+精度：0.850 / embedding_id 为 NULL / pending / source_message_id = 1                ✅
A6) SET NULL  ：DELETE FROM chat_messages → 记忆行仍在、0.850、溯源已断开 = t                    ✅
     （另有 F 项：手工插入 memory_type='emotion' 成功 → **实测确认** DB 层无兜底，与 spec §3.3 的
       "代价（诚实记录）"一致。拦截只能靠落库前校验，属成员 1 的提取链路）
A7) GORM Create：id=4 importanceScore=0.85 embeddingStatus="pending" embeddingID=<nil>
     users=1 personas=1 chat_messages=0 → 零值关联字段**未**连带写库 ✅
     （且 embeddingStatus 由 "" 变成 "pending" → 实测复现了「GORM 显式写入而非 DB 默认值」）
A8) 幂等      ：再次 AutoMigrate 成功；fk数=3 / 索引数=4 / 列数=10，与首次完全一致            ✅
A9) 代码 grep ：type:bigserial=0 / check:=0 / json:"-"(锚定)=7 / MemoryType=1 / column:=10     ✅
                 对外 JSON 名（column: 行排除 json:"-"）=6                                       ✅
A10) 红线 1   ：无 .env / .key / .pem；临时库已 dropdb，开发库一行未碰                          ✅
```

> ⚠️ **同一轮实测出的六类"假失败"**（也是这轮修文档的原因）——**全部由"照着自己写的文档实跑一遍"暴露**：
>
> | # | 陷阱 | 实跑值 vs 期望 |
> |---|---|---|
> | ① | 裸 grep 会把注释里的解释当成违规（`grep -c "bigserial"`=**2**、`grep -c 'json:"-"'`=**10**、`grep -ci "emotion"`=**5**，全来自"**不要**写成 bigserial"这类**正确**的说明） | 全部改锚定 tag |
> | ② | `json:"-"` 我按 4 个数据列数过，漏了 3 个关联字段 | **7**（不是 4） |
> | ③ | `grep "MemoryType string"` 命中 `type MemoryType string` 这行**类型声明本身** | 改锚定字段声明行 |
> | ④ | `\d` 显示 `id bigint + nextval` 被误判成"写成了 bigint 不是 bigserial" | 这是**正确**结果（PG 不显示 `bigserial` 这个字），失败信号是**别的列**长出 `nextval` |
> | ⑤ | "抽对外 JSON 名 = 6"只给结论没给完整命令；只跑前两步输出 **10 行** | 必须三步并 `grep -v 'json:"-"'` |
> | ⑥ | 期望 0 的 grep 用 `&&` 串联时，**第二条就断链**（无匹配 → 退出码 1），后四条静默不跑 | 逐条跑，不串联 |

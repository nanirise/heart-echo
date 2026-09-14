# plan · 聊天记录 CRUD（Chat Message）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-chat-message-model`
> 负责人：成员 3 ｜ 创建：2026-09-14 ｜ 最后更新：2026-09-14（第 1 版）

> ⚠️ **先读 spec §4.3**：本功能名里的「CRUD」只有 **R** 是端点。C 由服务端落库（SSE / 主动消息 / 日程提醒三条链路复用同一个 `Create`），U / D 在 DDL 与契约两层都不存在——**不该写的代码比该写的更重要**。
> ⚠️ **步骤 0-5 不等任何人**（只依赖已合入的 `pkg/errcode`）；步骤 6-7 依赖成员 1 尚未落地的 `internal/middleware` 与 `router.go`。**不要为了跑起来自己造一份**。

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 前置 | 状态 |
|---|---|---|---|---|---|
| 0 | 通用分页结构 | `internal/dto/common_dto.go` | `PageResult[T]` + `PageSizeDefault/PageSizeMax` + `ClampPage()` 只有这一份；persona 分支若已落地则**直接消费** | 无 | 未开始 |
| 1 | **翻译 Struct** | `internal/model/chat_message.go` | 9 列 + 2 个外键关联 + `role` 的 CHECK 全被声明；字段对照 spec §3.2 逐行对齐；每字段带注释 | 无 | 未开始 |
| 2 | 建表 | `internal/model/migrate.go` | `AutoMigrate` 追加 `&ChatMessage{}`，顺序在 `&Persona{}` 之后；`\d chat_messages` 有 2 个 CASCADE 外键 + CHECK + 2 个索引 | 步骤 1 | 未开始 |
| 3 | 请求 / 响应结构 | `internal/dto/chat_dto.go` | `ChatMessageResponse`（8 字段）+ `NewChatMessageResponse` + `MessageListQuery`；camelCase 对齐契约 §5 | 步骤 0、1 | 未开始 |
| 4 | 归属判定 | `internal/repository/persona_repo.go` 的 `ExistsOwnedByUser` | 一条查询、两个条件；**只回 bool**，调用方拿 false 一律 4043 | 无 | 未开始 |
| 5 | **仓储层** | `internal/repository/message_repo.go` | `ListByPersona`（含 `total`）与 `Create`；签名强制带 `userID` 与事务句柄；**不存在**任何不带 `user_id` 的业务查询、不存在删除方法 | 步骤 1 | 未开始 |
| 6 | **业务层** | `internal/service/chat_service.go` | **先判归属再查消息**；未命中一律 `4043`；分页钳制在此；不依赖 `*gin.Context` | 步骤 3、4、5 | 未开始 |
| 7 | **HTTP 层** | `internal/handler/chat_handler.go` | 薄；含 `RegisterChatRoutes`；错误 `_ = c.Error(err)` 上抛；`PersonaId` 解析失败传 0 | 步骤 6 + 成员 1 的 `middleware` / `response` | 未开始 |
| 8 | 路由挂载 | `router.go`（成员 1 的文件） | 群里同步后加一行，**不要与成员 1 同时改**；`POST /chat/stream` 由他那一步在同一个 `RegisterChatRoutes` 里追加，**现在不要写空壳路由** | 步骤 7 | 未开始 |
| 9 | 越权 / 分页 / 级联专项验证 | — | spec §8 的 20 项验收逐条打勾 | 步骤 8 | 未开始 |
| 10 | **人工审查 + PR** | `docs/dev_notes/chat_message_notes.md`、PR | §4 的审查清单全过；至少 1 人 Approve | 步骤 9 | 未开始 |
审查 ChatMessage.ID，严禁出现 bigserial，必须用 bigint + autoIncrement。
审查 total 的查询条件，必须带 user_id + persona_id，防止全站数据泄露。1
审查 GET 接口逻辑，必须先判归属，后查消息，越权一律 4043。

**优先级**：步骤 0-5 是**流式对话（Week 2 生死线）的前置**——SSE 链路要 `Create` 才能落库，所以仓储层先于端点交付。等待成员 1 的中间件期间，用 §4.4 的 SQL 直接对 PG 验证仓储层行为。

## 2. 文件清单

| 路径 | 作用 | 谁还会用到 |
|---|---|---|
| `backend/internal/dto/common_dto.go` | `PageResult[T]` + 分页常量 + `ClampPage`，**全项目五处分页共用** | **成员 1**（messages / memory）、成员 3（personas / moments / schedules） |
| `backend/internal/model/chat_message.go` | `chat_messages` 的 GORM 实体 | **成员 1**（SSE 落库）、成员 3（主动消息 / 日程） |
| `backend/internal/model/migrate.go` | 建表入口（追加一行） | 成员 1（`main.go` 调用） |
| `backend/internal/dto/chat_dto.go` | 消息响应结构 + 构造器 + 列表查询参数 | **成员 2**（据此写 `types/chat.ts` 与 Mock） |
| `backend/internal/repository/persona_repo.go` | 增 `ExistsOwnedByUser`（**人设模块的文件，本分支一并落地**） | **成员 1**（SSE 链路判归属；后续会再要 `GetOwned`） |
| `backend/internal/repository/message_repo.go` | `ListByPersona` / `Create` | **成员 1 + 成员 3**（三条写入链路共用 `Create`） |
| `backend/internal/service/chat_service.go` | 归属校验 + 分页 + DTO 转换 | 成员 1（SSE 的 service 与它同文件或同包） |
| `backend/internal/handler/chat_handler.go` | 1 个端点 + `RegisterChatRoutes` | 成员 1（他往里加 `POST /chat/stream`） |
| `docs/dev_notes/chat_message_notes.md` | **我的审查笔记**（不是给别人的文档） | 只有我；与 `user_model_notes.md` 同级 |

## 3. 关键实现要点

### 3.1 翻译 Struct（步骤 1）

**手法沿用 `docs/dev_notes/user_model_notes.md`**：每个字段一行注释，把对应的 DDL 原文贴在 GORM tag 上方。**先写带全注释的版本进 dev_notes，再落 `chat_message.go`。**

**四处最容易翻车**（完整对照表见 spec §3.2）：

1. **`ID` 的 tag 是 `type:bigint` + `autoIncrement`，不是 `type:bigserial`**：`user_memory.source_message_id` 引用本表主键，GORM 会把这里的 DataType 复制过去（`user.go` / `persona.go` 踩过同一个坑）。写 `bigserial` 会让下游外键列长出 `nextval` 默认值，漏传值时**静默指向一个不存在的消息**。
2. **`user_id` 用 `json:"-"`**：契约 §5 的 ChatMessage 实体**没有** `userId`（8 个字段逐个数一遍）。它同时被 `User` 的外键关联引用，是越权防线的载体。
3. **必须声明 `User` 与 `Persona` 两个关联字段**，否则 GORM **一个外键都不建**——"删人设级联删消息"与"删账号清理数据"两条验收项直接不成立。两个字段都 `json:"-"`，且 `Create` 时**不要赋值**（保持零值），否则 GORM 会尝试连带写 `users` / `personas` 表。
4. **`role` 用自定义类型 + 常量**（spec §3.3 第 2 条），并把 DDL 的 CHECK 翻成 `check:` tag。**CHECK 是最容易在"翻译 Struct"时整行丢掉的东西**——它不影响编译、不影响跑通，只在数据脏了以后才暴露。

```go
type MessageRole string

const (
    RoleUser      MessageRole = "user"
    RoleAssistant MessageRole = "assistant"
)

// 对应 SQL：role VARCHAR(10) NOT NULL CHECK (role IN ('user', 'assistant'))
//   check: 渲染出的约束名以 \d chat_messages 实际输出为准（GORM 默认 chk_chat_messages_role）。
//   这是"消息不能长成别的角色"的结构性保证；Go 层的常量是它前面的一道，两道都要有。
Role MessageRole `gorm:"column:role;type:varchar(10);not null;check:role IN ('user','assistant')" json:"role"`

// 对应 SQL：emotion_label VARCHAR(20)
//   指针：契约要求可以为 null（词典档分析不出时就是空）。值类型会把 null 变成 ""，
//   前端拿到 "" 会以为"有情绪但标签是空串"，与"没分析出"是两回事。
EmotionLabel *string `gorm:"column:emotion_label;type:varchar(20)" json:"emotionLabel"`
```

> **`emotion_score` 用 `*float64`**：`NUMERIC(4,3)` 的可空列。写入方（SSE 那一步）如果实测 pgx 编码不进去，退到自写 `driver.Valuer` 返回字符串——**但不要为了它引入 `shopspring/decimal` 之类的依赖**（`go.mod` 目前只有 gorm / gin / jwt / env / godotenv / zap 那一套，加一个只为一列的依赖不划算）。

### 3.2 建表（步骤 2）

- `migrate.go` 的 `AutoMigrate` 里追加 `&ChatMessage{}`，**顺序在 `&Persona{}` 之后**（它引用 `users` 与 `personas`）。
- 建完在真库上核对（`\d chat_messages`）：**两个** `ON DELETE CASCADE` 外键、`role` 的 CHECK、两个索引、**没有** `updated_at` / `deleted_at`。
- **顺手核一下序列**：`\ds` 里应当只有 `chat_messages_id_seq`。如果冒出了 `chat_messages_persona_id_seq` / `chat_messages_user_id_seq` 之类，就是 §3.1 第 1 条那个 DataType 复制坑又发生了。
- 重复执行一次 `AutoMigrate`，确认幂等（persona 那轮验证过同样的手法）。

### 3.3 DTO（步骤 0、3）

**`common_dto.go` 只放分页相关的东西**（AGENTS §3：不新建杂物间文件），且**必须只有一份**：

```go
type PageResult[T any] struct {
    List     []T   `json:"list"`
    Total    int64 `json:"total"`
    Page     int   `json:"page"`
    PageSize int   `json:"pageSize"`
}

const (
    PageSizeDefault = 20
    PageSizeMax     = 100
)

// ClampPage 把分页参数收敛到合法区间，非法输入取默认值（不报错）。
// 契约给分页端点只列了业务错误码，分页参数不合法不该凭空多出一个 4001。
// 保证返回的 pageSize >= 1：GORM 的 Limit(0) 会生成 LIMIT 0，静默返回空列表。
func ClampPage(page, pageSize int) (int, int) {
    if page < 1 { page = 1 }
    if pageSize < 1 { pageSize = PageSizeDefault }
    if pageSize > PageSizeMax { pageSize = PageSizeMax }
    return page, pageSize
}
```

- `ChatMessageResponse` **8 个字段**，逐个对着契约 §5 的 JSON 示例核（**没有 `userId`**）。
- 构造器 `NewChatMessageResponse(m *model.ChatMessage)` 只做字段搬运，**不做任何裁剪**：情绪两个字段照契约返回（**回读字段，由 UI 保证不渲染**，spec §3.3 第 3 条）。
- `MessageListQuery` 只声明 `Page` / `PageSize` 两个 `form` 字段——**不要声明 `userId` / `personaId`**：路径参数已经在 URL 里，query 里再声明一个就等于开了一个"前端能指定 personaId"的口子。

### 3.4 Repository（步骤 4、5）

**核心是"让不安全的方法不存在"**，而不是"记得每次都校验"：

```go
// persona_repo.go —— 归属语义的唯一实现，chat 侧不许再写一份 SELECT ... FROM personas
// 只回 bool：调用方拿到 false 一律按「人设不存在」返回 4043，不允许区分「不存在」与「不属于你」。
func (r *PersonaRepository) ExistsOwnedByUser(ctx context.Context, userID, personaID uint64) (bool, error) {
    var n int64
    err := r.db.WithContext(ctx).Model(&model.Persona{}).
        Where("id = ? AND user_id = ?", personaID, userID).
        Count(&n).Error
    return n > 0, err
}
```

```go
// message_repo.go —— 方法签名强制带 userID，调用方没有"忘了传"的选项
func (r *MessageRepository) ListByPersona(ctx context.Context, userID, personaID uint64, offset, limit int) ([]model.ChatMessage, int64, error)
func (r *MessageRepository) Create(ctx context.Context, tx *gorm.DB, m *model.ChatMessage) error
```

**列表查询**（两处条件必须逐字相同——口径不一致会让 `total` 与 `list` 对不上）：

```go
var (
    list  = make([]model.ChatMessage, 0) // 初始化：nil 会序列化成 null，前端 .map 直接崩
    total int64
)
where := "persona_id = ? AND user_id = ?" // total 与 list 共用一个口径

// 注意：两条链分开写。GORM 的 finisher 方法（Count / Find）会改写同一条 statement，
// 在同一个 *gorm.DB 上先 Count 再 Find 会互相污染（Select 子句被换成 count(*)）。
if err := r.db.WithContext(ctx).Model(&model.ChatMessage{}).
    Where(where, personaID, userID).Count(&total).Error; err != nil {
    return nil, 0, err
}
if err := r.db.WithContext(ctx).Model(&model.ChatMessage{}).
    Where(where, personaID, userID).
    Order("created_at DESC, id DESC").    // id 兜底不能省：NOW() 是事务时间，同事务多行时间戳相同
    Offset(offset).Limit(limit).
    Find(&list).Error; err != nil {
    return nil, 0, err
}
return list, total, nil
```

**`message_repo.go` 里刻意没有的东西**（spec §4.3 / §5.4）：

| 不存在 | 为什么 |
|---|---|
| `FindByID` / `ExistsByID` | 本功能没有任何"按 id 取一条消息"的场景；多一个方法就多一处泄漏面 |
| `Delete` / `DeleteByPersona` | 删消息只有一条路：删人设 → 外键级联 |
| `Update` / `Save` | 表里没有 `updated_at`，也没有"编辑消息"这个功能 |
| 任何不带 `user_id` 条件的查询 | 红线 3 |

**`Create` 的三个约束**：

1. **`tx *gorm.DB` 必传，不写 `if tx == nil { tx = r.db }` 这种兜底**——它会让"忘了传事务"静默变成"不在事务里"，而调用方（SSE 落库 + 更新 `last_message_at`）恰恰依赖同事务。缺少事务时 `last_message_at` 的更新不会被回滚，事务等于白做。
2. 只赋 `UserID` / `PersonaID` 两个标量，**不给 `User` / `Persona` 关联字段赋值**。
3. `user_id` 由调用方从 Token / 人设行取，**repo 不做任何"猜"**。

### 3.5 Service（步骤 6）

```go
func (s *ChatService) ListMessages(ctx context.Context, userID, personaID uint64, page, pageSize int) (*dto.PageResult[dto.ChatMessageResponse], error) {
    if userID == 0 {
        return nil, errcode.New(errcode.ErrUnauthorized) // 鉴权失败不许静默降级成空历史
    }

    // personaID == 0（路径解析失败）不需要特判：EXISTS 不命中 → 同一个 4043 出口。
    // 少一个分支 = 少一处可能与主路径不一致的出口。
    owned, err := s.personaRepo.ExistsOwnedByUser(ctx, userID, personaID)
    if err != nil {
        return nil, errcode.Wrap(errcode.ErrDBFailed, err)
    }
    if !owned {
        return nil, errcode.New(errcode.ErrPersonaNotFound) // 4043：不存在与不属于你，同一个答案
    }

    page, pageSize = dto.ClampPage(page, pageSize)
    offset := (page - 1) * pageSize

    list, total, err := s.messageRepo.ListByPersona(ctx, userID, personaID, offset, pageSize)
    if err != nil {
        return nil, errcode.Wrap(errcode.ErrDBFailed, err)
    }

    items := make([]dto.ChatMessageResponse, 0, len(list))
    for i := range list {
        items = append(items, dto.NewChatMessageResponse(&list[i]))
    }
    return &dto.PageResult[dto.ChatMessageResponse]{List: items, Total: total, Page: page, PageSize: pageSize}, nil
}
```

- 签名收 `context.Context`，**不出现 `*gin.Context`**（红线 7）。
- 错误一律 `errcode.New` / `errcode.Wrap`，**不拼接自定义文案**（红线 6）。
- **不出现 `4030`**（那是功能越权的码，本模块没有该场景），也**不出现任何裸错误码数字**。

### 3.6 Handler（步骤 7）

```go
func (h *ChatHandler) ListMessages(c *gin.Context) {
    userID, ok := middleware.GetUserID(c) // 具体取法随成员 1 的中间件（c.GetUint64 + ContextKeyUserID）
    if !ok {
        _ = c.Error(errcode.New(errcode.ErrUnauthorized))
        return
    }
    // 解析失败 → 0 → service 里自然走到 4043。不要在这里返回 4001（契约本端点只列了 4043）。
    personaID, _ := strconv.ParseUint(c.Param("personaId"), 10, 64)

    var q dto.MessageListQuery
    _ = c.ShouldBindQuery(&q) // 故意忽略 binding 错误：非法分页参数取默认值，不产生 4001

    data, err := h.svc.ListMessages(c.Request.Context(), userID, personaID, q.Page, q.PageSize)
    if err != nil {
        _ = c.Error(err)
        return
    }
    response.Success(c, data)
}

func RegisterChatRoutes(rg *gin.RouterGroup, h *ChatHandler) {
    g := rg.Group("/chat")
    g.GET("/personas/:personaId/messages", h.ListMessages)
    // POST /chat/stream 由 Week 2 那一步（成员 1）在此追加，现在不要写空壳路由
}
```

- Handler 只做三件事：**取 userID → 解析参数 → 调 service → `response.Success`**；出错 `_ = c.Error(err)` 后 `return`。**不写 `response.Fail`**。
- 路径参数名 `:personaId` 与契约 §1「路径参数用 camelCase 且与字段同名」一致。

---

## 4. 我手动审查 AI 代码的计划

> 前提：AI 生成的代码**默认不可信**，尤其是「越权防线」「分页边界」「不该写的代码有没有被顺手写上」这三处——它们写错了照样能跑通、照样能演示，只在特定输入下才暴露。审查的**目标不是"能跑"，是"我能逐行解释它为什么安全"**。

### 4.1 审查方法（三遍，沿用审 `User` / `Persona` struct 时的做法）

| 遍 | 做什么 | 判据 |
|---|---|---|
| **第一遍 · 逐行重写注释** | 把 AI 产出的每个文件抄进 `docs/dev_notes/chat_message_notes.md`，**用自己的话**给每一行加注释 | **写不出注释的那一行，就是我没看懂的地方** → 去查 GORM 文档或问，不放过 |
| **第二遍 · 对照核对** | 拿 DDL（spec §3.1）+ 契约 §5 + spec 全文，逐字段/逐端点核对，填成表格 | 每个 JSON 字段名都能在契约里指到来源；**响应里多出来的字段一律删掉**（尤其 `userId`） |
| **第三遍 · 对抗验证** | 主动构造能打破代码的输入（跨账号、空对话、不存在的 id、`pageSize=100000`、`page=abc`、`personaId=abc`、同秒消息、越界页），按 §4.4 的命令实跑 | 每个高危点都有一次实跑记录，不是"读代码觉得没问题" |

**第三遍是这份计划的核心**：前两遍只能证明"代码符合我的预期"，第三遍才能证明"代码在我不期望的输入下不泄漏、不崩、不静默返回错东西"。AI 写的代码在前两遍往往很干净。

### 4.2 逐文件审查清单

| 文件 | 我要盯的点 | 看懂的标准 |
|---|---|---|
| `dto/common_dto.go` | `PageResult[T]` 是否**只有这一份**；`ClampPage` 是否保证 `pageSize >= 1` 且 `<= 100`；非法值是否**取默认值而不是报错** | 全项目 `grep "type PageResult"` 只有 1 处命中；我能说出 `Limit(0)` 会发生什么 |
| `model/chat_message.go` | `ID` 是否 `type:bigint`（不是 `bigserial`）；`user_id` 是否 `json:"-"`；情绪两字段是否指针；`role` 是否带 `check:`；两个关联字段是否都在且都 CASCADE | 我能说出**每个 tag 去掉之后会发生什么**（尤其 `ID` 那个） |
| `model/migrate.go` | 追加的那一行是否在 `&Persona{}` 之后；有没有顺手加别的表 | `AutoMigrate` 一次跑过，无依赖顺序报错 |
| `dto/chat_dto.go` | 有没有偷偷声明 `userId` / 让 query 里出现 `personaId`；8 个字段是否与契约逐字一致 | 与契约 §5 的 JSON 示例并排比对，字段名一个字母都不差 |
| `repository/message_repo.go` | **每个方法签名是否都带 `userID`**；有没有 `db.First(&m, id)` 形态的裸查询；有没有 `Delete` / `Update` / `Save`；`Create` 是否**强制要 `tx`**（无 nil 兜底）；`Order` 是否带 `id DESC`；`list` 是否 `make(..., 0)` | `grep -nE "Delete\(|Save\(|First\(&|ExistsBy" message_repo.go` 无业务命中 |
| `repository/persona_repo.go` | 新增的 `ExistsOwnedByUser` 是否 `WHERE id = ? AND user_id = ?`；是否**只回 bool**；有没有顺手加别的探针 | 方法体不超过 5 行，且没有第二个 `personas` 查询出口 |
| `service/chat_service.go` | **顺序是不是"先判归属、再查消息"**；未命中是否统一 `4043`（**不出现 4030**）；分页钳制是否在 service；是否引了 `gin`；是否出现裸数字错误码 | `grep -n "gin" chat_service.go` 无结果；`grep -nE "\b(400[0-9]\|403[0-9]\|404[0-9])\b"` 无结果 |
| `handler/chat_handler.go` | 是否 `_ = c.Error(err)` 上抛；有没有 `response.Fail`；`userID` 是否只从 JWT 上下文取；`personaId` 解析失败是否传 0（不是 4001）；有没有多写 `POST /chat/messages` 之类的空路由 | `grep -n "response.Fail" chat_handler.go` 无结果 |
| `router.go` | 只加了一行，没动别人的行 | `git diff router.go` 只有 1 行新增 |

### 4.3 七个高危点专项审查

| # | 高危点 | 为什么会错 | 我怎么验 |
|---|---|---|---|
| 1 | **越权被伪装成空对话** | "只查消息表、查不到就返回空列表"是最自然、也最像对的写法——代码读起来完全合理，**但它把"看别人的对话"变成了 `200 + []`**。这是本功能头号风险，且完全静默 | 用**另一个账号的 Token** 打 A 的 `personaId`：必须 `4043`；再拿一个**自己没聊过的人设**打：必须 `200 + []`。两次结果**必须不同**，这才证明归属判定真的生效了 |
| 2 | **归属判定返回三态 / 多一个错误码** | AI 容易写 `if !exists { 4043 } else if !owned { 4043 }` 这类"看起来更精确"的分支，或给非法 `personaId` 返回 `4001` | `grep` 确认 service 里只有**一处** 4043 出口；各种非法输入实跑一遍，响应里**只出现 `4043` / `4010` / `200`** |
| 3 | **分页不设上限 / `total` 口径错** | `pageSize` 无上限 = 一次拉全表；`COUNT(*)` 全表 = **把全站消息量级泄漏给任意用户** | `pageSize=100000` → 最多 100 条、响应 `pageSize=100`；两个账号各造消息，互相核对 `total` 只是**自己那个人设**的消息数 |
| 4 | **排序不稳** | 只写 `created_at DESC`，同秒消息顺序不定 → 翻页重复或漏项，**但页面看着"能滚动"** | 手工把两条消息的 `created_at` 改成完全相同，`pageSize=1` 逐页拉，逐条核对不重不漏 |
| 5 | **空列表是 `null`** | Go 的 nil slice 序列化成 `null`，前端 `.map` 崩；而在后端看响应体之外的地方一切正常 | 构造"人设属于我但没消息" → 响应体里必须是 `"list":[]`；越界页同样 |
| 6 | **不该写的代码被顺手写上** | 最容易出现的是：给消息加 `DELETE`（违反总纲 §0）、加 `Save()`、给 `migrate.go` 顺手加 `updated_at`、或者给 SSE 预埋一个 `emotion` 事件 | `grep -rnE "DELETE FROM chat_messages\|func.*Delete\|updated_at\|deleted_at\|emotion\"" internal/` 逐条确认命中都合理 |
| 7 | **写入路径的 `last_message_at` / `state`** | 落库后对话列表不刷新（漏更新），或用 `Save(&persona)` 把 `state` 覆盖成零值（`familiarity` 直接归零，**不可逆**） | 走一次 `Create` + 更新，`psql` 里核对 `last_message_at` 等于消息时间、`state` 一字未变（先手工塞 `{"familiarity":42,"self_note":"x"}` 再验） |

### 4.4 必须亲眼看到的证据（不接受"我觉得没问题"）

```bash
# 0) 编译 + 静态检查（提交前必跑）
cd backend && go build ./... && go vet ./... && go test ./...

# 1) 表结构真的建对了吗（命令里不要写密码，红线 1）
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c '\d chat_messages'
#    要看到：FOREIGN KEY ... REFERENCES users(id) ON DELETE CASCADE
#            FOREIGN KEY ... REFERENCES personas(id) ON DELETE CASCADE
#            CHECK (role IN ('user','assistant'))
#            idx_messages_persona_time / idx_messages_user_id
#            且 **没有 updated_at / deleted_at 列**
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c '\ds'
#    要看到：只有 chat_messages_id_seq（多出 persona_id/user_id 的序列 = DataType 复制坑）

# 2) 越权 vs 空对话 vs 不存在（三条必须给出两种不同结果）
curl -s "localhost:8080/api/v1/chat/personas/<A的人设>/messages" -H "Authorization: Bearer $TOKEN_B"
#    期望 4043
curl -s "localhost:8080/api/v1/chat/personas/<A的人设>/messages" -H "Authorization: Bearer $TOKEN_A"
#    期望 200 + list 有内容；再拿 A 的「没聊过的人设」打 → 200 + []
curl -s "localhost:8080/api/v1/chat/personas/999999/messages" -H "Authorization: Bearer $TOKEN_A"
#    期望 4043，且 message 与第 1 条**逐字相同**

# 3) 非法输入不产生 4001 / 4030
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
curl -s "localhost:8080/api/v1/chat/personas/<id>/messages" -H "Authorization: Bearer $TOKEN"
#    期望：null / null（不是 "" 和 0）；0.87 读出来是 0.87（NUMERIC ↔ *float64 往返）

# 6) 级联删除
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c \
  'SELECT count(*) FROM chat_messages WHERE persona_id = <已删id>;'   # 删人设后必须是 0

# 7) `last_message_at` 不变量（走一次 Create + 更新）
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c \
  "UPDATE personas SET state = '{\"familiarity\":42,\"self_note\":\"x\"}' WHERE id = <id>;"
#    写一条消息后再查：last_message_at 等于消息时间，state 两个键都还在、familiarity 仍是 42

# 8) code 层面四连 grep（都要无输出）
grep -rn "response.Fail"        internal/handler/chat_handler.go
grep -rnE "Delete\(|Save\("     internal/repository/message_repo.go
grep -rnE "\b(400[0-9]|403[0-9]|404[0-9])\b" internal/service/chat_service.go
grep -rn "gin"                  internal/service/chat_service.go
grep -rn "type PageResult"      internal/dto/     # 期望只有 1 行

# 9) 红线 1 自查：不许有密钥进入本次改动
git status --short && git diff --cached --name-only | grep -E '\.env$|\.key$|\.pem$'
```

> `$TOKEN` / `$TOKEN_A` / `$TOKEN_B` 从登录接口拿，**不要写死在脚本或 `_test.go` 里**（红线 1）。需要真实数据时用环境变量或本地 `.env`（已在 `.gitignore` 中）。

### 4.5 拒绝标准（出现任一条就打回重写，不做"小修小补"）

| 打回条件 | 为什么不能只小修 |
|---|---|
| **越权请求返回 `200 + []`**（没判归属，或判了没用上） | 这是本功能最严重的错误：把"读别人的对话"伪装成"对话是空的"。防线必须是结构性的，逐处打补丁会漏 |
| 仓储层存在**不带 `user_id`** 的业务查询 | 同上，最后一道闸门失守 |
| service 里 4043 有多处出口 / 出现 `4030` / 出现 `4001` | 出口一多就会有分支漏改；多出的错误码 = 可观测差异 = 泄漏存在性 |
| `total` 用了不带 `persona_id` + `user_id` 条件的 `COUNT` | 数字错 + 泄漏全站量级 |
| `ListByPersona` 排序缺 `id DESC` | 翻页重复/漏项，且**页面看着正常**，最难排查 |
| 空列表返回 `null` | 前端直接崩 |
| 出现 `Delete` / `Update` / `Save` 的消息写法 | 违反总纲 §0（不可单独清空）；`Save` 覆盖 `state` 是不可逆的数据损坏 |
| `Create` 里有 `tx == nil` 的兜底分支 | 会让"忘了传事务"静默变成"不在事务里"，事务白做 |
| `ChatMessage.ID` 写成 `bigserial` | 下游 `user_memory.source_message_id` 长出 `nextval`，**今天看不出来，写记忆表那天才炸** |
| 多出一个 `POST /chat/messages` / `DELETE /chat/messages/:id` 之类的空壳路由 | 预留路径会被后来的人当成"已规划的功能"实现出来，与总纲 §0 冲突 |
| service 里出现 `*gin.Context` | 破坏分层（红线 7），后续无法单测 |
| `PageResult` 出现第二份定义 | 两份同名类型在联调时会出现"字段对不上"的诡异问题，最难排查 |

**审查通过的唯一标准**：§4.4 的 9 组命令全部实跑过，且 §4.2 每个文件我都能逐行解释。**审查笔记落到 `docs/dev_notes/chat_message_notes.md`**，作为"我确实看懂了"的证据留档。

## 5. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| **越权被伪装成空对话** | 读别人的聊天记录，且**完全静默**——没有报错、没有异常日志，只有内容不对 | §5.2 的三步顺序 + §4.3 #1 的"两种结果必须不同"实跑；这是本功能唯一必须演给 review 看的证据 |
| 成员 1 的 `middleware` / `router.go` 尚未落地 | 步骤 7-8 无法编译，端点不可达 | 先做步骤 0-5（不等任何人）；**不要自造 `pkg/middleware`**；等待期用 §4.4 的 SQL 直接验仓储层 |
| **两人同时写 `chat_*.go`** | 同 AGENTS §4.8 的理由：`router.go` / `chat_handler.go` 都是两人交汇点，同时改必冲突 | 步骤 10 前先在群里对齐（spec §7.3 #1）；`RegisterChatRoutes` 由成员 1 追加 `POST /chat/stream` 那一行，**同一时间只有一个人动这个文件** |
| `PageResult` 出现两份（persona 分支与本分支各写一份） | 联调时字段对不上，最难排查 | 本功能第一个提交就把 `common_dto.go` 落地并在群里说一声（spec §7.3 #2）；后落地的一方直接消费 |
| `emotion_score` 的 pgx 编码问题 | SSE 落库时才发现写不进去 | 本功能先定 `*float64`；**在 SSE 那一步写入前**用一条真实 INSERT 验一次（spec §7.3 #3），不行就换 `driver.Valuer` |
| 写入方漏更新 `last_message_at` | 对话列表顺序不刷新、主动消息空闲判定永远算不出来（表现为"AI 永远不主动发消息"） | §4.4 不变量 2 写进 `Create` 的注释 + 交接给成员 1 / 成员 3；验收里专门有一条 |
| `[nudge]` 的内容格式未定 | 两个模块各存一份格式，前端渲染时对不上 | spec §7.3 #5：由主动消息模块与 SSE 约定一次，本功能只原样存取 |
| `isNudge=true` 的消息前端渲染方式未对齐 | 前端可能把 `[nudge]` 原文当用户的话显示出来 | spec §7.2 已列为交接项，与成员 2 单独对齐（后端不参与） |

## 6. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-14 | spec / plan 第 1 版落地；确认契约 §5 该端点错误码已收敛为 `4043`（无需改全局文件、不阻塞合并） | 成员 1 的 `internal/middleware` / `router.go` 未落地（仅阻塞步骤 7-8）；文件归属待群里对齐（spec §7.3 #1） |

# plan · 人设 CRUD（Persona CRUD）· 接口层

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-persona-crud`
> 负责人：成员 3 ｜ 创建：2026-09-16 ｜ 最后更新：2026-09-16（v1）
> 前置文档：[persona-model/plan.md](../persona-model/plan.md)（模型层，步骤 0-3 已完成）｜ [MASTER.md §4.5](../dev/MASTER.md)（路由注册方式，已冻结）

> 📌 **范围已定（2026-09-16 用户裁定）：本 PR = 6 个文件**（4 个核心 + 2 个前置 `dto/common_dto.go`、`model/proactive_setting.go`）。两者都属成员 3 的职责范围（10 张表设计 + 分页结构），**不是越界**。**PR 描述里要写明扩大范围的理由**——草稿见 §6。

---

## 1. 目标与产出物

| # | 文件 | 作用 | 规模预估 | 前置 | 在本 PR 内 |
|---|---|---|---|---|---|
| 0 | `internal/dto/common_dto.go` | `PageResult[T]` + 分页常量 + `ClampPage`（五处分页共用） | ~25 行 | 无 | ✅ 前置 |
| 1 | `internal/model/proactive_setting.go` + `migrate.go` 一行 | `proactive_settings` 实体；`POST /personas` 播种的前置 | ~35 行 + 1 行 | 无 | ✅ 前置 |
| 2 | `internal/dto/persona_dto.go` | 请求 / 响应结构；`familiarity` 拍平 | ~60 行 | 0、1 | ✅ 核心 |
| 3 | `internal/repository/persona_repo.go` | 6 个方法，签名全部带 `userID` | ~90 行 | 2 | ✅ 核心 |
| 3b | `internal/repository/proactive_repo.go` | `CreateDefaultSettings` 一个函数 | ~25 行 | 1 | ✅ 核心 |
| 4 | `internal/service/persona_service.go` | 归属判定 + 事务编排 + DTO 转换 | ~110 行 | 3、3b | ✅ 核心 |
| 5 | `internal/handler/persona_handler.go` | 4 个端点 + `RegisterPersonaRoutes` | ~110 行 | 4 | ✅ 核心 |

**不产出**：`router.go`（成员 1 加一行）、任何前端文件、任何 `pkg/**`。

**为什么要带上 2 个前置文件**（详见 spec §2.1）：`GET /personas` 的响应类型是契约 §4 写死的 `PageResult<Persona>`，缺 `common_dto.go` 就只能返回裸数组；`POST /personas` 要同事务播种 `proactive_settings`（persona-model §4.3.1），缺 `proactive_setting.go` 就违反已定的设计，主动消息模块接手时会撞上孤儿配置。两者都归成员 3，**是本 PR 的正式交付物，不是顺手带的**。

---

## 2. 施工步骤

| # | 步骤 | 完成标准 | 前置 | 状态 |
|---|---|---|---|---|
| 0 | `dto/common_dto.go` | `PageResult[T]` + `ClampPage` **只有这一份**；字段 `list/total/page/pageSize` | 无 | ✅ 完成 |
| 1 | `model/proactive_setting.go` + `migrate.go` | 8 列 + 2 个关联字段 + 组合唯一索引；`\d` 有 2 条 CASCADE 外键；**三个 `INT` 列是 `integer` 不是 `bigint`** | 无 | ✅ 完成（含一处返工，见 §7 v3、spec §3.2 结论 5） |
| 2 | `dto/persona_dto.go` | 4 个结构体；camelCase 逐字对齐契约 §4；**无 `userId` 字段** | 0、1 | ✅ 完成 |
| 3 | `repository/persona_repo.go` | 6 个方法签名全带 `userID`；**不存在**单条件查询、不存在 `Save` | 1、2 | ✅ 完成 |
| 3b | `repository/proactive_repo.go` | `CreateDefaultSettings` 收 `tx`；四个值显式写出 | 1 | ✅ 完成 |
| 4 | `service/persona_service.go` | 未命中一律 `4043`；创建走事务；事务后不 `Wrap`；不依赖 `*gin.Context` | 3、3b | ✅ 完成 |
| 5 | `handler/persona_handler.go` | 薄；`currentUserID()` 一处读 userID；`_ = c.Error(errcode.New(...))`；不写 `response.Fail` | 4 | ✅ 完成 |
| 6 | 路由挂载 | 群里通知成员 1 在 `router.go` 加一行 `RegisterPersonaRoutes(api, personaHandler)` | 5 | ⏳ 待通知（本分支只交出注册函数） |
| 7 | **A 组验证**（spec §8） | 18 条锚定检查 + 6 组行为实验，**每条必须类都反向验证过** | 5 | ✅ 完成——读数全在 spec §8.6（含 6 次注入，**其中 1 次抓出真缺陷 `bigint`、1 次推翻一条旧结论**） |
| 8 | **B 组验证** | 🚧 阻塞：等成员 1 的 `JWTAuth` + `cmd/server` + `router.go` | 7 + 成员 1 | 🚧 阻塞（未做） |
| 9 | **人工审查 + PR** | spec §8 分组 C 全过；至少 1 人 Approve | 7、8 | ⏳ 等用户人工审查（**代码留在工作区未提交**） |

**优先级与阻塞**：步骤 0-5 **不等任何人**（`pkg/response`、`pkg/errcode`、`middleware` 都已落地）。**步骤 8 等成员 1**——`JWTAuth` 是空壳，4 个端点现在全部返回 `4010`（spec §7.1）。**等待期间不要为了"能跑"给 userID 兜底**，用步骤 7 的非 HTTP 验证路径。

> **步骤 7 的实际情况（2026-09-16）**：A 组全过程在**一次性的 `postgres:16` 容器 + 一个跑完即删的 `cmd/scratchcheck` 脚本**里跑完，两者**都已删除、都不进 PR**。19 项行为断言 + 18 条静态检查**全过**；6 次反向注入**全部响**。**结论写在 spec §8.6，本文不再复述。**

---

## 3. 关键实现要点

### 3.0 `common_dto.go`（步骤 0）

```go
package dto

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// PageResult 五个分页端点共用；只此一份，不要再定义第二个。
type PageResult[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

// ClampPage 收敛分页参数：page<1 → 1；pageSize<1 → 20；pageSize>100 → 100。
// 契约规定的是"默认值与上限"，不是"越界报错"——分页参数不产生错误码。
func ClampPage(page, pageSize int) (int, int) {
	if page < DefaultPage {
		page = DefaultPage
	}
	if pageSize < 1 {
		pageSize = DefaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	return page, pageSize
}
```

**为什么写 `DefaultPage = 1` 而不是裸 `1`**：函数里三个魔数各自含义不同（下限/默认/上限），命名后代码即文档，将来有人要改成 `pageSize` 默认 50 时改一处即可。

**这个文件的红线**：不要往里加别的东西。AGENTS §3 明确禁止杂物间文件（`utils.ts` / `common.go` 之类），而 `common_dto.go` 是为"五处分页共用一份 `PageResult`"而存在的**定点**文件，它一旦开始装别的东西就变成了杂物间。

### 3.1 `proactive_setting.go`（步骤 1）

字段与 tag 的完整对照表见 spec §3.2，**五条**结论也在那里（`uniqueIndex` 的落点、两个关联字段都要、`default:` 会替换零值、表名显式返回、**三个 `INT` 列必须写 `type:integer`**）。**实现时逐条对照，不要凭记忆写。**

```go
// 组合唯一索引：两个字段的索引名必须逐字相同，priority 分别为 1/2。
// 写成两个不同的名字 → 静默建出两个单列唯一索引，语义完全不同（spec §3.2 结论 1）。
UserID uint64 `gorm:"column:user_id;type:bigint;not null;uniqueIndex:uq_proactive_settings_user_persona,priority:1" json:"-"`

// ⛔ 这三个 int 列写 type:integer，写 type:int 会建出 bigint（spec §3.2 结论 5，实测踩过）。
IntervalMin int `gorm:"column:interval_min;type:integer;not null;default:30" json:"intervalMin"`
```

**播种函数（`proactive_repo.go`）**：

```go
// CreateDefaultSettings 创建人设时同步播种一行默认配置，必须与 personas 在同一事务里。
// 四个默认值显式写出是为了可读，不是正确性所必需 —— 留零值入库的也是这组值，
// 因为 GORM 会用 default: tag 的值替换零值（spec §3.2 结论 3，实测）。
func (r *ProactiveRepo) CreateDefaultSettings(ctx context.Context, tx *gorm.DB, userID, personaID uint64) error {
	s := &model.ProactiveSetting{
		UserID:      userID,
		PersonaID:   personaID,
		Enabled:     true,
		IntervalMin: 30,
		IntervalMax: 120,
		DailyLimit:  3,
		// LastNudgeAt 保持 nil：从未触发过
		// User / Persona 关联字段保持零值：赋了值 GORM 会连带去写 users / personas
	}
	return tx.WithContext(ctx).Create(s).Error
}
```

> ⛔ **反向的坑（这个文件最值钱的一条，spec §7.2 已列为对成员 3 的广播项）**：同样因为"零值会被 tag 值替换"，这几列**写不进 `false` / `0`**——`Updates(model.ProactiveSetting{Enabled: false})` 是 `Error=nil`、`RowsAffected=0` 的**静默空操作**。将来 `PUT /proactive/settings` 要关掉主动消息，**必须用 `map[string]any`**。

### 3.2 `persona_dto.go`（步骤 2）

```go
type CreatePersonaRequest struct {
	Name            string `json:"name"            binding:"required,max=50"`
	PersonalityDesc string `json:"personalityDesc" binding:"required,max=2000"`
	SpeakingStyle   string `json:"speakingStyle"   binding:"required,max=255"`
}

// PersonaResponse 契约 §4 冻结的 8 个字段，camelCase 逐字对齐。
// 没有 userId —— 它不进响应体（Persona.UserID 是 json:"-"）。
type PersonaResponse struct {
	ID              uint64     `json:"id"`
	Name            string     `json:"name"`
	PersonalityDesc string     `json:"personalityDesc"`
	SpeakingStyle   string     `json:"speakingStyle"`
	State           model.JSONB `json:"state"`         // 原样透传，不要转 map/string
	Familiarity     int        `json:"familiarity"`    // 从 State 里拍平出来，只读
	LastMessageAt   *time.Time `json:"lastMessageAt"`  // 必须指针：从未聊过为 null
	CreatedAt       time.Time  `json:"createdAt"`
}

// NewPersonaResponse 把实体转成响应体；familiarity 从 state 里解出来。
func NewPersonaResponse(p *model.Persona) PersonaResponse {
	var s struct {
		Familiarity int `json:"familiarity"`
	}
	// 解析失败或键不存在都取 0（语义 = 初识）。DDL 默认值是 '{}'，
	// 历史数据或别的模块写入的 state 都可能没有这个键 —— 不能让列表接口因此 500。
	_ = json.Unmarshal(p.State, &s)

	return PersonaResponse{
		ID:              p.ID,
		Name:            p.Name,
		PersonalityDesc: p.PersonalityDesc,
		SpeakingStyle:   p.SpeakingStyle,
		State:           p.State,
		Familiarity:     s.Familiarity,
		LastMessageAt:   p.LastMessageAt,
		CreatedAt:       p.CreatedAt,
	}
}
```

**三条不要**：不要声明 `userId`；不要把 `State` 转成 `map[string]any`（大整数变 `float64`，且违背"原样透传"）；不要在这里做任何归属判断（那是 service 的职责）。

### 3.3 `persona_repo.go`（步骤 3）

方法清单与签名在 spec §5.2 **已冻结**（成员 1 的 memory 模块直接引 `ExistsOwnedByUser`，改签名等于改接口）。

**列表查询**：

```go
func (r *personaRepo) ListByUser(ctx context.Context, userID uint64, offset, limit int) ([]model.Persona, int64, error) {
	var (
		list  = make([]model.Persona, 0)
		total int64
	)

	// Count 与 Find 的 WHERE 必须逐字相同，否则 total 与 list 对不上
	if err := r.db.WithContext(ctx).Model(&model.Persona{}).
		Where("user_id = ?", userID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// ORDER BY last_message_at DESC NULLS LAST, id DESC
	//   NULLS LAST 不能省：Postgres 的 DESC 默认是 NULLS FIRST，没聊过的人设会排最前。
	//   id DESC 兜底：大量行 last_message_at 为 NULL，只按它排会翻页重复/漏项。
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("last_message_at DESC NULLS LAST, id DESC").
		Offset(offset).Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
```

**更新（本功能最大的一个坑）**：

```go
func (r *personaRepo) UpdateOwned(ctx context.Context, tx *gorm.DB, userID, personaID uint64,
	name, personalityDesc, speakingStyle string) (int64, error) {

	res := tx.WithContext(ctx).Model(&model.Persona{}).
		Where("id = ? AND user_id = ?", personaID, userID). // 归属条件进 SQL，不在 Go 里比
		Updates(map[string]any{                             // map 而非 struct：struct 会静默跳过零值字段
			"name":             name,
			"personality_desc": personalityDesc,
			"speaking_style":   speakingStyle,
		})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil // 0 = WHERE 没匹配到（不是"值没变"），由 service 判 4043
}
```

**`DeleteOwned`** 同理，返回 `RowsAffected`；级联交给外键，**不写任何多表删除代码**。

**`FindOwned`**（`PUT` 的回读）：

```go
// FindOwned 读当前用户的某一行；未命中返回 gorm.ErrRecordNotFound。
// 允许的形态②（spec §5.2）：双条件、读的是自己的行，对"别人的 id"与"不存在的 id"都返回同一个 not found。
func (r *personaRepo) FindOwned(ctx context.Context, userID, personaID uint64) (*model.Persona, error) {
	var p model.Persona
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", personaID, userID).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}
```

**`ExistsOwnedByUser`**（形态③，跨模块共享闸门）：

```go
// ExistsOwnedByUser 只回答"这个 persona 是不是这个用户的"。
// 对"别人的 persona"与"不存在的 persona"都返回 (false, nil)，两者不可区分 —— 不泄漏存在性。
// 禁止改成单条件的 ExistsByID：那能回答"这个 id 到底存不存在"，可被顺序试号枚举（spec §5.2 形态④）。
func (r *personaRepo) ExistsOwnedByUser(ctx context.Context, userID, personaID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Persona{}).
		Where("id = ? AND user_id = ?", personaID, userID).
		Count(&count).Error
	return count > 0, err
}
```

### 3.4 `persona_service.go`（步骤 4）

**创建（事务编排，本功能唯一的跨表写入）**：

```go
func (s *personaService) Create(ctx context.Context, userID uint64, req *dto.CreatePersonaRequest) (*dto.PersonaResponse, error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized) // 鉴权失败不许静默降级
	}

	p := &model.Persona{
		UserID:          userID, // 只来自 Token
		Name:            req.Name,
		PersonalityDesc: req.PersonalityDesc,
		SpeakingStyle:   req.SpeakingStyle,
		State:           model.JSONB(`{"familiarity":0}`), // 显式初始化，不依赖 DDL 默认值
		// User 关联字段保持零值：赋了值 GORM 会连带去写 users 表
	}

	// 人设与配置行同成同败：先提交人设再插配置而后者失败，就留下孤儿配置（spec §4.3.1）
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.personaRepo.Create(ctx, tx, p); err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		// p.ID 由 GORM 从 BIGSERIAL 回填，播种要用它 —— 顺序不能反
		if err := s.proactiveRepo.CreateDefaultSettings(ctx, tx, userID, p.ID); err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		return nil
	})
	if err != nil {
		return nil, err // ⚠️ 原样透传，不要再 Wrap（陷阱 A，见下）
	}

	resp := dto.NewPersonaResponse(p)
	return &resp, nil
}
```

**更新（写 + 回读，同一事务）**：

```go
func (s *personaService) Update(ctx context.Context, userID, personaID uint64, req *dto.UpdatePersonaRequest) (*dto.PersonaResponse, error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}

	var p *model.Persona
	err := s.db.Transaction(func(tx *gorm.DB) error {
		n, err := s.personaRepo.UpdateOwned(ctx, tx, userID, personaID,
			req.Name, req.PersonalityDesc, req.SpeakingStyle)
		if err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		if n == 0 {
			// 未命中：归属条件在 SQL 里，这里天然不区分"别人的"与"不存在"（spec §5.2）
			return errcode.New(errcode.ErrPersonaNotFound)
		}
		// 回读真值：state / familiarity / lastMessageAt 必须来自数据库，不能手工拼装
		p, err = s.personaRepo.FindOwned(ctx, userID, personaID)
		if err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		return nil
	})
	if err != nil {
		return nil, err // ⚠️ 原样透传（陷阱 A）
	}

	resp := dto.NewPersonaResponse(p)
	return &resp, nil
}
```

**删除**：

```go
	n, err := s.personaRepo.DeleteOwned(ctx, tx, userID, personaID)
	if err != nil { return errcode.Wrap(errcode.ErrDBFailed, err) }
	if n == 0    { return errcode.New(errcode.ErrPersonaNotFound) }
	return nil   // 级联（chat_messages / user_memory / user_profile / proactive_settings / …）交给外键
```

**列表**：

```go
	page, pageSize := dto.ClampPage(page, pageSize)
	list, total, err := s.personaRepo.ListByUser(ctx, userID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	items := make([]dto.PersonaResponse, 0, len(list)) // 必须初始化：nil slice 会序列化成 null
	for i := range list {
		items = append(items, dto.NewPersonaResponse(&list[i]))
	}
	return &dto.PageResult[dto.PersonaResponse]{
		List: items, Total: total, Page: page, PageSize: pageSize,
	}, nil
```

> ⚠️ **`for i := range list` 而不是 `for _, p := range list`**：后者每轮 `p` 是**副本**，把 `&p` 传给 `NewPersonaResponse` 得到的是同一个栈地址的副本——目前 `NewPersonaResponse` 只用即时值、不会出问题，但一旦它将来开始保存指针或做惰性求值，就会静默取到最后一行。**用索引取地址**是零成本的写法，不留这个雷。

#### 陷阱 A：事务之后的 `errcode.Wrap` 会把 `4043` 洗成 `5003`

上面两个函数里的 `if err != nil { return nil, err }` 是**刻意的**，不是偷懒。

`errcode.Wrap(ErrDBFailed, err)` **不检查 `err` 是否已经是 `BizError`**，它无条件造一个新的 `5003`；而 `BizErrorHandler` 用 `errors.As` 取**最外层**那个。所以一旦写成：

```go
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)   // ❌ 越权/不存在全变成 5003
	}
```

**任何越权与不存在都返回 `5003`**——红线 3 的规则表面上有代码，实际上失效。这类 bug 不会让编译、`vet`、任何既有 grep 响，只有"越权必须 4043"这条断言会响。**判据**：`db.Transaction` 调用**之后**的代码块里不得出现 `errcode.Wrap`（A7 #14）。

#### 陷阱 B：绑定失败要把原始 error 换成 BizError

handler 层的事（§3.5），但根因在 `BizErrorHandler` 只识别 `*errcode.BizError`，所以在这里先记一笔。

### 3.5 `persona_handler.go`（步骤 5）

```go
// currentUserID 是 4 个端点唯一的 userID 来源读取点。
// gin v1.12 的 GetUint64 只返回一个值（内部是 val.(uint64) 直接断言，失败静默返回 0），
// 所以判据只能是 userID == 0 —— BIGSERIAL 从 1 起，0 不可能是合法用户。
func currentUserID(c *gin.Context) (uint64, bool) {
	userID := c.GetUint64(middleware.ContextKeyUserID)
	return userID, userID != 0
}
```

```go
func (h *PersonaHandler) Update(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		// 鉴权失败静默降级成 userID=0 会让查询返回空集，把鉴权失败伪装成"没有数据"
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	personaID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || personaID == 0 {
		// 非数字 / 0 一律按"人设不存在"处理（spec §4.7）：
		// DELETE 的契约错误码集合里没有 4001，这里返回 4001 就是契约漂移
		_ = c.Error(errcode.New(errcode.ErrPersonaNotFound))
		return
	}

	var req dto.UpdatePersonaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// ⚠️ 不能 _ = c.Error(err)：原始 error 不是 BizError，
		//    BizErrorHandler 会走 "unknown error" 分支返回 5000 —— 4001 变成 5000。
		//    原始 err 也不返回前端（红线 6）。
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.personaService.Update(c.Request.Context(), userID, personaID, &req)
	if err != nil {
		_ = c.Error(err) // 上抛的一定是 BizError：service 层保证
		return
	}
	response.Success(c, *resp)
}
```

```go
func (h *PersonaHandler) Delete(c *gin.Context) {
	// ... userID / personaID 同上 ...

	if err := h.personaService.Delete(c.Request.Context(), userID, personaID); err != nil {
		_ = c.Error(err)
		return
	}
	// data 必须是 null：Success(c, nil) 编译不过（泛型 T 无法从 untyped nil 推断），要显式指定类型参数
	response.Success[any](c, nil)
}
```

**路由注册（照 MASTER §4.5 冻结的样式，不要自创）**：

```go
// RegisterPersonaRoutes 由 router.go 汇总调用；不要直接在本文件里往 engine 上挂。
func RegisterPersonaRoutes(rg *gin.RouterGroup, h *PersonaHandler) {
	g := rg.Group("/personas")
	{
		g.GET("", h.List)
		g.POST("", h.Create)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
	}
}
```

**四条纪律**：handler 只做绑定 → 取 userID → 调 service → `response.Success`；**不写 `response.Fail`**；**不引 `repository` / `gorm`**（红线 7）；**不出现裸错误码数字**（红线 6）。

---

## 4. 我手动审查 AI 代码的计划

> 前提：AI 生成的代码**默认不可信**，尤其是"越权防线"、"事务是否真回滚"、"`4043` 有没有被洗成别的码"这三处——它们写错了照样能编译、照样能演示，只在特定输入下才暴露。审查的**目标不是"能跑"，是"我能逐行解释它为什么安全"**。

### 4.1 审查顺序

| 遍 | 做什么 | 判据 |
|---|---|---|
| **第一遍 · 先审 tag 与签名，再审逻辑** | 逐行读 `proactive_setting.go` 的 tag 与 `persona_repo.go` 的方法签名 | **签名与 spec §5.2 冻结清单逐字一致**——tag 是字符串字面量，编译器不看内容，"写漏"类缺陷只在这里能抓到 |
| **第二遍 · 对照核对** | 拿契约 §4 + spec 全文，逐字段/逐端点核对，填成表格 | 每个 JSON 字段名都能在契约里指到来源；找不到来源的字段一律删掉 |
| **第三遍 · 对抗验证** | 主动构造能打破代码的输入（跨账号、空列表、不存在的 id、非数字 id、重复删除、超长字段、**让播种失败**），按 §4.4 实跑 | 每个高危点都有一次实跑记录，不是"读代码觉得没问题" |

**第三遍是核心**：前两遍只能证明"代码符合我的预期"，第三遍才能证明"代码在我不期望的输入下不崩、不泄漏、不回滚错"。

### 4.2 逐文件审查清单

| 文件 | 我盯的点 | 看懂的标准 |
|---|---|---|
| `dto/common_dto.go` | `PageResult[T]` 是否**只有这一份**；字段是否是 `list/total/page/pageSize`（不是 `items`/`records`）；`ClampPage` 是否**只收敛不报错** | `grep -rc "^type PageResult" internal/` 命中**文件数 = 1** |
| `model/proactive_setting.go` | **两个**关联字段都在且有 `OnDelete:CASCADE`；组合唯一索引的名字两个字段**逐字相同**；四个默认值写对；`last_nudge_at` 是指针；`ID` 是 `bigint` 不是 `bigserial` | `\d proactive_settings` 有 2 条 CASCADE 外键 + Indexes 段有 `uq_proactive_settings_user_persona`；我能说出每个 tag 去掉之后会发生什么 |
| `model/migrate.go` | `&ProactiveSetting{}` 加在 `&UserProfile{}` **之后**（引用 `users(id)` / `personas(id)`，两张表都必须先建好）；语义上只多了这一行 | ⚠️ **不能数行数**：见下方说明，`git diff --stat` 是 6 增 6 删。正确读法是 `git diff --ignore-all-space internal/model/migrate.go` —— **必须恰好只有 1 减 1 增**，且增的是 `&ProactiveSetting{}`、减的是那行注释 |
| `dto/persona_dto.go` | 有没有偷偷声明 `userId`；camelCase 是否逐字对齐契约；`State` 是不是 `model.JSONB`（不是 `map[string]any`）；`familiarity` 是否容错取 0 | 与契约 §4 的 JSON 示例并排比对，字段名一个字母都不差 |
| `repository/persona_repo.go` | **每个方法签名是否都带 `userID`**；有没有 `First(&p, id)` 形态的**单条件**裸查询；有没有 `ExistsByID`；更新是否用了 `Save(`；`Updates` 是否传 `map` | `grep -nE "Save\(|First\(&[a-z]+, *id\b|func.*Exists(ByID\|ByPersonaID)\("` 无业务命中 |
| `repository/proactive_repo.go` | 播种函数是否**接受 `tx`** 而不是自取包级 `db`；四个值是否显式写出；有没有给关联字段赋值 | 参数表里有 `tx`；函数体内是 `tx.WithContext(ctx)` |
| `service/persona_service.go` | 未命中是否统一 `4043`（**不出现 `4030`/`4040`**）；事务**之后**有没有 `Wrap`；是否引了 `gin`；是否有裸数字错误码；`make([]dto.PersonaResponse, 0)` 在不在 | `grep -n "gin-gonic"` 无结果；`grep -nE "\b(400[0-9]\|403[0-9]\|404[0-9]\|500[0-9])\b"` 无结果；事务后无 `Wrap` |
| `handler/persona_handler.go` | `GetUint64` 的用法**是不是单返回值**；`_ = c.Error(...)` 的参数**是不是 `errcode.New(...)`**；有没有 `response.Fail`；`Delete` 是不是 `Success[any](c, nil)` | `grep -n "response.Fail"` 无结果；`grep -c "GetUint64"` = 1 |
| `router.go`（成员 1 改的） | 只加了一行，没动别人的行 | `git diff router.go` 只有 1 行新增 |

### 4.3 高危点排序（**只审三条的话，审这三条**）

| # | 高危点 | 为什么会错 | 我怎么验 |
|---|---|---|---|
| **1** | **事务之后的 `Wrap` 把 `4043` 洗成 `5003`** | `Wrap` 不检查 err 是否已是 `BizError`，无条件造新的 `5003`；`errors.As` 取最外层 → 越权规则表面有代码、实际失效。**编译/vet/既有 grep 全不响** | `grep -n -A20 "db.Transaction" persona_service.go`，**事务之后**不得出现 `errcode.Wrap`；再用 B 的 Token 打 `PUT`，必须拿到 `4043` 而不是 `5003` |
| **2** | **归属条件是否真的在 SQL 里** | "先查出来再在 Go 里比"的写法看起来逻辑完整，**漏掉 `if` 在 review 中极难发现**；写成单条件的存在性探针又能被逐号枚举 | `grep` 每个方法的 `Where` 子句，逐个确认**两个条件**；用 A4 的 SQL 实验证明 `RowsAffected=0` 确实发生 |
| **3** | **创建是否真的单事务** | 事务内用包级 `db`、或先提交人设再插配置——**代码看上去有 `Transaction`，实际没生效**；后果是"某个伴侣设置页打不开"，演示时才发现 | A5 的失败注入实验：人为让播种违例外键，确认 `personas` **不留新行** |

**次要但必查的三条**：`NULLS LAST`（写错顺序反了、页面看着正常）｜`state` 保留（`Save()` 或全字段 `Updates` 会静默清空，不可逆）｜`c.Set` 传 `uint64`（传 `int` 会让**每个请求都 4010**，且日志里看不出异常）。

### 4.4 反向验证记录

> **规矩**（`docs/dev_notes/user_memory_notes.md` §0）：每条"必须类"检查都要能用真实缺陷打穿——**注进去要响、不注不许响**。不然它可能是一条永远为真的假标准。

**已跑（2026-09-16，写文档时实测，代码尚未落地）**——证明这批检查今天**会响**、不是空转：

| 检查 | 读数 | 结论 |
|---|---|---|
| `grep -rn "^type PageResult" internal/` | **0** | 会响 |
| `grep -rn "^func ClampPage" internal/` | **0** | 会响 |
| `grep -rn "ExistsOwnedByUser" internal/` | **0** | 会响 |
| `grep -rn "func RegisterPersonaRoutes" internal/` | **0** | 会响 |
| `grep -rn "GetUint64" internal/` | **0** | 会响 |
| `grep -rn "response.Fail" internal/`（**裸词**） | **4** | ⚠️ **含 1 条注释**（`biz_error.go:15`）→ **裸词不可用** |
| `grep -rnE "response\.Fail\(" internal/`（**锚定**） | **3** | 全在 `middleware/`（合法位置）→ 锚定形式可用 |
| `grep -c "idx_personas_user_last_msg" internal/model/persona.go` | **3** | 索引字段各 1 + **注释 1** → 这条也必须锚定到 `gorm:"` |

**待代码落地后逐条实跑**（§4.3 的三条高危点 + A7 的 18 条）：

| # | 注入的缺陷 | 期望哪条检查响 | 期望读数变化 |
|---|---|---|---|
| 1 | 在 `db.Transaction` 之后加 `return nil, errcode.Wrap(errcode.ErrDBFailed, err)` | A7 #14 | 违规处数 0 → **1**；且 B 组"越权 = 4043"以 **5003** 失败 |
| 2 | 把 `UpdateOwned` 的 `Where` 改成只有 `id = ?` | A4 + A7 #6 | `RowsAffected` 从 0 变 **1**（越权写入"成功"）；A7 #6 报出缺 `userID` 的方法 |
| 3 | 在 `proactive_repo` 里把 `tx` 换成包级 `db` | A5 回滚实验 | `personas` **残留**新行（事务白做） |
| 4 | 删掉 `NULLS LAST` | A6 | 顺序从「较晚/较早/NULL」变成「NULL/较晚/较早」 |
| 5 | 把 `Updates` 的 `map` 换成 `model.Persona{...}` struct | A7 #10 | `grep Save(` 不一定响（不是 `Save`）→ **说明还需要一条"必须是 map"的定向检查** |
| 6 | 给 `PersonaResponse` 加一个 `UserID uint64` 字段 | 契约对照 | 与契约 §4 并排比对时多一列 |
| 7 | 删掉 `proactive_setting.go` 的 `Persona` 关联字段 | A2 + A3 | 外键从 **2 条变 1 条**（必须类检查必须能发现"少了一个"） |
| 8 | 把 `list` 的 `make([]PersonaResponse, 0)` 换成 `var list []PersonaResponse` | A7 #17 | 命中数 1 → **0**；空列表响应变成 `"list": null` |

> **第 5 条的教训**：注入 `Updates(struct)` 时，禁止类检查（`grep Save(`）**不会响**——说明"必须用 map"这条约束目前只有人工审查兜着。**补一条必须类检查**：`grep -cE 'Updates\(map\[string\]any\{' internal/repository/persona_repo.go` 期望 **1**。这正是 §0 那条规矩的用法：问一句"如果我把这行改坏，哪条检查会响？"答不上来的地方就是缺了一条检查。
>
> ⚠️ **这条补上的检查，第一版也是错的**：原本写成 `\.Updates\(map\[string\]any\{`（前面带个点），实测在**正确代码**上读出 **0**。原因是链式调用里方法名前面是**换行**、点号留在**上一行行尾**：
>
> ```go
> 	res := tx.WithContext(ctx).Model(&model.Persona{}).
> 		Where("id = ? AND user_id = ?", personaID, userID).
> 		Updates(map[string]any{          // ← 前面没有点
> ```
>
> 去掉那个 `\.` 之后读数才是 1。**这是本节第 6 个"照着自己以为的代码拟判据"的例子**（全表见 spec §8.5）。

**实跑后发现的第 9 条（2026-09-16，代码落地当天）——一条自己写出来、当场被证伪的假标准：**

本表原先写的 `migrate.go` 判据是「`git diff` 只有 1 行新增」。**实测是 6 增 6 删**，而代码是完全正确的：

```
$ git diff --stat backend/internal/model/migrate.go
 1 file changed, 6 insertions(+), 6 deletions(-)
$ git diff --ignore-all-space backend/internal/model/migrate.go | grep '^[+-]' | grep -v '^[+-][+-]'
-		// &ProactiveSetting{},  // 待 ...
+		&ProactiveSetting{}, // 引用 users(id) 与 personas(id) ON DELETE CASCADE，两张表都要先建好
```

**原因**：`AutoMigrate` 列表里每行都带行尾注释，gofmt 按"块内最宽元素"对齐注释。新增的 `&ProactiveSetting{},` 比 `&ChatMessage{},` 宽，于是**它上面的 5 行注释全部被重新对齐**——纯空白变动，语义零变化。

**为什么必须记下来**：这条检查**在正确实现上判出假失败**，而且没有任何人会怀疑它——"diff 比预期大"看上去就该要解释。这会诱使人要么去改代码迁就检查（把注释删掉以维持对齐 → 破坏注释），要么在 review 里花时间证明自己没多改东西。**两个后果都比检查本身糟。**

**改用**：`git diff --ignore-all-space`（`-w`）—— 它对纯空白重排免疫，实测刚好 **1 减 1 增**。**通用教训：凡是拿 `git diff` 行数当判据的检查，都要先加 `-w` 跑一遍**，否则 gofmt 的自动对齐随时会把它变成假失败。这条与 §8.4「期望为 0 的检查必须先拿正确文件跑一遍」是同一类错误的两个面。

**第 10 条 · 代码落地当天实跑的 6 次注入（完整读数在 spec §8.6）**

上表是我**代码还没写**时拟的 8 条预测。真到落地那天，实跑的是另外 6 条（`NULLS LAST`、`UpdateOwned` 去 `user_id`、`DeleteOwned` 去 `user_id`、播种移出事务、复合注入去事务+错 `personaID`、事务后 `Wrap`），**6 条全响**。两条预测被实测推翻，都记在 spec §8.6。这里只留**三条能改我以后怎么做事**的：

1. ⛔ **注射点选错，会得出"这条检查无效"的错误结论。** 第 6 次注入我第一次把 `Wrap` 注在 **`Create`** 的收尾：静态检查 #14 **响了**（0 → 1），但 A4 的六行行为读数**纹丝不动**。原因：`Create` 事务内只可能产生 `5003`，再 `Wrap` 一次还是 `5003`，**行为上不可观测**——陷阱 A 只在**事务内会产生非 `5003` 码**的方法里才有后果。改注到 `Update` 收尾后，越权改 / 不存在的 id / id=0 **三例当场从 `4043` 变成 `5003`**。
   **通用教训：注入的"可观测性"和"正确性"是两件事。** 注进去没响，先问"这个缺陷在这个位置有可观测后果吗"，再问"检查是不是假的"。
2. **预测必须写成"期望读数"再实跑，然后**照实记录不符**，不要事后把预测改成实际。** 第 3 次注入我预测"播种移出事务 → `personas` 残留新行（事务白做）"，实测是**硬外键违例 `5003`、一行都没进库**——因为未提交的人设在另一个连接里不可见。**这个实测比我的预测更有价值**：它说明外键约束是唯一一层"代码写错时还会替你拦一道"的防御。**改预测 = 把一次真实验证降级成一次自证。**
3. **A7 的 18 条 grep 全绿，与"schema 是对的"没有蕴含关系。** 落地当天 `go build` / `go vet` / `gofmt` / 18 条 grep **全部通过**，而三个 `INT` 列被建成了 `bigint`（spec §3.2 结论 5）。**只有 `\d` 看得见。**
   **通用教训：验收表里必须有一组"换一种观测方式"的检查。** grep 看文本、`\d` 看 schema、真跑看行为——**同一批断言用三种方式各测一遍**，只在其中一种里全绿，说明不了另外两种。

### 4.5 拒绝标准（出现任一条就退回重写，不做"小修小补"）

| 打回条件 | 为什么不能只小修 |
|---|---|
| **给 userID 兜底了假值**（`if userID == 0 { userID = 1 }` 之类） | 越权防线当场全废，且表面上一切正常（每个人看到的是第一个用户的人设）。这是最危险的临时改动 |
| 仓储层存在**不带 `user_id`** 的业务查询，或单条件的 `ExistsByID` | 防线是结构性的，逐处打补丁必漏；必须让不安全的方法不存在 |
| 未命中返回 `4030` / `4040` | 违反契约 §2/§4（资源越权 → `4043`）；且与"不存在"并存即可被二分探测 |
| **事务之后出现 `errcode.Wrap`** | `4043` 会被洗成 `5003`，规则失效但代码看起来完整 |
| `_ = c.Error(err)` 上抛的是**原始 error** 而非 `errcode.New(...)` | `4001` 会被洗成 `5000` |
| **创建人设不是单事务**（用了包级 `db`、或分两次提交） | 会留下孤儿配置行，正是 persona-model §4.3.1 要消灭的问题 |
| 更新用了 `Save()` 或全字段 `Updates(struct)` | 覆盖 `state` 是**不可逆**的数据损坏 |
| 未命中时返回成功（`200`） | 越权写入被伪装成成功，比报错危险 |
| 出现裸数字错误码 / 中文错误文案字面量 | 破坏"一 code 一 msg"（红线 6） |
| service 里出现 `*gin.Context`，或 handler 里引了 `gorm`/`repository` | 破坏分层（红线 7） |
| `proactive_settings` 的外键未带 `ON DELETE CASCADE`（**两个都要查**） | 孤儿数据会污染主动消息模块，事后清理成本高 |
| `PageResult` 出现第二份定义 | 两份同名类型在联调时会出现"字段对不上"的诡异问题，最难排查 |
| 往 `.go` 里塞了长篇学习性注释 | 违反 AGENTS §5.1 与任务书要求 6；学习内容放 `.learn/` |

**审查通过的唯一标准**：§4.4 的待跑清单**全部实跑过并留有读数**，且 §4.2 每个文件我都能逐行解释。**审查笔记落到 `docs/dev_notes/persona_crud_notes.md`**（该文件已在 `.git/info/exclude` 里，**不进 PR**——`user_memory_notes.md` 就是这么处理的）。

---

## 5. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| **`JWTAuth` 是空壳** | 4 个端点现在全部 `4010`，curl 级联调全线阻塞；容易诱发"给 userID 兜底"的临时改动 | 步骤 7 的非 HTTP 验证路径先行；**把"禁止兜底"写进拒绝标准**（§4.5）；群里催成员 1 的 `JWTAuth` |
| **`c.Set` 传了 `int` 而不是 `uint64`** | gin v1.12 的 `GetUint64` 是**直接类型断言**，失败静默返回 0 → **每个请求都 4010**，日志里看不出异常 | 主动广播（spec §7.1）；落地后先用一个"能拿到 userID"的测试确认 |
| `middleware/logger.go:49` 的 `userId` 日志永远是空串 | 排查越权问题时看不到是谁在打，排障效率下降 | **只报不改**（成员 1 的文件）；本分支不顺手改 |
| **2 个前置文件不在任务书清单里** | Review 时会被问"为什么多改了两个文件" | ✅ 已裁定纳入（spec §2.1 / §7.3 决策 7）。**PR 描述里写明理由**（§6 草稿），不靠 review 时口头解释 |
| `ExistsOwnedByUser` 的签名被改 | 成员 1 的 memory 模块直接引它，改签名 = 改接口，三个分支一起返工 | 签名已冻结（spec §5.2）；若要改，群里广播 |
| `common_dto.go` 被别的分支同时创建 | 合并冲突，或两份同名 `PageResult` 在联调时字段对不上 | 落地前先 `grep -rn "^type PageResult" internal/`；**本分支先合，后来者消费** |
| 播种的默认值与主动消息模块不一致 | 两处各写一套默认值，用户改过配置后被覆盖 | 默认值只在 `CreateDefaultSettings` 一处定义；主动消息模块的 `GET` 直接读表，不写第二套兜底 |
| `PUT` 的同值更新被误判成 `4043` | 用户重复点保存会看到"人设不存在" | A4 的**同值更新实验**（`RowsAffected` 必须 = 1）；这条是 `4043` 判定的基石，**必须实跑** |
| 契约 §4 的 `state: {}` 与 `familiarity: 12` 示例不一致 | 成员 2 写 Mock 时可能理解成独立列 | 实现后主动告知（spec §4.6 末尾）；`familiarity` 是 `state` 里的键 |
| **`user_profile.go` 缺两条外键**（用户已确认为真实 Bug） | 删账号/删人设时画像行不级联，留孤儿数据；与 user-profile spec §8 分组 A 直接矛盾 | **不在本分支修**（本分支范围只有人设 CRUD）；**合并后**另开 `fix(model): restore user_profile FK associations`，验收 = `\d user_profile` 从 **0 条 → 2 条** `ON DELETE CASCADE`（spec §7.3 待办 #2） |
| 验证时误伤开发库数据 | A5 的实验包含 `DELETE FROM users` / 删人设，跑在 `heart_echo_db` 上会**销毁真实数据** | **用一次性容器验证**（见下），不碰 dev 库 |

**验证用库的选择（照 user_profile 那次的做法）**：

```bash
# 一次性容器，trust 认证 → DSN 里根本没有密码（红线 1），端口错开避开 dev
docker run -d --name he_persona_check -p 55434:5432 \
  -e POSTGRES_HOST_AUTH_METHOD=trust -e POSTGRES_DB=schema_check postgres:16
# DSN 只经环境变量传入，绝不写进代码 / 文件 / _test.go
export SCHEMA_CHECK_DSN='postgres://postgres@127.0.0.1:55434/schema_check?sslmode=disable'
go run ./cmd/migrate
# ... 跑 A 组实验 ...
docker rm -f he_persona_check     # 收尾
```

理由：A5 的级联实验要 `DELETE FROM users`，A3/A6 要造多行人设——**这些都会破坏 dev 库里已有的数据**。dev 的 `heart_echo_db` 是成员 1 / 成员 2 联调在用的，不做实验场。

---

## 6. PR 描述草稿（**范围扩大必须在这里说明**）

> 用户裁定：扩范围的理由要写在 PR 描述里，不能只靠 review 时口头解释。下面是草稿，提 PR 时直接用。

```markdown
## 范围

本 PR 交付 **6 个文件**：任务书指定的 4 个（persona_repo / persona_service /
persona_handler / persona_dto）+ **2 个前置文件**。

### 为什么多带 2 个文件（超出任务书清单）

**`internal/dto/common_dto.go`** —— `GET /personas` 的响应类型是契约 §4 冻结的
`PageResult<Persona>`。该类型是五个分页端点（persona / chat-message / user-memory /
moments / schedules）**共用的一份**，persona-model §4.1 已定位置，user-memory spec
记着"尚未落地，谁先合谁建，后来者消费，不要重建"。本 PR 是最先落地的那个，不建
就只能让 `GET /personas` 返回裸数组 —— 与契约不符。

**`internal/model/proactive_setting.go` + `migrate.go` 一行** —— `POST /personas`
必须**同事务**播种一行 `proactive_settings`（persona-model §4.3.1），否则主动消息
模块上线时会撞上"某个伴侣的设置页打不开"的孤儿配置。`migrate.go:18` 那行现在还是
注释。

两个文件都属成员 3 的职责范围（10 张表设计 + 分页结构），**不是越界**。

### 明确不在本 PR 内

- `router.go`（成员 1 加一行 `RegisterPersonaRoutes`）
- `middleware/**`、`pkg/**`
- `personas` 表与 `persona.go`（零 schema 改动）
- **`user_profile.go` 缺失的 2 个关联字段** —— 已确认为真实 Bug，但本 PR 范围只在
  人设 CRUD，另开 `fix(model): restore user_profile FK associations` 修复

## 验证

见 docs/specs/persona-crud/spec.md §8。本 PR 可独立验证的是分组 A（18 条锚定检查 +
6 组行为实验，用一次性容器库，不碰 dev 库）；分组 B（端到端 curl）**阻塞在
`JWTAuth`**——它目前是空壳，4 个端点全部返回 4010。
```

---

## 7. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-16 | v1 | 创建。步骤 0-9；关键实现要点逐层给出（含陷阱 A/B 两处"错误码被中途改写"）；§4.4 记录 8 条已跑测量 + 8 条待跑的反向验证（其中第 5 条当场发现缺一条必须类检查）；确认验证用一次性容器而非 dev 库 | 人设 CRUD 接口层开工前的施工与自审计划 |
| 2026-09-16 | v2 | 用户裁定：**6 个文件全部纳入本 PR**（§1 加"在本 PR 内"列 + 理由）；`user_profile.go` 外键修复**移出本分支**，改为合并后的独立 fix 分支（§5 风险表同步）；新增 **§6 PR 描述草稿**，把扩范围理由写进 PR 而不是口头解释 | 用户确认两点裁决 |
| 2026-09-16 | v3 | **代码落地。** §2 步骤 0–5 标完成、7 标完成（读数在 spec §8.6）、8 仍阻塞、9 等人工审查；§3.1 补 `type:integer` 与零值不可写的反向坑；**§4.4 新增第 10 条**：6 次注入实跑结果 + 三条通用教训（注射点选错会误判检查无效 / 预测必须照实记录 / 18 条 grep 全绿不等于 schema 对）。**代码与文档均留在工作区未提交，等用户人工审查** | 落地当天实测推翻了本文两处预测（详见 spec §8.6） |

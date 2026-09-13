# plan · 人设 CRUD（Persona CRUD）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-persona-model`
> 负责人：成员 3 ｜ 创建：2026-09-13 ｜ 最后更新：2026-09-13（第 3 版：套用队长「通用错误码规则」）

> ⚠️ 本功能遵循队长 2026-09-13 的**通用错误码规则**：**资源越权 → `4040`**（隐藏资源存在性）、**功能越权 → `4031`**。人设端点属资源越权，故 `:id` 未命中一律 `4040`（spec §5.2）。
> `API_CONTRACT.md` / `AGENTS.md` 是**团队全局文件，本分支不直接改**——由用户去群里广播、队长同意后统一更新。**全局同步完成前不合并 PR**。见步骤 10。

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 前置 | 状态 |
|---|---|---|---|---|---|
| 0 | 通用分页结构 | `internal/dto/common_dto.go` | `PageResult[T]` 只有这一份定义；字段 `list/total/page/pageSize` | 无 | 未开始 |
| 1 | **翻译 Struct** | `internal/model/persona.go` | 8 列 + 外键关联全被声明；字段对照 spec §3.2 逐行对齐；每字段带注释 | 无 | 未开始 |
| 2 | 主动消息配置模型 | `internal/model/proactive_setting.go` | 照 DDL 翻译；对 `Persona` 的外键带 `ON DELETE CASCADE` | 步骤 1 | 未开始 |
| 3 | 建表 | `internal/model/migrate.go` | `AutoMigrate` 追加 `&Persona{}`、`&ProactiveSetting{}`，顺序在 `&User{}` 之后；`\d personas` 有 CASCADE 外键 | 步骤 1、2 | 未开始 |
| 4 | 请求/响应结构 | `internal/dto/persona_dto.go` | 4 个结构体；camelCase 对齐契约 §4；`familiarity` 拍平逻辑在此 | 步骤 1 | 未开始 |
| 5 | **仓储层** | `internal/repository/persona_repo.go`、`internal/repository/proactive_repo.go` | 方法签名全部强制带 `userID uint64` 与事务句柄；**不存在**"按 id 单查"或"判断存在性"的方法 | 步骤 1、2 | 未开始 |
| 6 | **业务层** | `internal/service/persona_service.go` | 未命中一律 `4040`；**创建走事务**（人设 + 配置同成同败）；不依赖 `*gin.Context` | 步骤 4、5 + 成员 1 的 `pkg/errcode` | 未开始 |
| 7 | **HTTP 层** | `internal/handler/persona_handler.go` | 薄；含 `RegisterPersonaRoutes`；错误 `_ = c.Error(err)` 上抛 | 步骤 6 + `pkg/response` + `middleware` | 未开始 |
| 8 | 路由挂载 | `router.go`（成员 1 的文件） | 群里同步后加一行，**不要与成员 1 同时改** | 步骤 7 | 未开始 |
| 9 | 越权 / 级联 / 事务专项验证 | — | spec §8 的 19 项验收逐条打勾（含「不产生 `4031`」） | 步骤 8 | 未开始 |
| 10 | **规则广播 + 全局文件同步（阻塞合并，不由我做）** | 群里广播；`API_CONTRACT.md`、`AGENTS.md` **由队长 / 成员 1 改** | 规则已广播并被接受；全局文件按 spec §5.2 表同步（§4 错误码 `4040`、§12 登记 + 版本号、§2 的 `4031` 文案、AGENTS §4.3）；**本分支保持不动这两个文件** | 步骤 9 | 未开始 |
| 11 | **人工审查 + PR** | `docs/dev_notes/persona_model_notes.md`、PR | §4 的审查清单全过；至少 1 人 Approve | 步骤 9、10 | 未开始 |

**优先级与阻塞说明**：步骤 0-5 **不等任何人**，现在就能做完——这是本功能的关键路径。步骤 6-8 依赖成员 1 尚未交付的 `pkg/response` / `pkg/errcode` / `internal/middleware`（见 spec §7.1），**在它们落地前不要去自己造一份**（红线：不 Own 的东西不要动，造了必冲突）。等待期间用 §4.4 里那几条 SQL 直接对 PG 验证仓储层。

## 2. 文件清单

| 路径 | 作用 | 谁还会用到 |
|---|---|---|
| `backend/internal/dto/common_dto.go` | `PageResult[T]`，**全项目五处分页共用** | **成员 1**（messages / memory）、成员 3（moments / schedules） |
| `backend/internal/model/persona.go` | `personas` 的 GORM 实体 | **成员 1**（`message_repo` 引 `Persona`）、成员 3 |
| `backend/internal/model/proactive_setting.go` | `proactive_settings` 的 GORM 实体 | 成员 3（主动消息模块） |
| `backend/internal/model/migrate.go` | 建表入口 | 成员 1（`main.go` 调用） |
| `backend/internal/dto/persona_dto.go` | 请求 / 响应结构 | 成员 2（据此写 `types/persona.ts` 与 Mock） |
| `backend/internal/repository/persona_repo.go` | 人设数据访问 | 成员 3（主动消息定时任务要读 `last_message_at`） |
| `backend/internal/repository/proactive_repo.go` | 主动消息配置数据访问；**本功能只用其"播种默认行"一个函数** | 成员 3（主动消息模块在此文件继续加 `Get` / `Upsert`） |
| `backend/internal/service/persona_service.go` | 归属校验与业务逻辑（含事务编排） | 成员 3（主动消息模块的 service 参照它的写法） |
| `backend/internal/handler/persona_handler.go` | 4 个端点 + `RegisterPersonaRoutes` | 成员 1（挂载一行） |
| `docs/dev_notes/persona_model_notes.md` | **我的审查笔记**（不是给别人的文档） | 只有我；是新文件，与 `user_model_notes.md` 同级 |

## 3. 关键实现要点

### 3.1 翻译 Struct（步骤 1、2）

**手法沿用 `docs/dev_notes/user_model_notes.md`**：每个字段一行注释，把对应的 DDL 原文贴在 GORM tag 上方。理由很实际——`user.go` 那份带注释的版本才是能看懂、能改的版本，光有代码的那份不是。**先写带全注释的版本进 dev_notes，再落 `persona.go`。**

字段与 tag 的对应（完整对照表见 spec §3.2），**三处最容易翻车**：

1. **`user_id` 用 `json:"-"`**——契约 §4 的 Persona 实体里没有 `userId`。它同时被 `User` 的外键关联引用，是越权防线的载体，不是响应字段。
2. **`state` 用 `datatypes.JSON`**，不要 `string`、不要固定 struct。固定 struct 会在读写中**静默丢掉未知键**（§5.6 预留的 `self_note` 就这么没的）。
3. **必须声明 `User User` 关联字段**，否则 GORM **不建外键约束**——"删人设级联删消息"这条验收项直接不成立。这个字段只为生成约束存在，同样 `json:"-"`。

```go
// 仅供 GORM 生成 ON DELETE CASCADE 外键；不参与序列化
User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
```

> **`Create` 时留意这个关联字段**：GORM 会尝试自动保存关联对象。构造待插入的 `Persona` 时**不要给它赋 `User`**，保持零值。若发现 `Create` 有异常写 `users` 表的行为，就是这里——`user_model_notes.md` 那种逐字段注释的做法能帮你第一时间定位。
>
> `proactive_setting.go` 对 `Persona` 的关联同理，**外键也要 `ON DELETE CASCADE`**；注意**不要**给 `ProactiveSetting` 也加一个指向 `Persona` 的非零关联对象去创建，播种时只赋 `UserID` / `PersonaID` 两个标量。

### 3.2 Repository（步骤 5）

**核心是"让不安全的方法不存在"**，而不是"记得每次都校验"：

```go
// persona_repo.go —— 方法签名强制带 userID，调用方没有"忘了传"的选项
ListByUser(ctx context.Context, userID uint64, offset, limit int) ([]model.Persona, int64, error)
Create(ctx context.Context, tx *gorm.DB, p *model.Persona) error
UpdateOwned(ctx context.Context, tx *gorm.DB, userID, personaID uint64, req *dto.UpdatePersonaRequest) (int64, error)
DeleteOwned(ctx context.Context, tx *gorm.DB, userID, personaID uint64) (int64, error)

// proactive_repo.go
CreateDefaultSettings(ctx context.Context, tx *gorm.DB, userID, personaID uint64) error
```

> **刻意没有 `ExistsByID` / `FindByID`**：spec §5.2 定下"不泄漏存在性"后，代码里**不需要**"存在性"这个概念。少一个方法就少一处泄漏面——这也是这版决定的附带收益。

**列表查询**（`NULLS LAST` 与 `id` 兜底排序的理由见 spec §4.2）：

```go
err := db.WithContext(ctx).Model(&model.Persona{}).
    Where("user_id = ?", userID).
    Count(&total).Error
// ...
err = db.WithContext(ctx).
    Where("user_id = ?", userID).
    Order("last_message_at DESC NULLS LAST, id DESC").
    Offset(offset).Limit(limit).Find(&list).Error
```

**更新——本功能最大的一个坑**：**绝对不要用 `Save()`**。`Save()` 写回全部列，会把 `state` 覆盖成零值、把 `last_message_at` 清空（spec §5.3 反例表）。只更新这三列：

```go
res := db.WithContext(ctx).Model(&model.Persona{}).
    Where("id = ? AND user_id = ?", personaID, userID).   // 归属条件写进 SQL，不在 Go 里比
    Updates(map[string]any{
        "name":             req.Name,
        "personality_desc": req.PersonalityDesc,
        "speaking_style":   req.SpeakingStyle,
    })
if res.Error != nil { /* errcode.Wrap(ErrDBFailed, ...) */ }
// 返回 res.RowsAffected，由 service 判 4040；0 绝不能当成功
```

> `Updates` 传 `map` 而非 struct：传 struct 时 GORM **忽略零值字段**，`name` 传空串会被静默跳过（这里虽然 `required` 拦住了，但别依赖另一层的校验来保证这一层的行为）。`RowsAffected == 0` 要向上暴露成"未命中"，**不能当成成功**——否则改别人的数据会返回 200。

**删除**：`Where("id = ? AND user_id = ?").Delete(&model.Persona{})`，级联交给外键（含 `proactive_settings`），**不写任何多表删除代码**。

### 3.3 Service（步骤 6）

- 签名收 `context.Context`，**不出现 `*gin.Context`**（红线 7）。
- **未命中一律 `errcode.New(errcode.ErrNotFound)`（4040）**，不区分"不属于你"与"不存在"（spec §5.2：资源越权 → `4040`）。**不要**写存在性探针，**也永远不要返回 `4031`**（那是功能越权的码，本模块没有该场景）。
- 错误一律 `errcode.New` / `errcode.Wrap`，**不拼接自定义文案**（红线 6）。
- `userID == 0` 要当错误拦住：`0` 说明上游鉴权失败，**不能拿它去查库**，返回 `4010`（spec §5.1 ①）。

### 3.4 创建人设的事务编排（步骤 6，**本版新增**）

人设与主动消息配置**必须同成同败**——只提交人设而配置没插进去，就留下了 §4.3.1 要消灭的那种孤儿（该 AI 伴侣的主动消息设置页打不开）。事务边界在 **service 层**，repo 只接受事务句柄：

```go
func (s *personaService) Create(ctx context.Context, userID uint64, req *dto.CreatePersonaRequest) (*dto.PersonaResponse, error) {
    if userID == 0 {
        return nil, errcode.New(errcode.ErrUnauthorized)   // 鉴权失败不许静默降级
    }
    p := &model.Persona{
        UserID:          userID,          // 只来自 Token
        Name:            req.Name,
        PersonalityDesc: req.PersonalityDesc,
        SpeakingStyle:   req.SpeakingStyle,
        State:           datatypes.JSON([]byte(`{"familiarity":0}`)),  // 显式初始化
    }

    err := s.db.Transaction(func(tx *gorm.DB) error {
        if err := s.personaRepo.Create(ctx, tx, p); err != nil {
            return errcode.Wrap(errcode.ErrDBFailed, err)
        }
        // p.ID 由 GORM 回填，播种行用它
        if err := s.proactiveRepo.CreateDefaultSettings(ctx, tx, userID, p.ID); err != nil {
            return errcode.Wrap(errcode.ErrDBFailed, err)
        }
        return nil
    })
    if err != nil {
        return nil, err   // 回滚后 personas 不留行
    }
    return dto.NewPersonaResponse(p), nil   // familiarity = 0，lastMessageAt = nil
}
```

**三个易错点**：

1. **`p.ID` 的回填时机**：必须在 `Create` 成功**之后**才能拿到自增 id 去播种——顺序不能反。GORM 的 `Create` 会把 `BIGSERIAL` 的返回值写回结构体。
2. **不要用嵌套的 `db.Begin()` 手工管理**：用 `db.Transaction(...)`，它会在 `return err` 时自动回滚、`return nil` 时提交，且在 panic 时也会回滚（配合 Recovery 中间件）。手工 `Begin`/`Commit` 一旦漏掉 `Rollback`，连接会被占住。
3. **`Tx` 与 `ctx` 都要往下传**：repo 内部用 `tx.WithContext(ctx)`，不要用包级的 `db`——用了包级 `db` 就等于脱离了事务，回滚时这条插入**不会被撤销**，事务白做。

### 3.5 DTO 与 Handler（步骤 0、4、7）

- `common_dto.go` 只放 `PageResult[T]`，**不放别的东西**（AGENTS.md：不新建杂物间文件）。成员 1、成员 3 都从这里引，**不要再定义第二份**。
- DTO 里**不声明** `userId` / `state` / `familiarity` / `lastMessageAt` —— 字段不存在，就没有被传入和被覆盖的可能。多传的键 Gin 默认忽略，正合契约「`PUT` 时忽略」。
- `familiarity` 的拍平（容错取 0）写法见 spec §4.6。
- Handler 只做三件事：绑定 → 调 service → `response.Success`；出错 `_ = c.Error(err)` 后 `return`。**不写 `response.Fail`**。
- `RegisterPersonaRoutes` 照总纲 §4.5 的样式写，`router.go` 由成员 1 加一行（或错开时间自己加）。

---

## 4. 我手动审查 AI 代码的计划

> 前提：AI 生成的代码**默认不可信**，尤其是"越权防线"、"`state` 保留"、"事务是否真回滚"这三处——它们写错了照样能跑通、照样能演示，只在特定输入下才暴露。所以审查的**目标不是"能跑"，是"我能逐行解释它为什么安全"**。

### 4.1 审查方法（三遍，对应我审 `User` struct 时用的做法）

| 遍 | 做什么 | 判据 |
|---|---|---|
| **第一遍 · 逐行重写注释** | 把 AI 产出的每个文件抄进 `docs/dev_notes/persona_model_notes.md`，**用自己的话**给每一行加注释 | **写不出注释的那一行，就是我没看懂的地方** → 那里就是审查重点，去查 GORM 文档或问，不放过 |
| **第二遍 · 对照核对** | 拿 DDL（spec §3.1）+ 契约 §4 + spec 全文，逐字段/逐端点核对，填成表格 | 每个 JSON 字段名都能在契约里指到来源；找不到来源的字段一律删掉 |
| **第三遍 · 对抗验证** | 主动构造能打破代码的输入（跨账号、空列表、不存在的 id、无 `familiarity` 的 `state`、重复删除、超长字段、**让播种插入失败**），按 §4.4 的命令实跑 | 每个高危点都有一次实跑记录，不是"读代码觉得没问题" |

**第三遍是这份计划的核心**：前两遍只能证明"代码符合我的预期"，第三遍才能证明"代码在我不期望的输入下不崩/不泄漏/不回滚错"。AI 写的代码在前两遍往往很干净，问题基本都在第三遍暴露。

### 4.2 逐文件审查清单

| 文件 | 我要盯的点 | 看懂的标准 |
|---|---|---|
| `dto/common_dto.go` | `PageResult[T]` 是否**只有这一份**；字段是否 `list/total/page/pageSize`（不是 `items/data/records`） | 全项目 `grep "type PageResult"` 只有 1 处命中 |
| `model/persona.go` | `user_id` 是否 `json:"-"`；`state` 是否原始 JSON 类型；`LastMessageAt` 是否指针；`User` 关联字段是否在且有 `OnDelete:CASCADE` | 我能说出每个 tag 去掉之后会发生什么 |
| `model/proactive_setting.go` | 对 `Persona` 的外键是否 `ON DELETE CASCADE`；四个默认值是否与 DDL 一致（`true`/`30`/`120`/`3`） | `\d proactive_settings` 能看到 FK 与默认值 |
| `model/migrate.go` | `&User{}` → `&Persona{}` → `&ProactiveSetting{}` 顺序是否对 | `AutoMigrate` 一次跑过，无依赖顺序报错 |
| `dto/persona_dto.go` | 有没有偷偷声明 `userId` / `state` / `familiarity`；camelCase 是否逐字对齐契约 | 与契约 §4 的 JSON 示例并排比对，字段名一个字母都不差 |
| `repository/persona_repo.go` | **每个方法签名是否都带 `userID uint64`**；有没有 `db.First(&p, id)` 形态的裸查询；有没有 `ExistsByID`；更新是否用了 `Save(` | `grep -nE "Save\(|First\(&|ExistsBy" persona_repo.go` 无业务命中 |
| `repository/proactive_repo.go` | 播种函数是否**接受 `tx *gorm.DB`** 而不是自取包级 `db` | 参数表里有 `tx`；函数体内是 `tx.WithContext(ctx)` |
| `service/persona_service.go` | 未命中是否统一 `4040`（**不出现 4031/4043**）；事务是否用 `db.Transaction`；是否引了 `gin`；是否出现裸数字错误码 | `grep -n "gin" persona_service.go` 无结果；`grep -nE "\b(403[0-9]\|404[0-9]\|400[0-9])\b"` 无结果 |
| `handler/persona_handler.go` | 是否 `_ = c.Error(err)` 上抛；有没有 `response.Fail`；`userID` 是否只从 `c` 的 JWT 上下文取 | `grep -n "response.Fail" persona_handler.go` 无结果 |
| `router.go` | 只加了一行，没动别人的行 | `git diff router.go` 只有 1 行新增 |

### 4.3 六个高危点专项审查

| # | 高危点 | 为什么会错 | 我怎么验 |
|---|---|---|---|
| 1 | **资源越权（`4040`）** | "先查出来再在 Go 里比"的写法看起来逻辑完整，漏掉 `if` 却在 review 中极难发现；写了 `ExistsByID` 就白增泄漏面；照旧契约把 `4031` 写回来等于没落地这条规则 | 用**另一个账号的 Token** 打 `PUT` / `DELETE`，必须 `4040`；再用不存在的 id 打，必须拿到**同码同文案**的 `4040`；`grep` 确认没有裸 id 查询、没有探针、**没有任何 `4031`** |
| 2 | **`state` 被覆盖** | `Save()` 或 `Updates` 传全字段 struct 都会静默清空 JSONB | 手工把 `state` 改成 `{"familiarity":42,"self_note":"x"}` → `PUT` → 两个键都还在、`familiarity` 仍 42 |
| 3 | **`NULLS LAST` 漏写** | Postgres 的 `DESC` **默认是 `NULLS FIRST`**，写错的排序语义与需求正好相反，但页面看着"正常" | 新建一个从未聊过的人设，确认它排在列表**最后** |
| 4 | **级联删除没建出来** | 只写标量 `UserID` 时 GORM 不建外键，删除人设会**留下孤儿消息与孤儿配置行** | `\d personas` / `\d proactive_settings` 看 FK 是否带 `ON DELETE CASCADE`；删人设后数两张子表 |
| 5 | **错误码硬编码 / 用错码** | AI 容易直接写 `c.JSON(404, ...)` 或中文文案字面量；也容易**照着契约旧文把 `4031` 写回来**（那是功能越权的码） | `grep` 错误码数字与中文文案，必须全部指向 `errcode.*` 常量；且人设模块**不应出现 `4031`/`4043`**——越权与不存在都只有一个答案 `4040` |
| 6 | **事务没真正回滚** | 事务内用包级 `db` 而非 `tx`，或先提交人设再插配置——**代码看上去有 `Transaction`，实际没生效** | 用 §4.4 的"播种失败"实验：让播种插入报错，确认 `personas` **不留新行** |

### 4.4 必须亲眼看到的证据（不接受"我觉得没问题"）

```bash
# 0) 编译 + 静态检查（提交前必跑）
cd backend && go build ./... && go vet ./... && go test ./...

# 1) 外键与索引真的建出来了吗
#    用 docker exec 进容器，命令里不要写密码（红线 1）
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c '\d personas'
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c '\d proactive_settings'
#    要看到：FOREIGN KEY ... REFERENCES users(id) ON DELETE CASCADE（personas）
#            FOREIGN KEY ... REFERENCES personas(id) ON DELETE CASCADE（proactive_settings）
#            以及 idx_personas_user_last_msg

# 2) 级联删除（人设 + 配置行一起没）
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c \
  'SELECT count(*) FROM chat_messages WHERE persona_id = 1;
   SELECT count(*) FROM proactive_settings WHERE persona_id = 1;'
#    删人设后两条都必须是 0

# 3) 播种：创建后必须恰好 1 行、默认值正确
curl -s -X POST localhost:8080/api/v1/personas -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"小暖","personalityDesc":"温柔","speakingStyle":"轻柔"}'
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c \
  'SELECT enabled, interval_min, interval_max, daily_limit, last_nudge_at
     FROM proactive_settings WHERE persona_id = <新id>;'
#    期望：t | 30 | 120 | 3 | NULL，且行数 = 1

# 4) 排序：从未聊过的人设必须排在最后
curl -s "localhost:8080/api/v1/personas?page=1&pageSize=20" -H "Authorization: Bearer $TOKEN"

# 5) 越权与不存在：两者必须是同一个 4040
curl -s -X PUT "localhost:8080/api/v1/personas/1" -H "Authorization: Bearer $TOKEN_B" \
  -H 'Content-Type: application/json' \
  -d '{"name":"x","personalityDesc":"y","speakingStyle":"z"}'
curl -s -X DELETE "localhost:8080/api/v1/personas/1"      -H "Authorization: Bearer $TOKEN_B"
curl -s -X DELETE "localhost:8080/api/v1/personas/999999" -H "Authorization: Bearer $TOKEN"
#    三条都该是 code=4040、message 相同 —— 外部无法区分
#    另外：全程不应出现 4031 —— 那是「功能越权」的码，本模块没有该场景（spec §5.2）

# 6) 无 Token → 4010；缺 name → 4001；空列表 → list 为 []
curl -s localhost:8080/api/v1/personas

# 7) 事务回滚实验（用临时 _test.go，不进 PR）
#    把 CreateDefaultSettings 收到的 personaID 临时改成不存在的值（触发外键违例），
#    期望：整个事务回滚 —— personas 表里没有刚插入的那一行。
#    若人设留下而配置没插进去 = 事务没生效，必须打回。

# 8) code 层面四连 grep（都要无输出）
grep -rn "response.Fail"        internal/handler/persona_handler.go
grep -rn "Save("                internal/repository/persona_repo.go
grep -rnE "\"(403[0-9]|404[0-9]|400[0-9])\"" internal/ | grep -i persona
grep -rnE "4031|4043|ErrPersonaNotFound" internal/service internal/repository internal/handler
#    ↑ 期望无输出：越权与不存在都只走 ErrNotFound(4040)（spec §5.2）
grep -rn "type PageResult"      internal/dto/     # 期望只有 1 行

# 9) 红线 1 自查：不许有密钥进入本次改动
git status --short && git diff --cached --name-only | grep -E '\.env$|\.key$|\.pem$'
```

> `$TOKEN` / `$TOKEN_B` 从登录接口拿，**不要写死在脚本或 `_test.go` 里**（红线 1）。测试里需要真实数据时，用环境变量或本地 `.env`（已在 `.gitignore` 中）。

### 4.5 拒绝标准（出现任一条就打回重写，不做"小修小补"）

| 打回条件 | 为什么不能只小修 |
|---|---|
| 仓储层存在**不带 `user_id`** 的业务查询 | 防线是结构性的，逐处打补丁会漏；必须让不安全的方法不存在 |
| 出现 `ExistsByID` / 按 id 单查的探针 | "存在性"一进入代码就有了泄漏面，且与 §5.2 的决定直接矛盾 |
| 未命中返回 `4031` / `4043` | 违反队长规则（**资源越权 → `4040`**）；`4031` 是功能越权的码、本模块无此场景，`4043` 与 `4040` 并存**会泄漏存在性** |
| **创建人设不是单事务**（用了包级 `db`、或分两次提交） | 会留下孤儿配置行，正是本版决策要消灭的问题；且这类 bug 在演示时表现为"某个伴侣设置页打不开" |
| 更新用了 `Save()` 或全字段 `Updates(struct)` | 覆盖 `state` 是数据损坏，且**不可逆**；必须改成指定列 |
| 未命中时返回成功（200） | 越权写入被伪装成成功，比报错危险 |
| 出现裸数字错误码 / 中文错误文案字面量 | 破坏"一 code 一 msg"（红线 6），一处放纵会蔓延 |
| service 里出现 `*gin.Context` | 破坏分层（红线 7），后续无法单测 |
| 外键未带 `ON DELETE CASCADE`（两张表都要查） | 孤儿数据会污染记忆/画像/朋友圈/主动消息，且事后清理成本高 |
| `PageResult` 出现第二份定义 | 两份同名类型在联调时会出现"字段对不上"的诡异问题，最难排查 |

**审查通过的唯一标准**：§4.4 的 10 组命令全部实跑过，且 §4.2 每个文件我都能逐行解释。**审查笔记落到 `docs/dev_notes/persona_model_notes.md`**，作为"我确实看懂了"的证据留档。

## 5. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| 成员 1 的公共层尚未交付 | 步骤 6-8 无法编译，阻塞联调 | 先做步骤 0-5；**不要自造 `pkg/*`**；把等待时间用于 §4.4 的 SQL 级验证 |
| **全局文件未同步就合并** | 成员 2 的 Mock 按 `4031`/`4043` 写，联调时错误分支全对不上；后续 AI 代理照 `AGENTS.md` §4.3 把 `4031` 写回来 | 步骤 10：**先广播规则**，再等队长 / 成员 1 更新 `API_CONTRACT.md` + `AGENTS.md`（spec §5.2 的 7 项表）；**未同步不得合并**。本分支不碰这两个文件 |
| 通用规则只改了人设模块，契约里另 10 个归属校验端点仍带 `4031` | 长期"半套规则"共存：同一个 `personaId` 归属失败，在不同端点返回不同码，前端与测试都难以统一 | 广播时**把 10 个端点的逐行清单一起交出去**（spec §5.2 末尾的表格，含契约行号与改法），由队长排一次统一整改；本 PR 只做 §4 |
| `state` 的写入方（对话链路）未定 | 本功能写 `{"familiarity":0}`，对话链路要 `+1`，两边格式不一致会互相踩 | 本功能只初始化、不累加；把 `state` 的 schema 约定写进 spec §3.3，交付时同步给成员 1 |
| 播种的默认值与主动消息模块不一致 | 两处各写一套默认值，用户改过配置后被覆盖 | 默认值只在 DDL 与 `CreateDefaultSettings` 一处定义；主动消息模块的 `GET` 直接读表，不写第二套兜底默认 |
| `proactive_repo.go` 后续被主动消息模块大改 | 函数签名变了，本功能的调用点要跟着改 | 播种函数签名保持 `(ctx, tx, userID, personaID)`，不要加可选参数 |
| 新增 `gorm.io/datatypes` 未广播 | `go.mod` 冲突 | 群里说一声；`go.mod` + `go.sum` 一并提交 |
| 契约文档 `state: {}` 与 `familiarity: 12` 的不一致 | 成员 2 写 Mock 时可能理解成独立列 | 实现后主动告知，避免前端 Mock 与真实响应结构不一致 |

## 6. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-13 | spec / plan 第 1 版落地 | 成员 1 的 `pkg/response` / `pkg/errcode` / `middleware` 尚未进仓库（仅阻塞步骤 6-8） |
| 2026-09-13 | 并入 4 项决策：`PageResult` 位置、ID 统一 `uint64`、创建时同事务播种 `proactive_settings`（新增步骤 0/2/10 与 §3.4）、`:id` 未命中统一 `4040`（契约变更，新增步骤 10 阻塞合并） | 同上；**契约变更待广播** |
| 2026-09-13 | 套用队长「通用错误码规则」（资源越权 → `4040` / 功能越权 → `4031`）：步骤 10 改为「广播规则 + 全局文件由队长/成员 1 改，本分支不动」；§3.3 / §4.3 #1 #5 / §4.4 / §4.5 / §5 同步措辞；新增「半套规则」风险行 | **等用户群里广播**；广播 + 全局同步完成前不合并（步骤 10） |

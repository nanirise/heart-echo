# plan · 人设 CRUD（Persona CRUD）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-persona-model`
> 负责人：成员 3 ｜ 创建：2026-09-13 ｜ 最后更新：2026-09-13

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 前置 | 状态 |
|---|---|---|---|---|---|
| 1 | **翻译 Struct** | `internal/model/persona.go` | 8 列 + 外键关联全被声明；字段对照 spec §3.2 逐行对齐；每字段带注释 | 无 | 未开始 |
| 2 | 建表 | `internal/model/migrate.go` | `AutoMigrate` 追加 `&Persona{}`，顺序在 `&User{}` 之后；`\d personas` 有 CASCADE 外键 | 步骤 1 | 未开始 |
| 3 | 请求/响应结构 | `internal/dto/persona_dto.go` | 3 个结构体；camelCase 对齐契约 §4；`familiarity` 拍平逻辑在此 | 步骤 1 | 未开始 |
| 4 | **仓储层** | `internal/repository/persona_repo.go` | 方法签名全部强制带 `userID`；不存在"只按 id 查"的业务方法 | 步骤 1 | 未开始 |
| 5 | **业务层** | `internal/service/persona_service.go` | `4031` / `4043` 判定在 service；不依赖 `*gin.Context` | 步骤 4 + 成员 1 的 `pkg/errcode` | 未开始 |
| 6 | **HTTP 层** | `internal/handler/persona_handler.go` | 薄；含 `RegisterPersonaRoutes`；错误 `_ = c.Error(err)` 上抛 | 步骤 5 + `pkg/response` + `middleware` | 未开始 |
| 7 | 路由挂载 | `router.go`（成员 1 的文件） | 群里同步后加一行，**不要与成员 1 同时改** | 步骤 6 | 未开始 |
| 8 | 越权与级联专项验证 | — | spec §8 的 12 项验收逐条打勾 | 步骤 7 | 未开始 |
| 9 | **人工审查 + PR** | `docs/dev_notes/persona_model_notes.md`、PR | §4 的审查清单全过；至少 1 人 Approve | 步骤 8 | 未开始 |

**优先级与阻塞说明**：步骤 1-4 **不等任何人**，现在就能做完——这是本功能的关键路径。步骤 5-7 依赖成员 1 尚未交付的 `pkg/response` / `pkg/errcode` / `internal/middleware`（见 spec §7.1），**在它们落地前不要去自己造一份**（红线：不 Own 的东西不要动，造了必冲突）。等待期间用步骤 8 里那几条 SQL 直接对 PG 验证仓储层。

## 2. 文件清单

| 路径 | 作用 | 谁还会用到 |
|---|---|---|
| `backend/internal/model/persona.go` | `personas` 的 GORM 实体 | **成员 1**（`message_repo` 引 `Persona`）、成员 3 |
| `backend/internal/model/migrate.go` | 建表入口 | 成员 1（`main.go` 调用） |
| `backend/internal/dto/persona_dto.go` | 请求 / 响应结构 | 成员 2（据此写 `types/persona.ts` 与 Mock） |
| `backend/internal/repository/persona_repo.go` | 数据访问 | 成员 3（主动消息定时任务要读 `last_message_at`） |
| `backend/internal/service/persona_service.go` | 归属校验与业务逻辑 | 成员 1 / 成员 3（记忆、日程的越权判定沿用同一套） |
| `backend/internal/handler/persona_handler.go` | 4 个端点 + `RegisterPersonaRoutes` | 成员 1（挂载一行） |
| `docs/dev_notes/persona_model_notes.md` | **我的审查笔记**（不是给别人的文档） | 只有我；是新文件，与 `user_model_notes.md` 同级 |

## 3. 关键实现要点

### 3.1 翻译 Struct（步骤 1）

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

### 3.2 Repository（步骤 4）

**核心是"让不安全的方法不存在"**，而不是"记得每次都校验"：

```go
// 方法签名强制带 userID，调用方没有"忘了传"的选项
ListByUser(ctx context.Context, userID uint64, offset, limit int) ([]model.Persona, int64, error)
UpdateOwned(ctx context.Context, userID, personaID uint64, req *dto.UpdatePersonaRequest) error
DeleteOwned(ctx context.Context, userID, personaID uint64) error
ExistsByID(ctx context.Context, personaID uint64) (bool, error)   // 仅供归属判定，业务不复用
```

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
if res.RowsAffected == 0 { /* 交给 service 判 4031 / 4043 */ }
```

> `Updates` 传 `map` 而非 struct：传 struct 时 GORM **忽略零值字段**，`name` 传空串会被静默跳过（这里虽然 `required` 拦住了，但别依赖另一层的校验来保证这一层的行为）。`RowsAffected == 0` 要向上暴露成"未命中"，**不能当成成功**——否则改别人的数据会返回 200。

**删除**：`Where("id = ? AND user_id = ?").Delete(&model.Persona{})`，级联交给外键，**不写任何多表删除代码**。

### 3.3 Service（步骤 5）

- 签名收 `context.Context`，**不出现 `*gin.Context`**（红线 7）。
- 归属判定（spec §5.2）：`UpdateOwned` / `DeleteOwned` 返回"未命中"时，再调 `ExistsByID` 二分出 `4031` 与 `4043`。
- 错误一律 `errcode.New` / `errcode.Wrap`，**不拼接自定义文案**（红线 6）。
- `userID` 的 `0` 值要当错误拦住：`0` 说明上游鉴权失败，**不能拿它去查库**（spec §5.1 第 3 条）。

### 3.4 DTO 与 Handler（步骤 3 / 6）

- DTO 里**不声明** `userId` / `state` / `familiarity` / `lastMessageAt` —— 字段不存在，就没有被传入和被覆盖的可能。多传的键 Gin 默认忽略，正合契约「`PUT` 时忽略」。
- `familiarity` 的拍平（容错取 0）写法见 spec §4.6。
- Handler 只做三件事：绑定 → 调 service → `response.Success`；出错 `_ = c.Error(err)` 后 `return`。**不写 `response.Fail`**。
- `RegisterPersonaRoutes` 照总纲 §4.5 的样式写，`router.go` 由成员 1 加一行（或错开时间自己加）。

---

## 4. 我手动审查 AI 代码的计划

> 前提：AI 生成的代码**默认不可信**，尤其是"越权防线"和"`state` 保留"这两处——它们写错了照样能跑通、照样能演示，只在特定输入下才暴露。所以审查的**目标不是"能跑"，是"我能逐行解释它为什么安全"**。

### 4.1 审查方法（三遍，对应我审 `User` struct 时用的做法）

| 遍 | 做什么 | 判据 |
|---|---|---|
| **第一遍 · 逐行重写注释** | 把 AI 产出的每个文件抄进 `docs/dev_notes/persona_model_notes.md`，**用自己的话**给每一行加注释 | **写不出注释的那一行，就是我没看懂的地方** → 那里就是审查重点，去查 GORM 文档或问，不放过 |
| **第二遍 · 对照核对** | 拿 DDL（spec §3.1）+ 契约 §4 + spec 全文，逐字段/逐端点核对，填成表格 | 每个 JSON 字段名都能在契约里指到来源；找不到来源的字段一律删掉 |
| **第三遍 · 对抗验证** | 主动构造能打破代码的输入（跨账号、空列表、不存在的 id、无 `familiarity` 的 `state`、重复删除、超长字段），按 §4.4 的命令实跑 | 每个高危点都有一次实跑记录，不是"读代码觉得没问题" |

**第三遍是这份计划的核心**：前两遍只能证明"代码符合我的预期"，第三遍才能证明"代码在我不期望的输入下不崩/不泄漏"。AI 写的代码在前两遍往往很干净，问题基本都在第三遍暴露。

### 4.2 逐文件审查清单

| 文件 | 我要盯的点 | 看懂的标准 |
|---|---|---|
| `model/persona.go` | `user_id` 是否 `json:"-"`；`state` 是否原始 JSON 类型；`LastMessageAt` 是否指针；`User` 关联字段是否在且有 `OnDelete:CASCADE` | 我能说出每个 tag 去掉之后会发生什么 |
| `model/migrate.go` | `&User{}` 是否在 `&Persona{}` 之前 | `AutoMigrate` 一次跑过，无依赖顺序报错 |
| `dto/persona_dto.go` | 有没有偷偷声明 `userId` / `state` / `familiarity`；camelCase 是否逐字对齐契约 | 与契约 §4 的 JSON 示例并排比对，字段名一个字母都不差 |
| `repository/persona_repo.go` | **每个方法签名是否都带 `userID`**；有没有 `db.First(&p, id)` 形态的裸查询；更新是否用了 `Save(` | `grep -n "Save(\|First(&" persona_repo.go` 无业务命中 |
| `service/persona_service.go` | `4031`/`4043` 判定次序；是否引了 `gin`；是否出现裸数字错误码 | `grep -n "gin" persona_service.go` 无结果；`grep -nE "\b(403[0-9]\|404[0-9]\|400[0-9])\b"` 无结果 |
| `handler/persona_handler.go` | 是否 `_ = c.Error(err)` 上抛；有没有 `response.Fail`；`userID` 是否只从 `c` 的 JWT 上下文取 | `grep -n "response.Fail" persona_handler.go` 无结果 |
| `router.go` | 只加了一行，没动别人的行 | `git diff router.go` 只有 1 行新增 |

### 4.3 五个高危点专项审查

| # | 高危点 | 为什么会错 | 我怎么验 |
|---|---|---|---|
| 1 | **越权（`4031`）** | "先查出来再在 Go 里比"的写法看起来逻辑完整，漏掉 `if` 却在 review 中极难发现 | 用**另一个账号的 Token** 打 `PUT` / `DELETE`，必须 `4031`；再 `grep` 确认没有裸 id 查询 |
| 2 | **`state` 被覆盖** | `Save()` 或 `Updates` 传全字段 struct 都会静默清空 JSONB | 手工把 `state` 改成 `{"familiarity":42,"self_note":"x"}` → `PUT` → 两个键都还在、`familiarity` 仍 42 |
| 3 | **`NULLS LAST` 漏写** | Postgres 的 `DESC` **默认是 `NULLS FIRST`**，写错的排序语义与需求正好相反，但页面看着"正常" | 新建一个从未聊过的人设，确认它排在列表**最后** |
| 4 | **级联删除没建出来** | 只写标量 `UserID` 时 GORM 不建外键，删除人设会**留下孤儿消息** | `\d personas` 看 FK 是否带 `ON DELETE CASCADE`；删人设后数 `chat_messages` |
| 5 | **错误码硬编码** | AI 容易直接写 `c.JSON(403, ...)` 或 `"该人设不属于当前用户"` 字面量 | `grep` 错误码数字与中文文案，必须全部指向 `errcode.*` 常量 |

### 4.4 必须亲眼看到的证据（不接受"我觉得没问题"）

```bash
# 0) 编译 + 静态检查（提交前必跑）
cd backend && go build ./... && go vet ./... && go test ./...

# 1) 外键与索引真的建出来了吗
#    用 docker exec 进容器，命令里不要写密码（红线 1）
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c '\d personas'
#    要看到：FOREIGN KEY ... REFERENCES users(id) ON DELETE CASCADE
#            以及 idx_personas_user_last_msg

# 2) 级联删除
#    删人设前/后各数一次，后一次必须是 0
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c 'SELECT count(*) FROM chat_messages WHERE persona_id = 1;'

# 3) 排序：从未聊过的人设必须排在最后
curl -s "localhost:8080/api/v1/personas?page=1&pageSize=20" -H "Authorization: Bearer $TOKEN"

# 4) 越权：换 B 账号的 Token 打 A 的人设 → 4031
curl -s -X PUT "localhost:8080/api/v1/personas/1" -H "Authorization: Bearer $TOKEN_B" \
  -H 'Content-Type: application/json' \
  -d '{"name":"x","personalityDesc":"y","speakingStyle":"z"}'
curl -s -X DELETE "localhost:8080/api/v1/personas/1" -H "Authorization: Bearer $TOKEN_B"

# 5) 不存在的 id → 4043；无 Token → 4010；缺 name → 4001
curl -s -X DELETE "localhost:8080/api/v1/personas/999999" -H "Authorization: Bearer $TOKEN"

# 6) code 层面三连 grep（都要无输出）
grep -rn "response.Fail"      internal/handler/persona_handler.go
grep -rn "Save("              internal/repository/persona_repo.go
grep -rnE "\"(403[0-9]|404[0-9]|400[0-9])\"" internal/ | grep persona

# 7) 红线 1 自查：不许有密钥进入本次改动
git status --short && git diff --cached --name-only | grep -E '\.env$|\.key$|\.pem$'
```

> `$TOKEN` / `$TOKEN_B` 从登录接口拿，**不要写死在脚本或 `_test.go` 里**（红线 1）。测试里需要真实数据时，用环境变量或本地 `.env`（已在 `.gitignore` 中）。

### 4.5 拒绝标准（出现任一条就打回重写，不做"小修小补"）

| 打回条件 | 为什么不能只小修 |
|---|---|
| 仓储层存在**不带 `user_id`** 的业务查询 | 防线是结构性的，逐处打补丁会漏；必须让不安全的方法不存在 |
| 更新用了 `Save()` 或全字段 `Updates(struct)` | 覆盖 `state` 是数据损坏，且**不可逆**；必须改成指定列 |
| 未命中时返回成功（200） | 越权写入被伪装成成功，比报错危险 |
| 出现裸数字错误码 / 中文错误文案字面量 | 破坏"一 code 一 msg"（红线 6），一处放纵会蔓延 |
| service 里出现 `*gin.Context` | 破坏分层（红线 7），后续无法单测 |
| 外键未带 `ON DELETE CASCADE` | 孤儿数据会污染记忆/画像/朋友圈，且事后清理成本高 |

**审查通过的唯一标准**：§4.4 的 7 组命令全部实跑过，且 §4.2 每个文件我都能逐行解释。**审查笔记落到 `docs/dev_notes/persona_model_notes.md`**，作为"我确实看懂了"的证据留档。

## 5. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| 成员 1 的公共层尚未交付 | 步骤 5-7 无法编译，阻塞联调 | 先做步骤 1-4；**不要自造 `pkg/*`**；把等待时间用于步骤 8 的 SQL 级验证 |
| `PageResult[T]` 位置未定，重复定义 | 两处定义类型不兼容，联调时字段对不上 | 实现前问成员 1；建议统一放 `pkg/response` |
| `ContextKeyUserID` 类型不符（`uint` vs `uint64`） | `GetUint64` 断言失败 → 取到 0 → **静默返回空列表**，看着像"没有数据" | 实现前问清类型；并在 §5.1 加 `userID == 0` 拦截，把静默失败变成 `4010` |
| 新增 `gorm.io/datatypes` 未广播 | `go.mod` 冲突 | 群里说一声；`go.mod` + `go.sum` 一并提交 |
| `state` 的写入方（对话链路）未定 | 本功能写 `{"familiarity":0}`，对话链路要 `+1`，两边格式不一致会互相踩 | 本功能只初始化、不累加；把 `state` 的 schema 约定写进 spec §3.3，交付时同步给成员 1 |
| `proactive_settings` 播种未定（spec §7.3 第 4 条） | 主动消息上线时 `GET /proactive/settings` 可能撞 `4043` | 实现前定清楚，写进 PR 描述 |
| 契约文档 `state: {}` 与 `familiarity: 12` 的不一致 | 成员 2 写 Mock 时可能理解成独立列 | 实现后主动告知，避免前端 Mock 与真实响应结构不一致 |

## 6. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-13 | spec / plan 落地，等待实现 | 成员 1 的 `pkg/response` / `pkg/errcode` / `middleware` 尚未进仓库（仅阻塞步骤 5-7） |

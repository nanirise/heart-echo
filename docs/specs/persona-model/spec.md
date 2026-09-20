# spec · 人设 CRUD（Persona CRUD）

> 本文件是**合同**：定义本功能做什么、做到什么算完成。合并后即冻结，任何字段或行为变更必须升版本并在群里广播。
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：`personas` DDL、契约字段、错误码表、通用约定一律**不复制**，只给链接；
> 本文件只写**本功能独有**的内容——独有陷阱、独有决策、独有验收项。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-persona-model`（路由部分 `feature/backend-persona-routes`，均已合并） |
| 状态 | 模型层 + 4 个端点已落地（PR #42 已合并） |
| **本功能范围** | `personas` 实体 + 4 个 CRUD 端点 + 创建人设时**同事务播种一行 `proactive_settings`** |
| 负责人 | 成员 3（数据 + 人设 + 部署） |
| 关联契约 | [API_CONTRACT §4](../../API_CONTRACT.md)（4 个端点 + Persona 实体）、§2（错误码总表） |
| 关联设计 | [TECH_DESIGN §6.1 / §6.2 / §5.6](../../TECH_DESIGN.md)、[总纲 §0.2 / §0.3](../../dev/MASTER.md)、[成员 3 任务书 §2](../../dev/MEMBER_3_DATA_MOMENTS_DEPLOY.md) |
| 关联红线 | [AGENTS §4.3](../../../AGENTS.md)（数据边界）、§5.1（注释纪律）、§7 |

---

## 0. 范围声明（先读这一节）

一个人设 = 一个 AI 伴侣 = 一个对话，`personas` 表**同时就是对话列表**（无独立会话表，TECH_DESIGN §6.1）。它是全项目第一个业务模块，也是人设管理页 / 聊天页 / 画像页的共同前置：聊天页靠 `GET /personas` 决定打开哪个对话；记忆、画像、日程全部挂在 `persona_id` 上——**这里的归属校验是它们共同的第一道闸门，这里漏了，后面每张子表都跟着漏**。

### 0.1 做什么

`internal/model/`（`persona.go`、`proactive_setting.go`、`migrate.go` 追加两行）、`internal/dto/persona_dto.go`、`internal/repository/`（`persona_repo.go`、`proactive_repo.go`）、`internal/service/persona_service.go`、`internal/handler/persona_handler.go`（含 `RegisterPersonaRoutes`）。

### 0.2 不做什么（**不该写的代码比该写的更重要**）

| 不做的事 | 原因 / 去向 |
|---|---|
| 人设管理页 UI、`api/persona.ts`、`stores/persona.ts` | 只做后端；前端页面是独立一步 |
| `GET`/`PUT /proactive/settings`、定时扫描、`TriggerNow` | 成员 3 的主动消息模块。**本功能只播种一行默认配置** |
| **人设名去重 / 数量上限** | 总纲 §0.3：数量不限；DDL 上 `name` 无 UNIQUE，**重名合法**。不要自作主张加 `4004` |
| `familiarity` 的累加 | 归对话链路（TECH_DESIGN §5.6）。本功能**只读展示，不写** |
| 单独清空对话历史 | 总纲 §0.2：**不可单独清空**，只能随人设一起删 |
| `state.self_note` 等演化字段的写入 | TECH_DESIGN §5.6 的加分项。本功能只负责**原样保留**，不解析、不写入 |
| 情绪相关任何字段 | 人设表没有情绪字段；`state` 是**人格状态**，不是情绪，不要混为一谈 |
| 新增错误码 | 一个都不需要，复用 `4001` / `4043` / `5003` |
| 删 `pkg/errcode.ErrPersonaNotFound` | 契约 §5/§7/§9/§10 的端点仍在用它，且那是成员 1 的文件 |
| 手写 `ALTER TABLE` / `DropTable` | 红线 8：改结构 = 改 struct + `AutoMigrate` |

### 0.3 两个共用落点

- `PageResult[T]` 定义在 `internal/dto/common_dto.go`，personas / messages / memory / moments / schedules **五个分页端点共用**——**不要各自再定义一份**。
- `router.go` 只加一行 `RegisterPersonaRoutes(protected, ...)`。该文件是多人交汇点，**不要与成员 1 同时改**。

---

## 1. 本功能独有的硬性要求

### 1.1 `state` 的三条硬约束

| 约束 | 说明 |
|---|---|
| **只读透传** | 前端不解析、不回传修改；服务端读出来原样放进响应，不 reshape |
| **写入时原样保留** | `PUT` / `DELETE` 路径**绝不触碰** `state` 列（§1.4） |
| **未知键必须活下来** | TECH_DESIGN §5.6 预留了 `state.self_note`。**不能**定义成固定字段的 struct——读写一次就把未知键丢了 |

`familiarity` **不是数据库列**，它存在 `state` 里（`{"familiarity": 0}`）。响应体里它是**顶层字段**，需要从 `state` 里解出来再拍平（`PersonaResponse` 的转换在 DTO 层，不写在 model 上）。取值的容错写法：只解 `familiarity` 一个键，**解析失败或键缺失取 `0`**，不要让整个列表接口 500。

> ⚠️ **契约 §4 示例有一处文档瑕疵**：它同时给出 `"state": {}` 与 `"familiarity": 12`，看起来像两件独立的东西；以 DDL 注释与 §5.6 为准。实现时提醒成员 2 一句，免得他写 Mock 时理解成独立列。

### 1.2 `model.JSONB`：自写，零新增依赖

`personas.state` 与 `user_profile.profile_data` 共用 `internal/model/jsonb.go`：

- **不用 `json.RawMessage`**：经 GORM + pgx 写 `jsonb` 列时会被当成 `bytea`，报 `column "state" is of type jsonb but expression is of type bytea`。关键在 `Value()` 必须返回 `string(j)`，**不能返回 `[]byte(j)`**。
- **不用 `gorm.io/datatypes`**：会拖进 MySQL 驱动等一串与本项目（纯 PostgreSQL）无关的依赖，且与成员 1 接 PGX 时的 `go.mod` 改动冲突。
- ⚠️ **必须实现 `MarshalJSON` / `UnmarshalJSON`**：底层类型是 `[]byte`，而 `encoding/json` 对 `[]byte` 默认做 **base64 编码**。少了这两个方法，`"state": {"familiarity": 0}` 会变成 `"state": "eyJmYW1pbGlh..."`——**它照样能编译、能跑、能演示**，只在看响应体时才暴露。

### 1.3 外键：必须显式声明关联字段；`bigint` 不是 `bigserial`

只写标量 `UserID uint64` 时 GORM **不会创建外键约束**——必须额外声明 belongs-to 关联字段（见 `persona.go`）。`proactive_setting.go` 对 `Persona` 同理，**外键也要 `ON DELETE CASCADE`**，否则删人设会留下孤儿配置行。`Create` 时**不要给关联字段赋值**（保持零值），否则 GORM 会尝试连带写 `users` 表。

> ⚠️ **被引用方的 `ID` tag 不能写 `type:bigserial`**（实测踩过，已修）：
> GORM 建 belongs-to 关联时会把**被引用主键的 `DataType` 复制到外键字段**上（它只检查类型串里有没有 `auto_increment` / `primary key`，而 `"bigserial"` 两者都没有，于是照抄）。Postgres 里 `bigserial` 的语义是「建序列 + 设为默认值」，结果 `personas.user_id` 被建成 `bigint NOT NULL DEFAULT nextval('personas_user_id_seq')`——**既偏离 DDL，又让漏传 `user_id` 的 INSERT 静默拿到一个可能撞上真实用户 id 的序列值**。
> **正确写法是 `type:bigint` + `autoIncrement`**：渲染结果仍是 `bigserial`，但 `DataType` 是 `bigint`，复制到外键就是对的。在本字段上加 `autoIncrement:false` **压不住**它。
> **这条对整个项目生效**：`chat_messages` / `user_memory` / `user_profile` / `schedules` / `ai_moments` / `proactive_settings` 的 `user_id` 全是同一个模式。

### 1.4 `PUT` 只更新三列，绝不 `Save()`

`db.Save(&p)` 写回全部列，会把 `state` 覆盖成零值、把 `last_message_at` 清空——**不可逆的数据损坏**。只用 `Updates` 指定 `name` / `personality_desc` / `speaking_style` 三列，且**传 `map` 而非 struct**（传 struct 时 GORM 忽略零值字段）。`RowsAffected == 0` 必须向上暴露成"未命中"，**不能当成功**——否则改别人的数据会返回 200。

更新后要**重新读一次**该行再返回：响应里的 `state` / `familiarity` / `lastMessageAt` 必须是库里的真值，不是拼装的。

### 1.5 列表排序：`NULLS LAST` 与 `id DESC` 都不能省

```sql
ORDER BY last_message_at DESC NULLS LAST, id DESC
```

1. Postgres 的 `DESC` **默认是 `NULLS FIRST`**——只写 `DESC` 的话，**刚创建、还没聊过的人设会排在最前面**，与需求正好相反（DDL 的索引也写的是 `NULLS LAST`，这是权威意图）。**GORM 的 `index` tag 表达不了 `NULLS LAST`**，但查询本身必须显式写；**不要为它手写 DDL**（红线 8）。
2. `id DESC` 兜底：大量人设的 `last_message_at` 都是 `NULL`，只按它排序时同值行顺序不确定——翻页会「第 2 页冒出第 1 页的人」或漏项。
3. `total` 必须用**同一个 `user_id` 条件**统计，不能 `COUNT(*)` 全表。
4. 空列表返回 `[]` 而不是 `null`（Go 的 nil slice 序列化成 `null`，前端 `v-for` 直接崩）。`page` 越界返回空列表 + 真实 `total`，仍是 `200`。

**列表端点不存在越权场景**：它不带任何 `:id`，`user_id` 只来自 Token，查询天然被限制在当前用户名下。**空就是空**——不要加"查不到就 4043"，那会让前端无法区分"我还没有人设"和"请求失败"。

---

## 2. 越权防线与错误码（本功能最重要的正确性约束）

对应红线 3（跨用户 / 跨人设泄漏）。**`user_id` 只有一个来源：JWT 声明**，永不从 body / query / path / header 读。请求 DTO 里**不声明** `userId` 字段——字段不存在就没有被传进来的可能（结构性防御，比"记得校验"可靠）。

**归属校验下沉到 SQL 的 `WHERE` 里**（`WHERE id = ? AND user_id = ?`），而不是"先查出来再在 Go 里比 `UserID`"：后者忘写那个 `if` 就静默越权，而这类 bug 在 review 里极难发现（代码"看起来逻辑完整"）。仓储层因此**不提供任何"按 id 单查"或"判断存在性"的方法**——少一个方法就少一处泄漏面。

**取不到 `userID`（`ok == false` 或 `0`）→ `4010` 并中断**，绝不退化成 `userID = 0` 继续查：`WHERE user_id = 0` 返回空集，**看起来没泄漏，实际是把鉴权失败伪装成了空列表**，这种静默降级比报错危险得多。

### 2.1 `:id` 未命中一律 `4043`

| 越权类型 | 定义 | 错误码 |
|---|---|---|
| **资源越权** | 功能你能用，但**资源不属于你** | **`4043`** —— 隐藏资源存在性 |
| **功能越权** | 功能本身你无权使用（如普通用户访问管理员接口） | **`4030`** |

人设 CRUD 全程属**资源越权**侧：`PUT` / `DELETE /personas/:id` 的 `WHERE id = ? AND user_id = ?` 未命中，**一律 `errcode.ErrPersonaNotFound`（4043）**，**不区分**「人设是别人的」与「人设不存在」。

> **关键推论：「不存在」也不能给 `4040`。** `id` 是全局自增。若"不属于你"回 `4043`、"不存在"回 `4040`，调用方拿同一个 id 各打一次就能判断它**到底存不存在**——隐藏存在性的目的当场失效。所以**同一个端点内 `4040` 与 `4043` 不能并存**。
> **本模块没有功能越权场景**（契约 §4 的 4 个端点对所有登录用户一视同仁，项目无管理员接口），因此**不产生 `4030`**。
> **代价（诚实记录）**：调试时"改别人的人设"与"改不存在的 id"返回同一个码，排查略麻烦——靠服务端日志区分（`BizErrorHandler` 为 4xxx 记 warn 日志）。
> **附带收益**：不需要"先带归属条件查、未命中再按 id 探针二分"的两步判定——那两步的唯一目的就是区分这两个码。现在一条 `WHERE id = ? AND user_id = ?` 就够，正常与异常路径都是**一次查询**。

**校验失败一律 `4001`**：契约 §4 这组端点只列了 `4001`，所以 `binding` 失败（缺 `name`、超长）统一返回 `errcode.ErrInvalidParams`。**不要**因为"缺必填字段"改成 `4002`——那是别的端点的语义，擅自细化就是契约漂移。

> 📌 **`4030` 的全局收敛不在本功能范围内**：按同一规则，契约里另有若干带归属校验的端点也应为 `4043`。它们的契约行与代码属其他模块，**不在本 PR 内顺手改**——那会让 review 无法界定范围，且"代码改了、契约没改"等于制造新的不一致。由队长统一安排一次性扫干净。

---

## 3. 创建人设：同事务播种 `proactive_settings`

`POST /personas` 成功时，`proactive_settings` 必须存在一行对应该人设的配置，取 DDL 默认值（`enabled=true` / `interval_min=30` / `interval_max=120` / `daily_limit=3` / `last_nudge_at=NULL`）。

**为什么必须播种**：主动消息模块靠 `WHERE persona_id = ?` 读这张表。创建人设时不留行，该人设的 `GET /proactive/settings` 就查不到 → **报 404**，在演示现场表现为"某个 AI 伴侣的主动消息设置打不开"。**用"创建时播种"把问题消灭在源头**，也让 `GET /proactive/settings` 可以无条件返回真实行、不必写"无行则返回默认值"的兜底分支。

**三条实现约束：**

1. **同一个事务**：先提交人设、再插配置而后者失败，就**恰好留下我们要避免的那种孤儿**。事务边界在 **service 层**（红线 7），repo 只接受事务句柄 `tx`。
2. **用 `db.Transaction(...)` 而不是手工 `Begin`/`Commit`**：前者在 `return err` 时自动回滚、panic 时也回滚（配 `Recovery` 中间件）；手工管理一旦漏掉 `Rollback`，连接会被占住。
3. **`tx` 与 `ctx` 都要往下传**，repo 内部用 `tx.WithContext(ctx)`。**用包级 `db` 就等于脱离了事务**，回滚时这条插入不会被撤销，事务白做。

`p.ID` 要在 `Create` 成功**之后**才能取（GORM 回填自增 id），顺序不能反。`UNIQUE (user_id, persona_id)` 是 DDL 上已有的约束，播种用 `INSERT` 即可；**不要写成 upsert** 去掩盖"重复创建"的 bug——真重复了就该报错。

---

## 4. 验收标准

**A · 模型层**

- [ ] `cd backend && go build ./... && go vet ./... && go test ./...` 全通过
- [ ] `AutoMigrate` 建出 `personas` 与 `proactive_settings`；`\d personas` 能看到 **`ON DELETE CASCADE` 外键**与组合索引
- [ ] **序列不污染**：`personas` / `proactive_settings` 的 `user_id` 等外键列**没有** `nextval` 默认值（§1.3）

**B · 端点与越权**

- [ ] 4 个端点 curl 全通：列表 / 创建 / 编辑 / 删除
- [ ] **资源越权（`:id`）**：用 B 的 Token 打 A 的 `personaId` → **`4043`**（`PUT` 与 `DELETE` 各验一次）
- [ ] **不存在**：不存在的 id → **`4043`**，且 `message` 与上一条**逐字相同**（这正是隐藏存在性要的效果）
- [ ] **不产生 `4030` / `4040`**：任意输入组合（越权 / 不存在 / 缺参 / 无 Token / 超长字段）都不出现这两个码
- [ ] 不带 Token / Token 无效 → `4010` / `4011`（由中间件产生）；校验失败（缺 `name`、超长）→ `4001`
- [ ] **排序**：新建一个从未聊过的人设，它排在列表**最后**（不是最前）
- [ ] **空态**：无任何人设时 `data.list` 是 `[]` 而不是 `null`，且 `code` 为 `200`

**C · `state` / `familiarity`**

- [ ] **`state` 是 JSON 对象而不是 base64**：响应里形如 `"state":{"familiarity":0}`，**不是** `"state":"eyJmYW1pbGlh..."`
- [ ] **`state` 保留**：把某行 `state` 手工改成 `{"familiarity": 42, "self_note": "x"}`，`PUT` 之后**两个键都还在**、`familiarity` 仍为 42
- [ ] **`familiarity` 拍平**：`GET` 返回**顶层** `familiarity`，且 `state` 原样透传
- [ ] **`familiarity` 容错**：`state` 为 `{}`（无该键）时取 `0`，**不要让整个列表接口 500**

**D · 事务与播种**

- [ ] **播种**：`POST /personas` 后 `SELECT * FROM proactive_settings WHERE persona_id = <新id>` 有且仅有 1 行，四个默认值正确、`last_nudge_at IS NULL`
- [ ] **事务性**：人为让 `proactive_settings` 插入失败（临时注入一个非法约束）→ **`personas` 不得残留新行**
- [ ] **`PUT` 不误伤**：改人设名字后 `proactive_settings` 行未被修改
- [ ] **级联删除**：删人设后 `SELECT count(*) FROM chat_messages WHERE persona_id = <已删id>` = 0

**E · 代码层**

- [ ] grep 通过：无硬编码错误码数字/文案、无 `response.Fail` in handler、无 `Save(` on persona、无 secrets
- [ ] `userID` 只从 JWT 上下文取；service 里**只有一处 4043 出口**，无存在性探针
- [ ] PR 已开、至少 1 人 Approve；commit 符合 `<type>(<scope>): <subject>`

---

## 5. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-13 | v1-v4 | 创建 → 并入 4 项决策（`PageResult` 位置、ID 统一 `uint64`、同事务播种、`:id` 未命中统一 `4043`）→ 套用队长通用错误码规则 → 模型层落地（自写 `model.JSONB`、切断外键 `bigserial` 污染） | 设计与实现推进 |
| 2026-09-20 | v5 | **按 AGENTS §5.2 引用优先精简**：删除从 TECH_DESIGN / API_CONTRACT 复制粘贴的 DDL 原文、字段对照表、通用约定表与红线自查表，改为链接；只保留本功能独有的陷阱、决策与验收项（500 → 191 行，行数按 `wc -l` 含空行）。**同时修正一处文档损坏**：§2.1 与验收项里的 `4040` 曾被误替换成 `4043`，产生"用 `4043` 表示不存在、`4043` 表示不属于你"这类自相矛盾的表述——已按 `pkg/errcode` 与契约 §2 恢复为 **`4040` = 资源不存在 / `4043` = 资源越权** | 文档膨胀 + 恢复被误改的语义 |

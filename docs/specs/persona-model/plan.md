# plan · 人设 CRUD（Persona CRUD）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-persona-model`
> 负责人：成员 3 ｜ 创建：2026-09-13 ｜ 最后更新：2026-09-20
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：步骤产物与代码写法以 [spec §1-§3](spec.md) 为准，本文件**不重复**；
> 这里只写**计划独有**的内容——步骤顺序、审查方法、打回标准、进度记录。

---

## 1. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 状态 |
|---|---|---|---|---|
| 0 | 通用分页结构 | `internal/dto/common_dto.go` | `PageResult[T]` 只有这一份定义 | ✅ 已合并 |
| 1 | **翻译 Struct** | `internal/model/persona.go` | 8 列 + 外键关联全被声明；字段对照契约 §4 逐行对齐 | ✅ 已合并 |
| 2 | 主动消息配置模型 | `internal/model/proactive_setting.go` | 对 `Persona` 的外键带 `ON DELETE CASCADE` | ✅ 已合并 |
| 3 | 建表 | `internal/model/migrate.go` | `AutoMigrate` 追加两行，顺序在 `&User{}` 之后；`\d personas` 有 CASCADE 外键 | ✅ 已合并 |
| 4 | 请求/响应结构 | `internal/dto/persona_dto.go` | camelCase 对齐契约 §4；`familiarity` 拍平逻辑在此 | ✅ 已合并 |
| 5 | **仓储层** | `internal/repository/persona_repo.go`、`proactive_repo.go` | 方法签名全部强制带 `userID uint64` 与事务句柄（§2.3）；**不存在**"按 id 单查"或"判断存在性"的方法 | ✅ 已合并 |
| 6 | **业务层** | `internal/service/persona_service.go` | 未命中一律 `4043`；**创建走事务**（§3）；不依赖 `*gin.Context` | ✅ 已合并 |
| 7 | **HTTP 层** | `internal/handler/persona_handler.go` | 薄；含 `RegisterPersonaRoutes`；错误 `_ = c.Error(err)` 上抛 | ✅ 已合并 |
| 8 | 路由挂载 | `router.go` | 只加一行，**不与成员 1 同时改** | ✅ 已合并（PR #42） |
| 9 | 越权 / 级联 / 事务专项验证 | 本文件 §3.4 的命令 | spec §4 的验收项逐条打勾 | ✅ 随 PR #42 合并 |
| 10 | 规则广播 + 全局文件同步 | 群里的话 | **全局文件（`API_CONTRACT.md` / `AGENTS.md`）由队长 / 成员 1 改，本分支不碰** | 见 §5 风险 |
| 11 | 人工审查 + PR | `docs/dev_notes/persona_model_notes.md`、PR | §3 的审查清单全过；至少 1 人 Approve | ✅ 已完成 |

**关键路径**：步骤 0-5 不等任何人（公共层未交付时即可开工）；步骤 6-8 依赖成员 1 的 `pkg/response` / `pkg/errcode` / `middleware`——公共层落地前**不要自己造一份**（不 Own 的东西不要动，造了必冲突），等待期用 §3.4 的 SQL 直接对 PG 验证仓储层。

## 2. 文件清单

| 路径 | 作用 | 谁还会用到 |
|---|---|---|
| `internal/model/persona.go` | `personas` 的 GORM 实体 | **成员 1**（`message_repo` 引 `Persona`；`chat_messages.persona_id` 的归属校验挂在它上面） |
| `internal/model/proactive_setting.go` | `proactive_settings` 的 GORM 实体 | 成员 3（主动消息模块） |
| `internal/model/jsonb.go` | 自写 `JSONB`，**零新增依赖** | **成员 1**（`user_profile.profile_data` 直接复用，不要再引 `gorm.io/datatypes`） |
| `internal/dto/common_dto.go` | `PageResult[T]`，**五处分页共用** | **成员 1、成员 3**——**别再各写一份** |
| `internal/dto/persona_dto.go` | 请求 / 响应结构 | 成员 2（据此写 `types/persona.ts` 与 Mock，字段名逐字对齐） |
| `internal/repository/persona_repo.go` | 人设数据访问 | 成员 3（主动消息定时任务要读 `last_message_at`） |
| `internal/repository/proactive_repo.go` | 主动消息配置数据访问；**本功能只用其"播种默认行"** | 成员 3（在此文件继续加 `Get` / `Upsert`） |
| `internal/service/persona_service.go` | 归属校验与业务逻辑（含事务编排） | 成员 3（主动消息模块的 service 参照它的写法） |
| `internal/handler/persona_handler.go` | 4 个端点 + `RegisterPersonaRoutes` | 成员 1（挂载一行） |

## 3. 我手动审查 AI 代码的计划

> 前提：AI 生成的代码**默认不可信**，尤其是「越权防线」「`state` 保留」「事务是否真回滚」这三处——它们写错了照样能跑通、照样能演示，只在特定输入下才暴露。审查的**目标不是"能跑"，是"我能逐行解释它为什么安全"**。

### 3.1 审查方法（三遍）

| 遍 | 做什么 | 判据 |
|---|---|---|
| **第一遍 · 逐行重写注释** | 把 AI 产出的每个文件抄进 `docs/dev_notes/persona_model_notes.md`，**用自己的话**给每一行加注释 | **写不出注释的那一行，就是我没看懂的地方** → 那就是审查重点，去查文档或问，不放过 |
| **第二遍 · 对照核对** | 拿 DDL + 契约 §4 + spec 全文逐字段/逐端点核对，填成表格 | 每个 JSON 字段名都能在契约里指到来源；**找不到来源的字段一律删掉** |
| **第三遍 · 对抗验证** | 主动构造能打破代码的输入（跨账号、空列表、不存在的 id、无 `familiarity` 的 `state`、重复删除、超长字段、**让播种插入失败**），按 §3.4 实跑 | 每个高危点都有一次实跑记录，不是"读代码觉得没问题" |

**第三遍是核心**：前两遍只能证明"代码符合我的预期"，第三遍才能证明"代码在我不期望的输入下不崩、不泄漏、不回滚错"。AI 写的代码在前两遍往往很干净。

### 3.2 六个高危点

| # | 高危点 | 为什么会错 | 怎么验（对应 spec §4 验收组） |
|---|---|---|---|
| 1 | **资源越权（`4043`）** | "先查出来再在 Go 里比"的写法看起来逻辑完整，漏掉 `if` 在 review 中极难发现；写了 `ExistsByID` 就白增泄漏面；照旧契约把 `4030` 写回来等于没落地这条规则 | 用**另一个账号的 Token** 打 `PUT` / `DELETE`，必须 `4043`；再用不存在的 id 打，必须拿到**同码同文案**的 `4043`；grep 确认无裸 id 查询、无探针、**无 `4030` / `4040`** |
| 2 | **`state` 被覆盖** | `Save()` 或 `Updates` 传全字段 struct 都会静默清空 JSONB，**不可逆** | 手工改成 `{"familiarity":42,"self_note":"x"}` → `PUT` → 两个键都还在、`familiarity` 仍 42 |
| 3 | **`NULLS LAST` 漏写** | Postgres 的 `DESC` **默认是 `NULLS FIRST`**，排序语义与需求正好相反，**但页面看着"正常"** | 新建一个从未聊过的人设，确认它排在列表**最后** |
| 4 | **级联删除没建出来** | 只写标量 `UserID` 时 GORM 不建外键，删人设会**留下孤儿消息与孤儿配置行** | `\d personas` / `\d proactive_settings` 看 FK 是否带 `ON DELETE CASCADE`；删人设后数两张子表 |
| 5 | **错误码硬编码 / 用错码** | 容易直接写 `c.JSON(404, ...)` 或中文文案字面量；也容易**照着契约旧文把 `4030` 写回来** | grep 错误码数字与中文文案，必须全部指向 `errcode.*` 常量；人设模块**不应出现 `4030` / `4040`**——越权与不存在都只有一个答案 `4043` |
| 6 | **事务没真正回滚** | 事务内用包级 `db` 而非 `tx`，或先提交人设再插配置——**代码看上去有 `Transaction`，实际没生效** | §3.4 的"播种失败"实验：让播种插入报错，确认 `personas` **不留新行** |

### 3.3 逐文件审查清单

| 文件 | 盯的点 | 看懂的标准 |
|---|---|---|
| `dto/common_dto.go` | `PageResult[T]` 是否**只有这一份**；字段是否 `list/total/page/pageSize` | `grep -rn "type PageResult" internal/` 只有 1 处命中 |
| `model/persona.go` | `user_id` 是否 `json:"-"`；`state` 是否原始 JSON 类型；`LastMessageAt` 是否指针；`User` 关联是否在且有 `OnDelete:CASCADE` | 我能说出每个 tag 去掉之后会发生什么 |
| `model/proactive_setting.go` | 对 `Persona` 的外键是否 `ON DELETE CASCADE`；四个默认值是否与 DDL 一致 | `\d proactive_settings` 能看到 FK 与默认值 |
| `dto/persona_dto.go` | 有没有偷偷声明 `userId` / `state` / `familiarity`；camelCase 是否逐字对齐契约 | 与契约 §4 的 JSON 示例并排比对，一个字母都不差 |
| `repository/persona_repo.go` | **每个方法签名是否都带 `userID`**；有没有 `db.First(&p, id)` 形态的裸查询；有没有 `ExistsByID`；更新是否用了 `Save(` | `grep -nE "Save\(|First\(&|ExistsBy" persona_repo.go` 无业务命中 |
| `repository/proactive_repo.go` | 播种函数是否**接受 `tx *gorm.DB`** 而不是自取包级 `db` | 参数表里有 `tx`；函数体内是 `tx.WithContext(ctx)` |
| `service/persona_service.go` | 未命中是否统一 `4043`；事务是否用 `db.Transaction`；是否引了 `gin`；是否出现裸数字错误码 | `grep -n "gin" persona_service.go` 无结果；`grep -nE "\b(400[0-9]\|403[0-9]\|404[0-9])\b"` 无结果 |
| `handler/persona_handler.go` | 是否 `_ = c.Error(err)` 上抛；有没有 `response.Fail`；`userID` 是否只从 JWT 上下文取 | `grep -n "response.Fail" persona_handler.go` 无结果 |
| `router.go` | 只加了一行，没动别人的行 | `git diff router.go` 只有 1 行新增 |

### 3.4 必须亲眼看到的证据（不接受"我觉得没问题"）

```bash
# 0) 编译 + 静态检查（提交前必跑）
cd backend && go build ./... && go vet ./... && go test ./...

# 1) 外键与索引真的建出来了吗（命令里不写密码，红线 1）
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c '\d personas'
docker compose -f deploy/docker-compose.dev.yml exec postgres \
  psql -U heart_echo -d heart_echo -c '\d proactive_settings'
#    要看到：FOREIGN KEY ... ON DELETE CASCADE（两张表各有）、idx_personas_user_last_msg
#    且外键列【没有】nextval 默认值（spec §1.3 的关联复制坑）

# 2) 越权与不存在：两者必须是同一个 4043
curl -s -X DELETE "localhost:8080/api/v1/personas/1"      -H "Authorization: Bearer $TOKEN_B"
curl -s -X DELETE "localhost:8080/api/v1/personas/999999" -H "Authorization: Bearer $TOKEN"
#    两条都该是 code=4043、message 逐字相同 —— 外部无法区分；全程不应出现 4030 / 4040

# 3) 排序：从未聊过的人设必须排在最后
curl -s "localhost:8080/api/v1/personas?page=1&pageSize=20" -H "Authorization: Bearer $TOKEN"

# 4) 播种与级联
curl -s -X POST localhost:8080/api/v1/personas -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"小暖","personalityDesc":"温柔","speakingStyle":"轻柔"}'
#    查：proactive_settings 恰好 1 行、t | 30 | 120 | 3 | NULL；删人设后两张子表计数为 0

# 5) 事务回滚实验（用临时 _test.go，不进 PR）
#    把 CreateDefaultSettings 收到的 personaID 改成不存在的值（触发外键违例），
#    期望：整个事务回滚 —— personas 表里没有刚插入的那一行。留行不留配置 = 事务没生效，打回。

# 6) code 层面 grep（都要无输出；⚠️ 期望 0 的 grep 不要用 && 串联，无匹配时退出码为 1 会断链）
grep -rn "response.Fail" internal/handler/persona_handler.go
grep -rn "Save("         internal/repository/persona_repo.go
grep -rnE "\b(4030|4040)\b" internal/service internal/repository internal/handler
grep -rn "type PageResult" internal/dto/     # 期望只有 1 行

# 7) 红线 1 自查：不许有密钥进入本次改动
git status --short && git diff --cached --name-only | grep -E '\.env$|\.key$|\.pem$'
```

> `$TOKEN` / `$TOKEN_B` 从登录接口拿，**不要写死在脚本或 `_test.go` 里**（红线 1）。测试需要真实数据时用环境变量或本地 `.env`。

### 3.5 拒绝标准（出现任一条就打回重写，不做"小修小补"）

| 打回条件 | 为什么不能只小修 |
|---|---|
| 仓储层存在**不带 `user_id`** 的业务查询 | 防线是结构性的，逐处打补丁会漏；必须让不安全的方法不存在 |
| 出现 `ExistsByID` / 按 id 单查的探针 | "存在性"一进入代码就有了泄漏面，且与"不泄漏存在性"的决定直接矛盾 |
| 未命中返回 `4030` / `4040` | 违反通用规则（**资源越权 → `4043`**）；`4030` 是功能越权的码、本模块无此场景；`4040` 与 `4043` 并存**会泄漏存在性** |
| **创建人设不是单事务**（用了包级 `db`、或分两次提交） | 会留下孤儿配置行；且这类 bug 在演示时表现为"某个伴侣设置页打不开" |
| 更新用了 `Save()` 或全字段 `Updates(struct)` | 覆盖 `state` 是数据损坏，且**不可逆**；必须改成指定列 |
| 未命中时返回成功（`200`） | 越权写入被伪装成成功，比报错危险 |
| 出现裸数字错误码 / 中文错误文案字面量 | 破坏"一 code 一 msg"（红线 6），一处放纵会蔓延 |
| service 里出现 `*gin.Context` | 破坏分层（红线 7），后续无法单测 |
| 外键未带 `ON DELETE CASCADE`（两张表都要查） | 孤儿数据会污染记忆/画像/朋友圈/主动消息，且事后清理成本高 |
| `PageResult` 出现第二份定义 | 两份同名类型在联调时会出现"字段对不上"的诡异问题，最难排查 |

**审查通过的唯一标准**：§3.4 的命令全部实跑过，且 §3.3 每个文件我都能逐行解释。**审查笔记落到 `docs/dev_notes/persona_model_notes.md`**，作为"我确实看懂了"的证据留档。

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| **全局文件未同步就合并** | 成员 2 的 Mock 按旧的 `4030`/`4043` 写，联调时错误分支全对不上；后续 AI 代理照 `AGENTS.md` §4.3 把 `4030` 写回来 | 步骤 10：**先广播规则本身**（不只是广播"我改了人设"），再等队长 / 成员 1 更新全局文件；**本分支不碰那两个文件** |
| 通用规则只改了人设模块，契约里另 10 个归属校验端点仍带 `4030` | 长期"半套规则"共存：同一个 `personaId` 归属失败，在不同端点返回不同码，前端与测试都难以统一 | 广播时把**逐行清单一起交出去**（含契约行号与改法），由队长排一次统一整改；本 PR 只做 §4 |
| `state` 的写入方（对话链路）未定 | 本功能写 `{"familiarity":0}`，对话链路要 `+1`，两边格式不一致会互相踩 | 本功能只初始化、不累加；`state` 的 schema 约定写进 spec §1.1，交付时同步给成员 1 |
| 播种的默认值与主动消息模块不一致 | 两处各写一套默认值，用户改过配置后被覆盖 | 默认值只在 DDL 与 `CreateDefaultSettings` 一处定义；主动消息模块的 `GET` 直接读表，不写第二套兜底默认 |
| `proactive_repo.go` 后续被主动消息模块大改 | 函数签名变了，本功能的调用点要跟着改 | 播种函数签名保持 `(ctx, tx, userID, personaID)`，不要加可选参数 |
| 外键列被关联复制污染成 `bigserial` | 每张子表的 `user_id` 都会长出 `nextval` 默认值，漏传 `user_id` 的 INSERT 会静默拿到别人的 id | **已在源头切断**（`user.go` / `persona.go` 的 `ID` 改为 `type:bigint`）。新增任何带外键的模型时，**被引用方的 `ID` 都不要写 `bigserial`**（spec §1.3） |
| 契约文档 `state: {}` 与 `familiarity: 12` 的不一致 | 成员 2 写 Mock 时可能理解成独立列 | 实现后主动告知（spec §1.1 注） |

## 5. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-13 | spec / plan 落地；并入 4 项决策（`PageResult` 位置、ID 统一 `uint64`、创建时同事务播种、`:id` 未命中统一 `4043`）；套用队长「通用错误码规则」（资源越权 → `4043` / 功能越权 → `4030`） | 成员 1 的公共层尚未进仓库；契约变更待广播 |
| 2026-09-13 | **步骤 0-3 完成**：`jsonb.go`、`persona.go`、`migrate.go`、`cmd/migrate/main.go` 落地。**顺带修掉两个实测缺陷**：外键被关联复制污染成 `bigserial`、自写 `JSONB` 漏 `MarshalJSON` 会让 `state` 变成 base64。已在全新空库上验证 schema 与 DDL 一致（含 CASCADE 行为、序列清单、幂等） | 变量名 `SCHEMA_CHECK_DSN` 与技术文档 §4.6 规划的 `DB_*` 不是同一套，**待队长拍板是否统一** |
| 2026-09-13 | 步骤 4-11 完成，PR #42 合并（含路由挂载与 `JWTAuth` 之后的路由断言） | 无 |
| 2026-09-20 | **按 AGENTS §5.2 引用优先精简**：删除与 spec 重复的实现要点、复制粘贴的通用约定与红线自查表（326 → 163 行，行数按 `wc -l` 含空行）；步骤状态按实际落地情况回填 | 无 |

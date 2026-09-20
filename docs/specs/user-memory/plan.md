# plan · 用户记忆（User Memory）

> 对应 spec：[spec.md](spec.md) ｜ 分支：`feature/backend-user-memory-model` ｜ 负责人：成员 3 ｜ 创建：2026-09-15
>
> **引用优先**（[AGENTS §5.2](../../../AGENTS.md)）：接口层设计（字段、签名、错误码、判定顺序）与验收项**不在此重复**，一律见 [spec.md](spec.md) §2 / §3 / §4。本文件只写**怎么做**与**怎么审**。
>
> ✅ **本分支只做模型层**：`internal/model/user_memory.go` + `migrate.go` 追加一行 + spec/plan。**`dto/` / `repository/` / `service/` / `handler/` / `router.go` 一律不动**。
> ⚠️ **先读 spec §2**：本功能最大的风险是**一行 `WHERE`**（`persona_id` + `user_id` 两个条件）与**一次归属判定**。两处写错都能编译、能跑、能演示，只在"换个账号 / 换个心态"时才暴露——§3 的审查计划不是形式，是本 plan 的核心。
> ✅ **外键行为现在就能验**：`AutoMigrate` 建完表后，级联 / `SET NULL` / 默认值都能用**纯 SQL** 验证（`psql` 里 INSERT + DELETE），**不需要任何 Go 业务代码，也不需要接口层**——这正是 A 组验收的主体。

---

## 1. 步骤拆解

### 1.1 本分支（模型层，不等任何人）

| # | 步骤 | 产物 | 完成标准 | 前置 | 状态 |
|---|---|---|---|---|---|
| 1 | **翻译 Struct** | `internal/model/user_memory.go` | 10 列 + **3 个外键关联**（2×CASCADE + 1×**SET NULL**）+ `MemoryType` 常量 + **`TableName()` 返回 `user_memory`**（这里**必需**，GORM 默认会复数成 `user_memories`） | 无 | ✅ **已完成** |
| 2 | 建表 | `internal/model/migrate.go` | `AutoMigrate` 追加 `&UserMemory{}`，顺序在 `&ChatMessage{}` **之后** | 步骤 1 | ✅ **已完成**（`git diff` 恰好 +1 行） |
| 3 | **模型层实库验证** | 临时库上的实跑记录 | spec §4 **分组 A 全绿（17/17）** | 步骤 1、2 | ✅ **已完成** |
| 4 | **人工审查** | §3 的清单 + 证据 | §3.1 前两遍 + §3.3 的 A 组命令全过；§3.2 各高危点逐条自查 | 步骤 3 | ✅ **已完成**。提交与 PR 暂缓——用户指定代码留在工作区 |
| 5 | **与成员 1 对齐接口层分工（阻塞合并）** | 群里的话 | 三选一说定：他写 / 本分支接手 / 一起写。**没结论就不合并**（spec §5.1 #1） | 步骤 4 | 未开始 |
| 6 | **契约空白广播（阻塞合并）** | 群里的话 | spec §2.2 的 `4001` 已广播、由队长拍板；**`API_CONTRACT.md` 不由本分支改** | 步骤 4 | 未开始 |

### 1.2 交接给成员 1（本分支**不做**）

| # | 步骤 | 产物 | 完成标准 |
|---|---|---|---|
| H0 | 通用分页结构 | `internal/dto/common_dto.go` | `PageResult[T]` + 分页常量 + `ClampPage` **只有这一份**（**已存在**：三个分支谁先落地谁建，后来者直接消费） |
| H1 | 请求 / 响应结构 | `internal/dto/memory_dto.go` | `MemoryItemResponse`（**恰好 6 字段**）+ `MemoryListQuery`（`form:"personaId"`，spec §3.1） |
| H2 | 归属判定 | `internal/repository/persona_repo.go` 的 `ExistsOwnedByUser` | **一条查询、两个条件；只回 bool**（与 chat-message 分支共用，**别写两份**） |
| H3 | **仓储层** | `internal/repository/memory_repo.go` | `ListByPersona`（含 `total`）+ `CreateBatch`；签名强制带 `userID` **与** `personaID`；**不存在**单条件查询、不存在删除方法 |
| H4 | **业务层** | `internal/service/memory_service.go` | **先判归属、后查数据**；未命中一律 `4043`；`userID == 0` → `4010`；`personaID == 0` → `4001`；不依赖 `*gin.Context` |
| H5 | **HTTP 层** | `internal/handler/memory_handler.go` | 薄；含 `RegisterMemoryRoutes`；错误 `_ = c.Error(err)` 上抛 |
| H6 | 路由挂载 | `router.go` | 加一行，**不要与成员 1 同时改** |
| H7 | 接口层验证 | — | spec §4 **分组 B**（✅ `JWTAuth` 已落地，无阻塞） |

> ✅ **前情澄清**：本 plan 早期版本称"接口层验收阻塞在成员 1 的 `middleware`（`JWTAuth` 是空壳）"。**已核实为过期信息**——`internal/middleware/jwt.go` 现在确实 `c.Set(ContextKeyUserID, claims.UserID)`，接口层验收（H7）**不再有前置阻塞**。

## 2. 文件清单

**A. 本分支产出**

| 路径 | 作用 | 谁还会用到 |
|---|---|---|
| `backend/internal/model/user_memory.go` | `user_memory` 的 GORM 实体（含 3 个外键，其中 1 个 SET NULL） | **成员 1**（提取链路落库） |
| `backend/internal/model/migrate.go` | 建表入口（追加一行） | 成员 1（`main.go` 调用） |
| `docs/specs/user-memory/spec.md` / `plan.md` | 本功能的规格与计划 | **成员 1**（接口层的规格）、成员 2（响应结构） |
| `docs/dev_notes/user_memory_notes.md` | **我的审查笔记**，非交付物 | 只有我。⚠️ **`docs/dev_notes/` 被 git 忽略**（`.git/info/exclude`），**不进 PR、不入库**——想进 PR 的内容必须写进 `spec.md` / `plan.md` |

**B. 交接给成员 1（本分支不动）**

| 路径 | 别踩的坑 |
|---|---|
| `backend/internal/dto/common_dto.go` | 五处分页共用，**谁先落地谁建**，不重建 |
| `backend/internal/dto/memory_dto.go` | `form:"personaId"`（spec §3.1 的 ⚠️） |
| `backend/internal/repository/persona_repo.go` | 增 `ExistsOwnedByUser`；**三个分支都会改这个文件** |
| `backend/internal/repository/memory_repo.go` | 唯一写入入口；**不要有 `ListByUser`** |
| `backend/internal/service/memory_service.go` | 顺序：先判归属后查数据 |
| `backend/internal/handler/memory_handler.go` | 1 个端点 + `RegisterMemoryRoutes` |

---

## 3. 我手动审查 AI 代码的计划

> 前提：AI 生成的代码**默认不可信**。本分支的产物只有两个文件，但它们是**下游一切的基座**——`AutoMigrate` 建出来的表长什么样，直接决定接口层能不能"改对了"，也决定阶段二接 ChromaDB 时要不要改表。审查的**目标不是"能编译"，是"我能逐行解释 `\d user_memory` 为什么长这样"**。

### 3.1 审查方法（两遍 + 实跑）

| 遍 | 做什么 | 判据 |
|---|---|---|
| **第一遍 · 逐行重写注释** | 每个字段一行注释，用我自己的话写（**落在要入库的 `user_memory.go` 里**，不是 `dev_notes`——审查者在 PR 里要直接看得到） | **写不出注释的那一行，就是我没看懂的地方** → 查文档或问，不放过 |
| **第二遍 · 对照 DDL 核对** | 逐列核对类型 / 可空性 / 默认值 / `ON DELETE` / 索引列与顺序 | 每一列都能指到 DDL 来源；**DDL 里没有的东西一律不加**（`memory_type` 的 CHECK 就是这么被排除的） |
| **第三遍 · 实库对抗验证** | `AutoMigrate` 到**干净库**，按 §3.3 逐条实跑 | 每条约束都有一次实跑记录，不是"读代码觉得没问题" |

**第三遍是本段的核心，而且现在就能做全**：外键行为是**数据库层**的事，不需要接口层、也不需要 `JWTAuth`。另两遍只能证明"代码符合我的预期"，第三遍才能证明"`\d` 出来的东西真的是 DDL 描述的那个表"。

### 3.2 高危点专项审查

| # | 高危点 | 为什么会错 | 我怎么验（命令见 §3.3） |
|---|---|---|---|
| 1 | **`SourceMessage` 不是 `SET NULL`**（写成 CASCADE，或整行漏掉） | 漏掉 → GORM **一个外键都不建**；写成 CASCADE → **删一条消息顺手删掉一条长期记忆**，属数据损坏。**这是本表与另两张表唯一不同的地方，也是 AI 最容易照抄上一份文件写错的一处** | `\d` 看第三个 FK 是否 `ON DELETE SET NULL`；**再实删一条消息**，确认记忆还在且 `source_message_id IS NULL` |
| 2 | **`SourceMessageID` 用了值类型** | `SET NULL` 要求列可空；值类型会建成 `NOT NULL`，**删消息时直接报错**——但这只在"真的删了一次消息"时才暴露 | `\d` 看 `source_message_id` 是否可空；A6 实删一次 |
| 3 | **`ID` 写成 `bigserial`** | GORM 把被引用主键的 `DataType` 复制到外键列，让下游列长出 `nextval` 默认值。**已踩三次，却是最容易"顺手写回去"的一处** | 锚定 grep = **0**；`pg_sequences` 里**不存在** `user_memory_*_id_seq`（期望恰好 4 条序列） |
| 4 | **`memory_type` 被加了 `check:` tag** | 「别的表有 CHECK，这里也加一个」是很自然的联想，但这会让 **struct 与权威 DDL 不一致**，且 `AutoMigrate` 会**真的把约束建出来** | 锚定 grep = 0；`\d` 里**不应**出现 `chk_user_memory_memory_type` |
| 5 | **`json:"-"` 少写 / 多写** | 少写 → `embeddingStatus` / `sourceMessageId` / `userId` 漏进响应体（内部状态泄漏 + 契约漂移）；多写 → 该给的字段没给 | 锚定 grep = **7**（4 数据列 + 3 关联字段）；抽对外 JSON 名 = **6**（三步并 `grep -v`，见 §3.3 陷阱 ⑤） |
| 6 | **`MemoryType` 用了裸 `string`** | 拼错 `"preference"` 编译期发现不了，落成脏数据，前端三分类渲染掉进"未知"分支 | 锚定字段声明行 = **1**（⚠️ **不能** `grep "MemoryType string"`——会命中类型声明本身） |
| 7 | **`importance_score` 的类型 / 编解码** | pgx 往 `numeric` 编 `float64` 若类型不匹配，落库直接失败（该列 NOT NULL，必写） | A5：插 `0.850` 读回 `0.850`；报错再退 `driver.Valuer`，**不要引入 `decimal`** |
| 8 | **`migrate.go` 动了别人的行** | 多人共用一个文件，顺手"整理"就会与别的分支冲突 | `git diff migrate.go` **只有 1 行新增** |
| 9 | **关联 tag 被截断 / 结构体没闭合**（实际发生过） | ① 结构体闭合 `}` 丢失 → CI 能报出来；② `SourceMessage` 的 tag 丢 `foreignKey` / `references` → **`go build` / `vet` / `gofmt` 全部照过**，但 GORM 会退回按约定推断关联，这条外键可能建错或建不出来——而它恰恰是本表唯一与另两张表不同的地方 | `gofmt -e` 退出码 = 0；tag 三 key 齐全的关联 = **3**（已实测：修复版 3 / 损坏版 2，**能报出缺陷**）；最终靠 `\d` 的 3 个 FK 兜底 |

**B 段高危点（接口层，必须用**另一个账号**实跑，不能只读代码）：**

| # | 高危点 | 为什么会错 | 我怎么验 |
|---|---|---|---|
| B1 | **跨用户泄漏**（只带 `persona_id`） | `persona_id` 全局自增，只按它查就是别人的记忆。AI 常把 `user_id` 当"可选优化"省掉 | 用 **B 的 Token** 打 A 的 `personaId` → 必须 `4043`；再直接用 SQL 跑**去掉 `user_id`** 的那条查询，亲眼看它**返回了别人的行**（证明这个条件必需，不是装饰） |
| B2 | **跨人设串号**（只带 `user_id`） | 把该用户所有人设的记忆混在一起。**响应看起来完全正常**（有数据、有条数、能翻页），只有对着"切人设问同一个问题"才暴露——**最隐蔽的一条** | 同一账号建**两个**人设、各写不同记忆 → 分别查，`list` 与 `total` 都必须只含自己那份；再 SQL 跑**去掉 `persona_id`** 的查询，看它把两个人设的记忆一起返回 |
| B3 | **空态与 `4043` 的分界** | 用 `len(list) == 0` 反推归属，会把"还没记住任何事"误报成 `4043` | 建一个**从未聊过**的人设，查它的记忆 → 必须 `200` + `list: []` |

### 3.3 必须亲眼看到的证据（A 段 · 现在就能全跑）

```bash
# ---- 准备：临时库。凭据从 deploy/.env 读，不写进代码 / 不写进文档（红线 1）----
# 注：cmd/migrate 读的是 SCHEMA_CHECK_DSN（裸 os.Getenv），与 backend/.env.example 的 DB_* 不是同一套
set -a; . deploy/.env; set +a
docker compose -f deploy/docker-compose.dev.yml exec -T postgres \
  sh -c 'createdb -U "$POSTGRES_USER" user_memory_schemacheck'
export SCHEMA_CHECK_DSN="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5432/user_memory_schemacheck?sslmode=disable"

# A0) 编译 + 静态检查
cd backend && go build ./... && go vet ./... && go test ./...

# A1/A8) 建表 + 幂等（打临时库，不动 heart_echo）
go run ./cmd/migrate                       # 期望：AutoMigrate 完成；再跑一次须无报错、无副作用

# A2) 表结构 / 外键 / 索引 —— 本分支最重要的一眼
docker compose -f deploy/docker-compose.dev.yml exec -T postgres \
  sh -c 'psql -U "$POSTGRES_USER" -d user_memory_schemacheck -c "\d user_memory"'
#    要看到：3 个 FK（fk_user_memory_user / _persona = CASCADE；_source_message = SET NULL ← 关键）
#            idx_memory_persona_type btree (persona_id, memory_type) ← 必须【两列】，顺序不能反
#            idx_memory_user_id / idx_memory_embedding_status
#            embedding_id 可空、embedding_status DEFAULT 'pending'、source_message_id【可空】
#            id bigint DEFAULT nextval(...) ← 这就是【正确】结果（PG 从不显示 bigserial 这个字，见陷阱 ④）
#    并且【不应】出现 chk_user_memory_memory_type；`\dt` 里应只有 user_memory（不是 user_memories）

# A3) 序列清单：多一个都不行（bigserial 污染的反向验证）
docker compose -f deploy/docker-compose.dev.yml exec -T postgres sh -c \
 'psql -U "$POSTGRES_USER" -d user_memory_schemacheck -c \
  "SELECT sequencename FROM pg_sequences WHERE schemaname=(SELECT current_schema()) ORDER BY 1;"'
#    期望【恰好四条】：chat_messages_id_seq / personas_id_seq / user_memory_id_seq / users_id_seq
#    出现 user_memory_{user_id,persona_id,source_message_id}_seq = 关联复制污染，打回

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

# A9) code 层面 grep —— ⚠️ 每个都必须【锚定 tag】：注释里故意写着 bigserial / json:"-" / emotion
#     来解释"为什么不能这么写"，裸 grep 单词会把这些【正确】的解释误判成违规（实测全误报）
#     ⛔ 不要用 && 串成一条链：期望 0 的检查匹配 0 行时退出码是 1，链条会断，后面几条静默不跑
grep -cE 'type:bigserial'                internal/model/user_memory.go   # 期望 0
grep -cE 'gorm:"[^"]*check:'              internal/model/user_memory.go   # 期望 0
grep -cE 'json:"-.{0,2}$'                 internal/model/user_memory.go   # 期望 7（4 数据列 + 3 关联字段）
grep -cE '^\s*MemoryType +MemoryType +`'  internal/model/user_memory.go   # 期望 1（不能 grep "MemoryType string"）
grep -c  'gorm:"column:'                  internal/model/user_memory.go   # 期望 10
# 三个关联 tag 的三个 key 必须齐全（少一个是静默失败：build/vet/gofmt 全部照过）
grep -cE 'gorm:"foreignKey:[^"]*;references:[^"]*;constraint:OnDelete:' internal/model/user_memory.go  # 期望 3
#    只跑前两步会输出 10 行（10 个数据列里 4 个是 json:"-"），看到 10 不等于失败 —— 三步都要写：
grep 'gorm:"column:' internal/model/user_memory.go | grep -oE 'json:"[^"]*"' | grep -v 'json:"-"' | wc -l  # 期望 6
git diff internal/model/migrate.go                                        # 只有 1 行新增

# A10) 红线 1 自查 + 收尾
git status --short | grep -E '\.env$|\.key$|\.pem$'      # 期望无输出
docker compose -f deploy/docker-compose.dev.yml exec -T postgres \
  sh -c 'dropdb -U "$POSTGRES_USER" user_memory_schemacheck'   # 不留临时库，不碰 heart_echo
```

> **实测记录**：A0-A10 已完整跑过一遍，spec §4 分组 A **17/17 全绿**：`\d` 的 10 列 / 3 FK / 3 索引逐条正确；序列恰好 4 条（无污染）；A4 级联删除后残留 0；A5 读回 `0.850` + `pending`；A6 删消息后**记忆行仍在**且溯源断开；A7 零值关联字段**未**连带写库（users=1 / personas=1 / chat_messages=0）；A8 幂等，fk数=3 / 索引数=4 / 列数=10 与首次一致。
> **`-T` 不能省**：不加时 `psql` 输出可能混入 TTY 控制字符，`\d` 的表格会很难读。
> **为什么用临时库**：A4-A6 会往库里插测试数据。留在开发库 `heart_echo` 会混进演示数据（总纲 §6 的演示脚本本来就要灌记忆）。临时库跑完直接 `dropdb`，**开发库一行都没碰**。

> ⚠️ **同一轮实测出的六类"假失败"**（也是这轮修文档的原因）——**全部由"照着自己写的文档实跑一遍"暴露**：
>
> | # | 陷阱 | 实跑值 vs 期望 |
> |---|---|---|
> | ① | 裸 grep 会把注释里的解释当成违规（`grep -c "bigserial"`=**2**、`grep -c 'json:"-'`=**10**、`grep -ci "emotion"`=**5**，全来自"**不要**写成 bigserial"这类**正确**的说明） | 全部改锚定 tag |
> | ② | `json:"-"` 按 4 个数据列数过，漏了 3 个关联字段 | **7**（不是 4） |
> | ③ | `grep "MemoryType string"` 命中 `type MemoryType string` 这行**类型声明本身** | 改锚定字段声明行 |
> | ④ | `\d` 显示 `id bigint + nextval` 被误判成"写成了 bigint 不是 bigserial" | 这是**正确**结果（PG 不显示 `bigserial` 这个字）；失败信号是**别的列**长出 `nextval` |
> | ⑤ | "抽对外 JSON 名 = 6"只给结论没给完整命令；只跑前两步输出 **10 行** | 必须三步并 `grep -v 'json:"-"'` |
> | ⑥ | 期望 0 的 grep 用 `&&` 串联时，**第二条就断链**（无匹配 → 退出码 1），后四条静默不跑 | 逐条跑，不串联 |

### 3.4 拒绝标准（出现任一条就打回重写，不做"小修小补"）

**A 段（本分支，现在就适用）：**

| 打回条件 | 为什么不能只小修 |
|---|---|
| `SourceMessageID` 用值类型，或关联写成 `OnDelete:CASCADE` / 整行漏掉 | 值类型 → `SET NULL` 建不出来、删消息直接失败；CASCADE → **删一条消息顺手删掉一条长期记忆**；漏掉 → 三个外键一个都不建 |
| `ID` 出现 `bigserial` | 静默污染下游外键列的默认值，**已踩三次** |
| `memory_type` 加了 `check:` tag | struct 与权威 DDL 不一致，且 `AutoMigrate` 会真的把约束建出来 |
| `json:"-"` 少了任意一个（应 7 处） | 内部状态泄漏 + 契约漂移；前端会照着 Mock 把它们渲染出来 |
| `MemoryType` 用裸 `string` | 编译期防线消失，脏数据落库 |
| `migrate.go` 动了别人的行 | 多人共用一个文件，冲突成本高、review 范围不清 |
| 出现裸数字错误码 / 中文错误文案字面量 | 破坏"一 code 一 msg"（红线 6），一处放纵会蔓延 |

**B 段（接口层，成员 1 开工时适用）：**

| 打回条件 | 为什么不能只小修 |
|---|---|
| 仓储层存在**不带 `user_id`** 或**不带 `persona_id`** 的业务查询 | 防线是结构性的，逐处打补丁会漏；**尤其 `ListByUser` 这种方法名本身就是漏洞的形状** |
| `Count` 与列表查询的条件**不一致** | `total` 会算进别人的记忆（或漏算自己的），且**只看响应体几乎发现不了** |
| 用 `len(list) == 0` 判越权 / 不存在 | 把"还没有记忆"误报成 `4043`，新伴侣的记忆页打不开 |
| 未命中返回 `4030` / `4040` | 违反队长规则（**资源越权 → `4043`**）；`4030` 是功能越权的码、本模块无此场景，`4040` 与 `4043` 并存**会泄漏存在性** |
| 缺 `personaId` 时返回 `200` + 空列表 | 静默降级：把调用方的 bug 伪装成正常空态 |
| 响应体出现 `userId` / `embeddingId` / `embeddingStatus` / `sourceMessageId` | 契约漂移 + 内部状态泄漏 |
| 出现 `Delete*` / `Update*` 记忆的方法或端点 | 违反总纲 §0「记忆只读」；删除路径还要再防一次越权，白增泄漏面 |
| `memoryType` 里出现 `emotion` | 违反"情绪是内部信号"（AGENTS §4.4 / 总纲 §0） |
| 排序只写 `created_at DESC`（无 `id` 兜底） | 批量写入的同批记忆时间戳相同，翻页重复 / 漏项 |
| service 里出现 `*gin.Context` | 破坏分层（红线 7），后续无法单测 |
| `CreateBatch` 用包级 `db` 而不是 `tx` | 事务不生效，半批落库；演示时表现为"AI 只记住了一部分" |
| `PageResult` 出现第二份定义 | 两份同名类型在联调时会出现"字段对不上"的诡异问题，最难排查 |

**审查通过的唯一标准**：§3.3 的 A 组命令全部实跑过，且 §3.2 的九个高危点我都能逐条解释清楚。注释落在要入库的 `user_memory.go` 里，验证证据落 `docs/dev_notes/user_memory_notes.md` 并在 §5 留原始输出。

## 4. 风险与对策

| 风险 | 影响 | 对策 |
|---|---|---|
| **接口层分工未定**（spec §5.1 #1） | 两人同时写 `memory_repo.go` / `memory_service.go` / `memory_handler.go`（AGENTS §4.8 明令避免） | 步骤 5：**合并前必须有结论**；本分支先只交模型层，**不碰那些文件** |
| ~~接口层端到端验收做不了~~ | ~~spec §4 分组 B 无法执行~~ | ✅ **已消除**：`middleware/jwt.go` 的 `c.Set(ContextKeyUserID, claims.UserID)` 已落地（早前版本记为"空壳"，属过期信息）。**`JWTAuth` 是成员 1 的文件，不要自己实现** |
| **三个分支都会改 `persona_repo.go` 与 `common_dto.go`** | 合并冲突，或各自写一份同名函数 | 群里确认**谁先合谁建**；用到时先 `grep -rn "type PageResult" internal/`，有就直接消费，**不重建** |
| 契约 §7 的 `4001` 未广播就合并 | 前端按"只会收到 4043"写错误分支，缺参时会走到 `message` 兜底分支 | 步骤 6：**只广播、不改全局文件**；由队长拍板后交契约负责人统一改。**未同步不得合并** |
| ~~验证时把测试数据留在开发库~~ | ~~演示脚本要灌记忆数据，混着"测试记忆"会很难看~~ | ✅ **已消除**：全程在临时库验证，跑完 `dropdb`，**开发库一行未碰** |
| ~~`importance_score` 的 pgx 编码报类型不匹配~~ | ~~落库直接失败（NOT NULL 必写）~~ | ✅ **已排除**：实测 GORM `Create` 写 `0.85` 读回 `0.85`。**阶段一不需要** `driver.Valuer` 兜底（那条备选留着，别提前引入）。仍**不要引入 `decimal` 包** |
| 阶段二 ChromaDB 的 `metadata` 只存 `persona_id` | **跨用户串号**（技术文档 §6.3 点名的风险） | spec §2 已写明：`metadata` 存 `{user_id, persona_id, memory_type, importance_score}`，检索两个过滤条件都带 |
| 阶段二 LLM 返回非法 `memory_type` | 落库脏数据，前端三分类渲染掉进"未知"分支 | spec §1.2 / §3.3 已写明：**落库前校验 / 归一，非法值丢弃并记日志**；属提取链路（成员 1），交付时口头确认一句 |
| 写入编排的归属两处不一致（技术文档 §9 有 `memory_service.go`，成员 1 任务书没有） | 落地时互相等 / 都写一份 | spec §5.1 #5；**模型层只保证 `CreateBatch` 的签名与约定稳定**，谁编排都接得上 |
| 排序契约没写、实现各来一套 | 记忆页顺序前后端不一致（前端自己再排一次，两边打架） | spec §2.3 已定为 `created_at DESC, id DESC` 并写明理由；接口层落地后同步给成员 2，让他不要在 Mock 里另排一遍 |

## 5. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-15 | spec / plan 第 1 版落地。定下 6 项设计决策（排序含 `id` 兜底、缺参 `4001`、归属单独判定、`SET NULL`、`MemoryType` 不加 DB CHECK、`CreateBatch` 进仓储层），记录 7 项待确认与 1 项契约空白 | 分工未定 |
| 2026-09-15 | **第 2 版：范围收窄到模型层** —— §1 拆成「本分支 6 步 / 交接成员 1 的 H0-H7」，§2 文件清单分 A/B；审查计划拆 A 段（现在就做）/ B 段（接口层启用），**A 段的外键验证改成 SQL 级**；契约空白只广播、不动全局文件 | **无阻塞**——步骤 1-3 今天就能全做完 |
| 2026-09-15 | **第 3 版：模型层落地 + 分组 A 全绿（17 项）** —— `user_memory.go` 新增（10 列 + 3 关联 + `MemoryType`/3 常量 + **必需的 `TableName()`**），`migrate.go` 追加 1 行；§3.3 证据命令全部换成**实跑过的版本**（凭据走 `deploy/.env`、grep 锚定 tag、临时库隔离）；修掉 **6 处会误伤正确代码的验收写法** + **1 处机制写反**（`embedding_status` 的默认值走 GORM 而非 DB）。**代码留在工作区，未提交**（用户指定） | **无阻塞**——模型层交付完毕。接口层（H0-H7）待与成员 1 对齐 |
| 2026-09-15 | **第 4 版：补一条静默缺陷检查**（另一提交实际发生过：`user_memory.go` 结构体闭合 `}` 丢失 + `SourceMessage` 的 tag 被截断，丢了 `foreignKey` / `references` → **`go build` / `vet` / `gofmt` 全部照过**）。**新增 §3.2 高危点 9** + §3.3 的 tag 完整性检查（已实测：修复版 = 3 / 损坏版 = 2，确实能报出缺陷） | 暴露了一个验收盲区：原 grep 全是"查有没有违规"，**没有一条查 tag 是否完整**，而缺 key 恰恰不报错。补上后这类损坏在 code 层就能拦住，不必等到 `\d` |
| 2026-09-20 | **按 AGENTS §5.2 引用优先精简**：删除与 spec 重复的接口层设计代码（repo / service / handler 三段示例、字段对照表）、复制粘贴的通用红线与约定（508 → 251 行，行数按 `wc -l` 含空行）；把 A0-A10 的原始输出收成一段结论。**修正一处过期事实**：原文称接口层验收阻塞在 `JWTAuth`（"空壳"），现已核实 `middleware/jwt.go` 已 `Set(ContextKeyUserID, ...)`，该阻塞解除 | 步骤 5、6 未开始；模型层可合并 |

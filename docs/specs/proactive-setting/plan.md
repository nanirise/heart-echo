# plan · 主动消息配置（Proactive Setting）· 施工与审查计划

> 配套合同：[spec.md](./spec.md)。分歧时以 spec 为准。
> 本文件写"怎么做"和"怎么审"，不重复 spec 的约束条文。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-proactive-setting-model`（基点 `3ea4051` = PR #43 合并后，工作区只含本支改动） |
| 负责人 | 成员 3（`JMX2033`） |
| **本支范围** | **仓储层：`proactive_repo.go` 增 `Get` / `UpdateOwned`**（spec §0.1） |
| 前置 | 模型层已随 `6fc970e` 进入 main（`model/proactive_setting.go` + `migrate.go` 一行 + `CreateDefaultSettings`）；spec 已于 2026-09-20 过人工审查（§9 进度记录第二行） |
| 分支存活 | ≤ 3 天（[AGENTS §6](../../../AGENTS.md)）——本支只有 1 个代码文件，半天内应收口 |

---

## 1. 目标与产出物

**一句话**：把这张表的**读**与**写**各收成唯一一个方法，并让"关掉主动消息"这件事**真的写得进去**。

产出物两件：`proactive_repo.go` 的 `Get` / `UpdateOwned`（代码）+ 本 spec/plan（合同）。
端点、DTO、service 全都不在本支——**它们照 spec §2 写，不由本支代劳**。

**为什么值得单开一支**：这张表唯一会**静默失败**；而它的失败形态恰好是"演示时用户点了关闭、
AI 第二天照样主动说话"。这种缺陷在读代码时完全看不出来，只能靠**成对的正反实测**（§3.2）。

---

## 2. 步骤拆解

| # | 步骤 | 产物 | 完成标准 | 状态 |
|---|---|---|---|---|
| 1 | 读现状，确认不动 | —— | `model/proactive_setting.go` 的 4 个 `default:` tag、组合唯一索引、两条 `CASCADE` 外键**逐条核对无误**；确认本支不改这个文件 | ✅ 已完成 |
| 2 | 写 `Get` | `proactive_repo.go` | 双条件 `WHERE persona_id = ? AND user_id = ?`；未命中回 `gorm.ErrRecordNotFound`；不引入单条件查询 | ✅ 已完成 |
| 3 | 写 `UpdateOwned` | 同上 | 收 `tx`；四个值类型参数；**`map[string]any`**；返回 `RowsAffected`；**`map` 里没有 `last_nudge_at`** | ✅ 已完成 |
| 4 | 临时库实测 | §4.3 的 A1-A10 读数 | **A2 与 A3 成对成立**（map 写进零值 / struct 写不进零值）；A4 越权行未动 | ✅ 已完成（读数见 `.learn/proactive-settings.md` §4） |
| 5 | 人工审查 | 本文件 §4 | §4.1 三遍 + §4.2 七个高危点逐条回看 | ✅ 已完成 |
| 6 | 契约空白广播（**阻塞合并项**） | 群里的话（草稿见 §6.1） | 2026-09-20 拍板：**按 `GET /schedules` 先例补 `4001`，本支不改 `API_CONTRACT.md`**。待把草稿发出，由契约 Owner 统一改 | ⬜ 草稿已就绪，待发 |
| 7 | 与 `-api` 支对齐 | 群里的话（草稿见 §6.2） | 2026-09-20 拍板：**接受 `Upsert → UpdateOwned`**，理由如 spec §1.4。待广播，说明 [persona-crud §7.2](../persona-crud/spec.md) 的旧清单被修正；两个端点的归属与开工顺序说定 | ⬜ 草稿已就绪，待发 |

> **提交与 PR 暂缓**：用户指定代码先留在工作区人工审查。

---

## 3. 施工要点

### 3.1 两个方法（spec §3.1 是签名，这里是形态）

- **`Get`**：读路径用包级 `r.db`，`First(&s)`；未命中把 `gorm.ErrRecordNotFound` **原样上抛**——由 service 翻译成 `4043`（仓储层**不引 errcode**，与 `persona_repo.FindOwned` 同一约定）。
- **`UpdateOwned`**：写路径用调用方 `tx`。`Model(&model.ProactiveSetting{}).Where(两个条件).Updates(map[string]any{...})`。
  **`Model` 传空 struct**：不传就退化成"全表更新"（GORM 的 `ErrMissingWhereClause` 保护只在**完全没条件**时触发，而这里恰好有条件，所以不会报错——更该盯紧）。
- **返回值**：`(int64, error)`。`0` 是**业务信号**（spec §1.2），调用方必须判；仓储层不替它决定该回哪个错误码。

### 3.2 实测必须成对（**本支的核心动作**）

| 方向 | 做什么 | 读到什么才算对 |
|---|---|---|
| **正** | `UpdateOwned(enabled=false, 0,0,0)` | `RowsAffected=1`，回读 `enabled=false` **且三个 `0` 都在** |
| **反** | `Updates(model.ProactiveSetting{Enabled:false, DailyLimit:0})` | `Error=nil`、`RowsAffected=0`、**库里值一动不动** |
| **旁证** | 零值 struct 直接 `INSERT` | 落库 `true / 30 / 120 / 3`（`default:` tag 会替换零值） |

**反方向读不出 `0`，就等于没验**：只证明"我的方法能写"不足以说明"别人的写法不行"，
而这份合同的全部价值就在后者。三条读数与执行顺序已原文记在 `.learn/proactive-settings.md` §4（含一处
"读数正确但看起来像错"的坑：`Get` 那条读到的 `enabled=true` 是**前一步 struct 更新改回去的**）。

### 3.3 临时库与收尾

沿用前几支的既定流程（[persona-crud plan](../persona-crud/plan.md) 的 A 段）：凭据从 `deploy/.env` 读（**不写进代码 / 文档**，红线 1）→
建临时库 `proactive_schemacheck` → `go run ./cmd/migrate` 建表 → 跑一次性程序（放 `cmd/tmpcheck/`，**跑完 `rm -rf`**）→ `dropdb`。
**开发库 `heart_echo` 一行都不碰**（它会混进演示数据）。

---

## 4. 我手动审查 AI 代码的计划

> 前提：AI 生成的代码**默认不可信**。本支的代码只有 **36 行新增**（`git diff --numstat`，含空行与注释；
> 文件总长 72 行），但它决定了"用户点关闭"能不能生效——
> 这是**演示时看得见**的功能。审查的目标不是"能编译"，是"我能说出每处零值去了哪里"。

### 4.1 审查方法（两遍 + 实跑）

| 遍 | 做什么 | 判据 |
|---|---|---|
| **第一遍 · 逐行读 tag 与签名** | `proactive_setting.go` 的 4 个 `default:` tag、`uniqueIndex` 的两个 `priority`、两条 `constraint:OnDelete:CASCADE` | **tag 是字符串字面量，编译器不看内容**——"写漏一个 key"只有这一遍能抓到 |
| **第二遍 · 对照 spec 核对形态** | 两个方法的 `WHERE`、参数表、返回值、`map` 的键集合 | 与 spec §3.1 **逐字一致**；`map` 里**没有** `last_nudge_at` |
| **第三遍 · 实库对抗** | §3.2 的三条读数 | 每一条都有实跑输出，不是"读代码觉得没问题" |

### 4.2 七个高危点

| # | 高危点 | 为什么会错 | 我怎么验 |
|---|---|---|---|
| 1 | **`UpdateOwned` 里写成 `Updates(model.ProactiveSetting{...})`** | 这是最自然的写法（前几支的 `UpdateOwned` 都传 struct/标量）——**零值被整条跳过**，"关掉开关"静默失败 | `grep -c 'Updates(model\.ProactiveSetting{'` = **0**；A3 反证 |
| 2 | **改成 `Upsert`**（交接清单写的就是它） | 照旧文档写就会掩盖"播种缺失"，并多一个写归属列的口子（spec §1.4） | `clause.OnConflict` / `.Save(` / `Upsert(` 三种形态的 grep 都 = **0**（命令见 §4.3） |
| 3 | **`last_nudge_at` 混进 `map`** | "顺手把上次触发时间也更新一下"——直接破坏契约 §9 的只读约定，且会让定时任务的空闲判定错乱 | `grep -c '"last_nudge_at"'` = **0**（**带引号**才是 map 键）；A7 |
| 4 | **`Where` 少一个条件** | 少 `user_id` 就是跨用户改别人的配置；少 `persona_id` 会一次改掉该用户的**所有人设**（同一 user 有多行！）——**本表的这个错法比别的表更严重** | A4 越权读数；两个方法体内都能看到 `persona_id = ? AND user_id = ?` |
| 5 | **忘了判 `RowsAffected`** | 调用方只看 `err` 就会把"没匹配到行"当成成功 | 返回 `(int64, error)`；spec §2.2 要求 `0 → 4043` |
| 6 | **`Model(...)` 传空 struct 被省掉** | 省掉后 GORM 仍能拼出 SQL（有条件在），但**更新目标不是本表的 schema**，将来加列会踩坑 | 方法体第一句就是 `Model(&model.ProactiveSetting{})` |
| 7 | **注释被 gofmt 重排** | 多行注释里以标点续行会被 gofmt 当成缩进块（**本轮实测踩过一次**） | `gofmt -l internal/` 无输出；**排在最后跑** |

### 4.3 必须亲眼看到的证据

```bash
# ---- 准备：临时库（凭据走 deploy/.env，不写进代码/文档）----
set -a; . deploy/.env; set +a
docker compose -f deploy/docker-compose.dev.yml exec -T postgres \
  sh -c 'dropdb -U "$POSTGRES_USER" --if-exists proactive_schemacheck && createdb -U "$POSTGRES_USER" proactive_schemacheck'
export SCHEMA_CHECK_DSN="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@localhost:5432/proactive_schemacheck?sslmode=disable"
cd backend && go run ./cmd/migrate

# A1) 编译 + 静态检查（gofmt 排最后，见高危点 7）
go build ./... && go vet ./... && go test ./...

# A2-A6) 一次性程序 cmd/tmpcheck/（跑完 rm -rf，不进仓库）：
#   种子：两个 user，各自一个 persona；persona1 走 CreateDefaultSettings，persona2 零值 struct 直接 INSERT
#   A2  UpdateOwned(1,1,false,0,0,0)                    → RowsAffected=1 / enabled=false / 0,0,0
#   A3  Updates(struct{Enabled:false, DailyLimit:0})    → Error=nil / RowsAffected=0 / 值一动不动
#   A4  UpdateOwned(userID=1, personaID=2)              → RowsAffected=0 / 对方行 enabled=true interval_min=30
#   A5  Get(1,1) / Get(1,2) / Get(1,999)                → nil / ErrRecordNotFound / 同一个 ErrRecordNotFound
#   A6  零值 struct INSERT                              → true / 30 / 120 / 3

# A7) 静态探针 —— ⚠️ 每条都必须【锚定代码形态】，不能用裸词：
#     注释里正写着"用 INSERT 不用 upsert"、"last_nudge_at 不进 map"、"LastNudgeAt 保持 nil"，
#     裸 grep 会把这些【正确】的解释当成违规（本支实测：三条裸 grep 全部误报，见下方陷阱 ①）。
#     ⛔ 也不要用 && 串起来：期望 0 命中的 grep 退出码是 1，链条会断，后面的静默不跑
grep -c 'Updates(model\.ProactiveSetting{' internal/repository/proactive_repo.go  # 期望 0
grep -cE 'clause\.OnConflict|\.Save\(|Upsert\(' internal/repository/proactive_repo.go  # 期望 0
grep -c '"last_nudge_at"'                    internal/repository/proactive_repo.go  # 期望 0（带引号 = map 键）
grep -c  "persona_id = ? AND user_id = ?"    internal/repository/proactive_repo.go  # 期望 2（Get / UpdateOwned 各一）

# A9) 外键没被改坏（本支不动 tag，只验证）
docker compose -f deploy/docker-compose.dev.yml exec -T postgres sh -c \
 'psql -U "$POSTGRES_USER" -d proactive_schemacheck -c "\d proactive_settings"'
#   要看到 2 条 ON DELETE CASCADE（users / personas 各一）、uq_proactive_settings_user_persona 唯一索引、
#   四列的 DEFAULT（true / 30 / 120 / 3）、last_nudge_at 可空

# A8/A10) 收尾
gofmt -l internal/ pkg/                                          # 期望无输出
git status --short | grep -E '\.env$|\.key$|\.pem$'              # 期望无输出
rm -rf backend/cmd/tmpcheck
docker compose -f deploy/docker-compose.dev.yml exec -T postgres \
  sh -c 'dropdb -U "$POSTGRES_USER" proactive_schemacheck'       # 不留临时库，不碰 heart_echo
```

> ⚠️ **本轮实测出的三条"假失败"**（全部来自"照着自己写的探针实跑一遍"，已按锚定写法修掉）：
>
> | # | 陷阱 | 裸 grep 的读数 vs 真相 |
> |---|---|---|
> | ① | 注释里写着**为什么不能这么写**：`// 用 INSERT 不用 upsert`、`// last_nudge_at 不进 map`、`// LastNudgeAt 保持 nil` | `grep -ci "upsert"` = **1**、`grep -c "last_nudge_at\|LastNudgeAt"` = **2**——**全部来自正确的注释**。锚定成 `clause\.OnConflict\|\.Save\(\|Upsert\(` 与 `"last_nudge_at"`（带引号 = map 键）后读 0 |
> | ② | `grep -c 'Updates(model.'` 看似安全，实则依赖调用点的写法 | 锚定到完整接收者 `Updates(model.ProactiveSetting{` 才有判别力 |
> | ③ | 期望 0 的 grep 用 `&&` 串联 | 无匹配 → 退出码 1 → 链条在第一条就断，**后面几条静默不跑**，"没输出"被误读成"全过了" |
>
> 与 [persona-crud §8.5](../persona-crud/spec.md) / [user-memory plan §3.3](../user-memory/plan.md) 记的是**同一类**教训：**探针的价值取决于锚定，不取决于跑没跑。**

### 4.4 拒绝标准（出现任一条就打回重写）

| 打回条件 | 为什么不能只小修 |
|---|---|
| `UpdateOwned` 里出现 struct 形态的 `Updates` / `Save` | 零值跳过或全列覆盖，都是静默的数据错误；`Save` 还会把 `last_nudge_at` 清零 |
| 写成了 `Upsert` / `Clause(clause.OnConflict{...})` | 掩盖播种缺失 + 多一个写归属列的口子（spec §1.4） |
| `UPDATE` 的 `WHERE` 只有 `persona_id` | **一次改掉该用户所有人设的配置**——本表一人设一行，这个错法的影响面比别的表大 |
| `map` 里出现 `last_nudge_at` / `persona_id` / `user_id` | 只读列被写 / 归属列可被调用方改写 |
| `Get` 用 `First(&s, id)` 之类的单条件形态 | 能回答"这个 id 存不存在"，可被顺序试号枚举 |
| 仓储层引 `errcode` / `dto` | 破坏分层：仓储层回 `gorm.ErrRecordNotFound`，翻译在 service |
| 新增了索引或 `ALTER TABLE` | 红线 8；索引也已够用（spec §0.2） |
| `gofmt -l` 有输出 | 格式即门禁（本轮已被拦下一次） |

---

## 5. 风险与对策

| # | 风险 | 影响 | 对策 |
|---|---|---|---|
| ~~1~~ | ~~本支的工作区与上一支混在一起~~ | ~~上一轮 `refactor/trim-model-comments` 的 9 个文件还在工作区，本支的新文件会与之同框~~ | ✅ **已消除**：上一支已作为 **PR #43** 合入 main（`3ea4051`），本支已从它切出 `feature/backend-proactive-setting-model`，工作区**只含本支的 1 个改动文件 + 1 个新目录** |
| 2 | 契约 §9 的 `GET` 行缺 `4001` | 前端只会处理 `4043`，缺参时会走 `message` 兜底分支 | 步骤 6：**只广播、不改全局文件**（2026-09-20 拍板：按 `GET /schedules` 先例补，本支不改 `API_CONTRACT.md`）；**阻塞合并**，契约 Owner 统一改后解除 |
| 3 | 下游照旧清单写 `Upsert` | 两套写入口并存，播种缺失被掩盖 | 步骤 7 群里说定；spec §1.4 写明理由（2026-09-20 拍板：**接受 `Upsert → UpdateOwned`**，广播草稿见 §6.2） |
| 4 | 别人以为"端点也在这个 PR 里" | review 范围不清，或两人同时写 `proactive_handler.go` | spec §0.2 逐个标明去向；PR 描述里写清本支交付面（§6） |
| 5 | 触发链路直接 `UPDATE ... SET last_nudge_at`（绕过本支） | 不冲突，但要保证它同样带两个条件 | 交给 `-api` / `-job` 支；本支只把 `last_nudge_at` 的写入方**排除在设置页之外** |

---

## 6. 广播草稿（2026-09-20 拍板后待发，复制即用）

**6.1 契约 §9 `GET /proactive/settings` 补 `4001`（阻塞合并项）**

```text
【契约空白】API_CONTRACT.md §9 的 GET /proactive/settings 错误码列只写了 4043，需要补 4001。

理由：缺 / 非法 personaId 时没有归属可校验，"查不到 → 4043"在语义上不成立；
而且不能当 personaId=0 查库——命中空集会返回 200 + 空对象，把调用方的 bug 伪装成正常态。
先例：§10 的 GET /schedules 同为"必带 personaId"的端点，错误码列写的是 4001 4043。

建议：GET 行改为 `4001 4043`，按契约纪律登记 §12 变更记录。
本支（feature/backend-proactive-setting-model）不直接改全局文件，等契约 Owner 统一改。
```

**6.2 `Upsert` → `UpdateOwned`：修正 persona-crud §7.2 的旧清单**

```text
【交接修正】persona-crud spec §7.2 的交接清单里写着主动消息模块"继续加 Get / Upsert"，
这一条被 proactive-setting spec §1.4 修正为 Get / UpdateOwned。

理由：POST /personas 同事务播种 + UNIQUE (user_id, persona_id) 已保证"恰好一行"，
所以查不到行 = 播种没跑的 bug，不是需要补一行的正常态。写成 Upsert 会把这个问题
掩盖成一次静默插入：RowsAffected 恒为 1，调用方再也拿不到"行不存在"的信号，
而且 upsert 还要求把 user_id / persona_id 一起塞进写入值——多一个能写归属列的口子。

已实测：Updates(struct) 在零值上是 Error=nil + RowsAffected=0 的静默空操作，
所以这一层必须走 map[string]any（关掉主动消息 = enabled:false + daily_limit:0）。
下游（-api / -job 支）按 proactive-setting spec §3.1 的签名写，不要照旧清单。
```

---

## 7. PR 描述要点（开 PR 时照写）

```markdown
## 本 PR 的范围
仓储层两个方法 + 独立 spec/plan：`proactive_repo.go` 的 `Get` / `UpdateOwned`。
**不含端点**（DTO / service / handler / router 属下一支）。

## 为什么这两个方法值得单开一支
`proactive_settings` 有四列带 default tag，零值在 DTO、仓储、建表三层各被误解一次，
其中两处会让"关掉主动消息"静默失败（Error=nil + RowsAffected=0）。
本 PR 把它写成合同并配成对的正反实测：map 写进零值 / struct 写不进零值。

## 与既有文档的一处差异
persona-crud §7.2 的交接清单写的是 `Upsert`，本 PR 定为 `UpdateOwned`——
查不到行是"播种没跑"的 bug，不该被 upsert 掩盖成一次静默插入。

## 顺带发现
- 契约 §9 `GET /proactive/settings` 的错误码列缺 `4001`（只写了 4043），已登记待广播，本 PR 不改契约文件。
- `model/proactive_setting.go` 的注释密度高于 AGENTS §5.1 的目标（约 20 行）——已拍板**不由本 PR 动**
  （属 PR #28 已过审范围），合并后另开 refactor 分支，与前 3 张表的注释清理一起做；
  其内容已先行落在 `.learn/proactive-settings.md`（该文件不入库）。
```

---

## 8. 已确认的待办（不在本支内）

| # | 待办 | 触发时机与完成标准 |
|---|---|---|
| 1 | **另开 refactor 分支清理 `model/proactive_setting.go` 的注释**（约 20 行 → §5.1 的目标密度） | **本支合并后**，可与前 3 张表（`persona.go` / `chat_message.go` / `user_memory.go`，已随 PR #43 清理）**一起做**。内容已先落在 `.learn/proactive-settings.md`，搬运即可。**2026-09-20 用户拍板：不在本支动**（该文件属 PR #28 已过审范围） |
| 2 | 契约 §9 `GET` 行补 `4001` | 见 §6.1 的广播草稿；**阻塞本支合并**，但**不由本支改** |
| 3 | `Upsert` → `UpdateOwned` 的说明广播 | 见 §6.2 的广播草稿；下游开工前发出 |

---

## 9. 进度记录

| 日期 | 进展 | 阻塞 |
|---|---|---|
| 2026-09-20 | 合同 v1 + 仓储层两个方法落地。**基点**：`3ea4051`（上一支 `refactor/trim-model-comments` 作为 PR #43 合入 main 之后）；分支 `feature/backend-proactive-setting-model`。定下三件事：① **零值 == 缺失**是同一根因在三层各出现一次（DTO 指针 / 仓储 map / `default:` tag），只有仓储层这层会**静默**失败；② `RowsAffected == 0` 语义唯一（Postgres 同值 UPDATE 也算匹配），因此 **`PUT` 的越权判定不需要额外查询**；③ 用 `UpdateOwned` 替代交接清单里的 `Upsert`。实测三条读数成对成立（正 / 反 / 旁证），**代码留在工作区未提交**（用户指定）。**另修掉三条自己写出来的"假失败"探针**（裸 `grep "upsert"` / `"last_nudge_at"` 命中的全是**解释为什么不能这么写**的注释，见 §4.3 陷阱 ①） | **工作区与上一支 `refactor/trim-model-comments` 混在一起**（§5 风险 1，**当日即消除**，见下行）；契约空白广播待发 |
| 2026-09-20（同日拍板回填） | 人工审查通过，**三处拍板全部落到本文与 spec**：① 契约 §9 的 `GET` 缺 `4001` → **按 `GET /schedules` 先例补**，登记为**阻塞合并项**，**本支不动 `API_CONTRACT.md`**（草稿 §6.1）；② `model/proactive_setting.go` 的约 20 行注释 → **本支不动**（属 PR #28 已过审范围），改为**合并后另开 refactor 分支**与前 3 张表的注释清理一起做（§8 第 1 行）；③ `.learn/` 被 gitignore → 认可，关键结论以 spec 为准，深度推导留本地。**并接受 `Upsert → UpdateOwned`**——播种机制保证恰好一行（`POST /personas` 同事务 + `UNIQUE`），查不到行 = **播种没跑的 bug**，`Upsert` 会把它掩盖成"一次静默插入"（§6.2 广播）。**代码仍留工作区未提交** | 无（**合并前需先发出 §6.1 广播**） |

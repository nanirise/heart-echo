# plan · AI 朋友圈动态（AI Moment）· 施工与审查计划

> 配套合同：[spec.md](./spec.md)。分歧时以 spec 为准。
> 本文件写"怎么做"和"怎么审"，不重复 spec 的约束条文。

| 项 | 值 |
|----|-----|
| 分支 | `feature/backend-ai-moment-model` |
| 负责人 | 成员 3（`JMX2033`） |
| **本支范围** | **模型层：`ai_moment.go` + `migrate.go` 一行**（spec §0） |
| 前置 | spec v1.2 已人工审查通过（**未过审不动手写代码**） |
| 分支存活 | ≤ 3 天（AGENTS §6）——本支只有 3 个文件，1 天内应收口 |

---

## 1. 目标与产出物

**一句话**：把 `ai_moments` 的表结构钉死，让下游两个分支可以直接开始写代码，不必再回头改表。

**本支产出（3 个文件）**：

| # | 文件 | 动作 | 内容 |
|---|------|------|------|
| 1 | `internal/model/ai_moment.go` | 新增 | `AIMoment` struct + `TableName()`（spec §3.2 有完整代码，逐字对照） |
| 2 | `internal/model/migrate.go` | 修改 | 追加一行 `&AIMoment{}` |
| 3 | `internal/model/ai_moment_test.go` | 新增 | JSON 键集断言（spec §9 分组 C） |
| 4 | `.learn/moment-gorm-tags.md` | 新增（**不入库**） | 学习笔记：DDL 对照、坑、反例展开 |

**交接物（本支只写设计，不写代码）**：

| 交付给 | 内容 | spec 位置 |
|--------|------|----------|
| `feature/backend-moment-api` | 四条 SQL 全文、仓储方法形态、DTO、错误码、越权防线 | spec §4、§5 |
| `feature/backend-moment-job` | `ContentGenerator` 接口 + 阶段一/二切换方案 + tick 顺序 | spec §6 |
| 下游模型分支 | `moment_comments` / `moment_likes` 的 DDL 与四个落地注意点 | spec §7 |
| **成员 1** | `RegisterMomentRoutes(protected, momentHandler)` 的一行挂载 | spec §0.2 |

---

## 2. 施工步骤

> 原则：**建表 → 真库核对 → 再写代码**。表结构错了，下游全部返工。

### 本支执行的步骤

| 步 | 动作 | 完成判据 |
|----|------|---------|
| 0 | 群里确认 spec §8.2 的 5 条（**特别是第 4 条：是否并入下游两个 model**） | 有结论 |
| 1 | 写 `model/ai_moment.go` | `go build ./...` 过 |
| 2 | `migrate.go` 追加一行 `&AIMoment{}` | `go run ./cmd/migrate` 建表成功 |
| 3 | **真库核对**（§3.2） | spec §9 分组 A 前四条全绿 |
| 4 | 写 `ai_moment_test.go`（§3.3） | `go test ./internal/model/` 过 |
| 5 | **反向验证**（§5.4）：4 条注入逐一确认报红 | 4/4 报红，读数记回 §5.4 |
| 6 | 写 `.learn/moment-gorm-tags.md`（§3.4） | 长注释已从 `.go` 挪走 |
| 7 | 自测 + PR（§7） | `go build ./... && go vet ./... && go test ./...` 过 |

**建议的 commit 切分**（AGENTS §6「产出即提交」）：

```
feat(moment): add ai moment model
feat(moment): register ai moment in automigrate
test(moment): assert ai moment json key set
docs(moment): add spec and plan for ai-moment
```

### ⛔ 步骤 8-11：**本分支不做，交接给下游分支**

| 步 | 交接给 | 内容 | 触发条件 |
|----|--------|------|---------|
| **8** | `feature/backend-moment-job` | **`ContentGenerator` + `moment_job.go` + `config.JobConfig` + `.env.example` + `go.mod` 的 `robfig/cron/v3`** | `ai-service` 的生成端点确认后（spec §8.2 第 1、2 条） |
| **9** | `feature/backend-moment-api` | `dto/moment_dto.go` + `repository/moment_repo.go` + `service/moment_service.go` + `handler/moment_handler.go` | 本支合并后即可开工 |
| **10** | 下游模型分支 | `moment_comments` / `moment_likes` 两个 model（spec §7 的 DDL 与四个注意点）；**落地时补跑序列污染验收** | 与步骤 9 同批即可 |
| **11** | **成员 1** | `router.go` 加一行 `RegisterMomentRoutes(protected, momentHandler)` | 步骤 9 完成后通知 |

> **步骤 8 为什么必须压后**：`robfig/cron/v3` 是本支唯一需要新增的依赖，而本支的 3 个文件**一行都用不到它**。为一个用不上的包动 `go.mod`，会让"本支只做模型"这个边界模糊掉——留着给真正需要它的分支加。
> **步骤 10 为什么不能省**：spec §1.1 的序列污染（`bigserial` 那个坑）**只有在下游表存在时才观测得到**。本支只能做静态 tag 检查，完整的 `\ds` 验收必须在步骤 10 补跑（spec §9 分组 E 第 1 条）。

---

## 3. 本支的实现要点

### 3.1 写 `ai_moment.go`（步骤 1）

代码在 **spec §3.2**——那里有完整的 26 行 struct，**逐字对照**，不要临场发挥。

三件最容易写错的事（只在这里重复一次）：

```go
// ① ID：bigint 不是 bigserial（spec §1.1）
ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"id"`

// ② 外键列不带 autoIncrement —— 否则 ai_moments_persona_id_seq 会冒出来（spec §1.1）
PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;index:idx_moments_persona_time,priority:1" json:"personaId"`

// ③ 情绪列：json 必须是 "-"（spec §1.2）
// 内部信号：只决定生成语气，不进任何响应体（AGENTS §4.4）。前端物理拿不到，从结构上杜绝误渲染。
EmotionLabel *string `gorm:"column:emotion_label;type:varchar(20)" json:"-"`
```

**注释预算：全文件 10-15 行**（spec §1.5）。三类各覆盖一处即可：安全红线（`EmotionLabel`）、GORM tag 语义（`ID` / `PersonaID`）、函数职责（`TableName`）。
`createAt` 的 `default:now();autoCreateTime` 组合、索引 `priority` 的含义、`sort:desc` 与查询侧 `ORDER BY` 的关系——**都不写**，进 `.learn/`。

### 3.2 真库核对（步骤 3，**别跳**）

```bash
docker compose -f deploy/docker-compose.dev.yml up -d
cd backend && go run ./cmd/migrate

psql "$DSN" -c '\d ai_moments'
psql "$DSN" -c '\ds' | grep ai_moment
```

**逐条确认**（spec §9 分组 A）：

| 看什么 | 期望 | 抓什么错 |
|--------|------|---------|
| 列 | **恰好 6 列** | 派生字段被写进 model（spec §3.5） |
| `\ds` | 只有 `ai_moments_id_seq` | 外键列误加 `autoIncrement` |
| 外键 | `persona_id → personas` 且 `ON DELETE CASCADE` | 漏写关联字段（spec §1.3） |
| 索引 | `idx_moments_persona_time (persona_id, created_at DESC)` | priority / sort 写错 |

再手工删一个人设，确认 `SELECT count(*) FROM ai_moments WHERE persona_id = <已删id>` 为 0。

> 每改一次 tag 都重跑这一节。表结构的偏差在编译期毫无提示。

### 3.3 序列化测试（步骤 4，**已确认落地**）

**为什么需要它**：本支没有 HTTP 层，`json:"-"` 与"派生字段混进 model"这两个最危险的约束**一个都验不了**。grep 只能验写法，验不了行为。一个 20 行的单测就能补上。
**这是 `internal/model` 下的第一个测试文件**——键集断言这套模式，后续每个 model 都可以照抄。

```go
// TestAIMomentJSONKeys 锁死对外的键集：契约 §8 的 Moment 没有 emotionLabel，
// 表里也没有 personaName / liked / commentCount 三列（spec §3.5）。
func TestAIMomentJSONKeys(t *testing.T) {
	// marshal 一个零值 AIMoment，把 JSON 解析成 map，断言键集恰好 5 个
	want := map[string]bool{
		"id": true, "personaId": true, "content": true,
		"likeCount": true, "createdAt": true,
	}
	// ... 断言 got 与 want 逐键相等，多一个少一个都 t.Fatalf
}
```

**关键**：断言的是**键集相等**（不是"包含"），这样多一个字段也会报红——正好抓住 §3.5 那个"加了字段就建出真列"的陷阱。

### 3.4 `.learn/moment-gorm-tags.md`（步骤 6）

从 `.go` 里**挪出来**的长内容，全部落这里：

- DDL 逐列 ↔ GORM tag ↔ `\d` 渲染结果的对照表；
- `copyableDataType` 为什么复制 `DataType`；`bigserial` 与 `bigint+autoIncrement` 渲染相同而 DataType 不同的原理；
- 这一次为什么是"第四次机会"（前三次在 `user.go` / `persona.go` / `chat_message.go`）；
- `autoCreateTime` + `default:now()` 为什么两个都要写；
- 索引 `priority` / `sort:desc` 的含义；
- spec §4.6 的 11 条反例逐条展开。

`.learn/` 在 `.gitignore` 里（`.gitignore:75`），**不入库**。

---

## 4. 交接速查（下游分支要执行的，本支不落）

不复制内容，只给指针——**单一事实来源在 spec**：

| 下游任务 | 抄哪里 | 三条最容易翻车的点 |
|---------|--------|------------------|
| `moment_repo.go` | spec §4.4（四条 SQL 全文）、§4.5（方法形态） | ① `SELECT` 必须写 `AS` 别名（不带别名 `personaName` 静默空串）；② `l.user_id = ?` 写在 **ON** 里不是 WHERE；③ `CountByUser` 与 `ListByUser` 的 JOIN/WHERE 逐字相同 |
| `moment_service.go` | spec §4.4 ③④、§4.7 | ① 自增**只在** `RowsAffected == 1` 时执行；② 事务里的业务错误原样透传，`Wrap` 会把 `4040` 洗成 `5003` |
| `moment_handler.go` | spec §5.1、§5.2-5.5 | 复用 `currentUserID` / `idParam`；`:id` 解析失败返回 `4040` 不是 `4001` |
| `moment_dto.go` | spec §5.6 | `grep -i emotion` 必须零命中 |
| `moment_job.go` | spec §6.3、§6.5 | 评论候选必须按 `user_id` 过滤（同账号） |
| 两个下游 model | spec §7 | 可空外键用**指针**；序列污染验收补跑 |

---

## 5. 我手动审查 AI 代码的计划

> 前提：代码由 AI 生成，我**逐行审**，不"看着像对就过"。审查的是 spec 的落实，不是代码风格。

### 5.1 审查顺序

`ai_moment.go` → `migrate.go` → `ai_moment_test.go` → 真库 `\d`。
**先审 tag（静态），再审真库（运行时）**——两者互为交叉验证：tag 对而 `\d` 不对，说明 tag 语义理解错了。

### 5.2 逐文件审查清单

| 文件 | 只看这几件事 |
|------|-------------|
| `ai_moment.go` | ① `ID` 是否 `type:bigint;primaryKey;autoIncrement`，全文件**无 `bigserial`**；② 全文件 `autoIncrement` **恰好 1 处**（只在 ID）；③ `EmotionLabel` 是否 `json:"-"` 且是 `*string`；④ 是否**没有** `personaName` / `liked` / `commentCount` / `UserID` 四个字段；⑤ 关联字段 `Persona Persona` 是否在且带 `OnDelete:CASCADE`；⑥ 索引 tag 是否 `priority:1` / `priority:2,sort:desc`；⑦ 注释是否 10-15 行、无 DDL 对照 |
| `migrate.go` | 是否只加了一行 `&AIMoment{}`，且在 `ProactiveSetting` 之后；没有动其他行 |
| `ai_moment_test.go` | 断言的是**键集相等**还是"包含"？后者是假测试（加字段不会报红） |
| `.learn/` | 是否**未被** `git status` 列出 |
| `git diff --name-only` | 是否只有 4 个文件（含 `.learn/` 则为异常） |

### 5.3 高危点排序（**只审三条的话，审这三条**）

1. **`ID` 与 `PersonaID` 的 tag**——一个决定下游会不会长出序列，一个决定这张表会不会自己长出序列；两者都是"编译通过、运行正常、事后才炸"。
2. **`EmotionLabel` 的 json tag 与 `*string`**——踩的是 AGENTS §4.4 红线，且它在 model 层是最后一次能拦住的地方（进了 DTO 就散到各处）。
3. **有没有多出来的字段**——多一个字段 = AutoMigrate 多一列真列 + 越权防线可能被削弱（spec §4.2）。

### 5.4 反向验证记录（**注入缺陷，确认检查会报红**）

只勾"通过"不够——**要证明检查在代码变坏时真的会失败**，否则可能是"检查本身失效"。

**本支可跑的 4 条**：

| # | 注入的缺陷 | 期望哪条报红 | 实际读数 |
|---|-----------|-------------|---------|
| 1 | `ID` 的 tag 改成 `type:bigserial` | spec §9 分组 A 的 `grep -n "bigserial"` 零命中 | ⬜ 待跑 |
| 2 | `EmotionLabel` 的 tag 改成 `json:"emotionLabel"` | 分组 C 键集断言（**行为层**报红） | ⬜ 待跑 |
| 3 | 给 struct 加 `PersonaName string \`json:"personaName"\`` | 分组 C 键集断言 | ⬜ 待跑 |
| 4 | 删掉 `Persona Persona` 关联字段 | 分组 A 的 `\d` 里外键消失 | ⬜ 待跑 |

**下游分支补跑的 2 条**（本支无 SQL / 无 HTTP，跑不了）：

| # | 注入的缺陷 | 期望报红 | 归属 |
|---|-----------|---------|------|
| 5 | 删掉 `ListByUser` 的 `WHERE p.user_id = ?` | 分组 B「A 的 list 不含 B 的动态」 | `backend-moment-api` |
| 6 | 去掉点赞的 `RowsAffected == 1` 判断 | 分组 C「连续点赞 3 次 likeCount 不变」 | `backend-moment-api` |

**任何一条注入后检查仍然"通过"，说明那条验收是假的**——先修验收，再改代码。结果连同日期提交（沿用 persona-crud §8.5 的做法）。

> ⚠️ **诚实说明第 1 条的局限**：`bigserial` 的真正后果（下游外键列长出 `nextval`）**本支观测不到**，因为下游表还不存在。本支只能证明"静态检查会报红"，**不能**证明"污染不会发生"。完整的运行时证据在步骤 10 补跑。

### 5.5 拒绝标准（出现任一条就退回重写，不做"小修小补"）

1. `ai_moment.go` 出现 `bigserial`；
2. `autoIncrement` 出现 2 次以上（外键列误加）；
3. `EmotionLabel` 的 json tag 不是 `-`，或不是 `*string`；
4. `model.AIMoment` 出现 `personaName` / `liked` / `commentCount` / `UserID` 任一个；
5. 删掉 `Persona` 关联字段（外键建不出来）；
6. `.go` 文件里出现 DDL 逐字对照 / 长篇学习性解释 / 反例分析（spec §1.5）；
7. 改了 `router.go`、`go.mod` 或他人的 model 文件；
8. 序列化测试断言的是"包含"而不是"键集相等"；
9. **`\d ai_moments` 的列宽与 DDL 不符**——尤其 `like_count` 不是 `integer`（`type:int` 会静默变 `bigint`），
   或 `persona_id` 的 Default 不为空（§1.1 的复制污染）。**只看列名不看类型，等于没查。**

---

## 6. 风险与对策

| 风险 | 概率 | 对策 |
|------|:---:|------|
| **`\ds` 序列污染本支验不了** | 高 | 静态 tag 检查 + §3.2 的 `ai_moments_persona_id_seq` 检查；完整验收登记在步骤 10 |
| 派生字段被写进 model，静默建成真列 | 中 | §3.3 的键集断言测试；§5.4 注入 3 |
| tag 写错但 `go build` 通过 | 中 | 步骤 3 真库核对，**每改一次 tag 重跑**。**已实际发生**：`type:int` → `bigint`（2026-09-17），编译/单测/静态 grep 全部放行 |
| 范围蔓延（顺手把 repo / DTO 写了） | 中 | §2 的步骤 8-11 表；`git diff --name-only` 必须是 4 个文件 |
| 阶段一正文来源未定，下游卡住 | 中 | 本支不受影响；spec §6.3 已给完整接口，下游照做即可 |
| 与既有 model 注释风格不一致，评审反复 | 中 | spec §8.1 已决策：新文件从简，**不以风格不一致为退回理由** |

---

## 7. PR 描述草稿

```markdown
# feat(moment): add ai_moment model and migration

## 范围（模型层，只有 3 个文件）
- `internal/model/ai_moment.go`：`AIMoment` struct
- `internal/model/migrate.go`：追加一行 `&AIMoment{}`
- `internal/model/ai_moment_test.go`：JSON 键集断言

## 明确不在本 PR 内（已交接，见 spec §0.2）
- `moment_comments` / `moment_likes` 两个 model —— 下游分支（**序列污染验收也在那时补跑**）
- dto / repo / service / handler / 路由 —— `feature/backend-moment-api`
- `ContentGenerator` + `moment_job` + `robfig/cron/v3` + 配置 —— `feature/backend-moment-job`
- 前端页面、手动触发接口（**后者永不做**，AGENTS §7.5）

## 实现要点
- `ID` 用 `bigint + autoIncrement`（**不是 `bigserial`**）——避免 GORM 把
  DataType 复制到下游外键列，让 `moment_comments.moment_id` 长出多余序列
- `emotion_label` 列照建但 `json:"-"`，不进任何响应体（AGENTS §4.4）
- `Persona` 关联字段只为生成 `ON DELETE CASCADE` 外键，不参与序列化

## 验证
- [ ] `go build ./... && go vet ./... && go test ./...`
- [ ] `\d ai_moments` 恰好 6 列；`\ds` 只有 `ai_moments_id_seq`
- [ ] 外键 `persona_id → personas` 为 `ON DELETE CASCADE`
- [ ] 删人设后其动态清零
- [ ] spec §5.4 的 4 条注入验证全部报红（附读数）
```

---

## 8. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|------|------|------|------|
| 2026-09-17 | v1 | 创建 | 与 spec v1 配套的施工步骤、Repository 要点、人工审查计划 |
| 2026-09-17 | v1.1 | **范围收窄至模型层**：本支只 3 个文件；原步骤 8-11 标为「本分支不做，交接给下游分支」；新增 §3.3 序列化测试与 §4 交接速查；§5.4 反向验证拆为「本支 4 条 / 下游 2 条」并写明局限 | 按负责人 2026-09-17 的 9 条决策收尾 |
| 2026-09-17 | v1.2 | §3.3 由建议转为**确认落地**；分支名更正为实际的 `feature/backend-ai-moment-model`；前置版本提到 v1.2 | 按负责人 2026-09-17 的五点确认收尾 |
| 2026-09-17 | v1.3 | 拒绝标准新增第 9 条（**看列宽，不只看列名**）；风险表"tag 写错但 build 通过"标注**已实际发生**（`type:int` → `bigint`） | 真跑 migrate 抓出 `like_count` 偏差后的回填 |

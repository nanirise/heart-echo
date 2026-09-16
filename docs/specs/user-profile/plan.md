# plan · 用户画像（User Profile）

> 配套文档：[spec.md](spec.md)（数据模型 / 接口定义 / 越权防线 / 验收标准）
> 分支：`feature/backend-user-profile-model` ｜ 负责人：成员 3 ｜ 创建：2026-09-16
> 参考基准：[user-memory/plan.md](../user-memory/plan.md)（同一套写法与教训来源）
> 关联：[AGENTS.md](../../../AGENTS.md) ｜ [总纲 §4.2 / §10](../dev/MASTER.md) ｜ [技术文档 §6.2 / §6.3 / §4.3](../TECH_DESIGN.md) ｜ [契约 §7](../../API_CONTRACT.md) ｜ [成员 1 任务书 §4](../dev/MEMBER_1_BACKEND_AI.md)

> ✅ **本分支只落 2 个代码文件 + 3 份文档/笔记**（§2 步骤 A，**2026-09-16 已落地，留在工作区未提交**）：`internal/model/user_profile.go`（新建，46 行）、`internal/model/migrate.go`（**+1 行**）、`docs/specs/user-profile/{spec,plan}.md`、`.learn/user-profile.md`（长讲解，按「代码只写必要注释」的要求从 .go 里搬出来）。**A 组验收已全绿**——证据见 §2 步骤 A4 / A5 与 [.learn/user-profile.md](../../../.learn/user-profile.md) §9。
> 🚧 **步骤 B / C（Repository / Service）本分支不落地**——按 [总纲 §4.2](../dev/MASTER.md)，`GET /profile/portrait` 的提供者是**成员 1**；按 [技术文档 §8](../TECH_DESIGN.md)，画像代码落在**成员 1 的** `memory_repo.go` / `memory_service.go` / `memory_handler.go` 里。**步骤 B / C 是写给成员 1 的施工规格 + 我自己的审查清单**，与 [user-memory/plan.md](../user-memory/plan.md) 的决策 D2 同一处理方式。
> 🚧 **硬阻塞**：接口层端到端验收依赖成员 1 的 `internal/middleware`（`JWTAuth` 目前是空壳，从不 `Set(ContextKeyUserID)`）——**本分支只做模型层验证，不做任何接口层测试**（§4 审查清单里凡属 B 组的条目都标注为"阻塞中"）。
> ⚠️ **阶段一没有画像的写入方**（[spec §7.3 #1](spec.md)）：画像页在阶段一恒为空。**这不是代码问题，是产品决策**——本 plan 的步骤 D 里单列了一条"演示兜底"。
> ⚠️ **不新增任何错误码**；**不写任何密钥**（红线 1）；**不手写 `ALTER TABLE`**（红线 8）。

---

## 1. 目标与产出物

**一句话目标**：让 `user_profile` 这张表**按权威 DDL 存在且只能被正确地读写**——模型层交付物与 DDL 逐列一致（含 `persona_id` 的 DB 级唯一性与两个 `CASCADE` 外键），接口层（交成员 1）的读写路径**在任何输入下都不可能读到或改到别人的画像**。

| # | 产出物 | 类型 | 本分支落地？ | 负责人 |
|---|---|---|---|---|
| 1 | `backend/internal/model/user_profile.go` | 新建 | ✅ | 成员 3 |
| 2 | `backend/internal/model/migrate.go` | **+1 行** | ✅ | 成员 3 |
| 3 | 真库上的表结构 + 行为验证（`\d` + 纯 SQL UPSERT） | 验证 | ✅ | 成员 3 |
| 4 | `docs/specs/user-profile/spec.md` | 文档 | ✅ | 成员 3 |
| 5 | `docs/specs/user-profile/plan.md`（本文） | 文档 | ✅ | 成员 3 |
| 5.1 | `.learn/user-profile.md`（长讲解：GORM 机制 / 坑的历史 / 两种假验证 / 实测记录） | 笔记 | ✅ | 成员 3 |
| 6 | `dto/memory_dto.go` 的画像结构 | 新建 | ❌ 交接 | 成员 1 |
| 7 | `repository/memory_repo.go` 的 `FindPortrait` / `UpsertPortrait` | 追加 | ❌ 交接 | 成员 1 |
| 8 | `repository/persona_repo.go` 的 `ExistsOwnedByUser` | 追加 | ❌ 交接（**消费，不重写**） | 成员 1 |
| 9 | `service/memory_service.go` 的画像方法 | 追加 | ❌ 交接 | 成员 1 |
| 10 | `handler/memory_handler.go` 的 `GetPortrait` | 追加 | ❌ 交接 | 成员 1 |
| 11 | `router.go` 加一行路由 | +1 行 | ❌ 交接 | 成员 1 |
| 12 | 提取链路落 `profile_updates` | 追加 | ❌ 交接 | 成员 1 |
| 13 | 前端画像页三件套 | 新建 | ❌ 成员 2 | 成员 2 |

**完成判据**：spec §8 分组 A 全绿 + 分组 C 前三项有结论。**接口层（B 组）不在本分支的合并门槛内。**

---

## 2. 施工步骤

### 步骤 A · 模型层（✅ 本分支落实）

#### A1. 新建 `internal/model/user_profile.go`

照抄 [DDL](../TECH_DESIGN.md) 的 5 列 + 2 个关联字段 + `TableName()`。**逐字对照 spec §3.2 的对照表**，一列一列译，不要"凭印象写 GORM"。

> ⚠️ **以下就是仓库里实际交付的内容**（2026-09-16 落地）。按用户要求「代码只写必要注释」，
> 长讲解（GORM 内部机制、坑的历史、两种假验证、实测记录）**不在 .go 里**，已移到 [`.learn/user-profile.md`](../../../.learn/user-profile.md)。
> **审查时注意**：代码里的注释是**刻意简短**的，不代表写的时候没想过 —— **"为什么"要去 `.learn/` 找**。

```go
package model

import "time"

// UserProfile 画像表结构体，对应数据库表 user_profile。
// 一个人设一份画像（persona_id 唯一），删账号 / 删人设时由外键级联删除。
// 与 UserMemory 的两点差别：画像可被反复覆盖（全项目唯一有 UPSERT 语义的表），
// 且 persona_id 上的唯一索引是那个 UPSERT 的冲突推断依据。
type UserProfile struct {
	// 对应 SQL：id BIGSERIAL PRIMARY KEY
	// type:bigint（不是 bigserial）：bigserial 会被关联复制到下游外键列，长出 DEFAULT nextval(...)。
	// json:"-"：契约 Portrait 没有 id，本表的身份是 personaId。
	ID uint64 `gorm:"column:id;type:bigint;primaryKey;autoIncrement" json:"-"`

	// 对应 SQL：user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE
	// json:"-"：越权防线的载体，不进响应体。
	// 无 autoIncrement（外键列+自增会让防线失效）、无 index（DDL 没有这条索引）。
	UserID uint64 `gorm:"column:user_id;type:bigint;not null" json:"-"`

	// 对应 SQL：persona_id BIGINT NOT NULL UNIQUE REFERENCES personas(id) ON DELETE CASCADE
	// uniqueIndex：UPSERT（ON CONFLICT (persona_id) DO UPDATE）的冲突推断依赖它，删掉即退化。
	// 该约束同时把候选行缩到 ≤1，故 user_id 无需索引。
	PersonaID uint64 `gorm:"column:persona_id;type:bigint;not null;uniqueIndex" json:"personaId"`

	// 对应 SQL：profile_data JSONB NOT NULL DEFAULT '{}'
	// 复用 jsonb.go 的 JSONB 原样透传，不要在 Go 侧解析成 struct/map（读写一次就会丢未知键）。
	// 单引号不能省：不带引号会渲染成 DEFAULT {}，建表直接失败。
	ProfileData JSONB `gorm:"column:profile_data;type:jsonb;not null;default:'{}'" json:"profileData"`

	// 对应 SQL：updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	// autoUpdateTime（不是 autoCreateTime）：语义是"画像最后一次刷新"。
	// 注意 UPSERT 路径要显式赋值——clause.OnConflict 的 DoUpdates 不走 autoUpdateTime。
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;default:now();autoUpdateTime" json:"updatedAt"`

	// 以下两个关联字段仅供 GORM 生成外键约束用，不参与序列化。
	// 只写标量 UserID / PersonaID 时 GORM 不会创建外键，"删账号 / 删人设级联清空画像"就不成立。
	// Create 时不要赋值（保持零值），否则会连带写入 users / personas。
	User    User    `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Persona Persona `gorm:"foreignKey:PersonaID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName 显式指定表名。
// 必需：GORM 会把 UserProfile 复数化成 user_profiles，与 DDL 的表名不一致。
func (UserProfile) TableName() string {
	return "user_profile"
}
```
**写法要点（每条都对应一个真会踩的坑）**：

| 要点 | 不这么写的后果 |
|---|---|
| `json:"-"` 共 **4 处**：`ID`、`UserID`、`User`、`Persona` | 响应体多出字段 → 契约漂移。**不是 2 处**——两个关联字段也要带 `json:"-"`（`user_memory` 在这里栽过：按 4 数过，实际 7） |
| `type:bigint` + `autoIncrement`，**不是 `type:bigserial`** | 全项目踩过三次：`bigserial` 会把 `DataType` 复制给下游外键列，长出 `DEFAULT nextval(...)` |
| `user_id` **不带** `autoIncrement`、**不带** `index:` | 前者让越权防线失效；后者偏离 DDL（DDL 没有这条索引），`AutoMigrate` 会真的建出来 → 库里多一个对象 |
| `persona_id` 带 `uniqueIndex` | 少了它，"一人设一份"退化成 Go 侧约定，且**没有任何静态检查会响**（spec §3.3）。**已实测**：`DROP INDEX` 后换一个 `user_id` 真能给同一 `persona_id` 插进第二行 |
| `ProfileData` 是 `model.JSONB`，`default:'{}'` 带单引号 | 用 `map` / 固定 struct → 丢键；不带引号 → 建表直接失败 |
| `UpdatedAt` 用 `autoUpdateTime` | 用 `autoCreateTime` → 更新时不再刷新，`updatedAt` 永远是第一次的时间 |
| 两个关联字段的 **三个 key 一个不省** | 少了 → GORM 推断错误或建不出外键，**编译期完全看不出来** |

#### A2. `internal/model/migrate.go` 追加一行

在 `&UserMemory{}` **之后**、注释掉的 `&ProactiveSetting{}` **之前**插入：

```go
		&UserProfile{}, // 引用 users(id) 与 personas(id) ON DELETE CASCADE，两张表都要先建好
```

**要求**：`git diff backend/internal/model/migrate.go` **只显示 1 行新增**。**不要动 `&UserMemory{}` 那行、不要加空行、不要顺手补 `&ProactiveSetting{}`**——那是别人的占位符（最小 diff，避免无谓冲突）。

**为什么排在 `&UserMemory{}` 之后**：与 `UserMemory` 之间**没有依赖关系**（两者互不引用），排在它后面只是为了让 diff 最小、不动既有行。真正的硬要求是**必须在 `&User{}` 与 `&Persona{}` 之后**——它同时引用这两张表。

#### A3. 编译 + 静态检查

```bash
cd backend
go build ./...
go vet ./...
gofmt -l internal/model/user_profile.go internal/model/migrate.go   # 期望：无输出
```

> ⚠️ **这三条过了不代表 tag 写对了**。已实测：一个把 `uniqueIndex` 整行删掉、把 `foreignKey`/`references` 截断、把 `json` 写错的样本，**`go build` / `go vet` / `gofmt -e` / CI 全部照过**（spec §8 分组 A 的说明）。tag 级缺陷只能靠 §4 的 tag 检查 + 真库实测。

#### A4. 真库建表 + 行为验证（**本步骤是模型层的核心验收，别跳过**）

```bash
cd backend
# DSN 从环境变量传，绝不写进代码 / 文件（红线 1）
export SCHEMA_CHECK_DSN='postgres://...@.../heart_echo_check?sslmode=disable'
go run ./cmd/migrate          # 期望无报错；再跑一次，验证幂等
psql "$SCHEMA_CHECK_DSN" -c '\d user_profile'
```

**逐项核对**（完整清单见 spec §8 分组 A）：

| 查什么 | 期望 |
|---|---|
| 5 列的类型 / 可空性 / 默认值 | 与 [DDL](../TECH_DESIGN.md) 逐行一致；`profile_data` 是 `jsonb NOT NULL DEFAULT '{}'`、`updated_at` 是 `timestamptz NOT NULL DEFAULT now()` |
| `persona_id` 的唯一性 | **Indexes 段**有 `idx_user_profile_persona_id UNIQUE, btree (persona_id)`（`uniqueIndex` 的落点）。**只查存在性，不查名字、不查段落**——换 `unique` 写法会落在 Constraints 段且名字不同，同样合格 |
| 2 个外键 | 都是 `ON DELETE CASCADE`；**本表不该出现 `SET NULL`** |
| Indexes 段 | **只有** `user_profile_pkey` 与 `idx_user_profile_persona_id`；**没有** `idx_user_profile_user_id` |
| 序列 | `pg_sequences` 里**没有** `user_profile_user_id_seq` / `user_profile_persona_id_seq` |
| 表名 | `\dt` 里是 `user_profile`，**不是** `user_profiles` |

**四条纯 SQL 行为验证**（不需要任何 Go 业务代码，也不需要接口层——`user_memory` 分支已证明这条路走得通）：

```sql
-- ① 唯一性真的生效（这条是"删掉 uniqueIndex tag 后唯一会响的检查"）
INSERT INTO user_profile (user_id, persona_id, profile_data) VALUES (<你的uid>, <你的pid>, '{}');
INSERT INTO user_profile (user_id, persona_id, profile_data) VALUES (<另一个uid>, <同一个pid>, '{}');
-- 期望：第二条报 ERROR: duplicate key value violates unique constraint "idx_user_profile_persona_id"
-- ⚠️ 判据是"出现 ERROR"：若造冲突行的子查询条件写错，会静默得到 INSERT 0 0（一行没插），
--    看起来"没有重复"，其实唯一性根本没被考验到——本轮亲自踩过这条假验证

-- ② DEFAULT '{}' 生效
INSERT INTO user_profile (user_id, persona_id) VALUES (<uid>, <pid>);
SELECT profile_data FROM user_profile WHERE persona_id = <pid>;   -- 期望 {}

-- ③ 级联删除
DELETE FROM personas WHERE id = <pid>;
SELECT count(*) FROM user_profile WHERE persona_id = <pid>;       -- 期望 0

-- ④ UPSERT 的合并 + 越权兜底（本表独有，spec §4.6 / §5.4）
--   先插 {"occupation":"程序员"}，再执行 upsert {"interests":["跑步"]}
ON CONFLICT (persona_id) DO UPDATE
   SET profile_data = user_profile.profile_data || EXCLUDED.profile_data,
       updated_at   = NOW()
 WHERE user_profile.user_id = <正确的uid>;
-- 期望：两个键都在（合并成功）+ updated_at 变大
--
--   然后故意用**另一个 user_id** 跑同一条语句：
-- 期望：UPDATE 0（静默跳过），库里的内容与 updated_at **都没变**
--   ⚠️ 这条的失败方式是"不报错"，所以必须靠"数据没变"来判，不能靠"有没有报错"
```

> ⚠️ **验证完记得清掉测试数据**；验证用的库不要用开发库/正式库。

#### A5. 反向验证（**每条必须类检查都要问一句"删掉它会怎样"**）

这是本项目吃过大亏之后立的规矩（`62d10fa`：`SourceMessage` 的 tag 被截断，`go build`/`vet`/`gofmt`/当时全部六条既有 grep 全过，因为那六条查的全是"有没有违规"，**没有一条查"该有的 key 是否齐全"**）。

**做法**：把 `user_profile.go` 复制一份副本，逐个注入真实缺陷，确认"至少有一条检查会响"：

| 注入的缺陷 | 期望响的检查 | 实测结果 |
|---|---|---|
| 删掉 `persona_id` 整行 tag 里的 `uniqueIndex` | A-5（`grep -cE 'gorm:"[^"]*unique'` 1 → **0**）+ A4 的唯一性 SQL | ✅ 已验：`unique` 1→0（`uniqueIndex` 含子串 `unique`，锚定式照样命中） |
| 删掉 `ID` / `UserID` 的 `json:"-"` | A-2（2 → **0**） | ✅ 已验：2→0 |
| 给 `ID` 补 `json:"id"`、给 `UserID` 补 `json:"userId"` | A-3（3 → **5**）+ A-13 | ✅ 已验：3→5 |
| 截断 `foreignKey`/`references`（只留 `constraint:OnDelete:CASCADE`） | A-4（2 → **1**） | ✅ 已验：2→1 |
| 给 `user_id` 加 `index:` | A-11（0 → **1**） | ✅ 已验：0→1 |
| 把 `type:bigint;primaryKey` 改成 `type:bigserial` | A-9（0 → **1**） | ✅ 已验：0→1 |
| 把 `TableName()` 的返回值改成复数 `"user_profiles"` | A-6（1 → **0**） | ✅ 已验：1→0 |
| 把 `ProfileData` 从 `JSONB` 改成 `map[string]any` | A-7（1 → **0**） | ✅ 已验：1→0 |
| （不注入缺陷）**只用裸形式跑带真实注释的正确文件** | A-9 / A-13 / A-14 **应全对** | ⛔ **实测反而误报**：A-9 0→1、A-13 0→1、A-14 1→2 → 四条已改锚定形式（§4.4"假检查 ②"） |

> **实测方法**：在临时目录（`%TEMP%`，**不在仓库里**）写正常版与损坏版样本跑上述 grep，用完即删。**本仓库从未被污染**（`git status` 全程干净）。
> ⚠️ **`gofmt` 对以上全部注入缺陷无反应**（实测通过）——这正说明 tag 级检查不能省。
> ⚠️ **"不注入缺陷"那行是被逼出来的**：原以为反向验证就是"注入缺陷看会不会响"，这一轮才发现**同等重要的是"不注入缺陷时会不会误响"**——后者会产出**假失败**，比漏报更消耗人（看到报警就跳过 → 真报警也被淹）。**两条都要跑**。

#### A6. 提交

```bash
git add backend/internal/model/user_profile.go backend/internal/model/migrate.go docs/specs/user-profile/
git commit -m "feat(model): add user_profile gorm struct and migrate"
```

- **scope 用 `user`**（AGENTS §6 的 scope 取值表里没有 `profile`，最贴近的是 `user`；其次是 `model`）。**不要自己发明 `profile`**——scope 表是约定。
- 走 PR，**禁止直推 `main` / `develop`**（红线 2）。

### 步骤 B · Repository（❌ 本分支不落地，写给成员 1）

> 目标文件：**`internal/repository/memory_repo.go`**（[技术文档 §8](../TECH_DESIGN.md) 明确标注"记忆与画像查询"在同一个文件）。
> ⚠️ **该文件同时也是 user-memory 分支的目标文件**——两个分支改同一个文件，**合并前必须在群里错开时间**（AGENTS §4.8）。见 spec §7.3 #2。

#### B1. `FindPortrait`（读）

```go
// FindPortrait 查某个人设的画像。
// ⚠️ 查不到行返回 (nil, nil) 而不是 gorm.ErrRecordNotFound：本表"查不到"是**合法空态**
//    （这个人设还没有画像），不是错误。在仓储层就翻译成"没有"，调用方不必靠 errors.Is 反推——
//    那条路上任何一处漏判都会把空态变成 5003。
// ⚠️ 两个条件都必须带：persona_id 隔离人设，user_id 防越权（AGENTS §4.3 / 技术文档 §4.3）。
//    只带 persona_id 会跨用户串号——persona_id 是全局自增的，别的用户同号人设会被命中。
//    （只带 user_id 在本表恰好不会串人设——一人设一份——但**仍然要带**：
//     防线的写法必须与记忆一致，不能"这张表看着安全就少写一个条件"。）
func (r *MemoryRepo) FindPortrait(ctx context.Context, userID, personaID uint64) (*model.UserProfile, error) {
	var p model.UserProfile
	err := r.db.WithContext(ctx).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		Take(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil // 空态，不是错误
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}
```

**⛔ 绝不提供的方法**：`FindPortraitByID`（按主键单查）、`FindPortraitByPersonaID`（**只带 `persona_id` 的查询**）。少一个方法就少一处泄漏面；尤其那个名字天生只带一个条件的，写出来早晚有人用（spec §5.1 ②）。

#### B2. `UpsertPortrait`（写，**本表最危险的一个方法**）

```go
// UpsertPortrait 按 persona_id 插入或合并画像键；updates 为空直接返回 nil（不写空更新）。
// ⚠️ 这是全项目唯一一个会**改写已有行**的写方法（其余都是 INSERT）。归属校验一旦被绕过，
//    后果不是"多读一条数据"而是**别人的画像被覆盖且不可恢复**（spec §5.4）。
func (r *MemoryRepo) UpsertPortrait(ctx context.Context, tx *gorm.DB, userID, personaID uint64, updates model.JSONB) error {
	if len(updates) == 0 || string(updates) == "{}" {
		return nil // 不写空更新：会把 updated_at 刷成假信息
	}
	db := r.db
	if tx != nil {
		db = tx // 事务句柄由调用方传：一轮提取的产物（记忆 + 画像）要么都落、要么都不落
	}
	return db.WithContext(ctx).Exec(`
		INSERT INTO user_profile (user_id, persona_id, profile_data, updated_at)
		VALUES (?, ?, ?::jsonb, NOW())
		ON CONFLICT (persona_id) DO UPDATE
		   SET profile_data = user_profile.profile_data || EXCLUDED.profile_data,
		       updated_at   = NOW()
		 WHERE user_profile.user_id = ?`,
		userID, personaID, string(updates), userID).Error
}
```

**四条命门（逐条都对应一个真会写出来的错法）**：

1. **`WHERE user_profile.user_id = ?` 是越权兜底，不是可选优化**。`persona_id` 上的 `UNIQUE` **不提供任何越权保护**——`user_id` 不在唯一键里（DDL 就是这样），冲突判定只看 `persona_id`。保护**完全来自这个 WHERE 与上游的归属校验**。不匹配时**静默跳过（0 行）**：失败方向是"没写进去"，不是"写错了地方"。落库方应检查 `RowsAffected`，为 0 时**记日志**（不返回给前端）。
2. **`?::jsonb` 这个转换不能省**：用参数绑定的字符串插入 jsonb 列时 PG 需要显式转换（`jsonb.go` 的 `Value()` 返回 string 正是同一原因）。
3. **在 SQL 侧合并（`jsonb ||`），不要在 Go 里读-改-写**：读出来改再写回有两重风险——丢未知键（spec §3.4）+ 丢更新竞态（两次提取并发时后写的覆盖前写的键）。
4. **`updated_at = NOW()` 要显式写**：`clause.OnConflict` 的 `DoUpdates` **不走 `autoUpdateTime` 的自动填充**。漏了会表现为"画像内容变了但 `updatedAt` 还是旧的"。

#### B3. `ExistsOwnedByUser`（**消费，不要重写**）

```go
// persona_repo.go —— 一条查询、两个条件、只回 bool。
// persona / chat-message / user-memory **已经三个分支声明过它**，本功能是第四个消费方。
// ⛔ 不要再写第四份。
func (r *PersonaRepo) ExistsOwnedByUser(ctx context.Context, userID, personaID uint64) (bool, error)
```

### 步骤 C · Service / Handler（❌ 本分支不落地，写给成员 1）

#### C1. 编排顺序：**先判归属，后查数据**

```go
func (s *MemoryService) GetPortrait(ctx context.Context, userID, personaID uint64) (*dto.PortraitResponse, error) {
	// ① 入口层已经保证 userID != 0（0 一律 4010），这里只做防御性兜底
	// ② 归属判定必须**单独查一次**，不能从画像查询结果反推：
	//    行查不到既可能是"不是你的"也可能是"还没总结过"，这两种在结果集上长得一模一样。
	//    ⚠️ 本表比记忆更隐蔽：记忆那版至少有 total 这个额外信号，画像连"条数"都没有。
	ok, err := s.personaRepo.ExistsOwnedByUser(ctx, userID, personaID)
	if err != nil {
		return nil, errcode.ErrDBFailed
	}
	if !ok {
		return nil, errcode.ErrPersonaNotFound // 4043；"不存在"与"不属于你"同码同文案
	}
	// ③ 查数据（仍然带两个条件——两道防线各司其职，不要用 ② 替代它）
	p, err := s.memoryRepo.FindPortrait(ctx, userID, personaID)
	if err != nil {
		return nil, errcode.ErrDBFailed
	}
	// ④ 空态兜底：没有行 → {} + updatedAt: null（**200，不是错**）
	if p == nil {
		return &dto.PortraitResponse{
			PersonaID:   personaID,
			ProfileData: model.JSONB("{}"), // ⚠️ 显式给 {}，零值会序列化成 null（jsonb.go 71-76）
			UpdatedAt:   nil,               // *time.Time，null
		}, nil
	}
	resp := dto.NewPortraitResponse(p)
	return &resp, nil
}
```

**四条硬要求**：

| # | 要求 | 违反的后果 |
|---|---|---|
| 1 | **`userID == 0` → `4010`**，不进入查询 | 鉴权失败伪装成空画像（与"取不到 `userID` 不许退化成 0"同一条原则） |
| 2 | **归属判定单独查一次**（`ExistsOwnedByUser`） | 用"查不到行"判越权 → **把"还没总结过"误报成 `4043`**，新伴侣的画像页打不开。阶段一**所有**人设都走这条路径 |
| 3 | **空态返回 `{}` 而不是零值** | `"profileData": null`，前端 `Object.keys(null)` 崩 |
| 4 | **未命中统一 `4043`**，不出现 `4030` | `4030`/`4043` 并存即可被二分探测出 id 是否存在；且 `4030` 是**功能越权**的码，本模块无该场景 |

#### C2. DTO（`dto/memory_dto.go`）

```go
type PortraitQuery struct {
	PersonaID uint64 `form:"personaId"` // ⚠️ form tag，不是 json；且大小写敏感
}

type PortraitResponse struct {
	PersonaID   uint64     `json:"personaId"`
	ProfileData model.JSONB `json:"profileData"`
	UpdatedAt   *time.Time `json:"updatedAt"` // ⚠️ 指针：没有画像时必须是 null
}

// NewPortraitResponse 转换函数放 DTO 层，不要写在 model 上（model 不依赖 dto）。
func NewPortraitResponse(p *model.UserProfile) PortraitResponse { ... }
```

- ⚠️ **`form:"personaId"` 必须是 camelCase**。Gin 的 `ShouldBindQuery` 读 `form` tag 且**区分大小写**——写成 `form:"persona_id"` 时 Gin 找不到匹配字段、**不报错**，`PersonaID` 静默为 `0`，然后被 `4001` 拦住。表现为"前端明明传了 `personaId`，接口却说缺参数"。
- ⚠️ **`UpdatedAt` 用 `*time.Time`**：值类型会把"没有画像"序列化成 `"0001-01-01T00:00:00Z"`（`persona.go` 的 `LastMessageAt` 同一条坑）。
- ⚠️ **`ProfileData` 用 `model.JSONB` 而不是 `map[string]any`**：`map` 会把数字变成 `float64` 吃掉大整数精度。

#### C3. Handler + 路由（成员 1）

```go
func (h *MemoryHandler) GetPortrait(c *gin.Context) {
	userID, ok := c.GetUint64(middleware.ContextKeyUserID)
	if !ok || userID == 0 {
		_ = c.Error(errcode.ErrUnauthorized) // 4010，直接中断
		return
	}
	var q dto.PortraitQuery
	if err := c.ShouldBindQuery(&q); err != nil || q.PersonaID == 0 {
		_ = c.Error(errcode.ErrInvalidParams) // 4001；缺 / 非数字 / 0 / 负数统一走这里
		return
	}
	resp, err := h.svc.GetPortrait(c.Request.Context(), userID, q.PersonaID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, response.OK(resp))
}
```

- **`user_id` 只从 JWT 上下文取**（`c.GetUint64`），**永不从 query / body / path 读**；请求 DTO 里**不声明** `userId` 字段（字段不存在就无法被传入——结构性防御比"记得校验"可靠）。
- **handler 里不出现 `response.Fail`**，错误一律 `_ = c.Error(err)` 上抛（红线 6 / 技术文档 §4.4）。
- **`binding` 失败一律 `4001`**，不要细分到 `4002`——那是别的端点的语义。
- 路由：`router.go` 加一行挂 `GET /profile/portrait`。⚠️ **该文件是成员 1 的地盘，且经常被多人改**，不要与别人同时动（AGENTS §4.8）。

#### C4. 提取链路落 `profile_updates`（成员 1，阶段二）

- `/memory/extract` 的响应里带 `profile_updates`（技术文档 §5.3 的 LLM 提取输出契约）。
- **归属字段（`user_id` / `persona_id`）由 Go 侧赋值，不采信 AI 服务的返回体**：`profile_updates` 是**一堆画像键**（`{"occupation": "程序员"}`），**没有归属字段，也不该有**。归属来自本轮对话所属的人设（已通过归属校验）。
- **建议与记忆同一个事务**：一轮提取的产物要么都落、要么都不落，避免"记住了但画像没更新"这种无法解释的半批状态（spec §7.3 #6）。
- **阶段一是空白**：`RuleMemoryExtractor` 只返回 memories。见步骤 D。

### 步骤 D · 演示兜底（⚠️ **产品决策，不是代码任务**）

**问题**：阶段一没有任何东西写 `user_profile`（画像的键由 `profile_updates` 产生，而它属阶段二）。**后果：画像页在阶段一恒为空**——用户看到的永远是"还没有总结出画像"。

**三个选项（spec §7.3 #1）**：

| 选项 | 成本 | 评价 |
|---|---|---|
| ① **演示前灌一批画像数据**（与朋友圈"演示前灌好一批历史动态"同一手法，技术文档 §5.5 已有先例） | 最低（一条 SQL） | ✅ **推荐** |
| ② 让规则档顺带产出 `profile_updates`（把 `FACT_PATTERNS` 命中的槽位映射成画像键，属 ai-service / 成员 1） | 中（约十几行） | 可选；**要成员 1 排期** |
| ③ 不解决，接受画像页演示时为空 | 零 | **可接受**——它在砍功能序第 ④ 位（[总纲 §10](../dev/MASTER.md)），时间紧时本来就不演示 |

**⛔ 唯一不能接受的**：**在 Go 侧从记忆里现编一份画像**。那是伪造数据，且与"画像由 AI 总结"的定位直接矛盾。

**动作**：拿这三个选项在群里要一个结论，**留个话**——不要自己拍板，也不要默默空着（空着的结果是演示当天才发现画像页是空的）。

---

## 3. 依赖与时间安排

| 阶段 | 依赖 | 状态 | 能否现在做 |
|---|---|---|---|
| 步骤 A（模型层） | `model/jsonb.go` ✅、`user.go` ✅、`persona.go` ✅、`migrate.go` ✅、`cmd/migrate` ✅ | **全部就绪** | ✅ **现在就能全做完** |
| 步骤 A4 真库验证 | 一个临时 Postgres 库 + DSN（环境变量传） | 需要自备 | ✅ 可做 |
| 步骤 B / C（接口层） | `middleware.JWTAuth`（🚧 空壳）、`pkg/errcode` ✅、`pkg/response` ✅、`persona_repo.ExistsOwnedByUser`（⚠️ 三处已声明、未落地）、`memory_repo.go`（⚠️ 未落地）、`router.go`（⚠️ 未落地）、`profile_updates`（⛔ 阶段一不存在） | **不全** | ❌ **交接给成员 1** |
| 步骤 D（演示兜底） | 队长 + 成员 1 的结论 | **未定** | ⚠️ 催 |

**建议的时间安排**：

1. **现在**：步骤 A1 / A2（写 struct + migrate 一行）→ A3（编译）→ **A6（提交 PR）**。
2. **A4 / A5 可以放在同一个 PR 里**：真库验证不需要等任何人；把 `\d user_profile` 的输出与四条 SQL 验证的结果贴进 PR 描述（这是"表结构真的对了"的证据，评审人一眼就能看）。
3. **PR 提交后**：拿 §2 的步骤 B / C 与成员 1 对齐（尤其是 `memory_repo.go` 的**文件竞争**——它同时也是 user-memory 分支的目标文件，见 spec §7.3 #2）。
4. **并行**：催步骤 D 的结论（它不影响本分支合并，但影响演示）。

---

## 4. 我手动审查 AI 代码的计划

> 这部分是**方法**，不是清单。清单在 spec §8。这里的重点是怎么审、先审什么、哪些地方 AI 最会犯而静态检查看不出来。
> **B 组条目全部处于 🚧 阻塞状态**（`JWTAuth` 是空壳）——**审查可以做，端到端测试做不了**，别把"审过了"当成"验过了"。

### 4.1 审查顺序（**先审 tag，再审逻辑**）

```
① tag 级（最高优先）→ ② 是否写了不该写的 → ③ 越权防线 → ④ 空态 / 边界 → ⑤ 编译与格式
```

**为什么 tag 优先**：AI 生成的 GORM struct **最容易在 tag 里出错，而这类错误对 `go build` / `go vet` / `gofmt` / CI 完全不可见**（已实测：截断 `foreignKey`、删掉 `unique`、加错 `json`、把 `bigint` 写成 `bigserial`，四类缺陷四道工具全部照过）。**最后才看编译与否**——因为编译通过是最低门槛，它根本筛不出这类问题。

**为什么"不该写的"排在"该写的"前面**：本功能**删除的成本远高于新增的成本**。多一个 `PUT /profile/portrait` 端点 = 多一条写路径 = 多一处越权面；多一个 `index:` tag = 库里多一个对象；多一个 `json:"id"` = 契约漂移。**审"有没有多做"，比审"有没有少做"更值钱。**

### 4.2 逐文件审查清单

#### A. `internal/model/user_profile.go`（本分支实际交付的代码文件）

| # | 看什么 | 怎么判 | 不通过的例子 |
|---|---|---|---|
| 1 | **`json:"-"` 恰好 4 处** | A-2 应得 2（两个数据列）+ 两个关联字段肉眼确认 | 关联字段漏了 `json:"-"` → 序列化时递归展开 `User` → **把 `PasswordHash` 也带出去**（`User.PasswordHash` 有 `json:"-"`，但依赖下游字段的 tag 来兜底是错的） |
| 2 | **`persona_id` 上有 `uniqueIndex`** | A-5 应得 1 + A4 的重复插入 SQL（**判据是出现 `ERROR`，不是"没看到重复行"**） | 少了它：唯一性从 DB 级约束退化成 Go 侧约定，**静态检查全不响**；已实测 `DROP INDEX` 后同一 `persona_id` 真能插进第二行 |
| 3 | **两个关联字段的三个 key 齐全** | A-4 应得 2 | 截断成 `constraint:OnDelete:CASCADE` → 建不出外键，`go build` 照过 |
| 4 | **`ID` 是 `type:bigint`+`autoIncrement`，不是 `bigserial`** | A-9 应得 0 | 下游外键列长出 `DEFAULT nextval(...)`（全项目已踩三次） |
| 5 | **`user_id` 无 `index:`、无 `autoIncrement`** | A-11 应得 0；肉眼看 tag | 前者偏离 DDL，后者**越权防线直接失效** |
| 6 | **`ProfileData` 是 `model.JSONB`** | A-7 应得 1 | AI 常写 `map[string]any` 或 `[]byte` → 丢键 / 序列化行为要额外操心 |
| 7 | **`UpdatedAt` 是 `autoUpdateTime`** | A-12 应得 0（不该出现 `autoCreateTime`） | 用 `autoCreateTime` → 更新时不再刷新 |
| 8 | **`default:'{}'` 的单引号在** | 肉眼看 | 少了引号 → `DEFAULT {}` → **建表直接失败**（这条倒是能被 A4 逮住） |
| 9 | **`TableName()` 存在且返回单数 `"user_profile"`** | A-6 应得 1 | 少了 → 静默建出 `user_profiles`，接口层才报"表不存在" |
| 10 | **注释里没有"解释为什么不能这么写"被误判的** | 逐条 eyeball A-9~A-14 用的 grep 是否锚定（`gorm:"` 前缀 / `gorm:"column:` 行 / `return` 语句） | **裸 grep 单词会误报**：注释里**故意**写着 `bigserial` / `json:"id"` / `check:` / `index:` / `autoUpdateTime` 来解释为什么不能这么写。**本轮实测（对着目标文件的真实注释跑）：A-9 0→1、A-13 0→1、A-14 1→2，三条全部把正确代码误报成失败**，已全部改成锚定形式（spec §8 分组 A 表格）。**审代码时若某条 grep 命中数意外偏高，第一件事是确认它有没有锚定** |

#### B. `internal/model/migrate.go`

| # | 看什么 | 怎么判 |
|---|---|---|
| 1 | **只有 1 行新增** | `git diff backend/internal/model/migrate.go` → `1 insertion(+), 0 deletions(-)` |
| 2 | **`&UserProfile{}` 在 `&Persona{}` 之后** | 肉眼（引用 `users` 与 `personas`，两张都要先建好） |
| 3 | **没动 `&UserMemory{}` 那行、没补 `&ProactiveSetting{}`** | diff 里不该出现它们 |
| 4 | **注释说明了依赖关系** | 参照相邻行的注释风格（`// 引用 users(id) 与 personas(id) ON DELETE CASCADE，两张表都要先建好`） |

#### C. 真库（A4 的产出）

| # | 看什么 | 怎么判 |
|---|---|---|
| 1 | `\d user_profile` 的 5 列与 DDL 逐行一致 | 对照 spec §3.2 的对照表 |
| 2 | **唯一性存在**（不查名字、不查段落） | 本实现落在 **Indexes 段：`idx_user_profile_persona_id` UNIQUE**。按"Constraints 段必须有 UNIQUE"查会误判失败 |
| 3 | Indexes 段**只有** `user_profile_pkey` + `idx_user_profile_persona_id` | 出现 `idx_user_profile_user_id` 就是**多做了一件事** |
| 4 | 外键都是 `CASCADE`，**没有 `SET NULL`** | 本表没有 `SET NULL`（那是 `user_memory.source_message_id` 的） |
| 5 | 表名是 `user_profile` 不是 `user_profiles` | `\dt` |
| 6 | 四条 SQL 行为验证都真的跑过 | 尤其是 **UPSERT 的越权兜底**（`UPDATE 0` + 数据没变）——**它不报错是设计，必须靠"数据没变"来判** |

#### D. 接口层代码（成员 1 的，**我审但我不改**）

> ⚠️ **🚧 全部阻塞**：`JWTAuth` 是空壳、`memory_repo.go` / `router.go` 未落地。**现在只能做静态审查**（读代码 + grep），**跑不了端到端**。别把"读过"当成"验过"。

| # | 看什么 | 高危点 |
|---|---|---|
| 1 | **`user_id` 只有一个来源** | 全文件 grep `userID` / `userId`，确认**没有一处**从 query / body / path 取。DTO 里**不该有** `userId` 字段 |
| 2 | **`c.GetUint64` 的第二个返回值被检查了** | 漏检 → 取不到时静默用 `0` 继续查 → 鉴权失败伪装成空画像 |
| 3 | **查询带两个条件** | `Where("persona_id = ? AND user_id = ?")`；**不存在**只带 `persona_id` 的查询方法 |
| 4 | **归属判定是单独的一次查询** | 不是从画像查询结果反推（**本表尤其危险：空态与越权在返回值上是完全同一件事**，连条数都没有） |
| 5 | **`UpsertPortrait` 的 `DO UPDATE` 带 `WHERE user_profile.user_id = ?`** | **本表最危险的一处**（spec §5.4）。漏了它，归属校验一旦被绕过就是**覆盖别人的画像且不可恢复** |
| 6 | **`DO UPDATE` 里显式写了 `updated_at = NOW()`** | 漏了 → "画像变了但 `updatedAt` 还是旧的"（`OnConflict` 不走 `autoUpdateTime`） |
| 7 | **`?::jsonb` 的转换在** | 漏了 → PG 报类型不匹配（会在测试中暴露，但知道原因能省十分钟） |
| 8 | **没有读-改-写合并** | grep 全仓库 `profile_data` / `ProfileData`，确认**没有** `json.Unmarshal` / 解成 `map`；合并只在 SQL 侧用 `||` |
| 9 | **空态给的是 `model.JSONB("{}")`** | 不是零值。**实测判据**：`jq -r '.data.profileData'` 必须是 `{}` 而不是 `null` |
| 10 | **`updatedAt` 是 `*time.Time`** | 值类型 → `"0001-01-01T00:00:00Z"` |
| 11 | **未命中只有 `4043` 一个码** | 不能出现 `4030`；也不能 `4030`/`4043` 混用 |
| 12 | **`form:"personaId"` 是 camelCase** | 写成 `persona_id` → Gin 静默不绑，`PersonaID` 为 0 → 被 `4001` 拦住 |
| 13 | **不该写的端点没有** | 全仓库无 `POST` / `PUT` / `DELETE` 的 `/profile/portrait` 路由 |
| 14 | **写入方唯一** | `grep -rn "user_profile" --include=*.go` 只命中 model + memory_repo + 迁移/测试；**没有第二处自己拼 INSERT 的地方** |
| 15 | **无硬编码错误码 / 文案 / secrets** | 红线 1 与红线 6；`_ = c.Error(err)` 而不是 `response.Fail(4043, "人设不存在")` |

### 4.3 高危点排序（**只审三条的话，审这三条**）

1. **`UpsertPortrait` 的 `DO UPDATE ... WHERE user_profile.user_id = ?`**（D-5）
   —— 本表独有的、**读泄漏之外的第二种越权**，且**结果不可逆**（覆盖别人的画像，没有历史版本）。这是本功能最危险的一处，也是最容易被"照抄记忆的检查清单"整个漏掉的一处（记忆只有 INSERT，**没有这个风险**）。
2. **`persona_id` 上的 `uniqueIndex` tag**（A-2）
   —— **四道静态工具全部对它无反应**（实测），而它的失效方式（并发建出两行、`First()` 随机返回、画像"时不时回退"）**极难在演示中定位**。
3. **空态：`profileData` 必须是 `{}` 而不是 `null`**（D-9）
   —— **阶段一所有请求都走这条路径**（没有写入方），所以这不是边界情况，**是默认路径**。而 `model.JSONB` 的零值序列化恰好就是 `null`，写错的方式**太自然了**。

### 4.4 反向验证记录（**本项目立的规矩：每条必须类检查都要能用真实缺陷打穿**）

**规矩的来由**：`62d10fa` —— `UserMemory.SourceMessage` 的 tag 被截断（`foreignKey` / `references` 没了），**`go build` / `go vet` / `gofmt` / 当时全部六条既有 grep 全部照过**。因为那六条查的全是"有没有违规"，**没有一条查"该有的 key 是否齐全"**。**只能检测"多做"，检测不了"少做"**——而少做的缺陷恰恰全静默。

**本次（user-profile）实测**：

| 检查 | 正常版 | 损坏版 | 结论 |
|---|---|---|---|
| A-1 `grep -c 'gorm:"column:'` | 5 | 5 | 数据列数正确 |
| **A-2** `grep 'gorm:"column:' \| grep -c 'json:"-"'` | **2** | **0**（删掉两处 `json:"-"`） | ✅ 能响 |
| **A-3** `grep 'gorm:"column:' \| grep -oE 'json:"[^"]*"' \| grep -v 'json:"-"' \| wc -l` | **3** | **5**（补 `json:"id"` / `json:"userId"`） | ✅ 能响 |
| **A-4** `grep -cE 'gorm:"foreignKey:[^"]*;references:[^"]*;constraint:OnDelete:'` | **2** | **1**（截断一个关联字段） | ✅ 能响 |
| **A-5** `grep -cE 'gorm:"[^"]*unique'` | **1** | **0**（删掉 `uniqueIndex`） | ✅ 能响 |
| **A-6** `grep -cE 'return "user_profile"'` | **1** | **0**（`TableName()` 返回复数） | ✅ 能响 |
| **A-7** `grep -cE 'ProfileData +JSONB +'` | **1** | **0**（改成 `map[string]any`） | ✅ 能响（本轮补验；并用 `cat -A` 确认 gofmt 用**空格**而非 tab 对齐结构体列，正则形态成立） |
| **A-9** `grep -cE 'gorm:"[^"]*type:bigserial'` | **0** | **1** | ✅ 能响 |
| **A-11** `grep -cE 'gorm:"[^"]*index:'` | 0 | 1 | ✅ 能响 |
| A-12 `grep -cE 'gorm:"[^"]*autoCreateTime'` | 0 | 0 | ⚠️ 见下方"未覆盖" |
| **`gofmt`** | 通过 | **通过** | ⛔ **对全部注入缺陷无反应**——tag 级检查不能省 |
| **仅注释中的缺陷** | 按设计无检查响 | — | ✅ **符合预期**（注释里的缺陷不是缺陷；同时也说明这些检查查的是"代码里真的有没有"） |

**⚠️ 发现并修掉了两条假检查（都是"会安静地骗过你"的那种）**：

**假检查 ①（继承自 user-memory，已弃用）**：`grep -cE 'json:"-.{0,2}$'` 靠**行尾**锚定——tag 后面跟了行尾注释就**静默少算到 0**（实测：正常版 4 → 加行尾注释后 **0**）。**本 spec 没有沿用**，改用 A-2 / A-3 两条不依赖行尾的形态（只要求 `json:"-"` 出现在**含 `gorm:"column:` 的那一行**上）。

**假检查 ②（本轮新发现：裸 grep 单词会被自己的注释误报）**：A-6 / A-9 / A-13 / A-14 原本写的是**裸形式**（`grep -c 'type:bigserial'` / `grep -c 'autoUpdateTime'` / `grep -c 'json:"id"'`）。**对着目标文件（含 plan §2 步骤 A1 里那些真实注释）跑，三条全部把正确代码误报成失败**：

| 裸形式 | 正常版读数 | 期望 | 会被什么误报 |
|---|---|---|---|
| `grep -c 'type:bigserial'` | **1** | 0 | 注释里"写成 `type:bigserial` 会让下游外键…" |
| `grep -c 'json:"id"'` | **1** | 0 | 注释里""顺手"补一个 `json:"id"` 就是契约漂移" |
| `grep -c 'autoUpdateTime'` | **2** | 1 | 注释里"`autoUpdateTime` 让 GORM 更新时填…" |

**已全部改成锚定形式**（锚定 `gorm:"` 前缀 / `gorm:"column:` 行 / `return` 语句），并**复测确认没有因此丢掉检出能力**（锚定版打在损坏版上：A-9 →1、A-13 →1/1、A-14 →0、A-6 →0，全部照常能响）。
> ⚠️ **这不是洁癖**：这三条误报的后果是**把正确的代码判成不通过**（假失败），而假失败会训练人"看到 grep 报警就跳过"——那比没有检查更糟，因为它把真报警也一起淹了。**注释里写反例说明是好事，检查工具就不能再用裸 grep。**

**未覆盖 / 已知弱点（诚实记录）**：

| # | 弱点 | 缓解 |
|---|---|---|
| 1 | **A-12（禁止 `autoCreateTime`）在正常版与损坏版都读到 0** | 该检查只能证明"没有违规"，**不能证明"有 `autoUpdateTime`"**——两条都漏写时它同样得 0。**已补配对项 A-14**（`grep -cE 'gorm:"[^"]*autoUpdateTime'` = 1）进 spec §8 分组 A 表格。**这是"禁止类旁边必须有必须类"的现场实例**（见 §4.4 开头那段规矩的来由） |
| 2 | ~~A-7 未反向验证~~ | ✅ **本轮已补验**：`grep -cE 'ProfileData +JSONB +'` 正常 1 / 改成 `map[string]any` 0。同时用 `cat -A` 确认了 gofmt 对结构体列用的是**空格**（tab 只在行首缩进），所以这个正则形态成立——**这条前提如果没验，A-7 有可能是一条恒不匹配的废检查** |
| 3 | **`DO UPDATE ... WHERE user_id` 没有任何静态检查能覆盖** | **只能靠 A4 的 SQL 行为验证**（`UPDATE 0` + 数据没变），以及 code review（D-5）。**这也是它被排在 4.3 高危第 1 位的原因**——最危险的一处恰好是最没有工具保障的一处 |
| 4 | **注释里的"反例说明"会让裸 grep 误报** | **本轮实测三条（A-9 / A-13 / A-14）确实误报**，已全部改成锚定形式。**审代码时若看到某条 grep 命中数意外偏高，第一件事是确认它有没有锚定**（见上方"假检查 ②"） |
| 5 | **`gofmt` / `go vet` / CI 对 tag 缺陷零覆盖** | 已实测确认。**这是"必须真库实测 A4"的根本理由**，不是"多验一道更稳妥" |
| 6 | **A-10 / A-11 / A-12 的锚定形式只做了"正确文件"复测，没做损坏版复测** | 它们的锚定前缀（`gorm:"[^"]*`）与已双向复测过的 A-9 形态**同构**，且正确性是"能否匹配到 tag 内部"而非"边界巧妙"，风险低。**落地后把全套 14 条打在真实文件上跑一次即可确认** |

### 4.5 拒绝标准（**出现任一条就退回重写，不要"先合了再说"**）

**模型层（本分支）**：

1. `persona_id` 上没有 `uniqueIndex` tag（A-5 ≠ 1）。
2. `ID` 的 tag 里出现 `bigserial`（A-9 ≠ 0）。
3. `user_id` 上出现 `index:` 或 `autoIncrement`（A-11 ≠ 0）。
4. `json:"-"` 不足 4 处，或出现了 `json:"id"` / `json:"userId"`（A-2 ≠ 2 / A-3 ≠ 3 / A-13 ≠ 0）。
5. 两个关联字段的 `foreignKey` / `references` / `constraint` 任一缺失（A-4 ≠ 2）。
6. `ProfileData` 不是 `model.JSONB`（A-7 ≠ 1），或出现 `check:` tag（A-10 ≠ 0）。
7. 没有 `TableName()` 或返回复数（A-6 ≠ 1）。
8. `updated_at` 上没有 `autoUpdateTime`（A-14 ≠ 1）——**禁止类的配对项，别只看 A-12 = 0 就放过**。
9. `migrate.go` 的 diff 超过 1 行新增，或 `&UserProfile{}` 排在 `&Persona{}` 之前。
10. **A4 没跑**（只有"编译通过了"）：**表结构没在真库上看过 = 没验过**。唯一性、外键的 `CASCADE`、`DEFAULT '{}'`、UPSERT 的合并与越权兜底，四条 SQL 行为验证缺一不可。
11. 任何测试 / 代码里出现 DSN、密码、Key（红线 1）。

**接口层（成员 1 的，我按此提意见）**：

12. `user_id` 出现在请求 DTO 里，或从 query / body / path 取。
13. 查询只带 `persona_id`（不过滤 `user_id`）。
14. **`UpsertPortrait` 的 `DO UPDATE` 没有 `WHERE user_profile.user_id = ?`**（**最高危，直接退回**）。
15. 用"查不到行"判越权（空态被误报成 `4043`）。
16. 空态返回 `profileData: null` 或 `4040` / `4043`。
17. 出现 `4030`，或出现 `4030`/`4043` 混用。
18. 出现 `POST` / `PUT` / `DELETE` 的 `/profile/portrait` 路由。
19. handler 里出现 `response.Fail` / 硬编码错误码数字或文案；handler 直接操作 DB；service 依赖 `*gin.Context`。
20. 在 Go 里读-改-写合并画像（`json.Unmarshal(profile_data)`）。
21. 出现在 `profile_data` 上做手写 `ALTER TABLE` / `DROP TABLE`（红线 8）。

---

## 5. 变更记录

| 日期 | 版本 | 变更 | 原因 |
|---|---|---|---|
| 2026-09-16 | v1 | 创建 | 用户画像的施工步骤与**手动审查 AI 代码的计划**。含模型层步骤 A（含真库验证 A4 与反向验证 A5）、交给成员 1 的 Repository / Service 规格（步骤 B / C）、演示兜底决策（步骤 D），以及审查方法（先 tag 后逻辑、逐文件清单、高危三点、反向验证记录、拒绝标准）。**A-2 / A-3 / A-4 / A-5 / A-6 / A-7 / A-9 / A-13 / A-14 已用真实缺陷双向验证**（§4.4）。共发现并修掉**两条假检查**：① 继承自 user-memory 的 `json:"-.{0,2}$`（依赖行尾锚定，带行尾注释时静默少算到 0）；② **本轮新发现：A-6 / A-9 / A-13 / A-14 的裸 grep 形式会被本文件自己的"反例说明注释"误报成失败**（对着目标文件实测 A-9 0→1、A-13 0→1、A-14 1→2），已全部改为锚定形式并复测检出能力未丢。**A-14 是 A-12 的配对项**（禁止类旁边必须有必须类）。反向验证的方法本身也补了一条：**除了"注入缺陷会不会响"，还要验"不注入缺陷时会不会误响"** |
| 2026-09-16 | v1.1 | 修订 | 补上 A-14 与 4.4 的"假检查 ②"实测记录；修正 A-6 / A-9 / A-13 / A-14 的命令为锚定形式；§4.5 拒绝标准补第 8 条（`autoUpdateTime` 缺失）并顺延编号 |
| 2026-09-16 | v1.2 | **步骤 A 落地 + 两处修订** | ① **步骤 A 已执行**：`model/user_profile.go` 已写、`migrate.go` +1 行（diff 实测 `1 insertion(+)` 未动他人行）、`go build` / `go vet` / `gofmt` 全过、**14 条锚定检查打在真实文件上 14/14**、**A 组真库验证全绿**（表结构 / 唯一索引 / 2 个 `CASCADE` 外键 / `DEFAULT '{}'` / JSONB 往返 / UPSERT 合并与越权兜底 / 两层级联 / 索引反向验证）——证据见 spec §3.3 与 [.learn/user-profile.md](../../../.learn/user-profile.md) §9。② **`uniqueIndex` 取代原定的 `unique`**（落点从 Constraints 段变为 Indexes 段的 `idx_user_profile_persona_id`）；代码块已换成**实际交付内容**。③ **代码只写必要注释，长讲解移到 `.learn/`**（新增产出物 5.1）——审查时"为什么"要去 `.learn/` 找，别把"注释少"误读成"没想过"。④ 记入**两种假验证**（子查询造冲突行静默 `INSERT 0 0`；用新人设测"重复 persona_id"等于没测）——已写进 §4.4 与 A4 的 SQL 验证块 |

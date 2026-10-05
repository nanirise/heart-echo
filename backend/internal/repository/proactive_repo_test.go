// 本文件的测试连真库：计数口径（只数 is_nudge、+08:00 当日零点起、跨日边界）、
// UPDATE 的命中/未命中、JOIN 扫描的过滤都只有真实 SQL 才能验证，DryRun 断言 SQL 片段等于没验。
//
// 隔离手段：连接本机开发用的 PostgreSQL（deploy/docker-compose.dev.yml 的 heart_echo_db），
// 全部读写放在一个事务里、测试结束回滚——包括测试内的 AutoMigrate DDL 与播种数据，
// 开发库的既有数据分毫不动。
// 连不上（CI 无 Postgres 服务、本机容器没起）即 Skip：如实跳过，不假装覆盖。

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/internal/model"
)

// testTx 开一个"用完即回滚"的事务，测试的一切读写都发生在这层里。
func testTx(t *testing.T) *gorm.DB {
	t.Helper()

	// 测试二进制的工作目录是本包目录，backend/.env 要往上找两级；
	// 环境变量的读取仍走 config.Load（本仓库只有 config 包读环境变量），不另开一条路径。
	_ = godotenv.Load("../../.env")
	cfg, err := config.Load()
	if err != nil {
		t.Skipf("配置不完整（CI 无 .env / 缺必填项时属预期），跳过连库测试: %v", err)
	}

	db, err := gorm.Open(
		postgres.Open(cfg.Database.DSN()+" connect_timeout=3"),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Skipf("初始化 GORM 连接失败，跳过连库测试: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Skipf("取底层连接池失败，跳过连库测试: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Ping(); err != nil {
		t.Skipf("PostgreSQL 不可达（CI 无库时属预期），跳过连库测试: %v", err)
	}

	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("开启测试事务失败: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback() })

	// 按外键依赖序迁移本文件用到的四张表：表已存在时是空操作；
	// 即便真的执行了 DDL，也在事务里、随回滚撤销。
	if err := tx.AutoMigrate(
		&model.User{}, &model.Persona{}, &model.ProactiveSetting{}, &model.ChatMessage{},
	); err != nil {
		t.Fatalf("测试事务内 AutoMigrate 失败: %v", err)
	}
	return tx
}

func seedUser(t *testing.T, tx *gorm.DB, username string) *model.User {
	t.Helper()
	u := &model.User{
		Username: username,
		Email:    username + "@proactive-test.local",
		// 一次性测试行，随事务回滚消失；不构成任何真实凭据
		PasswordHash: "not-a-real-credential",
	}
	if err := tx.Create(u).Error; err != nil {
		t.Fatalf("播种用户 %s 失败: %v", username, err)
	}
	return u
}

func seedPersona(t *testing.T, tx *gorm.DB, userID uint64, name string) *model.Persona {
	t.Helper()
	p := &model.Persona{
		UserID:          userID,
		Name:            name,
		PersonalityDesc: "测试用",
		SpeakingStyle:   "测试用",
	}
	if err := tx.Create(p).Error; err != nil {
		t.Fatalf("播种人设 %s 失败: %v", name, err)
	}
	return p
}

func seedMessage(
	t *testing.T, tx *gorm.DB,
	userID, personaID uint64, role model.MessageRole, isNudge bool, createdAt time.Time,
) {
	t.Helper()
	m := &model.ChatMessage{
		UserID:    userID,
		PersonaID: personaID,
		Role:      role,
		Content:   "测试消息",
		IsNudge:   isNudge,
		CreatedAt: createdAt,
	}
	if err := tx.Create(m).Error; err != nil {
		t.Fatalf("播种消息失败: %v", err)
	}
}

func TestCountTodayNudges(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	me := seedUser(t, tx, "count_me")
	other := seedUser(t, tx, "count_other")
	persona := seedPersona(t, tx, me.ID, "计数人设")

	midnight := todayMidnight()

	// 口径自检：边界必须落在 +08:00 的 00:00:00——防"跟随进程时区"与"按 UTC 截断"两类静默错。
	if _, offset := midnight.Zone(); offset != 8*60*60 {
		t.Fatalf("当日零点偏移应为 +08:00（%d 秒），实际 %d 秒", 8*60*60, offset)
	}
	if h, m, s := midnight.Clock(); h != 0 || m != 0 || s != 0 {
		t.Fatalf("当日零点应为 00:00:00，实际 %02d:%02d:%02d", h, m, s)
	}

	seedMessage(t, tx, me.ID, persona.ID, model.RoleUser, true, midnight)
	seedMessage(t, tx, me.ID, persona.ID, model.RoleUser, true, midnight.Add(time.Hour))
	seedMessage(t, tx, me.ID, persona.ID, model.RoleUser, true, midnight.Add(-time.Second))
	seedMessage(t, tx, me.ID, persona.ID, model.RoleUser, false, midnight.Add(time.Hour))
	seedMessage(t, tx, me.ID, persona.ID, model.RoleAssistant, false, midnight.Add(time.Hour))
	seedMessage(t, tx, other.ID, persona.ID, model.RoleUser, true, midnight.Add(time.Hour))

	repo := NewProactiveRepo(tx)
	got, err := repo.CountTodayNudges(ctx, me.ID, persona.ID)
	if err != nil {
		t.Fatalf("CountTodayNudges 出错: %v", err)
	}
	// 六条播种里只有两条该计：今日 00:00:00 整（下界含）与今日之内；
	// 其余四条分别代表昨日 23:59:59 / 用户普通消息 / assistant 回复 / 他人 user 的同 persona 行。
	if got != 2 {
		t.Fatalf("应计入 2 条，实际 %d 条", got)
	}
}

func TestUpdateLastNudgeAt(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	me := seedUser(t, tx, "nudge_me")
	other := seedUser(t, tx, "nudge_other")
	mine := seedPersona(t, tx, me.ID, "我的")
	theirs := seedPersona(t, tx, other.ID, "别人的")

	repo := NewProactiveRepo(tx)
	if err := repo.CreateDefaultSettings(ctx, tx, me.ID, mine.ID); err != nil {
		t.Fatalf("播种我的配置失败: %v", err)
	}
	if err := repo.CreateDefaultSettings(ctx, tx, other.ID, theirs.ID); err != nil {
		t.Fatalf("播种他人配置失败: %v", err)
	}

	// 命中自己的行：更新成功、last_nudge_at 变为逼近当前时刻
	if err := repo.UpdateLastNudgeAt(ctx, tx, me.ID, mine.ID); err != nil {
		t.Fatalf("命中行应返回 nil，实际: %v", err)
	}
	after, err := repo.Get(ctx, me.ID, mine.ID)
	if err != nil {
		t.Fatalf("回读我的配置失败: %v", err)
	}
	if after.LastNudgeAt == nil {
		t.Fatal("更新后 last_nudge_at 不应为 NULL")
	}
	if d := time.Since(*after.LastNudgeAt); d > time.Minute || d < -time.Minute {
		t.Fatalf("last_nudge_at 距当前应在一分钟内（NOW() 写入），实际 %v", d)
	}

	// 未命中（userID 是自己的、personaID 是别人的）：按方法约定返回 nil（理由见方法注释），且分文未动
	if err := repo.UpdateLastNudgeAt(ctx, tx, me.ID, theirs.ID); err != nil {
		t.Fatalf("未命中行按方法约定返回 nil，实际: %v", err)
	}
	theirsAfter, err := repo.Get(ctx, other.ID, theirs.ID)
	if err != nil {
		t.Fatalf("回读他人配置失败: %v", err)
	}
	if theirsAfter.LastNudgeAt != nil {
		t.Fatalf("他人的行不应被更新，实际 last_nudge_at = %v", theirsAfter.LastNudgeAt)
	}
}

func TestListEnabledForScan(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	me := seedUser(t, tx, "scan_me")
	other := seedUser(t, tx, "scan_other")
	talked := seedPersona(t, tx, me.ID, "开了·聊过")
	neverTalked := seedPersona(t, tx, me.ID, "开了·没聊过")
	disabled := seedPersona(t, tx, me.ID, "关了")
	otherUser := seedPersona(t, tx, other.ID, "别人的·开了")

	repo := NewProactiveRepo(tx)
	seedSettings := func(userID, personaID uint64, enabled bool, intervalMin, intervalMax, dailyLimit int) {
		if err := repo.CreateDefaultSettings(ctx, tx, userID, personaID); err != nil {
			t.Fatalf("播种配置失败: %v", err)
		}
		// 借 UpdateOwned 写目标值（enabled=false 也走 map——struct 路径会被 default tag 吞掉）
		if _, err := repo.UpdateOwned(ctx, tx, userID, personaID, enabled, intervalMin, intervalMax, dailyLimit); err != nil {
			t.Fatalf("改配置失败: %v", err)
		}
	}
	seedSettings(me.ID, talked.ID, true, 30, 120, 3)
	seedSettings(me.ID, neverTalked.ID, true, 5, 60, 10)
	seedSettings(me.ID, disabled.ID, false, 45, 180, 7)
	seedSettings(other.ID, otherUser.ID, true, 15, 90, 5)

	sentAt := time.Now().Add(-3 * time.Hour).Truncate(time.Microsecond)
	for _, personaID := range []uint64{talked.ID, otherUser.ID} {
		if err := tx.Model(&model.Persona{}).Where("id = ?", personaID).
			Update("last_message_at", sentAt).Error; err != nil {
			t.Fatalf("写 last_message_at 失败: %v", err)
		}
	}

	rows, err := repo.ListEnabledForScan(ctx)
	if err != nil {
		t.Fatalf("ListEnabledForScan 出错: %v", err)
	}

	// 本方法是跨用户的无 user_id 过滤扫描，开发库里既有的 enabled 行也会被一并返回——
	// 因此按"本测试这 4 个人设"逐一对账（每人出现的条数），不断言总行数；
	// 每人恰好一条的断言同时能抓住 JOIN 写错成笛卡尔积的 bug。
	byPersona := make(map[uint64][]ScanRow, len(rows))
	for _, row := range rows {
		byPersona[row.PersonaID] = append(byPersona[row.PersonaID], row)
	}

	talkedRows := byPersona[talked.ID]
	if len(talkedRows) != 1 {
		t.Fatalf("enabled 且聊过的行应恰好返回一条，实际 %d 条", len(talkedRows))
	}
	talkedRow := talkedRows[0]
	if talkedRow.UserID != me.ID || talkedRow.IntervalMin != 30 || talkedRow.IntervalMax != 120 || talkedRow.DailyLimit != 3 {
		t.Fatalf("配置与归属字段映射错误: %+v", talkedRow)
	}
	if talkedRow.LastMessageAt == nil || !talkedRow.LastMessageAt.Equal(sentAt) {
		t.Fatalf("last_message_at 应回读为 %v，实际 %v", sentAt, talkedRow.LastMessageAt)
	}

	neverTalkedRows := byPersona[neverTalked.ID]
	if len(neverTalkedRows) != 1 {
		t.Fatalf("last_message_at 为 NULL 的行仍应恰好返回一条（空闲判定归 TriggerNow，本层不过滤），实际 %d 条", len(neverTalkedRows))
	}
	if neverTalkedRows[0].LastMessageAt != nil {
		t.Fatalf("从未聊过的行 last_message_at 应为 nil，实际 %v", neverTalkedRows[0].LastMessageAt)
	}

	if len(byPersona[otherUser.ID]) != 1 {
		t.Fatal("跨用户行应返回（定时任务专用扫描，越权判定归 TriggerNow）")
	}
	if len(byPersona[disabled.ID]) != 0 {
		t.Fatalf("enabled=false 的行不应出现在扫描结果里，实际 %d 条", len(byPersona[disabled.ID]))
	}
}

func TestIsIdleOverThreshold(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	me := seedUser(t, tx, "idle_me")
	other := seedUser(t, tx, "idle_other")
	neverTalked := seedPersona(t, tx, me.ID, "没聊过")
	idle := seedPersona(t, tx, me.ID, "静默十分钟")
	theirs := seedPersona(t, tx, other.ID, "别人的")

	// 基准时间用 SQL 的 NOW() 写：本方法的比较也在 SQL 里做。事务内 NOW() 是常量
	// （事务开始时刻），播种与判定看到的是同一个值，恰好 600 秒的边界因此是确定性的，
	// 不会被应用与数据库的微小偏差咬到。
	for _, personaID := range []uint64{idle.ID, theirs.ID} {
		if err := tx.Exec(
			"UPDATE personas SET last_message_at = NOW() - INTERVAL '10 minutes' WHERE id = ?",
			personaID,
		).Error; err != nil {
			t.Fatalf("写 last_message_at 失败: %v", err)
		}
	}

	repo := NewProactiveRepo(tx)
	cases := []struct {
		name              string
		userID, personaID uint64
		thresholdSeconds  int
		want              bool
	}{
		// ② 的语义是"达到/超过"：600 秒整必须触发，601 秒不触发——写成"落在区间内"会把边界判反
		{"恰好达到阈值（600 秒）→ 触发", me.ID, idle.ID, 600, true},
		{"超过阈值（599 秒）→ 触发", me.ID, idle.ID, 599, true},
		{"未达阈值（601 秒）→ 不触发", me.ID, idle.ID, 601, false},
		// spec §3.b：没有空闲起点。阈值取 0 也不触发，防止哪天被写成 COALESCE/取反形式后静默放行
		{"从未聊过（last_message_at IS NULL）→ 不触发", me.ID, neverTalked.ID, 0, false},
		// 双条件各自生效：任何一边写成单条件，这两条里必有一条变红
		{"跨用户：我的 userID + 别人的 persona → 不触发", me.ID, theirs.ID, 600, false},
		{"跨用户：别人的 userID + 我的 persona → 不触发", other.ID, idle.ID, 600, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.IsIdleOverThreshold(ctx, tc.userID, tc.personaID, tc.thresholdSeconds)
			if err != nil {
				t.Fatalf("IsIdleOverThreshold 出错: %v", err)
			}
			if got != tc.want {
				t.Fatalf("阈值 %d 秒应返回 %v，实际 %v", tc.thresholdSeconds, tc.want, got)
			}
		})
	}
}

func TestHasRecentUserMessage(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	me := seedUser(t, tx, "recent_me")
	other := seedUser(t, tx, "recent_other")

	// 每个反例一条独立人设、里头只放该种消息：判定是整个 persona 一个布尔，
	// 混在一条人设上就分不清是哪条消息被算进去了。
	withRealUser := seedPersona(t, tx, me.ID, "有真人消息")
	withNudgeOnly := seedPersona(t, tx, me.ID, "只有注入行")
	withAssistantOnly := seedPersona(t, tx, me.ID, "只有 AI 回复")
	withOldUser := seedPersona(t, tx, me.ID, "真人消息在两小时前")
	silent := seedPersona(t, tx, me.ID, "没有任何消息")
	crossUser := seedPersona(t, tx, me.ID, "混入他人消息")

	recent := time.Now().Add(-30 * time.Minute)
	old := time.Now().Add(-2 * time.Hour)

	seedMessage(t, tx, me.ID, withRealUser.ID, model.RoleUser, false, recent)
	// ④ 的要害：注入的 [nudge] 也是 role='user'，上一次主动消息不能把下一次判定卡死（spec §3.c）
	seedMessage(t, tx, me.ID, withNudgeOnly.ID, model.RoleUser, true, recent)
	// role 过滤的另一半：assistant 回复的 is_nudge 也是 false
	seedMessage(t, tx, me.ID, withAssistantOnly.ID, model.RoleAssistant, false, recent)
	seedMessage(t, tx, me.ID, withOldUser.ID, model.RoleUser, false, old)
	seedMessage(t, tx, other.ID, crossUser.ID, model.RoleUser, false, recent)

	repo := NewProactiveRepo(tx)
	cases := []struct {
		name    string
		persona uint64
		want    bool
	}{
		{"30 分钟前的真人消息 → 命中", withRealUser.ID, true},
		{"只有注入的 [nudge] → 不命中", withNudgeOnly.ID, false},
		{"只有 assistant 回复 → 不命中", withAssistantOnly.ID, false},
		{"真人消息在 1 小时窗口外 → 不命中", withOldUser.ID, false},
		{"一条消息都没有 → 不命中", silent.ID, false},
		{"同 persona、别人的 user_id → 不命中", crossUser.ID, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.HasRecentUserMessage(ctx, me.ID, tc.persona)
			if err != nil {
				t.Fatalf("HasRecentUserMessage 出错: %v", err)
			}
			if got != tc.want {
				t.Fatalf("应返回 %v，实际 %v", tc.want, got)
			}
		})
	}
}

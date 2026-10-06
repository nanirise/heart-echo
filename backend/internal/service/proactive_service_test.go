// 覆盖 proactive_service 的 settings 读写，分两层：
//   - 参数校验与错误映射（4001 / 4010 / 4043）不需要真库：校验都在碰库之前，用不连库的 testDB 就能钉住；
//   - 落库行为（enabled=false 真的写进去了）必须连真库：全部读写包在一个回滚事务里，开发库分毫不动。
//
// 连不上（CI 无 Postgres 服务、本机容器没起）即 Skip：如实跳过，不假装覆盖。
//
// 构造 ProactiveService 时 chat 一律传 nil：本文件只覆盖 settings 读写，不触碰 TriggerNow。
// 触发链路（四层判定）的用例需要一个假的 AIClient，尚未落盘。
package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/internal/repository"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// proactiveTestTx 开一个"用完即回滚"的事务，与 repository 包的 testTx 同款。
// 不复用它：它在另一个包里，且这里只迁移本文件用到的三张表。
func proactiveTestTx(t *testing.T) *gorm.DB {
	t.Helper()

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

	// 按外键依赖序迁移；表已存在时是空操作，真执行了 DDL 也随回滚撤销
	if err := tx.AutoMigrate(
		&model.User{}, &model.Persona{}, &model.ProactiveSetting{},
	); err != nil {
		t.Fatalf("测试事务内 AutoMigrate 失败: %v", err)
	}
	return tx
}

func seedProactiveUser(t *testing.T, tx *gorm.DB, username string) *model.User {
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

func seedProactivePersona(t *testing.T, tx *gorm.DB, userID uint64, name string) *model.Persona {
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

func TestProactiveGetSettings(t *testing.T) {
	tx := proactiveTestTx(t)
	ctx := context.Background()
	repo := repository.NewProactiveRepo(tx)
	svc := NewProactiveService(tx, nil)

	me := seedProactiveUser(t, tx, "pro_get_me")
	other := seedProactiveUser(t, tx, "pro_get_other")
	mine := seedProactivePersona(t, tx, me.ID, "我的")
	theirs := seedProactivePersona(t, tx, other.ID, "别人的")
	if err := repo.CreateDefaultSettings(ctx, tx, me.ID, mine.ID); err != nil {
		t.Fatalf("播种我的配置失败: %v", err)
	}
	if err := repo.CreateDefaultSettings(ctx, tx, other.ID, theirs.ID); err != nil {
		t.Fatalf("播种他人配置失败: %v", err)
	}

	t.Run("自己的配置：字段映射 + 恰好 6 个键 + lastNudgeAt 为 null", func(t *testing.T) {
		resp, err := svc.GetSettings(ctx, me.ID, mine.ID)
		if err != nil {
			t.Fatalf("读自己的配置不应出错: %v", err)
		}
		if resp.PersonaID != mine.ID || !resp.Enabled ||
			resp.IntervalMin != 30 || resp.IntervalMax != 120 || resp.DailyLimit != 3 {
			t.Fatalf("默认值映射错误: %+v", resp)
		}

		// 契约 §9 形状：恰好 6 个键、无 id / userId；lastNudgeAt 序列化成 null，
		// 不是 0001-01-01 —— 后者会让消费方以为它刚被触发过（§1.5）。
		raw, err := json.Marshal(resp)
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("响应不是合法 JSON 对象: %v", err)
		}
		if len(obj) != 6 {
			t.Fatalf("响应应恰好 6 个字段，实际 %d 个: %v", len(obj), sortedKeys(obj))
		}
		for _, key := range []string{
			"personaId", "enabled", "intervalMin", "intervalMax", "dailyLimit", "lastNudgeAt",
		} {
			if _, ok := obj[key]; !ok {
				t.Fatalf("响应缺字段 %q，实际键: %v", key, sortedKeys(obj))
			}
		}
		if string(obj["lastNudgeAt"]) != "null" {
			t.Fatalf("从未触发过 lastNudgeAt 应为 null，实际 %s", obj["lastNudgeAt"])
		}
	})

	t.Run("跨用户 / 跨人设一律 4043，不可区分", func(t *testing.T) {
		// 双条件各自生效：我的 userID + 别人的 persona、别人的 userID + 我的 persona，
		// 都必须落进同一个 not found —— 任何一边写成单条件在这里变红
		assertBizCode(t, mustErr(svc.GetSettings(ctx, me.ID, theirs.ID)), errcode.ErrPersonaNotFound)
		assertBizCode(t, mustErr(svc.GetSettings(ctx, other.ID, mine.ID)), errcode.ErrPersonaNotFound)
	})

	t.Run("不存在的 personaId → 同样 4043", func(t *testing.T) {
		assertBizCode(t, mustErr(svc.GetSettings(ctx, me.ID, uint64(1)<<40)), errcode.ErrPersonaNotFound)
	})

	t.Run("personaId=0 → 4001，不当 0 查库", func(t *testing.T) {
		assertBizCode(t, mustErr(svc.GetSettings(ctx, me.ID, 0)), errcode.ErrInvalidParams)
	})

	t.Run("userID=0 → 4010", func(t *testing.T) {
		assertBizCode(t, mustErr(svc.GetSettings(ctx, 0, mine.ID)), errcode.ErrUnauthorized)
	})
}

// mustErr 收窄 assertBizCode 的接线：断言只关心 err，丢弃 resp。
// （assertBizCode 保持原签名不动 —— 另一个测试文件在用。）
func mustErr(_ *dto.SettingsResponse, err error) error { return err }

func TestProactiveUpdateSettingsValidation(t *testing.T) {
	// 用**不连库**的 db：校验全在碰库之前，这些用例一个都不该走到 Begin。
	// 哪个用例返回了 5003 而不是 4001，就说明它漏过了校验 —— 死库在这里是探针。
	svc := NewProactiveService(testDB(t), nil)
	ctx := context.Background()

	valid := func() *dto.UpdateSettingsRequest {
		e := true
		min, max, limit := 60, 240, 3
		return &dto.UpdateSettingsRequest{
			PersonaID: 1, Enabled: &e, IntervalMin: &min, IntervalMax: &max, DailyLimit: &limit,
		}
	}

	cases := []struct {
		name   string
		mutate func(*dto.UpdateSettingsRequest)
	}{
		{"personaId 为 0", func(r *dto.UpdateSettingsRequest) { r.PersonaID = 0 }},
		{"enabled 缺失", func(r *dto.UpdateSettingsRequest) { r.Enabled = nil }},
		{"intervalMin 缺失", func(r *dto.UpdateSettingsRequest) { r.IntervalMin = nil }},
		{"intervalMax 缺失", func(r *dto.UpdateSettingsRequest) { r.IntervalMax = nil }},
		{"dailyLimit 缺失", func(r *dto.UpdateSettingsRequest) { r.DailyLimit = nil }},
		{"intervalMin 小于 1", func(r *dto.UpdateSettingsRequest) { v := 0; r.IntervalMin = &v }},
		{"intervalMin 大于 1440", func(r *dto.UpdateSettingsRequest) { v := 1441; r.IntervalMin = &v }},
		{"intervalMax 小于 1", func(r *dto.UpdateSettingsRequest) { v := 0; r.IntervalMax = &v }},
		{"intervalMax 大于 1440", func(r *dto.UpdateSettingsRequest) { v := 1441; r.IntervalMax = &v }},
		{"intervalMin 等于 intervalMax", func(r *dto.UpdateSettingsRequest) {
			a, b := 60, 60
			r.IntervalMin, r.IntervalMax = &a, &b
		}},
		{"intervalMin 大于 intervalMax", func(r *dto.UpdateSettingsRequest) {
			a, b := 240, 60
			r.IntervalMin, r.IntervalMax = &a, &b
		}},
		{"dailyLimit 小于 1", func(r *dto.UpdateSettingsRequest) { v := 0; r.DailyLimit = &v }},
		{"dailyLimit 大于 10", func(r *dto.UpdateSettingsRequest) { v := 11; r.DailyLimit = &v }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid()
			tc.mutate(req)
			assertBizCode(t, mustErr(svc.UpdateSettings(ctx, 7, req)), errcode.ErrInvalidParams)
		})
	}

	// 反例的另一半：边界值必须放行。校验过严（比如把 1440 判成越界）只会在这一条上变红。
	// 测试库连不上，所以"放行"的证据就是停在 5003 上。
	t.Run("边界值 1/1440/10 全合法 → 越过校验停在库上", func(t *testing.T) {
		e := false
		min, max, limit := 1, 1440, 10
		_, err := svc.UpdateSettings(ctx, 7, &dto.UpdateSettingsRequest{
			PersonaID: 1, Enabled: &e, IntervalMin: &min, IntervalMax: &max, DailyLimit: &limit,
		})
		assertBizCode(t, err, errcode.ErrDBFailed)
	})

	t.Run("userID=0 → 4010，先于一切校验", func(t *testing.T) {
		assertBizCode(t, mustErr(svc.UpdateSettings(ctx, 0, valid())), errcode.ErrUnauthorized)
	})
}

func TestProactiveUpdateSettingsPersists(t *testing.T) {
	tx := proactiveTestTx(t)
	ctx := context.Background()
	repo := repository.NewProactiveRepo(tx)
	svc := NewProactiveService(tx, nil)

	me := seedProactiveUser(t, tx, "pro_put_me")
	other := seedProactiveUser(t, tx, "pro_put_other")
	mine := seedProactivePersona(t, tx, me.ID, "我的")
	theirs := seedProactivePersona(t, tx, other.ID, "别人的")
	if err := repo.CreateDefaultSettings(ctx, tx, me.ID, mine.ID); err != nil {
		t.Fatalf("播种我的配置失败: %v", err)
	}
	if err := repo.CreateDefaultSettings(ctx, tx, other.ID, theirs.ID); err != nil {
		t.Fatalf("播种他人配置失败: %v", err)
	}

	// 先让 last_nudge_at 有个非空值（模拟已触发过一次），
	// 用来证明 PUT 既不写它、也不把它清零（契约 §9 只读列）
	if err := repo.UpdateLastNudgeAt(ctx, tx, me.ID, mine.ID); err != nil {
		t.Fatalf("预置 last_nudge_at 失败: %v", err)
	}
	before, err := repo.Get(ctx, me.ID, mine.ID)
	if err != nil || before.LastNudgeAt == nil {
		t.Fatalf("预置后应能读到非空 last_nudge_at，实际 err=%v", err)
	}

	enabled := false
	min, max, limit := 60, 240, 5
	resp, err := svc.UpdateSettings(ctx, me.ID, &dto.UpdateSettingsRequest{
		PersonaID: mine.ID, Enabled: &enabled, IntervalMin: &min, IntervalMax: &max, DailyLimit: &limit,
	})
	if err != nil {
		t.Fatalf("改自己的配置不应出错: %v", err)
	}
	if resp.Enabled || resp.IntervalMin != 60 || resp.IntervalMax != 240 || resp.DailyLimit != 5 {
		t.Fatalf("响应应是更新后的回读值，实际 %+v", resp)
	}
	if resp.LastNudgeAt == nil || !resp.LastNudgeAt.Equal(*before.LastNudgeAt) {
		t.Fatalf("PUT 不得改动 last_nudge_at，PUT 前 %v、响应 %v", before.LastNudgeAt, resp.LastNudgeAt)
	}

	// 关键证据在库里，不在响应里：直连库回读（同一事务句柄）。
	// 若 UpdateOwned 哪天被改回 struct 路径，enabled=false 会被 default tag 跳过、库里仍是 true——
	// 这条断言就是冲着"库里真的是 false"来的。
	stored, err := repo.Get(ctx, me.ID, mine.ID)
	if err != nil {
		t.Fatalf("回读我的配置失败: %v", err)
	}
	if stored.Enabled {
		t.Fatal("库里 enabled 应已变成 false，实际仍为 true（UpdateOwned 走了 struct 路径？）")
	}
	if stored.IntervalMin != 60 || stored.IntervalMax != 240 || stored.DailyLimit != 5 {
		t.Fatalf("库里四个可改列应已更新，实际 %+v", stored)
	}
	if stored.LastNudgeAt == nil || !stored.LastNudgeAt.Equal(*before.LastNudgeAt) {
		t.Fatalf("库里 last_nudge_at 不得被 PUT 改动，PUT 前 %v、现在 %v", before.LastNudgeAt, stored.LastNudgeAt)
	}

	t.Run("别人的 personaId → 4043，且对方行分文未动", func(t *testing.T) {
		e := false
		a, b, c := 15, 90, 7
		_, err := svc.UpdateSettings(ctx, me.ID, &dto.UpdateSettingsRequest{
			PersonaID: theirs.ID, Enabled: &e, IntervalMin: &a, IntervalMax: &b, DailyLimit: &c,
		})
		assertBizCode(t, err, errcode.ErrPersonaNotFound)

		theirsStored, err := repo.Get(ctx, other.ID, theirs.ID)
		if err != nil {
			t.Fatalf("回读他人配置失败: %v", err)
		}
		if !theirsStored.Enabled || theirsStored.IntervalMin != 30 ||
			theirsStored.IntervalMax != 120 || theirsStored.DailyLimit != 3 {
			t.Fatalf("他人的配置不应被改动，实际 %+v", theirsStored)
		}
	})

	t.Run("不存在的 personaId → 4043（RowsAffected=0 即答案，不补查询）", func(t *testing.T) {
		e := true
		a, b, c := 60, 240, 3
		_, err := svc.UpdateSettings(ctx, me.ID, &dto.UpdateSettingsRequest{
			PersonaID: uint64(1) << 40, Enabled: &e, IntervalMin: &a, IntervalMax: &b, DailyLimit: &c,
		})
		assertBizCode(t, err, errcode.ErrPersonaNotFound)
	})
}

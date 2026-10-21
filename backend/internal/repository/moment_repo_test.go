// 本文件的测试连真库：+08:00 日界、边界包含关系与 user_id 双条件只有真实 SQL 才能验证。
// 隔离手段与 proactive_repo_test.go 相同：全部读写在一个事务里，结束回滚；连不上即 Skip。

package repository

import (
	"context"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

func seedMoment(t *testing.T, tx *gorm.DB, personaID uint64, createdAt time.Time) {
	t.Helper()
	m := &model.AIMoment{
		PersonaID: personaID,
		Content:   "测试动态",
		CreatedAt: createdAt,
	}
	if err := tx.Create(m).Error; err != nil {
		t.Fatalf("播种动态失败: %v", err)
	}
}

func TestCountTodayMomentsUsesCSTBoundaryAndUserScope(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	me := seedUser(t, tx, "moment_count_me")
	other := seedUser(t, tx, "moment_count_other")
	mine := seedPersona(t, tx, me.ID, "我的")
	theirs := seedPersona(t, tx, other.ID, "别人的")

	if err := tx.AutoMigrate(&model.AIMoment{}); err != nil {
		t.Fatalf("测试事务内 AutoMigrate ai_moments 失败: %v", err)
	}

	midnight := todayMidnight()
	now := time.Now().In(cstZone)

	seedMoment(t, tx, mine.ID, midnight)                   // 今日 00:00:00 整（下界含）
	seedMoment(t, tx, mine.ID, now)                        // 今日之内
	seedMoment(t, tx, mine.ID, midnight.Add(-time.Second)) // 昨日 23:59:59（不计）
	seedMoment(t, tx, theirs.ID, now)                      // 他人人设（不计）

	repo := NewMomentRepo(tx)

	got, err := repo.CountTodayMoments(ctx, me.ID, mine.ID)
	if err != nil {
		t.Fatalf("CountTodayMoments 出错: %v", err)
	}
	if got != 2 {
		t.Fatalf("应计入 2 条（00:00 整与今日内），实际 %d 条", got)
	}

	// 双条件：user_id 不匹配时，即使 persona_id 命中也不得计数
	cross, err := repo.CountTodayMoments(ctx, me.ID, theirs.ID)
	if err != nil {
		t.Fatalf("CountTodayMoments 出错: %v", err)
	}
	if cross != 0 {
		t.Fatalf("跨用户查询必须返回 0，实际 %d 条", cross)
	}
}

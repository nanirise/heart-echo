// 亲密度存在 personas.state JSONB 里，原子自增与封顶只有真实 SQL 能验证：
// 读出来 +1 再写回的并发覆盖在单测里很难复现，但 SQL 写法错了一眼就能看出来。
// 连库方式与隔离手段沿用 proactive_repo_test.go：整个测试在一个事务里、结束回滚，
// 连不上库即 Skip。

package repository

import (
	"context"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// readFamiliarity 返回 state.familiarity；第二个返回值表示键是否存在。
// 直接读 JSONB 而不是反序列化整个 model，避免测试和写入路径用同一段 Go 代码自证。
func readFamiliarity(t *testing.T, tx *gorm.DB, personaID uint64) (int, bool) {
	t.Helper()

	var rows []struct {
		Familiarity *int
	}
	err := tx.Raw(
		"SELECT (state->>'familiarity')::int AS familiarity FROM personas WHERE id = ?",
		personaID,
	).Scan(&rows).Error
	if err != nil {
		t.Fatalf("读取亲密度失败: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("应恰好读到 1 行，实际 %d 行", len(rows))
	}
	if rows[0].Familiarity == nil {
		return 0, false
	}
	return *rows[0].Familiarity, true
}

func TestIncrementFamiliarity(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	me := seedUser(t, tx, "familiarity_me")
	other := seedUser(t, tx, "familiarity_other")
	mine := seedPersona(t, tx, me.ID, "我的")
	theirs := seedPersona(t, tx, other.ID, "别人的")

	repo := NewPersonaRepo(tx)
	setState := func(personaID uint64, raw string) {
		t.Helper()
		if err := tx.Model(&model.Persona{}).Where("id = ?", personaID).
			Update("state", model.JSONB(raw)).Error; err != nil {
			t.Fatalf("写入 state 失败: %v", err)
		}
	}

	// ① 键缺失（DDL 默认 '{}'）：从 0 起算，写到 1
	if err := repo.IncrementFamiliarity(ctx, tx, me.ID, mine.ID); err != nil {
		t.Fatalf("累加亲密度失败: %v", err)
	}
	if got, ok := readFamiliarity(t, tx, mine.ID); !ok || got != 1 {
		t.Fatalf("键缺失时应从 0 加到 1，实际 familiarity=%d（存在=%v）", got, ok)
	}

	// ② 正常累加，且 state 里的其他键一个都不能丢
	setState(mine.ID, `{"familiarity": 41, "self_note": "别丢我"}`)
	if err := repo.IncrementFamiliarity(ctx, tx, me.ID, mine.ID); err != nil {
		t.Fatalf("累加亲密度失败: %v", err)
	}
	if got, _ := readFamiliarity(t, tx, mine.ID); got != 42 {
		t.Fatalf("41 应累加为 42，实际 %d", got)
	}
	var stateRows []struct {
		StateText string
	}
	if err := tx.Raw("SELECT state::text AS state_text FROM personas WHERE id = ?", mine.ID).
		Scan(&stateRows).Error; err != nil {
		t.Fatalf("回读 state 失败: %v", err)
	}
	if len(stateRows) != 1 || !strings.Contains(stateRows[0].StateText, `"self_note"`) {
		t.Fatalf("jsonb_set 不应丢掉其他键，实际 state=%v", stateRows)
	}

	// ③ 99 → 100
	setState(mine.ID, `{"familiarity": 99}`)
	if err := repo.IncrementFamiliarity(ctx, tx, me.ID, mine.ID); err != nil {
		t.Fatalf("累加亲密度失败: %v", err)
	}
	if got, _ := readFamiliarity(t, tx, mine.ID); got != 100 {
		t.Fatalf("99 应封顶到 100，实际 %d", got)
	}

	// ④ 已满 100：再调用仍是 100，不能越界
	if err := repo.IncrementFamiliarity(ctx, tx, me.ID, mine.ID); err != nil {
		t.Fatalf("累加亲密度失败: %v", err)
	}
	if got, _ := readFamiliarity(t, tx, mine.ID); got != 100 {
		t.Fatalf("100 应保持不变，实际 %d", got)
	}

	// ⑤ 越权：userID 与 personaID 不属于同一人时一行都不动
	if err := repo.IncrementFamiliarity(ctx, tx, me.ID, theirs.ID); err != nil {
		t.Fatalf("未命中行应返回 nil，实际: %v", err)
	}
	if got, ok := readFamiliarity(t, tx, theirs.ID); ok {
		t.Fatalf("他人的人设不应被更新，实际 familiarity=%d", got)
	}
}

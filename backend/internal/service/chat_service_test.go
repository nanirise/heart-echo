package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// TestParseFamiliarity 覆盖读 state 的所有兜底分支：
// 历史数据 / 别的模块写入的 state 都可能缺键或类型不对，不能让一次普通对话 500。
func TestParseFamiliarity(t *testing.T) {
	cases := []struct {
		name  string
		state string
		want  int
	}{
		{"键缺失按初识处理", `{}`, 0},
		{"正常值原样读出", `{"familiarity": 37}`, 37},
		{"负数收敛到 0", `{"familiarity": -5}`, 0},
		{"超过 100 收敛到 100", `{"familiarity": 250}`, 100},
		{"非数字退回 0", `{"familiarity": "high"}`, 0},
		{"坏 JSON 退回 0", `not-json`, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseFamiliarity(model.JSONB(tc.state)); got != tc.want {
				t.Fatalf("应解析为 %d，实际 %d", tc.want, got)
			}
		})
	}
}

// TestRelationshipStageBoundaries 钉住 TECH_DESIGN §5.6 的四档边界：
// 19/20、49/50、79/80 各错一档，说话语气就会整体错位，而编译与运行都不会报错。
func TestRelationshipStageBoundaries(t *testing.T) {
	cases := []struct {
		familiarity int
		wantStage   string
	}{
		{0, "初识"},
		{19, "初识"},
		{20, "熟悉"},
		{49, "熟悉"},
		{50, "亲近"},
		{79, "亲近"},
		{80, "默契"},
		{100, "默契"},
	}

	for _, tc := range cases {
		if got := relationshipStage(tc.familiarity); !strings.HasPrefix(got, tc.wantStage) {
			t.Fatalf("亲密度 %d 应属于「%s」，实际指令为 %q", tc.familiarity, tc.wantStage, got)
		}
	}
}

func TestSpeakingStyleWithStage(t *testing.T) {
	// 风格为空时只留阶段指令，且不能以换行开头（会被拼成 "说话风格：\n当前..."）
	empty := speakingStyleWithStage("", 0)
	if strings.HasPrefix(empty, "\n") || !strings.HasPrefix(empty, "当前关系阶段：初识") {
		t.Fatalf("空风格应只返回阶段指令，实际 %q", empty)
	}

	// 非空风格必须原样保留，并在下一行追加阶段；替换而不是追加会丢掉用户填的说话风格
	styled := speakingStyleWithStage("温柔", 80)
	if !strings.HasPrefix(styled, "温柔\n") || !strings.Contains(styled, "默契") {
		t.Fatalf("应保留原风格并追加阶段，实际 %q", styled)
	}
}

// TestIsValidMemoryType 钉住只有 fact / preference / event 能落库。
// LLM 可能返回 emotion 等非法值，必须丢弃而不是原样写入（user-memory spec §3.3）。
func TestIsValidMemoryType(t *testing.T) {
	for _, memoryType := range []model.MemoryType{
		model.MemoryTypeFact, model.MemoryTypePreference, model.MemoryTypeEvent,
	} {
		if !isValidMemoryType(memoryType) {
			t.Errorf("%q 应当是合法记忆类型", memoryType)
		}
	}
	for _, memoryType := range []model.MemoryType{"emotion", "FACT", ""} {
		if isValidMemoryType(memoryType) {
			t.Errorf("%q 不应是合法记忆类型", memoryType)
		}
	}
}

// TestNormalizeProfileUpdates 钉住空画像不触发 upsert：
// 空对象会把 updated_at 刷新，制造"画像刚更新过"的假信息（user-profile spec §4.6）。
func TestNormalizeProfileUpdates(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"字段缺失", "", ""},
		{"null", "null", ""},
		{"空对象", "{}", ""},
		{"有画像键", `{"occupation":"程序员"}`, `{"occupation":"程序员"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeProfileUpdates(json.RawMessage(tc.raw))
			if string(got) != tc.want {
				t.Errorf("应返回 %q，实际 %q", tc.want, got)
			}
		})
	}
}

package model

import (
	"encoding/json"
	"testing"
)

// TestAIMomentJSONKeys 锁死 AIMoment 对外的键集。
//
// 本表有两条静态检查验不了的约束，只有序列化后才看得见：
//   - emotion_label 是内部信号，绝不能进响应体（AGENTS §4.4）；
//   - personaName / liked / commentCount 是派生字段，一旦混进 model 会被 AutoMigrate 建成真列，
//     它们属于 dto.MomentItem，不属于这里。
//
// 所以断言用**恰好相等**而不是包含：多一个键也要报红。
func TestAIMomentJSONKeys(t *testing.T) {
	raw, err := json.Marshal(AIMoment{})
	if err != nil {
		t.Fatalf("序列化 AIMoment 失败：%v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("反序列化到 map 失败：%v", err)
	}

	want := map[string]bool{
		"id":        true,
		"personaId": true,
		"content":   true,
		"likeCount": true,
		"createdAt": true,
	}

	for key := range got {
		if !want[key] {
			t.Errorf("响应体多出字段 %q：是不是给内部信号或派生字段加了 json tag？（spec §1.2 / §3.5）", key)
		}
	}
	for key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("响应体缺少字段 %q", key)
		}
	}
}

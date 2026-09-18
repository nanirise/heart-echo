package model

import (
	"encoding/json"
	"testing"
)

// TestMomentCommentJSONKeys 锁死 MomentComment 对外的键集。
//
// 本表有一条静态检查验不了的约束，只有序列化后才看得见：
// authorName 是派生字段（COALESCE(personas.name, users.username)），
// 一旦混进 model 会被 AutoMigrate 建成 author_name 真列，它属于 dto.CommentItem。
//
// 所以断言用**恰好相等**而不是包含：多一个键也要报红。
func TestMomentCommentJSONKeys(t *testing.T) {
	raw, err := json.Marshal(MomentComment{})
	if err != nil {
		t.Fatalf("序列化 MomentComment 失败：%v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("反序列化到 map 失败：%v", err)
	}

	want := map[string]bool{
		"id":        true,
		"momentId":  true,
		"personaId": true,
		"userId":    true,
		"content":   true,
		"createdAt": true,
	}

	for key := range got {
		if !want[key] {
			t.Errorf("响应体多出字段 %q：是不是给派生字段加了 json tag？（spec §3 / §4.3 反例 5）", key)
		}
	}
	for key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("响应体缺少字段 %q", key)
		}
	}
}

// TestMomentCommentAuthorNullNotZero 验"二选一作者"在响应体里的分界：非作者的一侧必须序列化成 null。
//
// 值类型字段会把它变成 0，前端拿到 "userId": 0 会以为"用户 0 评论的"（契约 §8）。
// 构造值刻意走 JSON 往返而不是 struct 字面量：字面量里的 &id / nil 在字段被误改成值类型时
// 会让本文件先编译失败，报的是 compile error，就验不到"这条断言本身会不会红"（plan §3.3）。
func TestMomentCommentAuthorNullNotZero(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		absent  string // 非作者的一侧：必须是 null
		present string // 作者那一侧：必须有值
	}{
		{
			name:    "AI 评论",
			payload: `{"id":9,"momentId":3,"personaId":2,"userId":null,"content":"今天有点累","createdAt":"2026-09-18T10:00:00Z"}`,
			absent:  "userId",
			present: "personaId",
		},
		{
			name:    "用户评论",
			payload: `{"id":10,"momentId":3,"personaId":null,"userId":5,"content":"抱抱","createdAt":"2026-09-18T10:01:00Z"}`,
			absent:  "personaId",
			present: "userId",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var c MomentComment
			if err := json.Unmarshal([]byte(tc.payload), &c); err != nil {
				t.Fatalf("反序列化失败：%v", err)
			}

			raw, err := json.Marshal(c)
			if err != nil {
				t.Fatalf("序列化失败：%v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(raw, &got); err != nil {
				t.Fatalf("反序列化到 map 失败：%v", err)
			}

			v, ok := got[tc.absent]
			if !ok {
				t.Fatalf("响应体缺少字段 %q", tc.absent)
			}
			if v != nil {
				t.Errorf("非作者侧 %q 应为 null，实际是 %#v：字段被写成值类型了？（spec §1.2）", tc.absent, v)
			}

			if v, ok := got[tc.present]; !ok {
				t.Errorf("响应体缺少字段 %q", tc.present)
			} else if v == nil || v == float64(0) {
				t.Errorf("作者侧 %q 应有值，实际是 %#v", tc.present, v)
			}
		})
	}
}

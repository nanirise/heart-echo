package model

import (
	"encoding/json"
	"testing"
)

// TestMomentLikeJSONKeys 锁死 MomentLike 的序列化键集（恰好 4 个）。
//
// 本表**没有任何端点返回它的行**——契约里没有 Like 实体，点赞接口响应里的
// likeCount 读自 ai_moments、liked 是 LEFT JOIN 的存在性（spec §3.1）。
// 所以这条断言不是在**校验契约**，它是一道**结构探针**：
// 这些派生字段一旦混进 model，AutoMigrate 就会在本表建成真列，且不报错。
//
// 断言用**恰好相等**而不是包含：多一个键也要报红。
func TestMomentLikeJSONKeys(t *testing.T) {
	raw, err := json.Marshal(MomentLike{})
	if err != nil {
		t.Fatalf("序列化 MomentLike 失败：%v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("反序列化到 map 失败：%v", err)
	}

	want := map[string]bool{
		"id":        true,
		"userId":    true,
		"momentId":  true,
		"createdAt": true,
	}

	for key := range got {
		if !want[key] {
			t.Errorf("序列化结果多出字段 %q：它属于 dto.MomentItem 或别的表，"+
				"加进 model 会被 AutoMigrate 建成真列（spec §3.1）", key)
		}
	}
	for key := range want {
		if _, ok := got[key]; !ok {
			t.Errorf("序列化结果缺少字段 %q", key)
		}
	}
}

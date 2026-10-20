package dto

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// TestMemoryItemResponseJSONShape 钉住契约 §7 的 MemoryItem 恰好六个字段。
func TestMemoryItemResponseJSONShape(t *testing.T) {
	cst := time.FixedZone("CST", 8*60*60)
	raw, err := json.Marshal(NewMemoryItemResponse(&model.UserMemory{
		ID:              7,
		PersonaID:       1,
		MemoryType:      model.MemoryTypeFact,
		Content:         "用户养了一只叫豆豆的猫",
		ImportanceScore: 0.85,
		CreatedAt:       time.Date(2026, 9, 10, 14, 30, 0, 0, cst),
	}))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	const want = `{"id":7,"personaId":1,"memoryType":"fact","content":"用户养了一只叫豆豆的猫","importanceScore":0.85,"createdAt":"2026-09-10T14:30:00+08:00"}`
	if got := string(raw); got != want {
		t.Errorf("序列化结果与契约 §7 不符\n实际: %s\n期望: %s", got, want)
	}
}

// TestPortraitResponseJSONShape 钉住 Portrait 恰好三个字段，并覆盖两种空态陷阱：
// 没有画像行时 profileData 必须是 {} 而不是 null，updatedAt 必须是 null 而不是零值时间。
func TestPortraitResponseJSONShape(t *testing.T) {
	cst := time.FixedZone("CST", 8*60*60)
	updatedAt := time.Date(2026, 9, 10, 14, 30, 0, 0, cst)

	raw, err := json.Marshal(NewPortraitResponse(&model.UserProfile{
		PersonaID:   1,
		ProfileData: model.JSONB(`{"occupation":"程序员"}`),
		UpdatedAt:   updatedAt,
	}, 1))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	const want = `{"personaId":1,"profileData":{"occupation":"程序员"},"updatedAt":"2026-09-10T14:30:00+08:00"}`
	if got := string(raw); got != want {
		t.Errorf("有画像行的序列化结果与契约 §7 不符\n实际: %s\n期望: %s", got, want)
	}

	raw, err = json.Marshal(NewPortraitResponse(nil, 1))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	const wantEmpty = `{"personaId":1,"profileData":{},"updatedAt":null}`
	if got := string(raw); got != wantEmpty {
		t.Errorf("空画像的序列化结果与契约 §7 不符\n实际: %s\n期望: %s", got, wantEmpty)
	}
}

// TestMemoryQueriesBinding 钉住两个 query 的 form tag 是 camelCase personaId。
// tag 写错不会编译失败：ShouldBindQuery 绑不上时字段留在 0，被 required 拦成 4001，
// 与"真的没传"完全一样，极难排查。
func TestMemoryQueriesBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{"契约 §7 的 personaId", "?personaId=7", false},
		{"缺 personaId", "", true},
		{"personaId=0 视同缺失", "?personaId=0", true},
		{"字段名写成 persona_id", "?persona_id=7", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/memory"+tc.query, nil)

			var memoryQuery MemoryListQuery
			memoryErr := c.ShouldBindQuery(&memoryQuery)
			var portraitQuery PortraitQuery
			portraitErr := c.ShouldBindQuery(&portraitQuery)

			if tc.wantErr {
				if memoryErr == nil {
					t.Error("MemoryListQuery 应当绑定失败")
				}
				if portraitErr == nil {
					t.Error("PortraitQuery 应当绑定失败")
				}
				return
			}
			if memoryErr != nil {
				t.Fatalf("MemoryListQuery 绑定失败: %v", memoryErr)
			}
			if portraitErr != nil {
				t.Fatalf("PortraitQuery 绑定失败: %v", portraitErr)
			}
			if memoryQuery.PersonaID != 7 || portraitQuery.PersonaID != 7 {
				t.Errorf("personaId 应为 7，实际 memory=%d portrait=%d",
					memoryQuery.PersonaID, portraitQuery.PersonaID)
			}
		})
	}
}

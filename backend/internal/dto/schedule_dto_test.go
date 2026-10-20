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

// TestScheduleResponseJSONShape 钉住契约 §10 的 Schedule 恰好六个字段。
//
// 逐字比对整段 JSON，一次钉住字段名、数量、顺序，以及 remindAt 是
// RFC3339 带时区（契约要求绝对时间）。userId / sourceMessageId 一旦被误加进
// 响应体，这条会立刻红。
func TestScheduleResponseJSONShape(t *testing.T) {
	cst := time.FixedZone("CST", 8*60*60)
	raw, err := json.Marshal(NewScheduleResponse(&model.Schedule{
		ID:        5,
		PersonaID: 1,
		Content:   "开会",
		RemindAt:  time.Date(2026, 9, 11, 15, 0, 0, 0, cst),
		Status:    model.ScheduleStatusPending,
		CreatedAt: time.Date(2026, 9, 10, 20, 12, 0, 0, cst),
	}))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	const want = `{"id":5,"personaId":1,"content":"开会","remindAt":"2026-09-11T15:00:00+08:00","status":"pending","createdAt":"2026-09-10T20:12:00+08:00"}`
	if got := string(raw); got != want {
		t.Errorf("序列化结果与契约 §10 不符\n实际: %s\n期望: %s", got, want)
	}
}

// TestListSchedulesQueryBinding 钉住 form tag 的 camelCase。
//
// form tag 写错不会编译失败：ShouldBindQuery 绑不上 personaId 时字段留在 0，
// 紧接着被 required 拦下，前端看到的是 4001 —— 与"真的没传"完全一样。
func TestListSchedulesQueryBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name    string
		query   string
		wantID  uint64
		wantErr bool
	}{
		{"契约 §10 的 personaId", "?personaId=7", 7, false},
		{"缺 personaId", "", 0, true},
		{"personaId=0 视同缺失", "?personaId=0", 0, true},
		{"字段名写成 persona_id", "?persona_id=7", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodGet, "/schedules"+tc.query, nil)

			var query ListSchedulesQuery
			err := c.ShouldBindQuery(&query)

			if tc.wantErr {
				if err == nil {
					t.Fatalf("应当绑定失败，实际 personaId=%d", query.PersonaID)
				}
				return
			}
			if err != nil {
				t.Fatalf("绑定失败: %v", err)
			}
			if query.PersonaID != tc.wantID {
				t.Errorf("personaId 应为 %d，实际 %d", tc.wantID, query.PersonaID)
			}
		})
	}
}

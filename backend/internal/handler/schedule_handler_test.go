// 覆盖 schedule_handler 的路由形状、请求绑定与错误码分流。
//
// 与 proactive_handler_test.go 同理：日程路由要等 router.go 装配后才进全景引擎，
// 本文件先用同构最小链（BizErrorHandler → JWTAuth）钉住 handler 自己的行为。
//
// 库连不上是预期内的：断言分得清"绑定没过"（4001/4040）与"绑过去了、停在库上"（5003）
// —— 后者正是"请求被正确放行"的证据。
package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/middleware"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// newScheduleTestRouter 把日程三条路由挂进最小链。
// chat 传 nil：本文件的用例在归属查询/绑定层就停下，不触碰 InjectNudge。
func newScheduleTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BizErrorHandler(zap.NewNop()))
	protected := r.Group("/api/v1")
	protected.Use(middleware.JWTAuth(testSecret))
	RegisterScheduleRoutes(protected, NewScheduleHandler(service.NewScheduleService(db, nil)))
	return r
}

func TestScheduleListBinding(t *testing.T) {
	r := newScheduleTestRouter(t, testDB(t))
	auth := "Bearer " + mustPair(t, testSecret, time.Hour).AccessToken

	cases := []struct {
		name string
		path string
		want errcode.ErrorCode
	}{
		{"缺 personaId", "/api/v1/schedules", errcode.ErrInvalidParams},
		{"personaId=0", "/api/v1/schedules?personaId=0", errcode.ErrInvalidParams},
		{"personaId 非数字", "/api/v1/schedules?personaId=abc", errcode.ErrInvalidParams},
		// form tag 是 personaId、大小写敏感；写错时前端只会看到 4001，不会编译报错
		{"persona_id 不会命中 form tag", "/api/v1/schedules?persona_id=1", errcode.ErrInvalidParams},
		// 5003 = 绑定全过了（死库）；缺这条的话，"把所有请求都拒掉"的实现也能全绿
		{"personaId 合法 → 越过绑定停在库上", "/api/v1/schedules?personaId=1", errcode.ErrDBFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, r, http.MethodGet, tc.path, auth)
			if rec.Code == http.StatusNotFound {
				t.Fatalf("路由没挂上：GET %s 返回 404（应在 RegisterScheduleRoutes 里注册）", tc.path)
			}
			if got := decodeBody(t, rec).Code; got != int(tc.want) {
				t.Fatalf("响应 code 应为 %d（%s），实际 %d", tc.want, tc.want.Message(), got)
			}
		})
	}
}

func TestScheduleCancelAndTriggerIDParsing(t *testing.T) {
	r := newScheduleTestRouter(t, testDB(t))
	auth := "Bearer " + mustPair(t, testSecret, time.Hour).AccessToken

	cases := []struct {
		name   string
		method string
		path   string
		want   errcode.ErrorCode
	}{
		// 契约 §10：这组端点的资源级失败只有 4040，非数字 / 0 不暴露解析细节
		{"DELETE id 非数字 → 4040", http.MethodDelete, "/api/v1/schedules/abc", errcode.ErrNotFound},
		{"DELETE id=0 → 4040", http.MethodDelete, "/api/v1/schedules/0", errcode.ErrNotFound},
		{"DELETE id 合法 → 越过解析停在库上", http.MethodDelete, "/api/v1/schedules/1", errcode.ErrDBFailed},
		{"trigger id 非数字 → 4040", http.MethodPost, "/api/v1/schedules/abc/trigger", errcode.ErrNotFound},
		{"trigger id=0 → 4040", http.MethodPost, "/api/v1/schedules/0/trigger", errcode.ErrNotFound},
		{"trigger id 合法 → 越过解析停在库上", http.MethodPost, "/api/v1/schedules/1/trigger", errcode.ErrDBFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, r, tc.method, tc.path, auth)
			if rec.Code == http.StatusNotFound && tc.want == errcode.ErrDBFailed {
				t.Fatalf("路由没挂上：%s %s 返回 404（应在 RegisterScheduleRoutes 里注册）",
					tc.method, tc.path)
			}
			if got := decodeBody(t, rec).Code; got != int(tc.want) {
				t.Fatalf("响应 code 应为 %d（%s），实际 %d", tc.want, tc.want.Message(), got)
			}
		})
	}
}

// TestNoCreateScheduleRoute 钉住"日程只能从对话抽取产生"这条功能决策。
//
// POST /schedules 一旦被顺手加上，用户就能手填一条没有对话上下文的日程；
// 契约 §10 与 AGENTS §7.5 都明确禁止。这条用例是唯一的自动拦截点。
func TestNoCreateScheduleRoute(t *testing.T) {
	r := newScheduleTestRouter(t, testDB(t))
	auth := "Bearer " + mustPair(t, testSecret, time.Hour).AccessToken

	for _, path := range []string{"/api/v1/schedules", "/api/v1/schedules/1"} {
		rec := doRequest(t, r, http.MethodPost, path, auth)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("POST %s 应为 404（契约 §10 禁止手动建日程），实际 %d", path, rec.Code)
		}
	}
}

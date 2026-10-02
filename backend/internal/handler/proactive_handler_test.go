// 覆盖 proactive_handler 的请求绑定与上抛：GET 的 query 绑定（form tag 大小写敏感）、
// PUT 的指针 DTO（enabled=false 必须被放行，否则开关永远关不掉）。
//
// 没有用 newTestRouter：这两条路由要到成员 1 在 router.go 装配后才会出现在全景引擎里。
// 本文件用与 router.go 同构的最小链（BizErrorHandler → JWTAuth）先钉住 handler 自己的行为；
// 装配完成后这两条可以整体搬进 newTestRouter 系测试，本文件随之删除或改写。
//
// 库连不上是预期内的：断言分得清"绑定没过"（4001）与"绑过去了、停在库上"（5003）——
// 后者正是"请求被正确放行"的证据。
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

// newProactiveTestRouter 把 proactive 两条路由挂进一条最小中间件链。
// BizErrorHandler 与 JWTAuth 的顺序照 router.go：前者包全链、后者只护 protected 组。
func newProactiveTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.BizErrorHandler(zap.NewNop()))
	protected := r.Group("/api/v1")
	protected.Use(middleware.JWTAuth(testSecret))
	RegisterProactiveRoutes(protected, NewProactiveHandler(service.NewProactiveService(db)))
	return r
}

func TestProactiveSettingsGetBinding(t *testing.T) {
	r := newProactiveTestRouter(t, testDB(t))
	// 令牌合法即可：两条路由只验签，不看库
	auth := "Bearer " + mustPair(t, testSecret, time.Hour).AccessToken

	cases := []struct {
		name string
		path string
		want errcode.ErrorCode
	}{
		{"缺 personaId", "/api/v1/proactive/settings", errcode.ErrInvalidParams},
		{"personaId=0", "/api/v1/proactive/settings?personaId=0", errcode.ErrInvalidParams},
		{"personaId 非数字", "/api/v1/proactive/settings?personaId=abc", errcode.ErrInvalidParams},
		// form tag 是 personaId、大小写敏感：写成下划线会静默绑不上，
		// 若哪天 tag 被改错，这条会是唯一变红的用例
		{"persona_id 不会命中 form tag", "/api/v1/proactive/settings?persona_id=1", errcode.ErrInvalidParams},
		// 5003 = 绑定全过了（死库）；缺这条的话，"把所有请求都拒掉"的实现也能全绿
		{"personaId 合法 → 越过绑定停在库上", "/api/v1/proactive/settings?personaId=1", errcode.ErrDBFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, r, http.MethodGet, tc.path, auth)
			if rec.Code == http.StatusNotFound {
				t.Fatalf("路由没挂上：GET %s 返回 404（应在 RegisterProactiveRoutes 里注册）", tc.path)
			}
			if got := decodeBody(t, rec).Code; got != int(tc.want) {
				t.Fatalf("响应 code 应为 %d（%s），实际 %d", tc.want, tc.want.Message(), got)
			}
		})
	}
}

func TestProactiveSettingsUpdateBinding(t *testing.T) {
	r := newProactiveTestRouter(t, testDB(t))
	auth := "Bearer " + mustPair(t, testSecret, time.Hour).AccessToken

	cases := []struct {
		name string
		body string
		want errcode.ErrorCode
	}{
		{"空对象", `{}`, errcode.ErrInvalidParams},
		{"缺 personaId", `{"enabled":true,"intervalMin":60,"intervalMax":240,"dailyLimit":3}`, errcode.ErrInvalidParams},
		{"缺 enabled（字段缺席）", `{"personaId":1,"intervalMin":60,"intervalMax":240,"dailyLimit":3}`, errcode.ErrInvalidParams},
		{"enabled 显式为 null", `{"personaId":1,"enabled":null,"intervalMin":60,"intervalMax":240,"dailyLimit":3}`, errcode.ErrInvalidParams},
		{"缺 dailyLimit", `{"personaId":1,"enabled":true,"intervalMin":60,"intervalMax":240}`, errcode.ErrInvalidParams},
		// 指针 DTO 存在的全部理由：enabled=false 是有意义的输入。
		// 若哪天 DTO 退回值类型，false 会被 required 判成"没传"→ 4001，恰好只有这条变红。
		{"enabled=false 全字段合法 → 越过绑定停在库上", `{"personaId":1,"enabled":false,"intervalMin":60,"intervalMax":240,"dailyLimit":3}`, errcode.ErrDBFailed},
		{"enabled=true 全字段合法 → 同样越过绑定", `{"personaId":1,"enabled":true,"intervalMin":60,"intervalMax":240,"dailyLimit":3}`, errcode.ErrDBFailed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequestWithBody(t, r, http.MethodPut, "/api/v1/proactive/settings", auth, tc.body)
			if rec.Code == http.StatusNotFound {
				t.Fatal("路由没挂上：PUT /api/v1/proactive/settings 返回 404（应在 RegisterProactiveRoutes 里注册）")
			}
			if got := decodeBody(t, rec).Code; got != int(tc.want) {
				t.Fatalf("响应 code 应为 %d（%s），实际 %d", tc.want, tc.want.Message(), got)
			}
		})
	}
}

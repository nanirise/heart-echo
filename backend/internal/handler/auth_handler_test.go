// 覆盖 auth_handler 的**请求体处理**，走完整路由（含中间件链）但不连数据库。
//
// 与 router_test.go 的分工：那个文件管"路由挂上了没有、挂在哪个组上"，
// 这个文件管"handler 拿到请求之后怎么解析"。两者都用 newTestRouter，
// 因为它返回的就是真实装配出来的引擎 —— 重搭一条链等于什么都没测。
//
// 库里连不上是预期内的：这里的断言分得清"校验没过"和"过了校验但读不到数据"，
// 后者反而是"请求体被正确放行"的证据。
package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// doRequestWithBody 与 doRequest 相同，只是带请求体。
func doRequestWithBody(
	t *testing.T, r *gin.Engine, method, path, authHeader, body string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// TestUpdateProfileRejectsUnknownField 钉住 PUT /user/profile 的严格解码。
//
// 这个端点是全项目唯一一个"部分更新"接口：两个字段都可选，所以"什么都没传"
// 是一个合法请求。加上默认的宽松解码，下面两种情况会得到**完全相同的响应**：
//   - 调用方发 {} —— 什么都没想改，200 正确
//   - 调用方把 username 拼成 user_name —— 字段被静默丢弃，什么都不改，也是 200
//
// 第二种人拿到的是"保存成功"，只能自己去猜哪里不对。这条测试让两者分开。
//
// 用例分两类，缺一不可（两条注入验过）：
//   - 前两个是"该被拒"：不认识的字段名 → 4001。
//     去掉 bindStrictJSON 里的 DisallowUnknownFields，恰好是这两条变红。
//   - 后三个是"该放行"：合法请求体必须照常走到数据库那一步。
//     没有它们的话，一个"把所有请求都拒掉"的实现也能让前两条通过。
//     其中"太短的用户名"那条另有用处：去掉 ValidateStruct，恰好只有它变红。
func TestUpdateProfileRejectsUnknownField(t *testing.T) {
	r := newTestRouter(t)
	// 令牌合法即可，userID 与实际用户无关：这个路由组只验签，不查库找人
	auth := "Bearer " + mustPair(t, testSecret, time.Hour).AccessToken

	// 走到数据库那一步就返回 5003（测试里的 *gorm.DB 指向一个必然拒绝连接的端口）。
	// 所以 5003 = "校验全过了"，用它来证明请求体确实被放行。
	const passedValidation = errcode.ErrDBFailed

	cases := []struct {
		name string
		body string
		want errcode.ErrorCode
	}{
		{
			"字段名拼错（user_name 而不是 username）",
			`{"user_name":"newname"}`,
			errcode.ErrInvalidParams,
		},
		{
			// 前端很常见的写法：store 里就有 user 对象，整个 PUT 上来。
			// 这会把 id / email / createdAt 一起带上，严格模式下全是"不认识的字段"。
			// 契约 §3.5 为此专门写了警示。
			"把整个 user 对象 PUT 上来",
			`{"id":1,"username":"newname","email":"x@x.com","avatarUrl":null,"createdAt":"2026-09-10T14:30:00+08:00"}`,
			errcode.ErrInvalidParams,
		},
		{
			"只改用户名（合法）",
			`{"username":"newname"}`,
			passedValidation,
		},
		{
			// 这条同时钉住另一件事：严格解码之后 binding tag 仍在跑。
			// 若 bindStrictJSON 漏了 ValidateStruct 这一步，min=3 不再生效，
			// 这里会变成 5003 而不是 4001。
			"太短的用户名（校验仍在跑）",
			`{"username":"ab"}`,
			errcode.ErrInvalidParams,
		},
		{
			"清空头像，null 是契约允许的三态之一（合法）",
			`{"avatarUrl":null}`,
			passedValidation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequestWithBody(t, r, http.MethodPut, "/api/v1/user/profile", auth, tc.body)

			if rec.Code == http.StatusNotFound {
				t.Fatalf("路由没挂上: PUT /api/v1/user/profile 返回 404")
			}
			body := decodeBody(t, rec)
			if errcode.ErrorCode(body.Code) != tc.want {
				t.Errorf(
					"错误码应为 %d（%s），实际 %d（%s）\n请求体 %s\n响应体 %s",
					tc.want, tc.want.Message(), body.Code, errcode.ErrorCode(body.Code).Message(),
					tc.body, rec.Body.String(),
				)
			}
		})
	}
}

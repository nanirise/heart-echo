// 覆盖 JWTAuth 的三条失败路径（4010 / 4012 / 4011）与放行路径。
//
// 重点不在"状态码对不对"，而在两件肉眼看不出来的事：
//   - 失败时业务 handler 有没有被执行（漏写 c.Abort() 时它会照跑不误）
//   - 放行后写进 Context 的 userId 类型对不对（类型错了 GetUint64 静默返回 0）
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/jwt"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

const testSecret = "test_secret_at_least_32_chars_long!!"

// newTestEngine 造一个与 router.go 同构的最小引擎：
// BizErrorHandler 在前、JWTAuth 在后，顺序必须与 internal/handler/router.go 一致。
//
// 不能只挂 JWTAuth：它失败时只做 c.Error() + c.Abort()，一个字节的响应都不写，
// 响应由 BizErrorHandler 出口。只挂一个的话 c.Errors 没人消费，
// 跑出来永远是 HTTP 200 + 空 body，断言什么都是错的。
//
// 反过来说，这条链照抄 router.go，顺带就成了装配顺序的回归测试：
// 谁把两者顺序调换，错误会在 BizErrorHandler 跑完之后才写进 c.Errors，
// 再也没人读它，接口返回空响应。
//
// hit 由业务 handler 置位，用来证明"鉴权失败时它一次都没被调用"。
func newTestEngine(secret string, hit *bool) *gin.Engine {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(BizErrorHandler(zap.NewNop()))

	protected := r.Group("/protected")
	protected.Use(JWTAuth(secret))
	protected.GET("/ping", func(c *gin.Context) {
		*hit = true
		// 把取回来的值写进响应体。取法必须与 persona_handler 的 currentUserID
		// 一致（GetUint64）—— 类型写错时它静默返回 0，接口照样 200，
		// 只有回读到具体的值才能发现。
		c.String(http.StatusOK, "userID=%d username=%s",
			c.GetUint64(ContextKeyUserID), c.GetString(ContextKeyUsername))
	})
	return r
}

// mustPair 签发一对令牌。accessTTL 传负数 = 签出来的那一刻就已过期。
func mustPair(t *testing.T, secret string, accessTTL time.Duration) *jwt.TokenPair {
	t.Helper()

	pair, err := jwt.GenerateTokenPair(7, "alice", secret, accessTTL, 168*time.Hour)
	if err != nil {
		t.Fatalf("签发令牌失败: %v", err)
	}
	return pair
}

// doRequest 发一次请求。authHeader 为空串时表示不带 Authorization 头。
func doRequest(t *testing.T, engine *gin.Engine, authHeader string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/protected/ping", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

// tamperSignature 把签名段的首个字符换成一个不同的字符，模拟"签名被改过"。
//
// 不能改最后一个字符：HS256 签名 32 字节，base64url 编码 43 字符，末位字符
// 只有高 4 位是有效数据，低 2 位是填充位、解码时被丢弃。字母表里 U(20)=010100
// 与 X(23)=010111 高 4 位相同，末位恰好是 U 时（概率 1/16）把 U 改成 X，
// 解出的 32 字节一比特不差，签名依然有效 —— 断言会随机失败。
// 首字符的 6 位全是有效数据，改它必然改变签名字节；换成的字符也不能撞上原字符。
//
// pkg/jwt 的 jwt_test.go 里有一份同样的 helper（两个包互不引用）。
func tamperSignature(t *testing.T, token string) string {
	t.Helper()

	dot := strings.LastIndex(token, ".")
	if dot < 0 || dot+1 >= len(token) {
		t.Fatalf("令牌不是 header.payload.signature 形式，取不到签名段: %q", token)
	}

	repl := byte('A')
	if token[dot+1] == repl {
		repl = 'B'
	}
	return token[:dot+1] + string(repl) + token[dot+2:]
}

// decodeFail 解析失败响应体
func decodeFail(t *testing.T, rec *httptest.ResponseRecorder) response.Response[any] {
	t.Helper()

	var body response.Response[any]
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v，原文 %q", err, rec.Body.String())
	}
	return body
}

func TestJWTAuthRejects(t *testing.T) {
	valid := mustPair(t, testSecret, 2*time.Hour)
	expired := mustPair(t, testSecret, -time.Hour)
	otherKey := mustPair(t, "another_secret_at_least_32_chars!", 2*time.Hour)
	tampered := tamperSignature(t, valid.AccessToken)

	cases := []struct {
		name   string
		header string
		want   errcode.ErrorCode
	}{
		{"没带 Authorization 头", "", errcode.ErrUnauthorized},
		{"方案名不是 Bearer", "Basic " + valid.AccessToken, errcode.ErrUnauthorized},
		{"没有 Bearer 前缀，直接给令牌", valid.AccessToken, errcode.ErrUnauthorized},
		// 前缀后面是空的：不算"未登录"，算"给了个无效的令牌"。
		// 两条路径前端都是登出，区别只在排查时看日志的语义。
		{"只有 Bearer 前缀，后面是空的", "Bearer ", errcode.ErrTokenInvalid},
		{"access token 已过期", "Bearer " + expired.AccessToken, errcode.ErrTokenExpired},
		{"拿 refresh token 当 access 用", "Bearer " + valid.RefreshToken, errcode.ErrTokenInvalid},
		{"签名被篡改", "Bearer " + tampered, errcode.ErrTokenInvalid},
		{"换了个密钥签的", "Bearer " + otherKey.AccessToken, errcode.ErrTokenInvalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hit bool
			rec := doRequest(t, newTestEngine(testSecret, &hit), tc.header)

			// 这条先断言：handler 只要跑过，鉴权就是彻底失效，
			// 后面状态码和文案对不对都无关紧要了。
			if hit {
				t.Fatal("鉴权失败，业务 handler 却被执行了（漏了 c.Abort()）")
			}
			if got, want := rec.Code, tc.want.HTTPStatus(); got != want {
				t.Errorf("HTTP 状态码应为 %d，实际 %d", want, got)
			}

			body := decodeFail(t, rec)
			if body.Code != int(tc.want) {
				t.Errorf("业务码应为 %d，实际 %d", tc.want, body.Code)
			}
			// 文案必须与 errcode 表逐字一致。Fail 在类型上就不接受自定义 message，
			// 这里断言的是"没有人在半路上另写了一句提示语"。
			if got, want := body.Message, tc.want.Message(); got != want {
				t.Errorf("文案应为 %q，实际 %q", want, got)
			}
		})
	}
}

func TestJWTAuthAccepts(t *testing.T) {
	valid := mustPair(t, testSecret, 2*time.Hour)

	cases := []struct {
		name   string
		header string
	}{
		{"标准写法", "Bearer " + valid.AccessToken},
		// TrimSpace 的作用：前缀后多几个空格也认
		{"前缀后有多个空格", "Bearer     " + valid.AccessToken},
	}

	// userID 是 7、username 是 alice，与 mustPair 里签发时传的一致。
	// 这两个值能从响应体里读回来，才说明 Context 真的被写对了。
	const want = "userID=7 username=alice"

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var hit bool
			rec := doRequest(t, newTestEngine(testSecret, &hit), tc.header)

			if !hit {
				t.Fatal("合法令牌被拦下了，业务 handler 没有执行")
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("HTTP 状态码应为 200，实际 %d，响应体 %q", rec.Code, rec.Body.String())
			}
			if got := rec.Body.String(); got != want {
				t.Errorf("Context 未被正确写入，期望 %q，实际 %q", want, got)
			}
		})
	}
}

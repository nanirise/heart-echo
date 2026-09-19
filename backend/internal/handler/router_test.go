// 覆盖 router.go 的装配结果 —— 全程不连数据库。
//
// 三件在代码里看不出来的事，只能靠断言：
//   - persona 的 4 条路由挂上了没有、路径写错没有
//   - protected 组上到底有没有 JWTAuth（与上一条是两回事：handler 自己也返 4010，
//     "匿名请求被拒" 区分不出来。见各自测试的注释）
//   - /health 有没有被误加上鉴权（它在白名单里，多一道 JWTAuth 会让 healthcheck 全线失败）
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/jwt"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

const testSecret = "test_secret_at_least_32_chars_long!!"

// testDB 返回一个**不连库**的 *gorm.DB。
//
// DisableAutomaticPing 让 gorm.Open 跳过建连时的 Ping，于是 /health 里的 db.DB()
// 拿到的是非 nil 的 *sql.DB、Ping 失败后返回 "down" —— 而不是在 nil 指针上 panic。
// 不这么做就只能整段跳过 /health，白名单被误加鉴权时没人发现。
func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		postgres.Open("host=127.0.0.1 port=1 user=x password=x dbname=x sslmode=disable"),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("构造测试用 *gorm.DB 失败: %v", err)
	}
	return db
}

// newTestRouter 跑一遍完整的 NewRouter，只把 cfg 与 db 换成不依赖环境变量的替身。
//
// 刻意不复刻中间件链：被测对象就是装配本身，"测试里重搭一条链" 等于什么都没测。
func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		// 不能留空：gin-contrib/cors 收到空的 origin 列表会直接 panic
		//（all origins disabled），生产靠 .env 的 envDefault 兜着。
		Server: config.ServerConfig{CORSAllowOrigins: []string{"http://localhost:5173"}},
		JWT:    config.JWTConfig{Secret: testSecret},
		// 指向一个必然拒绝连接的端口：/health 里探 ai-service 会立刻失败，
		// 不必干等满 2 秒超时。
		AI: config.AIConfig{ServiceURL: "http://127.0.0.1:1"},
	}
	return NewRouter(cfg, zap.NewNop(), testDB(t))
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

// tamperSignature 把签名段的首个字符换掉，模拟 "签名被改过"。
//
// 不能改最后一个字符：HS256 签名 32 字节、base64url 编码 43 字符，末位只有高 4 位
// 是有效数据，低 2 位是填充位、解码时被丢弃。末位恰好落在填充位相同的字符对上时
// （概率 1/16）解出的签名字节一比特不差，断言会随机失败。
//
// pkg/jwt 与 internal/middleware 的测试里各有一份同样的 helper（各包互不引用）。
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

// doRequest 发一次请求。authHeader 为空串表示不带 Authorization 头。
func doRequest(
	t *testing.T, r *gin.Engine, method, path, authHeader string,
) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

// decodeBody 把响应体解成统一结构的失败响应。
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) response.Response[any] {
	t.Helper()

	var body response.Response[any]
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v，原文 %q", err, rec.Body.String())
	}
	return body
}

// personaRoutes 是 persona 的 4 条路由，:id 换成具体值用来发请求。
var personaRoutes = []struct {
	name   string
	method string
	path   string
}{
	{"列表", http.MethodGet, "/api/v1/personas"},
	{"创建", http.MethodPost, "/api/v1/personas"},
	{"更新", http.MethodPut, "/api/v1/personas/1"},
	{"删除", http.MethodDelete, "/api/v1/personas/1"},
}

// TestPersonaRoutesRejectAnonymousRequests 逐条打一遍：不带 token 必须全部 4010。
//
// ⚠️ 这条只证「挂上了 + 路径没写错、而且不是 404」，**证不了鉴权在不在链上**：
// persona_handler 的 currentUserID 取不到 userID 时自己也返 4010，把挂载点从
// protected 挪到 api 它照样绿 —— 反向注入实测过。
//
// 鉴权是否真挂上了，由另外两条负责：TestProtectedRoutesRejectBadTokens 里的
// 4011 / 4012 是 handler 自己产不出来的码；TestValidTokenPassesJWTAuth 则要求
// 合法令牌能越过中间件、把 userID 写进 Context。
func TestPersonaRoutesRejectAnonymousRequests(t *testing.T) {
	r := newTestRouter(t)

	for _, route := range personaRoutes {
		t.Run(route.name, func(t *testing.T) {
			rec := doRequest(t, r, route.method, route.path, "")

			if got, want := rec.Code, errcode.ErrUnauthorized.HTTPStatus(); got != want {
				t.Fatalf("HTTP 状态码应为 %d，实际 %d，响应体 %q", want, got, rec.Body.String())
			}
			if got := decodeBody(t, rec).Code; got != int(errcode.ErrUnauthorized) {
				t.Errorf("业务码应为 %d，实际 %d", errcode.ErrUnauthorized, got)
			}
		})
	}
}

// TestProtectedRoutesRejectBadTokens 覆盖 4011 / 4012 两条失败路径与响应体结构。
//
// 在真路由上验，不是在中间件的测试引擎上：中间件单独过测、却漏挂到路由组上，
// 是这类改动最容易留下的缺口。
func TestProtectedRoutesRejectBadTokens(t *testing.T) {
	r := newTestRouter(t)
	valid := mustPair(t, testSecret, 2*time.Hour)
	expired := mustPair(t, testSecret, -time.Hour)
	otherKey := mustPair(t, "another_secret_at_least_32_chars!", 2*time.Hour)

	cases := []struct {
		name   string
		header string
		want   errcode.ErrorCode
	}{
		{"方案名不是 Bearer", "Basic " + valid.AccessToken, errcode.ErrUnauthorized},
		{"access token 已过期", "Bearer " + expired.AccessToken, errcode.ErrTokenExpired},
		{"拿 refresh token 当 access 用", "Bearer " + valid.RefreshToken, errcode.ErrTokenInvalid},
		{"签名被篡改", "Bearer " + tamperSignature(t, valid.AccessToken), errcode.ErrTokenInvalid},
		{"换了个密钥签的", "Bearer " + otherKey.AccessToken, errcode.ErrTokenInvalid},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doRequest(t, r, http.MethodGet, "/api/v1/personas", tc.header)

			if got, want := rec.Code, tc.want.HTTPStatus(); got != want {
				t.Fatalf("HTTP 状态码应为 %d，实际 %d，响应体 %q", want, got, rec.Body.String())
			}

			body := decodeBody(t, rec)
			if body.Code != int(tc.want) {
				t.Errorf("业务码应为 %d，实际 %d", tc.want, body.Code)
			}
			// 文案必须与错误码表逐字一致：Fail 在类型上就不接受自定义 message，
			// 这里断言的是没人另写了一句提示语绕过它。
			if got, want := body.Message, tc.want.Message(); got != want {
				t.Errorf("文案应为 %q，实际 %q", want, got)
			}
			// 四段结构缺一不可：少了 timestamp，前端按契约解析会拿到 undefined
			if body.Timestamp == 0 {
				t.Error("timestamp 为 0，响应体似乎不是统一结构")
			}
		})
	}
}

// TestValidTokenPassesJWTAuth 确认 JWTAuth 真的挂在这组路由上。
//
// 判据是"不是 4010"，而不是"请求成功" —— 测试里没有 PostgreSQL，越过鉴权后
// 必然停在 DB 那一步。这正是这条断言的价值所在：把 JWTAuth 摘掉，Context 就没人写，
// 请求会退化到 handler 自己的 4010 防线，与"鉴权生效"看起来一模一样。
func TestValidTokenPassesJWTAuth(t *testing.T) {
	r := newTestRouter(t)
	valid := mustPair(t, testSecret, 2*time.Hour)

	rec := doRequest(t, r, http.MethodGet, "/api/v1/personas", "Bearer "+valid.AccessToken)
	body := decodeBody(t, rec)

	if body.Code == int(errcode.ErrUnauthorized) {
		t.Fatalf("合法 access token 拿到了 4010 —— JWTAuth 没挂在这条路由上，响应体 %q", rec.Body.String())
	}
	if want := int(errcode.ErrDBFailed); body.Code != want {
		t.Errorf("越过鉴权后应停在数据库失败 %d，实际 %d，响应体 %q", want, body.Code, rec.Body.String())
	}
}

// TestHealthIsNotBehindJWTAuth 确认白名单没被误伤。
//
// 依赖连不上是预期内的（测试里没有 PostgreSQL、没有 ai-service），所以这里不断言
// data.status 是 "ok"，只断言它没被鉴权拦下、且仍是 HTTP 200 —— 契约 §11 明写
// "依赖异常时仍返回 200，状态由 data 表达"，healthcheck 靠这条区分"进程死了"
// 与"进程活着但依赖挂了"。
func TestHealthIsNotBehindJWTAuth(t *testing.T) {
	r := newTestRouter(t)
	rec := doRequest(t, r, http.MethodGet, "/api/v1/health", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("HTTP 状态码应为 200，实际 %d，响应体 %q", rec.Code, rec.Body.String())
	}

	var body response.Response[struct {
		Status string `json:"status"`
	}]
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("响应体不是合法 JSON: %v，原文 %q", err, rec.Body.String())
	}
	if body.Code != int(errcode.Success) {
		t.Errorf("业务码应为 %d，实际 %d —— /health 可能被挂到了 JWTAuth 后面", errcode.Success, body.Code)
	}
	if body.Data.Status == "" {
		t.Error("data.status 为空，契约 §11 要求它是 ok 或 degraded")
	}
}

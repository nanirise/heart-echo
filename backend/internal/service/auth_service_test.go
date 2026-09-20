// 覆盖 auth_service 里**不碰数据库**的部分：哈希、假哈希、令牌签发、响应结构。
//
// 注册 / 登录 / 刷新的成功分支都要读 users 表，本机没有 PostgreSQL 跑不了——
// 那部分由 PR 描述里的"未端到端验证"如实标注，这里不假装覆盖了。
package service

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/pkg/jwt"
)

const (
	testSecret     = "test_secret_at_least_32_chars_long!!"
	testAccessTTL  = 2 * time.Hour
	testRefreshTTL = 168 * time.Hour
)

// testService 返回一个只填了 JWT 配置的 AuthService。
// userRepo 刻意留空：需要它的路径在这个文件里一律不测（见文件头注释）。
func testService() *AuthService {
	return &AuthService{
		jwtCfg: config.JWTConfig{
			Secret:             testSecret,
			AccessTokenExpire:  testAccessTTL,
			RefreshTokenExpire: testRefreshTTL,
		},
	}
}

// TestHashPassword 钉住 bcrypt 的三条硬性要求：不是明文、能验回、盐随机。
func TestHashPassword(t *testing.T) {
	const password = "Passw0rd!"

	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("生成哈希失败: %v", err)
	}

	if hash == password {
		t.Fatal("哈希与明文相同 —— 密码没有被加密")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		t.Errorf("哈希无法用原密码验证通过: %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte("Passw0rd?")); err == nil {
		t.Error("换一个密码也能通过验证")
	}

	// cost ≥ 10 是协作 §10.4 红线 5，也是技术文档 §4.8 的取值
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("读不出哈希的 cost: %v", err)
	}
	if cost < 10 {
		t.Errorf("cost 为 %d，低于要求的 10", cost)
	}

	// 盐必须随机：同一密码两次哈希结果相同时，库里两个用同样密码的账号会长得一模一样
	again, err := hashPassword(password)
	if err != nil {
		t.Fatalf("再次生成哈希失败: %v", err)
	}
	if again == hash {
		t.Error("同一密码两次哈希结果相同 —— 盐没有随机")
	}
}

// TestDummyHashIsUsable 保证"用户不存在"那条路径真的在跑一次比对。
// 假哈希不合法或没人用它，计时防护就是空话，而这件事从测试结果上看不出来。
func TestDummyHashIsUsable(t *testing.T) {
	h := dummyHash()

	cost, err := bcrypt.Cost(h)
	if err != nil {
		t.Fatalf("假哈希不是合法 bcrypt 串: %v", err)
	}
	// cost 与 bcryptCost 不一致时，两条登录路径的耗时对不上，计时防护失效
	if cost != bcryptCost {
		t.Errorf("假哈希 cost 为 %d，与 bcryptCost(%d) 不一致", cost, bcryptCost)
	}
	// 它得是个真的 bcrypt 哈希：换任何别的口令去比都通不过。
	// 注意别拿生成它的那个口令去比——那个是通过的，而这件事不影响任何东西：
	// 比对结果在 Login 里被丢弃，假哈希唯一的用途就是消耗掉与真实比对相当的时间。
	if err := bcrypt.CompareHashAndPassword(h, []byte("another-password")); err == nil {
		t.Error("随便一个口令就能通过假哈希比对 —— 它不是一个真的 bcrypt 哈希，那它耗不掉时间")
	}
	// 必须被缓存（sync.OnceValue）：每次重新生成会让第一次失败登录多付一次哈希开销
	if string(h) != string(dummyHash()) {
		t.Error("两次调用返回了不同的哈希 —— 没有缓存")
	}
}

// TestIssueTokens 覆盖注册 / 登录共用的签发逻辑。
func TestIssueTokens(t *testing.T) {
	u := &model.User{ID: 7, Username: "alice", Email: "alice@example.com"}

	resp, err := testService().issueTokens(u)
	if err != nil {
		t.Fatalf("签发令牌失败: %v", err)
	}

	access, err := jwt.ParseToken(resp.AccessToken, testSecret)
	if err != nil {
		t.Fatalf("access token 验签失败: %v", err)
	}
	if access.UserID != u.ID || access.Username != u.Username {
		t.Errorf("access token 里的身份是 (%d, %q)，与签发时不符", access.UserID, access.Username)
	}
	// typ 写错的话，这枚令牌要么访问不了业务接口（JWTAuth 只放行 access），
	// 要么反过来被当成 refresh 用，两种都是鉴权漏洞
	if access.TokenType != jwt.TokenTypeAccess {
		t.Errorf("access token 的 typ 是 %q", access.TokenType)
	}

	refresh, err := jwt.ParseToken(resp.RefreshToken, testSecret)
	if err != nil {
		t.Fatalf("refresh token 验签失败: %v", err)
	}
	if refresh.TokenType != jwt.TokenTypeRefresh {
		t.Errorf("refresh token 的 typ 是 %q", refresh.TokenType)
	}

	assertTTL(t, access.ExpiresAt.Time.Sub(access.IssuedAt.Time), testAccessTTL)
	assertTTL(t, refresh.ExpiresAt.Time.Sub(refresh.IssuedAt.Time), testRefreshTTL)

	if resp.User.ID != u.ID || resp.User.Username != u.Username || resp.User.Email != u.Email {
		t.Errorf("响应里的 user 与签发对象不符: %+v", resp.User)
	}
	if resp.User.AvatarURL != nil {
		t.Errorf("avatarUrl 应为 null，实际 %q", *resp.User.AvatarURL)
	}
}

// assertTTL 断言有效期来自配置而不是写死在代码里。
// 留 5 秒余量：JWT 的时间戳精确到秒，签发时的 IssuedAt 与 ExpiresAt 会被各自截断一次。
func assertTTL(t *testing.T, got, want time.Duration) {
	t.Helper()

	if diff := got - want; diff > 5*time.Second || diff < -5*time.Second {
		t.Errorf("令牌有效期 %v，期望 %v —— 可能没从配置读", got, want)
	}
}

// TestAuthResponseJSONShape 钉住契约 §3.1 / §3.2 的响应结构。
//
// AuthResponse 内嵌了 jwt.TokenPair，序列化成什么形状取决于内嵌的写法：
// 给内嵌字段加一个 json tag，Go 侧编译照过、resp.AccessToken 也照常取得到，
// 但序列化结果会变成 {"tokens":{"accessToken":...},"user":{...}} ——
// 前端按契约取 accessToken 拿到 undefined，只有真发一次请求才看得出来。
// 这条测试就是那个"发的这一次"。
//
// （写成具名字段 `TokenPair jwt.TokenPair` 不用它管：那会让 resp.AccessToken
// 直接编译不过，编译器先一步拦下了。）
func TestAuthResponseJSONShape(t *testing.T) {
	u := &model.User{
		ID:           1,
		Username:     "xiaoming",
		Email:        "xm@example.com",
		PasswordHash: "$2a$10$mustNeverReachTheClient",
	}

	resp, err := testService().issueTokens(u)
	if err != nil {
		t.Fatalf("签发令牌失败: %v", err)
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		t.Fatalf("响应不是 JSON 对象: %v", err)
	}

	for _, key := range []string{"accessToken", "refreshToken", "user"} {
		if _, ok := top[key]; !ok {
			t.Errorf("顶层缺少字段 %q，实际有 %v", key, sortedKeys(top))
		}
	}
	if len(top) != 3 {
		t.Errorf("顶层应只有 accessToken / refreshToken / user 三个字段，实际 %v", sortedKeys(top))
	}

	// 安全防线：密码哈希绝不能出现在响应体里，序列化后的原文也不行
	if strings.Contains(string(raw), "mustNeverReachTheClient") || strings.Contains(string(raw), "passwordHash") {
		t.Errorf("响应体里出现了密码哈希:\n%s", raw)
	}

	var user map[string]json.RawMessage
	if err := json.Unmarshal(top["user"], &user); err != nil {
		t.Fatalf("user 不是 JSON 对象: %v", err)
	}
	// 契约 §3.1 的 user 就是四个字段，多一个都是契约漂移
	if len(user) != 4 {
		t.Errorf("user 应有 4 个字段（契约 §3.1），实际 %v", sortedKeys(user))
	}
}

func sortedKeys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

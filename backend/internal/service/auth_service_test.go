// 覆盖 auth_service 里**不碰数据库**的部分：哈希、假哈希、令牌签发、响应结构；
// 外加"越过参数校验之后、拿不到数据"那一段的错误映射（用不连库的 *gorm.DB 造出来）。
//
// 注册 / 登录 / 刷新的成功分支都要读 users 表，本机没有 PostgreSQL 跑不了——
// 那部分由 PR 描述里的"未端到端验证"如实标注，这里不假装覆盖了。
//
// 同样够不到的还有 UpdateProfile / ChangePassword 里**读到用户之后**才走到的分支：
// 改资料撞名 4004、传了和现在一样的用户名要放行、老密码不对 4015。
// 原因是 AuthService.userRepo 是具体的 *repository.UserRepo，没有接口可以换替身
// （PersonaService 也一样），本仓库也没有连真库的测试基建。
// 这几条目前只有端到端冒烟测试覆盖 —— 不写，是因为写了也只能是假的。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
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

// testDB 返回一个**不连库**的 *gorm.DB，与 handler / repository 两个包里的同名 helper 一致。
// 指向一个必然拒绝连接的端口：任何真去执行 SQL 的路径都会立刻拿到驱动错误，不必干等超时。
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

// testServiceWithDeadDB 的 userRepo 连着一个必然连不上的库。
// 用来验证"参数校验过了、数据拿不到"这一段的错误映射。
func testServiceWithDeadDB(t *testing.T) *AuthService {
	t.Helper()
	return NewAuthService(testDB(t), config.JWTConfig{Secret: testSecret})
}

// assertBizCode 断言错误是**指定错误码**的 BizError。
//
// 只判 err != nil 不够：这里要区分的是 5003 和 5000 / 4041 / 4010 三个"看起来都像失败"
// 的结果，而这几种错误码在日志里长得几乎一样。也不比 err.Error() 字符串——
// 那是有标点的中文文案，改个字就断。
func assertBizCode(t *testing.T, err error, want errcode.ErrorCode) {
	t.Helper()

	if err == nil {
		t.Fatalf("应当返回错误码 %d（%s），实际没有错误", want, want.Message())
	}
	var biz *errcode.BizError
	if !errors.As(err, &biz) {
		t.Fatalf("错误不是 *errcode.BizError，实际 %T: %v", err, err)
	}
	if biz.Code != want {
		t.Errorf("错误码应为 %d（%s），实际 %d（%s）", want, want.Message(), biz.Code, biz.Code.Message())
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

// TestUpdateProfileRejectsBadAvatarBeforeDB 钉住参数校验**发生在**数据库访问之前。
//
// 顺序本身就是行为，不只是实现细节：反过来的话，一个把 avatarUrl 传成数字的请求
// 会先去查一次库（白跑一个来回），而且在库不可用时返回的是 5003「数据库操作失败」——
// 用户按提示去查库，实际上是他自己传错了类型，正确答案是 4001。
//
// 这条能成立的前提正是"校验在前"：testServiceWithDeadDB 的库连不上，
// 只要代码先碰到 userRepo，返回的就必然是 5003 而不是 4001。
func TestUpdateProfileRejectsBadAvatarBeforeDB(t *testing.T) {
	s := testServiceWithDeadDB(t)

	cases := []struct {
		name string
		body string
	}{
		{"传数字", `{"avatarUrl":123}`},
		{"传对象", `{"avatarUrl":{"a":1}}`},
		{"传布尔", `{"avatarUrl":true}`},
		{"超过 255 字符", `{"avatarUrl":"` + strings.Repeat("a", 256) + `"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req dto.UpdateProfileRequest
			if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
				t.Fatalf("请求体不是合法 JSON: %v，原文 %q", err, tc.body)
			}

			_, err := s.UpdateProfile(context.Background(), 1, &req)
			assertBizCode(t, err, errcode.ErrInvalidParams)
		})
	}
}

// TestProfileEndpointsWrapDBErrors 钉住"库出错 → 5003"这条映射，三个端点各一遍。
//
// 判据取 5003 而不是别的，是因为它的三个邻居都是"看起来也像失败"的结果：
//   - 不是 5000：原始错误忘了用 errcode.Wrap 包起来，BizErrorHandler 认不出 BizError，
//     会把它归成"服务端内部错误"——前端看不出是库的问题，日志里也丢了错误码这一层；
//   - 不是 4010 / 4012：连不上库和"登录过期"必须分开，否则库抖一下，全站用户被登出；
//   - 不是 4041：连不上库和"用户被删号"也必须分开，前者该重试、后者该登出。
//
// 用真实的驱动错误（连不上的端口）而不是造一个假 error：gorm.ErrRecordNotFound
// 也是 error，分不清这两种就全乱了。
func TestProfileEndpointsWrapDBErrors(t *testing.T) {
	s := testServiceWithDeadDB(t)
	ctx := context.Background()

	t.Run("GetProfile", func(t *testing.T) {
		_, err := s.GetProfile(ctx, 1)
		assertBizCode(t, err, errcode.ErrDBFailed)
	})

	t.Run("UpdateProfile", func(t *testing.T) {
		name := "newname"
		_, err := s.UpdateProfile(ctx, 1, &dto.UpdateProfileRequest{Username: &name})
		assertBizCode(t, err, errcode.ErrDBFailed)
	})

	t.Run("ChangePassword", func(t *testing.T) {
		err := s.ChangePassword(ctx, 1, &dto.ChangePasswordRequest{
			OldPassword: "Passw0rd!",
			NewPassword: "NewPassw0rd!",
		})
		assertBizCode(t, err, errcode.ErrDBFailed)
	})
}

// 覆盖 dto 的 binding tag 与 json tag —— 这两样编译器都不检查。
//
// 写错了不会编译失败：`min=3` 敲成 `min=03`、`printascii` 敲成 `printAscii`，
// 校验规则**静默消失**，接口照常返回 200，只有真发一次请求才看得出来。
// 这正是 plan §1 Step 2 的完成标准：tag 与契约 §3 逐条对应，表驱动钉住。
//
// 两个方向分开测：请求结构体的校验规则靠"该拦的拦没拦下"反推；
// 响应结构体的字段名靠 §3.1 逐字比对。请求体那侧的 json tag 名**钉不住**，
// 原因见下表中 userName 那一条的注释。
//
// 契约 §3.1 / §3.2 的响应结构（AuthResponse 内嵌 TokenPair 后的平铺形状）
// 由 internal/service/auth_service_test.go 的 TestAuthResponseJSONShape 负责，
// 那份测试能顺手断言"响应体里不含 passwordHash"，位置更合适。
package dto

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gin-gonic/gin/binding"
)

// validate 走的是 gin 在生产里用的那个 validator 实例。
// 自己 new 一个 validator.New() 也能跑，但那是个替身：gin 注册过的自定义 tag
// 与它可能不一致，测绿了不代表线上拦得住。
//
// 从 JSON 反序列化再校验，而不是直接构造结构体：json tag 写错时字段会是零值，
// 紧接着被 required 拦下 —— 一条用例同时钉住"名字对不对"和"规则在不在"。
func validate(t *testing.T, body string, req any) error {
	t.Helper()

	if err := json.Unmarshal([]byte(body), req); err != nil {
		t.Fatalf("请求体不是合法 JSON: %v，原文 %q", err, body)
	}
	return binding.Validator.ValidateStruct(req)
}

// assertBinding 断言这条用例该不该被拦下，以及被拦的是哪个字段。
//
// wantField 为空串表示"应当通过"；否则要求报错信息里出现该 Go 字段名 ——
// 只说"报错了"不够：把 max 规则从 email 误写到 username 上，同样会报错，
// 但拦错了人。
func assertBinding(t *testing.T, body string, req any, wantField string) {
	t.Helper()

	err := validate(t, body, req)
	if wantField == "" {
		if err != nil {
			t.Errorf("应当通过校验，实际被拦下: %v", err)
		}
		return
	}
	if err == nil {
		t.Errorf("应当被 %s 这条规则拦下，实际通过校验 —— 对应的 binding tag 可能没生效", wantField)
		return
	}
	if !strings.Contains(err.Error(), wantField) {
		t.Errorf("拦下的字段不是 %s: %v", wantField, err)
	}
}

// TestRegisterRequestBinding 逐条对应契约 §3.1 的三行校验规则。
// 每个规则的**两侧边界**各有一条用例：只测"合法"测不出上限写没写。
func TestRegisterRequestBinding(t *testing.T) {
	// 100 是 users.email 的 varchar(100)。不设 max 的话，超长邮箱会一路走到
	// 数据库报 22001（值太长），错误码变成 5003 而不是契约写的 4001。
	emailAtMax := strings.Repeat("a", 88) + "@example.com"   // 88+12 = 100，正好
	emailOverMax := strings.Repeat("a", 89) + "@example.com" // 101

	cases := []struct {
		name      string
		body      string
		wantField string
	}{
		{
			"契约 §3.1 的样例",
			`{"username":"xiaoming","email":"xm@example.com","password":"Passw0rd!"}`,
			"",
		},
		{
			"用户名正好 3 位（下界）",
			`{"username":"abc","email":"xm@example.com","password":"Passw0rd!"}`,
			"",
		},
		{
			"用户名 2 位（下界外）",
			`{"username":"ab","email":"xm@example.com","password":"Passw0rd!"}`,
			"Username",
		},
		{
			"用户名正好 20 位（上界）",
			`{"username":"` + strings.Repeat("a", 20) + `","email":"xm@example.com","password":"Passw0rd!"}`,
			"",
		},
		{
			"用户名 21 位（上界外）",
			`{"username":"` + strings.Repeat("a", 21) + `","email":"xm@example.com","password":"Passw0rd!"}`,
			"Username",
		},
		{
			"用户名含下划线（契约要求字母数字）",
			`{"username":"xiao_ming","email":"xm@example.com","password":"Passw0rd!"}`,
			"Username",
		},
		{
			"邮箱正好 100 字符（上界）",
			`{"username":"xiaoming","email":"` + emailAtMax + `","password":"Passw0rd!"}`,
			"",
		},
		{
			"邮箱 101 字符（上界外）",
			`{"username":"xiaoming","email":"` + emailOverMax + `","password":"Passw0rd!"}`,
			"Email",
		},
		{
			"邮箱格式不对",
			`{"username":"xiaoming","email":"not-an-email","password":"Passw0rd!"}`,
			"Email",
		},
		{
			"密码正好 8 位（下界）",
			`{"username":"xiaoming","email":"xm@example.com","password":"Passw0rd"}`,
			"",
		},
		{
			"密码 7 位（下界外）",
			`{"username":"xiaoming","email":"xm@example.com","password":"Passw0!"}`,
			"Password",
		},
		{
			"密码正好 32 位（上界）",
			`{"username":"xiaoming","email":"xm@example.com","password":"Passw0rd!` + strings.Repeat("a", 23) + `"}`,
			"",
		},
		{
			"密码 33 位（上界外）",
			`{"username":"xiaoming","email":"xm@example.com","password":"Passw0rd!` + strings.Repeat("a", 24) + `"}`,
			"Password",
		},
		{
			// printascii 是 \x20-\x7E，含空格。写成 \x21-\x7E 的规则会误伤这类密码。
			"密码含空格（printascii 含 \\x20，放行）",
			`{"username":"xiaoming","email":"xm@example.com","password":"Passw ord1"}`,
			"",
		},
		{
			"密码含制表符（\\x09，不在 printascii 内）",
			`{"username":"xiaoming","email":"xm@example.com","password":"Passw0rd\tx"}`,
			"Password",
		},
		{
			// 中文 3 字节，是"只限 ASCII"这条规则真正要挡的东西
			"密码含中文（非 ASCII，即使长度合法也拦）",
			`{"username":"xiaoming","email":"xm@example.com","password":"Passw0中中"}`,
			"Password",
		},
		{
			"缺 username",
			`{"email":"xm@example.com","password":"Passw0rd!"}`,
			"Username",
		},
		{
			// 这条**故意断言放行**。encoding/json 匹配字段时优先精确匹配、
			// 匹配不上会退化成大小写不敏感匹配，所以 `userName` 照样落进 Username
			//（哪怕是契约 §3.1 写的小写 username）。
			// 留着它是为了说明请求体这侧的 json tag 名在 Go 这层拦不住，
			// 别指望加一条用例就能钉住它。
			"字段名写成 userName —— json 大小写不敏感匹配，照样落进 Username",
			`{"userName":"xiaoming","email":"xm@example.com","password":"Passw0rd!"}`,
			"",
		},
		{
			// 下划线不是大小写差异，匹配不上，字段留空 → 被 required 拦下
			"字段名写成 user_name（契约是 username）",
			`{"user_name":"xiaoming","email":"xm@example.com","password":"Passw0rd!"}`,
			"Username",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertBinding(t, tc.body, &RegisterRequest{}, tc.wantField)
		})
	}
}

// TestLoginRequestBinding 钉住"登录只校验非空"这条刻意的取舍。
//
// 给 LoginRequest 也套上注册那套长度与字符集规则是很容易顺手做的事，
// 但那会让输错密码的人拿到 4001「参数校验失败」，而不是 4013「用户名或密码错误」——
// 契约 §3.2 的错误码只有 4001 和 4013，多出来的分支就是契约漂移。
func TestLoginRequestBinding(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantField string
	}{
		{"契约 §3.2 的样例", `{"username":"xiaoming","password":"Passw0rd!"}`, ""},
		{
			"用户名 1 位、密码 2 位也放行（不套注册的规则）",
			`{"username":"a","password":"ab"}`,
			"",
		},
		{
			"密码含中文也放行（比对必然失败，返回 4013 而不是 4001）",
			`{"username":"xiaoming","password":"密码密码密码"}`,
			"",
		},
		{"缺 password", `{"username":"xiaoming"}`, "Password"},
		{"password 传空串", `{"username":"xiaoming","password":""}`, "Password"},
		{"缺 username", `{"password":"Passw0rd!"}`, "Username"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertBinding(t, tc.body, &LoginRequest{}, tc.wantField)
		})
	}
}

// TestRefreshRequestBinding 钉住字段名的 camelCase ——
// 契约 §3.3 写的是 refreshToken，写成 refresh_token 前端拿不到任何提示，
// 只会看到"必填参数缺失"。
func TestRefreshRequestBinding(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantField string
	}{
		{"契约 §3.3 的样例", `{"refreshToken":"eyJhbGciOiJIUzI1NiJ9.x.y"}`, ""},
		{"缺 refreshToken", `{}`, "RefreshToken"},
		{"字段名写成 refresh_token", `{"refresh_token":"eyJhbGciOiJIUzI1NiJ9.x.y"}`, "RefreshToken"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertBinding(t, tc.body, &RefreshRequest{}, tc.wantField)
		})
	}
}

// TestUserResponseJSONTags 钉住契约 §3.1 里 user 对象的四个字段名。
//
// 逐字比对整段 JSON 而不是逐个取字段：这样字段名、数量、顺序、以及
// avatarUrl 为 nil 时序列化成 null（而不是被省略）四件事一次钉住。
// 少了 null，前端 `user.avatarUrl` 拿到 undefined、用首字母色块兜底的逻辑会走岔。
func TestUserResponseJSONTags(t *testing.T) {
	t.Run("头像为空时是 null，不是省略", func(t *testing.T) {
		raw, err := json.Marshal(UserResponse{ID: 1, Username: "xiaoming", Email: "xm@example.com"})
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}

		const want = `{"id":1,"username":"xiaoming","email":"xm@example.com","avatarUrl":null}`
		if got := string(raw); got != want {
			t.Errorf("序列化结果与契约 §3.1 不符\n实际: %s\n期望: %s", got, want)
		}
	})

	t.Run("头像有值时是字符串", func(t *testing.T) {
		url := "https://example.com/a.png"
		raw, err := json.Marshal(UserResponse{ID: 1, AvatarURL: &url})
		if err != nil {
			t.Fatalf("序列化失败: %v", err)
		}
		if !strings.Contains(string(raw), `"avatarUrl":"https://example.com/a.png"`) {
			t.Errorf("avatarUrl 字段名或取值不对: %s", raw)
		}
	})
}

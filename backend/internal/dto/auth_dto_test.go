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
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin/binding"

	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
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

// assertBizCode 断言错误是**指定错误码**的 BizError。
//
// 只判 err != nil 不够：ParseAvatarURL 的结果分"该拦下"和"该原样透传"两类，
// 大意写反一个分支就会把非法输入放过去，而"只判有没有错"的测试照样绿。
// 也不比 err.Error() 的字符串：那是有标点的中文文案，改个字就断。
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
		t.Errorf("错误码应为 %d，实际 %d", want, biz.Code)
	}
}

// TestParseAvatarURLThreeStates 钉住 avatarUrl 的三态（契约 §3.5）。
//
// 这是本功能里最容易写出**静默** bug 的一处：字段若用 *string 声明，
// 「没传」和「传了 null」都是 nil，用户点"清空头像"会被当成"没传"、静默不生效，
// 而只测 {"username":"x"} 的用例全绿。契约明确要求传 null 可以清空，
// 所以三种状态必须逐个钉死，present 与 value 两个返回值分开断言 ——
// 它们对应的是"UPDATE 发不发"和"写成什么"，错一个都是错。
func TestParseAvatarURLThreeStates(t *testing.T) {
	str := func(s string) *string { return &s }

	cases := []struct {
		name        string
		body        string
		wantValue   *string
		wantPresent bool
		wantErr     errcode.ErrorCode // 0 表示不该报错
	}{
		{
			"字段没传 —— 不动这一列",
			`{"username":"newname"}`,
			nil, false, 0,
		},
		{
			"请求体是空对象 —— 同样不动",
			`{}`,
			nil, false, 0,
		},
		{
			"传 null —— 要改，改成 nil（清空头像）",
			`{"avatarUrl":null}`,
			nil, true, 0,
		},
		{
			"传字符串 —— 要改，改成它",
			`{"avatarUrl":"https://example.com/a.png"}`,
			str("https://example.com/a.png"), true, 0,
		},
		{
			// 记录当前行为：契约只定义了 null 清空，没定义空串。
			// 按值透传（存 ""），前端 `if (!avatarUrl)` 同样走色块兜底，行为一致。
			"传空串 —— 按值透传，不是清空",
			`{"avatarUrl":""}`,
			str(""), true, 0,
		},
		{
			"正好 255 字符（上界）",
			`{"avatarUrl":"` + strings.Repeat("a", 255) + `"}`,
			str(strings.Repeat("a", 255)), true, 0,
		},
		{
			// 不拦的话会走到数据库报 22001（值太长），用户拿到 5003 而不是 4001
			"256 字符（上界外）",
			`{"avatarUrl":"` + strings.Repeat("a", 256) + `"}`,
			nil, false, errcode.ErrInvalidParams,
		},
		{
			"传数字 —— 契约只允许字符串或 null",
			`{"avatarUrl":123}`,
			nil, false, errcode.ErrInvalidParams,
		},
		{
			"传对象",
			`{"avatarUrl":{"a":1}}`,
			nil, false, errcode.ErrInvalidParams,
		},
		{
			"传布尔",
			`{"avatarUrl":true}`,
			nil, false, errcode.ErrInvalidParams,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var req UpdateProfileRequest
			if err := json.Unmarshal([]byte(tc.body), &req); err != nil {
				t.Fatalf("请求体不是合法 JSON: %v，原文 %q", err, tc.body)
			}

			gotValue, gotPresent, err := req.ParseAvatarURL()

			if tc.wantErr == 0 {
				if err != nil {
					t.Fatalf("不应报错，实际 %v", err)
				}
			} else {
				assertBizCode(t, err, tc.wantErr)
			}

			if gotPresent != tc.wantPresent {
				t.Errorf("present 应为 %v，实际 %v —— 它决定 UPDATE 语句发不发", tc.wantPresent, gotPresent)
			}
			switch {
			case tc.wantValue == nil && gotValue != nil:
				t.Errorf("值应为 nil，实际 %q", *gotValue)
			case tc.wantValue != nil && gotValue == nil:
				t.Errorf("值应为 %q，实际 nil", *tc.wantValue)
			case tc.wantValue != nil && gotValue != nil && *tc.wantValue != *gotValue:
				t.Errorf("值应为 %q，实际 %q", *tc.wantValue, *gotValue)
			}
		})
	}
}

// TestUpdateProfileRequestBinding 钉住契约 §3.5 里 username 的规则。
//
// ⚠️ 这里原本打算记录一个"空串能过 binding"的洞，实测发现是反的：
// omitempty 判的是**指针本身是不是 nil**，不是"指向的值是不是空"。
// {"username":""} 解出来是个非 nil 指针，omitempty 不跳过，min=3 照常拦下。
// 下面那条用例断言的就是"被拦下"—— 结论以实测为准，不以推断为准。
func TestUpdateProfileRequestBinding(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		wantField string
	}{
		{"契约 §3.5 的样例", `{"avatarUrl":"https://example.com/a.png","username":"newname"}`, ""},
		{"两个字段都不传（等于什么都不改）", `{}`, ""},
		{"只传 avatarUrl", `{"avatarUrl":null}`, ""},
		{"用户名正好 3 位（下界）", `{"username":"abc"}`, ""},
		{"用户名 2 位（下界外）", `{"username":"ab"}`, "Username"},
		{"用户名正好 20 位（上界）", `{"username":"` + strings.Repeat("a", 20) + `"}`, ""},
		{"用户名 21 位（上界外）", `{"username":"` + strings.Repeat("a", 21) + `"}`, "Username"},
		{"用户名含下划线（契约要求字母数字）", `{"username":"xiao_ming"}`, "Username"},
		{
			// 实测结论：非 nil 指针 + 空串 → omitempty 不跳过 → min=3 拦下。
			// 与上面 {} 那条（指针为 nil）正好构成一对：空不空看的是指针，不是值。
			"用户名为空串 —— 被 min 拦下",
			`{"username":""}`,
			"Username",
		},
		{
			"显式传 null —— 与没传同义，不动这一列",
			`{"username":null}`,
			"",
		},
		{
			// 两个字段都不认 → 都留 nil → 退化成"什么都不改"，binding 不报错。
			// 与 RegisterRequest 那条 userName 用例同理：请求体这侧的字段名在 Go 这层拦不住。
			"字段名写成 user_name",
			`{"user_name":"newname"}`,
			"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertBinding(t, tc.body, &UpdateProfileRequest{}, tc.wantField)
		})
	}
}

// TestChangePasswordRequestBinding 钉住契约 §3.6：oldPassword 只 required，
// newPassword 与注册**逐条同规则**。
//
// 老密码不套长度与字符集是有意的：格式不对和密码写错对用户是同一件事，
// 多校验一层只会让他拿到 4001 而不是 4015，反而看不出问题出在密码上。
//
// 下半段与 TestRegisterRequestBinding 用的是同一组边界值。两处规则必须一致 ——
// 不一致会出现"注册时能用的密码，改密码时说它不合法"，改规则要两边一起改。
func TestChangePasswordRequestBinding(t *testing.T) {
	const ok = `"oldPassword":"Passw0rd!"`

	cases := []struct {
		name      string
		body      string
		wantField string
	}{
		{"契约 §3.6 的样例", `{"oldPassword":"Passw0rd!","newPassword":"NewPassw0rd!"}`, ""},
		{"老密码只有 1 位也放行（不套注册的规则）", `{` + ok + `,"oldPassword":"a","newPassword":"NewPassw0rd!"}`, ""},
		{"老密码含中文也放行（比对必然失败，返回 4015 而不是 4001）", `{"oldPassword":"密码密码密码","newPassword":"NewPassw0rd!"}`, ""},
		{"新密码正好 8 位（下界）", `{` + ok + `,"newPassword":"NewPassw" }`, ""},
		{"新密码 7 位（下界外）", `{` + ok + `,"newPassword":"NewPass"}`, "NewPassword"},
		{
			"新密码正好 32 位（上界）",
			`{` + ok + `,"newPassword":"NewPassw0rd!` + strings.Repeat("a", 20) + `"}`,
			"",
		},
		{
			"新密码 33 位（上界外）",
			`{` + ok + `,"newPassword":"NewPassw0rd!` + strings.Repeat("a", 21) + `"}`,
			"NewPassword",
		},
		{"新密码含中文（非 ASCII，即使长度合法也拦）", `{` + ok + `,"newPassword":"NewPass中中" }`, "NewPassword"},
		{"新密码含制表符（\\x09，不在 printascii 内）", `{` + ok + `,"newPassword":"NewPass\tord" }`, "NewPassword"},
		{"新密码含空格（printascii 含 \\x20，放行）", `{` + ok + `,"newPassword":"New Passw0rd" }`, ""},
		{"缺 oldPassword", `{"newPassword":"NewPassw0rd!"}`, "OldPassword"},
		{"oldPassword 传空串", `{"oldPassword":"","newPassword":"NewPassw0rd!"}`, "OldPassword"},
		{"缺 newPassword", `{` + ok + `}`, "NewPassword"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertBinding(t, tc.body, &ChangePasswordRequest{}, tc.wantField)
		})
	}
}

// TestProfileResponseJSONShape 钉住契约 §3.4 的响应 data 结构。
//
// 与 §3.1 的区别只是多一个 createdAt，所以断言法也照搬：逐字比对整段 JSON，
// 一次钉住字段名、数量、顺序，以及"内嵌 UserResponse 是平铺、不是嵌套对象"。
// 内嵌字段哪天被改成具名的 `User UserResponse`，序列化会多出一层
// {"User":{...}}，前端按契约取 data.username 拿到 undefined。
func TestProfileResponseJSONShape(t *testing.T) {
	// 固定时区与时刻，免得断言随本机时区漂移
	created := time.Date(2026, 9, 10, 14, 30, 0, 0, time.FixedZone("CST", 8*60*60))

	raw, err := json.Marshal(NewProfileResponse(&model.User{
		ID:        1,
		Username:  "xiaoming",
		Email:     "xm@example.com",
		CreatedAt: created,
	}))
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	const want = `{"id":1,"username":"xiaoming","email":"xm@example.com","avatarUrl":null,"createdAt":"2026-09-10T14:30:00+08:00"}`
	if got := string(raw); got != want {
		t.Errorf("序列化结果与契约 §3.4 不符\n实际: %s\n期望: %s", got, want)
	}
}

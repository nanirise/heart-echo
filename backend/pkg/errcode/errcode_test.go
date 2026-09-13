package errcode

import "testing"

// allCodes 手工登记本包定义的全部错误码。
// Go 的 const 没有运行时元数据，反射也枚举不出来，所以只能手写一份。
// 漏登记这里同样会被发现：测试断言三个集合完全相等，
// 所以"加了常量却忘了加进本切片"会让测试失败，而不是静默通过。
// 唯一的漏网情形是"常量没加进来、两个 map 也没登记"——但那样的常量是死代码。
var allCodes = []ErrorCode{
	Success,

	ErrInvalidParams, ErrParamMissing, ErrEmailExists, ErrUsernameTaken,

	ErrUnauthorized, ErrTokenInvalid, ErrTokenExpired, ErrPasswordWrong,
	ErrRefreshInvalid, ErrOldPasswordWrong,

	ErrForbidden,

	ErrNotFound, ErrUserNotFound, ErrPersonaNotFound,

	ErrInternal, ErrLLMFailed, ErrAIUnavailable, ErrDBFailed,
}

// TestCodeSetsAreConsistent 校验常量集合与两个 map 的键集合完全一致。
// 存在的意义：4015 曾因漏登记 codeHTTPStatus，让"改密码失败"返回 HTTP 500，
// 排查方向会错误地偏向数据库。这个测试直接拦住这类漏登记。
func TestCodeSetsAreConsistent(t *testing.T) {
	want := make(map[ErrorCode]bool, len(allCodes))
	for _, code := range allCodes {
		if want[code] {
			t.Fatalf("allCodes 中重复登记了错误码 %d", code)
		}
		want[code] = true
	}

	got := make(map[ErrorCode]bool, len(codeMessages))
	for code := range codeMessages {
		got[code] = true
	}
	compareSets(t, "codeMessages", want, got)

	got = make(map[ErrorCode]bool, len(codeHTTPStatus))
	for code := range codeHTTPStatus {
		got[code] = true
	}
	compareSets(t, "codeHTTPStatus", want, got)
}

// compareSets 比对两个集合，分别报出"漏登记"与"未定义"两种不一致。
func compareSets(t *testing.T, name string, want, got map[ErrorCode]bool) {
	t.Helper()

	for code := range want {
		if !got[code] {
			t.Errorf("%s 漏登记错误码 %d", name, code)
		}
	}
	for code := range got {
		if !want[code] {
			t.Errorf("%s 出现未定义的错误码 %d", name, code)
		}
	}
}

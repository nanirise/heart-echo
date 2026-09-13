// Package errcode 定义全项目唯一的业务错误码。
//
// 核心约定是"一 code 一 msg"：一个错误码只能对应一句固定文案，
// 所有响应文案都从 codeMessages 查，调用方不得就地自创提示语。
// 这样才能保证同一个错误在任何接口里说法一致，前端也才能靠 code 稳定判断语义。
//
// 新增错误码时必须三处同时改：常量块、codeMessages、codeHTTPStatus。
// 漏改会被 errcode_test.go 拦下。
package errcode

import (
	"fmt"
	"net/http"
)

type ErrorCode int

const (
	// 成功
	Success ErrorCode = 200

	// 4xxx 客户端错误
	ErrInvalidParams ErrorCode = 4001 // 参数校验失败
	ErrParamMissing  ErrorCode = 4002 // 必填参数缺失
	ErrEmailExists   ErrorCode = 4003 // 该邮箱已被注册
	ErrUsernameTaken ErrorCode = 4004 // 该用户名已被占用

	// 行尾注释必须与下方 codeMessages 逐字一致
	ErrUnauthorized     ErrorCode = 4010 // 未登录或登录已过期
	ErrTokenInvalid     ErrorCode = 4011 // Token 无效
	ErrTokenExpired     ErrorCode = 4012 // Token 已过期
	ErrPasswordWrong    ErrorCode = 4013 // 用户名或密码错误
	ErrRefreshInvalid   ErrorCode = 4014 // 刷新令牌无效，请重新登录
	ErrOldPasswordWrong ErrorCode = 4015 // 原密码不正确

	// 4030 只用于功能层面的越权（封禁用户访问、无权限使用某功能）。
	// 资源层面的越权不走这里，见 4043 的说明。
	ErrForbidden ErrorCode = 4030 // 无权限访问该资源

	ErrNotFound     ErrorCode = 4040 // 资源不存在
	ErrUserNotFound ErrorCode = 4041 // 用户不存在
	// 4042 原「会话不存在」已废弃：一个人设只有一个对话，不存在会话实体
	//
	// 4043 同时承担"人设不存在"与"资源越权（访问别人的 persona）"两种情形。
	// 若越权时返回 403，等于告诉对方这个 persona_id 存在；而它是全局自增的，
	// 可以被顺序试号探测出系统里共有多少人设。所以统一按"不存在"返回，
	// 不区分这两种情况，也就不再有单独的"人设越权"错误码。
	ErrPersonaNotFound ErrorCode = 4043 // 人设不存在

	// 5xxx 服务端错误
	ErrInternal      ErrorCode = 5000 // 服务端内部错误
	ErrLLMFailed     ErrorCode = 5001 // AI 回复生成失败，请稍后重试
	ErrAIUnavailable ErrorCode = 5002 // AI 服务暂时不可用
	ErrDBFailed      ErrorCode = 5003 // 数据库操作失败
)

// 唯一数据源：一个 code 严格对应一个 msg
var codeMessages = map[ErrorCode]string{
	Success: "success",

	ErrInvalidParams: "参数校验失败",
	ErrParamMissing:  "必填参数缺失",
	ErrEmailExists:   "该邮箱已被注册",
	ErrUsernameTaken: "该用户名已被占用",

	ErrUnauthorized:     "未登录或登录已过期",
	ErrTokenInvalid:     "Token 无效",
	ErrTokenExpired:     "Token 已过期",
	ErrPasswordWrong:    "用户名或密码错误",
	ErrRefreshInvalid:   "刷新令牌无效，请重新登录",
	ErrOldPasswordWrong: "原密码不正确",

	ErrForbidden: "无权限访问该资源",

	ErrNotFound:        "资源不存在",
	ErrUserNotFound:    "用户不存在",
	ErrPersonaNotFound: "人设不存在",

	ErrInternal:      "服务端内部错误",
	ErrLLMFailed:     "AI 回复生成失败，请稍后重试",
	ErrAIUnavailable: "AI 服务暂时不可用",
	ErrDBFailed:      "数据库操作失败",
}

// 一个 code 对应一个 HTTP 状态码
var codeHTTPStatus = map[ErrorCode]int{
	Success: http.StatusOK,

	ErrInvalidParams: http.StatusBadRequest,
	ErrParamMissing:  http.StatusBadRequest,
	ErrEmailExists:   http.StatusBadRequest,
	ErrUsernameTaken: http.StatusBadRequest,

	ErrUnauthorized:   http.StatusUnauthorized,
	ErrTokenInvalid:   http.StatusUnauthorized,
	ErrTokenExpired:   http.StatusUnauthorized,
	ErrPasswordWrong:  http.StatusUnauthorized,
	ErrRefreshInvalid: http.StatusUnauthorized,
	// 4015 必须登记：漏登记会走 HTTPStatus() 的兜底分支，改密码失败变成 HTTP 500
	ErrOldPasswordWrong: http.StatusUnauthorized,

	ErrForbidden: http.StatusForbidden,

	ErrNotFound:        http.StatusNotFound,
	ErrUserNotFound:    http.StatusNotFound,
	ErrPersonaNotFound: http.StatusNotFound,

	ErrInternal:      http.StatusInternalServerError,
	ErrLLMFailed:     http.StatusInternalServerError,
	ErrAIUnavailable: http.StatusInternalServerError,
	ErrDBFailed:      http.StatusInternalServerError,
}

// Message 返回该错误码唯一对应的提示文案
func (e ErrorCode) Message() string {
	if msg, ok := codeMessages[e]; ok {
		return msg
	}
	return "未知错误"
}

// HTTPStatus 返回该错误码对应的 HTTP 状态码
func (e ErrorCode) HTTPStatus() int {
	if status, ok := codeHTTPStatus[e]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// BizError 业务异常，service 层统一抛出
type BizError struct {
	Code ErrorCode
	Err  error // 原始错误，仅用于日志，不返回给前端
}

func (e *BizError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Code.Message(), e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Code.Message())
}

func New(code ErrorCode) *BizError             { return &BizError{Code: code} }
func Wrap(code ErrorCode, err error) *BizError { return &BizError{Code: code, Err: err} }

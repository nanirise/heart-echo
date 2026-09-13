// Package response 统一 HTTP 响应体格式。
//
// 所有接口的响应都是 {code, message, data, timestamp} 四段结构，成功走 Success、失败走 Fail。
// Fail 只接受错误码、不接受自定义文案，这是"一 code 一 msg"在类型层面的保证：
// 调用方没有地方传文案，想绕过也绕不过去。
package response

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// Response 所有接口统一的响应体结构
type Response[T any] struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	Data      T      `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

// Success 写入成功响应。
// data 用泛型，是为了让不同接口能返回各自的数据结构（用户、人设列表、消息列表…），
// 同时保持响应体外层四段结构完全一致。
func Success[T any](c *gin.Context, data T) {
	c.JSON(http.StatusOK, Response[T]{
		Code:      int(errcode.Success),
		Message:   errcode.Success.Message(),
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	})
}

// Fail 只接受错误码，message 一律从错误码表查，禁止调用方自定义文案。
// HTTP 状态码由错误码决定（code.HTTPStatus()），与响应体里的业务 code 分离。
func Fail(c *gin.Context, code errcode.ErrorCode) {
	c.JSON(code.HTTPStatus(), Response[any]{
		Code:      int(code),
		Message:   code.Message(),
		Data:      nil,
		Timestamp: time.Now().UnixMilli(),
	})
}

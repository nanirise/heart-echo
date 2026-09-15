// Package middleware 存放 HTTP 中间件。
//
// 中间件负责"每个请求都要做一遍"的事（捕崩溃、记日志、鉴权、跨域），
// 让业务 handler 只关心业务本身。
//
// 链上的顺序不可随意调整，见 AGENTS §4.2：
//
//	Recovery → RequestLogger → CORS → BizErrorHandler → JWTAuth → Handler
package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// Recovery 接住链路上任何一处的 panic，转成 5000 响应，
// 避免单个请求的 bug 把整个服务带崩。
//
// 必须挂在最外层：它只能接住"比自己更靠里"的环节出的问题。
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// defer 在 c.Next() 之前，网要先铺好；
		// 放到 c.Next() 后面等于事后再铺，人已经摔下去了。
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered",
					// 与访问日志用同一条 traceId，两条日志靠它对上号。
					zap.String("traceId", c.GetString(ContextKeyTraceID)),
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
					zap.String("method", c.Request.Method),
					// 堆栈是排查 panic 唯一的线索，但只进日志——
					// 它会暴露文件路径与函数名，AGENTS §4.1 禁止返回前端。
					zap.Stack("stack"),
				)

				// 响应头一旦发出去就改不了了。SSE 流到一半崩就是这种情况：
				// 此时再写 JSON 会把 text/event-stream 搅乱，前端的解析器会直接报错。
				if !c.Writer.Written() {
					response.Fail(c, errcode.ErrInternal)
				}
				c.Abort()
			}
		}()

		c.Next()
	}
}

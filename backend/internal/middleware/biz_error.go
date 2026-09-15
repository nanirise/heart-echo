package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// BizErrorHandler 是所有业务错误的唯一出口。
//
// handler 不调 response.Fail，只把错误交给 c.Error()；记日志和写响应都由这里做。
// 必须挂在路由组上、业务 handler 之前：它先 c.Next()，跑完再看 c.Errors。
func BizErrorHandler(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}
		err := c.Errors.Last().Err
		traceID := c.GetString(ContextKeyTraceID)

		// 用 errors.As 而非类型断言：错误可能被 fmt.Errorf("%w") 在外面包过一层。
		var bizErr *errcode.BizError
		if errors.As(err, &bizErr) {
			// 5xxx 是我方故障，原始错误只进日志不返回前端（AGENTS §4.1）；
			// 4xxx 是调用方自己传错了，记 warn 就够，不必惊动值班。
			if bizErr.Code >= 5000 {
				logger.Error("business error",
					zap.String("traceId", traceID),
					zap.String("codeMsg", bizErr.Code.Message()),
					zap.Error(bizErr.Err),
				)
			} else {
				logger.Warn("business error",
					zap.String("traceId", traceID),
					zap.String("codeMsg", bizErr.Code.Message()),
				)
			}
			response.Fail(c, bizErr.Code)
			return
		}

		// 不是 BizError，说明是没人包装过的意外错误，一律按 5000 处理。
		logger.Error("unknown error",
			zap.String("traceId", traceID),
			zap.Error(err),
		)
		response.Fail(c, errcode.ErrInternal)
	}
}

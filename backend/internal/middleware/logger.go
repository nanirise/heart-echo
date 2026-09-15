package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// gin.Context 里存值的键名。值必须与 API_CONTRACT §4.1 的 JSON 字段同名。
// 命名对齐 TECH_DESIGN §4.7，成员 2、成员 3 按这些名字取用。
const (
	ContextKeyTraceID  = "traceId"
	ContextKeyUserID   = "userId"
	ContextKeyUsername = "username"
)

// RequestLogger 为每个请求记一条访问日志。
//
// 日志字段取自 TECH_DESIGN §10.4：traceId / path / method / status / latency / userId。
func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		// start 声明在内层，每次请求各有一份。
		// 若提到外层，就成了所有请求共用的一个变量，请求 A 刚写下时间
		// 就被请求 B 覆盖，算出来的 latency 毫无意义。
		start := time.Now()

		// 存进 c，让链路后面的环节也用得上同一条 traceId
		traceID := newTraceID()
		c.Set(ContextKeyTraceID, traceID)

		// 记日志必须写在 defer 里：handler panic 时，panic 会冲过 c.Next()
		// 这一行，把它后面的代码全部跳过——崩溃的请求恰恰最需要留下记录。
		//
		// 已知瑕疵：panic 时这里读到的是 panic 之前的状态码（一般是 200），
		// 因为最外层的 Recovery 写 500 发生在这之后。排查时以 level=error 的
		// "panic recovered" 日志为准，两条靠 traceId 对上号。
		defer func() {
			logger.Info("access",
				zap.String("traceId", traceID),
				zap.String("method", c.Request.Method),
				zap.String("path", c.Request.URL.Path),
				zap.Int("status", c.Writer.Status()),
				zap.Duration("latency", time.Since(start)),
				// userId 由 JWTAuth 解析 Token 后写入。本轮 JWTAuth 是空壳，
				// 所以这里暂时恒为空串，字段先留着。
				zap.String("userId", c.GetString(ContextKeyUserID)),
			)
		}()

		c.Next()
	}
}

// newTraceID 造一个 16 字节随机数，编码成 32 位十六进制串。
//
// 不引 google/uuid：没人会解析这个串，它唯一的用途是在日志里检索，
// 格式是不是标准 UUID 无关紧要，不值得为此多一个依赖。
func newTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 随机源不可用属于系统级故障。这里返回空串让日志缺一列，
		// 不因此让请求失败——traceId 是辅助信息，不是功能本身。
		return ""
	}
	return hex.EncodeToString(b)
}

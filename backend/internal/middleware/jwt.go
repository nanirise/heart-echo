package middleware

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/jwt"
)

// JWTAuth 校验 Authorization 头里的 access token，并把 userId 与 username 写入 Context。
//
// 失败路径分三类，对应 API_CONTRACT §2 的三个错误码：
//   - 没带 Authorization 头，或不是 "Bearer " 前缀 → 4010 未登录
//   - token 已过期 → 4012（前端据此拿 refresh token 换新的，然后重放原请求）
//   - 其余（签名不对、格式错、拿 refresh token 当 access 用）→ 4011 Token 无效
//
// 这里不自己写响应：错误用 c.Error() 上抛，由 BizErrorHandler 统一出口。
// 所以失败时只做两件事——c.Error() 和 c.Abort()，不做第三件。
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		const prefix = "Bearer "

		header := c.GetHeader("Authorization")
		// 必须校验前缀，不能只做 strings.TrimPrefix：
		// TrimPrefix 遇到 "Basic xxx" 会原样返回，等于把一个没带 Token 的请求
		// 当成带了 Token 的请求，往下走到"Token 无效"而不是"未登录"。
		//
		// [假设] 只认大写 "Bearer "。RFC 6750 §2.1 规定 scheme 大小写不敏感，
		// 但本项目没有第三方客户端，前端 request.ts 也写死小写以外的这种形式，
		// 不值得为此加分支。将来若接入外部调用方，改用 EqualFold 比较前 7 个字符。
		if !strings.HasPrefix(header, prefix) {
			_ = c.Error(errcode.New(errcode.ErrUnauthorized))
			c.Abort()
			return
		}

		claims, err := jwt.ParseToken(strings.TrimSpace(header[len(prefix):]), secret)
		if err != nil {
			// 过期是"预期内的失败"，和"令牌是假的"必须分开报：
			// 前端只有拿到 4012 才会去刷新重放，拿到 4011 就直接登出了。
			code := errcode.ErrTokenInvalid
			if errors.Is(err, jwt.ErrTokenExpired) {
				code = errcode.ErrTokenExpired
			}
			_ = c.Error(errcode.New(code))
			c.Abort()
			return
		}

		// refresh token 不得访问业务接口。不校验这一条的话，2 小时的 access
		// 有效期形同虚设——攻击者拿 7 天有效的 refresh token 当 access 用就是了。
		if claims.TokenType != jwt.TokenTypeAccess {
			_ = c.Error(errcode.New(errcode.ErrTokenInvalid))
			c.Abort()
			return
		}

		// 类型必须是 uint64：下游（如 persona_handler 的 currentUserID）用
		// c.GetUint64 取值，gin 内部是直接类型断言，类型不符会静默返回 0。
		c.Set(ContextKeyUserID, claims.UserID)
		c.Set(ContextKeyUsername, claims.Username)

		c.Next()
	}
}

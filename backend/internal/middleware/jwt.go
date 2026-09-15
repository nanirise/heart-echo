package middleware

import (
	"github.com/gin-gonic/gin"
)

// JWTAuth 校验 Authorization 头里的 access token，并把 userId 写入 Context。
//
// ⚠️ 本轮是空壳：直接放行，等于没有鉴权。关闭它是 `auth-login` 的第一条验收项。
//
// TODO(auth-login): 按 TECH_DESIGN §4.7 实现：
//  1. 取 Authorization 头，没有 "Bearer " 前缀 → ErrUnauthorized(4010)
//  2. pkg/jwt.ParseToken 失败：errors.Is(err, jwt.ErrTokenExpired) → 4012，否则 → 4011
//  3. claims.TokenType != "access" → 4011（refresh token 不得访问业务接口）
//  4. 通过后 c.Set(ContextKeyUserID, claims.UserID)、c.Set(ContextKeyUsername, claims.Username)
//  5. 任一步失败都要 c.Abort()，响应交给 BizErrorHandler 出口
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// Package jwt 封装双令牌（access / refresh）的签发与校验。
//
// 全项目只有这个文件 import github.com/golang-jwt/jwt/v5，其他包一律用
// 本包的 GenerateTokenPair / ParseToken。这样以后换鉴权方案只改这一个文件，
// 而且调用方拿到的是本包的 ErrTokenExpired / ErrTokenInvalid，
// 不需要知道底层用的是哪个库。
package jwt

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

// Issuer 写在令牌的签发方字段里，排查时能看出这个 token 是哪来的。
const Issuer = "heart-echo"

// 令牌类型。必须写进令牌并由调用方校验，
// 否则 refresh token 也能拿去访问业务接口，短有效期就白设了。
const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

// 校验失败的两类原因。调用方用 errors.Is 判断，据此分发错误码：
// 过期 → 4012（前端该拿 refresh 去换新令牌，然后重放原请求），
// 其余（签名不对、格式错）→ 4011。
//
// 4010「未登录」不在这两类里：那是"请求根本没带 Token"，
// 由中间件在调用本包之前就判掉了，解析器看不到这种情况。
var (
	ErrTokenExpired = errors.New("jwt: token expired")
	ErrTokenInvalid = errors.New("jwt: token invalid")
)

// Claims 自定义声明。
type Claims struct {
	// 类型与 internal/model 的 User.ID 保持一致（都是 uint64）。
	// 令牌里取出来的 userID 会直接拿去查库，两边类型不同就得处处转换。
	UserID    uint64 `json:"uid"`
	Username  string `json:"uname"`
	TokenType string `json:"typ"`
	jwtlib.RegisteredClaims
}

// TokenPair 一次签发产出的两个令牌。
// JSON tag 用 camelCase，与接口契约 §4.1 一致。
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// GenerateTokenPair 签发 access + refresh 两个令牌。
//
// 有效期由调用方从配置传入，不在本包写死——否则会和 .env 的
// JWT_ACCESS_EXPIRE / JWT_REFRESH_EXPIRE 形成两个事实来源，
// 改了 .env 不生效，排查起来很费劲。
func GenerateTokenPair(
	userID uint64,
	username, secret string,
	accessTTL, refreshTTL time.Duration,
) (*TokenPair, error) {
	access, err := sign(userID, username, secret, TokenTypeAccess, accessTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := sign(userID, username, secret, TokenTypeRefresh, refreshTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

// sign 签发单个令牌。两个令牌只有类型和有效期不同，其余完全一样。
func sign(userID uint64, username, secret, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		Username:  username,
		TokenType: tokenType,
		RegisteredClaims: jwtlib.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   strconv.FormatUint(userID, 10),
			IssuedAt:  jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// ParseToken 校验签名与有效期，通过则返回声明。
//
// 失败只返回本包的两个哨兵错误，调用方不必 import 底层 jwt 库：
//   - ErrTokenExpired 已过期
//   - ErrTokenInvalid 签名不对、格式错、算法被换过
func ParseToken(tokenStr, secret string) (*Claims, error) {
	var claims Claims
	_, err := jwtlib.ParseWithClaims(tokenStr, &claims, func(t *jwtlib.Token) (any, error) {
		// 必须限定签名算法。不校验的话，攻击者把 header 改成
		// {"alg":"none"} 就能伪造任意身份的令牌，签名形同虚设。
		if _, ok := t.Method.(*jwtlib.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		// 过期是"预期内的失败"（前端该去刷新），必须和"令牌是假的"分开报
		if errors.Is(err, jwtlib.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	return &claims, nil
}

// 覆盖两类路径：
//   - 正常：签发 → 校验，身份信息与令牌类型都对得上
//   - 拒绝：签名被篡改、密钥不对、已过期、伪造 alg=none
//
// 后四类错误肉眼看不出来，只能靠测试拦——尤其 alg=none，
// 算法校验一旦被改掉，任何实现都能伪造任意身份的令牌。
package jwt

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const testSecret = "test_secret_at_least_32_chars_long!!"

func TestGenerateTokenPair(t *testing.T) {
	pair, err := GenerateTokenPair(7, "alice", testSecret, 2*time.Hour, 168*time.Hour)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatal("令牌为空")
	}
	if pair.AccessToken == pair.RefreshToken {
		t.Fatal("access 与 refresh 不该是同一个字符串")
	}
}

func TestParseAccessToken(t *testing.T) {
	pair, err := GenerateTokenPair(7, "alice", testSecret, 2*time.Hour, 168*time.Hour)
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}

	claims, err := ParseToken(pair.AccessToken, testSecret)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if claims.UserID != 7 {
		t.Fatalf("userID 应回读为 7，实际 %d", claims.UserID)
	}
	if claims.Username != "alice" {
		t.Fatalf("username 应回读为 alice，实际 %s", claims.Username)
	}
	if claims.TokenType != TokenTypeAccess {
		t.Fatalf("令牌类型应为 %s，实际 %s", TokenTypeAccess, claims.TokenType)
	}
}

// refresh 必须带着 refresh 类型标记，否则中间件没法拒绝它访问业务接口。
func TestParseRefreshToken(t *testing.T) {
	pair, _ := GenerateTokenPair(7, "alice", testSecret, 2*time.Hour, 168*time.Hour)

	claims, err := ParseToken(pair.RefreshToken, testSecret)
	if err != nil {
		t.Fatalf("校验失败: %v", err)
	}
	if claims.TokenType != TokenTypeRefresh {
		t.Fatalf("令牌类型应为 %s，实际 %s", TokenTypeRefresh, claims.TokenType)
	}
}

// tamperSignature 把签名段的**首个**字符换成一个不同的字符，模拟"签名被改过"。
//
// 为什么不能改最后一个字符（这里原来就是那么写的，是个偶发失败的坑）：
// HS256 签名是 32 字节，base64url 编码后 43 个字符，最后 3 个字符编码 2 字节
// （16 位）却占 18 位，多出的 2 位是填充位，解码时被直接丢弃。
// 也就是说末位字符只有高 4 位算数。字母表里 U(20)=010100 与 X(23)=010111
// 高 4 位相同，于是末位恰好是 U 时（概率 1/16）把 U 改成 X，解出来的 32 字节
// 一个比特都没变 —— 签名依然有效，篡改根本没发生，测试就失败了。
//
// 签名段首字符的 6 位全是有效数据，改它必然改变签名字节。
// 换成的字符还要避开原字符：撞上了等于没改（概率 1/64），同样是偶发通过。
func tamperSignature(t *testing.T, token string) string {
	t.Helper()

	dot := strings.LastIndex(token, ".")
	if dot < 0 || dot+1 >= len(token) {
		t.Fatalf("令牌不是 header.payload.signature 形式，取不到签名段: %q", token)
	}

	repl := byte('A')
	if token[dot+1] == repl {
		repl = 'B'
	}
	return token[:dot+1] + string(repl) + token[dot+2:]
}

func TestParseRejectsTamperedToken(t *testing.T) {
	pair, _ := GenerateTokenPair(7, "alice", testSecret, 2*time.Hour, 168*time.Hour)
	tampered := tamperSignature(t, pair.AccessToken)

	if _, err := ParseToken(tampered, testSecret); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("篡改签名应返回 ErrTokenInvalid，实际 %v", err)
	}
}

func TestParseRejectsWrongSecret(t *testing.T) {
	pair, _ := GenerateTokenPair(7, "alice", testSecret, 2*time.Hour, 168*time.Hour)

	if _, err := ParseToken(pair.AccessToken, "another_secret_at_least_32_chars!"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("密钥不匹配应返回 ErrTokenInvalid，实际 %v", err)
	}
}

func TestParseRejectsExpiredToken(t *testing.T) {
	// 负数有效期 = 签出来的那一刻就已经过期
	pair, _ := GenerateTokenPair(7, "alice", testSecret, -time.Hour, 168*time.Hour)

	// 过期要和"令牌是假的"分开报：前端据此决定是去刷新还是重新登录
	if _, err := ParseToken(pair.AccessToken, testSecret); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("过期应返回 ErrTokenExpired，实际 %v", err)
	}
}

func TestParseRejectsAlgNone(t *testing.T) {
	// 手工构造一个 alg=none 的令牌：
	// header {"alg":"none","typ":"JWT"} + payload {"uid":1,"uname":"hacker"}
	// 不做算法校验的实现会接受它，等于任何人都能伪造任意身份。
	header := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0"
	payload := "eyJ1aWQiOjEsInVuYW1lIjoiaGFja2VyIiwidHlwIjoiYWNjZXNzIn0"
	forged := header + "." + payload + "."

	if _, err := ParseToken(forged, testSecret); err == nil {
		t.Fatal("alg=none 的伪造令牌竟然通过了校验")
	}
}

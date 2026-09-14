package config

import (
	"strings"
	"testing"
	"time"
)

// setRequired 填上两个 required 变量。
//
// t.Setenv 会在用例结束时自动还原，用例之间不会互相污染。
// 显式设成空串而不是"不设"，是因为 godotenv 只覆盖未设置的变量——
// 万一本地存在 backend/.env，空串能保证结果仍然确定。
func setRequired(t *testing.T, secret string) {
	t.Helper()
	t.Setenv("JWT_SECRET", secret)
	t.Setenv("AI_SERVICE_TOKEN", "test-internal-token")
}

func TestLoadRejectsEmptyJWTSecret(t *testing.T) {
	setRequired(t, "")

	if _, err := Load(); err == nil {
		t.Fatal("JWT_SECRET 为空时必须报错，否则程序会带着空密钥启动，令牌可被任意伪造")
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	// required 只管非空，长度是 validate() 自己查的，这条用例守的就是它
	setRequired(t, strings.Repeat("a", jwtSecretMinLength-1))

	if _, err := Load(); err == nil {
		t.Fatal("JWT_SECRET 短于 32 字符时必须报错")
	}
}

func TestLoadUsesDefaults(t *testing.T) {
	setRequired(t, strings.Repeat("a", jwtSecretMinLength))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}

	if cfg.Server.Port != "8080" {
		t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "8080")
	}
	// 这两个默认值直接决定令牌有效期，改错的后果是登录态行为异常，所以要守住
	if cfg.JWT.AccessTokenExpire != 2*time.Hour {
		t.Errorf("JWT.AccessTokenExpire = %v, want %v", cfg.JWT.AccessTokenExpire, 2*time.Hour)
	}
	if cfg.JWT.RefreshTokenExpire != 168*time.Hour {
		t.Errorf("JWT.RefreshTokenExpire = %v, want %v", cfg.JWT.RefreshTokenExpire, 168*time.Hour)
	}
	if len(cfg.Server.CORSAllowOrigins) != 1 || cfg.Server.CORSAllowOrigins[0] != "http://localhost:5173" {
		t.Errorf("CORSAllowOrigins 解析不对: %v", cfg.Server.CORSAllowOrigins)
	}
}

func TestDatabaseConfigDSN(t *testing.T) {
	d := DatabaseConfig{
		Host: "localhost", Port: "5432",
		User: "u", Password: "p", Name: "n", SSLMode: "disable",
	}
	want := "host=localhost port=5432 user=u password=p dbname=n sslmode=disable"

	if got := d.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

// Package config 把环境变量加载成结构化的配置。
//
// 全项目只有这个包读环境变量，其他地方一律用 main.go 注入的 *Config，
// 不许自己调 os.Getenv。这样"项目依赖哪些配置"看这一个文件就够，
// 改配置项也只改这一处。
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

const (
	// jwtSecretMinLength 是 JWT_SECRET 的最短长度。
	// AGENTS §4.3 的硬性要求：太短的密钥让 HS256 签名可被暴力破解。
	jwtSecretMinLength = 32

	// MomentGeneratorAI 优先调用 ai-service 生成朋友圈，失败回退模板。
	MomentGeneratorAI = "ai"
	// MomentGeneratorTemplate 完全不走 AI，只跑模板（调试 / 应急降级）。
	MomentGeneratorTemplate = "template"
)

// Config 是全部配置的总入口，四个分组各管一摊。
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	AI       AIConfig

	// ProactiveJobInterval 是主动消息定时扫描的周期（定时任务一律在 Go 侧，AGENTS §4.8）
	ProactiveJobInterval time.Duration `env:"PROACTIVE_JOB_INTERVAL" envDefault:"5m"`

	// MomentJobInterval 是朋友圈动态生成的周期；同时用作同一人设的节流阈值（ai-moment spec §6.4）
	MomentJobInterval time.Duration `env:"MOMENT_JOB_INTERVAL" envDefault:"30m"`

	// MomentGenerator 是朋友圈生成器档位：ai（默认，调 ai-service，失败回退模板）
	// / template（纯模板，调试或应急降级用）。
	MomentGenerator string `env:"MOMENT_GENERATOR" envDefault:"ai"`
}

// ServerConfig 是 HTTP 服务本身的配置。
type ServerConfig struct {
	Port string `env:"SERVER_PORT" envDefault:"8080"`
	// 逗号分隔的多个来源。本地开发时是 Vue 的 dev server；
	// 生产走 Nginx 同源代理，不需要 CORS，这个值用不上。
	CORSAllowOrigins []string `env:"CORS_ALLOW_ORIGINS" envDefault:"http://localhost:5173" envSeparator:","`
}

// DatabaseConfig 是 PostgreSQL 的连接参数，字段与 backend/.env.example 一一对应。
type DatabaseConfig struct {
	Host     string `env:"DB_HOST" envDefault:"localhost"`
	Port     string `env:"DB_PORT" envDefault:"5432"`
	User     string `env:"DB_USER" envDefault:"heart_echo"`
	Password string `env:"DB_PASSWORD" envDefault:"change_me"`
	Name     string `env:"DB_NAME" envDefault:"heart_echo"`
	SSLMode  string `env:"DB_SSLMODE" envDefault:"disable"`
}

// JWTConfig 是双令牌的配置。
//
// 有效期由这里传给 pkg/jwt，不在那边写死——否则会和 .env 形成两个事实来源，
// 改了 .env 不生效，排查起来很费劲。
type JWTConfig struct {
	Secret             string        `env:"JWT_SECRET,required"`
	AccessTokenExpire  time.Duration `env:"JWT_ACCESS_EXPIRE" envDefault:"2h"`
	RefreshTokenExpire time.Duration `env:"JWT_REFRESH_EXPIRE" envDefault:"168h"`
}

// AIConfig 是调用 Python AI 服务的参数。
type AIConfig struct {
	ServiceURL string `env:"AI_SERVICE_URL" envDefault:"http://localhost:8000"`
	// ServiceToken 必须与 ai-service 的 AI_SERVICE_TOKEN 完全一致，
	// 不一致的表现是内部调用恒 401，且很难从日志看出来。
	ServiceToken string `env:"AI_SERVICE_TOKEN,required"`
}

// Load 读取环境变量并组装配置。
//
// 分两步：先把 .env 灌成环境变量，再从环境变量取值。
// 第二步只认环境变量这一个来源，所以本地（靠 .env）和容器（靠 compose 注入）
// 走的是同一条代码路径。
func Load() (*Config, error) {
	// .env 只在本地开发时存在。容器里由 compose 直接注入环境变量，
	// 文件不存在是正常情况，所以这里忽略错误。
	// 路径相对于工作目录，AGENTS §2 规定在 backend/ 下启动服务，因此是 .env。
	_ = godotenv.Load(".env")

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, fmt.Errorf("config: parse env: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validate 负责 env tag 表达不了的规则。
//
// tag 里的 required 只能保证"非空"，管不了长度和格式，
// 所以这类校验必须写在这里，否则会一路漏到运行时才发现。
func (c Config) validate() error {
	if len(c.JWT.Secret) < jwtSecretMinLength {
		return fmt.Errorf(
			"config: JWT_SECRET must be at least %d characters, got %d",
			jwtSecretMinLength, len(c.JWT.Secret),
		)
	}
	// time.NewTicker 对非正数 interval 会 panic，所以在启动期拦下
	if c.ProactiveJobInterval <= 0 {
		return fmt.Errorf(
			"config: PROACTIVE_JOB_INTERVAL must be positive, got %s",
			c.ProactiveJobInterval,
		)
	}
	if c.MomentJobInterval <= 0 {
		return fmt.Errorf(
			"config: MOMENT_JOB_INTERVAL must be positive, got %s",
			c.MomentJobInterval,
		)
	}
	// 拼错时启动即失败，避免静默退回某一档（Python 侧用 Literal 表达同一约束）
	switch c.MomentGenerator {
	case MomentGeneratorAI, MomentGeneratorTemplate:
	default:
		return fmt.Errorf(
			"config: MOMENT_GENERATOR must be %q or %q, got %q",
			MomentGeneratorAI, MomentGeneratorTemplate, c.MomentGenerator,
		)
	}
	return nil
}

// DSN 拼出 PostgreSQL 连接串，供 GORM 初始化使用。
//
// 放这里而不是 main.go，是因为它纯粹是配置字段的拼接，不含任何业务判断。
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

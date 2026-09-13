// Package logger 封装项目统一的日志器。
//
// 全项目只有这个文件 import zap，其他包一律通过 *zap.Logger 记日志。
// 好处是以后换日志库只改这一个文件，业务代码一行不动。
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New 创建项目统一的日志器。
//
// 格式由 GIN_MODE 决定：
//   - release        → JSON（给容器和日志采集系统看，机器可解析）
//   - 其他（含未设置）→ 人类可读（本地开发用，带颜色）
//
// 这里直接读环境变量而不是调 gin.Mode()，是为了不让 pkg/ 下的通用工具包
// 反向依赖 gin 这个 Web 框架——pkg/ 的判据是"能脱离本项目复用"。
//
// 返回值由 main.go 创建一次后注入各中间件，不做包级全局变量，
// 否则多个测试之间会共用同一个 logger，互相污染。
//
// 调用方在进程退出前应执行 defer logger.Sync() 把缓冲区刷盘。
func New() *zap.Logger {
	if os.Getenv("GIN_MODE") == "release" {
		return newJSONLogger()
	}
	return newConsoleLogger()
}

// newJSONLogger 生产格式：一行一条 JSON，方便被采集系统按字段检索。
func newJSONLogger() *zap.Logger {
	cfg := baseEncoderConfig()
	return zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(cfg),
		zapcore.Lock(os.Stdout),
		zap.InfoLevel, // 生产只记 INFO 及以上，DEBUG 会淹掉真正的问题
	), zap.AddCaller())
}

// newConsoleLogger 开发格式：缩进对齐、级别带颜色，人扫一眼就能定位。
func newConsoleLogger() *zap.Logger {
	cfg := baseEncoderConfig()
	// 只有开发格式上色：JSON 里混入 ANSI 转义码会让采集系统解析失败
	cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zap.New(zapcore.NewCore(
		zapcore.NewConsoleEncoder(cfg),
		zapcore.Lock(os.Stdout),
		zap.DebugLevel, // 开发期全都要，方便排查
	), zap.AddCaller())
}

// baseEncoderConfig 两种格式共用的字段配置。
func baseEncoderConfig() zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	// 默认的时间是浮点时间戳（1757...），人读不出来；
	// 换成 RFC3339，与接口契约里的时间格式保持一致。
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.EncodeLevel = zapcore.CapitalLevelEncoder
	return cfg
}

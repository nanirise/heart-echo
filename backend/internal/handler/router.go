// Package handler 存放 HTTP 处理器与路由注册。
package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/internal/middleware"
	"github.com/nanirise/heart-echo/backend/internal/service"
)

// NewRouter 组装中间件链与全部路由，由 cmd/server 在启动时调用一次。
//
// 各模块在自己的 handler 文件里提供 RegisterXxxRoutes(rg, h)，本文件只加一行汇总调用——
// 目的是让三个人不必同时改这个文件（MASTER §4.5）。
func NewRouter(cfg *config.Config, logger *zap.Logger, db *gorm.DB) *gin.Engine {
	// gin.New() 而不是 gin.Default()：后者会塞进 gin 自带的 Logger 与 Recovery，
	// 与我们自己的中间件重复输出两遍日志。
	r := gin.New()

	// 链序不可调整，见 AGENTS §4.2。
	r.Use(
		middleware.Recovery(logger),
		middleware.RequestLogger(logger),
		middleware.CORS(cfg.Server.CORSAllowOrigins),
		// BizErrorHandler 挂在 engine 上而不是 api 组上：AGENTS §4.2 把整条链写成一串，
		// 挂这里效果相同（都在业务 handler 之前先 c.Next()），少一层嵌套。
		// 它只处理写进 c.Errors 的错误——未命中路由的 404 走 gin 默认响应，不经过这里。
		middleware.BizErrorHandler(logger),
	)

	api := r.Group("/api/v1")

	// 免鉴权区：契约 §1 的白名单共 4 个端点，本轮只有 /health 落地，
	// 另三个 auth 端点由 feature/auth-login 挂在同一个 api 组上。
	api.GET("/health", healthHandler(cfg, db))

	// 业务路由一律挂 protected：它比 api 多一道 JWTAuth。
	protected := api.Group("")
	protected.Use(middleware.JWTAuth(cfg.JWT.Secret))

	// 依赖在此逐层装配：装配点全局只此一处，单测才能整体替换（plan §3.1）。
	personaService := service.NewPersonaService(db)
	RegisterPersonaRoutes(protected, NewPersonaHandler(personaService))

	return r
}

package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// AuthHandler 是认证三个**免鉴权**端点的 HTTP 层：绑定 → 调 service → 写响应。
//
// 三个端点都不读 userID，与 PersonaHandler 最大的不同：注册和登录时用户还没有身份，
// 刷新的身份来自请求体里那枚已验签的 token，都不是从 Context 取的。
// 因此这里没有 currentUserID 调用，缺 token 也不该返回 4010 —— 它们在白名单里。
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler 构造 handler。
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// RegisterAuthRoutes 由 router.go 汇总调用。
//
// ⚠️ 挂在**免鉴权的 api 组**上，不是 protected：契约 §1 的白名单里就有这三个端点。
// 挂到 protected 上，等于要求"先登录才能登录"，前端会卡在登录页出不去。
func RegisterAuthRoutes(rg *gin.RouterGroup, h *AuthHandler) {
	g := rg.Group("/auth")
	{
		g.POST("/register", h.Register)
		g.POST("/login", h.Login)
		g.POST("/refresh", h.Refresh)
	}
}

// Register 处理 POST /auth/register。
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 上抛的必须是 BizError：原始绑定错误不是 BizError，会被 BizErrorHandler 归成 5000。
		// 原始 err 只进日志、不返回前端 —— 它会把校验规则的细节吐给调用方。
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// Login 处理 POST /auth/login。
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// Refresh 处理 POST /auth/refresh。
//
// 响应 data 与上面两个不同：契约 §3.3 只给一对令牌，不带 user ——
// 刷新拿到的用户信息不保证是最新的，契约刻意不让它成为"读用户信息"的第二个入口。
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	pair, err := h.authService.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *pair)
}

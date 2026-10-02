package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// ProactiveHandler 是主动消息 settings 两个端点的 HTTP 层：绑定 → 取 userID → 调 service → 写响应。
type ProactiveHandler struct {
	proactiveService *service.ProactiveService
}

// NewProactiveHandler 构造 handler。
func NewProactiveHandler(proactiveService *service.ProactiveService) *ProactiveHandler {
	return &ProactiveHandler{proactiveService: proactiveService}
}

// RegisterProactiveRoutes 由 router.go 汇总调用，挂**需鉴权**的 settings 两个端点。
//
// ⚠️ 必须挂在 protected 组上：两个端点读写的都是"令牌里那个人"的配置，
// 没有 userID 入参，越权入口要在这一层就不存在。
func RegisterProactiveRoutes(rg *gin.RouterGroup, h *ProactiveHandler) {
	g := rg.Group("/proactive")
	{
		g.GET("/settings", h.GetSettings)
		g.PUT("/settings", h.UpdateSettings)
		// TODO: POST /trigger —— 依赖 TriggerNow 落定后挂上（本支不写触发链路）
		// g.POST("/trigger", h.Trigger)
	}
}

// GetSettings 处理 GET /proactive/settings。
func (h *ProactiveHandler) GetSettings(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	var query dto.GetSettingsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		// 缺 / 非法 personaId 都是 4001（proactive-setting §2.3 的契约空白处）：
		// 缺参时没有归属可校验，不能退化成 personaId=0 查库。
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.proactiveService.GetSettings(c.Request.Context(), userID, query.PersonaID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// UpdateSettings 处理 PUT /proactive/settings。
func (h *ProactiveHandler) UpdateSettings(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	var req dto.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.proactiveService.UpdateSettings(c.Request.Context(), userID, &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

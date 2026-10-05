package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// ProactiveHandler 是主动消息三个端点的 HTTP 层：绑定 → 取 userID → 调 service → 写响应。
type ProactiveHandler struct {
	proactiveService *service.ProactiveService
}

// NewProactiveHandler 构造 handler。
func NewProactiveHandler(proactiveService *service.ProactiveService) *ProactiveHandler {
	return &ProactiveHandler{proactiveService: proactiveService}
}

// RegisterProactiveRoutes 由 router.go 汇总调用，挂**需鉴权**的主动消息三个端点。
//
// ⚠️ 必须挂在 protected 组上：三个端点读写的都是"令牌里那个人"的配置与对话，
// 没有 userID 入参，越权入口要在这一层就不存在。
func RegisterProactiveRoutes(rg *gin.RouterGroup, h *ProactiveHandler) {
	g := rg.Group("/proactive")
	{
		g.GET("/settings", h.GetSettings)
		g.PUT("/settings", h.UpdateSettings)
		// 🚨 答辩演示的硬性依赖，不是调试后门：与定时任务调同一个 TriggerNow（spec §4）
		g.POST("/trigger", h.Trigger)
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

// Trigger 处理 POST /proactive/trigger（契约 §9）。
//
// 它对本模块的作用与定时扫描完全同源：同一个 TriggerNow，只是"谁来点名"不同
// （手动点一个 personaId，定时扫全部 enabled）。判定与注入不分叉。
func (h *ProactiveHandler) Trigger(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	var req dto.TriggerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 缺 / 非法 personaId 都是 4001，与 GET /settings 同一口径（契约 §9 变更记录 2026-09-20）
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	res, err := h.proactiveService.TriggerNow(c.Request.Context(), userID, req.PersonaID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// (nil, nil) = 判定未通过（关着 / 没到空闲阈值 / 到日上限 / 刚聊过），不是错误。
	// 契约 §9 只给了成功形态、没有"被拦下"的表达，零值响应是群内对齐的口径：
	// messageId=0 / content="" / createdAt=null，前端按 messageId === 0 区分。
	if res == nil {
		response.Success(c, dto.TriggerResponse{})
		return
	}
	response.Success(c, dto.TriggerResponse{
		MessageID: res.MessageID,
		Content:   res.Content,
		CreatedAt: &res.CreatedAt,
	})
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

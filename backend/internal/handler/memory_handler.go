package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// MemoryHandler 是记忆与画像两个只读端点（契约 §7）的 HTTP 层。
// 两者都是一人设一份，personaId 必传；记忆只读，没有删除 / 修改端点。
type MemoryHandler struct {
	memoryService *service.MemoryService
}

// NewMemoryHandler 构造 handler。
func NewMemoryHandler(memoryService *service.MemoryService) *MemoryHandler {
	return &MemoryHandler{memoryService: memoryService}
}

// RegisterMemoryRoutes 由 router.go 汇总调用。
func RegisterMemoryRoutes(rg *gin.RouterGroup, h *MemoryHandler) {
	rg.GET("/memory", h.List)
	rg.GET("/profile/portrait", h.GetPortrait)
}

// List 处理 GET /memory?personaId=&page=&pageSize=。
func (h *MemoryHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	var query dto.MemoryListQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		// 缺 / 非法 personaId 是 4001：此刻还没有归属可校验，不能退化成 0 查库。
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	// 解析失败按未传处理（=0），交给 ClampPage 收敛；分页参数不产生错误码
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))

	result, err := h.memoryService.ListMemories(
		c.Request.Context(), userID, query.PersonaID, page, pageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *result)
}

// GetPortrait 处理 GET /profile/portrait?personaId=。
func (h *MemoryHandler) GetPortrait(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	var query dto.PortraitQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	result, err := h.memoryService.GetPortrait(c.Request.Context(), userID, query.PersonaID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *result)
}

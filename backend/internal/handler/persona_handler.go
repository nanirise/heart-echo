package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/middleware"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// PersonaHandler 是 4 个人设端点的 HTTP 层：只做绑定、取 userID、调 service、写响应。
type PersonaHandler struct {
	personaService *service.PersonaService
}

// NewPersonaHandler 构造 handler。
func NewPersonaHandler(personaService *service.PersonaService) *PersonaHandler {
	return &PersonaHandler{personaService: personaService}
}

// RegisterPersonaRoutes 由 router.go 汇总调用。
func RegisterPersonaRoutes(rg *gin.RouterGroup, h *PersonaHandler) {
	g := rg.Group("/personas")
	{
		g.GET("", h.List)
		g.POST("", h.Create)
		g.PUT("/:id", h.Update)
		g.DELETE("/:id", h.Delete)
	}
}

// currentUserID 是 4 个端点唯一的 userID 读取点。
// gin v1.12 的 GetUint64 只返回值（内部是直接类型断言，类型不符时静默返回零值），
// 所以判据只能是 0 —— BIGSERIAL 从 1 起，0 不可能是合法用户。
// 鉴权失败不许降级成 0：那会让查询返回空集，把鉴权失败伪装成"没有数据"。
func currentUserID(c *gin.Context) (uint64, bool) {
	userID := c.GetUint64(middleware.ContextKeyUserID)
	return userID, userID != 0
}

// idParam 解析路径参数 :id。非数字与 0 一律按"人设不存在"处理 ——
// 契约 §4 这组端点的错误码里没有 4001，返回 4001 就是契约漂移。
func idParam(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return id, true
}

// List 处理 GET /personas。
func (h *PersonaHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	// 解析失败按未传处理（=0），交给 ClampPage 收敛；分页参数不产生错误码
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))

	result, err := h.personaService.List(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *result)
}

// Create 处理 POST /personas。
func (h *PersonaHandler) Create(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	var req dto.CreatePersonaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 上抛的必须是 BizError：原始绑定错误不是 BizError，会被 BizErrorHandler 归成 5000。
		// 原始 err 只进日志、不返回前端。
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.personaService.Create(c.Request.Context(), userID, &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// Update 处理 PUT /personas/:id。
func (h *PersonaHandler) Update(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	personaID, ok := idParam(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrPersonaNotFound))
		return
	}

	var req dto.UpdatePersonaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.personaService.Update(c.Request.Context(), userID, personaID, &req)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// Delete 处理 DELETE /personas/:id。
func (h *PersonaHandler) Delete(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	personaID, ok := idParam(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrPersonaNotFound))
		return
	}

	if err := h.personaService.Delete(c.Request.Context(), userID, personaID); err != nil {
		_ = c.Error(err)
		return
	}
	// data 必须是 null；Success(c, nil) 编译不过（泛型 T 无法从 untyped nil 推断）
	response.Success[any](c, nil)
}

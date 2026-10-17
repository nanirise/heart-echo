package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// MomentHandler 是朋友圈 4 个端点的 HTTP 层：绑定、取 userID、调 service、写响应。
type MomentHandler struct {
	momentService *service.MomentService
}

// NewMomentHandler 构造 handler。
func NewMomentHandler(momentService *service.MomentService) *MomentHandler {
	return &MomentHandler{momentService: momentService}
}

// RegisterMomentRoutes 由 router.go 汇总调用，必须挂在 protected 组上。
func RegisterMomentRoutes(rg *gin.RouterGroup, h *MomentHandler) {
	g := rg.Group("/moments")
	{
		g.GET("", h.List)
		g.POST("/:id/like", h.Like)
		g.GET("/:id/comments", h.ListComments)
		g.POST("/:id/comments", h.CreateComment)
	}
}

// List 处理 GET /moments。
func (h *MomentHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	// 解析失败按未传处理（=0），交给 ClampPage 收敛；分页参数不产生错误码
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))

	result, err := h.momentService.ListMoments(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *result)
}

// Like 处理 POST /moments/:id/like。
func (h *MomentHandler) Like(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	momentID, ok := idParam(c)
	if !ok {
		// 动态资源统一 4040：契约 §8 的这组端点没有 4001，也没有 4043（那是人设专用）
		_ = c.Error(errcode.New(errcode.ErrNotFound))
		return
	}

	resp, err := h.momentService.LikeMoment(c.Request.Context(), userID, momentID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

// ListComments 处理 GET /moments/:id/comments。
func (h *MomentHandler) ListComments(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	momentID, ok := idParam(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrNotFound))
		return
	}

	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))

	result, err := h.momentService.ListComments(
		c.Request.Context(), userID, momentID, page, pageSize,
	)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *result)
}

// CreateComment 处理 POST /moments/:id/comments。
func (h *MomentHandler) CreateComment(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	momentID, ok := idParam(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrNotFound))
		return
	}

	var req dto.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	resp, err := h.momentService.CreateComment(
		c.Request.Context(), userID, momentID, req.Content,
	)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *resp)
}

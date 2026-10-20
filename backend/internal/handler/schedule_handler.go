package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// ScheduleHandler 日程的三个 HTTP 端点。
//
// 刻意没有 POST /schedules：日程只能从对话里抽取产生（契约 §10 / AGENTS §7.5）。
type ScheduleHandler struct {
	scheduleService *service.ScheduleService
}

// NewScheduleHandler 构造 handler。
func NewScheduleHandler(scheduleService *service.ScheduleService) *ScheduleHandler {
	return &ScheduleHandler{scheduleService: scheduleService}
}

// RegisterScheduleRoutes 注册日程路由，router.go 只加一行汇总调用。
func RegisterScheduleRoutes(rg *gin.RouterGroup, h *ScheduleHandler) {
	g := rg.Group("/schedules")
	{
		g.GET("", h.List)
		g.DELETE("/:id", h.Cancel)
		// 答辩演示的硬性依赖：与定时任务复用同一个 TriggerSchedule（契约 §10）。
		g.POST("/:id/trigger", h.Trigger)
	}
}

// List 处理 GET /schedules?personaId=&page=&pageSize=。
func (h *ScheduleHandler) List(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	var query dto.ListSchedulesQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		// 缺 / 非法 personaId 是 4001：此刻还没有归属可校验，不能退化成 0 查库。
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))

	result, err := h.scheduleService.ListSchedules(
		c.Request.Context(), userID, query.PersonaID, page, pageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}
	response.Success(c, *result)
}

// Cancel 处理 DELETE /schedules/:id（软删除，data 为 null）。
func (h *ScheduleHandler) Cancel(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	scheduleID, ok := idParam(c)
	if !ok {
		// 契约 §10 这组端点只列了 4040：非数字 / 0 一律按"日程不存在"处理，
		// 不暴露路径参数解析细节。
		_ = c.Error(errcode.New(errcode.ErrNotFound))
		return
	}

	if err := h.scheduleService.CancelSchedule(c.Request.Context(), userID, scheduleID); err != nil {
		_ = c.Error(err)
		return
	}
	// data 必须是 null；Success(c, nil) 无法推断泛型实参，编译不过。
	response.Success[any](c, nil)
}

// Trigger 处理 POST /schedules/:id/trigger（演示依赖，允许提前触发）。
func (h *ScheduleHandler) Trigger(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		return
	}

	scheduleID, ok := idParam(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrNotFound))
		return
	}

	result, err := h.scheduleService.TriggerSchedule(c.Request.Context(), userID, scheduleID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	// 响应结构与 /proactive/trigger 逐字相同（契约 §10 与 §9），复用同一个 DTO。
	response.Success(c, dto.TriggerResponse{
		MessageID: result.MessageID,
		Content:   result.Content,
		CreatedAt: &result.CreatedAt,
	})
}

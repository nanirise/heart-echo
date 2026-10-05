package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/service"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// ChatHandler 是对话链路的 HTTP 层：SSE 流式对话 + 历史消息分页读。
//
// 它只做三件事：取 userID、绑请求体、把结果写成 SSE 或 JSON。
// 所有业务规则（归属、落库、错误码）都在 service 里，handler 不重复判断——
// 同一套规则写两遍，迟早只改一处。
type ChatHandler struct {
	chatService *service.ChatService
}

// NewChatHandler 构造 handler。
func NewChatHandler(chatService *service.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

// RegisterChatRoutes 由 router.go 汇总调用。
//
// 两个端点：/stream 是本功能（chat-stream）的；/personas/:personaId/messages 是
// chat-message spec 的 R 端点，一直标着 ⬜ 没做，导致前端刷新后历史全区（2026-10-05 补）。
func RegisterChatRoutes(rg *gin.RouterGroup, h *ChatHandler) {
	g := rg.Group("/chat")
	{
		g.POST("/stream", h.Stream)
		g.GET("/personas/:personaId/messages", h.ListMessages)
	}
}

// Stream 处理 POST /chat/stream，是本功能唯一一个"响应不是 JSON"的端点。
func (h *ChatHandler) Stream(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	var req dto.StreamChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 上抛的必须是 BizError，理由同 persona_handler：原始绑定错误会被归成 5000
		_ = c.Error(errcode.New(errcode.ErrInvalidParams))
		return
	}

	// ⚠️ 这一行是一道单向门（spec §1.1）。
	// 返回 err 时流还没开，下面是普通 JSON 错误，前端 response.ok === false 会抛 ApiError；
	// 返回 channel 时说明所有校验都过了，下面立刻要写 SSE 响应头——
	// 从那之后任何错误都再也回不到 4xx，只能走 error 事件 + HTTP 200。
	ch, err := h.chatService.StreamChat(c.Request.Context(), userID, req)
	if err != nil {
		_ = c.Error(err)
		return
	}

	writeSSEHeaders(c)

	// range 到 channel 关闭为止。channel 由 service 侧 close，
	// 所有出口（正常收尾 / 5001 / 5002 / 5003 / 客户端断开）都会走到 close。
	for ev := range ch {
		writeChatEvent(c, ev)
	}
}

// ListMessages 处理 GET /chat/personas/:personaId/messages（契约 §5，chat-message spec §3.2）。
//
// 它是本功能唯一一个"响应是普通 JSON"的端点——与 /stream 相反，这里的错误能正常回 4xx。
func (h *ChatHandler) ListMessages(c *gin.Context) {
	userID, ok := currentUserID(c)
	if !ok {
		_ = c.Error(errcode.New(errcode.ErrUnauthorized))
		return
	}

	// 解析失败就把 personaID 留成 0 传下去，**不写特判**（chat-message spec §3.2 第 6 条）：
	// WHERE id = 0 AND user_id = ? 天然不命中，与"这不是你的人设"收敛到同一条 4043 出口。
	// 契约给本端点只列了 4043，凭空多一个 4001 就是契约漂移；少一个分支也少一处不一致。
	personaID, _ := strconv.ParseUint(c.Param("personaId"), 10, 64)

	// 解析失败同样按未传处理（=0），交给 ClampPage 收敛成 1 / 20。
	// 分页参数不产生错误码：pageSize 上限必须有，否则一次拉全表。
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("pageSize"))

	result, err := h.chatService.ListMessages(c.Request.Context(), userID, personaID, page, pageSize)
	if err != nil {
		_ = c.Error(err)
		return
	}

	response.Success(c, *result)
}

// writeSSEHeaders 写 SSE 的四项响应头并立刻把它们推出去。
//
// 四项一个都不能少：Content-Type 决定前端怎么解析，Cache-Control / Connection
// 防中间层缓存或提前关闭，X-Accel-Buffering 关掉 Nginx 的响应缓冲。
func writeSSEHeaders(c *gin.Context) {
	h := c.Writer.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	// Nginx 默认会把上游响应攒起来再转发；不关掉的话，字符一个不少，
	// 但是"全部一起出现"，打字机效果直接没了（spec §1.4）。
	h.Set("X-Accel-Buffering", "no")

	c.Writer.WriteHeader(http.StatusOK)
	// 立刻推出去：不 Flush 的话响应头可能一直躺在 Go 的缓冲里，
	// 前端要等到第一个 delta 才知道这次请求成功了。
	c.Writer.Flush()
}

// writeChatEvent 写一个事件块，格式固定为 "event: <名>\ndata: <json>\n\n"。
func writeChatEvent(c *gin.Context, ev service.ChatEvent) {
	// data 的形状按事件类型固定（契约 §6 只有这三种）。
	// 用一个 any 而不是三份判断，是为了让 payload 一定出现在同一处，
	// 加第四种事件时改这一个 switch 就够。
	var payload any
	switch ev.Type {
	case service.ChatEventDelta:
		payload = map[string]string{"text": ev.Text}
	case service.ChatEventDone:
		payload = map[string]uint64{"messageId": ev.MessageID}
	case service.ChatEventError:
		payload = map[string]any{"code": ev.Code, "message": ev.Message}
	default:
		// 未知事件名一律不发。宁可少一条，也不能往契约外塞新事件——
		// 前端不认的事件会被静默忽略，看上去像"偶发丢字"。
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		// payload 全是基本类型，Marshal 不会失败；真失败也只能放弃这一条，
		// 不能退而写一个非 JSON 字符串：前端 JSON.parse 会当场抛，整条流就断在这里。
		return
	}

	// 分隔符是 \n\n，不是 \r\n\r\n（契约 §6 已钉死）。两种写法在本地 Windows
	// 都正常，差别只在部署到 Linux 之后才出现，那时的现象是"页面一个字都不出"。
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", ev.Type, data)

	// 每写一条都 Flush。它管的是 Go 自己的响应缓冲层，
	// 与 X-Accel-Buffering（管 Nginx 那一层）是两个东西，两个都要（spec §1.4）。
	c.Writer.Flush()
}

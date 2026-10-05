package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// ChatRequest 是 Go → Python 的请求体（chat-stream spec §2.2）。
// 刻意保持 snake_case：它跟前端那个 {personaId, content} 长得不一样，
// 两边混起来会立刻解析报错，而不是悄悄串味。
type ChatRequest struct {
	UserID    uint64 `json:"user_id"`
	PersonaID uint64 `json:"persona_id"`
	Message   string `json:"message"`
}

// StreamEvent 是 ai-service 下发的一条事件，Type 只有三种，见下面的常量。
type StreamEvent struct {
	Type    string // delta | end | error
	Text    string // 仅 delta
	Code    int    // 仅 error
	Message string // 仅 error
}

const (
	EventDelta = "delta"
	EventEnd   = "end"
	EventError = "error"
)

// AIClient 是 Go 侧调用 Python AI 服务的唯一出入口。
//
// 只放 StreamChat：TECH_DESIGN §5.0 规划的完整形状还有 AnalyzeEmotion /
// ExtractMemories，那是后续的事，没有实现的接口方法只会变成噪音。
type AIClient interface {
	// StreamChat 发起一次流式生成。
	//
	// 返回的 error 只表示"没能开始"（连不上 / 非 200）：函数把 channel 交出去
	// 就返回了，之后再出的错没有机会走 error，只能从 channel 里发 EventError。
	StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error)
}

type aiHTTPClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewAIClient 构造 HTTP 实现。
func NewAIClient(cfg config.AIConfig) AIClient {
	// 克隆默认 Transport，只改"等响应头"的超时。
	// 不设 http.Client.Timeout：它算的是"从发出到读完 body"的总时间，
	// 而流式回复本来就要持续几十秒，设了会把正在流的回复直接掐断。
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = 10 * time.Second

	return &aiHTTPClient{
		baseURL: strings.TrimRight(cfg.ServiceURL, "/"),
		token:   cfg.ServiceToken,
		http:    &http.Client{Transport: tr},
	}
}

func (c *aiHTTPClient) StreamChat(ctx context.Context, req ChatRequest) (<-chan StreamEvent, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/internal/chat/stream", bytes.NewReader(body),
	)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrInternal, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	// 必须与 ai-service 的 AI_SERVICE_TOKEN 逐字相同。
	// 不一致的表现是恒 401，且日志里看不出是配置问题（AGENTS §4.3）。
	httpReq.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		// 还没开始流就失败：属于"AI 服务不可用"，不是"生成失败"
		return nil, errcode.Wrap(errcode.ErrAIUnavailable, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, errcode.Wrap(
			errcode.ErrAIUnavailable, fmt.Errorf("ai-service status %d", resp.StatusCode),
		)
	}

	ch := make(chan StreamEvent)
	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		// 契约 §6 把事件块分隔符写死成 \n\n。不能用默认的"按行切"：
		// 一个事件块横跨三行（event: / data: / 空行），按行切会被切碎。
		scanner.Split(splitSSEBlock)

		for scanner.Scan() {
			ev, ok := parseSSEBlock(scanner.Text())
			if !ok {
				continue
			}
			select {
			case ch <- ev:
			case <-ctx.Done():
				// 调用方已经不要了（浏览器断开）。继续推会永远卡在这里，
				// 这条 goroutine 就再也回收不了。
				return
			}
		}

		// 读到一半断了。ctx 没被取消才算真出错 —— 用户主动断开不算错。
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			select {
			case ch <- StreamEvent{
				Type:    EventError,
				Code:    int(errcode.ErrLLMFailed),
				Message: errcode.ErrLLMFailed.Message(),
			}:
			case <-ctx.Done():
			}
		}
	}()

	return ch, nil
}

// splitSSEBlock 按 \n\n 切块，切出来的内容不含结尾那两个换行。
func splitSSEBlock(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		return i + 2, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// parseSSEBlock 把 "event: delta\ndata: {...}" 变成 StreamEvent。
func parseSSEBlock(block string) (StreamEvent, bool) {
	var ev StreamEvent
	var data string

	for _, line := range strings.Split(block, "\n") {
		switch {
		case strings.HasPrefix(line, "event:"):
			ev.Type = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
	}
	// 空块（两个换行之间什么都没有）跳过
	if ev.Type == "" {
		return StreamEvent{}, false
	}

	switch ev.Type {
	case EventDelta:
		var payload struct {
			Text string `json:"text"`
		}
		_ = json.Unmarshal([]byte(data), &payload)
		ev.Text = payload.Text
	case EventError:
		var payload struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal([]byte(data), &payload)
		ev.Code, ev.Message = payload.Code, payload.Message
	}
	// end 事件的 data 是 {}，不需要解析任何东西
	return ev, true
}

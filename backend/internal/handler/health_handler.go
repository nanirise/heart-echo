package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/config"
	"github.com/nanirise/heart-echo/backend/pkg/response"
)

// dependencyCheckTimeout 是单个依赖探测的时限。
// 没有它的话，ai-service 网络不通时 /health 会一直挂着，
// 把 Docker healthcheck 一起拖死，最后表现为"容器反复重启"。
const dependencyCheckTimeout = 2 * time.Second

const (
	healthStatusOK       = "ok"
	healthStatusDegraded = "degraded"

	// 契约 §11 只定义了健康时的 "ok"，[假设] 异常时统一写 "down"。
	dependencyOK   = "ok"
	dependencyDown = "down"
)

// healthData 是 /health 响应体里的 data 部分，字段见 API_CONTRACT §11。
type healthData struct {
	Status       string            `json:"status"`
	Dependencies map[string]string `json:"dependencies"`
}

// healthHandler 报告本服务与两个依赖的连通状态，供 Docker healthcheck 与部署核对使用。
//
// 依赖挂掉时仍返回 HTTP 200 + code 200：端点本身执行成功了，依赖状态由 data 表达。
// 改成 5xx 的话，调用方就分不清"进程死了"和"进程活着但依赖挂了"。
func healthHandler(cfg *config.Config, db *gorm.DB) gin.HandlerFunc {
	// http.Client 按设计就是建一次、长期复用、并发安全的。
	// 超时也只在这一处配，将来要改连接池参数不必满文件找。
	client := &http.Client{Timeout: dependencyCheckTimeout}

	return func(c *gin.Context) {
		deps := map[string]string{
			"database":  checkDatabase(c.Request.Context(), db),
			"aiService": checkAIService(c.Request.Context(), client, cfg.AI.ServiceURL),
		}

		status := healthStatusOK
		for _, state := range deps {
			if state != dependencyOK {
				status = healthStatusDegraded
				break
			}
		}

		response.Success(c, healthData{Status: status, Dependencies: deps})
	}
}

// checkDatabase 用一次 Ping 探数据库连通性。
//
// 不看连接池里的现成状态：池里的连接可能是几分钟前建的，
// 中途数据库挂了它自己不会知道，读它等于报旧消息。
func checkDatabase(ctx context.Context, db *gorm.DB) string {
	sqlDB, err := db.DB()
	if err != nil {
		return dependencyDown
	}

	ctx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return dependencyDown
	}
	return dependencyOK
}

// checkAIService 探 ai-service 的 /health。
//
// 不带 X-Internal-Token：那是内部业务调用的凭证，
// 而 /health 是给 Docker healthcheck 用的端点，两边约定不同。
func checkAIService(ctx context.Context, client *http.Client, baseURL string) string {
	ctx, cancel := context.WithTimeout(ctx, dependencyCheckTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return dependencyDown
	}

	resp, err := client.Do(req)
	if err != nil {
		return dependencyDown
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return dependencyDown
	}
	return dependencyOK
}

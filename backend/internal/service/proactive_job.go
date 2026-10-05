package service

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// ProactiveJob 按 PROACTIVE_JOB_INTERVAL 周期扫描并触发主动消息。
// 定时任务一律跑在 Go 侧（AGENTS §4.8），不放 ai-service。
// 与手动端点共用同一个 TriggerNow（spec §4 / MASTER §4.2）。
type ProactiveJob struct {
	svc      *ProactiveService
	interval time.Duration
	logger   *zap.Logger
}

func NewProactiveJob(svc *ProactiveService, interval time.Duration, logger *zap.Logger) *ProactiveJob {
	return &ProactiveJob{svc: svc, interval: interval, logger: logger}
}

// Run 阻塞运行扫描循环，监听 ctx.Done() 退出。
// 在 cmd/server/main.go 里 go job.Run(ctx) 启动。
func (j *ProactiveJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	j.logger.Info("proactive job started", zap.Duration("interval", j.interval))

	for {
		select {
		case <-ctx.Done():
			j.logger.Info("proactive job stopped")
			return
		case <-ticker.C:
			j.scanOnce(ctx)
		}
	}
}

// scanOnce 扫一轮：拿 enabled 人设 → 逐行调 TriggerNow。
// TriggerNow 内部负责五层判定与 debug 日志（spec §3），本层只负责调度。
func (j *ProactiveJob) scanOnce(ctx context.Context) {
	rows, err := j.svc.repo.ListEnabledForScan(ctx)
	if err != nil {
		j.logger.Error("proactive scan: list failed", zap.Error(err))
		return
	}

	// 每一层判定不通过的 debug 日志由 TriggerNow 打印；本层只记本轮条数
	j.logger.Debug("proactive scan: candidates", zap.Int("count", len(rows)))

	for _, row := range rows {
		// 只看错误：判定未通过是 (nil, nil) 的正常态；触发成功产出的回复不属于本层
		// （前端从会话列表自己读），本层不接管它的去向。
		if _, err := j.svc.TriggerNow(ctx, row.UserID, row.PersonaID); err != nil {
			// TriggerNow 返回错误 = 基础设施失败（DB / 生成调用），不是"判定不通过"
			// 判定不通过是正常态，不应产生错误（spec §3.a）
			j.logger.Error("proactive trigger failed",
				zap.Uint64("userID", row.UserID),
				zap.Uint64("personaID", row.PersonaID),
				zap.Error(err))
		}
	}
}

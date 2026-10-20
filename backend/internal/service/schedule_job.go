package service

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// scheduleScanLimit 一轮最多处理多少条到期日程。
// 不设上限的话，数据库停摆半天后恢复的第一轮会连续调用几十次 LLM，
// 把 ai-service 打满；剩下的行下一轮（PROACTIVE_JOB_INTERVAL）继续处理。
const scheduleScanLimit = 100

// ScheduleJob 周期扫描到期的 pending 日程并触发提醒。
//
// 独立于 ProactiveJob：两者扫描对象不同，合并会牵连 main.go 的构造参数。
// 周期仍用 PROACTIVE_JOB_INTERVAL（AGENTS §4.8 规定两个扫描共用该变量）。
type ScheduleJob struct {
	svc      *ScheduleService
	interval time.Duration
	logger   *zap.Logger
}

// NewScheduleJob 构造定时任务。
func NewScheduleJob(svc *ScheduleService, interval time.Duration, logger *zap.Logger) *ScheduleJob {
	return &ScheduleJob{svc: svc, interval: interval, logger: logger}
}

// Run 启动 tick 循环，ctx 取消时退出（main.go 的优雅关闭会 cancel jobCtx）。
func (j *ScheduleJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	j.logger.Info("schedule job started", zap.Duration("interval", j.interval))
	for {
		select {
		case <-ctx.Done():
			j.logger.Info("schedule job stopped")
			return
		case <-ticker.C:
			j.scanOnce(ctx)
		}
	}
}

func (j *ScheduleJob) scanOnce(ctx context.Context) {
	due, err := j.svc.ListPendingDue(ctx, scheduleScanLimit)
	if err != nil {
		j.logger.Error("schedule scan: list due failed", zap.Error(err))
		return
	}
	if len(due) == 0 {
		return
	}

	j.logger.Debug("schedule scan: due found", zap.Int("count", len(due)))
	for i := range due {
		row := &due[i]
		// 走与手动触发完全相同的入口；行在扫描后可能已被取消，
		// 此时 TriggerSchedule 返回 4040，属于正常竞争而不是故障。
		if _, err := j.svc.TriggerSchedule(ctx, row.UserID, row.ID); err != nil {
			var bizErr *errcode.BizError
			if errors.As(err, &bizErr) && bizErr.Code == errcode.ErrNotFound {
				j.logger.Debug("schedule no longer pending before trigger",
					zap.Uint64("scheduleID", row.ID))
				continue
			}
			j.logger.Error("schedule trigger failed",
				zap.Uint64("scheduleID", row.ID),
				zap.Uint64("userID", row.UserID),
				zap.Error(err))
		}
	}
}

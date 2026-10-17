package service

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/internal/repository"
)

const (
	// momentRecentEventLimit 是喂给生成器的事件类记忆条数上限。
	momentRecentEventLimit = 5
	// aiCommentProbability 是每条新动态被同账号其他人设评论的概率。
	aiCommentProbability = 0.5
)

// MomentJob 按 MOMENT_JOB_INTERVAL 周期为全部人设生成朋友圈动态。
// 定时任务一律跑在 Go 侧（AGENTS §4.8）。
// ⚠️ 本 job 不提供手动触发接口：动态只由定时任务产生（AGENTS §4.6 / §7.5）。
type MomentJob struct {
	db        *gorm.DB
	repo      *repository.MomentRepo
	generator ContentGenerator
	interval  time.Duration
	logger    *zap.Logger
}

// NewMomentJob 构造定时任务；generator 由外部注入，阶段一传 TemplateGenerator。
func NewMomentJob(
	db *gorm.DB, generator ContentGenerator, interval time.Duration, logger *zap.Logger,
) *MomentJob {
	return &MomentJob{
		db:        db,
		repo:      repository.NewMomentRepo(db),
		generator: generator,
		interval:  interval,
		logger:    logger,
	}
}

// Run 阻塞运行生成循环，监听 ctx.Done() 退出。
// 在 cmd/server/main.go 里 go job.Run(ctx) 启动。
func (j *MomentJob) Run(ctx context.Context) {
	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	j.logger.Info("moment job started", zap.Duration("interval", j.interval))

	for {
		select {
		case <-ctx.Done():
			j.logger.Info("moment job stopped")
			return
		case <-ticker.C:
			j.tick(ctx)
		}
	}
}

// tick 扫一轮：逐个人设做节流 → 生成 → 落库 → 可选 AI 评论。
// 单个人设失败只记日志，不中断整轮（ai-moment spec §6.5）。
func (j *MomentJob) tick(ctx context.Context) {
	personas, err := j.repo.ListAllPersonasForJob(ctx)
	if err != nil {
		j.logger.Error("moment job: list personas failed", zap.Error(err))
		return
	}
	j.logger.Debug("moment job: candidates", zap.Int("count", len(personas)))

	for i := range personas {
		if err := j.generateForPersona(ctx, &personas[i]); err != nil {
			j.logger.Error("moment job: persona failed",
				zap.Uint64("userID", personas[i].UserID),
				zap.Uint64("personaID", personas[i].ID),
				zap.Error(err))
		}
	}
}

// generateForPersona 为单个生成一条动态；返回 nil 表示成功或按节流跳过。
func (j *MomentJob) generateForPersona(ctx context.Context, p *model.Persona) error {
	last, err := j.repo.GetRecentMomentTime(ctx, p.ID)
	if err != nil {
		return err
	}
	// 节流阈值复用 job 周期：本轮刚发过的人设跳过（ai-moment spec §6.4）
	if last != nil && time.Since(*last) < j.interval {
		j.logger.Debug("moment job: throttled", zap.Uint64("personaID", p.ID))
		return nil
	}

	events, err := j.repo.ListRecentEventsByPersona(
		ctx, p.UserID, p.ID, momentRecentEventLimit,
	)
	if err != nil {
		// 读不到记忆不阻断生成：没有事件槽位时模板会退到通用句式
		j.logger.Warn("moment job: load recent events failed",
			zap.Uint64("personaID", p.ID), zap.Error(err))
	}

	generated, err := j.generator.Generate(ctx, GenerateInput{
		Persona:      *p,
		RecentEvents: events,
	})
	if err != nil {
		return err
	}

	moment := &model.AIMoment{
		PersonaID:    p.ID,
		Content:      generated.Content,
		EmotionLabel: generated.EmotionLabel,
	}
	if err := j.repo.CreateMoment(ctx, moment); err != nil {
		return err
	}
	j.logger.Debug("moment job: created",
		zap.Uint64("personaID", p.ID), zap.Uint64("momentID", moment.ID))

	j.tryCreateAIComment(ctx, p, moment.ID)
	return nil
}

// tryCreateAIComment 按概率让同账号的其他人设评论这条动态。
// 每人设每 tick 最多 1 条；评论失败不影响已落库的动态。
func (j *MomentJob) tryCreateAIComment(
	ctx context.Context, poster *model.Persona, momentID uint64,
) {
	if rand.Float64() >= aiCommentProbability {
		return
	}

	others, err := j.repo.ListOtherPersonasInSameAccount(ctx, poster.UserID, poster.ID)
	if err != nil {
		j.logger.Warn("moment job: list commenters failed",
			zap.Uint64("personaID", poster.ID), zap.Error(err))
		return
	}
	if len(others) == 0 {
		return
	}

	commenter := others[rand.Intn(len(others))]
	comment := &model.MomentComment{
		MomentID:  momentID,
		PersonaID: &commenter.ID, // 作者是 AI 人设；user_id 必须保持 nil（CHECK 约束）
		Content:   pickCommentContent(),
	}
	if err := j.repo.CreateComment(ctx, j.db, comment); err != nil {
		j.logger.Warn("moment job: create ai comment failed",
			zap.Uint64("momentID", momentID),
			zap.Uint64("personaID", commenter.ID),
			zap.Error(err))
	}
}

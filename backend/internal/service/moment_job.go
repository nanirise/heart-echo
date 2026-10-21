package service

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/internal/repository"
)

const (
	// momentRecentEventLimit 是喂给生成器的事件类记忆条数上限。
	momentRecentEventLimit = 5
	// momentRecentMessageLimit 是喂给 AI 生成器的聊天记录条数上限。
	momentRecentMessageLimit = 5
	// aiCommentProbability 是每条新动态被同账号其他人设评论的概率。
	aiCommentProbability = 0.5
	// momentDailyLimit 是单个人设每日动态上限，防止朋友圈刷屏的机器人感。
	momentDailyLimit = 5
	// momentMinInterval 是同一人设两条动态之间的最小间隔。
	momentMinInterval = 3 * time.Hour
)

// momentJobRepo 是定时任务用到的那部分仓储能力。
// 收窄成接口后，频率限制可以在不连库的情况下单测；
// 真实 SQL（尤其 +08:00 日界与双条件过滤）由 repository 层的连库测试覆盖。
type momentJobRepo interface {
	ListAllPersonasForJob(ctx context.Context) ([]model.Persona, error)
	CountTodayMoments(ctx context.Context, userID, personaID uint64) (int64, error)
	GetRecentMomentTime(ctx context.Context, personaID uint64) (*time.Time, error)
	ListRecentEventsByPersona(
		ctx context.Context, userID, personaID uint64, limit int,
	) ([]string, error)
	ListRecentMessagesByPersona(
		ctx context.Context, userID, personaID uint64, limit int,
	) ([]model.ChatMessage, error)
	CreateMoment(ctx context.Context, m *model.AIMoment) error
	ListOtherPersonasInSameAccount(
		ctx context.Context, userID, excludePersonaID uint64,
	) ([]model.Persona, error)
	CreateComment(ctx context.Context, tx *gorm.DB, c *model.MomentComment) error
}

// MomentJob 按 MOMENT_JOB_INTERVAL 周期为全部人设生成朋友圈动态。
// 定时任务一律跑在 Go 侧（AGENTS §4.8）。
// ⚠️ 本 job 不提供手动触发接口：动态只由定时任务产生（AGENTS §4.6 / §7.5）。
type MomentJob struct {
	db        *gorm.DB
	repo      momentJobRepo
	generator ContentGenerator
	interval  time.Duration
	logger    *zap.Logger
}

// NewMomentJob 构造定时任务；generator 由外部注入。
// 生产传 AIGenerator（内部自带模板兜底），测试可传 TemplateGenerator。
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

// generateForPersona 为单个生成一条动态；返回 nil 表示成功或按频率限制跳过。
func (j *MomentJob) generateForPersona(ctx context.Context, p *model.Persona) error {
	// 频率限制放在最前面：先确定"今天还能不能发"，避免为了跳过而白白组装上下文、调一次 AI。
	todayCount, err := j.repo.CountTodayMoments(ctx, p.UserID, p.ID)
	if err != nil {
		return err
	}
	if todayCount >= momentDailyLimit {
		j.logger.Debug("moment job: skipped",
			zap.Uint64("personaID", p.ID),
			zap.Int64("todayCount", todayCount),
			zap.String("reason", "daily_limit_reached"))
		return nil
	}

	last, err := j.repo.GetRecentMomentTime(ctx, p.ID)
	if err != nil {
		return err
	}
	if last != nil && time.Since(*last) < momentMinInterval {
		j.logger.Debug("moment job: skipped",
			zap.Uint64("personaID", p.ID),
			zap.String("reason", "min_interval_not_reached"))
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

	messages, err := j.repo.ListRecentMessagesByPersona(
		ctx, p.UserID, p.ID, momentRecentMessageLimit,
	)
	if err != nil {
		// 读不到聊天记录同样不阻断：上下文为空时 AI 仍能按人设发一条通用动态
		j.logger.Warn("moment job: load recent messages failed",
			zap.Uint64("personaID", p.ID), zap.Error(err))
	}

	generated, err := j.generator.Generate(ctx, GenerateInput{
		Persona:      *p,
		RecentEvents: events,
		Context:      buildMomentContext(p.Name, messages, events),
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

// buildMomentContext 把最近聊天与事件记忆拼成一段纯文本，交给 AI 生成器。
//
// 仓储返回倒序（最新在前），这里反转成时间正序 —— 模型按发生顺序读，
// 顺序错了它不会报错，只会把因果说反。
func buildMomentContext(
	personaName string, messages []model.ChatMessage, events []string,
) string {
	var chatLines []string
	for i := len(messages) - 1; i >= 0; i-- {
		content := strings.TrimSpace(messages[i].Content)
		if content == "" {
			continue
		}
		speaker := "用户"
		if messages[i].Role == model.RoleAssistant {
			speaker = personaName
			if speaker == "" {
				speaker = "你"
			}
		}
		chatLines = append(chatLines, speaker+"："+content)
	}

	memoryLines := make([]string, 0, len(events))
	for _, event := range events {
		if event = strings.TrimSpace(event); event != "" {
			memoryLines = append(memoryLines, "- "+event)
		}
	}

	var b strings.Builder
	if len(chatLines) > 0 {
		b.WriteString("最近聊天记录：\n")
		for _, line := range chatLines {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	if len(memoryLines) > 0 {
		b.WriteString("你记得关于用户的事：\n")
		for _, line := range memoryLines {
			b.WriteString(line)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}

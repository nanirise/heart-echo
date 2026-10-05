package service

import (
	"context"
	"errors"
	"math/rand"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/repository"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// 契约 §9 的取值边界（proactive-setting §2.2 指定在 service 层校验，不写进 DTO 的 tag）。
const (
	settingsIntervalMin = 5
	settingsIntervalMax = 1440
	settingsDailyMin    = 1
	settingsDailyMax    = 10
)

// nudgeContent 是主动消息注入的固定正文（spec §1）：内容只有这个标记，
// "说什么"由聊天链路按人格与历史生成，本模块不写模板消息。
const nudgeContent = "[nudge]"

// ProactiveService 是主动消息的业务逻辑：settings 两个端点 + 触发链路 TriggerNow。
// 持有 *gorm.DB 而不是只把 db 交给仓储：PUT 与 last_nudge_at 的写入都要包事务
// （proactive-setting §2.2 / 契约 §9 只读列的唯一写入方）。
type ProactiveService struct {
	db   *gorm.DB
	repo *repository.ProactiveRepo
	// chat 是注入 [nudge] 的出口。主动消息不自己写 chat_messages，也不自己调 AI：
	// [nudge] 的写入语义单点真相在 ChatService（spec §7 第 1 条已选 A）。
	chat *ChatService
}

// NewProactiveService 构造服务，并组装它依赖的仓储。
// chat 由调用方传入：它要读 config（AI_SERVICE_URL / TOKEN），service 层不碰配置，
// 且两条链路（定时任务 / 手动端点）必须共用注入路径。
func NewProactiveService(db *gorm.DB, chat *ChatService) *ProactiveService {
	return &ProactiveService{db: db, repo: repository.NewProactiveRepo(db), chat: chat}
}

// GetSettings 读当前用户的某份配置（契约 §9 GET）。
func (s *ProactiveService) GetSettings(
	ctx context.Context, userID, personaID uint64,
) (*dto.SettingsResponse, error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}
	// 不能当 personaID = 0 查库：命中空集会返回 200 + 空对象，把调用方 bug 伪装成正常态
	if personaID == 0 {
		return nil, errcode.New(errcode.ErrInvalidParams)
	}

	setting, err := s.repo.Get(ctx, userID, personaID)
	if err != nil {
		// 越权（还是下面 UpdateSettings 的"别人的"）与"不存在"共用这个 not found，不可区分
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrPersonaNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	resp := dto.NewSettingsResponse(setting)
	return &resp, nil
}

// UpdateSettings 改四个可改列并回读真值（契约 §9 PUT）。
func (s *ProactiveService) UpdateSettings(
	ctx context.Context, userID uint64, req *dto.UpdateSettingsRequest,
) (*dto.SettingsResponse, error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}
	if err := validateSettings(req); err != nil {
		return nil, err
	}

	var resp dto.SettingsResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		n, err := s.repo.UpdateOwned(
			ctx, tx, userID, req.PersonaID,
			*req.Enabled, *req.IntervalMin, *req.IntervalMax, *req.DailyLimit,
		)
		if err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		if n == 0 {
			// 0 = WHERE 没匹配到：不是自己的 / 不存在的 / 播种缺失，同码同文案
			return errcode.New(errcode.ErrPersonaNotFound)
		}
		// 回读必须走事务句柄：s.repo 的读路径用的是一条独立连接，看不到本事务未提交的 UPDATE
		current, err := repository.NewProactiveRepo(tx).Get(ctx, userID, req.PersonaID)
		if err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		resp = dto.NewSettingsResponse(current)
		return nil
	})
	if err != nil {
		// 事务内的 BizError（4043 等）原样透传；Begin/Commit 这类还没被归类的
		// 基础设施错误补一次 5003，别让它到出口被归成 5000「服务端内部错误」
		var bizErr *errcode.BizError
		if errors.As(err, &bizErr) {
			return nil, err
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	return &resp, nil
}

// validateSettings 收敛 PUT 的入参校验（proactive-setting §2.2：越界一律 4001）。
// 缺失字段也在这里兜底：binding 只拦 HTTP 链路，换调用方就没人拦了。
func validateSettings(req *dto.UpdateSettingsRequest) error {
	if req == nil || req.PersonaID == 0 || req.Enabled == nil ||
		req.IntervalMin == nil || req.IntervalMax == nil || req.DailyLimit == nil {
		return errcode.New(errcode.ErrInvalidParams)
	}
	if *req.IntervalMin < settingsIntervalMin || *req.IntervalMin > settingsIntervalMax ||
		*req.IntervalMax < settingsIntervalMin || *req.IntervalMax > settingsIntervalMax {
		return errcode.New(errcode.ErrInvalidParams)
	}
	if *req.IntervalMin >= *req.IntervalMax {
		return errcode.New(errcode.ErrInvalidParams)
	}
	if *req.DailyLimit < settingsDailyMin || *req.DailyLimit > settingsDailyMax {
		return errcode.New(errcode.ErrInvalidParams)
	}
	return nil
}

// TriggerNow 是触发链路的唯一入口：手动端点与定时任务都调它（spec §4）。
//
// 三种出口（spec §3）：
//   - 全通过 → (*NudgeResult, nil)
//   - 判定未通过 → (nil, nil)，这是正常态：每 5 分钟扫一次，绝大多数人设都不触发，
//     不产生错误码、不记 ERROR 日志，只留一条 debug（spec §3.a）；
//   - DB / 生成调用失败 → (nil, err)，照常上抛。把基础设施错误吞成"未触发"，
//     会让演示时"点了没反应"变成常态。
func (s *ProactiveService) TriggerNow(ctx context.Context, userID, personaID uint64) (*NudgeResult, error) {
	// ① 配置存在且开启。这一查同时是归属闸门：不是自己的 / 不存在的 / 播种缺失
	// 都落进同一个 not found，一律 4043，不可区分（spec §2.3）。
	setting, err := s.repo.Get(ctx, userID, personaID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrPersonaNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if !setting.Enabled {
		logTriggerSkip(personaID, "disabled")
		return nil, nil
	}

	// ② 空闲达阈值。阈值每次调用在 [intervalMin, intervalMax] 分钟内取一次随机值
	// （spec §3.b 允许按次取）：静默越久命中概率越高，频率由 ③ 的日上限兜底。
	// 比较放在仓储里用数据库时钟做，见 IsIdleOverThreshold 的说明。
	idle, err := s.repo.IsIdleOverThreshold(
		ctx, userID, personaID, randomIdleThresholdSeconds(setting.IntervalMin, setting.IntervalMax),
	)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if !idle {
		logTriggerSkip(personaID, "idle_threshold")
		return nil, nil
	}

	// ③ 当日已注入条数 < dailyLimit。口径是 chat_messages 里 is_nudge=true 的条数，
	// 与 P1 日程提醒共用同一个上限（spec §3.d）——两套链路各数各的会变成两个防骚扰口径。
	sent, err := s.repo.CountTodayNudges(ctx, userID, personaID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if sent >= int64(setting.DailyLimit) {
		logTriggerSkip(personaID, "daily_limit")
		return nil, nil
	}

	// ④ 最近 1 小时没有用户消息。排除注入行是这条的全部要害（spec §3.c）。
	recent, err := s.repo.HasRecentUserMessage(ctx, userID, personaID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if recent {
		logTriggerSkip(personaID, "recent_user_message")
		return nil, nil
	}

	// ⑤ 全过：注入 [nudge] → 走聊天链路生成 → 落库，同步拿回生成的回复。
	// 生成失败原样上抛（5001 / 5002），绝不降级成"未触发"——否则演示时表现为
	// "点了没反应"，而日志里一条错都没有（spec §3.a）。
	res, err := s.chat.InjectNudge(ctx, NudgeInput{
		UserID:    userID,
		PersonaID: personaID,
		Content:   nudgeContent,
	})
	if err != nil {
		return nil, err
	}

	// last_nudge_at 只是记录列（契约 §9 只读），不参与任何判定；但它写不进去同样是 DB 故障，
	// 照常上抛——补不上记录却返回成功，会让"已触发过"这件事在库里查不到。
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.repo.UpdateLastNudgeAt(ctx, tx, userID, personaID)
	})
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	zap.L().Debug("proactive: triggered",
		zap.Uint64("personaID", personaID), zap.Uint64("messageID", res.MessageID))
	return res, nil
}

// randomIdleThresholdSeconds 在 [intervalMin, intervalMax] 分钟内取一个随机阈值（spec §3.b）。
//
// 每次调用取一次（不是每个空闲段取一次）：同一个空闲段在不同轮扫描里阈值不同，
// 手动连续点触发也各有各的机会，演示不必等一个固定的"倒霉阈值"。
// hi <= lo 时直接返回下界：数据被手工改坏也不能让 rand.Intn 收到非正数而 panic。
func randomIdleThresholdSeconds(intervalMin, intervalMax int) int {
	lo, hi := intervalMin*60, intervalMax*60
	if hi <= lo {
		return lo
	}
	return lo + rand.Intn(hi-lo+1)
}

// logTriggerSkip 记一条判定未通过的 debug 日志（spec §3 末尾：演示前排障全靠它，
// 否则只能看到"没触发"三个字，分不清是关着、没到点还是到限额了）。
//
// 走 zap 的全局 logger：main.go 启动时 ReplaceGlobals(appLogger)，未替换时它是 no-op、
// 日志静默丢弃——这条链路没有别的 logger 注入点（构造签名不带 logger）。
func logTriggerSkip(personaID uint64, reason string) {
	zap.L().Debug("proactive: not triggered",
		zap.Uint64("personaID", personaID), zap.String("reason", reason))
}

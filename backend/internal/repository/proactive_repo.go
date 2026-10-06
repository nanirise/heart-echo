package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// ProactiveRepo 主动消息配置表的数据访问。
type ProactiveRepo struct {
	db *gorm.DB
}

// NewProactiveRepo 构造仓储；db 供后续的读路径使用，写路径由调用方传入事务句柄。
func NewProactiveRepo(db *gorm.DB) *ProactiveRepo {
	return &ProactiveRepo{db: db}
}

// CreateDefaultSettings 播种一行默认配置，必须传入调用方的事务句柄。
// 四个默认值显式写出是为了可读，不是必须：这四列带 default: tag，GORM 会把零值替换成
// tag 值后入库，留零值入库的也是这组值（model/proactive_setting.go 有实测说明）。
// 用 INSERT 不用 upsert —— 重复插入就该报错。
func (r *ProactiveRepo) CreateDefaultSettings(ctx context.Context, tx *gorm.DB, userID, personaID uint64) error {
	s := &model.ProactiveSetting{
		UserID:      userID,
		PersonaID:   personaID,
		Enabled:     true,
		IntervalMin: 30,
		IntervalMax: 120,
		DailyLimit:  3,
		// LastNudgeAt 保持 nil：从未触发过
	}
	return tx.WithContext(ctx).Create(s).Error
}

// Get 读当前用户的某一行配置，未命中返回 gorm.ErrRecordNotFound。
// 两个条件都带：对"别人的 persona"与"不存在的 persona"返回同一个 not found，二者不可区分。
func (r *ProactiveRepo) Get(ctx context.Context, userID, personaID uint64) (*model.ProactiveSetting, error) {
	var s model.ProactiveSetting
	err := r.db.WithContext(ctx).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// UpdateOwned 改四个可改列并返回受影响行数。
// ⛔ 必须走 map：这四列带 default: tag，用 struct 传时零值字段会被整条跳过——关掉主动消息
// （enabled:false / daily_limit:0）会变成 Error=nil、RowsAffected=0 的静默空操作；map 的
// 键值原样进 SET，既不跳零值也不做 tag 值替换。last_nudge_at 不进 map（契约 §9 只读）。
func (r *ProactiveRepo) UpdateOwned(
	ctx context.Context, tx *gorm.DB, userID, personaID uint64,
	enabled bool, intervalMin, intervalMax, dailyLimit int,
) (int64, error) {
	res := tx.WithContext(ctx).Model(&model.ProactiveSetting{}).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		Updates(map[string]any{
			"enabled":      enabled,
			"interval_min": intervalMin,
			"interval_max": intervalMax,
			"daily_limit":  dailyLimit,
		})
	if res.Error != nil {
		return 0, res.Error
	}
	// 0 = WHERE 没匹配到（播种缺失或不是自己的行），不是"值没变"：同值 UPDATE 在 Postgres 里仍算匹配，返回 1
	return res.RowsAffected, nil
}

// cstZone 是「当日零点」的固定时区：+08:00，硬编码常量（不读 config、不随进程本地时区）。
// 容器与 CI 默认 UTC——跟随进程时区的"今日"会提前 8 小时跨日，与产品口径不符。
var cstZone = time.FixedZone("UTC+8", 8*60*60)

// todayMidnight 返回 +08:00 口径的今日零点。
func todayMidnight() time.Time {
	now := time.Now().In(cstZone)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, cstZone)
}

// CountTodayNudges 数该人设今天已注入的 [nudge] 条数，供日上限判定（spec §3.c）。
// 查 chat_messages——is_nudge 列在该表，不在 proactive_settings；只数注入行、不数 assistant 回复。
// ⚠️ 与 P1 日程提醒共用：两链路共用同一个日上限，计数口径不能各算各的。
// 日界时区为硬编码 +08:00 常量（不读 config）；当前时刻取自进程时钟并显式换算到该时区，不依赖进程 TZ。
// 本方法内的 time.Now() 是本仓库唯一允许的例外——『当日零点』是时区相关的业务逻辑，不宜外推给调用方；其他 repo 方法一律不得调 time.Now()。
func (r *ProactiveRepo) CountTodayNudges(ctx context.Context, userID, personaID uint64) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.ChatMessage{}).
		Where("persona_id = ? AND user_id = ? AND is_nudge = ? AND created_at >= ?",
			personaID, userID, true, todayMidnight()).
		Count(&n).Error
	return n, err
}

// IsIdleOverThreshold 回答触发判定 ②：该人设已静默达/超过 thresholdSeconds 秒了吗。
//
// 减法必须放在 SQL 里用数据库的 NOW()：last_message_at 由数据库时钟写入
// （persona_repo.TouchLastMessageAt），用 Go 的 time.Now() 做减法会在应用与数据库有时差时
// 让阈值整体偏移，且偏移量随部署环境变——查不出原因。
// last_message_at IS NULL（从未聊过）不满足条件 → false，与 spec §3.b「没有空闲起点，不触发」一致。
// 双条件（persona_id + user_id）不可省：手动触发链路上 user_id 来自 Token（AGENTS §4.3）。
func (r *ProactiveRepo) IsIdleOverThreshold(
	ctx context.Context, userID, personaID uint64, thresholdSeconds int,
) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&model.Persona{}).
		Where("id = ? AND user_id = ? AND last_message_at IS NOT NULL"+
			" AND EXTRACT(EPOCH FROM (NOW() - last_message_at)) >= ?",
			personaID, userID, thresholdSeconds).
		Count(&n).Error
	return n > 0, err
}

// UpdateLastNudgeAt 记录最近一次主动触发的时间；唯一写入方是触发链路——前端与 PUT 都不写它（契约 §9 只读）。
// 不并入 UpdateOwned：后者 map 的键集合已冻结为四个可改列，不含 last_nudge_at。
// 未匹配行不报错：调用方（TriggerNow）刚经 Get 校验过归属，未匹配只可能是并发删除的竞态；
// 记录列补不上不影响已注入的结果，调用方也没有可做的补救，故只回传基础设施错误。
func (r *ProactiveRepo) UpdateLastNudgeAt(ctx context.Context, tx *gorm.DB, userID, personaID uint64) error {
	return tx.WithContext(ctx).Model(&model.ProactiveSetting{}).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		Update("last_nudge_at", gorm.Expr("NOW()")).Error
}

// ScanRow 是定时任务扫描出的一行：一份配置 + 对应人设的最后消息时间（nil = 从未聊过）。
type ScanRow struct {
	UserID        uint64
	PersonaID     uint64
	IntervalMin   int
	IntervalMax   int
	DailyLimit    int
	LastMessageAt *time.Time
}

// ListEnabledForScan 定时任务专用，跨用户扫描 enabled=true；与其他方法的双条件查询纪律不冲突，
// 返回行不落库、不越权。调用方（proactive_job）负责逐行传 TriggerNow 做归属判定。
// 本层不做任何时间过滤——空闲阈值等判定一律归 TriggerNow（spec §3），last_message_at 为 NULL 的行照常返回。
func (r *ProactiveRepo) ListEnabledForScan(ctx context.Context) ([]ScanRow, error) {
	var rows []ScanRow
	err := r.db.WithContext(ctx).
		Model(&model.ProactiveSetting{}).
		Select("proactive_settings.user_id, proactive_settings.persona_id,"+
			" proactive_settings.interval_min, proactive_settings.interval_max,"+
			" proactive_settings.daily_limit, personas.last_message_at").
		Joins("JOIN personas ON personas.id = proactive_settings.persona_id").
		Where("proactive_settings.enabled = ?", true).
		Scan(&rows).Error
	return rows, err
}

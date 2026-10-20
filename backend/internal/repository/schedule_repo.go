package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// ScheduleRepo 日程表的数据访问。
// 用户侧读写一律带 user_id（越权防线，AGENTS §4.3）；跨用户读只有定时任务专用的
// ListPendingDue，它返回的行不回给任何用户。
type ScheduleRepo struct {
	db *gorm.DB
}

// NewScheduleRepo 构造仓储；db 用于读写路径，多表事务由调用方另行传入句柄。
func NewScheduleRepo(db *gorm.DB) *ScheduleRepo {
	return &ScheduleRepo{db: db}
}

// ListByPersona 按人设分页读日程，并返回该人设的真实总数。
//
// 两个条件都带：persona_id 隔离人设，user_id 防跨用户串号。
// 顺序按 model/schedule.go 钉死的 remind_at ASC, id DESC（同值兜底，避免翻页重复）。
// Count 与 Find 的 WHERE 必须逐字相同，否则 total 与 list 对不上。
func (r *ScheduleRepo) ListByPersona(
	ctx context.Context, userID, personaID uint64, offset, limit int,
) ([]model.Schedule, int64, error) {
	list := make([]model.Schedule, 0)
	var total int64

	err := r.db.WithContext(ctx).Model(&model.Schedule{}).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.WithContext(ctx).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		Order("remind_at ASC, id DESC").
		Offset(offset).Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// FindOwned 读当前用户的某条日程，未命中返回 gorm.ErrRecordNotFound。
// 对"别人的日程"与"不存在的日程"返回同一个 not found，二者不可区分。
func (r *ScheduleRepo) FindOwned(
	ctx context.Context, userID, scheduleID uint64,
) (*model.Schedule, error) {
	var s model.Schedule
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", scheduleID, userID).
		First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Cancel 软删除：把 status 置为 cancelled，返回受影响行数。
//
// 刻意不限制 status：契约 §10 要求重复删除同一条是幂等的。Postgres 对同值 UPDATE
// 仍计入 RowsAffected，所以已 cancelled 的行再次 DELETE 也是 1（200）；
// 只有"不属于当前用户 / 不存在"才返回 0。
func (r *ScheduleRepo) Cancel(
	ctx context.Context, userID, scheduleID uint64,
) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.Schedule{}).
		Where("id = ? AND user_id = ?", scheduleID, userID).
		Update("status", model.ScheduleStatusCancelled)
	return res.RowsAffected, res.Error
}

// MarkSent 把 pending 的日程置为 sent，返回受影响行数。
//
// 条件带 status='pending'：扫描后到注入前被取消的行不能再改回 sent。
// 返回 0 表示发生并发取消/重复触发——调用方已经发出提醒，不能回滚，只记日志。
func (r *ScheduleRepo) MarkSent(
	ctx context.Context, userID, scheduleID uint64,
) (int64, error) {
	res := r.db.WithContext(ctx).Model(&model.Schedule{}).
		Where("id = ? AND user_id = ? AND status = ?",
			scheduleID, userID, model.ScheduleStatusPending).
		Update("status", model.ScheduleStatusSent)
	return res.RowsAffected, res.Error
}

// ListPendingDue 定时任务专用：跨用户捞出已到期的待提醒日程。
//
// NOW() 用数据库时钟：remind_at 由应用写入，但"是否到期"必须与写入端同一时钟判断，
// 否则应用与数据库有时差时整体提前/延后。limit 防止积压后一轮拉全表；
// 剩余的下轮再处理。请求路径一律不得引用本方法。
func (r *ScheduleRepo) ListPendingDue(
	ctx context.Context, limit int,
) ([]model.Schedule, error) {
	list := make([]model.Schedule, 0)
	err := r.db.WithContext(ctx).
		Where("status = ? AND remind_at <= NOW()", model.ScheduleStatusPending).
		Order("remind_at ASC, id DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// Create 落一条日程。写路径由调用方传入事务句柄。
//
// Status 留空时补 pending：本列数据库侧没有 CHECK 约束，常量是唯一防线（model/schedule.go）。
// 不要给 User / Persona / SourceMessage 关联字段赋值，否则 GORM 会连带写那三张表。
func (r *ScheduleRepo) Create(
	ctx context.Context, tx *gorm.DB, s *model.Schedule,
) error {
	if s.Status == "" {
		s.Status = model.ScheduleStatusPending
	}
	return tx.WithContext(ctx).Create(s).Error
}

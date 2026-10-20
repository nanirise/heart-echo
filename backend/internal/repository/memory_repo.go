package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// MemoryRepo 记忆表与画像表的数据访问。
// 两张表都是一人设一份，读写方法签名强制带 userID 与 personaID：
// persona_id 隔离人设，user_id 是越权防线（AGENTS §4.3）。
type MemoryRepo struct {
	db *gorm.DB
}

// NewMemoryRepo 构造仓储。
func NewMemoryRepo(db *gorm.DB) *MemoryRepo {
	return &MemoryRepo{db: db}
}

// ListByPersona 按人设分页读记忆，并返回该人设的真实总数。
//
// Count 与 Find 的 WHERE 必须逐字相同，否则 total 会把别人的记忆算进来。
// id DESC 兜底不能省：一轮对话提取的记忆在同一事务写入，created_at 完全相同，
// 只按它排序时翻页会重复或漏项。
func (r *MemoryRepo) ListByPersona(
	ctx context.Context, userID, personaID uint64, offset, limit int,
) ([]model.UserMemory, int64, error) {
	list := make([]model.UserMemory, 0)
	var total int64

	err := r.db.WithContext(ctx).Model(&model.UserMemory{}).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = r.db.WithContext(ctx).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		Order("created_at DESC, id DESC").
		Offset(offset).Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// FindPortrait 读当前用户某个人设的画像。
// 查不到行返回 (nil, nil) 而不是 gorm.ErrRecordNotFound：没有画像是合法空态，
// 不是错误；把它翻译成错误会让 service 把空态误判成 5003。
func (r *MemoryRepo) FindPortrait(
	ctx context.Context, userID, personaID uint64,
) (*model.UserProfile, error) {
	var p model.UserProfile
	err := r.db.WithContext(ctx).
		Where("persona_id = ? AND user_id = ?", personaID, userID).
		First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

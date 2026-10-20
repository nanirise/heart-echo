package repository

import (
	"bytes"
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// ErrPortraitOwnershipMismatch 表示画像 UPSERT 的越权兜底 WHERE 没命中。
// 调用方只记日志：它意味着归属校验被绕过，绝不能当成"写成功了"（user-profile spec §4.6）。
var ErrPortraitOwnershipMismatch = errors.New("portrait upsert skipped: user_id mismatch")

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

// CreateBatch 在一个事务里写入一轮提取的全部记忆；空批次直接返回 nil。
//
// user_id / persona_id 由调用方赋值，本方法不采信 AI 服务的返回体
// （user-memory spec §3.3 约定 1）。embedding_id / embedding_status 留零值，
// GORM 会按 tag 补上 'pending'，阶段二接 ChromaDB 时再用。
func (r *MemoryRepo) CreateBatch(
	ctx context.Context, tx *gorm.DB, memories []*model.UserMemory,
) error {
	if len(memories) == 0 {
		return nil
	}
	return tx.WithContext(ctx).Create(memories).Error
}

// UpsertPortrait 按 persona_id 插入或合并画像键。
//
// 合并必须在 SQL 侧用 jsonb || 完成，不能在 Go 里读-改-写：后者既有丢键风险，
// 也有丢更新竞态（两次提取并发时后写的会覆盖前写的键）（user-profile spec §4.6）。
// DO UPDATE 里的 WHERE user_id = ? 是越权兜底，不是可选优化：归属校验一旦被
// 绕过，它保证失败方向是"没写进去"，而不是"写错了地方"。
func (r *MemoryRepo) UpsertPortrait(
	ctx context.Context, tx *gorm.DB, userID, personaID uint64, updates model.JSONB,
) error {
	// 空对象会刷新 updated_at，让"画像最近更新于"变成假信息（spec §4.6 约定 2）。
	trimmed := bytes.TrimSpace(updates)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("{}")) ||
		bytes.Equal(trimmed, []byte("null")) {
		return nil
	}

	res := tx.WithContext(ctx).Exec(`
		INSERT INTO user_profile (user_id, persona_id, profile_data, updated_at)
		VALUES (?, ?, ?::jsonb, NOW())
		ON CONFLICT (persona_id) DO UPDATE
		   SET profile_data = user_profile.profile_data || EXCLUDED.profile_data,
		       updated_at   = NOW()
		 WHERE user_profile.user_id = ?`,
		userID, personaID, string(trimmed), userID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrPortraitOwnershipMismatch
	}
	return nil
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

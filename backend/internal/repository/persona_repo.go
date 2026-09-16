package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// PersonaRepo 人设表的数据访问。
// 每个方法都带 userID，归属条件一律做进 SQL 的 WHERE —— 不是先查出来再在 Go 里比。
// 禁止出现不带 user_id 的单条件查询：那能回答"这个 id 存不存在"，可被顺序试号枚举。
type PersonaRepo struct {
	db *gorm.DB
}

// NewPersonaRepo 构造仓储；db 用于读路径，写路径由调用方传入事务句柄。
func NewPersonaRepo(db *gorm.DB) *PersonaRepo {
	return &PersonaRepo{db: db}
}

// ListByUser 按当前用户分页取人设，并返回该用户的真实总数。
func (r *PersonaRepo) ListByUser(ctx context.Context, userID uint64, offset, limit int) ([]model.Persona, int64, error) {
	var (
		list  = make([]model.Persona, 0)
		total int64
	)

	// Count 与 Find 的 WHERE 必须逐字相同，否则 total 与 list 对不上
	err := r.db.WithContext(ctx).Model(&model.Persona{}).
		Where("user_id = ?", userID).
		Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// NULLS LAST 不能省：Postgres 的 DESC 默认 NULLS FIRST，没聊过的人设会排在最前，与需求相反。
	// id DESC 兜底：大量行的 last_message_at 为 NULL，只按它排会让翻页重复或漏项。
	err = r.db.WithContext(ctx).Model(&model.Persona{}).
		Where("user_id = ?", userID).
		Order("last_message_at DESC NULLS LAST, id DESC").
		Offset(offset).Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// FindOwned 读当前用户的某一行，未命中返回 gorm.ErrRecordNotFound。
// 对"别人的 id"与"不存在的 id"返回同一个 not found，两者不可区分。
func (r *PersonaRepo) FindOwned(ctx context.Context, userID, personaID uint64) (*model.Persona, error) {
	var p model.Persona
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", personaID, userID).
		First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Create 插入人设；p.ID 由 BIGSERIAL 回填。Create 时不要给 User 关联字段赋值。
func (r *PersonaRepo) Create(ctx context.Context, tx *gorm.DB, p *model.Persona) error {
	return tx.WithContext(ctx).Create(p).Error
}

// UpdateOwned 更新三列并返回受影响行数。
// Updates 传 map 而不是 struct：struct 会忽略零值字段，空串会被静默跳过。
// 绝不能用 Save：它写回全部列，会把 state 覆盖成零值、last_message_at 清空，数据损坏且不可逆。
func (r *PersonaRepo) UpdateOwned(
	ctx context.Context, tx *gorm.DB, userID, personaID uint64,
	name, personalityDesc, speakingStyle string,
) (int64, error) {
	res := tx.WithContext(ctx).Model(&model.Persona{}).
		Where("id = ? AND user_id = ?", personaID, userID).
		Updates(map[string]any{
			"name":             name,
			"personality_desc": personalityDesc,
			"speaking_style":   speakingStyle,
		})
	if res.Error != nil {
		return 0, res.Error
	}
	// 0 = WHERE 没匹配到，不是"值没变"：同值 UPDATE 在 Postgres 里仍算匹配，返回 1
	return res.RowsAffected, nil
}

// DeleteOwned 硬删除并返回受影响行数；级联清理由数据库外键完成，这里不写多表删除。
func (r *PersonaRepo) DeleteOwned(ctx context.Context, tx *gorm.DB, userID, personaID uint64) (int64, error) {
	res := tx.WithContext(ctx).
		Where("id = ? AND user_id = ?", personaID, userID).
		Delete(&model.Persona{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// ExistsOwnedByUser 只回答"这个 persona 是不是这个用户的"，供跨模块的归属闸门使用。
// 对"别人的 persona"与"不存在的 persona"都返回 (false, nil)，不泄漏存在性。
func (r *PersonaRepo) ExistsOwnedByUser(ctx context.Context, userID, personaID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.Persona{}).
		Where("id = ? AND user_id = ?", personaID, userID).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

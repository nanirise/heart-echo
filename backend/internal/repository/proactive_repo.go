package repository

import (
	"context"

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

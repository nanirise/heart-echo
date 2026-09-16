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

package service

import (
	"context"
	"errors"

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

// ProactiveService 是主动消息 settings 两个端点的业务逻辑。
// 持有 *gorm.DB 而不是只把 db 交给仓储：PUT 要包事务（UPDATE + 回读同成同败，proactive-setting §2.2）。
type ProactiveService struct {
	db   *gorm.DB
	repo *repository.ProactiveRepo
}

// NewProactiveService 构造服务，并组装它依赖的仓储。
func NewProactiveService(db *gorm.DB) *ProactiveService {
	return &ProactiveService{db: db, repo: repository.NewProactiveRepo(db)}
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

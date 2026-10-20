package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/repository"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// MemoryService 记忆与画像的业务逻辑。
// 两者都是一人设一份：先单独判人设归属，再查数据——不能用"查不到行"反推归属，
// 否则"还没有记忆/画像"会被误报成 4043，新人设的页面直接打不开。
type MemoryService struct {
	memoryRepo  *repository.MemoryRepo
	personaRepo *repository.PersonaRepo
}

// NewMemoryService 构造服务，并组装它依赖的仓储。
func NewMemoryService(db *gorm.DB) *MemoryService {
	return &MemoryService{
		memoryRepo:  repository.NewMemoryRepo(db),
		personaRepo: repository.NewPersonaRepo(db),
	}
}

// ListMemories 返回某个人设的记忆分页。空列表是合法状态（200 + list: []）。
func (s *MemoryService) ListMemories(
	ctx context.Context, userID, personaID uint64, page, pageSize int,
) (*dto.PageResult[dto.MemoryItemResponse], error) {
	if err := s.requireOwnedPersona(ctx, userID, personaID); err != nil {
		return nil, err
	}

	page, pageSize = dto.ClampPage(page, pageSize)
	list, total, err := s.memoryRepo.ListByPersona(
		ctx, userID, personaID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	// 必须初始化：nil slice 会序列化成 null，前端 v-for 直接崩
	items := make([]dto.MemoryItemResponse, 0, len(list))
	for i := range list {
		items = append(items, dto.NewMemoryItemResponse(&list[i]))
	}
	return &dto.PageResult[dto.MemoryItemResponse]{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// GetPortrait 返回某个人设的画像。没有画像行是合法空态，返回 {} + updatedAt: null。
func (s *MemoryService) GetPortrait(
	ctx context.Context, userID, personaID uint64,
) (*dto.PortraitResponse, error) {
	if err := s.requireOwnedPersona(ctx, userID, personaID); err != nil {
		return nil, err
	}

	profile, err := s.memoryRepo.FindPortrait(ctx, userID, personaID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	resp := dto.NewPortraitResponse(profile, personaID)
	return &resp, nil
}

// requireOwnedPersona 是两个端点共用的归属闸门。
// 不存在与不属于当前用户返回同一个 4043：persona_id 全局自增，
// 区分两者就能被顺序试号探测出系统里有多少人设（AGENTS §4.3）。
func (s *MemoryService) requireOwnedPersona(
	ctx context.Context, userID, personaID uint64,
) error {
	if userID == 0 {
		return errcode.New(errcode.ErrUnauthorized)
	}
	if personaID == 0 {
		// 缺参在 handler 绑定层已拦；这里兜底，绝不退化成 0 查库。
		return errcode.New(errcode.ErrInvalidParams)
	}

	owned, err := s.personaRepo.ExistsOwnedByUser(ctx, userID, personaID)
	if err != nil {
		return errcode.Wrap(errcode.ErrDBFailed, err)
	}
	if !owned {
		return errcode.New(errcode.ErrPersonaNotFound)
	}
	return nil
}

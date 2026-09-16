package service

import (
	"context"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/internal/repository"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// PersonaService 人设的业务逻辑：归属判定 + 事务编排 + DTO 转换。
// 只依赖 context.Context，不感知 *gin.Context。
type PersonaService struct {
	db            *gorm.DB
	personaRepo   *repository.PersonaRepo
	proactiveRepo *repository.ProactiveRepo
}

// NewPersonaService 构造服务，并组装它依赖的仓储。
func NewPersonaService(db *gorm.DB) *PersonaService {
	return &PersonaService{
		db:            db,
		personaRepo:   repository.NewPersonaRepo(db),
		proactiveRepo: repository.NewProactiveRepo(db),
	}
}

// List 返回当前用户的人设分页列表。没有人设是正常状态，返回空列表而不是错误。
func (s *PersonaService) List(
	ctx context.Context, userID uint64, page, pageSize int,
) (*dto.PageResult[dto.PersonaResponse], error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}

	page, pageSize = dto.ClampPage(page, pageSize)
	list, total, err := s.personaRepo.ListByUser(ctx, userID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	// 必须初始化：nil slice 会序列化成 null，前端 v-for 直接崩
	items := make([]dto.PersonaResponse, 0, len(list))
	for i := range list {
		items = append(items, dto.NewPersonaResponse(&list[i]))
	}

	return &dto.PageResult[dto.PersonaResponse]{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// Create 新建人设，并在同一事务里播种主动消息配置。
func (s *PersonaService) Create(
	ctx context.Context, userID uint64, req *dto.CreatePersonaRequest,
) (*dto.PersonaResponse, error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}

	p := &model.Persona{
		UserID:          userID, // 只来自 Token，不从请求体取
		Name:            req.Name,
		PersonalityDesc: req.PersonalityDesc,
		SpeakingStyle:   req.SpeakingStyle,
		State:           model.JSONB(`{"familiarity":0}`), // 显式初始化，不依赖 DDL 默认值
	}

	// 人设与配置行同成同败：分两次提交而后者失败，就留下孤儿配置（persona-model §4.3.1）
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.personaRepo.Create(ctx, tx, p); err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		// p.ID 由 BIGSERIAL 回填，播种要用它 —— 顺序不能反
		if err := s.proactiveRepo.CreateDefaultSettings(ctx, tx, userID, p.ID); err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		return nil
	})
	if err != nil {
		// 原样透传：事务内的 BizError 在这里再 Wrap 一次会被洗成别的码
		return nil, err
	}

	resp := dto.NewPersonaResponse(p)
	return &resp, nil
}

// Update 整体替换三个字段，返回更新后的真值。越权与不存在同样返回 4043。
func (s *PersonaService) Update(
	ctx context.Context, userID, personaID uint64, req *dto.UpdatePersonaRequest,
) (*dto.PersonaResponse, error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}

	var p *model.Persona
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		n, err := s.personaRepo.UpdateOwned(
			ctx, tx, userID, personaID,
			req.Name, req.PersonalityDesc, req.SpeakingStyle,
		)
		if err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		if n == 0 {
			// 归属条件在 SQL 里，这里天然不区分"别人的"与"不存在的"
			return errcode.New(errcode.ErrPersonaNotFound)
		}
		// 回读真值：state / familiarity / lastMessageAt 必须来自数据库，不手工拼装
		p, err = s.personaRepo.FindOwned(ctx, userID, personaID)
		if err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	resp := dto.NewPersonaResponse(p)
	return &resp, nil
}

// Delete 删除人设；级联清空其消息、记忆、画像、主动消息配置由数据库外键完成。
func (s *PersonaService) Delete(ctx context.Context, userID, personaID uint64) error {
	if userID == 0 {
		return errcode.New(errcode.ErrUnauthorized)
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		n, err := s.personaRepo.DeleteOwned(ctx, tx, userID, personaID)
		if err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		if n == 0 {
			return errcode.New(errcode.ErrPersonaNotFound)
		}
		return nil
	})
	// 原样透传：事务内的 BizError 在这里再 Wrap 一次会被洗成别的码
	return err
}

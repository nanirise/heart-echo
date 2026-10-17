package service

import (
	"context"
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/dto"
	"github.com/nanirise/heart-echo/backend/internal/model"
	"github.com/nanirise/heart-echo/backend/internal/repository"
	"github.com/nanirise/heart-echo/backend/pkg/errcode"
)

// MomentService 朋友圈四个端点的业务逻辑：归属闸门 + 点赞事务 + DTO 转换。
// 只依赖 context.Context，不感知 *gin.Context。
type MomentService struct {
	db   *gorm.DB
	repo *repository.MomentRepo
}

// NewMomentService 构造服务，并组装它依赖的仓储。
func NewMomentService(db *gorm.DB) *MomentService {
	return &MomentService{db: db, repo: repository.NewMomentRepo(db)}
}

// ListMoments 返回当前用户全部人设的动态流。
func (s *MomentService) ListMoments(
	ctx context.Context, userID uint64, page, pageSize int,
) (*dto.PageResult[dto.MomentItem], error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}

	total, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	page, pageSize = dto.ClampPage(page, pageSize)
	rows, err := s.repo.ListByUser(ctx, userID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	// 必须初始化：nil slice 会序列化成 null
	items := make([]dto.MomentItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.MomentItem{
			ID:           row.ID,
			PersonaID:    row.PersonaID,
			PersonaName:  row.PersonaName,
			Content:      row.Content,
			LikeCount:    row.LikeCount,
			Liked:        row.Liked,
			CommentCount: row.CommentCount,
			CreatedAt:    row.CreatedAt,
		})
	}

	return &dto.PageResult[dto.MomentItem]{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// LikeMoment 幂等点赞：同一用户重复点击只加一次，越权与不存在统一返回 4040。
func (s *MomentService) LikeMoment(
	ctx context.Context, userID, momentID uint64,
) (*dto.LikeResponse, error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}

	var resp dto.LikeResponse
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		n, err := s.repo.InsertLikeIfAbsent(ctx, tx, userID, momentID)
		if err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}

		liked := n == 1
		if liked {
			if err := s.repo.IncrementLikeCount(ctx, tx, momentID); err != nil {
				return errcode.Wrap(errcode.ErrDBFailed, err)
			}
		} else {
			// RowsAffected=0 是二义的：已点过，或这条动态不属于当前账号。
			// 必须再查一次归属消歧；查点赞存在性会把"别人的动态"误判成"已点过"而越权放行。
			if _, err := s.repo.FindOwnedMomentTx(ctx, tx, userID, momentID); err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return errcode.New(errcode.ErrNotFound)
				}
				return errcode.Wrap(errcode.ErrDBFailed, err)
			}
			liked = true
		}

		count, err := s.repo.GetLikeCount(ctx, tx, userID, momentID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errcode.New(errcode.ErrNotFound)
			}
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}
		resp = dto.LikeResponse{LikeCount: count, Liked: liked}
		return nil
	})
	if err != nil {
		// 事务内的 BizError（4040 等）原样透传；Begin/Commit 这类基础设施错误补 5003
		var bizErr *errcode.BizError
		if errors.As(err, &bizErr) {
			return nil, err
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	return &resp, nil
}

// ListComments 返回某条动态的评论。先判归属、再查评论：
// 不属于当前用户时返回 4040，不能返回空列表（否则"别人的动态"与"没有评论"无法区分）。
func (s *MomentService) ListComments(
	ctx context.Context, userID, momentID uint64, page, pageSize int,
) (*dto.PageResult[dto.CommentItem], error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}

	if _, err := s.repo.FindOwnedMoment(ctx, userID, momentID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.New(errcode.ErrNotFound)
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	total, err := s.repo.CountCommentsByUser(ctx, userID, momentID)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	page, pageSize = dto.ClampPage(page, pageSize)
	rows, err := s.repo.ListCommentsByUser(ctx, userID, momentID, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}

	items := make([]dto.CommentItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, dto.CommentItem{
			ID:         row.ID,
			MomentID:   row.MomentID,
			PersonaID:  row.PersonaID,
			UserID:     row.UserID,
			AuthorName: row.AuthorName,
			Content:    row.Content,
			CreatedAt:  row.CreatedAt,
		})
	}

	return &dto.PageResult[dto.CommentItem]{
		List:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CreateComment 以当前用户身份发表评论；AI 评论由定时任务直接写库，不走本端点。
func (s *MomentService) CreateComment(
	ctx context.Context, userID, momentID uint64, content string,
) (*dto.CommentItem, error) {
	if userID == 0 {
		return nil, errcode.New(errcode.ErrUnauthorized)
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errcode.New(errcode.ErrInvalidParams)
	}

	var item dto.CommentItem
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.repo.FindOwnedMomentTx(ctx, tx, userID, momentID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errcode.New(errcode.ErrNotFound)
			}
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}

		comment := &model.MomentComment{
			MomentID: momentID,
			UserID:   &userID,
			Content:  content,
		}
		if err := s.repo.CreateComment(ctx, tx, comment); err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}

		// authorName 是派生字段：用户评论取 users.username，不来自请求体
		author, err := repository.NewUserRepo(tx).FindByID(ctx, userID)
		if err != nil {
			return errcode.Wrap(errcode.ErrDBFailed, err)
		}

		item = dto.CommentItem{
			ID:         comment.ID,
			MomentID:   comment.MomentID,
			PersonaID:  comment.PersonaID,
			UserID:     comment.UserID,
			AuthorName: author.Username,
			Content:    comment.Content,
			CreatedAt:  comment.CreatedAt,
		}
		return nil
	})
	if err != nil {
		var bizErr *errcode.BizError
		if errors.As(err, &bizErr) {
			return nil, err
		}
		return nil, errcode.Wrap(errcode.ErrDBFailed, err)
	}
	return &item, nil
}

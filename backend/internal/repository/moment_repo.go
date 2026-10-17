package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// MomentRepo 朋友圈三表的数据访问。
// ai_moments 没有 user_id 列，归属条件只能 JOIN personas 后落到 p.user_id 上 —— 缺了它就是全库泄漏。
type MomentRepo struct {
	db *gorm.DB
}

// NewMomentRepo 构造仓储；db 用于读路径，写路径由调用方传入事务句柄。
func NewMomentRepo(db *gorm.DB) *MomentRepo {
	return &MomentRepo{db: db}
}

// MomentRow 是动态列表的一行：ai_moments 的列 + personas.name 与两个派生值。
type MomentRow struct {
	ID           uint64
	PersonaID    uint64
	PersonaName  string
	Content      string
	LikeCount    int
	Liked        bool
	CommentCount int64
	CreatedAt    time.Time
}

// CommentRow 是评论列表的一行：作者名来自 COALESCE(personas.name, users.username)。
type CommentRow struct {
	ID         uint64
	MomentID   uint64
	PersonaID  *uint64
	UserID     *uint64
	AuthorName string
	Content    string
	CreatedAt  time.Time
}

// momentsScope 是列表与计数共用的 JOIN + WHERE。
// 两处必须逐字一致：count 漏掉 JOIN personas 会把 total 变成全库行数。
func (r *MomentRepo) momentsScope(ctx context.Context, userID uint64) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("ai_moments AS m").
		Joins("JOIN personas p ON p.id = m.persona_id").
		Joins("LEFT JOIN moment_likes l ON l.moment_id = m.id AND l.user_id = ?", userID).
		Where("p.user_id = ?", userID)
}

// ListByUser 返回当前用户全部人设的动态。
// SELECT 必须写 AS 别名：p.name 不带别名会被 GORM 扫进不存在的 Name 字段，personaName 静默为空。
func (r *MomentRepo) ListByUser(ctx context.Context, userID uint64, offset, limit int) ([]MomentRow, error) {
	rows := make([]MomentRow, 0)
	err := r.momentsScope(ctx, userID).
		Select(`m.id AS id,
			m.persona_id AS persona_id,
			p.name AS persona_name,
			m.content AS content,
			m.like_count AS like_count,
			(l.id IS NOT NULL) AS liked,
			(SELECT count(*) FROM moment_comments c WHERE c.moment_id = m.id) AS comment_count,
			m.created_at AS created_at`).
		Order("m.created_at DESC, m.id DESC").
		Offset(offset).Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// CountByUser 返回当前用户全部人设的动态总数，条件与 ListByUser 一致。
func (r *MomentRepo) CountByUser(ctx context.Context, userID uint64) (int64, error) {
	var total int64
	err := r.momentsScope(ctx, userID).Count(&total).Error
	return total, err
}

// FindOwnedMoment 是点赞/评论的归属闸门；"别人的"与"不存在的"都返回 gorm.ErrRecordNotFound。
func (r *MomentRepo) FindOwnedMoment(ctx context.Context, userID, momentID uint64) (*model.AIMoment, error) {
	return findOwnedMoment(ctx, r.db, userID, momentID)
}

// FindOwnedMomentTx 在事务句柄上复核归属，与后续写入同成同败。
func (r *MomentRepo) FindOwnedMomentTx(
	ctx context.Context, tx *gorm.DB, userID, momentID uint64,
) (*model.AIMoment, error) {
	return findOwnedMoment(ctx, tx, userID, momentID)
}

func findOwnedMoment(
	ctx context.Context, db *gorm.DB, userID, momentID uint64,
) (*model.AIMoment, error) {
	var list []model.AIMoment
	err := db.WithContext(ctx).
		Table("ai_moments AS m").
		Select("m.*").
		Joins("JOIN personas p ON p.id = m.persona_id").
		Where("m.id = ? AND p.user_id = ?", momentID, userID).
		Limit(1).
		Find(&list).Error
	if err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &list[0], nil
}

// InsertLikeIfAbsent 把归属校验与插入合成一条语句：归属不通过时 SELECT 不产出行，
// 不会写入任何东西。返回值 1 = 首次点赞；0 = 已点过或不属于当前用户（二义，由调用方消歧）。
// ON CONFLICT 必须用列清单：uq_moment_like 是唯一索引，不是可用 ON CONSTRAINT 引用的具名约束。
func (r *MomentRepo) InsertLikeIfAbsent(
	ctx context.Context, tx *gorm.DB, userID, momentID uint64,
) (int64, error) {
	res := tx.WithContext(ctx).Exec(`
		INSERT INTO moment_likes (user_id, moment_id, created_at)
		SELECT ?, m.id, NOW()
		FROM ai_moments m
		JOIN personas p ON p.id = m.persona_id
		WHERE m.id = ? AND p.user_id = ?
		ON CONFLICT (user_id, moment_id) DO NOTHING`,
		userID, momentID, userID)
	return res.RowsAffected, res.Error
}

// IncrementLikeCount 只在首次点赞（InsertLikeIfAbsent 返回 1）时调用；
// 无条件执行会让重复点击把计数越点越高，与 moment_likes 的真值漂移。
func (r *MomentRepo) IncrementLikeCount(ctx context.Context, tx *gorm.DB, momentID uint64) error {
	return tx.WithContext(ctx).Model(&model.AIMoment{}).
		Where("id = ?", momentID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

// GetLikeCount 与自增同事务读回，响应里的 likeCount 必须来自数据库而不是内存拼装。
func (r *MomentRepo) GetLikeCount(
	ctx context.Context, tx *gorm.DB, userID, momentID uint64,
) (int, error) {
	var list []struct {
		LikeCount int
	}
	err := tx.WithContext(ctx).
		Table("ai_moments AS m").
		Select("m.like_count AS like_count").
		Joins("JOIN personas p ON p.id = m.persona_id").
		Where("m.id = ? AND p.user_id = ?", momentID, userID).
		Limit(1).
		Find(&list).Error
	if err != nil {
		return 0, err
	}
	if len(list) == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	return list[0].LikeCount, nil
}

// ListCommentsByUser 返回某条动态的评论；IN 子查询是第二道归属闸门，
// 动态不属于当前用户时一行都不会返回。
func (r *MomentRepo) ListCommentsByUser(
	ctx context.Context, userID, momentID uint64, offset, limit int,
) ([]CommentRow, error) {
	rows := make([]CommentRow, 0)
	err := r.db.WithContext(ctx).
		Table("moment_comments AS c").
		Select(`c.id AS id,
			c.moment_id AS moment_id,
			c.persona_id AS persona_id,
			c.user_id AS user_id,
			COALESCE(p.name, u.username) AS author_name,
			c.content AS content,
			c.created_at AS created_at`).
		Joins("LEFT JOIN personas p ON p.id = c.persona_id").
		Joins("LEFT JOIN users u ON u.id = c.user_id").
		Where(`c.moment_id = ? AND c.moment_id IN (
			SELECT m.id FROM ai_moments m
			JOIN personas p2 ON p2.id = m.persona_id
			WHERE m.id = ? AND p2.user_id = ?)`,
			momentID, momentID, userID).
		Order("c.created_at ASC, c.id ASC").
		Offset(offset).Limit(limit).
		Scan(&rows).Error
	return rows, err
}

// CountCommentsByUser 数某条动态的评论；归属条件与 ListCommentsByUser 同源。
func (r *MomentRepo) CountCommentsByUser(
	ctx context.Context, userID, momentID uint64,
) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Table("moment_comments AS c").
		Joins("JOIN ai_moments m ON m.id = c.moment_id").
		Joins("JOIN personas p ON p.id = m.persona_id").
		Where("c.moment_id = ? AND p.user_id = ?", momentID, userID).
		Count(&total).Error
	return total, err
}

// CreateComment 插入一条用户评论；归属校验由调用方在同一事务内先行完成。
func (r *MomentRepo) CreateComment(
	ctx context.Context, tx *gorm.DB, c *model.MomentComment,
) error {
	return tx.WithContext(ctx).Create(c).Error
}

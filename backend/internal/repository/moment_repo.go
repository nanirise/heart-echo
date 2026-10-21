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

// ---- 以下方法仅供定时任务（moment_job）调用，请求路径一律不得引用 ----

// ListAllPersonasForJob 跨用户列出全部人设，供定时任务逐个人设生成动态。
// 这是 spec §4.5 唯一允许的跨用户读方法：系统级任务不属于任何用户，返回行带 UserID + PersonaID。
func (r *MomentRepo) ListAllPersonasForJob(ctx context.Context) ([]model.Persona, error) {
	var list []model.Persona
	err := r.db.WithContext(ctx).Find(&list).Error
	return list, err
}

// CountTodayMoments 数该人设今天（+08:00 零点起）已发布的动态条数，供每日上限判定。
// 日界复用仓库内的 todayMidnight：固定 +08:00，不跟随进程时区。
// 双条件不可省：persona_id 隔离人设，user_id 是越权防线（AGENTS §4.3）。
func (r *MomentRepo) CountTodayMoments(
	ctx context.Context, userID, personaID uint64,
) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).
		Table("ai_moments AS m").
		Joins("JOIN personas p ON p.id = m.persona_id").
		Where("m.persona_id = ? AND p.user_id = ? AND m.created_at >= ?",
			personaID, userID, todayMidnight()).
		Count(&n).Error
	return n, err
}

// GetRecentMomentTime 返回该人设最近一条动态的时间；从未发过时返回 nil。
// 只按 persona_id 查：调用方是系统级任务，且返回值不回给任何用户。
func (r *MomentRepo) GetRecentMomentTime(
	ctx context.Context, personaID uint64,
) (*time.Time, error) {
	var rows []struct {
		LastMomentAt *time.Time
	}
	err := r.db.WithContext(ctx).Model(&model.AIMoment{}).
		Select("MAX(created_at) AS last_moment_at").
		Where("persona_id = ?", personaID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0].LastMomentAt, nil
}

// ListOtherPersonasInSameAccount 取同一账号下的其他人设，作为 AI 评论者。
// user_id 条件不可省：否则 A 的动态下会出现 B 账号的人设名（ai-moment spec §4.6 反例 10）。
func (r *MomentRepo) ListOtherPersonasInSameAccount(
	ctx context.Context, userID, excludePersonaID uint64,
) ([]model.Persona, error) {
	var list []model.Persona
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND id <> ?", userID, excludePersonaID).
		Find(&list).Error
	return list, err
}

// ListRecentMessagesByPersona 取该人设最近的聊天记录（倒序，最新在前），
// 供定时任务给 AI 拼朋友圈上下文。
//
// 两个条件都带：persona_id 隔离人设，user_id 是越权防线（AGENTS §4.3）。
// 过滤 is_nudge：主动消息注入的 "[nudge]" 是系统标记，不是用户说的话，
// 喂给模型只会让它以为用户真发了这两个词。
func (r *MomentRepo) ListRecentMessagesByPersona(
	ctx context.Context, userID, personaID uint64, limit int,
) ([]model.ChatMessage, error) {
	list := make([]model.ChatMessage, 0, limit)
	err := r.db.WithContext(ctx).Model(&model.ChatMessage{}).
		Where("persona_id = ? AND user_id = ? AND is_nudge = ?",
			personaID, userID, false).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Find(&list).Error
	return list, err
}

// ListRecentEventsByPersona 取该人设近期的 event 类记忆，供模板填充生活事件槽位。
// 两个条件都带：persona_id 隔离人设，user_id 是越权防线（AGENTS §4.3）。
func (r *MomentRepo) ListRecentEventsByPersona(
	ctx context.Context, userID, personaID uint64, limit int,
) ([]string, error) {
	var contents []string
	err := r.db.WithContext(ctx).Model(&model.UserMemory{}).
		Where("persona_id = ? AND user_id = ? AND memory_type = ?",
			personaID, userID, model.MemoryTypeEvent).
		Order("created_at DESC, id DESC").
		Limit(limit).
		Pluck("content", &contents).Error
	return contents, err
}

// CreateMoment 落一条动态；动态只由定时任务产生，没有手动触发入口（AGENTS §4.6 / §7.5）。
func (r *MomentRepo) CreateMoment(ctx context.Context, m *model.AIMoment) error {
	return r.db.WithContext(ctx).Create(m).Error
}

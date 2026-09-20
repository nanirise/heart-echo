package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// pgUniqueViolation 是 PostgreSQL 唯一约束冲突的 SQLSTATE，写在官方错误码表里。
const pgUniqueViolation = "23505"

// users 表上两个唯一索引的名字。GORM 建索引的默认命名是 idx_<表名>_<列名>，
// model.User 的 Username / Email 只标了 uniqueIndex、没指定名字，所以恰好是这两个。
//
// ⚠️ 谁把 model.User 的字段或 tag 改名，这里的字符串就会与真实索引名对不上，
// 表现为撞唯一索引时退化成 5003 而不是 4003 / 4004 —— 不报错、只是错。
// user_repo_test.go 里有一条测试直接问 GORM "你到底会建出什么名字"，就是拦这个的。
const (
	indexUsername = "idx_users_username"
	indexEmail    = "idx_users_email"
)

// 撞唯一索引的两个哨兵错误。
// 仓储层只负责回答"撞了哪个唯一约束"，把 23505 翻译成 4003 / 4004 是 service 的事——
// HTTP 语义不该出现在数据库访问层。
var (
	ErrDuplicateUsername = errors.New("repository: username already taken")
	ErrDuplicateEmail    = errors.New("repository: email already registered")
)

// UserRepo 是 users 表的数据访问。
//
// 与人设那些表不同，这张表上的查询**不带 user_id 条件**，这不是漏写：
// users 表本身没有"归属"概念，username / email 是全库唯一的一行，
// 这里不存在越权问题，只有"全库唯一"这一条规则要守。
type UserRepo struct {
	db *gorm.DB
}

// NewUserRepo 构造仓储。
func NewUserRepo(db *gorm.DB) *UserRepo {
	return &UserRepo{db: db}
}

// Create 插入用户，u.ID 与时间戳由数据库 / GORM 回填。
//
// 撞唯一索引时返回 ErrDuplicateUsername / ErrDuplicateEmail 而不是原始错误：
// service 里"先查后插"的那次查询拦不住并发（两个请求同时查到"不存在"、然后一起插入），
// 唯一索引才是最后那道防线，这里必须把它翻译成能认出来的错误。
func (r *UserRepo) Create(ctx context.Context, u *model.User) error {
	err := r.db.WithContext(ctx).Create(u).Error
	if err == nil {
		return nil
	}
	if dup := translateUniqueViolation(err); dup != nil {
		return dup
	}
	return err
}

// FindByUsername 按用户名读一行，未命中返回 gorm.ErrRecordNotFound。
// 返回值带 PasswordHash（登录要比对密码），调用方不得把它拼进任何响应体。
func (r *UserRepo) FindByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).
		Where("username = ?", username).
		First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByEmail 按邮箱读一行，未命中返回 gorm.ErrRecordNotFound。
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	var u model.User
	err := r.db.WithContext(ctx).
		Where("email = ?", email).
		First(&u).Error
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// translateUniqueViolation 把 PostgreSQL 的唯一约束冲突翻译成哨兵错误。
// 不是唯一冲突、或约束名不认识时返回 nil，由调用方按普通数据库错误处理
// （认不出来时宁可退成 5003，也不能猜错列，那会让前端提示错误的字段）。
func translateUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgUniqueViolation {
		return nil
	}

	switch pgErr.ConstraintName {
	case indexUsername:
		return ErrDuplicateUsername
	case indexEmail:
		return ErrDuplicateEmail
	}
	return nil
}

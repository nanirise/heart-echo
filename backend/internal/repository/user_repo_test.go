package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

// testDB 返回一个**不连库**的 *gorm.DB。
// schema 解析与错误翻译都不执行 SQL，只有真的读写数据才需要连接——
// 本机没有 PostgreSQL，这个文件里所有测试也因此一条都不碰数据库。
func testDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		postgres.Open("host=127.0.0.1 port=1 user=x password=x dbname=x sslmode=disable"),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("构造测试用 *gorm.DB 失败: %v", err)
	}
	return db
}

// TestUniqueIndexNamesMatchGorm 断言 user_repo.go 里两个常量 == GORM 会给
// model.User 实际建出的索引名，且它们仍是唯一索引。
//
// 这条测试拦的是两个"不报错、只是错"的改动：
//   - 改了 model.User 的字段名 / tag → 索引改名 → 撞唯一索引时认不出来，
//     退化成 5003 而不是 4003 / 4004
//   - 把 uniqueIndex 改成 index → 唯一约束消失 → 23505 永不发生，
//     并发注册会静默写进重复用户名
//
// 两条路径本机都没有数据库、跑不到，只能靠"直接问 GORM"来兜。
func TestUniqueIndexNamesMatchGorm(t *testing.T) {
	stmt := &gorm.Statement{DB: testDB(t)}
	if err := stmt.Parse(&model.User{}); err != nil {
		t.Fatalf("解析 model.User 的 schema 失败: %v", err)
	}

	byName := map[string]*schema.Index{}
	var built []string
	for _, idx := range stmt.Schema.ParseIndexes() {
		byName[idx.Name] = idx
		built = append(built, idx.Name)
	}

	for _, want := range []string{indexUsername, indexEmail} {
		idx, ok := byName[want]
		if !ok {
			t.Errorf("GORM 不会建出名为 %q 的索引，常量已失效；GORM 实际会建: %v", want, built)
			continue
		}
		if idx.Class != "UNIQUE" {
			t.Errorf("索引 %q 的 class 是 %q 不是 UNIQUE —— 唯一约束已经不在，23505 不会再发生", want, idx.Class)
		}
	}
}

// TestTranslateUniqueViolation 覆盖翻译函数的全部分支。
func TestTranslateUniqueViolation(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want error
	}{
		{"用户名冲突", &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: indexUsername}, ErrDuplicateUsername},
		{"邮箱冲突", &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: indexEmail}, ErrDuplicateEmail},
		{"别的约束冲突（如外键）", &pgconn.PgError{Code: "23503", ConstraintName: indexUsername}, nil},
		{"唯一冲突但约束名不认识", &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: "idx_other_thing"}, nil},
		{"根本不是 pg 错误", errors.New("connection refused"), nil},
		// gorm 可能把驱动错误包一层，翻译函数必须能穿透包装才认得出
		{"被包了一层的唯一冲突", fmt.Errorf("create user: %w", &pgconn.PgError{Code: pgUniqueViolation, ConstraintName: indexEmail}), ErrDuplicateEmail},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := translateUniqueViolation(tc.err)

			if tc.want == nil {
				if got != nil {
					t.Fatalf("应返回 nil（交给调用方按普通数据库错误处理），实际 %v", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("应为 %v，实际 %v", tc.want, got)
			}
		})
	}
}

// Command migrate 对目标数据库执行 schema 迁移（model.AutoMigrate）。
//
// 用法：
//
//	export SCHEMA_CHECK_DSN='postgres://<user>:<password>@<host>:5432/<db>?sslmode=disable'
//	go run ./cmd/migrate
//
// 本工具不设任何默认 DSN，缺失环境变量直接失败——见 AGENTS.md 红线 1。
// 注意：它读的 SCHEMA_CHECK_DSN 与 backend/.env.example 规划的 DB_* 系列不是同一套变量，
// 因此不会随 .env 自动生效，执行前需自行 export；若后续要统一，只改下面这一处。
package main

import (
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/nanirise/heart-echo/backend/internal/model"
)

func main() {
	dsn := os.Getenv("SCHEMA_CHECK_DSN")
	if dsn == "" {
		log.Fatal("环境变量 SCHEMA_CHECK_DSN 未设置")
	}

	// 不做 DisableAutomaticPing：迁移必须连不上就立刻失败，不能"假装在工作"。
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("连接数据库失败：%v", err)
	}

	if err := model.AutoMigrate(db); err != nil {
		log.Fatalf("AutoMigrate 失败：%v", err)
	}

	log.Println("AutoMigrate 完成")
}

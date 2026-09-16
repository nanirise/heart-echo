package model

import "gorm.io/gorm"

// AutoMigrate 建表 / 补列的统一入口，由 cmd/server 启动时调用。
//
// 顺序即依赖顺序：被外键引用的表必须排在前面，否则建外键时会失败。
// 新增模型时在这里追加一行即可。
//
// 红线 8：改表结构只改 struct + AutoMigrate，禁止手写 ALTER TABLE / DropTable。
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},        // 无外键依赖，必须最先建
		&Persona{},     // 引用 users(id) ON DELETE CASCADE，必须在 User 之后
		&ChatMessage{}, // 引用 users(id) 与 personas(id)，两张表都要先建好
		&UserMemory{},  // 引用 users(id) 与 personas(id)，并引用 chat_messages(id)，三张表都要先建好
		&UserProfile{}, // 引用 users(id) 与 personas(id) ON DELETE CASCADE，两张表都要先建好
		// &ProactiveSetting{},  // 待 internal/model/proactive_setting.go 落地后追加（引用 personas(id)）
	)
}

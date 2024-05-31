package main

import (
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	. "mdm_sync/custom"
)

func main() {
	mysqlDB, err := gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Errorf("连接数据库失败: %v", err)
		return
	}

	postgresDB, err := gorm.Open(postgres.Open(PostgresDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Errorf("连接数据库失败: %v", err)
		return
	}

	// 删除旧表
	if err = postgresDB.Exec("DROP TABLE IF EXISTS users, cards, server_logs").Error; err != nil {
		log.Errorf("postgresDB 数据库删除失败: %v", err)
	}

	// 自动迁移数据库结构
	models := []interface{}{&Users{}, &Cards{}, &ServerLogs{}}
	for _, model := range models {
		if err = mysqlDB.AutoMigrate(model); err != nil {
			log.Fatalf("MySQL %v 数据库初始化失败: %v", model, err)
		}
		if err = postgresDB.AutoMigrate(model); err != nil {
			log.Fatalf("PostgreSQL %v 数据库初始化失败: %v", model, err)
		}
	}

	log.Println("初始化完成")
	// 数据同步
	if err = SyncData(mysqlDB, postgresDB, &[]Users{}); err != nil {
		log.Fatalf("数据同步失败: %v", err)
	}
	if err = SyncData(mysqlDB, postgresDB, &[]Cards{}); err != nil {
		log.Fatalf("数据同步失败: %v", err)
	}
	if err = SyncData(mysqlDB, postgresDB, &[]ServerLogs{}); err != nil {
		log.Fatalf("数据同步失败: %v", err)
	}
	//SELECT setval(pg_get_serial_sequence('cards', 'id'), coalesce(max(id)+1, 1), false) FROM server_logs;
	//SELECT setval(pg_get_serial_sequence('users', 'id'), coalesce(max(id)+1, 1), false) FROM server_logs;
	//SELECT setval(pg_get_serial_sequence('server_logs', 'id'), coalesce(max(id)+1, 1), false) FROM server_logs;
	if err = postgresDB.Exec("SELECT setval(pg_get_serial_sequence('cards', 'id'), coalesce(max(id)+1, 1), false) FROM cards;").Error; err != nil {
		log.Fatalf("PostgreSQL cards 表 ID 更新失败: %v", err)
	}
	if err = postgresDB.Exec("SELECT setval(pg_get_serial_sequence('users', 'id'), coalesce(max(id)+1, 1), false) FROM users;").Error; err != nil {
		log.Fatalf("PostgreSQL users 表 ID 更新失败: %v", err)
	}
	if err = postgresDB.Exec("SELECT setval(pg_get_serial_sequence('server_logs', 'id'), coalesce(max(id)+1, 1), false) FROM server_logs;").Error; err != nil {
		log.Fatalf("PostgreSQL server_logs 表 ID 更新失败: %v", err)
	}
	log.Println("数据同步完成")
}

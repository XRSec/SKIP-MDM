package main

import (
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	. "mdm_sync/custom"
)

func main() {
	sqliteDB, err := gorm.Open(sqlite.Open(SqliteDSN), &gorm.Config{})
	if err != nil {
		log.Errorf("连接数据库失败: %v", err)
		return
	}
	mysqlDB, err := gorm.Open(mysql.Open(MysqlDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Errorf("连接数据库失败: %v", err)
		return
	}

	if err = mysqlDB.Exec("DROP TABLE users").Error; err != nil {
		log.Errorf("mysqlDB Users 数据库删除失败: %v", err)
	}
	if err = mysqlDB.Exec("DROP TABLE cards").Error; err != nil {
		log.Errorf("mysqlDB Cards 数据库删除失败: %v", err)
	}

	if err = sqliteDB.AutoMigrate(&Users{}); err != nil {
		log.Errorf("sqliteDB Users 数据库初始化失败: %v", err)
		return
	}
	if err = mysqlDB.AutoMigrate(&Users{}); err != nil {
		log.Errorf("mysqlDB Users 数据库初始化失败: %v", err)
		return
	}
	if err = sqliteDB.AutoMigrate(&Cards{}); err != nil {
		log.Errorf("sqliteDB Cards 数据库初始化失败: %v", err)
		return
	}
	if err = mysqlDB.AutoMigrate(&Cards{}); err != nil {
		log.Errorf("mysqlDB Cards 数据库初始化失败: %v", err)
		return
	}

	// 从 SQLite 中读取数据
	var sqliteUserRecords []Users
	var sqliteCardRecords []Cards
	sqliteDB.Find(&sqliteUserRecords)
	sqliteDB.Find(&sqliteCardRecords)

	if err = mysqlDB.Create(&sqliteUserRecords).Error; err != nil {
		log.Errorf("mysqlDB Users 数据库迁移失败: %v", err)
		return
	}
	if err = mysqlDB.Create(&sqliteCardRecords).Error; err != nil {
		log.Errorf("mysqlDB Cards 数据库迁移失败: %v", err)
		return
	}

	log.Info("数据迁移完成")

	//UPDATE cards SET created_at = '2023-09-24 16:26:54.36721417+08:00', updated_at = '2023-09-24 16:26:54.36721417+08:00', deleted_at = null, card_id = 'ma0001', password = 'ose9IWKBO2Nb7aT', serial_number = 'x9qf7lvmpm' WHERE id = 1;
	//INSERT INTO cards (id, created_at, updated_at, deleted_at, card_id, password, serial_number) VALUES (1, '2023-09-24 16:26:54.36721417+08:00', '2023-09-24 16:26:54.36721417+08:00', null, 'ma0001', 'ose9IWKBO2Nb7aT', 'x9qf7lvmpm');
}

// migrate.go - 数据迁移工具
// 用途：从备份恢复数据到云端 MySQL
// 变更：cards 表废弃，users 表移除 card_id，card_type 改为 rule (0->1, 1->2)

package main

import (
	"fmt"
	. "mdm_sync/custom"
	"os"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	mysqlDB  *gorm.DB
	backupDB *gorm.DB
)

const backupFile = "backup_20260114_000637.db"

func init() {
	if _, err := os.Stat("../../server/zoneinfo.zip"); err == nil {
		fmt.Println("初始化 ZONEINFO", os.Setenv("ZONEINFO", "../../server/zoneinfo.zip"))
	}
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
}

func main() {
	var err error

	// 连接备份数据库
	backupDB, err = gorm.Open(sqlite.Open(backupFile), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Fatalf("打开备份数据库失败: %v", err)
	}
	log.Infof("打开备份数据库: %s", backupFile)

	// 连接云端 MySQL
	mysqlDB, err = gorm.Open(mysql.Open(MysqlDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Fatalf("连接云端MySQL失败: %v", err)
	}
	log.Infof("连接云端MySQL成功")

	// 清空并重建表
	if err := mysqlDB.Exec("DROP TABLE IF EXISTS users").Error; err != nil {
		log.Fatalf("删除 users 表失败: %v", err)
	}
	log.Infof("已删除旧 users 表")

	if err := mysqlDB.AutoMigrate(&Users{}); err != nil {
		log.Fatalf("创建 users 表失败: %v", err)
	}
	log.Infof("已创建新 users 表结构")

	// 从备份读取旧用户数据
	var oldUsers []OldUsers
	if err := backupDB.Find(&oldUsers).Error; err != nil {
		log.Fatalf("读取备份数据失败: %v", err)
	}
	log.Infof("读取到 %d 条用户数据", len(oldUsers))

	// 转换数据
	var newUsers []Users
	for _, old := range oldUsers {
		newUser := ConvertOldUserToNew(old)
		newUser.ID = 0
		newUsers = append(newUsers, newUser)
	}

	// 批量插入，每次 200 条
	batchSize := 200
	totalBatches := (len(newUsers) + batchSize - 1) / batchSize

	for i := 0; i < len(newUsers); i += batchSize {
		end := i + batchSize
		if end > len(newUsers) {
			end = len(newUsers)
		}
		batch := newUsers[i:end]
		batchNum := i/batchSize + 1

		if err := mysqlDB.Create(&batch).Error; err != nil {
			log.Errorf("批次 %d/%d 上传失败: %v", batchNum, totalBatches, err)
			continue
		}
		log.Infof("批次 %d/%d 上传成功，%d 条", batchNum, totalBatches, len(batch))
	}

	log.Infof("恢复完成，共上传 %d 条用户数据", len(newUsers))
}

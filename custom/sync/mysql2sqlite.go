package main

import (
	"bufio"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	. "mdm_sync/custom"
	"os"
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

	// 自动迁移数据库结构
	models := []interface{}{&Users{}, &Cards{}}
	for _, model := range models {
		if err = sqliteDB.Migrator().DropTable(model); err != nil {
			log.Fatalf("MySQL %v 数据库删除失败: %v", model, err)
		}
		if err = mysqlDB.AutoMigrate(model); err != nil {
			log.Fatalf("MySQL %v 数据库初始化失败: %v", model, err)
		}
		if err = sqliteDB.AutoMigrate(model); err != nil {
			log.Fatalf("sqliteDB %v 数据库初始化失败: %v", model, err)
		}
	}
	if err = mysqlDB.AutoMigrate(&ServerLogs{}); err != nil {
		log.Fatalf("MySQL %v 数据库初始化失败: %v", "ServerLogs", err)
	}

	log.Println("初始化完成")
	// 数据同步
	if err = SyncData(mysqlDB, sqliteDB, &[]Users{}); err != nil {
		log.Fatalf("数据同步失败: %v", err)
	}
	if err = SyncData(mysqlDB, sqliteDB, &[]Cards{}); err != nil {
		log.Fatalf("数据同步失败: %v", err)
	}
	if err = syncLogs(mysqlDB); err != nil {
		log.Fatalf("数据同步失败: %v", err)
	}

	log.Println("数据同步完成")
}

func syncLogs(mysqlDB *gorm.DB) error {
	// 打开（或创建）app.log 文件，并准备追加内容
	file, err := os.OpenFile("../../server/logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("打开文件失败: %v", err)
		return err
	}

	writer := bufio.NewWriter(file)

	var records = ""
	var serverLogs []ServerLogs
	var offset, totalRecords int
	var batchSize = 500

	for {
		// 从源数据库中选择要迁移的数据，限制每次批量选择的数量
		result := mysqlDB.Offset(offset).Limit(batchSize).Find(&serverLogs)
		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			return result.Error
		}
		log.Infof("从 %T 中选择 %d 条记录", serverLogs, result.RowsAffected)
		// 如果没有记录了，退出循环
		if result.RowsAffected == 0 {
			break
		}

		// 将数据追加到 record
		for _, v := range serverLogs {
			records += fmt.Sprintf("%v %v | %5v | %13v | %15s | %-7s %s\n",
				v.APP,
				v.Timestamp.Format("2006/01/02 15:04:05"),
				v.Status,
				v.Latency,
				v.IP,
				v.Method,
				v.Path,
			)
		}

		// 更新偏移量和总记录数
		offset += batchSize
		totalRecords += int(result.RowsAffected)
	}

	if _, err := writer.WriteString(records); err != nil {
		log.Fatalf("写入文件失败: %v", err)
		return err
	}

	// 确保所有缓冲的数据都写入文件
	if err := writer.Flush(); err != nil {
		log.Fatalf("刷新缓冲失败: %v", err)
		return err
	}

	if err := file.Close(); err != nil {
		log.Fatalf("关闭文件失败: %v", err)
		return err
	}

	log.Printf("已同步 %d 条记录", totalRecords)

	return nil
}

package main

import (
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	. "mdm_sync/custom"
	"os"
)

func main() {
	if _, err := os.Stat("../../server/zoneinfo.zip"); err == nil {
		fmt.Println("初始化 ZONEINFO", os.Setenv("../../server/ZONEINFO", "zoneinfo.zip"))
	}
	mysqlDB, err := gorm.Open(mysql.Open(MysqlDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Errorf("连接mysqlDB失败: %v", err)
		return
	}
	log.Infof("连接mysqlDB成功")

	postgresDB, err := gorm.Open(postgres.Open(PostgresDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Errorf("连接postgresDB失败: %v", err)
		return
	}
	log.Infof("连接postgresDB成功")

	var auditLogs []AuditLog
	if err := postgresDB.Find(&auditLogs).Error; err != nil {
		log.Fatalf("查询审计日志时失败: %v", err)
	}
	log.Infof("查询审计日志成功")

	// Write audit logs to MySQL
	for _, field := range auditLogs {
		var operation interface{}
		switch field.TableName {
		case "cards":
			operation = &Cards{}
		case "users":
			operation = &Users{}
		case "server_logs":
			operation = &ServerLogs{}
		default:
			log.Fatalf("不支持的表: %s", field.TableName)
		}
		data := []byte(field.NewData)
		if field.Operation == "DELETE" {
			data = []byte(field.OldData)
		}
		if err := json.Unmarshal(data, &operation); err != nil {
			log.Fatalf("解析新数据时失败: %v", err)
		}

		switch op := operation.(type) {
		case *ServerLogs:
			op.ID = 0
		}

		if field.Operation == "INSERT" {
			if err := mysqlDB.Table(field.TableName).Create(operation).Error; err != nil {
				log.Fatalf("插入mysqlDB数据时失败: %v", err)
			}
			log.Infof("插入mysqlDB数据 OID:%v TBName:%v [ %v ]", field.ID, field.TableName, field.NewData)
		} else if field.Operation == "UPDATE" {
			if err := mysqlDB.Table(field.TableName).Save(operation).Error; err != nil {
				log.Fatalf("更新mysqlDB数据时失败: %v", err)
			}
			log.Infof("更新mysqlDB数据 OID:%v TBName:%v [ %v ] [ %v ]", field.ID, field.TableName, field.NewData)
		} else if field.Operation == "DELETE" {
			if field.TableName == "server_logs" {
				continue
			}
			if err := mysqlDB.Unscoped().Table(field.TableName).Delete(operation).Error; err != nil {
				log.Fatalf("删除mysqlDB数据时失败: %v", err)
			}
			log.Infof("删除mysqlDB数据 OID:%v TBName:%v [ %v ]", field.ID, field.TableName, field.OldData)
		} else {
			log.Fatalf("不支持的操作: %s", field.Operation)
		}
		if err := postgresDB.Delete(&field).Error; err != nil {
			log.Fatalf("删除postgresDB记录失败: %v", err)
		}
		log.Infof("删除postgresDB记录 OID:%v", field.ID)
	}
	log.Infof("同步完成, 共%v条记录", len(auditLogs))
}

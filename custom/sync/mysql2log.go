package main

import (
	"bufio"
	"fmt"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	. "mdm_sync/custom"
	"os"
	"time"
)

var (
	err        error
	db         *gorm.DB
	doc        = ""
	debug      = "true"
	PrivateIP  = "107.148.31.165"
	PublicIP   = "mdm.xrsec.fun"
	serverPort = "9000"         // 9000 | 6
	htmlPath   = "/tmp"         // /tmp | html
	logPath    = "/tmp/app.log" // logs/app.log | /tmp/app.log
)

func init() {
	if _, err := os.Stat("../../server/zoneinfo.zip"); os.IsExist(err) {
		fmt.Println(os.Setenv("ZONEINFO", "./../server/zoneinfo.zip"))
	}
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	db, err = gorm.Open(mysql.Open(MysqlDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Errorf("连接数据库失败%v", err)
		return
	}
	if err = db.AutoMigrate(&ServerLogs{}); err != nil {
		log.Errorf("Logs 数据库初始化失败: %v", err)
		return
	}
}

func main() {
	// 获取当天 00:00 的时间
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	log.Infof("查询时间范围: %v - %v", midnight, midnight.AddDate(0, 0, 1))

	// 查询 00:00 之前的数据
	var serverLogs []ServerLogs
	var records = ""
	if err := db.Where("timestamp < ?", midnight).Find(&serverLogs).Error; err != nil {
		log.Fatalf("查询数据失败: %v", err)
	}

	// 打开（或创建）app.log 文件，并准备追加内容
	file, err := os.OpenFile("../../server/logs/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("打开文件失败: %v", err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatalf("关闭文件失败: %v", err)
		}
	}(file)

	writer := bufio.NewWriter(file)

	// 将数据写入到 app.log 文件
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

	if _, err := writer.WriteString(records); err != nil {
		log.Fatalf("写入文件失败: %v", err)
	}

	// 确保所有缓冲的数据都写入文件
	if err := writer.Flush(); err != nil {
		log.Fatalf("刷新缓冲失败: %v", err)
	}

	os.Exit(0) // TODO 不敢删除数据

	// 删除 00:00 之前的数据
	if err := db.Where("timestamp < ?", midnight).Delete(ServerLogs{}).Error; err != nil {
		log.Fatalf("Failed to delete logs: %v", err)
	}
}

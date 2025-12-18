package main

import (
	"bufio"
	"fmt"
	"log"
	. "mdm_sync/custom"
	"os"
	"regexp"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	appName         = "[GIN]"               // 统一应用名称常量
	batchSize       = 500                   // 批处理大小
	logProcessCount = 500                   // 日志处理计数间隔
	timeLayout      = "2006/01/02 15:04:05" // 时间格式
	timezone        = "Asia/Shanghai"
)

/**

	grep -E ' "(/d|/add|/auth|/del|/getLatestID|/getLatest|/unsafe|/getLogs|/getCard|/getKami|/cardDel|/cardUpdate)\?' ../../server/logs/app.log \
	| tac \
	| awk '!seen[$12]++' \
	| tac > filtered_app.log

	SELECT *
	FROM server_logs
	ORDER BY `timestamp` DESC
	LIMIT 10;

 	SELECT * FROM server_logs
	WHERE
	path = '"/favicon.ico"'
	    OR path = '"/"'
	    OR path LIKE '%.js%'
	    OR path LIKE '%auth_error%'
			OR path LIKE '"/auth?serial_number="'
	;
 	DELETE FROM server_logs
	WHERE
	path = '"/favicon.ico"'
	    OR path = '"/"'
	    OR path LIKE '%.js%'
	    OR path LIKE '%auth_error%'
			OR path LIKE '"/auth?serial_number="'
	;
**/

func main() {
	// 设置时区 - 双重保障
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("警告: 无法加载时区 '%s'，使用系统默认时区: %v", timezone, err)
		loc = time.Local
	}

	// 设置全局时区
	time.Local = loc

	// 连接数据库（确保使用正确的时区）
	db, err := connectDBWithTimezone(loc)
	if err != nil {
		log.Fatal(err)
	}

	// 自动迁移表结构
	if err := db.AutoMigrate(&ServerLogs{}); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	// 清空并重置表
	if err := resetDatabase(db); err != nil {
		log.Fatal(err)
	}

	// 读取并处理日志文件
	logFile := "filtered_app.log"
	totalCount, err := processLogFile(db, logFile, loc)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("导入完成！总共导入了 %d 条记录\n", totalCount)
}

// 带时区设置的数据库连接
func connectDBWithTimezone(loc *time.Location) (*gorm.DB, error) {
	// 获取时区名称（如 "Asia/Shanghai"）
	zoneName := loc.String()

	// 调整 DSN 添加时区参数
	adjustedDSN := MysqlDSN
	if !strings.Contains(adjustedDSN, "parseTime") {
		if strings.Contains(adjustedDSN, "?") {
			adjustedDSN += "&parseTime=True"
		} else {
			adjustedDSN += "?parseTime=True"
		}
	}

	// 添加时区参数
	if !strings.Contains(adjustedDSN, "loc=") {
		adjustedDSN += fmt.Sprintf("&loc=%s", zoneName)
	}

	return gorm.Open(mysql.Open(adjustedDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
}

// 重置数据库表
func resetDatabase(db *gorm.DB) error {
	fmt.Println("正在清空server_logs表...")
	if err := db.Exec("DELETE FROM server_logs").Error; err != nil {
		return fmt.Errorf("清空表失败: %w", err)
	}

	// 重置AUTO_INCREMENT
	if err := db.Exec("ALTER TABLE server_logs AUTO_INCREMENT = 1").Error; err != nil {
		return fmt.Errorf("重置AUTO_INCREMENT失败: %w", err)
	}

	fmt.Println("表已清空并重置ID")
	return nil
}

// 处理日志文件（添加时区参数）
func processLogFile(db *gorm.DB, filename string, loc *time.Location) (int64, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, fmt.Errorf("打开日志文件失败: %w", err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	// 解析日志的正则表达式
	re := regexp.MustCompile(`\[GIN] (\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \| +(\d+) \| +(\d+\.?\d*[µmn]?s) \| +(\d+\.\d+\.\d+\.\d+) \| (\w+) +(".*?")`)

	scanner := bufio.NewScanner(file)
	var logs []ServerLogs
	count := 0
	var currentLine string

	fmt.Println("开始解析日志文件...")

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "[GIN]") {
			if currentLine != "" {
				if logEntry, ok := parseLogLine(re, currentLine, loc); ok {
					logs = append(logs, logEntry)
					count++
					if count%batchSize == 0 {
						if err := insertBatch(db, logs); err != nil {
							return 0, err
						}
						logs = logs[:0]
					}
				}
			}
			currentLine = line
		} else {
			fmt.Println(fmt.Sprintf("日志格式不匹配: [%v]", line))
			currentLine += line
		}
	}

	// 处理最后一行
	if currentLine != "" {
		if logEntry, ok := parseLogLine(re, currentLine, loc); ok {
			logs = append(logs, logEntry)
			count++
		}
	}

	// 插入剩余的记录
	if len(logs) > 0 {
		if err := insertBatch(db, logs); err != nil {
			return 0, err
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("读取文件失败: %w", err)
	}

	// 查询总记录数
	var totalCount int64
	if err := db.Model(&ServerLogs{}).Count(&totalCount).Error; err != nil {
		return 0, fmt.Errorf("查询记录数失败: %w", err)
	}
	fmt.Println("最后一条记录的时间: ", logs[len(logs)-1].Timestamp)

	return totalCount, nil
}

// 解析单行日志（添加时区参数）
func parseLogLine(re *regexp.Regexp, line string, loc *time.Location) (ServerLogs, bool) {
	matches := re.FindStringSubmatch(line)
	if len(matches) != 7 {
		fmt.Printf("日志格式不匹配: %s\n", line)
		return ServerLogs{}, false
	}

	// 使用指定时区解析时间戳
	timestamp, err := time.ParseInLocation(timeLayout, matches[1], loc)
	if err != nil {
		fmt.Printf("解析时间戳失败: %s, 错误: %v\n", matches[1], err)
		return ServerLogs{}, false
	}

	return ServerLogs{
		Timestamp: timestamp,
		APP:       appName,
		Method:    matches[5],
		Path:      matches[6],
		IP:        matches[4],
		Status:    matches[2],
		Latency:   matches[3],
	}, true
}

// 批量插入记录
func insertBatch(db *gorm.DB, logs []ServerLogs) error {
	if len(logs) == 0 {
		return nil
	}

	if err := db.CreateInBatches(logs, len(logs)).Error; err != nil {
		return fmt.Errorf("批量插入失败: %w", err)
	}

	fmt.Printf("已批量插入 %d 条记录\n", len(logs))
	return nil
}

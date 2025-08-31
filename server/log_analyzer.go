package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// SystemInfo 系统信息结构体（与客户端保持一致）
type SystemInfo struct {
	SerialNumber    string            `json:"serial_number"`
	OSVersion       string            `json:"os_version"`
	Timestamp       string            `json:"timestamp"`
	Volumes         []string          `json:"volumes"`
	LaunchAgents    []string          `json:"launch_agents"`
	LaunchDaemons   []string          `json:"launch_daemons"`
	AppSupport      []string          `json:"app_support"`
	UserPrefs       []string          `json:"user_prefs"`
	Applications    []string          `json:"applications"`
	MDMSettings     []string          `json:"mdm_settings"`
	CloudConfig     string            `json:"cloud_config"`
	MDMDomains      []string          `json:"mdm_domains"`
	SystemLogs      []string          `json:"system_logs"`
	ProcessList     []string          `json:"process_list"`
	NetworkInfo     map[string]string `json:"network_info"`
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	SerialNumber     string   `json:"serial_number"`
	TotalLogs        int      `json:"total_logs"`
	FirstSeen        string   `json:"first_seen"`
	LastSeen         string   `json:"last_seen"`
	OSVersions       []string `json:"os_versions"`
	MDMSoftware      []string `json:"mdm_software"`
	SuspiciousFiles  []string `json:"suspicious_files"`
	NetworkChanges   int      `json:"network_changes"`
	CleaningAttempts int      `json:"cleaning_attempts"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run log_analyzer.go [analyze|report|search] [options]")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  analyze <serial_number>  - Analyze logs for specific device")
		fmt.Println("  report                   - Generate summary report")
		fmt.Println("  search <keyword>         - Search logs by keyword")
		os.Exit(1)
	}

	// 连接数据库
	dsn := "root:password@tcp(localhost:3306)/mdm_logs?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	command := os.Args[1]

	switch command {
	case "analyze":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run log_analyzer.go analyze <serial_number>")
			os.Exit(1)
		}
		analyzeDevice(db, os.Args[2])
	case "report":
		generateReport(db)
	case "search":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run log_analyzer.go search <keyword>")
			os.Exit(1)
		}
		searchLogs(db, os.Args[2])
	default:
		fmt.Println("Unknown command:", command)
		os.Exit(1)
	}
}

// 分析特定设备
func analyzeDevice(db *sql.DB, serialNumber string) {
	fmt.Printf("Analyzing device: %s\n", serialNumber)
	fmt.Println(strings.Repeat("=", 50))

	// 查询设备的所有日志
	query := `
		SELECT raw_data, server_timestamp 
		FROM mdm_logs 
		WHERE serial_number = ? 
		ORDER BY server_timestamp ASC
	`

	rows, err := db.Query(query, serialNumber)
	if err != nil {
		log.Fatal("Query failed:", err)
	}
	defer rows.Close()

	var result AnalysisResult
	result.SerialNumber = serialNumber
	mdmSoftwareSet := make(map[string]bool)
	suspiciousSet := make(map[string]bool)
	osVersionSet := make(map[string]bool)

	for rows.Next() {
		var rawData string
		var timestamp time.Time
		
		if err := rows.Scan(&rawData, &timestamp); err != nil {
			continue
		}

		result.TotalLogs++
		
		if result.FirstSeen == "" {
			result.FirstSeen = timestamp.Format("2006-01-02 15:04:05")
		}
		result.LastSeen = timestamp.Format("2006-01-02 15:04:05")

		// 解析JSON数据
		var info SystemInfo
		if err := json.Unmarshal([]byte(rawData), &info); err != nil {
			continue
		}

		// 收集OS版本
		if info.OSVersion != "" {
			osVersionSet[info.OSVersion] = true
		}

		// 分析MDM软件
		allFiles := append(info.LaunchAgents, info.LaunchDaemons...)
		allFiles = append(allFiles, info.AppSupport...)
		allFiles = append(allFiles, info.Applications...)

		for _, file := range allFiles {
			lowerFile := strings.ToLower(file)
			if strings.Contains(lowerFile, "jamf") ||
			   strings.Contains(lowerFile, "mosyle") ||
			   strings.Contains(lowerFile, "mdm") ||
			   strings.Contains(lowerFile, "management") {
				mdmSoftwareSet[file] = true
			}
		}

		// 检查可疑文件
		for _, file := range allFiles {
			lowerFile := strings.ToLower(file)
			if strings.Contains(lowerFile, "tinyapp") ||
			   strings.Contains(lowerFile, "freshservice") ||
			   strings.Contains(lowerFile, "zoom") {
				suspiciousSet[file] = true
			}
		}
	}

	// 转换为切片
	for version := range osVersionSet {
		result.OSVersions = append(result.OSVersions, version)
	}
	for software := range mdmSoftwareSet {
		result.MDMSoftware = append(result.MDMSoftware, software)
	}
	for file := range suspiciousSet {
		result.SuspiciousFiles = append(result.SuspiciousFiles, file)
	}

	// 输出分析结果
	fmt.Printf("Total logs: %d\n", result.TotalLogs)
	fmt.Printf("First seen: %s\n", result.FirstSeen)
	fmt.Printf("Last seen: %s\n", result.LastSeen)
	fmt.Printf("OS versions: %v\n", result.OSVersions)
	fmt.Printf("MDM software detected: %v\n", result.MDMSoftware)
	fmt.Printf("Suspicious files: %v\n", result.SuspiciousFiles)
}

// 生成总体报告
func generateReport(db *sql.DB) {
	fmt.Println("MDM Log Collection Report")
	fmt.Println(strings.Repeat("=", 50))

	// 统计总数
	var totalLogs, uniqueDevices int
	db.QueryRow("SELECT COUNT(*), COUNT(DISTINCT serial_number) FROM mdm_logs").Scan(&totalLogs, &uniqueDevices)
	
	fmt.Printf("Total logs: %d\n", totalLogs)
	fmt.Printf("Unique devices: %d\n", uniqueDevices)

	// 最近7天的活动
	var recentLogs int
	db.QueryRow("SELECT COUNT(*) FROM mdm_logs WHERE server_timestamp >= DATE_SUB(NOW(), INTERVAL 7 DAY)").Scan(&recentLogs)
	fmt.Printf("Logs in last 7 days: %d\n", recentLogs)

	// 最活跃的设备
	fmt.Println("\nTop 10 most active devices:")
	rows, err := db.Query(`
		SELECT serial_number, COUNT(*) as log_count, MAX(server_timestamp) as last_seen
		FROM mdm_logs 
		GROUP BY serial_number 
		ORDER BY log_count DESC 
		LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var serial string
			var count int
			var lastSeen time.Time
			rows.Scan(&serial, &count, &lastSeen)
			fmt.Printf("  %s: %d logs (last: %s)\n", serial, count, lastSeen.Format("2006-01-02 15:04"))
		}
	}

	// 最常见的OS版本
	fmt.Println("\nMost common OS versions:")
	rows, err = db.Query(`
		SELECT 
			JSON_UNQUOTE(JSON_EXTRACT(raw_data, '$.os_version')) as os_version,
			COUNT(*) as count
		FROM mdm_logs 
		WHERE JSON_EXTRACT(raw_data, '$.os_version') IS NOT NULL
		GROUP BY os_version
		ORDER BY count DESC
		LIMIT 5
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var version string
			var count int
			rows.Scan(&version, &count)
			fmt.Printf("  %s: %d devices\n", version, count)
		}
	}
}

// 搜索日志
func searchLogs(db *sql.DB, keyword string) {
	fmt.Printf("Searching for: %s\n", keyword)
	fmt.Println(strings.Repeat("=", 50))

	query := `
		SELECT serial_number, server_timestamp, ip_address
		FROM mdm_logs 
		WHERE raw_data LIKE ?
		ORDER BY server_timestamp DESC
		LIMIT 50
	`

	rows, err := db.Query(query, "%"+keyword+"%")
	if err != nil {
		log.Fatal("Search failed:", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var serial, ip string
		var timestamp time.Time
		
		if err := rows.Scan(&serial, &timestamp, &ip); err != nil {
			continue
		}

		fmt.Printf("%s | %s | %s\n", 
			timestamp.Format("2006-01-02 15:04:05"), 
			serial, 
			ip)
		count++
	}

	fmt.Printf("\nFound %d matching logs\n", count)
}
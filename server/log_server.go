package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// SystemInfo 系统信息结构体
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

// LogEntry 数据库日志条目
type LogEntry struct {
	ID              int       `json:"id"`
	SerialNumber    string    `json:"serial_number"`
	OSVersion       string    `json:"os_version"`
	ClientTimestamp string    `json:"client_timestamp"`
	ServerTimestamp time.Time `json:"server_timestamp"`
	RawData         string    `json:"raw_data"`
	IPAddress       string    `json:"ip_address"`
	UserAgent       string    `json:"user_agent"`
}

var db *sql.DB

// 初始化数据库连接
func initDB() {
	var err error
	// 修改为您的数据库连接信息
	dsn := "root:password@tcp(localhost:3306)/mdm_logs?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 测试连接
	if err = db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	// 创建表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS mdm_logs (
		id INT AUTO_INCREMENT PRIMARY KEY,
		serial_number VARCHAR(255) NOT NULL,
		os_version VARCHAR(100),
		client_timestamp VARCHAR(50),
		server_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		raw_data LONGTEXT,
		ip_address VARCHAR(45),
		user_agent VARCHAR(500),
		INDEX idx_serial_number (serial_number),
		INDEX idx_server_timestamp (server_timestamp)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`

	if _, err = db.Exec(createTableSQL); err != nil {
		log.Fatal("Failed to create table:", err)
	}

	log.Println("Database initialized successfully")
}

// 验证认证token
func validateAuth(authHeader, serialNumber string) bool {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	
	token := strings.TrimPrefix(authHeader, "Bearer ")
	
	// 生成预期的token（允许时间窗口为±5分钟）
	now := time.Now()
	for i := -5; i <= 5; i++ {
		testTime := now.Add(time.Duration(i) * time.Minute)
		expectedToken := generateExpectedToken(serialNumber, testTime)
		if token == expectedToken {
			return true
		}
	}
	
	return false
}

// 生成预期的认证token
func generateExpectedToken(serialNumber string, timestamp time.Time) string {
	hash := sha256.New()
	timeStr := timestamp.Format("200601021504")
	data := "mdm_log_auth" + serialNumber + timeStr + "m'd'm"
	hash.Write([]byte(data))
	return hex.EncodeToString(hash.Sum(nil))[:32]
}

// 处理日志接收
func handleLog(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 读取请求体
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// 解析JSON
	var systemInfo SystemInfo
	if err := json.Unmarshal(body, &systemInfo); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// 验证认证
	authHeader := r.Header.Get("Authorization")
	if !validateAuth(authHeader, systemInfo.SerialNumber) {
		log.Printf("Authentication failed for serial: %s", systemInfo.SerialNumber)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 获取客户端IP
	clientIP := r.Header.Get("X-Real-IP")
	if clientIP == "" {
		clientIP = r.Header.Get("X-Forwarded-For")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	// 存储到数据库
	insertSQL := `
		INSERT INTO mdm_logs (
			serial_number, os_version, client_timestamp, 
			raw_data, ip_address, user_agent
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	rawDataJSON, _ := json.Marshal(systemInfo)
	
	_, err = db.Exec(insertSQL,
		systemInfo.SerialNumber,
		systemInfo.OSVersion,
		systemInfo.Timestamp,
		string(rawDataJSON),
		clientIP,
		r.Header.Get("User-Agent"),
	)

	if err != nil {
		log.Printf("Error inserting to database: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Printf("Log received from serial: %s, IP: %s", systemInfo.SerialNumber, clientIP)
	
	// 返回成功响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Log received successfully",
	})
}

// 处理日志查询
func handleQuery(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取查询参数
	serialNumber := r.URL.Query().Get("serial")
	limitStr := r.URL.Query().Get("limit")
	
	limit := 100 // 默认限制
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 && parsedLimit <= 1000 {
			limit = parsedLimit
		}
	}

	var rows *sql.Rows
	var err error

	if serialNumber != "" {
		// 查询特定序列号的日志
		querySQL := `
			SELECT id, serial_number, os_version, client_timestamp, 
				   server_timestamp, ip_address, user_agent
			FROM mdm_logs 
			WHERE serial_number = ?
			ORDER BY server_timestamp DESC 
			LIMIT ?
		`
		rows, err = db.Query(querySQL, serialNumber, limit)
	} else {
		// 查询所有日志
		querySQL := `
			SELECT id, serial_number, os_version, client_timestamp, 
				   server_timestamp, ip_address, user_agent
			FROM mdm_logs 
			ORDER BY server_timestamp DESC 
			LIMIT ?
		`
		rows, err = db.Query(querySQL, limit)
	}

	if err != nil {
		log.Printf("Error querying database: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []LogEntry
	for rows.Next() {
		var entry LogEntry
		err := rows.Scan(
			&entry.ID,
			&entry.SerialNumber,
			&entry.OSVersion,
			&entry.ClientTimestamp,
			&entry.ServerTimestamp,
			&entry.IPAddress,
			&entry.UserAgent,
		)
		if err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		logs = append(logs, entry)
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"count":  len(logs),
		"logs":   logs,
	})
}

// 处理详细日志查询
func handleDetail(w http.ResponseWriter, r *http.Request) {
	// 设置CORS头
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取日志ID
	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "Missing log ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid log ID", http.StatusBadRequest)
		return
	}

	// 查询详细日志
	querySQL := `
		SELECT id, serial_number, os_version, client_timestamp, 
			   server_timestamp, raw_data, ip_address, user_agent
		FROM mdm_logs 
		WHERE id = ?
	`

	var entry LogEntry
	err = db.QueryRow(querySQL, id).Scan(
		&entry.ID,
		&entry.SerialNumber,
		&entry.OSVersion,
		&entry.ClientTimestamp,
		&entry.ServerTimestamp,
		&entry.RawData,
		&entry.IPAddress,
		&entry.UserAgent,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Log not found", http.StatusNotFound)
		} else {
			log.Printf("Error querying database: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"log":    entry,
	})
}

// 健康检查
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format("2006-01-02 15:04:05"),
	})
}

func main() {
	// 初始化数据库
	initDB()
	defer db.Close()

	// 设置路由
	http.HandleFunc("/log", handleLog)
	http.HandleFunc("/query", handleQuery)
	http.HandleFunc("/detail", handleDetail)
	http.HandleFunc("/health", handleHealth)

	// 静态文件服务（可选）
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>MDM Log Server</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { margin: 20px 0; padding: 10px; border-left: 4px solid #007cba; }
    </style>
</head>
<body>
    <h1>MDM Log Collection Server</h1>
    <h2>Available Endpoints:</h2>
    
    <div class="endpoint">
        <h3>POST /log</h3>
        <p>Submit system information logs</p>
        <p><strong>Auth:</strong> Bearer token required</p>
    </div>
    
    <div class="endpoint">
        <h3>GET /query</h3>
        <p>Query logs</p>
        <p><strong>Parameters:</strong> serial (optional), limit (optional, max 1000)</p>
    </div>
    
    <div class="endpoint">
        <h3>GET /detail</h3>
        <p>Get detailed log information</p>
        <p><strong>Parameters:</strong> id (required)</p>
    </div>
    
    <div class="endpoint">
        <h3>GET /health</h3>
        <p>Health check endpoint</p>
    </div>
    
    <h2>Usage Examples:</h2>
    <pre>
# Query all logs (latest 100)
curl "http://localhost:8080/query"

# Query logs for specific serial number
curl "http://localhost:8080/query?serial=ABC123&limit=50"

# Get detailed log
curl "http://localhost:8080/detail?id=1"

# Health check
curl "http://localhost:8080/health"
    </pre>
</body>
</html>
			`)
		} else {
			http.NotFound(w, r)
		}
	})

	// 启动服务器
	port := ":8080"
	log.Printf("MDM Log Server starting on port %s", port)
	log.Printf("Database connection established")
	
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
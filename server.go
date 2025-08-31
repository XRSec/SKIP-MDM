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
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

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

var db *sql.DB

func initDB() {
	var err error
	dsn := "root:password@tcp(localhost:3306)/mdm_logs?charset=utf8mb4&parseTime=True&loc=Local"
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	createTable := `
	CREATE TABLE IF NOT EXISTS mdm_logs (
		id INT AUTO_INCREMENT PRIMARY KEY,
		serial_number VARCHAR(255) NOT NULL,
		os_version VARCHAR(100),
		timestamp VARCHAR(50),
		raw_data LONGTEXT,
		ip_address VARCHAR(45),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		INDEX idx_serial (serial_number),
		INDEX idx_created (created_at)
	) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
	`
	
	if _, err = db.Exec(createTable); err != nil {
		log.Fatal("Create table failed:", err)
	}
}

func validateAuth(authHeader, serialNumber string) bool {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	
	token := strings.TrimPrefix(authHeader, "Bearer ")
	now := time.Now()
	
	for i := -5; i <= 5; i++ {
		testTime := now.Add(time.Duration(i) * time.Minute)
		hash := sha256.New()
		timeStr := testTime.Format("200601021504")
		data := "mdm_log_auth" + serialNumber + timeStr + "m'd'm"
		hash.Write([]byte(data))
		expectedToken := hex.EncodeToString(hash.Sum(nil))[:32]
		
		if token == expectedToken {
			return true
		}
	}
	return false
}

func handleLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var info SystemInfo
	if err := json.Unmarshal(body, &info); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !validateAuth(r.Header.Get("Authorization"), info.SerialNumber) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	clientIP := r.RemoteAddr
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		clientIP = realIP
	}

	_, err = db.Exec(`
		INSERT INTO mdm_logs (serial_number, os_version, timestamp, raw_data, ip_address) 
		VALUES (?, ?, ?, ?, ?)`,
		info.SerialNumber, info.OSVersion, info.Timestamp, string(body), clientIP)

	if err != nil {
		log.Printf("Database insert failed: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("Log received: %s from %s", info.SerialNumber, clientIP)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func handleQuery(w http.ResponseWriter, r *http.Request) {
	serial := r.URL.Query().Get("serial")
	var rows *sql.Rows
	var err error
	
	if serial != "" {
		rows, err = db.Query("SELECT serial_number, os_version, timestamp, ip_address FROM mdm_logs WHERE serial_number = ? ORDER BY created_at DESC LIMIT 100", serial)
	} else {
		rows, err = db.Query("SELECT serial_number, os_version, timestamp, ip_address FROM mdm_logs ORDER BY created_at DESC LIMIT 100")
	}
	
	if err != nil {
		http.Error(w, "Query failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []map[string]string
	for rows.Next() {
		var sn, version, timestamp, ip string
		rows.Scan(&sn, &version, &timestamp, &ip)
		logs = append(logs, map[string]string{
			"serial_number": sn,
			"os_version":    version,
			"timestamp":     timestamp,
			"ip_address":    ip,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func main() {
	initDB()
	defer db.Close()

	http.HandleFunc("/log", handleLog)
	http.HandleFunc("/query", handleQuery)
	
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
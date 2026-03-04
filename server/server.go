//go:build !sync

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	//"github.com/robfig/cron/v3"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	defaultIPLocationCooldown = 5 * time.Minute
	ipAPIRequestTimeout       = 15 * time.Second
)

var (
	err   error
	db    *gorm.DB // 数据库1：存授权信息（MDM授权和DFU已授权信息）
	debug = "true"
	//go:embed zoneinfo
	zoneinfo []byte
	location *time.Location

	serverIP = "mdm.xrsec.fun"
	//serverIP   = "192.168.58.247"
	serverPort = "9000" // 9000 | 6
	secureKey  = "qXN4C6ACpwcz94R2"

	htmlPath              = "html" // html | ../html
	shellPath             = "html"
	logRetentionDays      = -2             // 日志保留天数（负数表示往前推）
	obfuscatePath         = "/tmp"         // /tmp | ../html
	logPath               = "/tmp/app.log" // /tmp/app.log logs/app.log
	mysqlDSN              = "mdms_db:a29bab90b26002a2@tcp(mysql.sqlpub.com:3306)/mdms_db?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai"
	ipApiBatchUrl         = "http://ip-api.com/batch?fields=status,country,regionName,city,query&lang=zh-CN"
	sqliteDSN             = "/tmp/server.db?_loc=Asia%2FShanghai"
	ConfigurationProfiles = []byte{0x12, 0x0D, 0x40, 0x4F, 0x16, 0x01, 0x12, 0x4F, 0x04, 0x02, 0x4F, 0x23, 0x0F, 0x0E, 0x06, 0x09, 0x07, 0x15, 0x12, 0x01, 0x14, 0x09, 0x0F, 0x0E, 0x30, 0x12, 0x0F, 0x06, 0x09, 0x0C, 0x05, 0x13, 0x4F, 0x4A}

	// IP 位置更新控制
	ipLocationUpdate struct {
		lastUpdate time.Time     // 上次触发更新的时间
		cooldown   time.Duration // 冷却时间
		mu         sync.Mutex
	}

	// IP API 速率限制控制（基于 ip-api.com 响应头）
	ipApiRateLimit struct {
		remaining int       // X-Rl 剩余请求数
		resetTime time.Time // X-Ttl 重置时间
		banned    bool      // 是否被临时封禁（429）
		mu        sync.Mutex
	}

	regexPatterns = map[string]*regexp.Regexp{
		"sn":              regexp.MustCompile(`^(\w|\d){8,14}$`),
		"ps":              regexp.MustCompile(`^(\w|\d){16}$`),
		"card_id":         regexp.MustCompile(`^(\w|\d){5,10}$`),
		"password":        regexp.MustCompile(`^(\w|\d){15}$`),
		"serial":          regexp.MustCompile(`^[A-Za-z0-9]{8,14}$`),
		"email":           regexp.MustCompile(`^[\w.-]+@[\w.-]+\.[a-zA-Z]{2,}$`),
		"phone":           regexp.MustCompile(`^\d{10,11}$`),
		"has_digit":       regexp.MustCompile(`\d`),
		"uuid":            regexp.MustCompile(`^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$`),
		"modelIdentifier": regexp.MustCompile(`^[a-zA-Z0-9,]{1,20}$`),
	}

	ipLookupHTTPClient = &http.Client{Timeout: ipAPIRequestTimeout}
)

type (
	DFUDevices struct {
		gorm.Model             // TODO 更新数据库结构，先把数据导出到 Sqlite 再导入回 Mysql
		SerialNumber    string `gorm:"column:serial_number;type:varchar(255);uniqueIndex"` // db1 表：已授权的 DFU 设备信息
		HardwareUUID    string `gorm:"column:hardware_uuid;type:varchar(255);index"`
		ModelIdentifier string `gorm:"column:model_identifier;type:varchar(255)"`
		IPAddress       string `gorm:"column:ip_address;type:varchar(255)"`
	}
	DFUAuthLogs struct {
		ID              uint      `gorm:"primaryKey" json:"id"`
		SerialNumber    string    `gorm:"column:serial_number;type:varchar(255);index" json:"serial_number"` // db 表：DFU 授权请求记录（允许重复）
		HardwareUUID    string    `gorm:"column:hardware_uuid;type:varchar(255);index" json:"hardware_uuid"`
		ModelIdentifier string    `gorm:"column:model_identifier;type:varchar(255)" json:"model_identifier"`
		IPAddress       string    `gorm:"column:ip_address;type:varchar(255)" json:"ip_address"`
		Status          int8      `gorm:"column:status;type:integer;default:0" json:"status"` // 0=未授权，1=已授权
		Location        string    `gorm:"column:location;type:text" json:"location"`          // IP 位置信息
		AuthCreatedAt   time.Time `gorm:"column:auth_created_at" json:"auth_created_at"`
		AuthUpdatedAt   time.Time `gorm:"column:auth_updated_at" json:"auth_updated_at"`
	}
	IPApiResponse struct {
		Status     string `json:"status"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
		Query      string `json:"query"`
		Message    string `json:"message,omitempty"`
	}
	logWriter struct{}
	// ExportedLogs 日志导出结构
	ExportedLogs struct {
		ExportTime   string        `json:"export_time"`
		ExportPeriod string        `json:"export_period"`
		MDMLogs      []MDMAuthLog  `json:"mdm_logs"`
		DFULogs      []DFUAuthLogs `json:"dfu_logs"`
		TotalMDM     int           `json:"total_mdm"`
		TotalDFU     int           `json:"total_dfu"`
	}

	Users struct {
		gorm.Model
		SerialNumber string `gorm:"column:serial_number;type:varchar(255);uniqueIndex"`
		IPAddress    string `gorm:"column:ip_address;type:varchar(255)"`
		Rule         int8   `gorm:"column:rule;type:integer;default:0"`
	}

	MDMAuthLog struct {
		ID            uint      `gorm:"primaryKey" json:"id"`
		SerialNumber  string    `gorm:"column:serial_number;type:varchar(255);index" json:"serial_number"`
		IPAddress     string    `gorm:"column:ip_address;type:varchar(255)" json:"ip_address"`
		Status        int8      `gorm:"column:status;type:integer;default:0;index" json:"status"`
		Location      string    `gorm:"column:location;type:text" json:"location"`
		AuthCreatedAt time.Time `gorm:"column:auth_created_at" json:"auth_created_at"`
		AuthUpdatedAt time.Time `gorm:"column:auth_updated_at" json:"auth_updated_at"`
	}
	BatchAuthItem struct {
		SN   string `json:"sn"`
		Rule int8   `json:"rule"` // 0=删除, 1=临时(MDM)/授权(DFU), 2=永久(仅MDM)
	}
	BatchAuthRequest struct {
		SecureKey string          `json:"ps" binding:"required,len=16"`
		MDM       []BatchAuthItem `json:"mdm"`
		DFU       []BatchAuthItem `json:"dfu"`
	}
	BatchResultItem struct {
		SN      string `json:"sn"`
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
	}
	AuthItem interface {
		GetSN() string
		GetRule() int8
	}
	AuthRequest struct {
		SerialNumber string `json:"serial_number" gorm:"column:serial_number;size:20;index"`
	}
	// SystemInfo 接收日志收集的 JSON 数据结构（与客户端保持一致）
	SystemInfo struct {
		AuthRequest   `gorm:"embedded"`
		OSVersion     string    `json:"os_version" gorm:"column:os_version;size:50"`
		OsType        bool      `json:"os_type" gorm:"column:os_type"`            // true: 桌面模式, false: 恢复模式
		Timestamp     time.Time `json:"timestamp" gorm:"column:client_timestamp"` // 客户端时间戳
		Volumes       []string  `json:"volumes" gorm:"column:volumes;type:text;serializer:json"`
		LaunchAgents  []string  `json:"launch_agents" gorm:"column:launch_agents;type:text;serializer:json"`
		LaunchDaemons []string  `json:"launch_daemons" gorm:"column:launch_daemons;type:text;serializer:json"`
		AppSupport    []string  `json:"app_support" gorm:"column:app_support;type:text;serializer:json"`
		UserPrefs     []string  `json:"user_prefs" gorm:"column:user_prefs;type:text;serializer:json"`
		SysPrefs      []string  `json:"sys_prefs" gorm:"column:sys_prefs;type:text;serializer:json"`
		Applications  []string  `json:"applications" gorm:"column:applications;type:text;serializer:json"`
		MDMSettings   []string  `json:"mdm_settings" gorm:"column:mdm_settings;type:text;serializer:json"`
		CloudConfig   string    `json:"cloud_config" gorm:"column:cloud_config;type:text"`
		MDMDomains    string    `json:"mdm_domains" gorm:"column:mdm_domains;type:text"`
		Users         []string  `json:"users" gorm:"column:users;type:text;serializer:json"`
		ProcessList   []string  `json:"process_list" gorm:"column:process_list;type:longtext;serializer:json"`
	}
	ClientLogs struct {
		ID        uint       `gorm:"primarykey" json:"id"`
		Timestamp time.Time  `gorm:"column:created_timestamp" json:"timestamp"` // 服务端记录时间
		Logs      SystemInfo `gorm:"embedded" json:"logs"`                      // 嵌入 SystemInfo 结构
		IP        string     `json:"ip"`
	}
)

func (b BatchAuthItem) GetSN() string { return b.SN }
func (b BatchAuthItem) GetRule() int8 { return b.Rule }

func (m *MDMAuthLog) GetID() uint            { return m.ID }
func (m *MDMAuthLog) GetIPAddress() string   { return m.IPAddress }
func (m *MDMAuthLog) GetLocation() string    { return m.Location }
func (m *MDMAuthLog) SetLocation(loc string) { m.Location = loc }

func (d *DFUAuthLogs) GetID() uint            { return d.ID }
func (d *DFUAuthLogs) GetIPAddress() string   { return d.IPAddress }
func (d *DFUAuthLogs) GetLocation() string    { return d.Location }
func (d *DFUAuthLogs) SetLocation(loc string) { d.Location = loc }

// BeforeSave Hook: 统一将序列号转为小写，避免大小写混写带来的查询/唯一键问题。
func (u *Users) BeforeSave(*gorm.DB) error {
	u.SerialNumber = strings.ToLower(u.SerialNumber)
	return nil
}

func (d *DFUDevices) BeforeSave(*gorm.DB) error {
	d.SerialNumber = strings.ToLower(d.SerialNumber)
	return nil
}

func (m *MDMAuthLog) BeforeSave(*gorm.DB) error {
	m.SerialNumber = strings.ToLower(m.SerialNumber)
	return nil
}

func (d *DFUAuthLogs) BeforeSave(*gorm.DB) error {
	d.SerialNumber = strings.ToLower(d.SerialNumber)
	return nil
}

func validateItems[T AuthItem](items []T, maxRule int8) error {
	seen := make(map[string]bool)
	for _, item := range items {
		sn := strings.ToLower(strings.TrimSpace(item.GetSN()))
		if !validateField("sn", sn) {
			return fmt.Errorf("序列号格式错误: %s", sn)
		}
		if item.GetRule() < 0 || item.GetRule() > maxRule {
			return fmt.Errorf("rule 参数错误: %s", sn)
		}
		if seen[sn] {
			return fmt.Errorf("序列号重复: %s", sn)
		}
		seen[sn] = true
	}
	return nil
}

func init() {
	location, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location, err = time.LoadLocationFromTZData("Asia/Shanghai", zoneinfo)
		if err != nil {
			log.Errorln("时区设置失败", err)
			// 使用 UTC 作为后备方案
			location = time.UTC
		}
	}
	time.Local = location
	ipLocationUpdate.cooldown = defaultIPLocationCooldown

	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// 数据库1：存授权信息（MDM授权和DFU已授权信息）
	db, err = gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{
		//Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Errorf("数据库初始化失败: %v", err)
	}
	log.Infoln("使用 MySQL 数据库1（授权信息）")

	// 配置连接池，避免默认连接参数在高并发下成为瓶颈。
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("获取数据库连接失败: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if err = db.AutoMigrate(&Users{}, &DFUDevices{}, &MDMAuthLog{}, &DFUAuthLogs{}); err != nil {
		log.Fatalf("数据库迁移失败: %v，程序退出", err)
	}
}

func validateField(fieldName, value string) bool {
	if value == "" {
		return false
	}
	pattern, exists := regexPatterns[fieldName]
	if !exists {
		return false
	}
	return pattern.MatchString(value)
}
func getTimeGap(createdAt time.Time) bool {
	return time.Since(createdAt) <= 24*time.Hour
}

func checkAuch(c *gin.Context) (msg string, users Users, status bool) {
	serialNumber := strings.ToLower(c.Query("sn"))
	isAdminQuery := false
	if referer := c.GetHeader("Referer"); referer != "" && strings.Contains(referer, "/4RWmh") && strings.Contains(referer, "ps="+secureKey) {
		isAdminQuery = true
	}
	var err error
	defer func() {
		// 参数错误不记录日志（因为可能序列号都没有）
		if msg == "参数错误" {
			log.Errorln(msg, serialNumber)
			return
		}
		// 其他情况记录错误日志
		if err != nil || msg != "" || !status {
			log.Errorln(msg, serialNumber, err)
		}
		if serialNumber != "" {
			go recordMDMAuthLog(serialNumber, c.ClientIP(), isAdminQuery)
		}
	}()
	if !validateField("sn", serialNumber) {
		msg = "参数错误"
		return msg, users, false
	}
	// 查询用户信息（数据库1：MDM授权）
	if err = db.Where("LOWER(serial_number) = ?", serialNumber).First(&users).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			msg = "用户未授权"
		} else {
			msg = "数据库查询失败"
		}
		return msg, users, false
	}
	if users.Rule == 1 && !getTimeGap(users.CreatedAt) {
		msg = "临时用户 授权超时"
		return msg, users, false
	}
	if users.Rule == 0 {
		msg = "用户未授权"
		return msg, users, false
	}
	return msg, users, true
}
func isCurl(c *gin.Context) bool {
	return strings.Contains(strings.ToLower(c.GetHeader("User-Agent")), "curl")
}

func isWeb(c *gin.Context) bool {
	tmpHeader := strings.ToLower(c.GetHeader("User-Agent"))
	shortcutAgentStatus := strings.Contains(tmpHeader, "shortcut")
	android := strings.Contains(tmpHeader, "mobile") && strings.Contains(tmpHeader, "safari")
	linux := strings.Contains(tmpHeader, "linux") && strings.Contains(tmpHeader, "safari")
	mac := strings.Contains(tmpHeader, "mac") && strings.Contains(tmpHeader, "safari")
	//iPhone := strings.Contains(tmpHeader, "iphone") && strings.Contains(tmpHeader, "mobile")
	if shortcutAgentStatus || android || mac || linux {
		return true
	}
	return false
}

// getRealHost 获取真实的主机名，优先从 X-Forwarded-Host 读取（用于 nginx 反向代理场景）
func getRealHost(c *gin.Context) string {
	// 优先从 X-Forwarded-Host 读取（nginx 反向代理会设置此头部）
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		return strings.TrimSpace(strings.Split(forwardedHost, ",")[0])
	}
	// 尝试从 X-Host 读取（自定义头部）
	if xHost := c.GetHeader("X-Host"); xHost != "" {
		return strings.TrimSpace(strings.Split(xHost, ",")[0])
	}
	if CDNHeader := c.GetHeader("Tencent-Acceleration-Domain-Name"); CDNHeader != "" {
		return strings.TrimSpace(strings.Split(CDNHeader, ",")[0])
	}
	// 使用原始 Host（c.GetHeader("Host") 和 c.Request.Host 通常相同）
	return c.Request.Host
}

func decodeString(data []byte, key byte) string {
	decoded := make([]byte, len(data))
	for i, b := range data {
		decoded[i] = b ^ key
	}
	return string(decoded)
}

func roundTimeToNextQuarter(t time.Time) time.Time {
	base := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	quarterMinute := ((t.Minute() + 15) / 15) * 15
	return base.Add(time.Duration(quarterMinute) * time.Minute)
}

func encodeHash(sn string) string {
	hash := sha256.New()
	now := time.Now()
	roundedTime := roundTimeToNextQuarter(now).Format("200601021504")
	pv := decodeString(ConfigurationProfiles, 96)
	data := pv + strings.ToLower(sn) + roundedTime + pv
	hash.Write([]byte(data))
	hashValue := hash.Sum(nil)
	filePaths := hex.EncodeToString(hashValue)
	front := filePaths[:8]
	end := filePaths[len(filePaths)-8:]
	return front + end
}

func decodeHash(sn, ps string) bool {
	if ps == secureKey {
		return true
	}
	ps1 := encodeHash(sn)
	if strings.EqualFold(ps, ps1) {
		return true
	}
	return false
}
func encodeHashDFU(serialNumber, modelIdentifier, hardwareUUID string) string {
	currentTime := time.Now().In(location).Format("2006-01-02-15")

	kv := decodeString(ConfigurationProfiles, 96)
	combinedString := serialNumber + modelIdentifier + hardwareUUID + kv + currentTime

	hash := sha256.New()
	hash.Write([]byte(combinedString))
	hashValue := hash.Sum(nil)
	hashHex := hex.EncodeToString(hashValue)

	prefix := hashHex[:4]
	suffix := hashHex[len(hashHex)-8:]

	return strings.ToLower(prefix + suffix)
}

// isPrivateIP 检查IP地址是否是私有IP地址
func isPrivateIP(ip string) bool {
	cleanIP := strings.TrimSpace(ip)
	if cleanIP == "" {
		return true
	}

	if host, _, err := net.SplitHostPort(cleanIP); err == nil {
		cleanIP = host
	}
	cleanIP = strings.Trim(cleanIP, "[]")
	cleanIP, _, _ = strings.Cut(cleanIP, "%")

	if addr, err := netip.ParseAddr(cleanIP); err == nil {
		return addr.IsLoopback() ||
			addr.IsPrivate() ||
			addr.IsLinkLocalUnicast() ||
			addr.IsLinkLocalMulticast() ||
			addr.IsUnspecified()
	}

	// 解析失败时保守回退到常见私网前缀判断
	if strings.HasPrefix(cleanIP, "127.") ||
		strings.HasPrefix(cleanIP, "10.") ||
		strings.HasPrefix(cleanIP, "192.168.") ||
		cleanIP == "::1" {
		return true
	}
	if strings.HasPrefix(cleanIP, "172.") {
		parts := strings.Split(cleanIP, ".")
		if len(parts) >= 2 {
			if second, convErr := strconv.Atoi(parts[1]); convErr == nil && second >= 16 && second <= 31 {
				return true
			}
		}
	}
	return false
}

func batchGetLocationFromIP(ipAddresses []string) map[string]string {
	result := make(map[string]string)

	validIPs := make([]string, 0)
	ipMap := make(map[string]bool)
	for _, ip := range ipAddresses {
		if isPrivateIP(ip) {
			result[ip] = "本地"
			continue
		}
		if !ipMap[ip] {
			validIPs = append(validIPs, ip)
			ipMap[ip] = true
		}
	}

	if len(validIPs) == 0 {
		return result
	}

	ipApiRateLimit.mu.Lock()
	now := time.Now()
	if ipApiRateLimit.banned && now.Before(ipApiRateLimit.resetTime) {
		ipApiRateLimit.mu.Unlock()
		log.Warnf("IP API 被临时封禁，等待至 %s", ipApiRateLimit.resetTime.Format("15:04:05"))
		return result
	}
	if ipApiRateLimit.remaining <= 0 && now.Before(ipApiRateLimit.resetTime) {
		ipApiRateLimit.mu.Unlock()
		log.Warnf("IP API 速率限制，等待至 %s", ipApiRateLimit.resetTime.Format("15:04:05"))
		return result
	}
	// 重置封禁状态
	if ipApiRateLimit.banned && now.After(ipApiRateLimit.resetTime) {
		ipApiRateLimit.banned = false
	}
	ipApiRateLimit.mu.Unlock()

	// 构建批量请求（直接使用 IP 数组）
	jsonData, err := json.Marshal(validIPs)
	if err != nil {
		log.Warnln("构建IP查询请求失败", err)
		return result
	}

	req, err := http.NewRequest("POST", ipApiBatchUrl, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Warnln("创建IP查询请求失败", err)
		return result
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := ipLookupHTTPClient.Do(req)
	if err != nil {
		log.Warnln("IP查询请求失败", err)
		return result
	}
	defer resp.Body.Close()

	// 检查速率限制响应头
	ipApiRateLimit.mu.Lock()
	if xRl := resp.Header.Get("X-Rl"); xRl != "" {
		if remaining, err := strconv.Atoi(xRl); err == nil {
			ipApiRateLimit.remaining = remaining
		}
	}
	if xTtl := resp.Header.Get("X-Ttl"); xTtl != "" {
		if ttl, err := strconv.Atoi(xTtl); err == nil {
			ipApiRateLimit.resetTime = time.Now().Add(time.Duration(ttl) * time.Second)
		}
	}

	// 处理 429 响应，标记为封禁状态
	if resp.StatusCode == http.StatusTooManyRequests {
		ipApiRateLimit.banned = true
		// 如果没有 X-Ttl，默认封禁 1 小时
		if ipApiRateLimit.resetTime.Before(time.Now()) {
			ipApiRateLimit.resetTime = time.Now().Add(time.Hour)
		}
		ipApiRateLimit.mu.Unlock()
		log.Warnln("IP API 请求过于频繁，被临时封禁")
		return result
	}
	ipApiRateLimit.mu.Unlock()

	// 处理其他 HTTP 错误状态码
	if resp.StatusCode != http.StatusOK {
		log.Warnf("IP API 请求失败，状态码: %d", resp.StatusCode)
		return result
	}

	// 解析响应
	var responses []IPApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&responses); err != nil {
		log.Warnln("解析IP查询响应失败", err)
		return result
	}

	// 处理响应结果（使用 resp.Query 字段来匹配IP地址，而不是依赖索引顺序）
	for _, resp := range responses {
		ip := resp.Query
		if ip == "" {
			continue
		}
		// 验证这个IP是否在我们请求的列表中
		if !ipMap[ip] {
			continue
		}
		if resp.Status == "success" {
			location := ""
			if resp.Country != "" {
				location = resp.Country
			}
			if resp.RegionName != "" {
				if location != "" {
					location += " " + resp.RegionName
				} else {
					location = resp.RegionName
				}
			}
			if resp.City != "" && resp.City != resp.RegionName {
				if location != "" {
					location += " " + resp.City
				} else {
					location = resp.City
				}
			}
			// 只有当 location 不为空时才设置，否则保持为空字符串（下次可以再查询）
			if location != "" {
				result[ip] = location
			}
		}
	}

	// 为没有返回结果的IP设置空字符串（下次可以再查询）
	for _, ip := range validIPs {
		if _, exists := result[ip]; !exists {
			result[ip] = ""
		}
	}

	return result
}

// checkLogNeedUpdate 检查日志是否需要更新
func checkLogNeedUpdate(currentStatus, newStatus int8, currentCreatedAt, newCreatedAt, currentUpdatedAt, newUpdatedAt time.Time) bool {
	if currentStatus != newStatus {
		return true
	}
	if !currentCreatedAt.Equal(newCreatedAt) {
		return true
	}
	if !currentUpdatedAt.Equal(newUpdatedAt) {
		return true
	}
	return false
}

func recordMDMAuthLog(serialNumber, ipAddress string, isAdminQuery bool) {
	if serialNumber == "" {
		return
	}
	// 管理员查询时不记录日志
	if isAdminQuery {
		return
	}
	// Status 直接存储 rule 值：0=未授权，1=临时，2=永久
	if err := db.Create(&MDMAuthLog{
		SerialNumber:  strings.ToLower(serialNumber),
		IPAddress:     ipAddress,
		Status:        0,
		AuthCreatedAt: time.Now(),
		AuthUpdatedAt: time.Now(),
	}).Error; err != nil {
		log.Errorln("记录 MDM 认证日志失败", err)
	}
}

func recordDFUAuthLog(serialNumber, ipAddress, hardwareUUID, modelIdentifier string, status int8, isAdminQuery bool) {
	if serialNumber == "" {
		return
	}
	// 管理员查询时不记录日志
	if isAdminQuery {
		return
	}

	// 记录到数据库2（DFU授权日志）
	// AuthCreatedAt 和 AuthUpdatedAt 会在 /1aRLn 接口中更新
	if err := db.Create(&DFUAuthLogs{
		SerialNumber:    strings.ToLower(serialNumber),
		HardwareUUID:    hardwareUUID,
		ModelIdentifier: modelIdentifier,
		IPAddress:       ipAddress,
		Status:          status, // 0=未授权，1=已授权
		AuthCreatedAt:   time.Now(),
		AuthUpdatedAt:   time.Now(),
	}).Error; err != nil {
		log.Errorln("记录 DFU 授权日志失败", err)
	}
}

// ProcessLogsResult 处理日志的结果
type ProcessLogsResult struct {
	AllMDMLogs      []MDMAuthLog
	AllDFULogs      []DFUAuthLogs
	LatestMDMLogs   map[string]*MDMAuthLog
	LatestDFULogs   map[string]*DFUAuthLogs
	MDMSnCount      map[string]int
	DFUSnCount      map[string]int
	MDMLogsToUpdate []MDMAuthLog
	DFULogsToUpdate []DFUAuthLogs
	MDMUserMap      map[string]Users
	DFUDeviceMap    map[string]DFUDevices
}

// 查询指定时间范围内的日志（泛型版本）
func queryLogsByTimeRange[T any](startTime time.Time) ([]T, error) {
	var logs []T
	if err := db.Where("auth_updated_at >= ?", startTime).
		Order("auth_updated_at DESC").
		Find(&logs).Error; err != nil {
		return nil, fmt.Errorf("查询日志失败: %w", err)
	}
	return logs, nil
}

// 批量查询授权状态
func queryAuthStatus(mdmSns, dfuSns []string) (map[string]Users, map[string]DFUDevices) {
	mdmUserMap := make(map[string]Users)
	if len(mdmSns) > 0 {
		var users []Users
		if err := db.Where("LOWER(serial_number) IN ?", mdmSns).Find(&users).Error; err == nil {
			for _, user := range users {
				mdmUserMap[strings.ToLower(user.SerialNumber)] = user
			}
		}
	}

	dfuDeviceMap := make(map[string]DFUDevices)
	if len(dfuSns) > 0 {
		var devices []DFUDevices
		if err := db.Where("LOWER(serial_number) IN ?", dfuSns).Find(&devices).Error; err == nil {
			for _, device := range devices {
				dfuDeviceMap[strings.ToLower(device.SerialNumber)] = device
			}
		}
	}

	return mdmUserMap, dfuDeviceMap
}

// LogEntry 日志条目接口，用于统一处理 MDM 和 DFU 日志
type LogEntry interface {
	GetID() uint
	GetIPAddress() string
	GetLocation() string
	SetLocation(string)
}

func extractUniqueSNs(allMDMLogs []MDMAuthLog, allDFULogs []DFUAuthLogs) ([]string, []string) {
	mdmSnSet := make(map[string]bool)
	dfuSnSet := make(map[string]bool)
	for _, log6 := range allMDMLogs {
		mdmSnSet[strings.ToLower(log6.SerialNumber)] = true
	}
	for _, log7 := range allDFULogs {
		dfuSnSet[strings.ToLower(log7.SerialNumber)] = true
	}

	mdmSns := make([]string, 0, len(mdmSnSet))
	dfuSns := make([]string, 0, len(dfuSnSet))
	for sn := range mdmSnSet {
		mdmSns = append(mdmSns, sn)
	}
	for sn := range dfuSnSet {
		dfuSns = append(dfuSns, sn)
	}
	return mdmSns, dfuSns
}

func processMDMLogs(logs []MDMAuthLog, userMap map[string]Users, snCount map[string]int, latestLogs map[string]*MDMAuthLog) []MDMAuthLog {
	logsToUpdate := make([]MDMAuthLog, 0)

	for i := range logs {
		log6 := &logs[i]
		sn := strings.ToLower(log6.SerialNumber)
		snCount[sn]++

		if existing, exists := latestLogs[sn]; !exists || log6.AuthUpdatedAt.After(existing.AuthUpdatedAt) {
			latestLogs[sn] = log6
		}

		var newStatus int8
		var newCreated, newUpdated time.Time

		if user, exists := userMap[sn]; !exists {
			newStatus = 0
			newCreated, newUpdated = log6.AuthCreatedAt, log6.AuthUpdatedAt
		} else {
			newStatus, newCreated, newUpdated = user.Rule, user.CreatedAt, user.UpdatedAt
			// 临时用户过期判断：Rule=1 且超过24小时，Status 设为 0
			if user.Rule == 1 && !getTimeGap(user.CreatedAt) {
				newStatus = 0
			}
		}

		if checkLogNeedUpdate(log6.Status, newStatus, log6.AuthCreatedAt, newCreated, log6.AuthUpdatedAt, newUpdated) {
			log6.Status, log6.AuthCreatedAt, log6.AuthUpdatedAt = newStatus, newCreated, newUpdated
			logsToUpdate = append(logsToUpdate, *log6)
		}
	}
	return logsToUpdate
}

func processDFULogs(logs []DFUAuthLogs, deviceMap map[string]DFUDevices, snCount map[string]int, latestLogs map[string]*DFUAuthLogs) []DFUAuthLogs {
	logsToUpdate := make([]DFUAuthLogs, 0)

	for i := range logs {
		log7 := &logs[i]
		sn := strings.ToLower(log7.SerialNumber)
		snCount[sn]++

		if existing, exists := latestLogs[sn]; !exists || log7.AuthUpdatedAt.After(existing.AuthUpdatedAt) {
			latestLogs[sn] = log7
		}

		var newStatus int8
		var newCreated, newUpdated time.Time

		if device, exists := deviceMap[sn]; !exists {
			newStatus = 0
			newCreated, newUpdated = log7.AuthCreatedAt, log7.AuthUpdatedAt
		} else {
			newStatus, newCreated, newUpdated = 1, device.CreatedAt, device.UpdatedAt
			// 补充硬件信息
			if log7.HardwareUUID == "" && device.HardwareUUID != "" {
				log7.HardwareUUID = device.HardwareUUID
			}
			if log7.ModelIdentifier == "" && device.ModelIdentifier != "" {
				log7.ModelIdentifier = device.ModelIdentifier
			}
		}

		if checkLogNeedUpdate(log7.Status, newStatus, log7.AuthCreatedAt, newCreated, log7.AuthUpdatedAt, newUpdated) {
			log7.Status, log7.AuthCreatedAt, log7.AuthUpdatedAt = newStatus, newCreated, newUpdated
			logsToUpdate = append(logsToUpdate, *log7)
		}
	}
	return logsToUpdate
}

// 收集日志中需要查询位置的 IP 地址（泛型版本）
func collectIPAddresses[T any, P interface {
	*T
	LogEntry
}](logs []T) []string {
	ipSet := make(map[string]bool)
	result := make([]string, 0)
	for i := range logs {
		logPtr := P(&logs[i])
		ip := logPtr.GetIPAddress()
		loc := logPtr.GetLocation()
		if ip != "" && (loc == "" || loc == "本地") && !isPrivateIP(ip) && !ipSet[ip] {
			result = append(result, ip)
			ipSet[ip] = true
		}
	}
	return result
}

// 更新日志位置信息（泛型版本）
func updateLogsLocation[T any, P interface {
	*T
	LogEntry
}](logs []T, logsToUpdate *[]T, locationMap map[string]string) {
	updateIndex := make(map[uint]int, len(*logsToUpdate))
	for i := range *logsToUpdate {
		updatePtr := P(&(*logsToUpdate)[i])
		updateIndex[updatePtr.GetID()] = i
	}

	for i := range logs {
		logPtr := P(&logs[i])
		if loc, exists := locationMap[logPtr.GetIPAddress()]; exists && loc != "" {
			if logPtr.GetLocation() == "" || logPtr.GetLocation() == "本地" {
				logPtr.SetLocation(loc)
				if idx, found := updateIndex[logPtr.GetID()]; found {
					updatePtr := P(&(*logsToUpdate)[idx])
					updatePtr.SetLocation(loc)
				} else {
					*logsToUpdate = append(*logsToUpdate, logs[i])
					updateIndex[logPtr.GetID()] = len(*logsToUpdate) - 1
				}
			}
		}
	}
}

// 处理日志数据：更新状态、统计、过滤最新记录
func processLogsData(allMDMLogs []MDMAuthLog, allDFULogs []DFUAuthLogs) *ProcessLogsResult {
	result := &ProcessLogsResult{
		AllMDMLogs:    allMDMLogs,
		AllDFULogs:    allDFULogs,
		LatestMDMLogs: make(map[string]*MDMAuthLog),
		LatestDFULogs: make(map[string]*DFUAuthLogs),
		MDMSnCount:    make(map[string]int),
		DFUSnCount:    make(map[string]int),
	}

	// 提取唯一序列号并批量查询授权状态
	mdmSns, dfuSns := extractUniqueSNs(allMDMLogs, allDFULogs)
	result.MDMUserMap, result.DFUDeviceMap = queryAuthStatus(mdmSns, dfuSns)

	// 处理日志
	result.MDMLogsToUpdate = processMDMLogs(allMDMLogs, result.MDMUserMap, result.MDMSnCount, result.LatestMDMLogs)
	result.DFULogsToUpdate = processDFULogs(allDFULogs, result.DFUDeviceMap, result.DFUSnCount, result.LatestDFULogs)

	return result
}

// 批量更新数据库
func saveLogsToDatabase(mdmLogsToUpdate []MDMAuthLog, dfuLogsToUpdate []DFUAuthLogs) error {
	if len(mdmLogsToUpdate) == 0 && len(dfuLogsToUpdate) == 0 {
		return nil
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if len(mdmLogsToUpdate) > 0 {
			if err := tx.Save(&mdmLogsToUpdate).Error; err != nil {
				return fmt.Errorf("批量更新 MDM 日志失败: %w", err)
			}
		}
		if len(dfuLogsToUpdate) > 0 {
			if err := tx.Save(&dfuLogsToUpdate).Error; err != nil {
				return fmt.Errorf("批量更新 DFU 日志失败: %w", err)
			}
		}
		return nil
	})
}

// 查询并处理日志数据的公共方法
func fetchAndProcessLogs() (*ProcessLogsResult, error) {
	startTime := time.Now().AddDate(0, 0, logRetentionDays)

	allMDMLogs, err := queryLogsByTimeRange[MDMAuthLog](startTime)
	if err != nil {
		return nil, fmt.Errorf("查询 MDM 日志失败: %w", err)
	}
	allDFULogs, err := queryLogsByTimeRange[DFUAuthLogs](startTime)
	if err != nil {
		return nil, fmt.Errorf("查询 DFU 日志失败: %w", err)
	}

	return processLogsData(allMDMLogs, allDFULogs), nil
}

// 异步更新 IP 位置信息（带冷却时间控制）
func triggerAsyncIPLocationUpdate() {
	ipLocationUpdate.mu.Lock()
	now := time.Now()
	cooldown := ipLocationUpdate.cooldown
	if cooldown <= 0 {
		cooldown = defaultIPLocationCooldown
	}
	if now.Sub(ipLocationUpdate.lastUpdate) < cooldown {
		ipLocationUpdate.mu.Unlock()
		return
	}
	ipLocationUpdate.lastUpdate = now
	ipLocationUpdate.mu.Unlock()

	go func() {
		startTime := time.Now().AddDate(0, 0, logRetentionDays)
		allMDMLogs, err := queryLogsByTimeRange[MDMAuthLog](startTime)
		if err != nil {
			log.Errorln("异步 IP 更新：查询 MDM 日志失败", err)
			return
		}
		allDFULogs, err := queryLogsByTimeRange[DFUAuthLogs](startTime)
		if err != nil {
			log.Errorln("异步 IP 更新：查询 DFU 日志失败", err)
			return
		}

		// 收集需要查询位置的 IP
		ipMDMToQuery := collectIPAddresses[MDMAuthLog, *MDMAuthLog](allMDMLogs)
		ipDFUToQuery := collectIPAddresses[DFUAuthLogs, *DFUAuthLogs](allDFULogs)

		// 合并去重
		ipSet := make(map[string]bool)
		ipAddressesToQuery := make([]string, 0, len(ipMDMToQuery)+len(ipDFUToQuery))
		for _, ip := range ipMDMToQuery {
			if !ipSet[ip] {
				ipAddressesToQuery = append(ipAddressesToQuery, ip)
				ipSet[ip] = true
			}
		}
		for _, ip := range ipDFUToQuery {
			if !ipSet[ip] {
				ipAddressesToQuery = append(ipAddressesToQuery, ip)
				ipSet[ip] = true
			}
		}

		if len(ipAddressesToQuery) == 0 {
			return
		}

		locationMap := batchGetLocationFromIP(ipAddressesToQuery)
		if len(locationMap) == 0 {
			return
		}

		// 更新日志位置
		mdmLogsToUpdate := make([]MDMAuthLog, 0)
		dfuLogsToUpdate := make([]DFUAuthLogs, 0)
		updateLogsLocation[MDMAuthLog](allMDMLogs, &mdmLogsToUpdate, locationMap)
		updateLogsLocation[DFUAuthLogs](allDFULogs, &dfuLogsToUpdate, locationMap)

		// 保存到数据库
		if err := saveLogsToDatabase(mdmLogsToUpdate, dfuLogsToUpdate); err != nil {
			log.Errorln("异步 IP 更新：保存失败", err)
			//} else if len(mdmLogsToUpdate) > 0 || len(dfuLogsToUpdate) > 0 {
			//log.Infof("异步 IP 更新：成功更新 %d 条 MDM 日志和 %d 条 DFU 日志", len(mdmLogsToUpdate), len(dfuLogsToUpdate))
		}
	}()
}

func exportAndCleanOldLogs() {
	now := time.Now().In(location)
	cutoffDate := now.AddDate(0, 0, logRetentionDays)
	cutoffTime := time.Date(cutoffDate.Year(), cutoffDate.Month(), cutoffDate.Day(), 0, 0, 0, 0, location)

	var oldMDMLogs []MDMAuthLog
	if err := db.Where("auth_updated_at < ?", cutoffTime).
		Order("auth_updated_at ASC").
		Find(&oldMDMLogs).Error; err != nil {
		log.Errorln("查询旧 MDM 日志失败", err)
		return
	}

	var oldDFULogs []DFUAuthLogs
	if err := db.Where("auth_updated_at < ?", cutoffTime).
		Order("auth_updated_at ASC").
		Find(&oldDFULogs).Error; err != nil {
		log.Errorln("查询旧 DFU 日志失败", err)
		return
	}

	if len(oldMDMLogs) == 0 && len(oldDFULogs) == 0 {
		log.Infoln("没有需要导出的旧日志")
		return
	}

	log.Infof("找到 %d 条 MDM 日志和 %d 条 DFU 日志需要导出", len(oldMDMLogs), len(oldDFULogs))

	exportData := ExportedLogs{
		ExportTime:   now.Format("2006-01-02 15:04:05"),
		ExportPeriod: fmt.Sprintf("before %s", cutoffTime.Format("2006-01-02 00:00:00")),
		MDMLogs:      oldMDMLogs,
		DFULogs:      oldDFULogs,
		TotalMDM:     len(oldMDMLogs),
		TotalDFU:     len(oldDFULogs),
	}

	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Errorln("创建日志目录失败", err)
		return
	}

	fileName := fmt.Sprintf("logs/visitor_%s.json", now.Format("2006-01-02_15-04-05"))
	jsonData, err := json.MarshalIndent(exportData, "", "  ")
	if err != nil {
		log.Errorln("序列化日志数据失败", err)
		return
	}

	if err := os.WriteFile(fileName, jsonData, 0644); err != nil {
		log.Errorln("写入日志文件失败", err)
		return
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		if len(oldMDMLogs) > 0 {
			if err := tx.Where("auth_updated_at < ?", cutoffTime).Delete(&MDMAuthLog{}).Error; err != nil {
				return fmt.Errorf("删除旧 MDM 日志失败: %w", err)
			}
		}

		if len(oldDFULogs) > 0 {
			if err := tx.Where("auth_updated_at < ?", cutoffTime).Delete(&DFUAuthLogs{}).Error; err != nil {
				return fmt.Errorf("删除旧 DFU 日志失败: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		log.Errorln("清理旧日志失败", err)
		return
	}
}

func (f *logWriter) Format(entry *log.Entry) ([]byte, error) {
	method := strconv.Itoa(entry.Caller.Line)
	logString := fmt.Sprintf("[LOG] %v | %-5v | %-12v | %v\n",
		entry.Time.Format("2006/01/02 - 15:04:05"),
		entry.Level.String(),
		method,
		entry.Message)
	return []byte(logString), nil
}

func replaceServer(defaultPath, filePath, Host string) {
	if _, err := os.Stat(filePath); err != nil {
		// 读取文件内容
		content, err := os.ReadFile(defaultPath)
		if err != nil {
			log.Errorln("脚本读取失败", err)
		}

		// 替换内容
		newContent := strings.Replace(string(content), "服务器地址", Host, -1)

		// 写入新内容
		err = os.WriteFile(filePath+"_tmp", []byte(newContent), 0666)
		if err != nil {
			log.Errorln("脚本保存失败", err)
		}
		_, err = exec.Command("./bash-obfuscate", filePath+"_tmp", "-o", filePath).Output()
		if err != nil {
			log.Errorln("脚本加密失败", err)
		}
		if err := os.Remove(filePath + "_tmp"); err != nil {
			log.Errorln("移动脚本失败", err)
		}
	}
}

func getLogsByFile(query string) (msg string, tmpLogs string, err error) {
	const maxLines = 500      // 最多读取 500 行匹配结果
	const maxMemory = 1 << 20 // 最多占用 1MB 缓冲

	filePath := logPath // 日志文件路径

	file, err := os.Open(filePath)
	if err != nil {
		msg = "日志打开失败:"
		return msg, tmpLogs, err
	}
	defer file.Close()

	var lines []string
	var totalSize int
	scanner := bufio.NewScanner(file)
	// 允许更长日志行，避免默认 token 太小导致扫描失败。
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxMemory)

	for scanner.Scan() {
		line := scanner.Text()
		if query != "" {
			if strings.Contains(strings.ToLower(line), query) {
				if totalSize+len(line) > maxMemory {
					log.Warnf("日志读取达到内存限制，提前结束: %d bytes", maxMemory)
					break
				}
				lines = append(lines, line)
				totalSize += len(line)
			}
		} else {
			if totalSize+len(line) > maxMemory {
				log.Warnf("日志读取达到内存限制，提前结束: %d bytes", maxMemory)
				break
			}
			lines = append(lines, line)
			totalSize += len(line)
		}
		if len(lines) >= maxLines {
			log.Warnf("日志读取达到行数限制，提前结束: %d lines", maxLines)
			break
		}
	}
	if err := scanner.Err(); err != nil {
		msg = "扫描日志失败"
		log.Errorln(msg)
		return msg, tmpLogs, err
	}

	totalLines := len(lines)
	startIndex := 0
	if totalLines > 50 {
		startIndex = totalLines - 50
	}

	var builder strings.Builder
	for i := startIndex; i < totalLines; i++ {
		builder.WriteString(lines[i])
		builder.WriteByte('\n')
	}
	tmpLogs = builder.String()
	return msg, tmpLogs, err
}

func checkAuthorizationHeader(c *gin.Context) (msg string, ps string) {
	ps = strings.TrimSpace(c.GetHeader("ps"))
	if !validateField("ps", ps) {
		return "authh_err", ""
	}
	return "", ps
}

func main() {
	log.Infoln("Run models...")
	// 配置日志输出到文件
	logFile := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    20,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	}
	log.SetFormatter(new(logWriter))
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Output: logFile,
		SkipPaths: []string{
			"/background.png",
			"/favicon.ico",
			"/apple-touch-icon-120x120-precomposed.png",
			"/apple-touch-icon-120x120.png",
			"/apple-touch-icon-precomposed.png",
			"/apple-touch-icon.png",
			"/robots.txt",
			"/ztWWT",
			"/4RWmh",
			"/1aRLn",
			"/d",
		},
	}))
	r.Use(func(c *gin.Context) {
		isWebCheck := isWeb(c)
		isCurlCheck := isCurl(c)

		if !(isWebCheck || isCurlCheck) {
			// log.Warnf("User-Agent 验证失败 - UA: [%v] isWeb: [%v] isCurl: [%v]", userAgent, isWebCheck, isCurlCheck)
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		realHost := getRealHost(c)
		hosts := strings.Split(realHost, ":")
		hostname := hosts[0]

		if debug != "true" {
			isIPAccess := net.ParseIP(hostname) != nil
			isServer := strings.Contains(realHost, serverIP)

			if isIPAccess || !isServer {
				//log.Warnf("Host 验证失败 - realHost: [%v] isServer: [%v] isIPAccess: [%v] X-Forwarded-Host: [%v] Host: [%v] Request.Host: [%v]", realHost, isServer, isIPAccess, c.GetHeader("X-Forwarded-Host"), c.GetHeader("Host"), c.Request.Host)
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		}

		c.Writer.Header().Set("Vary", "Origin")
		proto := "http"
		if c.Request.TLS != nil {
			proto = "https"
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", proto+"://"+hostname)

		c.Next()
	})
	r.Use(func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() == http.StatusServiceUnavailable {
			c.File(htmlPath + "/error.html")
		}
	})
	r.GET("/", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		if isWeb(c) {
			c.File(htmlPath + "/index.html")
		} else if isCurl(c) {
			fileTmp := fmt.Sprintf("%v/cli-%v.sh", obfuscatePath, serverIP)
			replaceServer(shellPath+"/cli.sh", fileTmp, serverIP)
			c.File(fileTmp)
		} else {
			c.AbortWithStatus(http.StatusServiceUnavailable)
		}
		return
	})

	// 静态文件
	{
		staticR := r.Group("/").Use(func(c *gin.Context) {
			c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
			if !isWeb(c) {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		})
		staticR.GET("/robots.txt", func(c *gin.Context) {
			c.String(http.StatusOK, "User-agent: *\nDisallow: /")
			return
		})
		staticR.GET("/background.png", func(c *gin.Context) {
			c.File(htmlPath + "/background.png")
			return
		})
		staticR.GET("/favicon.ico", func(c *gin.Context) {
			c.File(htmlPath + "/favicon.ico")
			return
		})
		icons := []string{"/apple-touch-icon-120x120-precomposed.png", "/apple-touch-icon-120x120.png", "/apple-touch-icon-precomposed.png", "/apple-touch-icon.png"}
		for _, path := range icons {
			staticR.GET(path, func(c *gin.Context) {
				c.File(htmlPath + "/favicon.png")
				return
			})
		}
	}
	r.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
	})
	{
		// 远程调试
		r.GET("/d", func(c *gin.Context) {
			cli := c.Query("c")
			ps := strings.TrimSpace(c.Query("ps"))

			if ps == "" || cli == "" || !validateField("ps", ps) || !decodeHash("", ps) {
				c.JSON(http.StatusBadRequest, gin.H{
					"code": http.StatusBadRequest,
				})
				return
			}

			ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, "/bin/sh", "-c", cli)

			//cmd.Dir = workDir

			output, err := cmd.CombinedOutput()
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					c.String(http.StatusRequestTimeout, fmt.Sprintf("code: %v\nmsg: %v\noutput:%v", http.StatusRequestTimeout, "Execution timeout", string(output)))
					return
				}
				c.String(http.StatusRequestTimeout, fmt.Sprintf("code: %v\nmsg: %v\noutput:%v", http.StatusRequestTimeout, "Command execution failed", string(output)))
				return
			}

			c.String(http.StatusOK, string(output))

			return
		})

		// MDM 查询/验证授权
		r.GET("/gqK1I", func(c *gin.Context) {
			if msg, users, status := checkAuch(c); !status {
				c.JSON(http.StatusBadRequest, gin.H{
					"code":  http.StatusBadRequest,
					"msg":   msg,
					"users": users,
				})
			} else {
				ps := strings.TrimSpace(c.Query("ps"))
				if !validateField("ps", ps) || !decodeHash(users.SerialNumber, ps) {
					c.JSON(http.StatusOK, gin.H{
						"code":  http.StatusOK,
						"users": users,
					})
				} else {
					encodeHashKey := encodeHash(strings.ToLower(users.SerialNumber))
					c.JSON(http.StatusOK, gin.H{
						"code":  http.StatusOK,
						"pass":  encodeHashKey,
						"users": users,
					})
				}
			}
			return
		})
		// DFU 查询/验证授权
		r.GET("/J0Fpf", func(c *gin.Context) {
			serialNumber := strings.ToLower(strings.TrimSpace(c.Query("sn")))
			hardwareUUID := strings.TrimSpace(c.Query("uuid"))
			modelIdentifier := strings.TrimSpace(c.Query("model"))
			clientIP := c.ClientIP()

			// 检查是否为管理员查询
			isAdminQuery := false
			if referer := c.GetHeader("Referer"); referer != "" && strings.Contains(referer, "/4RWmh") && strings.Contains(referer, "ps="+secureKey) {
				isAdminQuery = true
			}

			// 验证序列号（必需参数）
			if !validateField("sn", serialNumber) {
				c.JSON(http.StatusBadRequest, gin.H{
					"code": http.StatusBadRequest,
					"msg":  "参数错误",
				})
				return
			}

			// 验证可选参数，无效则清空
			if !validateField("uuid", hardwareUUID) {
				hardwareUUID = ""
			}
			if !validateField("modelIdentifier", modelIdentifier) {
				modelIdentifier = ""
			}

			// 查询数据库1（DFU授权信息）
			var dfuDevice DFUDevices
			if err := db.Where("LOWER(serial_number) = ?", serialNumber).First(&dfuDevice).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					go recordDFUAuthLog(serialNumber, clientIP, hardwareUUID, modelIdentifier, 0, isAdminQuery)
					c.JSON(http.StatusBadRequest, gin.H{
						"code": http.StatusBadRequest,
						"msg":  "设备未授权",
					})
				} else {
					log.Errorln("查询 DFU 授权失败", err)
					c.JSON(http.StatusBadRequest, gin.H{
						"code": http.StatusBadRequest,
						"msg":  "数据库查询失败",
					})
				}
				return
			}

			if dfuDevice.HardwareUUID == "" && hardwareUUID != "" {
				dfuDevice.HardwareUUID = hardwareUUID
			}
			if dfuDevice.ModelIdentifier == "" && modelIdentifier != "" {
				dfuDevice.ModelIdentifier = modelIdentifier
			}
			if clientIP != "" {
				dfuDevice.IPAddress = clientIP
			}

			if err := db.Save(&dfuDevice).Error; err != nil {
				log.Errorln("更新 DFU 授权信息失败", err)
			}

			// 生成授权密码
			password := encodeHashDFU(serialNumber, modelIdentifier, hardwareUUID)

			// 记录授权日志
			go recordDFUAuthLog(serialNumber, clientIP, hardwareUUID, modelIdentifier, 1, isAdminQuery)

			c.JSON(http.StatusOK, gin.H{
				"code":     http.StatusOK,
				"msg":      "授权成功",
				"pass":     password,
				"dfu_auth": dfuDevice,
			})
		})
		// MDM/DFU 授权/删除（统一接口）
		r.POST("/FSElO", func(c *gin.Context) {
			var req BatchAuthRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": "参数错误"})
				return
			}
			if req.SecureKey != secureKey {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": "密码错误"})
				return
			}
			if len(req.MDM) == 0 && len(req.DFU) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": "mdm 和 dfu 不能同时为空"})
				return
			}

			// 校验 MDM 和 DFU 数据
			if err := validateItems(req.MDM, 2); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
				return
			}
			if err := validateItems(req.DFU, 1); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": http.StatusBadRequest, "msg": err.Error()})
				return
			}

			mdmResults := make([]BatchResultItem, len(req.MDM))
			dfuResults := make([]BatchResultItem, len(req.DFU))

			// 分类操作：删除 vs 创建/更新
			var mdmDelSNs, mdmUpsertSNs, dfuDelSNs, dfuUpsertSNs []string
			mdmIdxMap := make(map[string]int)
			dfuIdxMap := make(map[string]int)
			mdmRuleMap := make(map[string]int8)

			for i, item := range req.MDM {
				sn := strings.ToLower(strings.TrimSpace(item.SN))
				mdmResults[i].SN = sn
				mdmIdxMap[sn] = i
				if item.Rule == 0 {
					mdmDelSNs = append(mdmDelSNs, sn)
				} else {
					mdmUpsertSNs = append(mdmUpsertSNs, sn)
					mdmRuleMap[sn] = item.Rule
				}
			}

			for i, item := range req.DFU {
				sn := strings.ToLower(strings.TrimSpace(item.SN))
				dfuResults[i].SN = sn
				dfuIdxMap[sn] = i
				if item.Rule == 0 {
					dfuDelSNs = append(dfuDelSNs, sn)
				} else {
					dfuUpsertSNs = append(dfuUpsertSNs, sn)
				}
			}

			mdmSuccess, dfuSuccess := 0, 0

			// 并行查询已存在的记录
			var existingUsers []Users
			var existingDFU []DFUDevices
			var wg sync.WaitGroup

			if len(mdmUpsertSNs) > 0 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					db.Where("LOWER(serial_number) IN ?", mdmUpsertSNs).Find(&existingUsers)
				}()
			}
			if len(dfuUpsertSNs) > 0 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					db.Where("LOWER(serial_number) IN ?", dfuUpsertSNs).Find(&existingDFU)
				}()
			}
			wg.Wait()

			// 构建已存在记录的 map
			mdmExistMap := make(map[string]*Users, len(existingUsers))
			for i := range existingUsers {
				mdmExistMap[strings.ToLower(existingUsers[i].SerialNumber)] = &existingUsers[i]
			}
			dfuExistSet := make(map[string]bool, len(existingDFU))
			for _, d := range existingDFU {
				dfuExistSet[strings.ToLower(d.SerialNumber)] = true
			}

			// 使用事务处理所有数据库操作
			txErr := db.Transaction(func(tx *gorm.DB) error {
				// 1. 批量删除 MDM 和 DFU
				for _, del := range []struct {
					sns     []string
					model   any
					results []BatchResultItem
					idxMap  map[string]int
					counter *int
				}{
					{mdmDelSNs, &Users{}, mdmResults, mdmIdxMap, &mdmSuccess},
					{dfuDelSNs, &DFUDevices{}, dfuResults, dfuIdxMap, &dfuSuccess},
				} {
					if len(del.sns) == 0 {
						continue
					}
					if err := tx.Unscoped().Where("LOWER(serial_number) IN ?", del.sns).Delete(del.model).Error; err != nil {
						for _, sn := range del.sns {
							del.results[del.idxMap[sn]].Msg = "删除失败"
						}
						return err
					}
					for _, sn := range del.sns {
						del.results[del.idxMap[sn]].Success, del.results[del.idxMap[sn]].Msg = true, "删除成功"
						*del.counter++
					}
				}

				// 2. 批量处理 MDM 创建/更新
				if len(mdmUpsertSNs) > 0 {
					var toCreate, toUpdate []Users
					for _, sn := range mdmUpsertSNs {
						rule := mdmRuleMap[sn]
						if user, exists := mdmExistMap[sn]; exists {
							if rule == 1 {
								user.CreatedAt = time.Now()
							}
							user.Rule = rule
							toUpdate = append(toUpdate, *user)
							mdmResults[mdmIdxMap[sn]].Msg = "更新成功"
						} else {
							toCreate = append(toCreate, Users{SerialNumber: sn, Rule: rule})
							mdmResults[mdmIdxMap[sn]].Msg = "新建授权"
						}
					}

					if len(toCreate) > 0 {
						if err := tx.Create(&toCreate).Error; err != nil {
							for _, u := range toCreate {
								mdmResults[mdmIdxMap[u.SerialNumber]].Msg = "创建失败"
							}
							return err
						}
						for _, u := range toCreate {
							mdmResults[mdmIdxMap[u.SerialNumber]].Success = true
							mdmSuccess++
						}
					}

					if len(toUpdate) > 0 {
						updateNow := time.Now()
						for _, u := range toUpdate {
							updateData := map[string]any{
								"rule":       u.Rule,
								"updated_at": updateNow,
							}
							// 临时授权续期时显式覆盖 created_at，避免字段被 ORM 忽略。
							if u.Rule == 1 {
								updateData["created_at"] = u.CreatedAt
							}
							if err := tx.Model(&Users{}).Where("id = ?", u.ID).Updates(updateData).Error; err != nil {
								mdmResults[mdmIdxMap[strings.ToLower(u.SerialNumber)]].Msg = "更新失败"
								return err
							}
							mdmResults[mdmIdxMap[strings.ToLower(u.SerialNumber)]].Success = true
							mdmSuccess++
						}
					}
				}

				// 3. 批量处理 DFU 创建
				if len(dfuUpsertSNs) > 0 {
					var toCreate []DFUDevices
					for _, sn := range dfuUpsertSNs {
						if dfuExistSet[sn] {
							dfuResults[dfuIdxMap[sn]].Success, dfuResults[dfuIdxMap[sn]].Msg = true, "已授权"
							dfuSuccess++
						} else {
							toCreate = append(toCreate, DFUDevices{SerialNumber: sn})
						}
					}

					if len(toCreate) > 0 {
						if err := tx.Create(&toCreate).Error; err != nil {
							for _, d := range toCreate {
								dfuResults[dfuIdxMap[d.SerialNumber]].Msg = "创建失败"
							}
							return err
						}
						for _, d := range toCreate {
							dfuResults[dfuIdxMap[d.SerialNumber]].Success, dfuResults[dfuIdxMap[d.SerialNumber]].Msg = true, "授权成功"
							dfuSuccess++
						}
					}
				}

				return nil
			})

			if txErr != nil {
				log.Errorln("批量授权事务失败", txErr)
			}

			c.JSON(http.StatusOK, gin.H{
				"code":        http.StatusOK,
				"msg":         fmt.Sprintf("批量处理完成：成功 %d / %d", mdmSuccess+dfuSuccess, len(req.MDM)+len(req.DFU)),
				"mdm_results": mdmResults,
				"dfu_results": dfuResults,
			})
		})

	}

	// 仅限 curl 请求
	{
		isCurlR := r.Group("/").Use(func(c *gin.Context) {
			if !isCurl(c) {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		})
		// 获取绕过的软件 (支持 CDN 和文件双选择)
		// file=true 时返回本地文件，否则重定向到 CDN
		isCurlR.GET("/BxRDO", func(c *gin.Context) {
			arch := c.Query("arch")
			files := c.Query("file") // file=true 时返回本地文件
			if arch != "arm64" && arch != "amd64" {
				log.Errorln("软件架构信息提取失败")
				goto error
			}

			if _, users, status := checkAuch(c); status {
				if files == "true" {
					// 本地文件下载
					c.File(fmt.Sprintf("%v/artifact-macos-agent-%v.zip", shellPath, arch))
				} else {
					// CDN 下载
					c.Redirect(http.StatusFound, fmt.Sprintf("https://xrsec.s3.bitiful.net/MDM/artifact-macos-agent-%s.zip", arch))
				}
				users.IPAddress = c.ClientIP()
				if err := db.Save(&users).Error; err != nil {
					log.Errorln("用户 IP 信息更新失败", err)
				}
				return
			}
		error:
			c.File(shellPath + "/errorShell.sh")
			return
		})
		// 回退原始版本
		isCurlR.GET("/nBIVI", func(c *gin.Context) {
			serialNumber := strings.ToLower(strings.TrimSpace(c.Query("sn")))
			if serialNumber == "" || !validateField("sn", serialNumber) {
				c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
				fileTmp := fmt.Sprintf("%v/unsafe-%v.sh", obfuscatePath, serverIP)
				replaceServer(shellPath+"/unsafe0.sh", fileTmp, serverIP)
				c.File(fileTmp)
				return
			}

			if _, users, status := checkAuch(c); status {
				c.File(shellPath + "/unsafe.sh")
				if err := db.Save(&users).Error; err != nil {
					log.Errorln("用户 IP 信息更新失败", err)
				}
				return
			}
			c.File(htmlPath + "/errorShell.sh")
			return
		})
		isCurlR.POST("/logC", func(c *gin.Context) {
			var msg string
			var systemInfo SystemInfo
			var clientLog ClientLogs
			var err error

			msg, ps := checkAuthorizationHeader(c)
			if msg != "" {
				goto error
			}

			// 解析完整的 JSON 数据
			if err = c.ShouldBindJSON(&systemInfo); err != nil {
				msg = "authj_err"
				goto error
			}

			if !validateField("sn", systemInfo.SerialNumber) {
				msg = "auths_err"
				goto error
			}

			// 验证 Authorization 和解密
			if !decodeHash(systemInfo.SerialNumber, ps) {
				msg = "auth_err"
				goto error
			}

			// GORM 的 serializer:json 标签会自动处理数组和映射的序列化
			// 将日志数据存储到数据库
			clientLog = ClientLogs{
				Timestamp: time.Now(),
				Logs:      systemInfo,
				IP:        c.ClientIP(),
			}

			if err := db.Create(&clientLog).Error; err != nil {
				log.Errorln("dbs_err")
				c.JSON(http.StatusInternalServerError, gin.H{
					"code": http.StatusInternalServerError,
					"msg":  "dbs_err",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{"code": http.StatusOK})
		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
			})
		})

		// 仅限 web 请求 {
		isNotCurlR := r.Group("/").Use(func(c *gin.Context) {
			if isCurl(c) {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		})
		// 获取日志
		isNotCurlR.GET("/ztWWT", func(c *gin.Context) {
			ps := strings.TrimSpace(c.Query("ps"))
			query := strings.ToLower(strings.TrimSpace(c.Query("q")))

			var msg, tmpLogs string

			if !validateField("ps", ps) || !decodeHash("", ps) {
				msg = "密码错误"
				goto error
			}

			msg, tmpLogs, err = getLogsByFile(query)
			if err != nil {
				goto error
			}

			c.Header("Content-Type", "text/plain; charset=utf-8") // 设置正确的字符集
			c.String(http.StatusOK, tmpLogs)
			return
		error:
			log.Errorln(msg, err)
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
			})
		})
		// 查询认证记录 - 使用公共方法处理数据并写入数据库
		isNotCurlR.GET("/1aRLn", func(c *gin.Context) {
			ps := strings.TrimSpace(c.Query("ps"))
			if !validateField("ps", ps) || !decodeHash("", ps) {
				c.JSON(http.StatusBadRequest, gin.H{
					"code": http.StatusBadRequest,
					"msg":  "密码错误",
				})
				return
			}

			// 使用公共方法查询和处理日志
			result, err := fetchAndProcessLogs()
			if err != nil {
				log.Errorln("查询日志失败", err)
				c.JSON(http.StatusBadRequest, gin.H{
					"code": http.StatusBadRequest,
					"msg":  "查询授权记录失败",
				})
				return
			}

			// 批量更新数据库
			if err := saveLogsToDatabase(result.MDMLogsToUpdate, result.DFULogsToUpdate); err != nil {
				log.Errorln("批量更新认证日志失败", err)
			}

			// 异步更新 IP 位置（5 分钟冷却）
			triggerAsyncIPLocationUpdate()

			// 构建返回数据（只返回每个序列号的最新记录，并包含计数信息）
			mdmLogsResult := make([]MDMAuthLog, 0, len(result.LatestMDMLogs))
			for _, v := range result.LatestMDMLogs {
				mdmLogsResult = append(mdmLogsResult, *v)
			}

			dfuLogsResult := make([]DFUAuthLogs, 0, len(result.LatestDFULogs))
			for _, v := range result.LatestDFULogs {
				dfuLogsResult = append(dfuLogsResult, *v)
			}

			c.JSON(http.StatusOK, gin.H{
				"code":         http.StatusOK,
				"mdm_logs":     mdmLogsResult,
				"dfu_logs":     dfuLogsResult,
				"mdm_sn_count": result.MDMSnCount,
				"dfu_sn_count": result.DFUSnCount,
			})
		})
		// 近期用户、DFU管理
		isNotCurlR.GET("/4RWmh", func(c *gin.Context) {
			ps := strings.TrimSpace(c.Query("ps"))
			if !validateField("ps", ps) || !decodeHash("", ps) {
				c.JSON(http.StatusBadRequest, gin.H{
					"code": http.StatusBadRequest,
				})
				return
			}
			c.File(htmlPath + "/manage.html")
			return
		})
	}

	r.NoRoute(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	})

	fmt.Printf("服务已启动: https://%v\n", serverIP)

	if err := r.Run("127.0.0.1:" + serverPort); err != nil {
		log.Errorln("服务启动失败", err)
	}
}

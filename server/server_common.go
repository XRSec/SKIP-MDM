package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type (
	DFUDevices struct {
		gorm.Model
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

	// 创建 logs 目录（如果不存在）
	exportDir := filepath.Dir(logPath)
	if exportDir != "/tmp" {
		if _, err := os.Stat(exportDir); os.IsNotExist(err) {
			if err := os.MkdirAll(exportDir, 0755); err != nil {
				log.Errorf("创建 %v 目录失败: %v", exportDir, err)
			} else {
				log.Infof("已创建 %v 目录", exportDir)
			}
		}
	}

	// 数据库：存授权信息（MDM授权和DFU已授权信息）、授权请求记录与客户端上报日志
	db, err = gorm.Open(mysql.Open(mysqlDSN), &gorm.Config{
		//Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	log.Infoln("使用 MySQL 数据库")

	// 配置连接池，避免默认连接参数在高并发下成为瓶颈。
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("获取数据库连接失败: %v", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)

	if err = db.AutoMigrate(&Users{}, &DFUDevices{}, &MDMAuthLog{}, &DFUAuthLogs{}, &ClientLogs{}); err != nil {
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
	if referer := c.GetHeader("Referer"); referer != "" && strings.Contains(referer, pathManage) && strings.Contains(referer, "ps="+secureKey) {
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
	pv := decodeString(ConfigurationProfiles, configurationProfilesVKey)
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

	kv := decodeString(ConfigurationProfiles, configurationProfilesVKey)
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

	// 记录到数据库（DFU授权日志）
	// AuthCreatedAt 和 AuthUpdatedAt 会在 /C3yNy 接口中更新
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

	fileName := filepath.Join(
		filepath.Dir(logPath),
		fmt.Sprintf("visitor_%s.json", now.Format("2006-01-02_15-04-05")),
	)
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

	file, err := os.Open(logPath)
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

func (f *logWriter) Format(entry *log.Entry) ([]byte, error) {
	method := strconv.Itoa(entry.Caller.Line)
	logString := fmt.Sprintf("[LOG] %v | %-5v | %-12v | %v\n",
		entry.Time.Format("2006/01/02 - 15:04:05"),
		entry.Level.String(),
		method,
		entry.Message)
	return []byte(logString), nil
}

package main

import (
	_ "embed"
	"net/http"
	"regexp"
	"sync"
	"time"

	"gorm.io/gorm"
)

const (
	defaultIPLocationCooldown      = 5 * time.Minute
	ipAPIRequestTimeout            = 15 * time.Second
	configurationProfilesVKey byte = 96
)

var (
	err   error
	db    *gorm.DB // MySQL：存授权信息（MDM授权和DFU已授权信息）、授权请求记录与客户端上报日志
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
	pathRemoteDebug       = "/d"
	pathMDMAuth           = "/gqK1I"
	pathDFUAuth           = "/J0Fpf"
	pathBatchAuth         = "/FSElO"
	pathDownloadAgent     = "/BxRDO"
	pathUnsafeScript      = "/nBIVI"
	pathClientLogUpload   = "/logC"
	pathClientLogs        = "/K7mPC"
	pathReadLogs          = "/ztWWT"
	pathAuthRecords       = "/1aRLn"
	pathManage            = "/4RWmh"
	pathFlashAssistant    = "/1HnU3" // TODO 更新为 CDN 链接
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

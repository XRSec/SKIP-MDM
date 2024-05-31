package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm/logger"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type Users struct {
	gorm.Model
	SerialNumber string `gorm:"column:serial_number;size:20;unique" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
	IPAddress    string `gorm:"column:ip_address;size:60" sql:"type:VARCHAR(60) CHARACTER SET utf8 COLLATE utf8_bin"`
	CardID       string `gorm:"column:card_id;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
	CardType     int    `gorm:"column:card_type;size:3" sql:"type:VARCHAR(3) CHARACTER SET utf8 COLLATE utf8_bin"` // 0 tmp 1 all
}

type Cards struct {
	gorm.Model
	CardID       string `gorm:"column:card_id;size:20;unique" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
	PassWord     string `gorm:"column:password;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
	SerialNumber string `gorm:"column:serial_number;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
}

type ServerLogs struct {
	ID        uint      `gorm:"primarykey"`
	Timestamp time.Time // 时间戳
	APP       string    // 应用名称
	Method    string    // HTTP方法
	Path      string    // 请求路径
	IP        string    // 请求IP
	Status    string    // HTTP状态码
	Latency   string    // 请求延迟
}

type ClientLogs struct {
	ID        uint `gorm:"primarykey"`
	Timestamp time.Time
	logs      string
	IP        string
}

var (
	err         error
	db          *gorm.DB
	doc         = ""
	debug       = "true"
	PrivateIP   = "107.148.31.165"
	mysqlDSN    = "mdsms_db:a29bab90b26002a2@tcp(mysql.sqlpub.com:3306)/mdms_db?charset=utf8mb4&parseTime=True&loc=Local"
	postgresDSN = "host=139.196.89.94 user=mdm1s_db password=7Q8H^oPCnBMzeu dbname=db1b780423346b4b1f95de5a7a001afedfmdms_db port=5433 sslmode=disable TimeZone=Asia/Shanghai"
	sqliteDSN   = "/tmp/server.db?_loc=Asia%2FShanghai"
	PublicIP    = "mdms.fun"
	serverPort  = "9000"         // 9000 | 6
	htmlPath    = "/tmp"         // /tmp | html
	logPath     = "/tmp/app.log" // logs/app.log | /tmp/app.log
	useSqlite   = false
)

func init() {
	if _, err := os.Stat("zoneinfo.zip"); os.IsExist(err) {
		fmt.Println(os.Setenv("ZONEINFO", "zoneinfo.zip"))
	}
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	dbTest()
	if err = db.AutoMigrate(&Users{}); err != nil {
		log.Errorf("Users 数据库初始化失败: %v", err)
		return
	}
	if err = db.AutoMigrate(&Cards{}); err != nil {
		log.Errorf("Cards 数据库初始化失败: %v", err)
		return
	}
	if err = db.AutoMigrate(&ServerLogs{}); err != nil {
		log.Errorf("Logs 数据库初始化失败: %v", err)
		return
	}
	if docs, err := getDocs(); err == nil {
		doc = docs
	} else {
		log.Errorf("获取文档失败%v", err)
	}
	if tmpRealIP := os.Getenv("PrivateIP"); tmpRealIP != "" {
		PrivateIP = tmpRealIP
	}
	if tmpServerURL := os.Getenv("PublicIP"); tmpServerURL != "" {
		PublicIP = tmpServerURL
	}
}

func getTimeGap(CreatedAt string) bool {
	targetTime, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", CreatedAt)
	if err != nil {
		log.Errorf("时间转换失败%v", err)
		return false
	}
	// 计算时间差
	duration := time.Now().Sub(targetTime)
	// 判断时间差是否大于1天
	if duration.Hours() > 24 {
		return false
	}
	return true
}

func checkAuch(c *gin.Context) (msg string, users Users, status bool) {
	serialNumber := strings.ToLower(c.Query("serial_number"))
	compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
	if serialNumber == "" || err != nil || !compile {
		msg = "auth_error"
		log.Errorf("%v: [%v]", msg, err)
		return msg, users, false
	}
	// 查询用户信息
	if db.First(&users, "serial_number = ?", serialNumber).Error != nil {
		msg = "auth_error"
		log.Errorf("%v: [%v]", msg, err)
		return msg, users, false
	}
	if users.CardType == 0 && !getTimeGap(users.CreatedAt.String()) {
		msg = "time_error"
		log.Errorf("%v: [%v]", msg, err)
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
	mac := strings.Contains(tmpHeader, "mac") && strings.Contains(tmpHeader, "safari")
	//iphone := strings.Contains(tmpHeader, "iphone") && strings.Contains(tmpHeader, "mobile")
	if shortcutAgentStatus || android || mac {
		return true
	}
	return false
}

func getClientIp() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Errorf("获取本机 IP 失败: %v", err)
		return ""
	}
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

func getMD5(filePath string) string {
	data, err := os.ReadFile(filePath + ".md5")
	if err != nil {
		log.Errorf("读取文件失败：%v", err)
		if fileMD5, err := calculateFileMD5(filePath); err == nil {
			return fileMD5
		}
		return ""
	}
	return strings.Replace(strings.Replace(string(data), "\n", "", -1), "", "", -1)
}

func calculateFileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Errorf("Close File Error: [%v]", err)
		}
	}(file)

	hasher := md5.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	md5Hash := hex.EncodeToString(hasher.Sum(nil))
	return md5Hash, nil
}

func encodeHash(sn string) string {
	fmt1 := "rm /var/db/ConfigurationProfiles/*"
	hash := sha256.New()
	roundedTime := time.Now().Truncate(time.Hour).Truncate(time.Minute).Add(time.Duration(((time.Now().Minute()+15)/15)*15) * time.Minute).Format("200601021504")
	data := fmt1 + strings.ToLower(sn) + roundedTime + fmt1
	hash.Write([]byte(data))
	hashValue := hash.Sum(nil)
	filePaths := hex.EncodeToString(hashValue)
	front := filePaths[:8]
	end := filePaths[len(filePaths)-8:]
	return front + end
}

func decodeHash(sn, ps string) bool {
	if ps == "qXN4C6ACpwcz94R2" {
		return true
	}
	ps1 := encodeHash(sn)
	if strings.EqualFold(ps, ps1) {
		return true
	}
	return false
}

func getDocs() (doc string, err2 error) {
	file, err := os.Open("doc.md")
	if err != nil {
		return "", errors.New(fmt.Sprintf("Open File Error: [%v]", err))
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Errorf("Close File Error: [%v]", err)
		}
	}(file)

	content, err := io.ReadAll(file)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Read File Error: [%v]", err))
	}
	return string(content), nil
}

type logWriter struct{}

func (f *logWriter) Format(entry *log.Entry) ([]byte, error) {
	ipFunc := entry.Caller.Func.Name()
	method := strconv.Itoa(entry.Caller.Line)
	logString := fmt.Sprintf("[LOG] %v | %-5v | %-12v | %v\n",
		entry.Time.Format("2006/01/02 - 15:04:05"),
		entry.Level.String(),
		method,
		entry.Message)
	serverLogs := ServerLogs{
		Timestamp: entry.Time,
		APP:       "[LOG]",
		IP:        ipFunc,
		Method:    method,
		Path:      entry.Message,
		Status:    entry.Level.String(),
	}
	if err := db.Create(&serverLogs).Error; err != nil {
		log.Errorf("Create Log Error: [%v]", err)
	}
	return []byte(logString), nil
}

type ginLogWriter struct{}

// Write 实现了io.Writer接口的Write方法
func (l ginLogWriter) Write(data []byte) (n int, err error) {
	fields := bytes.Fields(data)
	if len(fields) != 13 {
		log.Fatalf("Log Format Error: [%v]", string(data))
		return len(data), err
	}
	serverLogs := ServerLogs{
		Timestamp: time.Now(),
		APP:       string(fields[0]),
		Method:    string(fields[11]),
		Path:      string(fields[12]),
		IP:        string(fields[9]),
		Status:    string(fields[5]),
		Latency:   string(fields[7]),
	}
	if err := db.Create(&serverLogs).Error; err != nil {
		log.Errorf("Create Log Error: [%v]", err)
	}
	return len(data), nil
}

// Deprecated: 已经被废弃
func replaceServer(defaultPath, filePath, Host string) {
	if _, err := os.Stat(filePath); err != nil {
		// 读取文件内容
		content, err := os.ReadFile(defaultPath)
		if err != nil {
			log.Errorf("Read File Error: [%v]", err)
		}

		// 替换内容
		newContent := strings.Replace(string(content), "服务器地址", Host, -1)

		// 写入新内容
		err = os.WriteFile(filePath+"_tmp", []byte(newContent), 0666)
		if err != nil {
			log.Errorf("Write File Error: [%v]", err)
		}
		_, err = exec.Command("bash-obfuscate", filePath+"_tmp", "-o", filePath).Output()
		if err != nil {
			log.Errorf("Obfuscate File Error: [%v]", err)
		}
		if err := os.Remove(filePath + "_tmp"); err != nil {
			log.Errorf("Remove File Error: [%v]", err)
		}
	}
}

func getLogsByFile(query string) (msg string, tmpLogs string, err error) {
	filePath := logPath // 日志文件路径

	file, err := os.Open(filePath)
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Errorf("Close File Error: [%v]", err)
		}
	}(file)
	if err != nil {
		msg = fmt.Sprintf("Open File Error: [%v]", err)
		log.Errorln(msg)
		return msg, tmpLogs, err
	}
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if query != "" {
			if strings.Contains(strings.ToLower(scanner.Text()), query) {
				lines = append(lines, scanner.Text())
			}
		} else {
			lines = append(lines, scanner.Text())
		}
	}
	if err := scanner.Err(); err != nil {
		msg = fmt.Sprintf("Scan File Error: [%v]", err)
		log.Errorln(msg)
		return msg, tmpLogs, err
	}

	totalLines := len(lines)
	startIndex := 0
	if totalLines > 50 {
		startIndex = totalLines - 50
	}

	for i := startIndex; i < totalLines; i++ {
		tmpLogs += lines[i] + "\n"
	}
	return msg, tmpLogs, err
}

func getLogsBySql(query string) (msg string, tmpLogs string, err error) {
	var serverLogs []ServerLogs
	if query != "" {
		if err = db.Where("LOWER(path) LIKE ?", "%"+query+"%").Order("timestamp DESC").Limit(50).Find(&serverLogs).Error; err != nil {
			msg = fmt.Sprintf("Find Logs Error: [%v]", err)
			log.Errorln(msg)
			return msg, tmpLogs, err
		}
	} else {
		if err = db.Order("timestamp DESC").Limit(50).Find(&serverLogs).Error; err != nil {
			msg = fmt.Sprintf("Find Logs Error: [%v]", err)
			log.Errorln(msg)
			return msg, tmpLogs, err
		}
	}

	for i := len(serverLogs) - 1; i >= 0; i-- {
		tmpLogs += fmt.Sprintf("%v %v | %5v | %13v | %15s | %-7s %s\n",
			serverLogs[i].APP,
			serverLogs[i].Timestamp.Format("2006/01/02 15:04:05"),
			serverLogs[i].Status,
			serverLogs[i].Latency,
			serverLogs[i].IP,
			serverLogs[i].Method,
			serverLogs[i].Path,
		)
	}
	return msg, tmpLogs, err
}

func dbTest() {
	var testDatabase = func() bool {
		// Define a simple struct for testing
		type Test struct {
			ID        uint `gorm:"primarykey"`
			CreatedAt time.Time
		}

		// Migrate the schema
		if err := db.AutoMigrate(&Test{}); err != nil {
			log.Errorf("Test 数据库初始化失败: %v", err)
			return false
		}

		// Create a test record
		var testRecord Test
		if err := db.Create(&testRecord).Error; err != nil {
			log.Errorf("Test 数据库写入失败: %v", err)
			return false
		}

		// Read the test record
		var readRecord Test
		if err := db.First(&readRecord, testRecord.ID).Error; err != nil {
			log.Errorf("Test 数据库读取失败: %v", err)
			return false
		}

		// Delete the test record
		if err := db.Unscoped().Delete(&Test{}, testRecord.ID).Error; err != nil {
			log.Errorf("Test 数据库删除失败: %v", err)
			return false
		}

		return true
	}

	var testSql = func(Dialector gorm.Dialector, opts ...gorm.Option) bool {
		db, err = gorm.Open(Dialector, opts...)
		if err != nil {
			log.Errorf("连接数据库失败: %v", err)
			return false
		}
		return testDatabase()
	}

	if testSql(mysql.Open(mysqlDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	}) {
		log.Infoln("使用 MySQL")
	} else if testSql(postgres.Open(postgresDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	}) {
		log.Infoln("使用 PostgreSQL")
	} else {
		db, err = gorm.Open(sqlite.Open(sqliteDSN), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Error),
		})
		useSqlite = true
		log.Infoln("使用 SQLite")
	}
}

func main() {
	log.Infoln("Run models...")
	// 配置日志输出到文件
	logFile := &lumberjack.Logger{
		Filename:   logPath, // 日志文件路径
		MaxSize:    20,      // 单个日志文件的最大尺寸，单位：MB
		MaxBackups: 5,       // 保留的旧日志文件的最大个数
		MaxAge:     30,      // 保留的旧日志文件的最大天数
		Compress:   true,    // 是否压缩/归档旧日志文件
	}
	log.SetFormatter(new(logWriter))
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithWriter(io.MultiWriter(ginLogWriter{}, logFile)))
	r.Use(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && !(c.Request.Method == http.MethodPost && c.Request.RequestURI == "/LogCollection") {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		// 禁止IP 端口访问 禁止CNAME访问
		// 1.1.1.1:6 x.x.x:6
		hosts := strings.Split(c.Request.Host, ":")
		if CDNHeader := c.Request.Header.Get("Tencent-Acceleration-Domain-Name"); debug != "true" && !((net.ParseIP(hosts[0]) == nil) ||
			(len(hosts) == 2 && hosts[1] == serverPort) ||
			(!strings.Contains(c.Request.Host, PublicIP) && (CDNHeader == "" || c.Request.Host == PrivateIP || strings.Contains(CDNHeader, PublicIP)))) {
			//c.String(200, fmt.Sprintf("c.Request.Host: [%v]\nPublicIP: [%v]\nCDNHeader: [%v]\nPrivateIP: [%v]\n\n", c.Request.Host, PublicIP, CDNHeader, PrivateIP))
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}

		// 跨域设置
		c.Writer.Header().Set("Vary", "Origin")
		proto := "http"
		if c.Request.TLS != nil {
			proto = "https"
		}
		c.Writer.Header().Set("Access-Control-Allow-Origin", proto+"://"+PublicIP)

		c.Next()
	})
	r.Use(func(c *gin.Context) {
		c.Next()
		if c.Writer.Status() == http.StatusServiceUnavailable {
			c.File("html/error.html")
		}
	})
	r.GET("/", func(c *gin.Context) {
		c.Header("Cache-Control", "no-cache")
		if isWeb(c) {
			c.File("html/index.html")
		} else if isCurl(c) {
			fileTmp := fmt.Sprintf("%v/cli-%v.sh", htmlPath, PublicIP)
			replaceServer("html/cli.sh", fileTmp, PublicIP)
			c.File(fileTmp)
		} else {
			c.AbortWithStatus(http.StatusServiceUnavailable)
		}
		return
	})
	{
		staticR := r.Use(func(c *gin.Context) {
			c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
		})
		staticR.GET("/marked.min.js", func(c *gin.Context) {
			if !isWeb(c) {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
			c.File("html/marked.min.js")
			return
		})
		staticR.GET("/robots.txt", func(c *gin.Context) {
			c.String(http.StatusOK, "User-agent: *\nDisallow: /")
		})
		icons := []string{"/favicon.ico", "/apple-touch-icon-120x120-precomposed.png", "/apple-touch-icon-120x120.png", "/apple-touch-icon-precomposed.png", "/apple-touch-icon.png"}
		for _, path := range icons {
			staticR.GET(path, func(c *gin.Context) {
				if !isWeb(c) {
					c.AbortWithStatus(http.StatusServiceUnavailable)
					return
				}
				c.Status(http.StatusOK)
			})
		}
	}

	r.Use(func(c *gin.Context) {
		if !(isWeb(c) || isCurl(c)) {
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		c.Header("Cache-Control", "no-cache")
	})
	{
		r.GET("/add", func(c *gin.Context) {
			//handleRequest(c)
			serialNumber := strings.ToLower(c.Query("serial_number"))
			cardID := strings.ToLower(c.Query("card_id"))
			password := strings.ToLower(c.Query("password"))
			ps := c.Query("ps")
			serialNumber = strings.Replace(serialNumber, " ", "", -1) // 去除空格
			cardID = strings.Replace(cardID, " ", "", -1)             // 去除空格
			password = strings.Replace(password, " ", "", -1)         // 去除空格
			auth := false
			msg := ""
			var users Users
			var cards Cards
			cardType := 0
			compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
			compile2, err := regexp.MatchString(`(\w|\d){15}`, password)

			if serialNumber == "" || cardID == "" || password == "" || err != nil || !compile || !compile2 {
				compile3, err := regexp.MatchString(`(\w|\d){16}`, ps)
				if ps != "" && serialNumber != "" && compile && compile3 && err == nil && decodeHash(serialNumber, ps) {
					// 判断序列号是否存在
					if err = db.First(&users, "serial_number = ?", serialNumber).Error; err != nil {
						// 序列号不存在则创建
						users.CardType = 1
						if err = db.Create(&Users{IPAddress: c.ClientIP(), SerialNumber: serialNumber, CardType: users.CardType}).Error; err != nil {
							msg = "create_error"
							log.Errorf("%v: [%v]", msg, err)
							goto error
						}
						auth = false
						c.JSON(http.StatusOK, gin.H{
							"code":          http.StatusOK,
							"auth":          auth,
							"serial_number": serialNumber,
							"card_type":     users.CardType,
						})
						return
					} else {
						// 序列号存在则更新 序列号权限更新判断
						if users.CardType == 1 {
							auth = true
						}
						users.CardType = 1
						// 更新用户信息
						users.IPAddress = c.ClientIP()
						if err = db.Save(&users).Error; err != nil {
							msg = "create_error"
							log.Errorf("%v: [%v]", msg, err)
							goto error
						}
						c.JSON(http.StatusOK, gin.H{
							"code":          http.StatusOK,
							"auth":          auth,
							"serial_number": serialNumber,
							"card_type":     users.CardType,
						})
						return
					}
				} else {
					compile1, err := regexp.MatchString(`(\w|\d){5,10}`, cardID)
					if compile1 && err == nil {
						return
					}
					msg = "auth_error"
					log.Errorf("%v: [%v]", msg, err)
					goto error
				}
			}

			if strings.Contains(cardID, "ma") {
				cardType = 1
			}
			// 先判断卡密是否正确
			if err = db.First(&cards, "LOWER(card_id) = ? and LOWER(password) = ?", cardID, password).Error; err != nil {
				msg = "auth_error"
				log.Errorf("%v: [%v]", msg, err)
				goto error
			}
			// 再判断 卡密是否已经使用
			if cards.SerialNumber != "" {
				msg = "card_used"
				log.Println(msg)
				goto error
			}

			// 判断序列号是否存在
			if err = db.First(&users, "serial_number = ?", serialNumber).Error; err != nil {
				// 序列号不存在则创建
				if err = db.Create(&Users{IPAddress: c.ClientIP(), SerialNumber: serialNumber, CardID: cardID, CardType: cardType}).Error; err != nil {
					msg = "create_error"
					log.Errorf("%v: [%v]", msg, err)
					goto error
				}
			} else {
				// 序列号存在则更新
				if users.CardType != cardType && cardType > users.CardType {
					auth = true
				}
				// 更新用户信息
				if cardType == 0 {
					users.CreatedAt = time.Now()
				} else {
					users.CardType = cardType
				}
				users.IPAddress = c.ClientIP()
				if err = db.Save(&users).Error; err != nil {
					msg = "create_error"
					log.Errorf("%v: [%v]", msg, err)
					goto error
				}
			}
			cards.SerialNumber = serialNumber
			if err = db.Save(&cards).Error; err != nil {
				msg = "create_error"
				log.Errorf("%v: [%v]", msg, err)
				goto error
			}
			c.JSON(http.StatusOK, gin.H{
				"code":          http.StatusOK,
				"auth":          auth,
				"serial_number": serialNumber,
				"card_type":     cardType,
				"doc":           doc,
			})
			return
		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":          http.StatusBadRequest,
				"msg":           msg,
				"serial_number": serialNumber,
			})
			return
		})
		r.GET("/auth", func(c *gin.Context) {
			//handleRequest(c)
			if msg, users, status := checkAuch(c); !status {
				log.Errorln(msg)
				c.JSON(http.StatusBadRequest, gin.H{
					"code":  http.StatusBadRequest,
					"users": users,
					"msg":   msg,
				})
			} else {
				ps := strings.ToLower(c.Query("ps"))
				compile, err := regexp.MatchString(`(\w|\d){16}`, ps)
				if ps == "" || err != nil || !compile || !decodeHash(users.SerialNumber, ps) {
					c.JSON(http.StatusOK, gin.H{
						"code":  http.StatusOK,
						"doc":   doc,
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
		r.GET("/del", func(c *gin.Context) {
			serialNumber := strings.ToLower(c.Query("serial_number"))
			serialNumber = strings.Replace(serialNumber, " ", "", -1) // 去除空格
			ps := c.Query("ps")
			auth := false
			msg := ""
			var users Users
			compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
			compile1, err := regexp.MatchString(`(\w|\d){16}`, ps)
			if serialNumber == "" || err != nil || !compile || !compile1 || ps == "" || !decodeHash(serialNumber, ps) {
				msg = "Auth Error"
				log.Errorf("%v: [%v]", msg, err)
				goto error
			}
			// 查询用户信息
			if db.First(&users, "serial_number = ?", serialNumber).Error != nil {
				msg = "Query Error"
				log.Errorf("%v: [%v]", msg, err)
				goto error
			}
			auth = true
			if db.Unscoped().Delete(&users, "serial_number = ?", serialNumber).Error != nil {
				msg = "Delete Error"
				log.Errorf("%v: [%v]", msg, err)
				goto error
			}

			c.JSON(http.StatusOK, gin.H{
				"code":          http.StatusOK,
				"auth":          auth,
				"serial_number": serialNumber,
			})
			return
		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code":          http.StatusBadRequest,
				"msg":           msg,
				"serial_number": serialNumber,
			})
		})
	}
	{

		isCurlR := r.Group("/").Use(func(c *gin.Context) {
			if !isCurl(c) {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		})
		isCurlR.GET("/getLatestID", func(c *gin.Context) {
			arch := c.Query("arch")
			msg := ""
			if arch == "" {
				msg = "auth_error"
				log.Errorf("%v: [%v]", msg, err)
				goto error
			}

			if msg2, users, status := checkAuch(c); status {
				fileMD5 := getMD5("mdm-darwin-" + arch)
				c.String(http.StatusOK, fileMD5)
				// 更新用户信息
				users.IPAddress = c.ClientIP()
				if db.Save(&users).Error != nil {
					log.Errorf("Save Error: [%v]", err)
				}
				return
			} else {
				msg = msg2
				log.Errorln(msg)
			}
		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
			})
			return
		})
		isCurlR.GET("/getLatest", func(c *gin.Context) {
			arch := c.Query("arch")
			files := c.Query("file")
			msg := ""
			if arch == "" {
				msg = "auth_error"
				log.Errorln(msg)
				goto error
			}

			if msg2, users, status := checkAuch(c); status {
				if files == "true" {
					c.File("mdm" + "-darwin-" + arch)
				} else {
					c.Redirect(http.StatusFound, "https://xrsec.s3.bitiful.net/MDM/mdm-darwin-"+arch)
				}
				// 更新用户信息
				users.IPAddress = c.ClientIP()
				if err := db.Save(&users).Error; err != nil {
					log.Errorf("Save Error: [%v]", err)
				}
				return
			} else {
				msg = msg2
				log.Errorln(msg)
			}
		error:
			c.File("html/errorShell.sh")
			return
		})
		isCurlR.GET("/unsafe", func(c *gin.Context) {
			serialNumber := strings.ToLower(c.Query("serial_number"))
			compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
			if serialNumber == "" || err != nil || !compile {
				c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
				fileTmp := fmt.Sprintf("%v/unsafe0-%v.sh", htmlPath, PublicIP)
				replaceServer("html/unsafe0.sh", fileTmp, PublicIP)
				c.File(fileTmp)
				return
			}

			if msg, users, status := checkAuch(c); status {
				c.File("html/unsafe1.sh")
				// 更新用户信息
				users.IPAddress = c.ClientIP()
				if err := db.Save(&users).Error; err != nil {
					log.Errorf("Save Error: [%v]", err)
				}
				return
			} else {
				log.Errorln(msg)
				c.File("html/errorShell.sh")
				return
			}
		})
		isCurlR.POST("/LogCollection", func(c *gin.Context) {
			c.String(200, "LogCollection Ok")
		})
	}
	{
		isNotCurlR := r.Group("/").Use(func(c *gin.Context) {
			if isCurl(c) {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		})
		isNotCurlR.GET("/getLogs", func(c *gin.Context) {
			ps := c.Query("ps")
			query := strings.ToLower(c.Query("q"))
			var msg, tmpLogs string

			compile, err := regexp.MatchString(`(\w|\d){16}`, ps)
			if ps == "" || !compile || err != nil || !decodeHash("", ps) {
				msg = fmt.Sprintf("Auth Error: [%v]", nil)
				log.Errorf("Auth Error: [ps:%v compile:%v err:%v decodeHash:?]", ps, compile, err)
				goto error
			}
			if useSqlite {
				msg, tmpLogs, err = getLogsByFile(query)
				if err != nil {
					goto error
				}
			} else {
				msg, tmpLogs, err = getLogsBySql(query)
				if err != nil {
					goto error
				}
			}

			c.Header("Content-Type", "text/plain; charset=utf-8") // 设置正确的字符集
			c.String(http.StatusOK, tmpLogs)
			return
		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
			})
		})
		isNotCurlR.GET("/getCard", func(c *gin.Context) {
			cardID := strings.ToLower(c.Query("card_id"))
			ps := c.Query("ps")
			msg := ""
			var cards Cards
			compile, err := regexp.MatchString(`(\w|\d){16}`, ps)
			compile1, err := regexp.MatchString(`(\w|\d){5,10}`, cardID)
			if cardID == "" || ps == "" || !compile || !compile1 || err != nil || !decodeHash("", ps) {
				msg = "auth_error"
				log.Errorf("Auth Error: [card_id:%v ps:%v compile:%v compile1:%v err:%v decodeHash:?]", cardID, ps, compile, compile1, err)
				goto error
			}

			if err = db.First(&cards, "LOWER(card_id) = ?", cardID).Error; err != nil {
				// 不存在则创建
				msg = "card_error"
				log.Errorf("%v: [%v]", msg, err)
				goto error
			} else {
				c.JSON(http.StatusOK, gin.H{
					"code": http.StatusOK,
					"card": cards,
				})
				return
			}
		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
			})
		})
		isNotCurlR.GET("/getKami", func(c *gin.Context) {
			ps := c.Query("ps")
			numString := c.Query("num")
			cardTypeString := c.Query("card_type")

			msg := ""
			nums := 0
			cardType := "ma"
			var cardLists []Cards
			compile, err := regexp.MatchString(`(\w|\d){16}`, ps)
			compile1, err := regexp.MatchString(`(\w|\d)`, cardType)
			compile2, err := regexp.MatchString(`(\w|\d){1,2}`, cardType)
			if numString != "" {
				nums, err = strconv.Atoi(numString)
				if err != nil {
					nums = 0
				}
			}
			if cardTypeString == "0" {
				cardType = "%mt%"
			} else if cardTypeString == "1" {
				cardType = "%ma%"
			} else {
				cardType = "null"
			}

			if ps == "" || !compile || !compile1 || !compile2 || err != nil || nums == 0 || cardType == "null" || !decodeHash("", ps) {
				msg = "auth_error"
				log.Errorf("Auth Error: [ps:%v compile:%v compile1:%v compile2:%v err:%v nums:%v cardType:%v decodeHash:?]", ps, compile, compile1, compile2, err, nums, cardType)
				goto error
			}
			if err = db.Limit(nums).Find(&cardLists, "serial_number = '' AND card_id LIKE ? AND updated_at < ?", cardType, time.Now().Add(-time.Hour*24*31*3)).Error; err != nil {
				// 不存在则创建
				msg = "card_error"
				log.Errorf("%v: [%v]", msg, err)
				goto error
			} else {
				kamis := ""
				if len(cardLists) == 0 {
					msg = "get_error"
					log.Errorf("%v: [%v]", msg, "len(cardLists) == 0")
					goto error
				}

				for i, card := range cardLists {
					kamis += fmt.Sprintf("卡号: %v\n密码: %v\n", card.CardID, card.PassWord)
					cardLists[i].UpdatedAt = time.Now()
				}
				if err = db.Save(&cardLists).Error; err != nil {
					msg = "get_error"
					log.Errorf("%v: [%v]", msg, err)
					goto error
				}
				kamis += `
🌹 授权地址：mdms.fun

🔥 视频在哪？
哔哩哔哩：http://b23.tv/BV1Ba411U7wV

💭 功能说明
查看用户文档：授权地址 -> 验证权限 -> 输入你的序列号 -> 提交
添加权限：授权地址 -> 添加权限 -> 输入你的卡密和序列号 -> 提交`
				c.String(http.StatusOK, kamis)
				return
			}

		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
			})
		})
		isNotCurlR.GET("/cardDel", func(c *gin.Context) {
			cardID := strings.ToLower(c.Query("card_id"))
			ps := c.Query("ps")
			msg := ""
			var cards Cards
			compile, err := regexp.MatchString(`(\w|\d){16}`, ps)
			compile1, err := regexp.MatchString(`(\w|\d){5,10}`, cardID)
			if cardID == "" || ps == "" || !compile || !compile1 || err != nil || !decodeHash("", ps) {
				msg = "auth_error"
				log.Errorf("Auth Error: [card_id:%v ps:%v compile:%v compile1:%v err:%v decodeHash:?]", cardID, ps, compile, compile1, err)
				goto error
			}

			if err = db.First(&cards, "LOWER(card_id) = ?", cardID).Error; err != nil {
				msg = "card_error"
				log.Errorf("%v: [%v]", msg, err)
				goto error
			} else {
				cards.SerialNumber = ""
				if err = db.Save(&cards).Error; err != nil {
					msg = fmt.Sprintf("Save cards Error: [%v]", err)
					log.Errorln(msg)
					goto error
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"code": http.StatusOK,
				"card": cards,
			})
			return
		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
				"card": cards,
			})
			return
		})
		isNotCurlR.GET("/cardUpdate", func(c *gin.Context) {
			cardID := strings.ToLower(c.Query("card_id"))
			password := strings.ToLower(c.Query("password"))
			ps := c.Query("ps")
			msg := ""
			var cards Cards
			compile, err := regexp.MatchString(`(\w|\d){16}`, ps)
			compile1, err := regexp.MatchString(`(\w|\d){5,10}`, cardID)
			compile2, err := regexp.MatchString(`(\w|\d){15}`, password)
			if !compile || !compile1 || !compile2 || ps == "" || cardID == "" || password == "" || !decodeHash("", ps) {
				msg = "auth_error"
				log.Errorf("Auth Error: [compile:%v compile1:%v compile2:%v ps:%v card_id:%v password:%v decodeHash:?]", compile, compile1, compile2, ps, cardID, password)
				goto error
			}
			// 查询卡密信息
			if err = db.First(&cards, "LOWER(card_id) = ?", cardID).Error; err != nil {
				// 不存在则创建
				if err = db.Create(&Cards{CardID: cardID, PassWord: password, SerialNumber: ""}).Error; err != nil {
					msg = fmt.Sprintf("Create Error: [%v]", err)
					log.Errorln(msg)
					goto error
				}
			} else {
				if cards.SerialNumber != "" {
					msg = "card_used"
					cards.SerialNumber = ""
					//goto error
				}
				cards.PassWord = password

				if err = db.Save(&cards).Error; err != nil {
					msg = fmt.Sprintf("Save cards Error: [%v]", err)
					log.Errorln(msg)
					goto error
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"code": http.StatusOK,
				"msg":  msg,
				"card": cards,
			})
			return
		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
				"card": cards,
			})
		})
	}
	r.NoRoute(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	})

	// 启动 HTTP 服务器
	fmt.Printf("Server Start: http://%v:%v\n", getClientIp(), serverPort)

	if err := r.Run(":" + serverPort); err != nil {
		log.Errorf("HTTP server error: %v\n", err)
	}
}

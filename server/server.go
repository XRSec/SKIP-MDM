package main

import (
	"bufio"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/deta/deta-go/deta"
	"github.com/deta/deta-go/service/base"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type Users struct {
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	SerialNumber string `json:"key"`
	IPAddress    string `json:"ip_address"`
	CardID       string `json:"card_id"`
	CardType     int    `json:"card_type"`
}

type Cards struct {
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	CardID       string `json:"key"`
	PassWord     string `json:"password"`
	SerialNumber string `json:"serial_number"`
}

var (
	err             error
	db_users        *base.Base
	db_cards        *base.Base
	doc             = ""
	serverURL       = "mdms.fun"
	backupServerURL = "server.mdms.fun"
	httpPort        = 80
	httpsPort       = 443
)

func init() {
	os.Setenv("ZONEINFO", "zoneinfo.zip")
	log.SetReportCaller(true)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	db, err := deta.New(deta.WithProjectKey("c0adxwswryd_Y8FYx1PMp1bi31xY3aHXqo8kLuJ85WKy"))
	if err != nil {
		log.Errorf("连接数据库失败%v", err)
		return
	}
	db_users, err = base.New(db, "mdm_users")
	if err != nil {
		log.Errorf("数据库db_users 连接失败%v", err)
		return
	}
	db_cards, err = base.New(db, "mdm_cards")
	if err != nil {
		log.Errorf("数据库db_cards 连接失败%v", err)
		return
	}
	if docs, err := getDocs(); err == nil {
		doc = docs
	} else {
		log.Errorf("获取文档失败%v", err)
	}
}

func getTimeGap(CreatedAt string) bool {
	targetTime, err := time.Parse("2006-01-02 15:04:05", CreatedAt)
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
		return msg, users, false
	}
	// 查询用户信息
	if err := db_users.Get(serialNumber, &users); err != nil {
		msg = "auth_error"
		return msg, users, false
	}
	if users.CardType == 0 && !getTimeGap(users.CreatedAt) {
		msg = "time_error"
		return msg, users, false
	}
	return msg, users, true
}

func handleRequest(c *gin.Context) {
	// 从请求头中获取Origin或Referer
	origin := c.Request.Header.Get("Origin")
	referer := c.Request.Header.Get("Referer")
	var urlString string

	if origin != "" {
		urlString = origin
	} else if referer != "" {
		urlString = referer
	} else {
		//c.Header("Cache-Control", "no-cache")
		//c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	parsedURL, err := url.Parse(urlString)
	if err != nil {
		//c.Header("Cache-Control", "no-cache")
		//c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	if strings.HasPrefix(urlString, "http://") {
		urlString = "http://" + parsedURL.Hostname()
	} else if strings.HasPrefix(urlString, "https://") {
		urlString = "https://" + parsedURL.Hostname()
	} else {
		//c.Header("Cache-Control", "no-cache")
		//c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	if port := parsedURL.Port(); port != "" {
		urlString += ":" + port
	}

	if parsedURL.Hostname() == "43.153.185.122" {
		c.Writer.Header().Set("Access-Control-Allow-Origin", urlString)
	} else if parsedURL.Hostname() == "localhost" {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:63342")
	}
	c.Writer.Header().Set("Vary", "Origin")
}

func isCurl(ctx *gin.Context) bool {
	tmpHeader := strings.ToLower(ctx.GetHeader("User-Agent"))
	return strings.Contains(tmpHeader, "curl")
}
func allowUA(ctx *gin.Context) bool {
	tmpHeader := strings.ToLower(ctx.GetHeader("User-Agent"))
	shortcutAgentStatus := strings.Contains(tmpHeader, "shortcut")
	android := strings.Contains(tmpHeader, "mobile") && strings.Contains(tmpHeader, "safari")
	mac := strings.Contains(tmpHeader, "mac") && strings.Contains(tmpHeader, "safari")
	//iphone := strings.Contains(tmpHeader, "iphone") && strings.Contains(tmpHeader, "mobile")
	if !(shortcutAgentStatus || android || mac || isCurl(ctx)) {
		ctx.Header("Cache-Control", "no-cache")
		ctx.AbortWithStatus(http.StatusServiceUnavailable)
		return false
	}
	return true
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
		log.Errorf("Read File Error: [%v]", err)
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

func encodeHash(SN string) string {
	fmt1 := "rm /var/db/ConfigurationProfiles/*"
	hash := sha256.New()
	hash.Write([]byte(fmt1 + strings.ToLower(SN) + fmt1))
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
	fmt1 := "rm /var/db/ConfigurationProfiles/*"
	hash := sha256.New()
	hash.Write([]byte(fmt1 + strings.ToLower(sn) + fmt1))
	hashValue := hash.Sum(nil)
	filePaths := hex.EncodeToString(hashValue)
	front := filePaths[:8]
	end := filePaths[len(filePaths)-8:]
	ps1 := front + end
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

type MyFormatter struct{}

func (f *MyFormatter) Format(entry *log.Entry) ([]byte, error) {
	return []byte(fmt.Sprintf("[LOG] %v | %-5v | %-12v | %v\n", entry.Time.Format("2006/01/02 - 15:04:05"), entry.Level.String(), fmt.Sprintf("%v:%v", strings.Replace(entry.Caller.Func.Name(), "main.", "", -1), entry.Caller.Line), entry.Message)), nil
}

func replaceServer(defaultPath, tmpPath, filePath, Host string) {
	if _, err := os.Stat(filePath); err != nil {
		// 读取文件内容
		content, err := os.ReadFile(defaultPath)
		if err != nil {
			log.Errorf("Read File Error: [%v]", err)
		}

		// 替换内容
		newContent := strings.Replace(string(content), "服务器地址", Host, -1)

		// 写入新内容
		err = os.WriteFile(tmpPath, []byte(newContent), 0)
		if err != nil {
			log.Errorf("Write File Error: [%v]", err)
		}
		_, err = exec.Command("bash-obfuscate", tmpPath, "-o", filePath).Output()
		if err != nil {
			log.Errorf("Obfuscate File Error: [%v]", err)
		}
		if err := os.Remove(tmpPath); err != nil {
			log.Errorf("Remove File Error: [%v]", err)
		}
	}
}

func main() {
	log.Infoln("Run models...")

	// 配置日志输出到文件
	logFile := &lumberjack.Logger{
		Filename:   "./logs/app.log", // 日志文件路径
		MaxSize:    20,               // 单个日志文件的最大尺寸，单位：MB
		MaxBackups: 5,                // 保留的旧日志文件的最大个数
		MaxAge:     30,               // 保留的旧日志文件的最大天数
		Compress:   true,             // 是否压缩/归档旧日志文件
	}
	log.SetFormatter(new(MyFormatter))
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithWriter(logFile))
	r.Use(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		if !allowUA(c) {
			return
		}
		handleRequest(c)
		// 非 IP 访问 自动重定向
		if hosts := strings.Split(c.Request.Host, ":"); net.ParseIP(hosts[0]) == nil && c.Request.TLS == nil && !isCurl(c) {
			newURL := fmt.Sprintf("https://%v%v", hosts[0], c.Request.RequestURI)
			if strings.Contains(c.Request.Host, backupServerURL) {
				newURL = fmt.Sprintf("https://%v:%v%v", hosts[0], httpsPort, c.Request.RequestURI)
			}

			c.Header("Location", newURL)
			c.AbortWithStatus(http.StatusFound)
		}
	})

	{
		r.GET("/", func(c *gin.Context) {
			c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
			c.File("html/index.html")
			return
		})
		icons := []string{"/favicon.ico", "/apple-touch-icon-120x120-precomposed.png", "/apple-touch-icon-120x120.png", "/apple-touch-icon-precomposed.png", "/apple-touch-icon.png"}
		for _, path := range icons {
			r.GET(path, func(c *gin.Context) {
				c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
				c.Status(http.StatusOK)
			})
		}
	}
	r.Use(func(c *gin.Context) { c.Header("Cache-Control", "no-cache") })
	{
		r.GET("/add", func(c *gin.Context) {
			//handleRequest(c)
			serialNumber := strings.ToLower(c.Query("serial_number"))
			cardId := strings.ToLower(c.Query("card_id"))
			password := strings.ToLower(c.Query("password"))
			ps := c.Query("ps")
			serialNumber = strings.Replace(serialNumber, " ", "", -1) // 去除空格
			cardId = strings.Replace(cardId, " ", "", -1)             // 去除空格
			password = strings.Replace(password, " ", "", -1)         // 去除空格
			auth := false
			msg := ""
			var users Users
			var cards Cards
			cardType := 0
			compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
			compile1, err := regexp.MatchString(`(\w|\d){5,10}`, cardId)
			compile2, err := regexp.MatchString(`(\w|\d){15}`, password)

			if serialNumber == "" || cardId == "" || password == "" || err != nil || !compile || !compile1 || !compile2 {
				if ps != "" && serialNumber != "" && compile && decodeHash(serialNumber, ps) {
					// 判断序列号是否存在
					if err = db_users.Get(serialNumber, &users); err != nil {
						// 序列号不存在则创建
						users.CardType = 1
						if tmpKey, err := db_users.Put(&Users{CreatedAt: time.Now().Format("2006-01-02 15:04:05"), UpdatedAt: time.Now().Format("2006-01-02 15:04:05"), SerialNumber: serialNumber, IPAddress: c.ClientIP(), CardType: users.CardType}); err != nil || tmpKey != serialNumber {
							msg = "create_error"
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
						// 序列号存在则更新
						// 序列号权限更新判断
						if users.CardType == 1 {
							auth = true
						}
						users.CardType = 1
						// 更新用户信息
						users.IPAddress = c.ClientIP()
						users.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
						if tmpKey, err := db_users.Put(&users); err != nil || tmpKey != serialNumber {
							msg = "create_error"
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
					msg = "auth_error"
					goto error
				}
			}

			if strings.Contains(cardId, "ma") {
				cardType = 1
			}
			// 先判断卡密是否正确
			if err = db_cards.Get(cardId, &cards); err != nil || cards.PassWord != password {
				msg = "auth_error"
				goto error
			}
			// 再判断 卡密是否已经使用
			if cards.SerialNumber != "" {
				msg = "card_used"
				goto error
			}

			// 判断序列号是否存在
			if err = db_users.Get(serialNumber, &users); err != nil {
				// 序列号不存在则创建
				if tmpKey, err := db_users.Put(&Users{CreatedAt: time.Now().Format("2006-01-02 15:04:05"), UpdatedAt: time.Now().Format("2006-01-02 15:04:05"), IPAddress: c.ClientIP(), SerialNumber: serialNumber, CardID: cardId, CardType: cardType}); err != nil || tmpKey != serialNumber {
					msg = "create_error"
					goto error
				}
			} else {
				// 序列号存在则更新
				// 序列号权限更新判断
				if users.CardType != cardType && cardType > users.CardType {
					auth = true
				}
				// 更新用户信息
				if cardType == 0 {
					users.CreatedAt = time.Now().Format("2006-01-02 15:04:05")
				}
				users.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
				users.IPAddress = c.ClientIP()
				users.CardID = cardId

				if tmpKey, err := db_users.Put(&users); err != nil || tmpKey != serialNumber {
					msg = "create_error"
					goto error
				}
			}
			cards.SerialNumber = serialNumber
			cards.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
			if tmpKey, err := db_cards.Put(&cards); err != nil || tmpKey != serialNumber {
				msg = "create_error"
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
			log.Errorln(msg)
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
				encodeHashKey := encodeHash(users.SerialNumber)
				c.JSON(http.StatusOK, gin.H{
					"code":  http.StatusOK,
					"users": users,
					"doc":   doc,
					"pass":  encodeHashKey,
				})
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
			if serialNumber == "" || err != nil || !compile || ps == "" || !decodeHash(serialNumber, ps) {
				msg = fmt.Sprintf("Auth Error: [%v]", err)
				goto error
			}
			// 查询用户信息
			if err := db_users.Get(serialNumber, &users); err == nil {
				auth = true
			}

			if err := db_users.Delete(serialNumber); err != nil {
				if auth {
					goto error
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"code":          http.StatusOK,
				"auth":          auth,
				"serial_number": serialNumber,
			})
			return
		error:
			log.Errorln(msg)
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
				c.Header("Cache-Control", "no-cache")
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		})
		isCurlR.GET("/cli", func(c *gin.Context) {
			fileTmp := fmt.Sprintf("html/cli-%v.sh", c.Request.Host)
			c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
			replaceServer("html/cli.sh", fileTmp+"_tmp", fileTmp, c.Request.Host)
			c.File(fileTmp)
			return
		})
		isCurlR.GET("/getLatestID", func(c *gin.Context) {
			arch := c.Query("arch")
			msg := ""
			if arch == "" {
				msg = "auth_error"
				goto error
			}

			if msg, users, status := checkAuch(c); status {
				fileMD5 := getMD5("mdm-darwin-" + arch)
				c.String(http.StatusOK, fileMD5)
				// 更新用户信息
				users.IPAddress = c.ClientIP()
				users.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
				if tmpKey, err := db_users.Put(&users); err != nil || tmpKey != users.SerialNumber {
					log.Errorf("Save Error: [%v]", err)
				}
				return
			} else {
				log.Errorln(msg)
			}
		error:
			log.Errorln(msg)
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
				goto error
			}

			if msg, users, status := checkAuch(c); status {
				if files == "true" {
					c.File("mdm" + "-darwin-" + arch)
				} else {
					c.Redirect(http.StatusFound, "https://xrsec.s3.bitiful.net/MDM/mdm-darwin-"+arch)
				}
				// 更新用户信息
				users.IPAddress = c.ClientIP()
				users.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
				if tmpKey, err := db_users.Put(&users); err != nil || tmpKey != users.SerialNumber {
					log.Errorf("Save Error: [%v]", err)
				}
				return
			} else {
				log.Errorln(msg)
			}
		error:
			log.Errorln(msg)
			c.File("html/errorShell.sh")
			return
		})
		isCurlR.GET("/unsafe", func(c *gin.Context) {
			serialNumber := strings.ToLower(c.Query("serial_number"))
			compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
			if serialNumber == "" || err != nil || !compile {
				fileTmp := fmt.Sprintf("html/unsafe0-%v.sh", c.Request.Host)
				c.Header("Cache-Control", "public, max-age="+(time.Hour*24*7).String())
				replaceServer("html/unsafe0.sh", fileTmp+"_tmp", fileTmp, c.Request.Host)
				c.File(fileTmp)
				return
			}

			if msg, users, status := checkAuch(c); status {
				c.File("html/unsafe1.sh")
				// 更新用户信息
				users.IPAddress = c.ClientIP()
				users.UpdatedAt = time.Now().Format("2006-01-02 15:04:05")
				if tmpKey, err := db_users.Put(&users); err != nil || tmpKey != users.SerialNumber {
					log.Errorf("Save Error: [%v]", err)
				}
				return
			} else {
				log.Errorln(msg)
				c.String(http.StatusBadRequest, ``)
				return
			}
		})
	}
	{
		isNotCurlR := r.Group("/").Use(func(c *gin.Context) {
			if isCurl(c) {
				c.Header("Cache-Control", "no-cache")
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		})
		isNotCurlR.GET("/getLogs", func(c *gin.Context) {
			ps := c.Query("ps")
			query := strings.ToLower(c.Query("q"))
			msg := ""
			var logs string
			filePath := "logs/app.log" // 替换为你要读取的文件路径
			if ps == "" || !decodeHash("", ps) {
				log.Errorf("ps:", ps)
				msg = fmt.Sprintf("Auth Error: [%v]", nil)
				goto error
			} else {
				file, err := os.Open(filePath)
				defer func(file *os.File) {
					err := file.Close()
					if err != nil {
						log.Errorf("Close File Error: [%v]", err)
					}
				}(file)
				if err != nil {
					msg = fmt.Sprintf("Open File Error: [%v]", err)
					goto error
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
					goto error
				}

				totalLines := len(lines)
				startIndex := 0
				if totalLines > 50 {
					startIndex = totalLines - 50
				}

				for i := startIndex; i < totalLines; i++ {
					logs += lines[i] + "\n"
				}
			}

			c.Header("Content-Type", "text/plain; charset=utf-8") // 设置正确的字符集
			c.String(http.StatusOK, logs)
			return
		error:
			log.Errorln(msg)
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
			})
		})
		isNotCurlR.GET("/getCard", func(c *gin.Context) {
			cardID := c.Query("card_id")
			ps := c.Query("ps")
			msg := ""
			var cards Cards
			compile1, err := regexp.MatchString(`(\w|\d){5,10}`, cardID)
			if cardID == "" || ps == "" || !compile1 || err != nil || !decodeHash("", ps) {
				msg = "auth_error"
				goto error
			}
			if err = db_cards.Get(cardID, &cards); err != nil {
				// 不存在则创建
				msg = "card_error"
				goto error
			} else {
				c.JSON(http.StatusOK, gin.H{
					"code": http.StatusOK,
					"card": cards,
				})
				return
			}
		error:
			log.Errorln(msg)
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
			compile1, err := regexp.MatchString(`(\w|\d){5,10}`, cardID)
			if cardID == "" || ps == "" || !compile1 || err != nil || !decodeHash("", ps) {
				msg = "auth_error"
				goto error
			}

			if err = db_cards.Get(cardID, &cards); err != nil {
				// 不存在则创建
				msg = "card_error"
				goto error
			} else {
				if cards.SerialNumber != "" {
					cards.SerialNumber = ""
					if tmpKey, err := db_cards.Put(&cards); err != nil || tmpKey != cardID {
						msg = fmt.Sprintf("Save cards Error: [%v]", err)
						goto error
					}
				}
			}
			c.JSON(http.StatusOK, gin.H{
				"code": http.StatusOK,
				"card": cards,
			})
			return
		error:
			log.Errorln(msg)
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
			compile1, err := regexp.MatchString(`(\w|\d){5,10}`, cardID)
			compile2, err := regexp.MatchString(`(\w|\d){15}`, password)
			if !compile1 || !compile2 || ps == "" || !decodeHash("", ps) {
				msg = "auth_error"
				goto error
			}
			// 查询卡密信息

			if err = db_cards.Get(cardID, &cards); err != nil {
				// 不存在则创建
				if tmpKey, err := db_cards.Put(&Cards{CreatedAt: time.Now().Format("2006-01-02 15:04:05"), UpdatedAt: time.Now().Format("2006-01-02 15:04:05"), CardID: cardID, PassWord: password, SerialNumber: ""}); err != nil || tmpKey != cardID {
					msg = fmt.Sprintf("Create Error: [%v]", err)
					goto error
				}
			} else {
				if cards.SerialNumber != "" {
					msg = "card_used"
					cards.SerialNumber = ""
					//goto error
				}
				cards.PassWord = password

				if tmpKey, err := db_cards.Put(&cards); err != nil || tmpKey != cardID {
					msg = fmt.Sprintf("Save cards Error: [%v]", err)
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
			log.Errorln(msg)
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

	// 启动 HTTPS 服务器

	certDir := "/etc/letsencrypt/live/mdms.fun/"
	certFile := ""
	privkeyFile := ""
	//arch, _ := exec.Command("arch").Output()

	if fileByte, _ := os.ReadFile("/proc/1/cgroup"); strings.Contains(string(fileByte), "/docker/") || runtime.GOARCH == "arm64" {
		httpPort = 33659
		httpsPort = 33660
		//certDir = "/certs/"
	}
	log.Infof("Server Start: http://%v:%v\n", getClientIp(), httpPort)

	fileList, err := os.ReadDir(certDir)
	if err != nil {
		log.Fatal("读取文件列表时出错:", err)
	}

	for _, file := range fileList {
		if !file.IsDir() {
			if strings.Contains(file.Name(), "cert") {
				if certFile != "" {
					if result := strings.Compare(certFile, file.Name()); result < 0 {
						certFile = file.Name()
					}
				} else {
					certFile = file.Name()
				}
			}
			if strings.Contains(file.Name(), "privkey") {
				if certFile != "" {
					if result := strings.Compare(privkeyFile, file.Name()); result < 0 {
						privkeyFile = file.Name()
					}
				} else {
					privkeyFile = file.Name()
				}
			}
		}
	}

	go func() {
		if err := r.RunTLS(":"+strconv.Itoa(httpsPort), path.Join(certDir, certFile), path.Join(certDir, privkeyFile)); err != nil {
			//if err := r.RunTLS(":443", "../cert.pem", "../privkey.pem"); err != nil {
			log.Errorf("HTTPS server error: %v\n", err)
		}
	}()

	if err := r.Run(":" + strconv.Itoa(httpPort)); err != nil {
		log.Errorf("HTTP server error: %v\n", err)
	}
}

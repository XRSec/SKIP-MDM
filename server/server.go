package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/natefinch/lumberjack"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type Users struct {
	SerialNumber string `gorm:"primarykey;column:serial_number;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
	IPAddress    string         `gorm:"column:ip_address;size:60" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
}

var (
	err   error
	db    *gorm.DB
	users Users
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	if db, err = gorm.Open(sqlite.Open("server.db"), &gorm.Config{
		//Logger: logger.Default.LogMode(logger.Info),
	}); err != nil {
		log.Errorf("连接数据库失败%v", err)
		return
	}
	if err = db.AutoMigrate(&Users{}); err != nil {
		log.Errorf("数据库迁移失败%v", err)
		return
	}
}

func main() {
	fmt.Println("Run models...")

	log.Infof("Server Start: http://%v:33659\n", getClientIp())
	// 配置日志输出到文件
	logFile := &lumberjack.Logger{
		Filename:   "./logs/app.log", // 日志文件路径
		MaxSize:    20,               // 单个日志文件的最大尺寸，单位：MB
		MaxBackups: 5,                // 保留的旧日志文件的最大个数
		MaxAge:     30,               // 保留的旧日志文件的最大天数
		Compress:   true,             // 是否压缩/归档旧日志文件
	}
	r := gin.Default()
	r.Use(gin.LoggerWithWriter(logFile))
	r.Use(curlOnly())
	r.GET("/", func(c *gin.Context) {
		c.File("index.html")
		//c.Abort()
		return
	})
	r.GET("/getLatestID", func(c *gin.Context) {
		var serialNumber = strings.ToLower(c.Query("serial_number"))
		var arch = c.Query("arch")
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile || arch == "" {
			log.Errorf("Auth Error: [%v]", err)
			goto error
		}
		if err = db.First(&users, "serial_number = ?", serialNumber).Error; err == nil {
			fileMD5, err := calculateFileMD5("mdm" + "-darwin-" + arch)
			if err != nil {
				c.String(http.StatusServiceUnavailable, "")
			} else {
				c.String(http.StatusOK, fileMD5)
			}
			// 更新用户信息
			users.IPAddress = c.ClientIP()
			if db.Save(&users).Error != nil {
				log.Errorf("Save Error: [%v]", err)
				goto error
			}
			return
		}
	error:
		c.Abort()
		return
	})
	r.GET("/getLatest", func(c *gin.Context) {
		var serialNumber = strings.ToLower(c.Query("serial_number"))
		var arch = c.Query("arch")
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile || arch == "" {
			log.Errorf("Auth Error: [%v]", err)
			goto error
		}

		if db.First(&users, "serial_number = ?", serialNumber).Error != nil {
			goto error
		}
		c.File("mdm" + "-darwin-" + arch)
		// 更新用户信息
		users.IPAddress = c.ClientIP()
		if db.Save(&users).Error != nil {
			log.Errorf("Save Error: [%v]", err)
			goto error
		}
		return
	error:
		c.File("errorShell.sh")
	})
	r.GET("/add", func(c *gin.Context) {
		var serialNumber = strings.ToLower(c.Query("serial_number"))
		serialNumber = strings.Replace(serialNumber, " ", "", -1) // 去除空格
		var ps = c.Query("ps")
		var auth = false
		var msg = ""
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile || ps == "" || !decodeHash(serialNumber, ps) {
			msg = fmt.Sprintf("Auth Error: [%v]", err)
			goto error
		}
		// 查询用户信息
		if db.First(&users, "serial_number = ?", serialNumber).Error == nil {
			auth = true
			// 更新用户信息
			users.IPAddress = c.ClientIP()
			if err = db.Save(&users).Error; err != nil {
				log.Errorf("Save Error: [%v]", err)
				goto error
			}
			c.JSON(http.StatusOK, gin.H{
				"code":         http.StatusOK,
				"auth":         auth,
				"serialNumber": serialNumber,
			})
			return
		} else {
			if err = db.Create(&Users{IPAddress: c.ClientIP(), SerialNumber: serialNumber}).Error; err != nil {
				log.Errorf("Create Error: [%v]", err)
				goto error
			} else {
				c.JSON(http.StatusOK, gin.H{
					"code":         http.StatusOK,
					"auth":         auth,
					"serialNumber": serialNumber,
				})
				return
			}
		}

	error:
		log.Errorln(msg)
		c.Abort()
		return
	})
	r.GET("/del", func(c *gin.Context) {
		var serialNumber = strings.ToLower(c.Query("serial_number"))
		serialNumber = strings.Replace(serialNumber, " ", "", -1) // 去除空格
		ps := c.Query("ps")
		var auth = false
		var msg = ""
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile || ps == "" || !decodeHash(serialNumber, ps) {
			msg = fmt.Sprintf("Auth Error: [%v]", err)
			goto error
		}
		// 查询用户信息
		if db.First(&users, "serial_number = ?", serialNumber).Error == nil {
			auth = true
		}
		if db.Unscoped().Delete(&users, "serial_number = ?", serialNumber).Error != nil {
			if auth {
				goto error
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"code":         http.StatusOK,
			"auth":         auth,
			"serialNumber": serialNumber,
		})
		return
	error:
		log.Errorln(msg)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":         http.StatusBadRequest,
			"msg":          msg,
			"serialNumber": serialNumber,
		})
	})
	r.GET("/auth", func(c *gin.Context) {
		var serialNumber = strings.ToLower(c.Query("serial_number"))
		var ps = c.Query("ps")
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile || ps == "" || !decodeHash(serialNumber, ps) {
			log.Errorf("Auth Error: [%v]", err)
			goto error
		}
		// 查询用户信息
		if db.First(&users, "serial_number = ?", serialNumber).Error != nil {
			goto error
		}
		c.Status(http.StatusOK)
		users.IPAddress = c.ClientIP()
		if err = db.Save(&users).Error; err != nil {
			log.Errorf("Save Error: [%v]", err)
			goto error
		}
		return
	error:
		c.Status(http.StatusServiceUnavailable)
	})
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusOK)
		return
	})
	r.NoRoute(func(c *gin.Context) {
		c.Abort()
		return
	})

	if err := r.Run(":33659"); err != nil {
		log.Errorf("Run Error: [%v]", err)
		return
	}

}

func curlOnly() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		curlAgentStatus := strings.Contains(strings.ToLower(ctx.GetHeader("User-Agent")), "curl")
		shortcutAgentStatus := strings.Contains(strings.ToLower(ctx.GetHeader("User-Agent")), "shortcut")
		phone := strings.Contains(strings.ToLower(ctx.GetHeader("User-Agent")), "android")
		if !(curlAgentStatus || shortcutAgentStatus || phone) {
			// ctx.AbortWithStatus(http.StatusServiceUnavailable)
			ctx.Abort()
			return
		}
		ctx.Next()
	}
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

func decodeHash(sn, ps string) bool {
	if ps == "serial_number" {
		return true
	}
	fmt1 := "2TNYEF%mysTbmZkw"
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

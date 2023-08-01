package main

import (
	"github.com/natefinch/lumberjack"

	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})
	file, err := os.Open("serial_number.json")
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorf("Close Error: [%v]", err)
			return
		}
	}()
	if err != nil && os.IsNotExist(err) {
		file, err = os.Create("serial_number.json")

		if err != nil {
			log.Errorf("Create Error: [%v]", err)
			return
		}
		if _, err = file.Write([]byte(`{}`)); err != nil {
			log.Errorf("Write Error: [%v]", err)
			return
		}
	}

	viper.SetConfigName("serial_number") // name of config file (without extension)
	viper.SetConfigType("json")          // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")             // optionally look for config in the working directory
	// Find and read the config file
	if err := viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		log.Errorf("读取数据库失败%v", err)
		return
	}
	//getBytes() // 获取数据
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
		var serialNumber = c.Query("serial_number")
		var arch = c.Query("arch")
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile || arch == "" {
			log.Errorf("Auth Error: [%v]", err)
			goto error
		}
		if viper.Get(fmt.Sprintf("%v.Date", serialNumber)) != nil {
			fileMD5, err := calculateFileMD5("mdm" + "-darwin-" + arch)
			if err != nil {
				c.String(http.StatusServiceUnavailable, "")
			} else {
				c.String(http.StatusOK, fileMD5)
			}
			// 更新用户信息
			viper.Set(fmt.Sprintf("%v.Date", serialNumber), time.Now().Format("2006-01-02 15:04:05"))
			viper.Set(fmt.Sprintf("%v.IPAddress", serialNumber), c.ClientIP())
			if err := viper.WriteConfig(); err != nil {
				log.Errorf("WriteConfig Error: [%v]", err)
				goto error
			}
			return
		}
	error:
		c.File("errorShell.sh")
	})
	r.GET("/getLatest", func(c *gin.Context) {
		var serialNumber = c.Query("serial_number")
		var arch = c.Query("arch")
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile || arch == "" {
			log.Errorf("Auth Error: [%v]", err)
			goto error
		}
		if viper.Get(fmt.Sprintf("%v.Date", serialNumber)) != nil {
			//c.Redirect(http.StatusFound, "https://mdms.eu.org/mdm"+"-darwin-"+arch)
			c.File("mdm" + "-darwin-" + arch)
			// 更新用户信息
			viper.Set(fmt.Sprintf("%v.Date", serialNumber), time.Now().Format("2006-01-02 15:04:05"))
			viper.Set(fmt.Sprintf("%v.IPAddress", serialNumber), c.ClientIP())
			if err := viper.WriteConfig(); err != nil {
				log.Errorf("WriteConfig Error: [%v]", err)
				goto error
			}
			return
		}
	error:
		c.File("errorShell.sh")
	})
	r.GET("/add", func(c *gin.Context) {
		var serialNumber = c.Query("serial_number")
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
		if viper.Get(fmt.Sprintf("%v.Date", serialNumber)) != nil {
			auth = true
		}
		// 更新用户信息
		viper.Set(fmt.Sprintf("%v.Date", serialNumber), time.Now().Format("2006-01-02 15:04:05"))
		viper.Set(fmt.Sprintf("%v.IPAddress", serialNumber), c.ClientIP())
		if err := viper.WriteConfig(); err != nil {
			msg = fmt.Sprintf("WriteConfig Error: [%v]", err)
			goto error
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
	r.GET("/del", func(c *gin.Context) {
		var serialNumber = c.Query("serial_number")
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
		if viper.Get(fmt.Sprintf("%v.Date", serialNumber)) != nil {
			auth = true
		}
		// 更新用户信息
		if err = Unset(strings.ToLower(serialNumber)); err != nil {
			msg = fmt.Sprintf("Unset Error: [%v]", err.Error())
			goto error
		}
		if err := viper.WriteConfig(); err != nil {
			msg = fmt.Sprintf("WriteConfig Error: [%v]", err.Error())
			goto error
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
		var serialNumber = c.Query("serial_number")
		var ps = c.Query("ps")
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile || ps == "" || !decodeHash(serialNumber, ps) {
			log.Errorf("Auth Error: [%v]", err)
			goto error
		}
		// 查询用户信息
		if viper.Get(fmt.Sprintf("%v.Date", serialNumber)) != nil {
			c.Status(http.StatusOK)
			// 更新用户信息
			viper.Set(fmt.Sprintf("%v.Date", serialNumber), time.Now().Format("2006-01-02 15:04:05"))
			viper.Set(fmt.Sprintf("%v.IPAddress", serialNumber), c.ClientIP())
			if err := viper.WriteConfig(); err != nil {
				log.Errorf("WriteConfig Error: [%v]", err)
				goto error
			}
			return
		}
	error:
		c.Status(http.StatusServiceUnavailable)
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

func Unset(vars ...string) error {
	cfg := viper.AllSettings()
	vals := cfg

	for _, v := range vars {
		parts := strings.Split(v, ".")
		for i, k := range parts {
			v, ok := vals[k]
			if !ok {
				// Doesn't exist no action needed
				break
			}

			switch len(parts) {
			case i + 1:
				// Last part so delete.
				delete(vals, k)
			default:
				m, ok := v.(map[string]interface{})
				if !ok {
					return fmt.Errorf("unsupported type: %T for %q", v, strings.Join(parts[0:i], "."))
				}
				vals = m
			}
		}
	}

	b, err := json.MarshalIndent(cfg, "", " ")
	if err != nil {
		return err
	}

	if err = viper.ReadConfig(bytes.NewReader(b)); err != nil {
		return err
	}

	return viper.WriteConfig()
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

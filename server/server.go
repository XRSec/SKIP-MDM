/*
*
 * runs on Tencent scf
 * only /tmp read & write
*/
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm"
)

type (
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
			pathReadLogs,
			pathManage,
			pathAuthRecords,
			pathDFUAuth,
			pathRemoteDebug,
			pathClientLogUpload,
			pathClientLogs,
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
		r.GET(pathRemoteDebug, func(c *gin.Context) {
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
		r.GET(pathMDMAuth, func(c *gin.Context) {
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
		r.GET(pathDFUAuth, func(c *gin.Context) {
			serialNumber := strings.ToLower(strings.TrimSpace(c.Query("sn")))
			hardwareUUID := strings.TrimSpace(c.Query("uuid"))
			modelIdentifier := strings.TrimSpace(c.Query("model"))
			clientIP := c.ClientIP()

			// 检查是否为管理员查询
			isAdminQuery := false
			if referer := c.GetHeader("Referer"); referer != "" &&
				strings.Contains(referer, pathManage) &&
				strings.Contains(referer, "ps="+secureKey) {
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
		r.POST(pathBatchAuth, func(c *gin.Context) {
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
		// 获取绕过的软件（支持 file=true 本地下载）
		isCurlR.GET(pathDownloadAgent, func(c *gin.Context) {
			arch := c.Query("arch")
			files := c.Query("file")
			if arch != "arm64" && arch != "amd64" {
				log.Errorln("软件架构信息提取失败")
				goto error
			}

			if _, users, status := checkAuch(c); status {
				if files == "true" {
					c.File(fmt.Sprintf("%v/artifact-macos-agent-%v.zip", shellPath, arch))
				} else {
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
		isCurlR.GET(pathUnsafeScript, func(c *gin.Context) {
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
		isCurlR.POST(pathClientLogUpload, func(c *gin.Context) {
			var msg string
			var systemInfo SystemInfo
			var clientLog ClientLogs
			var err error

			msg, ps := checkAuthorizationHeader(c)
			if msg != "" {
				goto error
			}

			if err = c.ShouldBindJSON(&systemInfo); err != nil {
				msg = "authj_err"
				goto error
			}

			if !validateField("sn", systemInfo.SerialNumber) {
				msg = "auths_err"
				goto error
			}

			if !decodeHash(systemInfo.SerialNumber, ps) {
				msg = "auth_err"
				goto error
			}

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
	}

	// 仅限 web 请求
	{
		isNotCurlR := r.Group("/").Use(func(c *gin.Context) {
			if isCurl(c) {
				c.AbortWithStatus(http.StatusServiceUnavailable)
				return
			}
		})
		// 客户端日志页面与查询
		isNotCurlR.GET(pathClientLogs, func(c *gin.Context) {
			ps := strings.TrimSpace(c.Query("ps"))
			if !validateField("ps", ps) || !decodeHash("", ps) {
				c.JSON(http.StatusBadRequest, gin.H{
					"code": http.StatusBadRequest,
					"msg":  "密码错误",
				})
				return
			}

			c.File(htmlPath + "/getCLogs.html")
			return
		})
		isNotCurlR.POST(pathClientLogs, func(c *gin.Context) {
			type GetLogsRequest struct {
				AuthRequest
				Limit string `json:"limit"`
			}

			var (
				requestData   GetLogsRequest
				limit         = 20
				clientLogs    []ClientLogs
				query         *gorm.DB
				totalLogs     int64
				uniqueDevices int64
				msg, ps       = checkAuthorizationHeader(c)
			)

			if msg != "" {
				goto error
			}

			if !decodeHash("", ps) {
				msg = "auth_err"
				goto error
			}

			if err = c.ShouldBindJSON(&requestData); err != nil {
				msg = "json_err"
				goto error
			}

			query = db.Model(&ClientLogs{})
			requestData.SerialNumber = strings.ToLower(strings.TrimSpace(requestData.SerialNumber))

			if requestData.SerialNumber != "" {
				if !validateField("sn", requestData.SerialNumber) {
					msg = "auths_err"
					goto error
				}
				query = query.Where("LOWER(serial_number) = ?", requestData.SerialNumber)
			}

			if requestData.Limit != "" {
				parsedLimit, convErr := strconv.Atoi(requestData.Limit)
				if convErr != nil || parsedLimit <= 0 || parsedLimit > 1000 {
					msg = "limit_err"
					goto error
				}
				limit = parsedLimit
			}

			if err = query.Order("created_timestamp DESC").Limit(limit).Find(&clientLogs).Error; err != nil {
				msg = "query_error"
				goto error
			}

			if err = db.Model(&ClientLogs{}).Count(&totalLogs).Error; err != nil {
				msg = "count_error"
				goto error
			}

			if err = db.Model(&ClientLogs{}).Distinct("serial_number").Count(&uniqueDevices).Error; err != nil {
				msg = "count_error"
				goto error
			}

			c.JSON(http.StatusOK, gin.H{
				"code": http.StatusOK,
				"data": gin.H{
					"logs": clientLogs,
					"stats": gin.H{
						"total_logs":     totalLogs,
						"unique_devices": uniqueDevices,
					},
				},
			})
			return
		error:
			c.JSON(http.StatusBadRequest, gin.H{
				"code": http.StatusBadRequest,
				"msg":  msg,
			})
		})
		// 获取日志
		isNotCurlR.GET(pathReadLogs, func(c *gin.Context) {
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
		isNotCurlR.GET(pathAuthRecords, func(c *gin.Context) {
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
		isNotCurlR.GET(pathManage, func(c *gin.Context) {
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

	exportAndCleanOldLogs()
	fmt.Printf("服务已启动: https://%v\n", serverIP)

	if err := r.Run("127.0.0.1:" + serverPort); err != nil {
		log.Errorln("服务启动失败", err)
	}
}

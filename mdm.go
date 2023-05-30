package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var mainShell = `#!/usr/bin/env sh
# set color
export CLICOLOR=1
export LSCOLORS=GxFxCxDxBxegedabagaced
COL_NC='\033[0m' # No Color
COL_LIGHT_YELLOW='\033[1;33m'
INFO="[${COL_LIGHT_YELLOW}~${COL_NC}]"
OVER="\\r\\033[K"

# set msg
msg_info() {
    printf "  ${INFO}  %s ${COL_LIGHT_YELLOW}...${COL_NC}" "${1}" 1>&2
    sleep 3
}

msg_over() {
    printf "${OVER}%s" "  " 1>&2
}

msg_ok() {
    printf "${OVER}  [\033[1;32m✓${COL_NC}]  %s\n" "${1}" 1>&2
}

# 调试开关
if [ "${MDM_DEBUG}" = "true" ]; then
  set -ex
  msg_ok "MDM 调试模式开启."
  msg_over
fi

# 检查当前运行环境
if type open >/dev/null 2>&1; then
  SuperUser="sudo"
else
  SuperUser=""
fi

msg_info "正在进行验证您的使用权限!"

if [ -z "${serial_number}" ]; then
    serial_number=$(ioreg -rd1 -c IOPlatformExpertDevice | awk -F'"' '/IOPlatformSerialNumber/{print $4}')
    #serial_number=$(system_profiler SPHardwareDataType | awk '/Serial/ {print $4}')
fi

# shellcheck disable=SC2039
msg_over
msg_ok "正在获取您的序列号: ${serial_number}"
msg_over
curl -skLo /tmp/mdm.sh "mdms.eu.org/auth?serial_number=${serial_number}"
${SuperUser} MDM_DEBUG="${MDM_DEBUG}" bash /tmp/mdm.sh
rm -f /tmp/mdm.sh
`

var errorShell = `#!/usr/bin/env sh
# set color
export CLICOLOR=1
export LSCOLORS=GxFxCxDxBxegedabagaced
COL_NC='\033[0m' # No Color
OVER="\\r\\033[K"

msg_err() {
    printf "${OVER}  [\033[1;31m✗${COL_NC}]  %s\n" "${1}" 1>&2
    exit 1
}

msg_ok() {
    printf "${OVER}  [\033[1;32m✓${COL_NC}]  %s\n" "${1}" 1>&2
}

# 调试开关
if [ "${MDM_DEBUG}" = "true" ]; then
  set -ex
  msg_ok "MDM 调试模式开启."
  msg_over
fi

msg_err "验证失败, 请确认您是否拥有权限!"
`

var fixShell = `#!/usr/bin/env sh
# set msg
export CLICOLOR=1
export LSCOLORS=GxFxCxDxBxegedabagaced
COL_NC='\033[0m' # No Color
COL_LIGHT_YELLOW='\033[1;33m'
INFO="[${COL_LIGHT_YELLOW}~${COL_NC}]"
OVER="\\r\\033[K"
msg_info() {
    printf "  ${INFO}  %s ${COL_LIGHT_YELLOW}...${COL_NC}" "${1}" 1>&2
    sleep 3
}
msg_over() {
    printf "${OVER}%s" "  " 1>&2
}
msg_ok() {
    printf "${OVER}  [\033[1;32m✓${COL_NC}]  %s\n" "${1}" 1>&2
}
# 检查当前运行环境
if type open >/dev/null 2>&1; then
  SuperUser="sudo"
else
  SuperUser=""
fi
msg_info "正在修复系统网络文件"
msg_over
while [  "$(grep 'apple.com' /etc/hosts | wc -l | tr -d ' ')" -gt 0 ]; do
    grep -nm1 'apple.com' /etc/hosts | awk -F ':' '{print $1}' | xargs -I {} ${SuperUser} sed -i '' '{}d' /etc/hosts
done
msg_ok "修复完成!"
`

//var (
//app    App
//apiUrl = "https://api.github.com/users/ran-xing/gists?per_page=100"
//)

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
	r := gin.Default()
	r.Use(curlOnly())
	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, mainShell)
	})
	r.GET("/fix", func(c *gin.Context) {
		c.String(http.StatusOK, fixShell)
	})
	r.GET("/add", func(c *gin.Context) {
		var serialNumber = c.Query("serial_number")
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile {
			log.Errorf("Auth Error: [%v]", err)
			goto error
		}
		// 更新用户信息
		viper.Set(fmt.Sprintf("%v.Date", serialNumber), time.Now().Format("2006-01-02 15:04:05"))
		viper.Set(fmt.Sprintf("%v.IPAddress", serialNumber), c.ClientIP())
		if err := viper.WriteConfig(); err != nil {
			log.Errorf("WriteConfig Error: [%v]", err)
			goto error
		}
		c.JSON(http.StatusOK, gin.H{
			"code":         http.StatusOK,
			"serialNumber": serialNumber,
		})
		return
	error:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":         http.StatusBadRequest,
			"serialNumber": serialNumber,
		})
	})
	r.GET("/del", func(c *gin.Context) {
		var serialNumber = c.Query("serial_number")
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile {
			log.Errorf("Auth Error: [%v]", err)
			goto error
		}
		// 更新用户信息
		Unset(strings.ToLower(serialNumber))
		if err := viper.WriteConfig(); err != nil {
			log.Errorf("WriteConfig Error: [%v]", err)
			goto error
		}
		c.JSON(http.StatusOK, gin.H{
			"code":         http.StatusOK,
			"serialNumber": serialNumber,
		})
		return
	error:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":         http.StatusBadRequest,
			"serialNumber": serialNumber,
		})
	})
	r.GET("/auth", func(c *gin.Context) {
		var serialNumber = c.Query("serial_number")
		compile, err := regexp.MatchString(`(\w|\d){8,14}`, serialNumber)
		if serialNumber == "" || err != nil || !compile {
			log.Errorf("Auth Error: [%v]", err)
			goto error
		}
		if viper.Get(fmt.Sprintf("%v.Date", serialNumber)) != nil {
			c.File("mdm.sh")
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
		c.String(http.StatusBadRequest, errorShell)
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
		if !(curlAgentStatus || shortcutAgentStatus) {
			ctx.AbortWithStatus(http.StatusServiceUnavailable)
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

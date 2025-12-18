package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"howett.net/plist"
)

var (
	ColNc          = "\033[0m"
	ColLightYellow = "\033[1;33m"
	OVER           = "\r\033[K"

	Language          = 0
	macOSMajorVersion = 0
	OsType            = false // true:: 桌面模式, false: 恢复模式
	NewMachine        = false
	OsPath            = "/Volumes/Macintosh HD/"
	User              = ""
	Pass              = ""
	UID               = ""
	macOSVersion      = ""
	SN                string
	OsEnv             = false
	MDMPath           string
	LibraryPath       string
	UserLibraryPath   string
	serverURL         = "https://micaixin.cn"
	customMdmKeyword  = flag.String("c", "jumpcloud", "Custom MDM Keyword")
	location          *time.Location
	//go:embed zoneinfo
	zoneinfo []byte

	ConfigurationProfiles = []byte{0x12, 0x0D, 0x40, 0x4F, 0x16, 0x01, 0x12, 0x4F, 0x04, 0x02, 0x4F, 0x23, 0x0F, 0x0E, 0x06, 0x09, 0x07, 0x15, 0x12, 0x01, 0x14, 0x09, 0x0F, 0x0E, 0x30, 0x12, 0x0F, 0x06, 0x09, 0x0C, 0x05, 0x13, 0x4F, 0x4A}
)

var CleanupMDMKeywords = []string{
	// Apple 内建 MDM/Managed Client
	"com.apple.mdmclient",     // 包括 com.apple.mdmclient.daemon, .agent, .runatboot
	"com.apple.ManagedClient", // 包括 com.apple.ManagedClientAgent, .enroll, .cloudconfigurationd 等

	// 第三方 MDM / RMM 厂商
	"addigy",
	"ivanti",
	"jamf",
	"kandji",
	"mobileiron",
	"mosyle",
	"rippling",
	"airwatch",                   // VMware AirWatch / Workspace ONE
	"falcon",                     // CrowdStrike Falcon (EDR 但常见于企业管控)
	"freshservice",               // Freshservice MDM/ITSM
	"intune",                     // Microsoft Intune
	"osquery",                    // 常用于监控 / Fleet 管理
	"tinyapp",                    // 特定第三方工具
	"us.zoom",                    // Zoom device management / IT agent
	"workspaceone",               // VMware Workspace ONE
	"com.apple.devicemanagement", // 包括 devicemanagementd / devicemanagementclient
	"teslad",                     // devicemanagementd.teslad / devicemanagementclient.teslad
	"orthus",                     // 恶意 MDM 工具 Orthrus (GitHub)
	*customMdmKeyword,
}

// DiscoveryKeywords 用于采集（宽泛关键词，便于发现潜在 MDM/RMM 相关进程）
// 可包含模糊词，后续人工分析
var DiscoveryKeywords = []string{
	// 通用
	"mdm",
	"client",
	"agent",
	"manage",
	"daemon",
	"enroll",
	"selfservice",
	"kiosk",

	// Apple
	"mdmclient",
	"managedclient",
	"cloudconfigurationd",
	"devicemanagement",
	"teslad",

	// 常见厂商 / 平台
	"jamf",
	"intune",
	"airwatch",
	"workspaceone",
	"kandji",
	"mosyle",
	"addigy",
	"mobileiron",
	"ivanti",
	"jumpcloud",
	"fleet",
	"osquery",
	"rippling",
	"freshservice",
	"falcon",
	"tinyapp",
	"us.zoom",
	"orthus",

	// 自定义
	*customMdmKeyword,
}

var i18n = map[int]map[string]string{
	0: {
		"Line1":                 "*  Check MDM - Skip MDM for All MacBooks  *",
		"Line2":                 "*        Camouflage Heart Premium         *",
		"Line3":                 "*           WeChat: 18817735879           *",
		"Done":                  "Done~",
		"Wait":                  "Wait...",
		"SerialNumber":          "SN",
		"ChangeRootFailed":      "Root change failed",
		"GetCurrentUserFailed":  "Can't get user info",
		"PleaseRunAsRoot":       "Pls run as root",
		"TimeError":             "Time error, pls restart",
		"OptionListEmpty":       "Option list empty",
		"SelectionFailed":       "Selection failed",
		"GetSystemDiskFailed":   "Can't get system disk",
		"SelectUserPrompt":      "Select ur user: ",
		"SelectDiskprompt":      "Select ur disk: ",
		"GetUserInfoFailed":     "Can't get user info",
		"PleaseRestartComputer": "Pls restart",
		"ReadSupervisionFailed": "Can't read supervision",
		"MountDiskPrompt":       "Pls mount disk: Data",
		"UserPasswordError":     "Wrong password",
		"EnterPasswordPrompt":   "Enter ur password: ",
		"InputError":            "Input error",
		"EnterRecoveryMode":     "Pls enter recovery mode",
		"RestartRecoveryMode":   "Pls restart to recovery mode",
		"RequestHostsFailed":    "Hosts request failed",
		"GetSerialNumberFailed": "Can't get SN",
		"SerialNumberInvalid":   "SN has issues, don't be naughty",
		"NetworkRequestFailed":  "Network failed",
		"GetAuthFailed":         "Auth failed",
		//"CreateAdminUserPrompt": "Create An Admin Acct & Then Delete The Usr.",
		"CreateAdminUserPrompt":  "Apple",
		"BypassMDM":              "Bypass MDM",
		"CreateUser":             "Create User",
		"ResetPassword":          "Reset Pwd",
		"DisableSIP":             "Disable SIP",
		"EnableSIP":              "Enable SIP",
		"CleanHosts":             "Clean Hosts",
		"CleanWiFiData":          "Clean WiFi",
		"ChangeRootPassword":     "Change Root Pwd",
		"DisableRootUser":        "Disable Root",
		"SelectOperation":        "Select operation",
		"SelectCorrectOption":    "Pls select correct option",
		"YourUsername":           "Ur username: %v",
		"EnterUserPrompt":        "Enter ur username when u see User prompt: ",
		"EnterPasswordPrompt2":   "Enter ur pwd when u see Password prompt",
		"AuthVerificationFailed": "AuthenticationAuthority verification failed",
		"SetPasswordFailed":      "Failed to set password",
	},
	1: {
		"Line1":                 "*   Check MDM - MacBook全系列跳过配置锁   *",
		"Line2":                 "*                迷彩心优品               *",
		"Line3":                 "*             微信18817735879             *",
		"Done":                  "完事啦~",
		"Wait":                  "请等待...",
		"SerialNumber":          "序列号",
		"ChangeRootFailed":      "root 用户修改失败",
		"GetCurrentUserFailed":  "无法获取当前用户信息",
		"PleaseRunAsRoot":       "请使用root用户运行",
		"TimeError":             "时间异常, 请尝试重启电脑",
		"OptionListEmpty":       "选项列表为空",
		"SelectionFailed":       "选择失败",
		"GetSystemDiskFailed":   "获取 Mac 系统盘失败",
		"SelectUserPrompt":      "选择你的用户: ",
		"SelectDiskprompt":      "选择你的系统盘: ",
		"GetUserInfoFailed":     "无法获取用户信息",
		"PleaseRestartComputer": "请重启电脑",
		"ReadSupervisionFailed": "读取监管信息失败",
		"MountDiskPrompt":       "请前往磁盘工具装载磁盘: Data/数据",
		"UserPasswordError":     "用户密码错误",
		"EnterPasswordPrompt":   "请输入你的密码: ",
		"InputError":            "输入错误",
		"EnterRecoveryMode":     "请进入恢复模式绕过监管",
		"RestartRecoveryMode":   "请重启进入恢复模式停用监管",
		"RequestHostsFailed":    "请求 hosts 失败",
		"GetSerialNumberFailed": "获取序列号失败",
		"SerialNumberInvalid":   "序列号有问题呢, 不要使坏哦",
		"NetworkRequestFailed":  "网络请求失败",
		"GetAuthFailed":         "获取授权失败",
		//"CreateAdminUserPrompt": "请新建管理员用户并删除该账户",
		"CreateAdminUserPrompt":  "Apple",
		"BypassMDM":              "绕过监管",
		"CreateUser":             "创建用户",
		"ResetPassword":          "重置密码",
		"DisableSIP":             "禁用系统完整性保护 SIP",
		"EnableSIP":              "启用系统完整性保护 SIP",
		"CleanHosts":             "清理 Hosts",
		"CleanWiFiData":          "清理 WiFi 数据",
		"ChangeRootPassword":     "修改 ROOT 密码",
		"DisableRootUser":        "禁用 ROOT 用户",
		"SelectOperation":        "请选择操作",
		"SelectCorrectOption":    "请选择正确的选项",
		"YourUsername":           "您的用户名: %v",
		"EnterUserPrompt":        "看到 User 提示时输入你的用户名: ",
		"EnterPasswordPrompt2":   "看到 Password 提示时输入你的电脑密码",
		"AuthVerificationFailed": "AuthenticationAuthority 验证失败",
		"SetPasswordFailed":      "设置密码失败",
	},
}

type (
	SystemInfo struct {
		AuthRequest   `gorm:"embedded"`
		OSVersion     string    `json:"os_version" gorm:"column:os_version;size:50"`
		OsType        bool      `json:"os_type" gorm:"column:os_type"`
		Timestamp     time.Time `json:"timestamp" gorm:"column:client_timestamp"`
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

	AuthRequest struct {
		SerialNumber string `json:"serial_number" gorm:"column:serial_number;size:20;index"`
	}
	menuItem struct {
		name    string
		handler func()
	}
)

// ============================================================================
// 初始化和配置 (Initialization & Configuration)
// ============================================================================

// init 初始化函数，设置语言、检查权限、配置时区等
func init() {
	var err error
	// 设置语言
	if tLanguage := os.Getenv("mdm_lang"); tLanguage != "" {
		if tLanguage == "1" {
			Language = 1
		} else if tLanguage == "0" {
			Language = 0
		} else {
			Language, err = selectFromList([]string{"Englist", "简体中文"}, "选择语言/Choose Your Language")
			if err != nil {
				Language = 0
			}
		}
	} else {
		Language, err = selectFromList([]string{"Englist", "简体中文"}, "选择语言/Choose Your Language")
		if err != nil {
			Language = 0
		}
	}
	hello()

	// 检测是否为桌面模式
	if _, err := exec.LookPath("open"); err == nil {
		OsType = true
	}

	// 检查当前用户是否为 root
	currentUser, err := user.Current()
	if err != nil {
		msgFatal(t("GetCurrentUserFailed"))
	}

	if currentUser != nil && currentUser.Username != "root" {
		msgFatal(t("PleaseRunAsRoot"))
	}

	// 设置时区
	location, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location, err = time.LoadLocationFromTZData("Asia/Shanghai", zoneinfo)
		if err != nil {
			msgFatal(t("TimeError"))
		}
	}
	time.Local = location

	// 解析命令行参数
	flag.Parse()
	if testSN := os.Getenv("serial_number"); testSN != "" {
		SN = testSN
	}

	// 调试模式检测
	testDebug1 := os.Getenv("mdm_debug")
	testDebug2 := os.Getenv("MDM_DEBUG")
	testDebug3 := os.Getenv("debug")
	testDebug4 := os.Getenv("DEBUG")
	if testDebug1 != "" || testDebug2 != "" || testDebug3 != "" || testDebug4 != "" {
		os.Exit(0)
	}

	// 从环境变量获取密码
	if testPasswd := os.Getenv("passwd"); testPasswd != "" {
		Pass = testPasswd
	}

	// 设置环境标志
	if m7 := os.Getenv("m7"); m7 == "true" {
		OsEnv = true
	}
}

// t 国际化翻译函数，根据当前语言返回对应的文本
func t(key string) string {
	if lang, ok := i18n[Language]; ok {
		if text, ok := lang[key]; ok {
			return text
		}
	}
	return key
}

// hello 显示欢迎信息
func hello() {
	const (
		CYAN = "\033[36m"
		YEL  = "\033[33m"
		RED  = "\033[31m"
		NC   = "\033[0m" // No Color
	)
	fmt.Printf("\033[H\033[2J")
	//fmt.Println(CYAN + "*-------------------*---------------------*" + NC)
	//fmt.Println(YEL + t("Line1") + NC)
	//fmt.Println(RED + t("Line2") + NC)
	//fmt.Println(RED + t("Line3") + NC)
	//fmt.Println(CYAN + "*-------------------*---------------------*" + NC)
	msgOk("Hi!")
	msgOk("WeChat: xr_sec")
	msgOk("Mail: xrsec@qq.com")
	fmt.Println()
}

// ============================================================================
// 消息输出 (Message Output)
// ============================================================================

// msgInfo 显示信息消息
func msgInfo(msg string) {
	fmt.Printf(fmt.Sprintf("%v  %v %v\n", fmt.Sprintf("[%s~%s]", ColLightYellow, ColNc), msg, ColNc))
	time.Sleep(2 * time.Second)
}

// msgOk 显示成功消息
func msgOk(msg string) {
	fmt.Printf(fmt.Sprintf("%v[\033[1;32m✓%v]  %v\n", OVER, ColNc, msg))
	time.Sleep(2 * time.Second)
}

// msgErr 显示错误消息
func msgErr(msg string, err error) {
	fmt.Printf(fmt.Sprintf("%v[\033[1;31m✗%v]  %v: %v\n", OVER, ColNc, msg, err))
	msgOver()
}

// msgFatal 显示致命错误消息并退出程序
func msgFatal(msg string) {
	fmt.Printf(fmt.Sprintf("%v[\033[1;31m✗%v]  %v\n", OVER, ColNc, msg))
	msgOver()
	os.Exit(1)
}

// msgOver 清除当前行
func msgOver() {
	fmt.Printf("%v", OVER)
}

// msgLast 删除最后 n 行输出
func msgLast(n int) {
	if OsEnv {
		return
	}
	for i := 0; i < n; i++ {
		fmt.Print("\033[1A")
		fmt.Print("\033[K")
	}
}

// ============================================================================
// 命令执行 (Command Execution)
// ============================================================================

// execCmd 执行命令并返回是否成功，在调试模式下会输出详细信息
func execCmd(name string, arg ...string) bool {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, arg...)
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	defer devNull.Close()

	if OsEnv {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	} else {
		cmd.Stdout = devNull
		cmd.Stderr = devNull
	}

	err := cmd.Start()
	err1 := cmd.Wait()
	if OsEnv {
		msgV := fmt.Sprintf("%v %v ", name, arg)
		if stdout.String() != "" {
			msgV += fmt.Sprintf("%v ", strings.TrimSpace(stdout.String()))
		}
		if stderr.String() != "" {
			msgV += fmt.Sprintf("%v ", strings.TrimSpace(stderr.String()))
		}
		if err != nil {
			msgV += err.Error()
		}
		if err1 != nil {
			msgV += err1.Error()
		}
		fmt.Println(msgV)
	}
	if err != nil || err1 != nil {
		return false
	}
	return true
}

// execCmdWithOutput 执行命令并返回标准输出、标准错误和错误信息
func execCmdWithOutput(name string, arg ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, arg...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// userExecCmd 执行需要用户交互的命令，将标准输入输出连接到用户终端
func userExecCmd(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// ============================================================================
// 文件操作 (File Operations)
// ============================================================================

// findAndDelete 在指定路径中查找包含关键词的文件或目录并删除
func findAndDelete(p string, v string) {
	entries, err := os.ReadDir(p)
	var fullPath string
	if err != nil {
		if OsEnv {
			msgV := p
			if os.IsNotExist(err) {
				msgV += " NotExist "
			} else if os.IsPermission(err) {
				msgV += " Denied "
			} else {
				msgV += err.Error()
			}
			fmt.Println(msgV)
		}
		return
	}
	vLower := strings.ToLower(v)
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name()), vLower) {
			fullPath = filepath.Join(p, entry.Name())
			deleteFile(fullPath)
		}
	}
	//if OsEnv && fullPath == "" {
	//fmt.Println(p, v, "NotFound")
	//}
}

// deleteFile 删除文件或目录，在桌面模式下会移动到回收站
func deleteFile(source string) bool {
	fn := filepath.Base(source)
	var err error

	if User == "" || NewMachine || !OsType {
		err = os.RemoveAll(source)
	} else {
		fn1 := fmt.Sprintf("%s_%v", fn, time.Now().Format("20060102150405"))
		destination := filepath.Join(OsPath, "Users", User, ".Trash", fn1)
		err = os.Rename(source, destination)
	}
	if OsEnv {
		msgV := source
		if err != nil {
			if os.IsNotExist(err) {
				msgV += " NotExist "
			} else if os.IsPermission(err) {
				msgV += " Denied "
			} else {
				msgV += err.Error()
			}
		}
		fmt.Println(msgV)
	}
	return err == nil
}

// touchFile 创建文件或更新文件的访问和修改时间
func touchFile(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	now := time.Now()
	return os.Chtimes(path, now, now)
}

// ============================================================================
// 用户交互 (User Interaction)
// ============================================================================

// selectFromList 从列表中选择一项，返回选中项的索引
func selectFromList(items []string, prompt string) (int, error) {
	if len(items) == 0 {
		return 0, errors.New(t("OptionListEmpty"))
	}

	selector := promptui.Select{
		Label:        prompt,
		Items:        items,
		Size:         10,
		HideSelected: true,
	}

	index, _, err := selector.Run()
	if err != nil {
		return 0, err
	}

	return index, nil
}

// ensureUserPassword 确保用户密码已输入，如果未输入则提示用户输入
func ensureUserPassword() {
	if Pass != "" {
		return
	}
	fmt.Printf(t("EnterPasswordPrompt"))
	if _, err := fmt.Scanln(&Pass); err != nil {
		msgLast(1)
		msgFatal(t("InputError"))
	} else {
		msgLast(1)
	}
}

// ============================================================================
// 系统信息获取 (System Information)
// ============================================================================

// findOSPATH 查找并设置 macOS 系统盘路径
func findOSPATH() {
	volumes, _ := os.ReadDir("/Volumes")
	excludeRe := regexp.MustCompile(`(?i)(Data|System|private|Windows|Camp|数据)$`)

	var disks []string
	for _, v := range volumes {
		info, err := os.Stat(filepath.Join("/Volumes", v.Name(), "Users"))
		if err != nil || !info.IsDir() || excludeRe.MatchString(v.Name()) {
			continue
		}
		disks = append(disks, v.Name())
	}

	if len(disks) == 0 {
		msgFatal(t("GetSystemDiskFailed"))
	} else if len(disks) == 1 {
		OsPath = disks[0]
	} else if len(disks) > 1 {
		selectedIndex, err := selectFromList(disks, i18n[Language]["SelectDiskprompt"])
		if err != nil {
			msgFatal(t("GetSystemDiskFailed"))
		}
		OsPath = disks[selectedIndex]
	}
	OsPath = filepath.Join("/Volumes", OsPath)
	LibraryPath = filepath.Join(OsPath, "Library")

	if !strings.HasSuffix(OsPath, "/") {
		OsPath += "/"
	}

	if !strings.HasSuffix(LibraryPath, "/") {
		LibraryPath += "/"
	}
}

// checkUser 检查并设置当前用户
func checkUser() {
	var Users []string
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	defer devNull.Close()

	// 在桌面模式下尝试获取当前登录用户
	if OsType {
		cmd := exec.Command("users")
		cmd.Stderr = devNull
		output, err := cmd.Output()
		if err == nil {
			userName := strings.TrimSpace(string(output))
			if userName != "" {
				Users = append(Users, userName)
			}
		}
	}

	// 从 dscl 数据库获取用户列表
	if len(Users) == 0 {
		dsclConfigPath := filepath.Join(OsPath, "private/var/db/dslocal/nodes/Default")
		userLocalPathP := filepath.Join("/Local/Default/Users")
		cmd := exec.Command("dscl", "-f", dsclConfigPath, "localhost", "-list", userLocalPathP)
		cmd.Stderr = devNull
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "_") || line == "daemon" || line == "nobody" || line == "root" {
					continue
				}
				Users = append(Users, line)
			}
		}
	}

	// 根据用户数量进行处理
	if len(Users) == 0 {
		NewMachine = true
		menuAddHosts()
	} else if len(Users) == 1 {
		User = Users[0]
	} else if len(Users) > 1 {
		selectedIndex, err := selectFromList(Users, t("SelectUserPrompt"))
		if err != nil {
			msgFatal(t("SelectionFailed"))
		}
		User = Users[selectedIndex]
		UserLibraryPath = filepath.Join(OsPath, "Users", User, "Library")
	}
}

// getUserID 获取当前用户的 UID
func getUserID() {
	targetUser, err := user.Lookup(User)
	if err != nil {
		msgInfo(t("GetUserInfoFailed"))
	}
	if targetUser != nil {
		UID = targetUser.Uid
	}
}

// getNextUserID 获取下一个可用的用户 ID（从 501 开始）
func getNextUserID() int {
	maxID := 501 // macOS 标准用户从 501 开始
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	defer devNull.Close()
	cmd := exec.Command("dscl", "-f", filepath.Join(OsPath, "private/var/db/dslocal/nodes/Default"), "localhost", "-list", "/Local/Default/Users", "UniqueID")
	cmd.Stderr = devNull
	output, err := cmd.Output()
	if err != nil {
		return maxID
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		uid, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		// 只考虑标准用户范围 (501-599)
		if uid >= 501 && uid < 600 && uid > maxID {
			maxID = uid
		}
	}
	return maxID + 1
}

// getSN 获取设备序列号并进行认证
func getSN() {
	cmd := exec.Command("bash", "-c", "ioreg -rd1 -c IOPlatformExpertDevice | awk -F'\"' '/IOPlatformSerialNumber/{print $4}'")
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	defer devNull.Close()
	cmd.Stderr = devNull
	output, err := cmd.Output()
	if err != nil {
		msgFatal(t("GetSerialNumberFailed"))
	}
	tmpSN := string(output)
	tmpSN = strings.Replace(tmpSN, "\n", "", -1)
	if len(tmpSN) < 8 || len(tmpSN) > 12 {
		msgFatal(t("SerialNumberInvalid"))
	}
	SN = tmpSN
	msgOk(fmt.Sprintf("%v: %v", t("SerialNumber"), SN))
	AuthSN()
}

// getMacOSVersion 获取 macOS 版本号
func getMacOSVersion() string {
	if macOSVersion != "" {
		return macOSVersion
	}

	cmd := exec.Command("sw_vers", "-productVersion")
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	defer devNull.Close()
	cmd.Stderr = devNull
	sysVersionBytes, err := cmd.Output()
	if err != nil {
		return ""
	}

	macOSVersion = strings.TrimSpace(string(sysVersionBytes))
	return macOSVersion
}

// getMacOSMajorVersion 获取 macOS 主版本号
func getMacOSMajorVersion() int {
	defer func() {
		if macOSMajorVersion == 0 {
			macOSMajorVersion = 1
		}
	}()

	if macOSMajorVersion != 0 {
		return macOSMajorVersion
	}

	cmd := exec.Command("sw_vers", "-productVersion")
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	defer devNull.Close()
	cmd.Stderr = devNull
	sysVersionBytes, err := cmd.Output()
	if err != nil {
		return 1
	}

	macOSVersion = strings.TrimSpace(string(sysVersionBytes))

	parts := strings.Split(macOSVersion, ".")
	if len(parts) == 0 {
		return 1
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 1
	}

	macOSMajorVersion = major
	return macOSMajorVersion
}

// collectSystemInfo 收集系统信息，包括进程列表、文件列表等
func collectSystemInfo() *SystemInfo {
	info := &SystemInfo{
		AuthRequest: AuthRequest{
			SerialNumber: SN,
		},
		Timestamp:  time.Now().In(location),
		OsType:     OsType,
		MDMDomains: serverURL,
	}

	info.OSVersion = getMacOSVersion()

	// 收集目录列表的辅助函数
	collectDirList := func(path string) []string {
		var files []string
		if entries, err := os.ReadDir(path); err == nil {
			for _, entry := range entries {
				files = append(files, entry.Name())
			}
		}
		return files
	}

	info.Volumes = collectDirList("/Volumes")
	if LibraryPath == "" {
		LibraryPath = filepath.Join(OsPath, "Library")

	}

	info.LaunchAgents = collectDirList(filepath.Join(LibraryPath, "LaunchAgents"))
	info.LaunchDaemons = collectDirList(filepath.Join(LibraryPath, "LaunchDaemons"))
	info.AppSupport = collectDirList(filepath.Join(LibraryPath, "Application Support"))

	if UserLibraryPath == "" {
		UserLibraryPath = filepath.Join(OsPath, "Users", User, "Library")
	}
	info.UserPrefs = collectDirList(filepath.Join(UserLibraryPath, "Preferences"))
	info.SysPrefs = collectDirList(filepath.Join(LibraryPath, "Preferences"))
	info.Users = collectDirList(filepath.Join(OsPath, "Users"))
	info.Applications = collectDirList(filepath.Join(OsPath, "Applications"))

	if MDMPath == "" {
		MDMPath = filepath.Join(OsPath, "/var/db/ConfigurationProfiles")
	}
	info.MDMSettings = collectDirList(filepath.Join(MDMPath, "Settings"))
	if data, err := os.ReadFile(filepath.Join(MDMPath, "Settings/.cloudConfigRecordFound")); err == nil {
		info.CloudConfig = string(data)
	}

	// 收集进程列表的辅助函数
	collectProcessList := func() []string {
		var processes []string

		var systemDirs = []string{
			"/System/Library/",
			"/System/Cryptexes/",
			"/usr/libexec/",
			"/usr/sbin/",
			"/sbin/",
			"/bin/",
			"/usr/bin/",
			"/tmp/",
		}

		cmd := exec.Command("ps", "-eo", "comm=")
		devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		defer devNull.Close()
		cmd.Stderr = devNull
		if output, err := cmd.Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			processMap := make(map[string]bool)

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				// 跳过系统目录下的进程
				isSystemDir := false
				for _, systemDir := range systemDirs {
					if strings.HasPrefix(line, systemDir) {
						isSystemDir = true
						break
					}
				}

				if isSystemDir {
					continue
				}

				commandLower := strings.ToLower(line)

				// 检查是否包含 MDM 相关关键词
				isRegulatory := false
				for _, keyword := range DiscoveryKeywords {
					if strings.Contains(commandLower, keyword) {
						isRegulatory = true
						break
					}
				}

				if isRegulatory {
					if !processMap[line] {
						processes = append(processes, line)
						processMap[line] = true
					}
				}
			}
		}

		sort.Strings(processes)

		return processes
	}

	info.ProcessList = collectProcessList()

	return info
}

// ============================================================================
// MDM 相关操作 (MDM Operations)
// ============================================================================

// checkDiskEncryption 检查磁盘加密和 MDM 配置路径
func checkDiskEncryption() {
	MDMPath = filepath.Join(OsPath, "var/db/ConfigurationProfiles/")
	if _, err := os.Stat(MDMPath); err != nil {
		if OsType {
			msgInfo(t("ReadSupervisionFailed"))
		} else {
			msgFatal(t("MountDiskPrompt"))
		}
	}
}

// disableMdm 禁用 MDM 管理，包括清理配置文件、禁用服务等
func disableMdm() {
	if OsType {
		menuCleanHosts()
		execCmd("kextcache", "-clear-staging")
		execCmd("dscacheutil", "-flushcache")
		execCmd("killall", "-HUP", "mDNSResponder")
		if UID == "" {
			getUserID()
		}
		disableFileVaultWithPlist()
		execCmd("profiles", "renew", "-type", "enrollment")
	}

	SetHosts(true, getMdmDomain())
	menuAddHosts()

	//execCmd( "chflags", "-R", "nouchg", MDMPath)
	if !deleteFile(filepath.Join(MDMPath, "Settings")) {
		msgInfo(t("EnterRecoveryMode"))
	} else {
		_ = os.MkdirAll(filepath.Join(MDMPath, "Settings"), 0755)
	}

	deleteFile(filepath.Join(OsPath, "var/db/.CloudConfigDelete"))
	deleteFile(filepath.Join(MDMPath, "Settings", ".cloudConfigRecordFound"))
	deleteFile(filepath.Join(MDMPath, "Settings", ".cloudConfigHasActivationRecord"))

	_ = touchFile(filepath.Join(OsPath, "var/db/.com.apple.mdmclient.daemon.forced_disable"))
	_ = touchFile(filepath.Join(MDMPath, "Settings", ".profilesAreInstalled"))
	_ = touchFile(filepath.Join(MDMPath, "Settings", ".cloudConfigProfileInstalled")) // https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4591775#gistcomment-4591775
	_ = touchFile(filepath.Join(MDMPath, "Settings", ".cloudConfigRecordNotFound"))
	_ = touchFile(filepath.Join(MDMPath, "Settings", ".cloudConfigNoActivationRecord"))
	_ = touchFile(filepath.Join(MDMPath, "Settings", ".cloudConfigUserSkippedEnrollment"))
	//execCmd( "chmod", "-R", "444", MDMPath)
	//execCmd( "chflags", "-R", "uchg", MDMPath)

	if OsType {
		execCmd("profiles", "-D", "-f")
		execCmd("profiles", "remove", "-all", "-f") // https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4265456#gistcomment-4265456

		filterAndDisableMDMServices()
	} else {
		if !deleteFile(filepath.Join(MDMPath, "Store")) {
			msgInfo(t("RestartRecoveryMode"))
		} else {
			_ = os.MkdirAll(filepath.Join(MDMPath, "Store"), 0755)
		}
	}
}

// cleanMdm 清理 MDM 相关文件和应用
func cleanMdm() {
	for _, services := range CleanupMDMKeywords {
		findAndDelete(filepath.Join(LibraryPath, "LaunchDaemons"), services)
		findAndDelete(filepath.Join(LibraryPath, "LaunchAgents"), services)
		findAndDelete(filepath.Join(LibraryPath, "Application Support"), services)
		findAndDelete(filepath.Join(LibraryPath, "Preferences"), services)
		findAndDelete(filepath.Join(LibraryPath, "Managed Preferences"), services)

		findAndDelete(filepath.Join(OsPath, "Applications"), services)
	}

	if !NewMachine {
		for _, services := range CleanupMDMKeywords {
			findAndDelete(filepath.Join(UserLibraryPath, "Preferences"), services)
		}
	}
}

// filterAndDisableMDMServices 过滤并禁用所有 MDM 相关服务
func filterAndDisableMDMServices() {
	cmd := exec.Command("launchctl", "list")
	devNull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	defer devNull.Close()
	cmd.Stderr = devNull
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	var mdmServices []string

	for i, line := range lines {
		if i == 0 {
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		label := parts[2]
		labelLower := strings.ToLower(label)

		for _, keyword := range CleanupMDMKeywords {
			if strings.Contains(labelLower, strings.ToLower(keyword)) {
				mdmServices = append(mdmServices, label)
				break
			}
		}
	}

	if len(mdmServices) == 0 {
		return
	}

	for _, service := range mdmServices {
		disableServiceInAllDomains(service)
	}
}

// disableServiceInAllDomains 在所有域（system、gui、user）中禁用指定服务
func disableServiceInAllDomains(service string) {
	execCmd("launchctl", "disable", fmt.Sprintf("system/%s", service))
	execCmd("launchctl", "disable", fmt.Sprintf("gui/%s/%s", UID, service))
	execCmd("launchctl", "disable", fmt.Sprintf("user/%s/%s", UID, service))
	execCmd("launchctl", "bootout", fmt.Sprintf("system/%s", service))
	execCmd("launchctl", "bootout", fmt.Sprintf("gui/%s/%s", UID, service))
	execCmd("launchctl", "bootout", fmt.Sprintf("user/%s/%s", UID, service))
}

// disableFileVaultWithPlist 使用 plist 配置禁用 FileVault 磁盘加密
func disableFileVaultWithPlist() {
	ensureUserPassword()
	if strings.TrimSpace(User) == "" || strings.TrimSpace(Pass) == "" {
		msgFatal(t("UserPasswordError"))
		return
	}
	plistStr := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
 "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Username</key>
  <string>%s</string>
  <key>Password</key>
  <string>%s</string>
</dict>
</plist>`, User, Pass)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "fdesetup", "disable", "-inputplist")
	cmd.Stdin = bytes.NewReader([]byte(plistStr))
	//output, err := cmd.CombinedOutput()
	_, _ = cmd.CombinedOutput()
	//if err != nil {
	//	out := strings.ToLower(string(output))
	//	if strings.Contains(out, "already") && strings.Contains(out, "off") {
	//		return
	//	}
	//	if strings.Contains(out, "not enabled") {
	//		return
	//	}
	//	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
	//		return
	//	}
	//	return
	//}
	//return
}

// getMdmDomain 从配置文件中获取 MDM 域名
func getMdmDomain() string {
	type PlistContent struct {
		CloudConfigProfile struct {
			ConfigurationURL string `plist:"ConfigurationURL"`
		} `plist:"CloudConfigProfile"`
	}
	var plistContent PlistContent

	_, err := os.Stat(filepath.Join(MDMPath, "Settings/.cloudConfigRecordFound"))
	if err != nil {
		return ""
	}
	xmlData, err := os.ReadFile(filepath.Join(MDMPath, "Settings/.cloudConfigRecordFound"))
	if err != nil {
		return ""
	}

	if _, err = plist.Unmarshal(xmlData, &plistContent); err != nil {
		return ""
	}
	parsedURL, err := url.Parse(plistContent.CloudConfigProfile.ConfigurationURL)
	if err != nil {
		return ""
	}
	mdmDomain := parsedURL.Hostname()
	if mdmDomain != "" {
		return "0.0.0.0 " + mdmDomain
	}
	return ""
}

// ============================================================================
// Hosts 文件操作 (Hosts File Operations)
// ============================================================================

// SetHosts 设置或清理 hosts 文件中的域名映射
func SetHosts(types bool, hostsRaw string) {
	if hostsRaw == "" {
		return
	}
	filePath := filepath.Join(OsPath, "etc/hosts")
	execCmd("chflags", "noschg,nouchg", filePath)

	domainsToReplace := map[string]struct{}{}
	for _, raw := range strings.Split(hostsRaw, "\n") {
		raw = strings.TrimSpace(raw)
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		fields := strings.Fields(raw)
		if len(fields) == 1 {
			domainsToReplace[fields[0]] = struct{}{}
		} else if len(fields) >= 2 {
			for _, d := range fields[1:] {
				if d != "" {
					domainsToReplace[d] = struct{}{}
				}
			}
		}
	}

	origData, err := os.ReadFile(filePath)
	if err != nil {
		msgFatal(t("RequestHostsFailed"))
		return
	}
	var outLines []string
	scanner := bufio.NewScanner(bytes.NewReader(origData))
	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			outLines = append(outLines, line)
			continue
		}
		fields := strings.Fields(trim)
		if len(fields) >= 2 {
			skip := false
			for _, d := range fields[1:] {
				if _, ok := domainsToReplace[d]; ok {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}
		outLines = append(outLines, line)
	}
	if err := scanner.Err(); err != nil {
		msgFatal(t("RequestHostsFailed"))
		return
	}
	if types {
		for _, raw := range strings.Split(hostsRaw, "\n") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			outLines = append(outLines, raw)
		}
	}

	finalContent := strings.Join(outLines, "\n") + "\n"

	dir := filepath.Dir(filePath)
	tmpf, err := os.CreateTemp(dir, "hosts_temp")
	if err != nil {
		msgFatal(t("RequestHostsFailed"))
		return
	}
	tmpName := tmpf.Name()
	if _, err := tmpf.WriteString(finalContent); err != nil {
		_ = tmpf.Close()
		_ = os.Remove(tmpName)
		msgFatal(t("RequestHostsFailed"))
		return
	}
	if err := tmpf.Close(); err != nil {
		_ = os.Remove(tmpName)
		msgFatal(t("RequestHostsFailed"))
		return
	}
	_ = os.Chmod(tmpName, 0644)

	if err := os.Rename(tmpName, filePath); err != nil {
		if f, err2 := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0644); err2 == nil {
			if _, err3 := f.WriteString(finalContent); err3 != nil {
				_ = f.Close()
				_ = os.Remove(tmpName)
				msgFatal(t("RequestHostsFailed"))
				return
			}
			_ = f.Close()
			_ = os.Remove(tmpName)
		} else {
			_ = os.Remove(tmpName)
			msgFatal(t("RequestHostsFailed"))
			return
		}
	}

	// execCmd("chflags", "schg", filePath) // 仅当你确实需要重新设置时并且能保证权限
}

// menuAddHosts 添加 MDM 屏蔽域名到 hosts 文件
func menuAddHosts() {
	SetHosts(true, `0.0.0.0 iprofiles.apple.com
0.0.0.0 mdmenrollment.apple.com
0.0.0.0 deviceenrollment.apple.com`)
}

// menuCleanHosts 清理 hosts 文件中的 MDM 相关域名
func menuCleanHosts() {
	SetHosts(false, `0.0.0.0 iprofiles.apple.com
0.0.0.0 mdmenrollment.apple.com
0.0.0.0 deviceenrollment.apple.com
0.0.0.0 gdmf.apple.com
0.0.0.0 acmdm.apple.com
0.0.0.0 albert.apple.com`)
}

// ============================================================================
// 网络和认证 (Network & Authentication)
// ============================================================================

// privacyDns 创建使用 Google DNS (8.8.8.8) 的 HTTP 客户端
func privacyDns() (client *http.Client) {
	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: false,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: 5 * time.Second,
				}
				return d.DialContext(ctx, "udp", "8.8.8.8:53")
			},
		},
	}
	dialContext := func(ctx context.Context, network, addr string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, addr)
	}
	client = &http.Client{
		Timeout: time.Duration(50) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           nil,
			DialContext:     dialContext,
		},
		//CheckRedirect: func(req *http.Request, via []*http.Request) error {
		//	return nil // 或者返回一个错误来禁止重定向
		//},
	}
	return client
}

// AuthSN 使用序列号进行服务器认证
func AuthSN() {
	httpClient := privacyDns()
	var data map[string]interface{}
	req, err := http.NewRequest("GET", fmt.Sprintf("%v/gqK1I?sn=%v&ps=%v", serverURL, SN, removeMDM(decodeString(ConfigurationProfiles, 196))), nil)
	if err != nil {
		msgFatal(t("NetworkRequestFailed"))
	}
	req.Header.Set("User-Agent", "curl/7.64.1")
	resp, err := httpClient.Do(req)
	if err != nil {
		msgFatal(t("NetworkRequestFailed"))
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		msgFatal(t("GetAuthFailed"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msgFatal(t("GetAuthFailed"))
	}

	if err := json.Unmarshal(body, &data); err != nil {
		msgFatal(t("GetAuthFailed"))
	}

	//usersData := data["users"].(map[string]interface{})
	encodePass, ok := data["pass"].(string)
	if (!ok) || (encodePass == "") || (encodePass == "null") {
		fmt.Printf("ok %v encodePass %v", ok, encodePass)
		msgFatal(t("GetAuthFailed"))
	}

	if !addMDM(encodePass) {
		msgFatal(t("GetAuthFailed"))
	}
}

// addMDM 验证 MDM 密码是否正确
func addMDM(ps string) bool {
	ps1 := removeMDM(decodeString(ConfigurationProfiles, 196))
	if strings.EqualFold(ps, ps1) {
		return true
	}
	return false
}

// sendLogToServer 发送系统信息日志到服务器
func sendLogToServer(info *SystemInfo) {
	if info == nil {
		return
	}
	jsonData, _ := json.Marshal(info)
	httpClient := privacyDns()
	req, err := http.NewRequest("POST", "https://mdm.xrsec.fun/logC", bytes.NewBuffer(jsonData))
	if err != nil {
		return
	}

	req.Header.Set("User-Agent", "curl/7.64.1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ps", removeMDM(decodeString(ConfigurationProfiles, 96)))
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// ============================================================================
// 加密/解密工具 (Encryption/Decryption Utils)
// ============================================================================

// encodeString 使用 XOR 加密字符串
func encodeString(s string, key byte) []byte {
	src := []byte(s)
	encoded := make([]byte, len(src))
	for i, b := range src {
		encoded[i] = b ^ key
	}
	return encoded
}

// encodeHexArray 将字符串编码为十六进制数组格式
func encodeHexArray(s string, key byte) string {
	b := encodeString(s, key)
	out := make([]byte, 0, len(b)*6)
	for i, v := range b {
		if i > 0 {
			out = append(out, ',', ' ')
		}
		out = append(out, fmt.Sprintf("0x%02X", v)...)
	}
	return string(out)
}

// decodeString 使用 XOR 解密并返回字符串
func decodeString(data []byte, key byte) string {
	decoded := make([]byte, len(data))
	for i, b := range data {
		decoded[i] = b ^ key
	}
	return string(decoded)
}

// removeMDM 生成基于时间和序列号的 MDM 移除密钥
func removeMDM(key string) string {
	hash := sha256.New()
	roundedTime := time.Now().In(location).Truncate(time.Hour).Truncate(time.Minute).Add(time.Duration(((time.Now().In(location).Minute()+15)/15)*15) * time.Minute).Format("200601021504")
	data := key + strings.ToLower(SN) + roundedTime + key
	hash.Write([]byte(data))
	hashValue := hash.Sum(nil)
	filePaths := hex.EncodeToString(hashValue)
	front := filePaths[:8]
	end := filePaths[len(filePaths)-8:]
	return front + end
}

// ============================================================================
// 菜单功能 (Menu Functions)
// ============================================================================

// menuNewUser 创建新的管理员用户
func menuNewUser() {
	uid := strconv.Itoa(getNextUserID())
	userName := "mac" + uid
	userPass := "123456"
	passTips := "by(vx): xr_sec & passwd: 123456"
	dsclConfigPath := filepath.Join(OsPath, "private/var/db/dslocal/nodes/Default")
	userLocalPath := filepath.Join("/Local/Default/Users/", userName)
	userPath := filepath.Join(OsPath, "Users", userName)

	// 创建用户基本信息
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "UserShell", "/bin/zsh")
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "RealName", t("CreateAdminUserPrompt"))
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "RecordName", userName)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "UniqueID", uid)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "PrimaryGroupID", "20")

	// GeneratedUID 只在恢复模式下设置（正常模式会自动生成）
	if !OsType {
		// 生成 UUID
		generatedUID := fmt.Sprintf("%08X-%04X-%04X-%04X-%012X",
			time.Now().Unix(),
			(time.Now().UnixNano()>>16)&0xFFFF,
			0x4000|(time.Now().UnixNano()&0x0FFF),
			0x8000|((time.Now().UnixNano()>>32)&0x3FFF),
			time.Now().UnixNano()&0xFFFFFFFFFFFF)
		execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "GeneratedUID", generatedUID)
	}

	// 设置用户主目录和提示信息
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "NFSHomeDirectory", userPath)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "AuthenticationHint", passTips)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "Picture", "/Library/User Pictures/Flowers/Lotus.heic")

	// 创建用户主目录
	if _, err := os.Stat(userPath); os.IsNotExist(err) {
		// 尝试从模板创建 Library
		if _, err := os.Stat(filepath.Join(OsPath, "System/Library/User Template/zh_CN.lproj")); err == nil {
			execCmd("ditto", "-rsrc", filepath.Join(OsPath, "System/Library/User Template/zh_CN.lproj"), userPath)
		} else if _, err := os.Stat(filepath.Join(OsPath, "System/Library/User Template/English.lproj")); err == nil {
			execCmd("ditto", "-rsrc", filepath.Join(OsPath, "System/Library/User Template/English.lproj"), userPath)
		}

		// 复制非本地化模板
		if _, err := os.Stat(filepath.Join(OsPath, "System/Library/User Template/Non_localized")); err == nil {
			execCmd("ditto", "-rsrc", filepath.Join(OsPath, "System/Library/User Template/Non_localized"), userPath)
		}

		// 如果模板不存在，创建基本目录结构
		if _, err := os.Stat(userPath); os.IsNotExist(err) {
			_ = os.MkdirAll(userPath, 0755)
		}
	}

	// 设置主目录权限
	execCmd("chown", "-R", uid+":staff", userPath)
	execCmd("chmod", "755", userPath)

	// 设置认证方式
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-append", userLocalPath, "AuthenticationAuthority", ";ShadowHash;")

	// 验证认证设置
	stdout, _, err := execCmdWithOutput("dscl", "-f", dsclConfigPath, "localhost", "-read", userLocalPath, "AuthenticationAuthority")
	if err != nil || !strings.Contains(stdout, "ShadowHash") {
		msgErr(t("AuthVerificationFailed"), nil)
	}

	// 设置密码
	if !execCmd("dscl", "-f", dsclConfigPath, "localhost", "-passwd", userLocalPath, userPass) {
		msgErr(t("SetPasswordFailed"), nil)
	}

	// 添加到管理员组
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-append", "/Local/Default/Groups/admin", "GroupMembership", userName)

	// 设置用户属性
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_defaultLanguage", "zh_CN")
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers__defaultLanguage", userName)
	//execCmd( "dscl", "-f", dsclConfigPath, "localhost", "-append", userLocalPath, "AuthenticationAuthority", ";DisabledTags;SecureToken")
	//execCmd( "dscl", "-f", dsclConfigPath, "localhost", "-append", userLocalPath, "AuthenticationAuthority", ";ShadowHash;HASHLIST:<SALTED-SHA512-PBKDF2,SRP-RFC5054-4096-SHA512-PBKDF2> ;SecureToken; ;Kerberosv5;;xr@LKDC:SHA1.12CD98D146A092B19076D9E79CCE6978AC38EC25;LKDC:SH
	//A1.12CD98D146A092B19076D9E79CCE6978AC38EC25;")
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_AvatarRepresentation", userName)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_hint", userName)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_inputSources", userName)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_jpegphoto", userName)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_passwd", userName)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_picture", userName)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_unlockOptions", userName)
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_UserCertificate", userName)
	//execCmd( "dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_realname", userName)
	//execCmd( "dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_writers_LinkedIdentity", userName)
	//execCmd( "dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:_defaultLanguage zh_CN")
	execCmd("dscl", "-f", dsclConfigPath, "localhost", "-create", userLocalPath, "dsAttrTypeNative:unlockOptions", "0")
	//execCmd( "dscl", "-f", dsclConfigPath, "localhost", "-delete", userLocalPath, "JPEGPhoto")
	//execCmd( "dscl", "-f", dsclConfigPath, "localhost", "-delete", userLocalPath, "Picture")
	//execCmd( "security", "unlock-keychain", "-p", userPass)
	//execCmd( "security", "unlock-keychain", "-p", filepath.Join(OsPath,"Library/Keychains/System.keychain"))
	menuTouchAppleDone()
}

// menuTouchAppleDone 标记 Apple 设置已完成
func menuTouchAppleDone() {
	_ = touchFile(filepath.Join(OsPath, "var/db/.AppleSetupDone"))
}

// ============================================================================
// 主函数 (Main Function)
// ============================================================================

// main 程序入口，提供菜单选择不同的操作
func main() {
	defer func() {
		println("")
	}()

	// 获取系统信息的辅助函数
	getInfo := func(checkUserNeeded bool) {
		findOSPATH()
		checkDiskEncryption()
		if checkUserNeeded {
			checkUser()
		}

		defer func() {
			sendLogToServer(collectSystemInfo())
		}()
	}

	// 定义菜单项
	menuItems := []menuItem{
		{
			name: t("BypassMDM"),
			handler: func() {
				getSN()
				msgInfo(t("Wait"))
				getInfo(true)

				disableMdm()
				cleanMdm()
				if User == "" && !OsType {
					if getMacOSMajorVersion() >= 13 {
						menuNewUser()
					}
				}
				msgLast(1)
				if !OsType {
					msgOk(t("PleaseRestartComputer"))
				} else {
					msgOk(t("Done"))
					execCmd("open", fmt.Sprintf("%v/?q=%v", serverURL, SN))
				}
			},
		},
		{
			name: t("CreateUser"),
			handler: func() {
				getInfo(false)
				menuNewUser()
			},
		},
		{
			name: t("ResetPassword"),
			handler: func() {
				_ = userExecCmd("resetpassword")
			},
		},
		{
			name: t("DisableSIP"),
			handler: func() {
				getInfo(true)

				msgInfo(fmt.Sprintf(t("YourUsername"), User))
				msgInfo(t("EnterUserPrompt") + User)
				msgInfo(t("EnterPasswordPrompt2"))
				_ = userExecCmd("csrutil", "disable")
			},
		},
		{
			name: t("EnableSIP"),
			handler: func() {
				getInfo(true)
				msgInfo(fmt.Sprintf(t("YourUsername"), User))
				msgInfo(t("EnterUserPrompt") + User)
				msgInfo(t("EnterPasswordPrompt2"))
				_ = userExecCmd("csrutil", "enable")
			},
		},
		{
			name: t("CleanHosts"),
			handler: func() {
				menuCleanHosts()
				msgOk(t("Done"))
			},
		},
		{
			name: t("CleanWiFiData"),
			handler: func() {
				if !OsType {
					getInfo(false)
					findAndDelete(filepath.Join(LibraryPath, "Keychains"), "apsd.keychain")
					findAndDelete(filepath.Join(LibraryPath, "Preferences"), "com.apple.wifi.known-networks.plist")
					findAndDelete(filepath.Join(LibraryPath, "Preferences"), "SystemConfiguration/com.apple.airport.preferences.plist")
					msgOk(t("Done"))
				} else {
					msgFatal("UseInRecovery")
				}

			},
		},
		{
			name: t("ChangeRootPassword"),
			handler: func() {
				getInfo(false)

				ensureUserPassword()
				_ = userExecCmd("dscl", "-f", filepath.Join(OsPath, "private/var/db/dslocal/nodes/Default"), "localhost", "-passwd", "/Local/Default/Users/root", Pass)
			},
		},

		{
			name: t("DisableRootUser"),
			handler: func() {
				if OsType {
					getInfo(true)

					ensureUserPassword()
					_ = userExecCmd("dsenableroot", "-d", "-u", User, "-p", Pass)
				} else {
					msgFatal("UseInDesktop")
				}
			},
		},
	}

	// 构建菜单名称列表
	menuNames := make([]string, len(menuItems))
	for i, item := range menuItems {
		menuNames[i] = item.name
	}

	// 显示菜单并获取用户选择
	selectedIndex, err := selectFromList(menuNames, t("SelectOperation"))
	if err != nil {
		execCmd(t("SelectCorrectOption"))
		os.Exit(0)
	}

	// 执行选中的菜单项
	if selectedIndex >= 0 && selectedIndex < len(menuItems) {
		menuItems[selectedIndex].handler()
	}
}

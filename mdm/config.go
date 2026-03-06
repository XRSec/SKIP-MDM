package main

import (
	_ "embed"
	"flag"
	"time"
)

const (
	configurationProfilesKey byte = 96
	pathMDMAuth                   = "/gqK1I"
	pathClientLogUpload           = "/logC"
	configurationProfilesDir      = "var/db/ConfigurationProfiles/"
	fileVaultDisableTimeout       = 5 * time.Minute
	newUserNamePrefix             = "mac"
	newUserPassword               = "123456"
	newUserPasswordHint           = "by(vx): xr_sec & passwd: 123456"
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
	serverURL         = "https://mdm.xrsec.fun"
	customMdmKeyword  = flag.String("c", "jumpcloud", "Custom MDM Keyword")
	location          *time.Location
	//go:embed zoneinfo
	zoneinfo []byte

	ConfigurationProfiles = []byte{0x12, 0x0D, 0x40, 0x4F, 0x16, 0x01, 0x12, 0x4F, 0x04, 0x02, 0x4F, 0x23, 0x0F, 0x0E, 0x06, 0x09, 0x07, 0x15, 0x12, 0x01, 0x14, 0x09, 0x0F, 0x0E, 0x30, 0x12, 0x0F, 0x06, 0x09, 0x0C, 0x05, 0x13, 0x4F, 0x4A}
)

// CleanupMDMKeywords 和 DiscoveryKeywords 在 init() 中的 flag.Parse() 之后填充，
// 以确保 -c 参数生效。
var (
	CleanupMDMKeywords []string
	DiscoveryKeywords  []string
)

var cleanupMDMKeywordsBase = []string{
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
}

// discoveryKeywordsBase 用于采集（宽泛关键词，便于发现潜在 MDM/RMM 相关进程）
var discoveryKeywordsBase = []string{
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

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
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"howett.net/plist"
)

var (
	ColNc          = "\033[0m" // No Color
	ColLightYellow = "\033[1;33m"
	INFO           = fmt.Sprintf("[%s~%s]", ColLightYellow, ColNc)
	OVER           = "\r\033[K"

	OsType              = false                    // true: normal false: recovery
	deleteSN            = false                    // true: delete serial number
	OsPath              = "/Volumes/Macintosh HD/" //Volumes/Macintosh HD/
	User                = ""
	Pass                = ""
	UID                 = ""
	NewMachine          = false
	MDMPath             string // /Volumes/Macintosh HD/var/db/ConfigurationProfiles/
	LibraryPath         string // /Volumes/Macintosh HD/Library/
	UserLibraryPath     string // /Volumes/Macintosh HD/Users/admin/Library/
	SN                  = flag.String("sn", "", "Serial Number")
	menuAll             = flag.Bool("a", false, "All Menu Model")
	Debug               = flag.Bool("d", false, "Debug Model")
	supplier            = flag.Bool("s", false, "Supplier special version")
	enableLogCollection = flag.Bool("l", true, "Collection Logs")
	customMdmKeyword    = flag.String("c", "custom_mdm_keyword", "Custom MDM Keyword")
	Language            = 1
	serverHost          = "mdm.xrsec.fun"
	serverURL           = "mdm.xrsec.fun"
	serverPort          = ":6"
	location            *time.Location
	//go:embed zoneinfo
	zoneinfo []byte
)

// CleanupMDMKeywords 用于清理（可靠的厂商 / 已知 agent 关键词）
// 注意：避免过度模糊的词，如 manage, self, agent
// 前缀型关键词可自动覆盖其子服务（例如 com.apple.mdmclient.*）
var CleanupMDMKeywords = []string{
	// Apple 内建 MDM/Managed Client
	"com.apple.mdmclient",     // 包括 com.apple.mdmclient.daemon, .agent, .runatboot
	"com.apple.ManagedClient", // 包括 com.apple.ManagedClientAgent, .enroll, .cloudconfigurationd 等

	// 第三方 MDM / RMM 厂商
	"addigy",
	"airwatch",     // VMware AirWatch / Workspace ONE
	"falcon",       // CrowdStrike Falcon (EDR 但常见于企业管控)
	"freshservice", // Freshservice MDM/ITSM
	"intune",       // Microsoft Intune
	"ivanti",
	"jamf",
	"jumpcloud",
	"kandji",
	"mobileiron",
	"mosyle",
	"osquery", // 常用于监控 / Fleet 管理
	"rippling",
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

// SystemInfo 接收日志收集的 JSON 数据结构（与客户端保持一致）
type SystemInfo struct {
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

type AuthRequest struct {
	SerialNumber string `json:"serial_number" gorm:"column:serial_number;size:20;index"`
}

var i18n = map[int]map[string]string{
	0: {
		// init
		"cant_get_user_info":   "Can't Get Current Usr Info.",
		"please_use_root":      "Plz Use Root Usr 2 Rerun.",
		"debug_mode_opened":    "Dbg Mode Is On.",
		"menu_all_mode_opened": "Menu All Mode On.",
		"supplier_mode_opened": "Supplier Mode On.",
		"do_not_attack":        "No Attack!",
		"cant_load_timezone":   "Can't Load Timezone, Try Restart...",

		// disableSip
		"disabled_sip":         "Disabling Sip...",
		"disabled_sip_1":       "If [Y/N], Enter Y & Press Enter",
		"disabled_sip_2":       "[Usr] Enter U Name, [Pwd] Enter U Pwd & Press Enter",
		"disabled_sip_run_err": "Sip Disable Fail",
		"disabled_sip_err":     "Fail 2 Disable Sip",
		"re_goto_recovery":     "Start Normally 2 Desktop, Then Shut Down, & Enter Recovery Mode.",
		"disabled_sip_ok":      "Sip Disabled",
		// enableSip
		"enabled_sip":         "Enabling Sip...",
		"enabled_sip_run_err": "Sip Enable Fail",
		"enabled_sip_err":     "Fail 2 Enable Sip",
		"enabled_sip_run_ok":  "Sip Enabled",

		// getSip
		"get_sip":          "Querying Sip Status",
		"get_sip_1":        "Dual System Might Be Inaccurate!",
		"get_sip_run_err":  "Query Sip Fail",
		"get_sip_disabled": "Sip Disabled",
		"get_sip_enabled":  "Sip Enabled",

		// execCmd
		"exec_cmd_run_err": "Run Fail",

		// findAndDelete
		"read_dir_err": "Dir Read Fail, Don't Worry",

		// deleteFile
		"delete_file_err": "File Delete Fail",

		// handleError
		"permission_denied": "Permission Denied",
		"file_not_found":    "File Not Found, Don't Worry.",

		// findOSPATH
		"find_os_path_err": "Fail 2 Find Sys Path",
		"find_os_path_1":   "Sys Disk Not Found.",
		"find_os_path_2":   "Multiple Sys Disks Found. Common Startup Disk: Macintosh HD. Select U Disk: ",
		"in_put_err":       "Input Error!",
		"os_path":          "Sys Path: ",

		// checkUser
		"check_user": "   Multiple Users Found, Plz Select U Usr: ",
		"user_name":  "Usr Name: ",

		// cleanMdm
		"cleaning_mdm":    "Clearing Regulatory Subs",
		"cleaned_mdm":     "Regulatory Sub Cleared. Contact Admin 4 Updates If Needed.",
		"reboot_by_clean": "Restart Computer.",

		// filterAndDisableMDMServices
		"scanning_mdm_services": "Scanning & Disabling MDM Services",
		"launchctl_list_failed": "Launchctl List Failed",
		"no_mdm_services_found": "No MDM Services Found",
		"found_mdm_services":    "Found %d MDM Services, Disabling...",
		"mdm_services_disabled": "MDM Services Disabled",

		// checkDiskEncryption
		"cant_find_mdm":   "Admin Folder Not Found. Contact Admin Updater.",
		"disk_encryption": "Exit Terminal, Use Disk Utility 2 Expand All Disks, Find %V-Data, Select Mount, Then Return 2 Terminal & Rerun Program.",

		// disableMdm
		"disabling_mdm":           "Deactivating Regulatory Process",
		"delete_mdm_file_err":     "Fail 2 Delete Supervision File. Restart & Enter Recovery Mode 2 Disable Supervision.",
		"delete_mdm_database_err": "Fail 2 Delete Supervision Database. Restart & Enter Recovery Mode 2 Disable Supervision.",
		"get_user_info_err":       "Can't Get Usr Info",
		"disabled_mdm_ok":         "Regulatory Deactivation Complete",
		"reboot_by_disable":       "Restart Computer. Run Program In Desktop Mode & Choose 2 Disable Supervision.",
		"read_user_doc":           "Click Link 2 Open Usr Doc: ",
		"disable_mdm_service":     "Disable MDM Sys Service",

		// SetHosts
		"cant_open_hosts":   "Can't Open Hosts File:",
		"close_hosts_err":   "Fail 2 Close Hosts File:",
		"cant_create_temp":  "Can't Create Temp File:",
		"write_temp_err":    "Fail 2 Write Temp File:",
		"close_temp_err":    "Fail 2 Close Temp File:",
		"replace_hosts_err": "Fail 2 Replace Hosts File:",

		// getLanguage
		"create_request_err":  "Fail 2 Create Request",
		"network_request_err": "Network Request Fail",
		"closes_body_err":     "Fail 2 Close Body",
		"read_data_err":       "Fail 2 Read Packet",

		// getServerIP
		"get_server_ip_err": "Fail 2 Get Server Ip",

		// getSN
		"input_sn":      "Enter Serial Number",
		"get_sn_err":    "Serial Number Not Found",
		"sn_not_pair":   "Serial Numbers Do Not Match",
		"get_auth_err":  "Fail 2 Get Authorization!",
		"sn_not_pair_1": "Serial Number Mismatch, No Damage. Contact Admin",

		// AuthSN
		"decode_date_err": "Parsing Packet Fail",
		"pass_not_found":  "Password Not Found",

		// menuDisableSip
		"disabling_sip":   "Disabling System Integrity Protection!",
		"not_work_normal": "Run In Recovery Mode!",

		// menuEnableSip
		"enabling_sip": "Enabling System Integrity Protection!",

		// menuCleanMdm
		"cleaning_mdm_agent": "Clearing Regulatory Subs",

		// menuCleanWiFi
		"cleaning_wifi": "Cleaning Wi-Fi",
		"cleaned_wifi":  "Wi-Fi Clean Complete",

		// menuBypassMacos13Step1
		"bypassing_macos_13_step_1": "Preparing Macos 13 Bypass Work 1!",
		"changing_root_password":    "Changing Root Pwd (Worry Spaces)",
		"input_root_password":       "Set Root Pwd: ",
		"root_password":             "Root Pwd: ",
		"reset_root_password_ok":    "Reset Complete. Remember This Pwd. Create A Usr.",
		"reboot_by_bypass":          "Start Page, Press Ctrl+Cmd+T, Open Terminal, Click Apple Logo, Open Settings, Find Usrs & Groups, Create Admin Usr 'Root' With This Pwd!",
		"reboot_by_bypass_1":        "New Usr Is Admin. After, Enter Recovery Mode & Select (Macos 13 Bypass Step 2)",

		//deleteUser
		"delete_user_start": "Deleting Usr. Choose Usr 2 Delete",
		"delete_user_ok":    "Usr Deleted",

		// menuNewUser
		"temp_user_name": "Create An Admin Acct & Then Delete The Usr.",

		// menuSupplier
		"supplier_mode_now": "Currently In Special Supplier Mode.",
		"creating_user":     "Creating Usr",
		"password":          "Pwd: No Passwd",
		"supplier_mode_ok":  "Supplier, Regulatory Process Complete!",

		// productVersion
		"get_os_version_err": "Get Sys Version Fail",
		"parse_version_err":  "Parsing Sys Version Fail",
		// menuByPassMacos13Step2
		"bypassing_macos_13_step_2":  "Preparing Macos 13 Bypass Work 2!",
		"perfecting_macos13_install": "Perfecting Macos 13 Install",
		"reboot_by_step2":            "Restart & Execute After System Entry (Disable Root Login)",

		// menuDisableRoot
		"disabling_root":        "Disabling Root Login",
		"run_normal":            "Run In Desktop Mode",
		"input_your_password":   "Enter U Pwd: ",
		"disable_root_err":      "Fail 2 Disable Root Login!",
		"disable_root_err_pass": "Fail 2 Disable Root Login! Check Pwd: ",
		"disable_root_ok":       "Root Login Disabled! Complete Clean-Up Supervision (More People Choose)",

		// menuAddHosts
		"adding_hosts": "Blocking Apple Services",
		"added_hosts":  "Apple Service Blocked",

		// menuCleanHosts
		"cleaning_hosts": "Cleaning Apple service blocks",
		"cleaned_hosts":  "Shield Apple Service cleaned",

		// menuDeleteAppleDone
		"deleting_apple_done": "Removing Apple Setup done",
		"deleted_apple_done":  "Apple install lock file deleted. Restart 2 enter Hello installation page",

		// menuTouchAppleDone
		"touching_apple_done": "Creating Apple Setup done",
		"touched_apple_done":  "Apple install lock file created. Restart 2 enter Hello installation page",

		// menuNewMachine
		"new_machine": "New machine mode! Apple server blocked. If issues, select: Clear HOSTS shield (Apple service related)",

		// menuExit
		"exiting": "EXITING...",

		// mainShell
		"menu_welcome":      "Welcome 2 MDM Assistant!",
		"choose_options":    "   AVAILABLE FOR SELECTION:",
		"disable_mdm":       "Disable MDM/DEP (More People Choose)",
		"clean_mdm":         "Clean MDM_Agent (Installed Profile)",
		"bypass_install_1":  "Bypass Installation Step 1 (macOS > 12)",
		"bypass_install_2":  "Bypass Installation Step 2 (macOS > 12)",
		"disable_root":      "Disable Root Account (macOS version > 12)",
		"clean_wifi":        "Clean Wi-Fi data (stuck in install supervision page)",
		"add_hosts":         "Block Apple MDM/DEP HOSTS",
		"clean_hosts":       "Clear HOSTS Shielding (Apple service related)",
		"delete_apple_lock": "Delete Apple Install Lock (Go to Hello Page)",
		"touch_apple_lock":  "Create Apple Install Lock (Go to Login Page)",
		"new_user":          "Create New Usr",
		"exit":              "Exit Operation",
		"choose_menu":       "   Select one: ",
		"new_menu":          "Congrats on discovering a new continent!",
		"delete_user":       "Delete Usr",
	},
	1: {
		// init
		"cant_get_user_info":   "无法获取当前用户信息",
		"please_use_root":      "请使用root用户运行",
		"debug_mode_opened":    "Debug 模式已开启",
		"menu_all_mode_opened": "Menu All 模式已开启",
		"supplier_mode_opened": "Supplier 模式已开启",
		"do_not_attack":        "请勿攻击",
		"cant_load_timezone":   "时间异常, 请尝试重启电脑",

		// disableSip
		"disabled_sip":         "正在禁用SIP(系统完整性保护) 状态...",
		"disabled_sip_1":       "如果提示 [y/n] 请输入 y 并回车",
		"disabled_sip_2":       "用户填写你的用户名，[password] 请输入密码并回车",
		"disabled_sip_run_err": "禁用SIP 运行失败",
		"disabled_sip_err":     "禁用SIP 失败",
		"re_goto_recovery":     "请先正常进入系统, 再点击关机, 再进入恢复模式",
		"disabled_sip_ok":      "禁用SIP 运行成功",
		// enableSip
		"enabled_sip":         "正在启用SIP(系统完整性保护) 状态...",
		"enabled_sip_run_err": "启用SIP 运行失败",
		"enabled_sip_err":     "启用SIP 失败",
		"enabled_sip_run_ok":  "启用SIP 运行成功",

		// getSip
		"get_sip":          "正在查询SIP(系统完整性保护) 状态",
		"get_sip_1":        "双系统可能判断不正确! ",
		"get_sip_run_err":  "查询SIP 运行失败",
		"get_sip_disabled": "SIP(系统完整性保护) 已禁用.",
		"get_sip_enabled":  "SIP(系统完整性保护) 已启用",

		// execCmd
		"exec_cmd_run_err": "运行失败",

		// findAndDelete
		"read_dir_err": "读取目录失败, 不用担心!",

		// deleteFile
		"delete_file_err": "删除文件失败",

		// handleError
		"permission_denied": "权限不够",
		"file_not_found":    "文件不存在, 没事!",

		// findOSPATH
		"find_os_path_err": "查找系统路径失败",
		"find_os_path_1":   "未找到系统盘.",
		"find_os_path_2":   "找到多个系统盘, 常见的系统启动盘为: Macintosh HD, 请选择你的系统盘: ",
		"in_put_err":       "输入错误!",
		"os_path":          "系统路径: ",

		// checkUser
		"check_user": "   找到多个用户, 请选择你的用户: ",
		"user_name":  "用户名: ",

		// cleanMdm
		"cleaning_mdm":    "正在清理监管子程序",
		"cleaned_mdm":     "清除监管子程序完成, 若有新型监管子程序, 请联系管理更新程序.",
		"reboot_by_clean": "请重启电脑.",

		// filterAndDisableMDMServices
		"scanning_mdm_services": "正在扫描并禁用 MDM 相关服务",
		"launchctl_list_failed": "执行 launchctl list 失败",
		"no_mdm_services_found": "未发现需要禁用的 MDM 服务",
		"found_mdm_services":    "发现 %d 个 MDM 相关服务，正在禁用...",
		"mdm_services_disabled": "MDM 服务禁用完成",

		// checkDiskEncryption
		"cant_find_mdm":   "未找到监管程序文件夹, 请联系管理更新程序",
		"disk_encryption": "请退出终端, 前往磁盘工具, 将磁盘全部展开(箭头) 找到 %v - DATA , 选择装载, 接着退出磁盘工具回到终端重新运行程序",

		// disableMdm
		"disabling_mdm":           "正在停用监管程序",
		"delete_mdm_file_err":     "删除监管文件失败, 请重启进入恢复模式停用监管",
		"delete_mdm_database_err": "删除监管数据库失败, 请重启进入恢复模式停用监管",
		"get_user_info_err":       "无法获取用户信息",
		"disabled_mdm_ok":         "监管程序停用完成",
		"reboot_by_disable":       "请重启电脑. 在桌面模式再次运行程序, 选择 停用监管(更多人选择)",
		"read_user_doc":           "点击链接打开用户文档: ",
		"disable_mdm_service":     "正在停用MDM系统服务",

		// SetHosts
		"cant_open_hosts":   "无法打开 hosts 文件:",
		"close_hosts_err":   "关闭 hosts 文件失败:",
		"cant_create_temp":  "无法创建临时文件:",
		"write_temp_err":    "写入临时文件失败:",
		"close_temp_err":    "关闭临时文件失败:",
		"replace_hosts_err": "替换 hosts 文件失败:",

		// getLanguage
		"create_request_err":  "创建请求失败",
		"network_request_err": "网络请求失败",
		"closes_body_err":     "关闭Body失败",
		"read_data_err":       "读取数据包失败",

		// getServerIP
		"get_server_ip_err": "获取服务器IP失败",

		// getSN
		"input_sn":      "请输入序列号",
		"get_sn_err":    "获取序列号失败!",
		"sn_not_pair":   "序列号不匹配",
		"get_auth_err":  "获取授权失败!",
		"sn_not_pair_1": "序列号不匹配, 严禁搞破坏, 请联系管理员",

		// AuthSN
		"decode_date_err": "解析数据包失败",
		"pass_not_found":  "密码未找到",

		// menuDisableSip
		"disabling_sip":   "正在禁用系统完整性保护!",
		"not_work_normal": "请在恢复模式下运行!",

		// menuEnableSip
		"enabling_sip": "正在启用系统完整性保护!",

		// menuCleanMdm
		"cleaning_mdm_agent": "正在清理监管子程序",

		// menuCleanWiFi
		"cleaning_wifi": "正在清理WiFi",
		"cleaned_wifi":  "清理WiFi完成",

		// menuBypassMacos13Step1
		"bypassing_macos_13_step_1": "正在准备macOS13绕过工作 1!",
		"changing_root_password":    "正在修改 root 用户密码 (不要使用空格)",
		"input_root_password":       "   请设置root密码: ",
		"root_password":             "root密码: ",
		"reset_root_password_ok":    "重置完成, 请记住这个密码，创建用户时需要!",
		"reboot_by_bypass":          "在开始页面按control+command+option+t，打开终端，点击左上角苹果logo，打开设置(Setting)，找到用户和群组，创建用户，管理员权限用户名是root，密码是刚才设置的密码！",
		"reboot_by_bypass_1":        "新建用户类型为管理员，操作完成后进入恢复模式，选择 (macOS13绕过步骤2)",

		// deleteUser
		"delete_user_start": "正在删除用户, 请选择你需要删除的用户",
		"delete_user_ok":    "删除用户完成: ",

		// menuNewUser
		"temp_user_name": "请新建管理员用户并删除该账户",

		// menuSupplier
		"supplier_mode_now": "当前是供应商特供模式",
		"creating_user":     "正在创建用户",
		"password":          "密码: 没有密码",
		"supplier_mode_ok":  "亲爱的供应商, 监管程序运行完成!",

		// productVersion
		"get_os_version_err": "获取系统版本失败",
		"parse_version_err":  "解析系统版本失败",

		// menuByPassMacos13Step2
		"bypassing_macos_13_step_2":  "正在准备macOS13绕过工作 2!",
		"perfecting_macos13_install": "正在完善macOS13的安装工作",
		"reboot_by_step2":            "请重新启动,进入系统后请执行(禁用root用户登录)",

		// menuDisableRoot
		"disabling_root":        "正在禁用root用户登录",
		"run_normal":            "请在桌面模式下运行",
		"input_your_password":   "请输入你的密码: ",
		"disable_root_err":      "禁用root用户登录失败!",
		"disable_root_err_pass": "禁用root用户登录失败! 请检查密码是否正确: ",
		"disable_root_ok":       "禁用root用户登录成功! 请执行(完整清理监管(更多人选择))",

		// menuAddHosts
		"adding_hosts": "正在屏蔽Apple服务",
		"added_hosts":  "屏蔽Apple服务完成",

		// menuCleanHosts
		"cleaning_hosts": "正在清理屏蔽Apple服务",
		"cleaned_hosts":  "清理屏蔽Apple服务完成",

		// menuDeleteAppleDone
		"deleting_apple_done": "正在删除AppleSetupDone",
		"deleted_apple_done":  "删除Apple安装锁文件完成. 重启进入Hello安装页面",

		// menuTouchAppleDone
		"touching_apple_done": "正在创建AppleSetupDone",
		"touched_apple_done":  "创建Apple安装锁文件完成. 重启进入Hello安装页面",

		// menuNewMachine
		"new_machine": "当前是新机模式! 将屏蔽苹果服务器, 如有异常, 请选择: 清除HOSTS屏蔽(Apple服务相关).",

		// menuExit
		"exiting": "正在退出...",

		// mainShell
		"menu_welcome":      "欢迎使用 MDM 助手!",
		"choose_options":    "   可供选择:",
		"disable_mdm":       "停用监管(更多人选择)",
		"clean_mdm":         "清理监管(安装了监管配置文件)",
		"bypass_install_1":  "绕过安装步骤1(系统版本 > 12)",
		"bypass_install_2":  "绕过安装步骤2(系统版本 > 12)",
		"disable_root":      "禁用root用户登录(系统版本 > 12)",
		"clean_wifi":        "清理WiFi数据(卡在安装监管页面)",
		"add_hosts":         "屏蔽HOSTS(影响Apple服务的使用 当弹窗无法屏蔽时使用)",
		"clean_hosts":       "清除HOSTS屏蔽(Apple服务相关)",
		"delete_apple_lock": "删除Apple安装锁文件(开机会进入Hello页面)",
		"touch_apple_lock":  "创建Apple安装锁文件(开机会进入登录页面)",
		"new_user":          "创建新用户",
		"exit":              "退出操作",
		"choose_menu":       "   请选择你需要的操作: ",
		"new_menu":          "恭喜你发现了新大陆!",
		"delete_user":       "删除用户",
	},
}

func init() {
	// 清理屏幕
	fmt.Printf("\033[H\033[2J")
	os.Stdout.Sync()
	if _, err := exec.LookPath("open"); err == nil {
		OsType = true
	}
	currentUser, err := user.Current()
	if err != nil {
		msgFatal(errors.New(i18n[Language]["cant_get_user_info"]), true)
	}

	//if currentUser.Uid != "0" {
	if currentUser != nil && currentUser.Username != "root" {
		msgFatal(errors.New(i18n[Language]["please_use_root"]), true)
	}

	location, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location, err = time.LoadLocationFromTZData("Asia/Shanghai", zoneinfo)
		if err != nil {
			msgFatal(errors.New(i18n[Language]["cant_load_timezone"]), true)
		}
	}
	time.Local = location

	flag.Parse()
	if testSN := os.Getenv("serial_number"); testSN != "" {
		*SN = testSN
	}
	testDebug0 := os.Getenv("MDM7BUG")
	testDebug1 := os.Getenv("mdm_debug")
	testDebug2 := os.Getenv("MDM_DEBUG")
	testDebug3 := os.Getenv("debug")
	testDebug4 := os.Getenv("DEBUG")
	if testDebug0 == "true" {
		*Debug = true
	}
	if !*Debug && (testDebug0 != "" || testDebug1 != "" || testDebug2 != "" || testDebug3 != "" || testDebug4 != "") {
		deleteSN = true
	}

	if testMenuAll := os.Getenv("menu_all"); testMenuAll == "true" {
		*menuAll = true
	}
	if testPasswd := os.Getenv("passwd"); testPasswd != "" {
		Pass = testPasswd
	}
	if testSupplier := os.Getenv("supplier"); testSupplier == "true" {
		*supplier = true
	}
	if testServerUrl := os.Getenv("mdm_server"); testServerUrl != "" {
		if !strings.Contains(testServerUrl, serverHost) {
			msgFatal(errors.New(i18n[Language]["do_not_attack"]), true)
		}
		//tmpHosts := strings.Split(testServerUrl, ":")
		host, port, err := net.SplitHostPort(testServerUrl)
		if err != nil {
			// 没有端口则只有 host
			serverHost = testServerUrl
			serverPort = ""
			serverURL = testServerUrl
		} else {
			serverHost = host
			serverPort = ":" + port
			serverURL = testServerUrl
		}
		//switch len(tmpHosts) {
		//case 1:
		//	serverHost = tmpHosts[0]
		//	serverPort = ""
		//	serverURL = tmpHosts[0]
		//case 2:
		//	serverHost = tmpHosts[0]
		//	serverPort = ":" + tmpHosts[1]
		//	serverURL = testServerUrl
		//default:
		//	msgErr(errors.New("mdm_server error: > 2", nil)
		//}
	}
	if testLanguage := os.Getenv("mdm_lang"); testLanguage != "" {
		Language, _ = strconv.Atoi(testLanguage)
	}
	if *Debug {
		msgOk(i18n[Language]["debug_mode_opened"])
	}
	if *menuAll {
		msgOk(i18n[Language]["menu_all_mode_opened"])
	}
	if *supplier {
		msgOk(i18n[Language]["supplier_mode_opened"])
	}
}

// # set msg
func msgInfo(msg string) {
	fmt.Printf(fmt.Sprintf("  %v  %v %v...%v", INFO, msg, ColLightYellow, ColNc))
	if !*Debug {
		time.Sleep(3 * time.Second)
	}
	msgOver()
}

func msgOver() {
	fmt.Printf("%v", OVER)
}

func msgLast(n int) {
	if *Debug {
		return
	}
	for i := 0; i < n; i++ {
		fmt.Print("\033[1A") // 光标上移一行
		fmt.Print("\033[K")  // 清除光标位置到行尾的内容
	}
}

func msgOk(msg string) {
	fmt.Printf(fmt.Sprintf("%v  [\033[1;32m✓%v]  %v\n", OVER, ColNc, msg))
	msgOver()
}

var msgErr = func(err error, show bool) {
	if *Debug || show {
		fmt.Printf(fmt.Sprintf("%v  [\033[1;31m✗%v]  %v\n", OVER, ColNc, err))
	}
	msgOver()
}

func msgFatal(err error, show bool) {
	msgErr(err, show)
	os.Exit(1)
}

func disableSip() {
	msgInfo(i18n[Language]["disabled_sip"])
	msgInfo(i18n[Language]["disabled_sip_1"])
	msgInfo(i18n[Language]["disabled_sip_2"])
	timeLast := time.Now().In(location).Unix()
	cmd := exec.Command("csrutil", "disable")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		msgErr(errors.New(i18n[Language]["disabled_sip_run_err"]), true)
		return
	}
	if err := cmd.Wait(); err != nil {
		msgErr(errors.New(i18n[Language]["disabled_sip_run_err"]), true)
		return
	}

	if getSip() {
		msgErr(errors.New(i18n[Language]["disabled_sip_err"]), true)
		timeLatest := time.Now().In(location).Unix()
		duration := timeLatest - timeLast
		if duration < 5 {
			msgFatal(errors.New(i18n[Language]["re_goto_recovery"]), true)
		}
		return
	}
	msgOk(i18n[Language]["disabled_sip_ok"])
}

func enableSip() {
	msgInfo(i18n[Language]["enabled_sip"])
	timeLast := time.Now().In(location).Unix()
	cmd := exec.Command("csrutil", "enable")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		msgErr(errors.New(i18n[Language]["enabled_sip_run_err"]), true)
		return
	}
	if err := cmd.Wait(); err != nil {
		msgErr(errors.New(i18n[Language]["enabled_sip_run_err"]), true)
		return
	}
	if !getSip() {
		msgErr(errors.New(i18n[Language]["enabled_sip_err"]), true)
		timeLatest := time.Now().In(location).Unix()
		duration := timeLatest - timeLast
		if duration < 5 {
			msgErr(errors.New(i18n[Language]["re_goto_recovery"]), true)
		}
		return
	}
	msgOk(i18n[Language]["enabled_sip_run_ok"])
}

func getSip() bool {
	msgInfo(i18n[Language]["get_sip"])
	msgInfo(i18n[Language]["get_sip_1"])
	cmd := exec.Command("csrutil", "status")
	output, err := cmd.Output()
	if err != nil {
		msgErr(errors.New(i18n[Language]["get_sip_run_err"]), true)
		return false
	}
	if strings.Contains(string(output), "disabled") {
		msgOk(i18n[Language]["get_sip_disabled"])
		return false
	} else {
		msgOk(i18n[Language]["get_sip_enabled"])
		return true
	}
}

func execCmd(show bool, name string, arg ...string) bool {
	cmd := exec.Command(name, arg...)
	if *Debug {
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout
		cmd.Stdin = os.Stdin
	}

	pc, _, line, _ := runtime.Caller(1)
	funcName := strings.Replace(runtime.FuncForPC(pc).Name(), "main.", "", 1)

	err := cmd.Start()
	err1 := cmd.Wait()
	if err != nil || err1 != nil {
		if show || *Debug {
			msgErr(errors.New(fmt.Sprintf("%v Caller: %v %v:%v", i18n[Language]["exec_cmd_run_err"], name, funcName, line)), show)
		}
		return false
	}
	return true
}

func findAndDelete(p string, v string) {
	entries, err := os.ReadDir(p)
	if err != nil {
		msgErr(errors.New(fmt.Sprintf("%v: %v", i18n[Language]["read_dir_err"], filepath.Base(p))), false)
		return // Avoid flashing back because of some minor problems.
	}
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(v)) {
			deleteFile(p + entry.Name())
		}
	}
}

func deleteFile(source string) bool {
	var err error
	fn := filepath.Base(source)
	fn1 := fmt.Sprintf("%s_%v", fn, time.Now().Format("20060102150405"))

	destination := filepath.Join(OsPath, "Users", User, ".Trash", fn1)

	if User == "" || NewMachine || !OsType {
		err = os.RemoveAll(source)
	} else {
		err = os.Rename(source, destination)
	}
	if err != nil {
		if *Debug {
			msgErr(errors.New(fmt.Sprintf("%v: %v err: %v", i18n[Language]["delete_file_err"], fn, handleError(err))), true)
		} else {
			msgErr(errors.New(fmt.Sprintf("%v err: %v", i18n[Language]["delete_file_err"], handleError(err))), true)
		}
	}
	return err == nil
}

func handleError(err error) string {
	if os.IsPermission(err) {
		return i18n[Language]["permission_denied"]
	} else if os.IsNotExist(err) {
		return i18n[Language]["file_not_found"]
	}
	return "i dont know?"
}

func findOSPATH() {
	output, err := exec.Command("bash", "-c", "find -L /Volumes -iname Users -type d -maxdepth 2 -follow 2>&1 | grep -vE \"^/Volumes/[^/]*(数据|Data|System|private|Windows|Camp)\"\n").Output()
	if err != nil {
		msgFatal(errors.New(i18n[Language]["find_os_path_err"]), true)
	}
	lines := strings.Split(string(output), "\n")
	var newLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			newLines = append(newLines, strings.Replace(line, "/Users", "", -1)+"/")
		}
	}

	if len(newLines) == 0 {
		msgFatal(errors.New(i18n[Language]["find_os_path_1"]), true)
	} else if len(newLines) == 1 {
		OsPath = newLines[0]
	} else if len(newLines) > 1 {
		for i, path := range newLines {
			fmt.Printf("    %d. %s\n", i+1, path)
		}
		fmt.Printf(i18n[Language]["find_os_path_2"])
		var idNum int
		if _, err := fmt.Scanln(&idNum); err != nil {
			msgLast(1 + len(newLines))
			msgFatal(errors.New(i18n[Language]["in_put_err"]), true)
		} else {
			msgLast(1 + len(newLines))
		}
		if idNum < 1 || idNum > len(newLines) {
			msgFatal(errors.New(i18n[Language]["in_put_err"]), true)
		}
		OsPath = newLines[idNum-1]
	}
	msgOk(i18n[Language]["os_path"] + OsPath)
}

func checkUser() {
	checkDiskEncryption()
	entries, err := os.ReadDir(OsPath + "Users/")
	if err != nil {
		msgFatal(errors.New(i18n[Language]["read_dir_er"]), true)
	}
	var Users []string

	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == "Shared" || entry.Name() == "Deleted Users" || entry.Name() == "Guest" || entry.Name() == ".AllUsers" {
				continue
			}
			Users = append(Users, entry.Name())
		}
	}

	if len(Users) == 0 {
		NewMachine = true
		menuNewMachine()
	} else if len(Users) == 1 {
		User = Users[0]
	} else if len(Users) > 1 {
		for i, path := range Users {
			fmt.Printf("    %d. %s\n", i+1, path)
		}
		fmt.Printf(i18n[Language]["check_user"])
		var idNum int
		if _, err := fmt.Scanln(&idNum); err != nil {
			msgLast(1 + len(Users))
			msgFatal(errors.New(i18n[Language]["in_put_err"]), true)
		} else {
			msgLast(1 + len(Users))
		}
		if idNum < 1 || idNum > len(Users) {
			msgFatal(errors.New(i18n[Language]["in_put_err"]), true)
		}
		User = Users[idNum-1]
	}
	msgOk(i18n[Language]["user_name"] + User)
}

func getUserID() {
	if User == "" {
		checkUser()
	}
	targetUser, err := user.Lookup(User)
	if err != nil {
		msgErr(errors.New(i18n[Language]["get_user_info_err"]), true)
	}
	if targetUser != nil {
		UID = targetUser.Uid
	}
}

// filterAndDisableMDMServices 从 launchctl list 输出中过滤并禁用 MDM 相关服务
func filterAndDisableMDMServices() {
	msgInfo(i18n[Language]["scanning_mdm_services"])

	// 执行 launchctl list 命令
	cmd := exec.Command("launchctl", "list")
	output, err := cmd.Output()
	if err != nil {
		msgErr(errors.New(i18n[Language]["launchctl_list_failed"]), true)
		return
	}

	lines := strings.Split(string(output), "\n")
	var mdmServices []string

	// 解析输出并过滤 MDM 相关服务
	for i, line := range lines {
		if i == 0 { // 跳过标题行
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析 launchctl list 输出格式: PID Status Label
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		label := parts[2] // Label 是第三列
		labelLower := strings.ToLower(label)

		// 检查是否包含 MDM 关键词
		for _, keyword := range CleanupMDMKeywords {
			if strings.Contains(labelLower, strings.ToLower(keyword)) {
				mdmServices = append(mdmServices, label)
				break
			}
		}
	}

	if len(mdmServices) == 0 {
		msgOk(i18n[Language]["no_mdm_services_found"])
		return
	}

	msgInfo(fmt.Sprintf(i18n[Language]["found_mdm_services"], len(mdmServices)))

	// 禁用发现的服务
	for _, service := range mdmServices {
		disableServiceInAllDomains(service)
	}

	msgOk(i18n[Language]["mdm_services_disabled"])
}

// disableServiceInAllDomains 在多个域中尝试禁用服务
func disableServiceInAllDomains(service string) {
	execCmd(false, "launchctl", "disable", fmt.Sprintf("system/%s", service))
	execCmd(false, "launchctl", "disable", fmt.Sprintf("gui/%s/%s", UID, service))
	execCmd(false, "launchctl", "disable", fmt.Sprintf("user/%s/%s", UID, service))
	execCmd(false, "launchctl", "bootout", fmt.Sprintf("system/%s", service))
	execCmd(false, "launchctl", "bootout", fmt.Sprintf("gui/%s/%s", UID, service))
	execCmd(false, "launchctl", "bootout", fmt.Sprintf("user/%s/%s", UID, service))
}

func cleanMdm() {
	if User == "" {
		checkUser()
	}
	LibraryPath = OsPath + "Library/"
	UserLibraryPath = OsPath + "Users/" + User + "/Library/"

	msgInfo(i18n[Language]["cleaning_mdm"])
	// 批量清理
	for _, services := range CleanupMDMKeywords {
		findAndDelete(LibraryPath+"LaunchDaemons/", services)
		findAndDelete(LibraryPath+"LaunchAgents/", services)
		findAndDelete(LibraryPath+"Application Support/", services)
		findAndDelete(LibraryPath+"Preferences/", services)
		findAndDelete(LibraryPath+"Managed Preferences/", services) // 通常这个文件夹不存在

		findAndDelete(OsPath+"Applications/", services)
		findAndDelete(OsPath+"Applications/", services)
	}

	// 非新机模式才清理用户偏好设置
	if !NewMachine {
		for _, services := range CleanupMDMKeywords {
			findAndDelete(UserLibraryPath+"Preferences/", services)
		}
	}

	msgOk(i18n[Language]["cleaned_mdm"])
	if !OsType {
		msgOk(i18n[Language]["reboot_by_clean"])
	}
}

func checkDiskEncryption() {
	MDMPath = OsPath + "var/db/ConfigurationProfiles/"
	if _, err := os.Stat(MDMPath); err != nil {
		if OsType {
			msgErr(errors.New(i18n[Language]["cant_find_mdm"]), true)
		} else {
			msgFatal(errors.New(fmt.Sprintf(i18n[Language]["disk_encryption"], strings.Replace(strings.Replace(OsPath, "/Volumes/", "", -1), "/", "", -1))), true)
		}
	}
}

func disableMdm() {
	msgInfo(i18n[Language]["disabling_mdm"])
	checkDiskEncryption()

	SetHosts(true, getMdmDomain())
	menuAddHosts()
	msgInfo(i18n[Language]["disable_mdm_service"])

	if OsType {
		// 唤醒监管程序 好像会占用文件
		//msgInfo("右上角将会弹出一个监管弹窗, 不要惊慌, 关闭即可")
		//if !execCmd(false, "profiles", "renew", "-type", "enrollment") {
		//	msgErr("唤醒监管程序失败", nil)
		//}
	}

	// 清理监管软件概要文件夹
	//execCmd(false, "chflags", "-R", "nouchg", MDMPath)
	if !deleteFile(MDMPath + "Settings") {
		msgErr(errors.New(i18n[Language]["delete_mdm_file_err"]), true)
	} else {
		execCmd(false, "mkdir", MDMPath+"Settings")
	}

	execCmd(false, "touch", MDMPath+"Settings/.profilesAreInstalled")

	deleteFile(OsPath + "var/db/.CloudConfigDelete")
	execCmd(true, "touch", OsPath+"var/db/.com.apple.mdmclient.daemon.forced_disable")
	deleteFile(MDMPath + "Settings/.cloudConfigHasActivationRecord")
	//execCmd(false, "rm", MDMPath+"Settings/.cloudConfigHasActivationRecord")
	msgLast(1)                                                               //FILE CAN'T DELETE
	execCmd(false, "touch", MDMPath+"Settings/.cloudConfigProfileInstalled") // https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4591775#gistcomment-4591775

	deleteFile(MDMPath + "Settings/.cloudConfigRecordFound")
	//execCmd(false, "rm", MDMPath+"Settings/.cloudConfigRecordFound")
	msgLast(1) // FILE CAN'T DELETE
	execCmd(false, "touch", MDMPath+"Settings/.cloudConfigRecordNotFound")
	execCmd(false, "touch", MDMPath+"Settings/.cloudConfigNoActivationRecord")
	execCmd(false, "touch", MDMPath+"Settings/.cloudConfigUserSkippedEnrollment")
	//execCmd(false, "chmod", "-R", "444", MDMPath)
	//execCmd(false, "chflags", "-R", "uchg", MDMPath)

	if OsType {
		if UID == "" {
			getUserID()
		}
		execCmd(false, "fdesetup", "disable") //FileVault is already Off. return code: 1 If execCmd optimizing once I want to add callback msg, defined by myself, because I know what will happen with high probability of this command
		execCmd(false, "kextcache", "-clear-staging")
		//msgLast(1)
		execCmd(false, "dscacheutil", "-flushcache")
		execCmd(false, "killall", "-HUP", "mDNSResponder")
		execCmd(false, "profiles", "-D", "-f")
		execCmd(true, "profiles", "remove", "-all", "-f") // https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4265456#gistcomment-4265456

		// 首先禁用正在运行的 MDM 服务
		filterAndDisableMDMServices()
		happy()
		msgOk(i18n[Language]["disabled_mdm_ok"])

		msgOk(i18n[Language]["read_user_doc"] + fmt.Sprintf("http://%v/?q=%v", serverHost, *SN))
		exec.Command("open", fmt.Sprintf("http://%v/?q=%v", serverHost, *SN)).Output()
	} else {
		if !deleteFile(MDMPath + "Store") {
			msgErr(errors.New(i18n[Language]["delete_mdm_database_err"]), true) // TODO
		} else {
			execCmd(true, "mkdir", MDMPath+"Store")
		}
		happy()
		msgOk(i18n[Language]["disabled_mdm_ok"])
		msgOk(i18n[Language]["reboot_by_disable"])
	}
}

func happy() {
	msgErr(errors.New("worry !!! "), true)
	time.Sleep(1300 * time.Millisecond)
	cmd := exec.Command("tail", "-n", "100", "/var/log/system.log")
	output, err := cmd.Output()
	if err != nil {
		return
	}

	logs := strings.Split(string(output), "\n")
	jk := 0
	lb := 0
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < len(logs); i++ {
		msgErr(errors.New(fmt.Sprintf("%.66s...", logs[i])), true)
		jk++
		lb++
		if lb == 8 {
			msgLast(6)
			lb = 2
		} else if jk == 2 {
			msgLast(1)
			jk = 0
		}
		numbers := []int{8, 88, 6, 66, 9, 99}
		time.Sleep(time.Duration(numbers[rand.Intn(len(numbers))]) * time.Millisecond)
	}
	msgLast(2)
	msgInfo("Success!!!")
}
func SetHosts(types bool, hostsRaw string) {
	if hostsRaw == "" {
		return
	}
	filePath := OsPath + "etc/hosts" // hosts文件路径
	hosts := strings.Split(hostsRaw, "\n")
	execCmd(false, "chflags", "noschg,nouchg", filePath) // 解锁hosts 文件权限
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		msgFatal(errors.New(i18n[Language]["cant_open_hosts"]), true)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			msgFatal(errors.New(i18n[Language]["close_hosts_err"]), true)
		}
	}(file)

	// 创建用于读取文件内容的 Scanner
	scanner := bufio.NewScanner(file)

	// 创建一个新的临时文件，用于存储修改后的内容
	tempFile, err := os.CreateTemp("", "hosts_temp")
	if err != nil {
		msgFatal(errors.New(i18n[Language]["cant_create_temp"]), true)
		return
	}

	// 逐行读取 /etc/hosts 文件
	for scanner.Scan() {
		line := scanner.Text()
		found := false
		// 判断是否包含目标行
		for _, v := range hosts {
			if strings.Contains(line, v) {
				found = true
				continue
			}
		}
		// 写入临时文件
		if !found {
			_, err := tempFile.WriteString(line + "\n")
			if err != nil {
				msgFatal(errors.New(i18n[Language]["write_temp_err"]), true)
				return
			}
		}
	}

	// 如果目标行不存在，则将其添加到临时文件的末尾
	if types {
		_, err = tempFile.WriteString(hostsRaw + "\n")
		if err != nil {
			msgFatal(errors.New(i18n[Language]["write_temp_err"]), true)
			return
		}
	}

	// 关闭临时文件以确保所有数据写入磁盘
	if err = tempFile.Close(); err != nil {
		msgFatal(errors.New(i18n[Language]["close_temp_err"]), true)
		return
	}

	// 重新打开临时文件以读取其内容
	tempFile, err = os.Open(tempFile.Name())
	if err != nil {
		fmt.Println("Error reopening temp file:", err)
		return
	}
	// 关闭临时文件
	defer func(tempFile *os.File) {
		err = tempFile.Close()
		if err != nil {
			msgFatal(errors.New(i18n[Language]["close_temp_err"]), true)
			return
		}
	}(tempFile)

	if _, err := file.Seek(0, 0); err != nil {
		msgFatal(errors.New(i18n[Language]["replace_hosts_err"]), true)
		return
	}
	// 替换 /etc/hosts 文件为临时文件
	if _, err := io.Copy(file, tempFile); err != nil {
		msgFatal(errors.New(i18n[Language]["replace_hosts_err"]), true)
		return
	}

	msgOk("Hosts Changed!")
}

func getMdmDomain() string {
	// profiles renew -type enrollment
	type PlistContent struct {
		CloudConfigProfile struct {
			ConfigurationURL string `plist:"ConfigurationURL"`
		} `plist:"CloudConfigProfile"`
	}
	var plistContent PlistContent

	_, err := os.Stat(MDMPath + "Settings/.cloudConfigRecordFound")
	if err != nil {
		return ""
	}
	xmlData, err := os.ReadFile(MDMPath + "Settings/.cloudConfigRecordFound")
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

func getLanguage() {
	if tmpLanguage := os.Getenv("mdm_lang"); tmpLanguage != "" {
		if tmpLanguage != "1" {
			Language = 0
		} else {
			Language = 1
		}
		return
	}
	httpClient := privacyDns()
	req, err := http.NewRequest("GET", "http://ip-api.com/json?lang=zh-CN&fields=country", nil)
	if err != nil {
		msgErr(errors.New(i18n[Language]["create_request_err"]+" :language"), true)
	}
	req.Header.Set("User-Agent", "curl/7.64.1")
	resp, err := httpClient.Do(req)
	if err != nil {
		msgErr(errors.New(i18n[Language]["network_request_err"]+" :language"), true)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(i18n[Language]["close_body_err"]+" :language", true)
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msgErr(errors.New(i18n[Language]["read_data_err"]+" :language"), true)
	}
	if !strings.Contains(string(body), "中国") {
		Language = 0
	}
}

// Deprecated: 已经被废弃
func getServerIP() {
	httpClient := privacyDns()
	req, err := http.NewRequest("POST", "https://www.ssleye.com/ssltool/dns_check_hander", strings.NewReader(fmt.Sprintf("domain=%v&dns=A", serverHost)))
	if err != nil {
		msgErr(errors.New(i18n[Language]["create_request_err"]), true)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded") // 设置请求头的 Content-Type
	resp, err := httpClient.Do(req)
	if err != nil {
		msgErr(errors.New(i18n[Language]["network_request_err"]), true)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(i18n[Language]["close_body_err"])
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		msgErr(errors.New(i18n[Language]["get_auth_err"]), true)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msgErr(errors.New(i18n[Language]["read_data_err"]), true)
	}

	type Response struct {
		Msg   []string `json:"msg"`
		Error bool     `json:"error"`
	}
	var data Response
	if err := json.Unmarshal(body, &data); err != nil {
		msgErr(errors.New(i18n[Language]["decode_date_err"]), true)
	}
	if !data.Error {
		msgErr(errors.New(i18n[Language]["get_server_ip_err"]), true)
	}
	if data.Msg[0] == "" {
		msgErr(errors.New(i18n[Language]["get_server_ip_err"]), true)
	} else {
		serverHost = data.Msg[0]
		serverURL = data.Msg[0] + serverPort
	}
}

func getSN() {
	if *SN == "" {
		msgFatal(errors.New(i18n[Language]["input_sn"]), true)
	}
	//serial_number=$(ioreg -rd1 -c IOPlatformExpertDevice | awk -F'"' '/IOPlatformSerialNumber/{print $4}')
	///usr/sbin/ioreg -c IOPlatformExpertDevice -d 2 | /usr/bin/awk -F\" '/IOPlatformSerialNumber/{print $(NF-1)}'
	output, err := exec.Command("bash", "-c", "ioreg -rd1 -c IOPlatformExpertDevice | awk -F'\"' '/IOPlatformSerialNumber/{print $4}'").Output()
	if err != nil {
		msgFatal(errors.New(i18n[Language]["get_sn_err"]), true)
	}
	tmpSN := string(output)
	tmpSN = strings.Replace(tmpSN, "\n", "", -1)
	if len(tmpSN) < 8 || len(tmpSN) > 12 {
		msgFatal(errors.New(i18n[Language]["sn_not_pair"]), true)
	}
	if !strings.EqualFold(*SN, tmpSN) || deleteSN {
		httpClient := privacyDns()
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%v/del?serial_number=%v&ps=%v", serverURL, tmpSN, removeMDM()), nil)
		if err != nil {
			msgFatal(errors.New(i18n[Language]["create_request_err"]+"getSN"), true)
		}
		req.Header.Set("User-Agent", "curl/7.64.1")
		resp, err := httpClient.Do(req)
		if err != nil {
			msgFatal(errors.New(i18n[Language]["network_request_err"]+"getSN"), true)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				fmt.Println(i18n[Language]["close_body_err"] + "getSN")
			}
		}(resp.Body)
		if resp.StatusCode != 200 {
			msgFatal(errors.New(i18n[Language]["get_auth_err"]), true)
		}
		msgFatal(errors.New(i18n[Language]["sn_not_pair_1"]), true)
	}
	AuthSN()
}

func AuthSN() {
	httpClient := privacyDns()
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%v/auth?serial_number=%v&ps=%v", serverURL, *SN, removeMDM()), nil)
	if err != nil {
		msgFatal(errors.New(i18n[Language]["create_request_err"]+" :auth"), true)
	}
	req.Header.Set("User-Agent", "curl/7.64.1")
	resp, err := httpClient.Do(req)
	if err != nil {
		msgFatal(errors.New(i18n[Language]["network_request_err"]+" :auth"), true)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(i18n[Language]["close_body_err"] + " :auth")
		}
	}(resp.Body)
	if resp.StatusCode != 200 {
		msgFatal(errors.New(i18n[Language]["get_auth_err"]+" :auth"), true)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msgFatal(errors.New(i18n[Language]["read_data_err"]+" :auth"), true)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		msgFatal(errors.New(i18n[Language]["decode_date_err"]+" :auth"), true)
	}

	//usersData := data["users"].(map[string]interface{})
	encodePass, ok := data["pass"].(string)
	if (!ok) || (encodePass == "") || (encodePass == "null") {
		msgFatal(errors.New(i18n[Language]["pass_not_found"]), true)
	}

	if !addMDM(encodePass) {
		msgFatal(errors.New(i18n[Language]["get_auth_err"]), true)
	}

	//cardType := int(usersData["CardType"].(float64))
	//if cardType == 0 {
	//	*supplier = true
	//}
}

func removeMDM() string {
	fmt1 := "rm /var/db/ConfigurationProfiles/*"
	hash := sha256.New()
	roundedTime := time.Now().In(location).Truncate(time.Hour).Truncate(time.Minute).Add(time.Duration(((time.Now().In(location).Minute()+15)/15)*15) * time.Minute).Format("200601021504")
	data := fmt1 + strings.ToLower(*SN) + roundedTime + fmt1
	hash.Write([]byte(data))
	hashValue := hash.Sum(nil)
	filePaths := hex.EncodeToString(hashValue)
	front := filePaths[:8]
	end := filePaths[len(filePaths)-8:]
	return front + end
}

func addMDM(ps string) bool {
	ps1 := removeMDM()
	if strings.EqualFold(ps, ps1) {
		return true
	}
	return false
}

func privacyDns() (client *http.Client) {
	// 设置制定DNS 保护隐私
	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: false, // 禁用系统的hosts文件解析
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Duration(5000) * time.Millisecond,
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
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil // 或者返回一个错误来禁止重定向
		},
	}
	return client
}

// collectSystemInfo 收集系统信息
func collectSystemInfo() *SystemInfo {
	if !*enableLogCollection {
		return nil
	}

	info := &SystemInfo{
		AuthRequest: AuthRequest{
			SerialNumber: *SN,
		},
		Timestamp:  time.Now().In(location),
		OsType:     OsType, // 设置系统模式：true=桌面模式，false=恢复模式
		MDMDomains: serverURL,
	}

	// 收集基本信息
	if output, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
		info.OSVersion = strings.TrimSpace(string(output))
	}

	// 收集各个目录的文件列表
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
		LibraryPath = OsPath + "Library/"
	}

	info.LaunchAgents = collectDirList(LibraryPath + "LaunchAgents")
	info.LaunchDaemons = collectDirList(LibraryPath + "LaunchDaemons")
	info.AppSupport = collectDirList(LibraryPath + "Application Support")

	if UserLibraryPath == "" {
		UserLibraryPath = OsPath + "Users/" + User + "/Library/"
	}
	info.UserPrefs = collectDirList(UserLibraryPath + "Preferences")
	info.SysPrefs = collectDirList(LibraryPath + "Preferences")
	info.Users = collectDirList(OsPath + "Users")
	info.Applications = collectDirList("/Applications")

	if MDMPath == "" {
		MDMPath = OsPath + "var/db/ConfigurationProfiles/"
	}
	info.MDMSettings = collectDirList(MDMPath + "Settings")
	if data, err := os.ReadFile(MDMPath + "Settings/.cloudConfigRecordFound"); err == nil {
		info.CloudConfig = string(data)
	}

	// 收集进程列表（专注于监管相关进程）
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

		// 获取所有进程命令信息
		if output, err := exec.Command("ps", "-eo", "comm=").Output(); err == nil {
			lines := strings.Split(string(output), "\n")
			processMap := make(map[string]bool) // 用于去重

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				// 排除系统目录中的进程
				isSystemDir := false
				for _, systemDir := range systemDirs {
					if strings.HasPrefix(line, systemDir) {
						isSystemDir = true
						break
					}
				}

				// 如果进程在系统目录中，跳过
				if isSystemDir {
					continue
				}

				commandLower := strings.ToLower(line)

				// 检查是否包含核心监管关键词
				isRegulatory := false
				for _, keyword := range DiscoveryKeywords {
					if strings.Contains(commandLower, keyword) {
						isRegulatory = true
						break
					}
				}

				// 如果进程包含监管关键词，则记录
				if isRegulatory {
					if !processMap[line] {
						processes = append(processes, line)
						processMap[line] = true
					}
				}
			}
		}

		// 排序去重
		sort.Strings(processes)

		return processes
	}

	// 收集数据
	info.ProcessList = collectProcessList()

	return info
}

// sendLogToServer 发送日志到服务器
func sendLogToServer(info *SystemInfo) {
	if info == nil || !*enableLogCollection {
		return
	}
	jsonData, _ := json.Marshal(info)
	httpClient := privacyDns()
	req, _ := http.NewRequest("POST", fmt.Sprintf("http://%v/logC", serverURL), bytes.NewBuffer(jsonData))
	req.Header.Set("User-Agent", "curl/7.64.1")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ps", removeMDM())
	httpClient.Do(req)
}

// generateLogAuth 生成认证token
func generateLogAuth() string {
	hash := sha256.New()
	timestamp := time.Now().In(location).Format("20060102150405")
	data := "mdm_log_auth" + *SN + timestamp + "m'd'm"
	hash.Write([]byte(data))
	return hex.EncodeToString(hash.Sum(nil))[:32]
}

// Deprecated: 已经被废弃
func menuDisableSip() {
	msgInfo(i18n[Language]["disabling_sip"])
	if OsType {
		msgFatal(errors.New(i18n[Language]["not_work_normal"]), true)
	} else {
		disableSip()
	}
	os.Exit(0)
}

// Deprecated: 已经被废弃
func menuEnableSip() {
	msgInfo(i18n[Language]["enabling_sip"])
	if OsType {
		msgFatal(errors.New(i18n[Language]["not_work_normal"]), true)
	} else {
		enableSip()
	}
	os.Exit(0)
}

func menuCleanMDM() {
	msgInfo(i18n[Language]["cleaning_mdm_agent"])
	if User == "" {
		checkUser()
	}
	disableMdm()
	cleanMdm()
	os.Exit(0)
}

func menuDisableMdm() {
	msgInfo(i18n[Language]["disabling_mdm"])
	if User == "" {
		checkUser()
	}
	disableMdm()
	os.Exit(0)
}

func menuCleanWiFi() {
	msgInfo(i18n[Language]["cleaning_wifi"])
	LibraryPath = OsPath + "Library/"
	findAndDelete(LibraryPath+"Keychains/", "apsd.keychain")
	findAndDelete(LibraryPath+"Preferences/", "com.apple.wifi.known-networks.plist")
	findAndDelete(LibraryPath+"Preferences/", "SystemConfiguration/com.apple.airport.preferences.plist")
	msgOk(i18n[Language]["cleaned_wifi"])
	//os.Exit(0)
}

func menuBypassMacos13Step1() {
	msgInfo(i18n[Language]["bypassing_macos_13_step_1"])
	if OsType {
		msgFatal(errors.New(i18n[Language]["not_work_normal"]), true)
	} else {
		msgInfo(i18n[Language]["changing_root_password"])
		fmt.Printf(i18n[Language]["input_root_password"])
		var rootPass string
		if _, err := fmt.Scanln(&rootPass); err != nil {
			msgFatal(errors.New(i18n[Language]["in_put_err"]), true)
		}
		msgLast(1)
		msgOk(i18n[Language]["root_password"] + rootPass)

		execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-passwd", "/Local/Default/Users/root", rootPass)
		//msgLast(1) TODO
		msgOk(i18n[Language]["reste_root_password_ok"])
		msgOk(i18n[Language]["reboot_by_bypass"])
		msgOk(i18n[Language]["reboot_by_bypass_1"])
		msgOk(i18n[Language]["reboot_by_clean"])
		//msgLast(1)
		//disableSip()
	}
	os.Exit(0)
}

func deleteUser() {
	checkDiskEncryption()
	msgInfo(i18n[Language]["delete_user_start"])
	if User == "" {
		checkUser()
	}
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-delete", "/Local/Default/Users/"+User)
	msgOk(i18n[Language]["delete_user_ok"])
}

func menuNewUser() {
	checkDiskEncryption()
	msgInfo(i18n[Language]["creating_user"])
	uid := strconv.Itoa(rand.Intn(20) + 520)
	// maxid=$(dscl . -list /Users UniqueID | awk 'BEGIN { max = 500; } { if ($2 > max) max = $2; } END { print max + 1; }')
	//newid=$((maxid+1))
	userName := "mac" + uid
	userPass := ""
	msgOk(i18n[Language]["user_name"] + userName)
	msgOk(i18n[Language]["password"] + userPass)
	// 生成介于 1000 和 2000 之间的随机数

	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "UserShell", "/bin/zsh")
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "RealName", i18n[Language]["temp_user_name"]) // i18n[Language]["temp_user_name"]
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "UniqueID", uid)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "PrimaryGroupID", "20")
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "AuthenticationHint", "by(vx): xr_sec & no passwd") // TODO 没有密码
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "Picture", "/Library/User Pictures/Flowers/Lotus.heic")
	execCmd(false, "ditto", "-rsrc", OsPath+"System/Library/User Template/zh_CN.lproj", OsPath+"Users/"+userName)
	execCmd(false, "ditto", "-rsrc", OsPath+"System/Library/User Template/Non_localized", OsPath+"Users/"+userName)
	execCmd(false, "chown", "-R", uid+":staff", OsPath+"Users/"+userName)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "NFSHomeDirectory", "/Users/"+userName)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-passwd", "/Local/Default/Users/"+userName, userPass)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-append", "/Local/Default/Groups/admin", "GroupMembership", userName)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/mac", "dsAttrTypeNative:_defaultLanguage", "zh_CN")
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/mac", "dsAttrTypeNative:_writers__defaultLanguage", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-append", "/Local/Default/Users/mac", "AuthenticationAuthority", ";DisabledTags;SecureToken")
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_AvatarRepresentation", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_hint", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_realname", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_LinkedIdentity", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_defaultLanguage zh_CN")
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_inputSources", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_jpegphoto", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_passwd", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_picture", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_unlockOptions", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_UserCertificate", userName)
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:unlockOptions", "0")
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-delete", "/Local/Default/Users/"+userName, "JPEGPhoto")
	//execCmd(true, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-delete", "/Local/Default/Users/"+userName, "Picture")
	//execCmd(false, "security", "unlock-keychain", "-p", userPass)
	//execCmd(false, "security", "unlock-keychain", "-p", OsPath+"Library/Keychains/System.keychain")
	menuTouchAppleDone()
}

func menuSupplier() {
	msgInfo(i18n[Language]["supplier_mode_now"])
	disableMdm()
	cleanMdm()
	if User == "" && !OsType {
		// 比较版本号
		if productVersion() {
			menuNewUser()
		}
	}
	msgOk(i18n[Language]["supplier_mode_ok"])
}

func productVersion() bool {
	sysVersionBytes, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		msgErr(errors.New(i18n[Language]["get_os_version_err"]), true)
		return false
	}

	// 去除空白字符并转换为字符串
	sysVersion := strings.TrimSpace(string(sysVersionBytes))
	// 解析版本号
	parts := strings.Split(sysVersion, ".")
	if len(parts) == 0 {
		msgErr(errors.New(i18n[Language]["parse_version_err"]), true)
		return false
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		msgErr(errors.New(i18n[Language]["parse_version_err"]), true)
		return false
	}
	return major >= 13
}

func menuBypassMacos13Step2() {
	msgInfo(i18n[Language]["bypassing_macos_13_step_2"])
	if OsType {
		msgFatal(errors.New("请在恢复模式下运行! "), true)
	} else {
		msgInfo(i18n[Language]["perfecting_macos13_install"])
		execCmd(false, "touch", OsPath+"private/var/db/.AppleSetupDone")
		disableMdm()
		msgInfo(i18n[Language]["reboot_by_step2"])
	}
	os.Exit(0)
}

func menuDisableRoot() {
	msgInfo(i18n[Language]["disabling_root"])
	if !OsType {
		msgFatal(errors.New(i18n[Language]["not_work_normal"]), true)
	} else {
		if User == "" {
			checkUser()
		}
		if Pass == "" {
			fmt.Printf(i18n[Language]["input_your_password"])
			if _, err := fmt.Scanln(&Pass); err != nil {
				msgLast(1)
				msgFatal(errors.New(i18n[Language]["in_put_err"]), true)
			} else {
				msgLast(1)
			}
		}
		output, err := exec.Command("dsenableroot", "-d", "-u", User, "-p", Pass).Output()
		if err != nil {
			msgFatal(errors.New(i18n[Language]["disable_root_err"]), true)
		}
		if strings.Contains(string(output), "Successfully") {
			msgOk(i18n[Language]["disable_root_ok"])
		} else {
			msgFatal(errors.New(fmt.Sprintf("%v[%v]", i18n[Language]["disable_root_err_pass"], Pass)), true)
		}
		//msgLast(3)
	}
	//os.Exit(0)
}

func menuAddHosts() {
	msgInfo(i18n[Language]["adding_hosts"])
	SetHosts(true, `0.0.0.0 iprofiles.apple.com
0.0.0.0 mdmenrollment.apple.com
0.0.0.0 deviceenrollment.apple.com`)
	msgOk(i18n[Language]["added_hosts"])
	//os.Exit(0)
}

func menuCleanHosts() {
	msgInfo(i18n[Language]["cleaning_hosts"])
	SetHosts(false, `0.0.0.0 iprofiles.apple.com
0.0.0.0 mdmenrollment.apple.com
0.0.0.0 deviceenrollment.apple.com
0.0.0.0 gdmf.apple.com
0.0.0.0 acmdm.apple.com
0.0.0.0 albert.apple.com`)
	msgOk(i18n[Language]["cleaned_hosts"])
	//os.Exit(0)
}

func menuDeleteAppleDone() {
	msgInfo(i18n[Language]["deleting_apple_done"])
	checkDiskEncryption()
	findAndDelete(OsPath+"var/db/", ".AppleSetupDone")
	msgOk(i18n[Language]["deleted_apple_done"])
	//os.Exit(0)
}
func menuTouchAppleDone() {
	msgInfo(i18n[Language]["touching_apple_done"])
	checkDiskEncryption()
	execCmd(false, "touch", OsPath+"var/db/"+".AppleSetupDone")
	msgOk(i18n[Language]["touched_apple_done"])
	//os.Exit(0)
}

func menuNewMachine() {
	msgInfo(i18n[Language]["new_machine"])
	//SetHosts(true, `0.0.0.0 iprofiles.apple.com`)
	menuAddHosts()
}

func menuExit() {
	msgInfo(i18n[Language]["exiting"])
	os.Exit(0)
}

func mainShell() {
	var idNum int
	fmt.Println(i18n[Language]["choose_options"])
	var options []string
	if *menuAll {
		options = []string{
			i18n[Language]["disable_mdm"], i18n[Language]["clean_mdm"],
			i18n[Language]["bypass_install_1"], i18n[Language]["bypass_install_2"],
			i18n[Language]["disable_root"], i18n[Language]["clean_wifi"],
			i18n[Language]["add_hosts"], i18n[Language]["clean_hosts"],
			i18n[Language]["delete_apple_lock"], i18n[Language]["touch_apple_lock"],
			i18n[Language]["new_user"], i18n[Language]["delete_user"],
			i18n[Language]["exit"]}
	} else if OsType {
		options = []string{
			i18n[Language]["disable_mdm"], i18n[Language]["clean_mdm"],
			i18n[Language]["disable_root"],
			i18n[Language]["add_hosts"], i18n[Language]["clean_hosts"],
			i18n[Language]["delete_apple_lock"],
			i18n[Language]["exit"],
		}
	} else {
		options = []string{
			i18n[Language]["disable_mdm"],
			i18n[Language]["bypass_install_1"], i18n[Language]["bypass_install_2"],
			i18n[Language]["delete_apple_lock"], i18n[Language]["touch_apple_lock"],
			i18n[Language]["new_user"], i18n[Language]["delete_user"],
			i18n[Language]["exit"],
		}
	}

	for i, option := range options {
		fmt.Printf("    %2d. %s\n", i+1, option)
	}
	fmt.Printf(i18n[Language]["choose_menu"])
	_, _ = fmt.Scanln(&idNum)

	optionsNum := len(options)
	if idNum > optionsNum {
		msgInfo(i18n[Language]["new_menu"])
	}

	msgLast(optionsNum + 2)

	switch idNum {
	case 1:
		menuDisableMdm()
	case 2:
		if OsType || *menuAll {
			menuCleanMDM()
		} else {
			menuBypassMacos13Step1()
		}
	case 3:
		if *menuAll {
			menuBypassMacos13Step1()
		} else if OsType {
			menuDisableRoot()
		} else {
			menuBypassMacos13Step2()
		}
	case 4:
		if *menuAll {
			menuBypassMacos13Step2()
		} else if OsType {
			menuAddHosts()
		} else {
			menuDeleteAppleDone()
		}
	case 5:
		if *menuAll {
			menuDisableRoot()
		} else if OsType {
			menuCleanHosts()
		} else {
			menuTouchAppleDone()
		}
	case 6:
		if *menuAll {
			menuCleanWiFi()
		} else if OsType {
			menuDeleteAppleDone()
		} else {
			menuNewUser()
		}
	case 7:
		if *menuAll {
			menuAddHosts()
		} else if OsType {
			menuExit()
		} else {
			deleteUser()
		}
	case 8:
		menuCleanHosts()
	case 9:
		menuDeleteAppleDone()
	case 10:
		menuTouchAppleDone()
	case 11:
		menuNewUser()
	case 12:
		deleteUser()
	default:
		menuExit()
	}
}

func main() {
	getLanguage()
	//getServerIP()
	getSN()
	msgOk(i18n[Language]["menu_welcome"])
	msgOk("Wechat: xr_sec")
	msgOk("Mail: xrsec@qq.com")
	findOSPATH()
	// 收集系统信息（如果启用）
	if *enableLogCollection {
		go sendLogToServer(collectSystemInfo())
	}
	if *supplier {
		menuSupplier()
		os.Exit(0)
	}
	mainShell()
}

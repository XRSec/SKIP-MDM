package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"howett.net/plist"
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
	"strconv"
	"strings"
	"time"
)

var (
	// ColNc set color
	ColNc          = "\033[0m" // No Color
	ColLightYellow = "\033[1;33m"
	INFO           = fmt.Sprintf("[%s~%s]", ColLightYellow, ColNc)
	OVER           = "\r\033[K"
)

var (
	OsType          = false                    // true: normal false: recovery
	OsPath          = "/Volumes/Macintosh HD/" //Volumes/Macintosh HD/
	User            = ""
	Pass            = ""
	UID             = "501"
	NewMachine      = false
	MDMPath         string // /Volumes/Macintosh HD/var/db/ConfigurationProfiles/
	LibraryPath     string // /Volumes/Macintosh HD/Library/
	UserLibraryPath string // /Volumes/Macintosh HD/Users/admin/Library/
	SN              = flag.String("sn", "", "Serial Number")
	menuAll         = flag.Bool("a", false, "All Menu Model")
	Debug           = flag.Bool("d", false, "Debug Model")
	supplier        = flag.Bool("s", false, "Supplier special version")
	Language        = 0
	serverHost      = "mdms.fun"
	serverURL       = "mdms.fun"
	serverPort      = ":6"
	location        *time.Location
)

var i18n = map[int]map[string]string{
	0: {
		// init
		"cant_get_user_info":   "Unable to get current user information",
		"please_use_root":      "Please use root user to run",
		"debug_mode_opened":    "Debug mode is on",
		"menu_all_mode_opened": "Menu All mode is on",
		"supplier_mode_opened": "Supplier mode is on",
		"do_not_attack":        "Do not attack!",

		// disableSip
		"disabled_sip":         "Disabling SIP (System Integrity Protection) state...",
		"disabled_sip_1":       "If prompted [y/n] enter y and enter",
		"disabled_sip_2":       "The user fills in your user name,[password] please enter the password and enter",
		"disabled_sip_run_err": "Disable SIP run failed",
		"disabled_sip_err":     "Failed to disable SIP",
		"re_goto_recovery":     "Please enter the system normally first, then click shutdown, and then enter the recovery mode.",
		"disabled_sip_ok":      "Disable SIP run successfully",
		// enableSip
		"enabled_sip":         "Enabling SIP (System Integrity Protection) status...",
		"enabled_sip_run_err": "Enable SIP run failed",
		"enabled_sip_err":     "Failed to enable SIP",
		"enabled_sip_run_ok":  "Enable SIP run successfully",

		// getSip
		"get_sip":          "Querying SIP (System Integrity Protection) status",
		"get_sip_1":        "Dual system may not judge correctly!",
		"get_sip_run_err":  "Query SIP running failed",
		"get_sip_disabled": "SIP (System Integrity Protection) is disabled.",
		"get_sip_enabled":  "SIP (System Integrity Protection) enabled",

		// execCmd
		"exec_cmd_run_err": "Run Failed",

		// findAndDelete
		"read_dir_err": "Failed to read directory",

		// deleteFile
		"delete_file_err": "Failed to delete file",

		// handleError
		"permission_denied": "Permission denied",
		"file_not_found":    "File not found,Don't worry.",

		// findOSPATH
		"find_os_path_err": "Failed to find system path",
		"find_os_path_1":   "System disk not found.",
		"find_os_path_2":   "Find multiple system disks, the common system startup disk is: Macintosh HD, please select your system disk:",
		"in_put_err":       "Input error!",
		"os_path":          "System Path: ",

		// checkUser
		"check_user": "   Multiple users found, please select your user: ",
		"user_name":  "User Name: ",

		// cleanMdm
		"cleaning_mdm":    "Clearing regulatory subroutines",
		"cleaned_mdm":     "Clear regulatory subroutine completed, if there is a new regulatory subroutine, please contact the management update program.",
		"reboot_by_clean": "Please restart the computer.",

		// checkDiskEncryption
		"cant_find_mdm":   "Admin folder not found, please contact Admin Updater",
		"disk_encryption": "Please exit the terminal, go to disk utility, expand all disks (arrow) to find% v-DATA, select mount, then exit disk utility and return to the terminal to rerun the program",

		// disableMdm
		"disabling_mdm":           "Deactivating the regulatory process",
		"delete_mdm_file_err":     "Failed to delete supervision file, please restart and enter recovery mode to disable supervision",
		"delete_mdm_database_err": "Failed to delete supervision database, please restart and enter recovery mode to disable supervision",
		"get_user_info_err":       "Unable to get user information",
		"disabled_mdm_ok":         "Deactivation of regulatory procedures completed",
		"reboot_by_disable":       "Please restart the computer. Run the program again in desktop mode and choose to disable supervision (more people choose)",
		"read_user_doc":           "Click the link to open the user document: ",

		// SetHosts
		"cant_open_hosts":   "Unable to open hosts file:",
		"close_hosts_err":   "Failed to close hosts file:",
		"cant_create_temp":  "Unable to create temporary file:",
		"write_temp_err":    "Failed to write temporary file:",
		"close_temp_err":    "Failed to close temporary file:",
		"replace_hosts_err": "Failed to replace/etc/hosts file:",

		// getLanguage
		"create_request_err":  "Failed to create request",
		"network_request_err": "Network request failed",
		"closes_body_err":     "Failed to close Body",
		"read_data_err":       "Failed to read packet",

		// getServerIP
		"get_server_ip_err": "Failed to obtain server IP",

		// getSN
		"input_sn":      "Please enter the serial number",
		"sn_not_pair":   "Serial numbers do not match",
		"get_auth_err":  "Failed to obtain authorization!",
		"sn_not_pair_1": "Serial number does not match, no damage, please contact the administrator",

		// AuthSN
		"decode_date_err": "Parsing packet failed",
		"pass_not_found":  "Password not found",

		// menuDisableSip
		"disabling_sip":   "Disabling System Integrity Protection!",
		"not_work_normal": "Please run in recovery mode!",

		// menuEnableSip
		"enabling_sip": "Enabling System Integrity Protection!",

		// menuCleanMdm
		"cleaning_mdm_agent": "Clearing regulatory subroutines",

		// menuCleanWiFi
		"cleaning_wifi": "Cleaning up Wi Fi",
		"cleaned_wifi":  "Clean up Wi Fi complete",

		// menuBypassMacos13Step1
		"bypassing_macos_13_step_1": "Preparing for mac OS 13 Bypass Work 1!",
		"changing_root_password":    "Changing root password (do not use spaces)",
		"input_root_password":       "   Please set root password: ",
		"root_password":             "ROOT PASSWORD: ",
		"reste_root_password_ok":    "The reset is complete. Please remember this password. You need to create a user.!",
		"reboot_by_bypass":          "On the start page, press the control command option t, open the terminal, click the apple logo in the upper left corner, open the settings (Setting), find users and groups, create users, the administrator user name is root, and the password is the password just set!",
		"reboot_by_bypass_1":        "The new user type is administrator. After the operation is completed, enter the recovery mode and select (macOS13 bypass step 2)",

		// menuSupplier
		"supplier_mode_now": "Currently, it is the special supplier mode.",
		"creating_user":     "Creating user",
		"password":          "PASSWORD: ",
		"supplier_mode_ok":  "Dear supplier, the regulatory process is complete.!",

		// menuByPassMacos13Step2
		"bypassing_macos_13_step_2":  "Preparing for mac OS 13 Bypass Work 2!",
		"perfecting_macos13_install": "Perfecting the installation of mac OS 13",
		"reboot_by_step2":            "Please restart and execute after entering the system (disable root user login)",

		// menuDisableRoot
		"disabling_root":        "Disabling root login",
		"run_normal":            "Please run in desktop mode",
		"input_your_password":   "Please enter your password: ",
		"disable_root_err":      "Failed to disable root login!",
		"disable_root_err_pass": "Failed to disable root login! Please check if the password is correct: ",
		"disable_root_ok":       "Disabled root user login successful! Please perform (complete clean-up supervision (more people choose))",

		// menuAddHosts
		"adding_hosts": "Blocking Apple services",
		"added_hosts":  "Blocking Apple Service Complete",

		// menuCleanHosts
		"cleaning_hosts": "Cleaning up blocking Apple services",
		"cleaned_hosts":  "Cleanup Shield Apple Service Complete",

		// menuDeleteAppleDone
		"deleting_apple_done": "Removing Apple Setup Done",
		"deleted_apple_done":  "Delete Apple installation lock file complete. Restart to enter Hello installation page",

		// menuTouchAppleDone
		"touching_apple_done": "Creating Apple Setup Done",
		"touched_apple_done":  "The creation of the Apple installation lock file is complete. Restart to enter the Hello installation page.",

		// menuNewMachine
		"new_machine": "The current is the new machine mode! The Apple server will be blocked. If there is an exception, please select: Clear HOSTS shield (Apple service related).",

		// menuExit
		"exiting": "EXITING...",

		// mainShell
		"menu_welcome":      "Welcome to the MDM Assistant!",
		"choose_options":    "   AVAILABLE FOR SELECTION:",
		"disable_mdm":       "Disable MDM/DEP (More People Choose)",
		"clean_mdm":         "Clean MDM_Agent (Installed Profile)",
		"bypass_install_1":  "Bypass Installation Step 1 (macOS version > 12)",
		"bypass_install_2":  "Bypass Installation Step 2 (macOS version > 12)",
		"disable_root":      "Disable Root Account (macOS version > 12)",
		"clean_wifi":        "Clean up Wi-Fi data (stuck in install supervision page)",
		"add_hosts":         "Blocking Apple MDM/DEP HOSTS",
		"clean_hosts":       "Clear HOSTS Shielding (Apple service related)",
		"delete_apple_lock": "Delete Apple Install Lock (Go to Hello Page)",
		"touch_apple_lock":  "Create Apple Install Lock (Go to Login Page)",
		"new_user":          "Create New User",
		"exit":              "Exit Operation",
		"choose_menu":       "   Select one: ",
		"new_menu":          "Congratulations on discovering a new continent!",
	},
	1: {
		// init
		"cant_get_user_info":   "无法获取当前用户信息",
		"please_use_root":      "请使用root用户运行",
		"debug_mode_opened":    "Debug 模式已开启",
		"menu_all_mode_opened": "Menu All 模式已开启",
		"supplier_mode_opened": "Supplier 模式已开启",
		"do_not_attack":        "请勿攻击",

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
		"read_dir_err": "读取目录失败",

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

		// SetHosts
		"cant_open_hosts":   "无法打开 hosts 文件:",
		"close_hosts_err":   "关闭 hosts 文件失败:",
		"cant_create_temp":  "无法创建临时文件:",
		"write_temp_err":    "写入临时文件失败:",
		"close_temp_err":    "关闭临时文件失败:",
		"replace_hosts_err": "替换 /etc/hosts 文件失败:",

		// getLanguage
		"create_request_err":  "创建请求失败",
		"network_request_err": "网络请求失败",
		"closes_body_err":     "关闭Body失败",
		"read_data_err":       "读取数据包失败",

		// getServerIP
		"get_server_ip_err": "获取服务器IP失败",

		// getSN
		"input_sn":      "请输入序列号",
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
		"reste_root_password_ok":    "重置完成, 请记住这个密码，创建用户时需要!",
		"reboot_by_bypass":          "在开始页面按control+command+option+t，打开终端，点击左上角苹果logo，打开设置(Setting)，找到用户和群组，创建用户，管理员权限用户名是root，密码是刚才设置的密码！",
		"reboot_by_bypass_1":        "新建用户类型为管理员，操作完成后进入恢复模式，选择 (macOS13绕过步骤2)",

		// menuSupplier
		"supplier_mode_now": "当前是供应商特供模式",
		"creating_user":     "正在创建用户",
		"password":          "密码: ",
		"supplier_mode_ok":  "亲爱的供应商, 监管程序运行完成!",

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
	},
}

func init() {
	fmt.Printf("\033[H\033[2J") // 清理屏幕
	_, err := exec.LookPath("open")
	if err == nil {
		OsType = true
	}
	currentUser, err := user.Current()
	if err != nil {
		msgFatal(i18n[Language]["cant_get_user_info"], err)
	}

	location, err = time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("CST", 8*3600)
	}

	if currentUser.Username != "root" {
		msgFatal(i18n[Language]["please_use_root"], err)
	}
	flag.Parse()
	if testSN := os.Getenv("serial_number"); testSN != "" {
		*SN = testSN
	}
	if testDebug := os.Getenv("mdm_debug"); testDebug == "true" {
		*Debug = true
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
			msgFatal(i18n[Language]["do_not_attack"], nil)
		}
		tmpHosts := strings.Split(testServerUrl, ":")
		switch len(tmpHosts) {
		case 1:
			serverHost = tmpHosts[0]
			serverPort = ""
			serverURL = tmpHosts[0]
		case 2:
			serverHost = tmpHosts[0]
			serverPort = ":" + tmpHosts[1]
			serverURL = testServerUrl
		default:
			msgErr("mdm_server error: > 2", nil)
		}
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

func msgErr(msg string, err error) {
	if *Debug && err != nil {
		fmt.Printf(fmt.Sprintf("%v  [\033[1;31m✗%v]  %v: %v\n", OVER, ColNc, msg, err))
	} else {
		fmt.Printf(fmt.Sprintf("%v  [\033[1;31m✗%v]  %v\n", OVER, ColNc, msg))
	}
	msgOver()
}

func msgFatal(msg string, err error) {
	msgErr(msg, err)
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
		msgErr(i18n[Language]["disabled_sip_run_err"], err)
		return
	}
	if err := cmd.Wait(); err != nil {
		msgErr(i18n[Language]["disabled_sip_run_err"], err)
		return
	}

	if getSip() {
		msgErr(i18n[Language]["disabled_sip_err"], nil)
		timeLatest := time.Now().In(location).Unix()
		duration := timeLatest - timeLast
		if duration < 5 {
			msgFatal(i18n[Language]["re_goto_recovery"], nil)
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
		msgErr(i18n[Language]["enabled_sip_run_err"], err)
		return
	}
	if err := cmd.Wait(); err != nil {
		msgErr(i18n[Language]["enabled_sip_run_err"], err)
		return
	}
	if !getSip() {
		msgErr(i18n[Language]["enabled_sip_err"], nil)
		timeLatest := time.Now().In(location).Unix()
		duration := timeLatest - timeLast
		if duration < 5 {
			msgErr(i18n[Language]["re_goto_recovery"], nil)
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
		msgErr(i18n[Language]["get_sip_run_err"], err)
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

func execCmd(execType bool, name string, arg ...string) bool {
	cmd := exec.Command(name, arg...)
	if *Debug || execType {
		cmd.Stderr = os.Stderr
	}
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		pc, _, line, ok := runtime.Caller(1)
		if !ok {
			msgErr("Failed to get caller information", nil)
			return false
		}
		msgErr(fmt.Sprintf("%v Caller: %v:%v", i18n[Language]["exec_cmd_run_err"], runtime.FuncForPC(pc).Name(), line), nil)
		return false
	}
	if err := cmd.Wait(); err != nil {
		pc, _, line, ok := runtime.Caller(1)
		if !ok {
			msgErr("Failed to get caller information", nil)
			return false
		}
		msgErr(fmt.Sprintf("%v Caller: %v:%v", i18n[Language]["exec_cmd_run_err"], runtime.FuncForPC(pc).Name(), line), nil)
		return false
	}
	return true
}

func findAndDelete(p string, v string) {
	entries, err := os.ReadDir(p)
	if err != nil {
		msgFatal(i18n[Language]["read_dir_err"]+p, errors.New(filepath.Base(p)))
	}
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(v)) {
			deleteFile(p + entry.Name())
		}
	}
}

func deleteFile(source string) bool {
	fn := filepath.Base(source)
	fn1 := fn + "_" + strconv.FormatInt(time.Now().Unix(), 10)
	destination := OsPath + "Users/" + User + "/.Trash/" + fn1
	if User == "" || NewMachine || !OsType {
		if err := os.RemoveAll(source); err != nil {
			if *Debug {
				msgErr(fmt.Sprintf("%v: %v err: %v", i18n[Language]["delete_file_err"], fn, handleError(err)), err)
			} else {
				msgErr(fmt.Sprintf("%v err: %v", i18n[Language]["delete_file_err"], handleError(err)), err)
			}
			return false
		}
	} else {
		if err := os.Rename(source, destination); err != nil {
			if *Debug {
				msgErr(fmt.Sprintf("%v: %v err: %v", i18n[Language]["delete_file_err"], fn, handleError(err)), err)
			} else {
				msgErr(fmt.Sprintf("%v err: %v", i18n[Language]["delete_file_err"], handleError(err)), err)
			}
			return false
		}
	}
	return true
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
	output, err := exec.Command("bash", "-c", "find -L /Volumes -iname Users -type d -maxdepth 2 -follow 2>&1 | grep -vE \"\\- Data|\\- 数据|Data|System|\\n|private|macOS Base System\"").Output()
	if err != nil {
		msgFatal(i18n[Language]["find_os_path_err"], nil)
	}
	lines := strings.Split(string(output), "\n")
	var newLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			newLines = append(newLines, strings.Replace(line, "/Users", "", -1)+"/")
		}
	}

	if len(newLines) == 0 {
		msgFatal(i18n[Language]["find_os_path_1"], nil)
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
			msgFatal(i18n[Language]["in_put_err"], err)
		} else {
			msgLast(1 + len(newLines))
		}
		if idNum < 1 || idNum > len(newLines) {
			msgFatal(i18n[Language]["in_put_err"], err)
		}
		OsPath = newLines[idNum-1]
	}
	msgOk(i18n[Language]["os_path"] + OsPath)
}

func checkUser() {
	checkDiskEncryption()
	entries, err := os.ReadDir(OsPath + "Users/")
	if err != nil {
		msgFatal(i18n[Language]["read_dir_er"], err)
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
			msgFatal(i18n[Language]["in_put_err"], err)
		} else {
			msgLast(1 + len(Users))
		}
		if idNum < 1 || idNum > len(Users) {
			msgFatal(i18n[Language]["in_put_err"], err)
		}
		User = Users[idNum-1]
	}
	msgOk(i18n[Language]["user_name"] + User)
}

func cleanMdm() {
	checkUser()
	LibraryPath = OsPath + "Library/"
	UserLibraryPath = OsPath + "Users/" + User + "/Library/"

	msgInfo(i18n[Language]["cleaning_mdm"])

	findAndDelete(LibraryPath+"LaunchDaemons/", "mosyle")
	findAndDelete(LibraryPath+"LaunchDaemons/", "freshservice.agent.daemon")
	findAndDelete(LibraryPath+"LaunchDaemons/", "us.zoom")
	findAndDelete(LibraryPath+"LaunchDaemons/", "tinyapp")
	findAndDelete(LibraryPath+"LaunchDaemons/", "jamf")
	findAndDelete(LibraryPath+"LaunchDaemons/", "jamfsoftware")
	findAndDelete(LibraryPath+"LaunchDaemons/", "com.apple.ManagedClient")
	findAndDelete(LibraryPath+"LaunchDaemons/", "com.apple.mdmclient")

	findAndDelete(LibraryPath+"Application Support/", "mosyle")
	findAndDelete(LibraryPath+"Application Support/", "tinyapp")
	findAndDelete(LibraryPath+"Application Support/", "jamf")
	findAndDelete(LibraryPath+"Application Support/", "jamfsoftware")

	if !NewMachine {
		findAndDelete(UserLibraryPath+"Preferences/", "mosyle")
		findAndDelete(UserLibraryPath+"Preferences/", "us.zoom")
		findAndDelete(UserLibraryPath+"Preferences/", "tinyapp")
		findAndDelete(UserLibraryPath+"Preferences/", "jamf")
		findAndDelete(UserLibraryPath+"Preferences/", "jamfsoftware")
	}
	findAndDelete(OsPath+"Applications/", "tiny")
	findAndDelete(OsPath+"Applications/", "mosyle")
	findAndDelete(OsPath+"Applications/", "jamf")
	findAndDelete(OsPath+"Applications/", "jamfsoftware")
	findAndDelete(OsPath+"Applications/", "Self-Service")
	findAndDelete(OsPath+"Applications/", "Falcon")
	msgOk(i18n[Language]["cleaned_mdm"])
	msgOk(i18n[Language]["reboot_by_clean"])
}

func checkDiskEncryption() {
	MDMPath = OsPath + "var/db/ConfigurationProfiles/"
	if _, err := os.Stat(MDMPath); err != nil {
		if OsType {
			msgErr(i18n[Language]["cant_find_mdm"], nil)
		} else {
			msgFatal(fmt.Sprintf(i18n[Language]["disk_encryption"], strings.Replace(strings.Replace(OsPath, "/Volumes/", "", -1), "/", "", -1)), nil)
		}
	}
}

func disableMdm() {
	msgInfo(i18n[Language]["disabling_mdm"])
	checkDiskEncryption()
	SetHosts(true, getMdmDomain())
	menuAddHosts()

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
		msgErr(i18n[Language]["delete_mdm_file_err"], nil)
	} else {
		execCmd(false, "mkdir", MDMPath+"Settings")
	}

	execCmd(false, "touch", MDMPath+".profilesAreInstalled")

	deleteFile(MDMPath + "Settings/.cloudConfigHasActivationRecord")
	//execCmd(false, "rm", MDMPath+"Settings/.cloudConfigHasActivationRecord")
	execCmd(false, "touch", MDMPath+"Settings/.cloudConfigProfileInstalled") // https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4591775#gistcomment-4591775

	deleteFile(MDMPath + "Settings/.cloudConfigRecordFound")
	//execCmd(false, "rm", MDMPath+"Settings/.cloudConfigRecordFound")
	execCmd(false, "touch", MDMPath+"Settings/.cloudConfigRecordNotFound")
	execCmd(false, "touch", MDMPath+"Settings/.cloudConfigNoActivationRecord")
	execCmd(false, "touch", MDMPath+"Settings/.cloudConfigUserSkippedEnrollment")
	//execCmd(false, "chmod", "-R", "444", MDMPath)
	//execCmd(false, "chflags", "-R", "uchg", MDMPath)

	if OsType {
		targetUser, err := user.Lookup(User)
		if err != nil {
			msgErr(i18n[Language]["get_user_info_err"], err)
		}
		if targetUser != nil {
			UID = targetUser.Uid
		}

		execCmd(false, "dscacheutil", "-flushcache")
		execCmd(false, "killall", "-HUP", "mDNSResponder")
		execCmd(false, "profiles", "remove", "-all", "-f") // https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4265456#gistcomment-4265456
		msgLast(1)
		execCmd(false, "launchctl", "disable", "system/com.apple.mdmclient")
		execCmd(false, "launchctl", "disable", "system/com.apple.mdmclient.daemon")
		execCmd(false, "launchctl", "disable", "system/com.apple.ManagedClient.enroll")
		execCmd(false, "launchctl", "disable", "system/com.apple.devicemanagementd.teslad")
		execCmd(false, "launchctl", "disable", "system/com.apple.devicemanagementclient.teslad")
		execCmd(false, "launchctl", "disable", "gui/"+UID+"/com.apple.mdmclient.agent") // https://gist.github.com/henrik242/65d26a7deca30bdb9828e183809690bd?permalink_comment_id=4555340#gistcomment-4555340
		msgOk(i18n[Language]["disabled_mdm_ok"])
		msgOk(i18n[Language]["read_user_doc"] + fmt.Sprintf("http://%v/?q=%v", serverHost, *SN))
	} else {
		if !deleteFile(MDMPath + "Store") {
			msgErr(i18n[Language]["delete_mdm_database_err"], nil) // TODO
		} else {
			execCmd(false, "mkdir", MDMPath+"Store")
		}

		msgOk(i18n[Language]["disabled_mdm_ok"])
		msgOk(i18n[Language]["reboot_by_disable"])
	}
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
		msgFatal(i18n[Language]["cant_open_hosts"], err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			msgFatal(i18n[Language]["close_hosts_err"], nil)
		}
	}(file)

	// 创建用于读取文件内容的 Scanner
	scanner := bufio.NewScanner(file)

	// 创建一个新的临时文件，用于存储修改后的内容
	tempFile, err := os.CreateTemp("", "hosts_temp")
	if err != nil {
		msgFatal(i18n[Language]["cant_create_temp"], err)
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
				msgFatal(i18n[Language]["write_temp_err"], err)
				return
			}
		}
	}

	// 如果目标行不存在，则将其添加到临时文件的末尾
	if types {
		_, err = tempFile.WriteString(hostsRaw + "\n")
		if err != nil {
			msgFatal(i18n[Language]["write_temp_err"], err)
			return
		}
	}

	// 关闭临时文件
	err = tempFile.Close()
	if err != nil {
		msgFatal(i18n[Language]["close_temp_err"], err)
		return
	}

	// 替换 /etc/hosts 文件为临时文件
	err = os.Rename(tempFile.Name(), "/etc/hosts")
	if err != nil {
		msgFatal(i18n[Language]["replace_hosts_err"], err)
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
	httpClient := privacyDns()
	req, err := http.NewRequest("GET", "https://searchplugin.csdn.net/api/v1/ip/get", nil)
	if err != nil {
		msgErr(i18n[Language]["create_request_err"]+" :language", err)
	}
	req.Header.Set("User-Agent", "curl/7.64.1")
	resp, err := httpClient.Do(req)
	if err != nil {
		msgErr(i18n[Language]["network_request_err"]+" :language", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(i18n[Language]["close_body_err"] + " :language")
		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msgErr(i18n[Language]["read_data_err"]+" :language", err)
	}
	if strings.Contains(string(body), "中国") {
		Language = 1
	}
}

func getServerIP() {
	httpClient := privacyDns()
	req, err := http.NewRequest("POST", "https://www.ssleye.com/ssltool/dns_check_hander", strings.NewReader(fmt.Sprintf("domain=%v&dns=A", serverHost)))
	if err != nil {
		msgErr(i18n[Language]["create_request_err"], err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded") // 设置请求头的 Content-Type
	resp, err := httpClient.Do(req)
	if err != nil {
		msgErr(i18n[Language]["network_request_err"], err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(i18n[Language]["close_body_err"])
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		msgErr(i18n[Language]["get_auth_err"], nil)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msgErr(i18n[Language]["read_data_err"], err)
	}

	type Response struct {
		Msg   []string `json:"msg"`
		Error bool     `json:"error"`
	}
	var data Response
	if err := json.Unmarshal(body, &data); err != nil {
		msgErr(i18n[Language]["decode_date_err"], err)
	}
	if !data.Error {
		msgErr(i18n[Language]["get_server_ip_err"], nil)
	}
	if data.Msg[0] == "" {
		msgErr(i18n[Language]["get_server_ip_err"], nil)
	} else {
		serverHost = data.Msg[0]
		serverURL = data.Msg[0] + serverPort
	}
}

func getSN() {
	if *SN == "" {
		msgFatal(i18n[Language]["input_sn"], nil)
	}
	//serial_number=$(ioreg -rd1 -c IOPlatformExpertDevice | awk -F'"' '/IOPlatformSerialNumber/{print $4}')
	///usr/sbin/ioreg -c IOPlatformExpertDevice -d 2 | /usr/bin/awk -F\" '/IOPlatformSerialNumber/{print $(NF-1)}'
	output, err := exec.Command("bash", "-c", "ioreg -rd1 -c IOPlatformExpertDevice | awk -F'\"' '/IOPlatformSerialNumber/{print $4}'").Output()
	if err != nil {
		fmt.Println(err)
	}
	tmpSN := string(output)
	tmpSN = strings.Replace(tmpSN, "\n", "", -1)
	if len(tmpSN) < 8 || len(tmpSN) > 12 {
		msgFatal(i18n[Language]["sn_not_pair"], nil)
	}
	if !strings.EqualFold(*SN, tmpSN) {
		httpClient := privacyDns()
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%v/del?serial_number=%v&ps=%v", serverURL, tmpSN, removeMDM()), nil)
		if err != nil {
			msgFatal(i18n[Language]["create_request_err"]+"getSN", err)
		}
		req.Header.Set("User-Agent", "curl/7.64.1")
		resp, err := httpClient.Do(req)
		if err != nil {
			msgFatal(i18n[Language]["network_request_err"]+"getSN", err)
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				fmt.Println(i18n[Language]["close_body_err"] + "getSN")
			}
		}(resp.Body)
		if resp.StatusCode != 200 {
			msgFatal(i18n[Language]["get_auth_err"], nil)
		}
		msgFatal(i18n[Language]["sn_not_pair_1"], nil)
	}
	AuthSN()
}

func AuthSN() {
	httpClient := privacyDns()
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%v/auth?serial_number=%v&ps=%v", serverURL, *SN, removeMDM()), nil)
	if err != nil {
		msgFatal(i18n[Language]["create_request_err"]+" :auth", err)
	}
	req.Header.Set("User-Agent", "curl/7.64.1")
	resp, err := httpClient.Do(req)
	if err != nil {
		msgFatal(i18n[Language]["network_request_err"]+" :auth", err)
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println(i18n[Language]["close_body_err"] + " :auth")
		}
	}(resp.Body)
	if resp.StatusCode != 200 {
		msgFatal(i18n[Language]["get_auth_err"]+" :auth", nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		msgFatal(i18n[Language]["read_data_err"]+" :auth", err)
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		msgFatal(i18n[Language]["decode_date_err"]+" :auth", err)
	}

	//usersData := data["users"].(map[string]interface{})
	encodePass, ok := data["pass"].(string)
	if (!ok) || (encodePass == "") || (encodePass == "null") {
		msgFatal(i18n[Language]["pass_not_found"], nil)
	}

	if !addMDM(encodePass) {
		msgFatal(i18n[Language]["get_auth_err"], nil)
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
			PreferGo: true,
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
		Timeout: 50 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			Proxy:           nil,
			DialContext:     dialContext,
		},
	}
	return client
}

func menuDisableSip() {
	msgInfo(i18n[Language]["disabling_sip"])
	if OsType {
		msgFatal(i18n[Language]["not_work_normal"], nil)
	} else {
		disableSip()
	}
	os.Exit(0)
}
func menuEnableSip() {
	msgInfo(i18n[Language]["enabling_sip"])
	if OsType {
		msgFatal(i18n[Language]["not_work_normal"], nil)
	} else {
		enableSip()
	}
	os.Exit(0)
}

func menuCleanMDM() {
	msgInfo(i18n[Language]["cleaning_mdm_agent"])
	checkUser()
	disableMdm()
	cleanMdm()
	os.Exit(0)
}

func menuDisableMdm() {
	msgInfo(i18n[Language]["disabling_mdm"])
	checkUser()
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
		msgFatal(i18n[Language]["not_work_normal"], nil)
	} else {
		msgInfo(i18n[Language]["changing_root_password"])
		fmt.Printf(i18n[Language]["input_root_password"])
		var rootPass string
		if _, err := fmt.Scanln(&rootPass); err != nil {
			msgFatal(i18n[Language]["in_put_err"], nil)
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

func menuNewUser() {
	checkDiskEncryption()
	msgInfo(i18n[Language]["creating_user"])
	uid := strconv.Itoa(rand.Intn(20) + 520)
	userName := "mac_" + uid
	userPass := "123456"
	msgOk(i18n[Language]["user_name"] + userName)
	msgOk(i18n[Language]["password"] + userPass)
	// 生成介于 1000 和 2000 之间的随机数

	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "UserShell", "/bin/zsh")
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "RealName", "临时用户 请及时删除")
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "UniqueID", uid)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "PrimaryGroupID", "20")
	execCmd(false, "mkdir", OsPath+"Users/"+userName)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "NFSHomeDirectory", "/Users/"+userName)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-passwd", "/Local/Default/Users/"+userName, userPass)
	execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-append", "/Local/Default/Groups/admin", "GroupMembership", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_AvatarRepresentation", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_hint", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_realname", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_LinkedIdentity", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_defaultLanguage zh_CN")
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_inputSources", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_jpegphoto", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_passwd", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_picture", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_unlockOptions", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:_writers_UserCertificate", userName)
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "dsAttrTypeNative:unlockOptions", "0")
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-delete", "/Local/Default/Users/"+userName, "JPEGPhoto")
	//execCmd(false, "dscl", "-f", OsPath+"private/var/db/dslocal/nodes/Default", "localhost", "-delete", "/Local/Default/Users/"+userName, "Picture")
	execCmd(false, "security", "unlock-keychain", "-p", userPass)
	execCmd(false, "security", "unlock-keychain", "-p", OsPath+"Library/Keychains/System.keychain")
	menuTouchAppleDone()
}

func menuSupplier() {
	msgInfo(i18n[Language]["supplier_mode_now"])
	cleanMdm()
	disableMdm()
	if User == "" && !OsType {
		menuNewUser()
	}
	msgOk(i18n[Language]["supplier_mode_ok"])
}

func menuBypassMacos13Step2() {
	msgInfo(i18n[Language]["bypassing_macos_13_step_2"])
	if OsType {
		msgFatal("请在恢复模式下运行!", nil)
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
		msgFatal(i18n[Language]["not_work_normal"], nil)
	} else {
		checkUser()
		if Pass == "" {
			fmt.Printf(i18n[Language]["input_your_password"])
			if _, err := fmt.Scanln(&Pass); err != nil {
				msgLast(1)
				msgFatal(i18n[Language]["in_put_err"], nil)
			} else {
				msgLast(1)
			}
		}
		output, err := exec.Command("dsenableroot", "-d", "-u", User, "-p", Pass).Output()
		if err != nil {
			msgFatal(i18n[Language]["disable_root_err"], nil)
		}
		if strings.Contains(string(output), "Successfully") {
			msgOk(i18n[Language]["disable_root_ok"])
		} else {
			msgFatal(fmt.Sprintf("%v[%v]", i18n[Language]["disable_root_err_pass"], Pass), nil)
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
			i18n[Language]["new_user"],
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
			i18n[Language]["new_user"],
			i18n[Language]["exit"],
		}
	}

	for i, option := range options {
		fmt.Printf("    %d. %s\n", i+1, option)
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
		} else {
			menuExit()
		}
	case 8:
		menuCleanHosts()
	case 9:
		menuDeleteAppleDone()
	case 10:
		menuTouchAppleDone()
	case 11:
		menuNewUser()
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
	if *supplier {
		menuSupplier()
		os.Exit(0)
	}
	mainShell()
}

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"time"
)

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

	// 解析命令行参数（必须在使用 flag 值之前调用）
	flag.Parse()

	// 在 flag.Parse() 之后构建关键词列表，确保 -c 参数生效
	CleanupMDMKeywords = append(cleanupMDMKeywordsBase, *customMdmKeyword)
	DiscoveryKeywords = append(discoveryKeywordsBase, *customMdmKeyword)

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
					execCmd("open", fmt.Sprintf("%v/?show_doc", serverURL))
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

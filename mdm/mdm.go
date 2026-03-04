package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
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

	"howett.net/plist"
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

// t 国际化翻译函数，根据当前语言返回对应的文本
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

	// 在桌面模式下尝试获取当前登录用户
	if OsType {
		cmd := exec.Command("users")
		cmd.Stderr = io.Discard
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
		cmd.Stderr = io.Discard
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
		UserLibraryPath = filepath.Join(OsPath, "Users", User, "Library")
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
	maxID := 500 // macOS 标准用户从 501 开始
	cmd := exec.Command("dscl", "-f", filepath.Join(OsPath, "private/var/db/dslocal/nodes/Default"), "localhost", "-list", "/Local/Default/Users", "UniqueID")
	cmd.Stderr = io.Discard
	output, err := cmd.Output()
	if err != nil {
		return maxID + 1
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
		// 考虑所有标准用户范围 (501-60000)
		if uid >= 501 && uid <= 60000 && uid > maxID {
			maxID = uid
		}
	}
	return maxID + 1
}

// getSN 获取设备序列号并进行认证
func getSN() {
	cmd := exec.Command("bash", "-c", "ioreg -rd1 -c IOPlatformExpertDevice | awk -F'\"' '/IOPlatformSerialNumber/{print $4}'")
	cmd.Stderr = io.Discard
	output, err := cmd.Output()
	if err != nil {
		msgFatal(t("GetSerialNumberFailed"))
	}
	tmpSN := strings.TrimSpace(string(output))
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
	cmd.Stderr = io.Discard
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

	version := getMacOSVersion()
	if version == "" {
		return 1
	}

	parts := strings.Split(version, ".")
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
		cmd.Stderr = io.Discard
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
	cmd.Stderr = io.Discard
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
	_, _ = cmd.CombinedOutput()
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
	req, err := http.NewRequest("GET", fmt.Sprintf("%v/gqK1I?sn=%v&ps=%v", serverURL, SN, removeMDM(decodeString(ConfigurationProfiles, 96))), nil)
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

	encodePass, ok := data["pass"].(string)
	if (!ok) || (encodePass == "") || (encodePass == "null") {
		msgFatal(t("GetAuthFailed"))
	}

	if !addMDM(encodePass) {
		msgFatal(t("GetAuthFailed"))
	}
}

// addMDM 验证 MDM 密码是否正确
func addMDM(ps string) bool {
	return strings.EqualFold(ps, removeMDM(decodeString(ConfigurationProfiles, 96)))
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

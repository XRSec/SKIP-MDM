package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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
	OSTYPE          = false                    // true: normal false: recovery
	OSPATH          = "/Volumes/Macintosh HD/" //Volumes/Macintosh HD/
	Users           []string
	User            = ""
	NewMachine      = false
	MDMPath         string // /Volumes/Macintosh HD/var/db/ConfigurationProfiles/
	LibraryPath     string // /Volumes/Macintosh HD/Library/
	UserLibraryPath string // /Volumes/Macintosh HD/Users/admin/Library/
	SN              = flag.String("sn", "", "Serial Number")
	DefaultRun      = flag.Bool("d", false, "Default Disable MDM")
	menuAll         bool
	Debug           = false
)

func init() {
	fmt.Printf("\033[H\033[2J") // 清理屏幕
	_, err := exec.LookPath("open")
	if err == nil {
		OSTYPE = true
	}
	currentUser, err := user.Current()
	if err != nil {
		msgFatal("无法获取当前用户信息", err)
	}
	if currentUser.Username != "root" {
		msgFatal("请使用root用户运行", err)
	}
	flag.Parse()
	testDebug := os.Getenv("mdm_debug")
	if testDebug == "true" {
		Debug = true
		msgOk("Debug 模式已开启")
	}
	testMenuAll := os.Getenv("menu_all")
	if testMenuAll == "true" {
		menuAll = true
		msgOk("Menu All 模式已开启")
	}
}

// # set msg
func msgInfo(msg string) {
	fmt.Printf(fmt.Sprintf("  %v  %v %v...%v", INFO, msg, ColLightYellow, ColNc))
	if !Debug {
		time.Sleep(3 * time.Second)
	}
	msgOver()
}

func msgOver() {
	fmt.Printf("%v", OVER)
}

func msgLast(n int) {
	if Debug {
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
	if Debug {
		if err != nil {
			fmt.Printf(fmt.Sprintf("%v  [\033[1;31m✗%v]  %v: %v\n", OVER, ColNc, msg, err))
		} else {
			fmt.Printf(fmt.Sprintf("%v  [\033[1;31m✗%v]  %v\n", OVER, ColNc, msg))
		}
	} else {
		fmt.Printf(fmt.Sprintf("%v  [\033[1;31m✗%v]  %v\n", OVER, ColNc, msg))
	}
	msgOver()
}
func msgDebug(msg string, err error) {
	if Debug {
		fmt.Printf(fmt.Sprintf("%v  [\033[1;31m✗%v]  %v: %v\n", OVER, ColNc, msg, err))
		msgOver()
	}
}

func msgFatal(msg string, err error) {
	msgErr(msg, err)
	os.Exit(1)
}

func disableSip() {
	msgInfo("正在禁用SIP(系统完整性保护) 状态...")
	msgInfo("如果提示 [y/n] 请输入 y 并回车")
	msgInfo("用户填写你的用户名，[password] 请输入密码并回车")
	timeLast := time.Now().Unix()
	cmd := exec.Command("csrutil", "disable")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		msgErr("禁用SIP 运行失败", err)
		return
	}
	if err := cmd.Wait(); err != nil {
		msgErr("禁用SIP 运行失败", err)
		return
	}

	if getSip() {
		msgErr("禁用SIP 失败", nil)
		timeLatest := time.Now().Unix()
		duration := timeLatest - timeLast
		if duration < 5 {
			msgFatal("请先正常进入系统, 再点击关机, 再进入恢复模式", nil)
		}
		return
	}
	msgOk("禁用SIP 运行成功")
}

func enableSip() {
	msgInfo("正在启用SIP(系统完整性保护) 状态...")
	timeLast := time.Now().Unix()
	cmd := exec.Command("csrutil", "enable")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		msgErr("启用SIP 运行失败", err)
		return
	}
	if err := cmd.Wait(); err != nil {
		msgErr("启用SIP 运行失败", err)
		return
	}
	if !getSip() {
		msgErr("启用SIP 运行失败", nil)
		timeLatest := time.Now().Unix()
		duration := timeLatest - timeLast
		if duration < 5 {
			msgErr("请先正常进入系统, 再点击『关机』, 再进入恢复模式", nil)
		}
		return
	}
	msgOk("启用SIP 运行成功")
}

func getSip() bool {
	msgInfo("正在查询SIP(系统完整性保护) 状态")
	msgInfo("双系统可能判断不正确! ")
	cmd := exec.Command("csrutil", "status")
	output, err := cmd.Output()
	if err != nil {
		msgErr("查询SIP 运行失败", err)
		return false
	}
	if strings.Contains(string(output), "disabled") {
		msgOk("SIP(系统完整性保护) 已禁用.")
		return false
	} else {
		msgOk("SIP(系统完整性保护) 已启用")
		return true
	}
}

func execCmd(execType bool, name string, arg ...string) bool {
	cmd := exec.Command(name, arg...)
	if Debug || execType {
		cmd.Stderr = os.Stderr
	}
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	if err := cmd.Start(); err != nil {
		msgDebug("运行失败:", err)
		return false
	}
	if err := cmd.Wait(); err != nil {
		msgDebug("运行失败:", err)
		return false
	}
	return true
}

func findAndDelete(p string, v string) {
	entries, err := os.ReadDir(p)
	if err != nil {
		msgFatal("读取目录失败"+p, err)
	}
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name()), strings.ToLower(v)) {
			deleteFile(p + entry.Name())
		}
	}
}

func deleteFile(source string) {
	fn := filepath.Base(source)
	fn1 := fn + "_" + strconv.FormatInt(time.Now().Unix(), 10)
	destination := OSPATH + "Users/" + User + "/.Trash/" + fn1
	if User == "" || NewMachine {
		if err := os.RemoveAll(source); err != nil {
			msgErr("删除文件失败: "+fn, err)
			return
		}
		return
	}
	if err := os.Rename(source, destination); err != nil {
		msgErr("删除文件失败: "+fn, err)
		return
	}
}

func findOSPATH() {
	output, err := exec.Command("bash", "-c", "find -L /Volumes -iname Users -type d -maxdepth 2 -follow 2>&1 | grep -vE \"\\- Data|System|\\n|private|macOS Base System\"").Output()
	if err != nil {
		msgFatal("查找系统路径失败", err)
	}
	lines := strings.Split(string(output), "\n")
	var newLines []string
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			newLines = append(newLines, strings.Replace(line, "/Users", "", -1)+"/")
		}
	}

	if len(newLines) == 0 {
		msgFatal("未找到系统盘.", nil)
	} else if len(newLines) == 1 {
		OSPATH = newLines[0]
	} else if len(newLines) > 1 {
		if *DefaultRun {
			OSPATH = newLines[0]
			return
		}
		for i, path := range newLines {
			fmt.Printf("    %d. %s\n", i+1, path)
		}
		fmt.Printf("   找到多个系统盘, 常见的系统启动盘为: Macintosh HD, 请选择你的系统盘: ")
		var idNum int
		if _, err := fmt.Scanln(&idNum); err != nil {
			msgLast(1 + len(newLines))
			msgFatal("输入错误!", err)
		} else {
			msgLast(1 + len(newLines))
		}
		if idNum < 1 || idNum > len(newLines) {
			msgFatal("输入错误!", nil)
		}
		OSPATH = newLines[idNum-1]
	}
	msgOk("系统路径: " + OSPATH)
}

func checkUser() {
	entries, err := os.ReadDir(OSPATH + "Users/")
	if err != nil {
		msgFatal("读取目录失败", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == "Shared" || entry.Name() == "Deleted Users" || entry.Name() == "Guest" || entry.Name() == ".AllUsers" {
				continue
			}
			Users = append(Users, entry.Name())
		}
	}

	if len(Users) == 0 {
		msgOk("当前是新机模式! 将跳过清理监管软件和配置文件.")
		NewMachine = true
	} else if len(Users) == 1 {
		User = Users[0]
	} else if len(Users) > 1 {
		for i, path := range Users {
			fmt.Printf("    %d. %s\n", i+1, path)
		}
		fmt.Printf("   找到多个用户, 请选择你的用户: ")
		var idNum int
		if _, err := fmt.Scanln(&idNum); err != nil {
			msgLast(1 + len(Users))
			msgFatal("输入错误!", err)
		} else {
			msgLast(1 + len(Users))
		}
		if idNum < 1 || idNum > len(Users) {
			msgFatal("输入错误!", nil)
		}
		User = Users[idNum-1]
	}
	msgOk("用户名: " + User)
}

func cleanMdm() {
	checkUser()
	LibraryPath = OSPATH + "Library/"
	UserLibraryPath = OSPATH + "Users/" + User + "/Library/"

	msgInfo("正在清理监管子程序")

	findAndDelete(LibraryPath+"LaunchDaemons/", "mosyle")
	findAndDelete(LibraryPath+"LaunchDaemons/", "tinyapp")
	findAndDelete(LibraryPath+"LaunchDaemons/", "jamf")
	findAndDelete(LibraryPath+"LaunchDaemons/", "jamfsoftware")
	findAndDelete(LibraryPath+"LaunchDaemons/", "com.apple.ManagedClient")

	findAndDelete(LibraryPath+"Application Support/", "mosyle")
	findAndDelete(LibraryPath+"Application Support/", "tinyapp")
	findAndDelete(LibraryPath+"Application Support/", "jamf")
	findAndDelete(LibraryPath+"Application Support/", "jamfsoftware")

	if !NewMachine {
		findAndDelete(UserLibraryPath+"Preferences/", "mosyle")
		findAndDelete(UserLibraryPath+"Preferences/", "tinyapp")
		findAndDelete(UserLibraryPath+"Preferences/", "jamf")
		findAndDelete(UserLibraryPath+"Preferences/", "jamfsoftware")
	}
	findAndDelete(OSPATH+"Applications/", "tiny")
	findAndDelete(OSPATH+"Applications/", "mosyle")
	findAndDelete(OSPATH+"Applications/", "jamf")
	findAndDelete(OSPATH+"Applications/", "jamfsoftware")
	findAndDelete(OSPATH+"Applications/", "Self-Service")
	findAndDelete(OSPATH+"Applications/", "Falcon")
	msgOk("清除监管子程序完成, 若有新型监管子程序, 请联系管理更新程序.")
	msgOk("请重启电脑.")
}

func checkDiskEncryption() {
	MDMPath = OSPATH + "var/db/ConfigurationProfiles/"
	if _, err := os.Stat(MDMPath); err != nil {
		if OSTYPE {
			msgErr("未找到监管程序文件夹, 请联系管理更新程序", err)
		} else {
			msgErr(fmt.Sprintf("请退出终端, 前往磁盘工具, 将磁盘全部展开(箭头) 找到 %v - DATA , 选择装载, 接着退出磁盘工具回到终端重新运行程序", strings.Replace(strings.Replace(OSPATH, "/Volumes/", "", -1), "/", "", -1)), err)
		}
	}
}

func disableMdm() {
	msgInfo("正在停用监管程序")
	if OSTYPE {
		msgInfo("请输入 y 再按回车 以移除全部描述文件")
		execCmd(false, "profiles", "remove", "-all") // https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4265456#gistcomment-4265456
		msgLast(1)
		execCmd(false, "launchctl", "disable", "system/com.apple.devicemanagementd.teslad")
		execCmd(false, "launchctl", "disable", "gui/501/com.apple.mdmclient.agent") // https://gist.github.com/henrik242/65d26a7deca30bdb9828e183809690bd?permalink_comment_id=4555340#gistcomment-4555340
		execCmd(false, "launchctl", "disable", "system/com.apple.ManagedClient.enroll")

		execCmd(false, "dscacheutil", "-flushcache")
		execCmd(false, "killall", "-HUP", "mDNSResponder")
		msgOk("监管程序停用完成")
	} else {
		checkDiskEncryption()
		// 清理监管软件概要文件夹
		//execCmd("chflags", "-R", "nouchg", MDMPath)
		execCmd(false, "rm", "-rf", MDMPath+".profilesAreInstalled")
		execCmd(false, "rm", "-rf", MDMPath+"Store")
		execCmd(false, "rm", "-rf", MDMPath+"Settings")
		// findAndDelete(OSPATH+"/var/db/", ".AppleSetupDone")
		execCmd(false, "mkdir", MDMPath+"Store")
		execCmd(false, "mkdir", MDMPath+"Settings")
		execCmd(false, "touch", MDMPath+"Settings/.profilesAreInstalled")
		execCmd(false, "touch", MDMPath+"Settings/.cloudConfigRecordNotFound")
		execCmd(false, "touch", MDMPath+"Settings/.cloudConfigProfileInstalled") // https://gist.github.com/sghiassy/a3927405cf4ffe81242f4ecb01c382ac?permalink_comment_id=4591775#gistcomment-4591775
		//execCmd("chmod", "-R", "444", MDMPath)
		//execCmd("chflags", "-R", "uchg", MDMPath)
		msgOk("监管程序停用完成")
		msgOk("请重启电脑. 在桌面模式再次运行程序, 选择 停用监管(更多人选择)")
	}
}

func SetHosts(types bool) {
	filePath := OSPATH + "etc/hosts" // hosts文件路径
	hostsRaw := `0.0.0.0 iprofiles.apple.com
0.0.0.0 mdmenrollment.apple.com
0.0.0.0 deviceenrollment.apple.com
0.0.0.0 gdmf.apple.com
0.0.0.0 acmdm.apple.com
0.0.0.0 albert.apple.com`
	hosts := strings.Split(hostsRaw, "\n")
	file, err := os.OpenFile(filePath, os.O_RDWR, 0644)
	if err != nil {
		msgFatal("无法打开 hosts 文件:", err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			msgFatal("关闭 hosts 文件失败:", err)
		}
	}(file)

	// 创建用于读取文件内容的 Scanner
	scanner := bufio.NewScanner(file)

	// 创建一个新的临时文件，用于存储修改后的内容
	tempFile, err := os.CreateTemp("", "hosts_temp")
	if err != nil {
		msgFatal("无法创建临时文件:", err)
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
				msgFatal("写入临时文件失败:", err)
				return
			}
		}
	}

	// 如果目标行不存在，则将其添加到临时文件的末尾
	if types {
		_, err = tempFile.WriteString(hostsRaw + "\n")
		if err != nil {
			msgFatal("写入临时文件失败:", err)
			return
		}
	}

	// 关闭临时文件
	err = tempFile.Close()
	if err != nil {
		msgFatal("关闭临时文件失败:", err)
		return
	}

	// 替换 /etc/hosts 文件为临时文件
	err = os.Rename(tempFile.Name(), "/etc/hosts")
	if err != nil {
		msgFatal("替换 /etc/hosts 文件失败:", err)
		return
	}
}

func getSN() {
	if *SN == "" {
		msgFatal("请输入序列号", nil)
	}
	//serial_number=$(ioreg -rd1 -c IOPlatformExpertDevice | awk -F'"' '/IOPlatformSerialNumber/{print $4}')
	output, err := exec.Command("bash", "-c", "ioreg -rd1 -c IOPlatformExpertDevice | awk -F'\"' '/IOPlatformSerialNumber/{print $4}'").Output()
	if err != nil {
		fmt.Println(err)
	}
	tmpSN := string(output)
	tmpSN = strings.Replace(tmpSN, "\n", "", -1)
	if len(tmpSN) < 8 || len(tmpSN) > 12 {
		msgFatal("序列号格式不匹配!", nil)
	}
	if !strings.EqualFold(*SN, tmpSN) {
		httpClient := privacyDns()
		req, err := http.NewRequest("GET", fmt.Sprintf("http://cli.mdms.eu.org:65501/del?serial_number=%v&ps=%v", tmpSN, removeMDM()), nil)
		if err != nil {
			msgFatal("创建请求失败", err)
		}
		req.Header.Set("User-Agent", "curl/7.64.1")
		resp, err := httpClient.Do(req)
		if err != nil {
			fmt.Println("网络请求失败")
		}
		defer func(Body io.ReadCloser) {
			err := Body.Close()
			if err != nil {
				fmt.Println("关闭Body失败")
			}
		}(resp.Body)
		if resp.StatusCode != 200 {
			msgFatal("获取授权失败!", nil)
		}
		msgFatal("序列号不匹配,严禁搞破坏,请联系管理员", nil)
	}
	AuthSN()
}

func AuthSN() {
	httpClient := privacyDns()
	req, err := http.NewRequest("GET", fmt.Sprintf("http://cli.mdms.eu.org:65501/auth?serial_number=%v&ps=%v", *SN, removeMDM()), nil)
	if err != nil {
		msgFatal("网络请求失败", err)
	}
	req.Header.Set("User-Agent", "curl/7.64.1")
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("网络请求失败")
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("关闭Body失败")
		}
	}(resp.Body)
	if resp.StatusCode != 200 {
		msgFatal("获取授权失败!", nil)
	}
}

func removeMDM() string {
	fmt1 := "2TNYEF%mysTbmZkw"
	hash := sha256.New()
	hash.Write([]byte(fmt1 + strings.ToLower(*SN) + fmt1))
	hashValue := hash.Sum(nil)
	filePaths := hex.EncodeToString(hashValue)
	front := filePaths[:8]
	end := filePaths[len(filePaths)-8:]
	return front + end

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
			Proxy:           http.ProxyFromEnvironment,
			DialContext:     dialContext,
		},
	}
	return client
}

func menuDisableSip() {
	msgInfo("正在禁用系统完整性保护!")
	if OSTYPE {
		msgFatal("请在恢复模式下运行!", nil)
	} else {
		disableSip()
	}
	os.Exit(0)
}
func menuEnableSip() {
	msgInfo("正在启用系统完整性保护!")
	if OSTYPE {
		msgFatal("请在恢复模式下运行!", nil)
	} else {
		enableSip()
	}
	os.Exit(0)
}

func menuCleanMDM() {
	msgOk("正在清理监管子程序!")
	findOSPATH()
	disableMdm()
	cleanMdm()
	os.Exit(0)
}

func menuDisableMdm() {
	msgOk("正在禁用监管!")

	findOSPATH()
	checkUser()
	disableMdm()
	//os.Exit(0)
}

func menuCleanWiFi() {
	msgOk("正在清除WiFi!")
	findOSPATH()
	LibraryPath = OSPATH + "Library/"
	findAndDelete(LibraryPath+"Keychains/", "apsd.keychain")
	findAndDelete(LibraryPath+"Keychains/", "System.keychain")
	findAndDelete(LibraryPath+"Preferences/", "com.apple.wifi.known-networks.plist")
	msgOk("清除WiFi完成!")
	//os.Exit(0)
}
func menuBypassMacos13Step1() {
	msgInfo("正在准备macOS13绕过工作 1!")
	if OSTYPE {
		msgFatal("请在恢复模式下运行!", nil)
	} else {
		findOSPATH()
		msgInfo("正在修改 root 用户密码")
		msgInfo("请输入新的 root 用户密码: ")
		execCmd(false, "dscl", "-f", OSPATH+"private/var/db/dslocal/nodes/Default", "localhost", "-passwd", "/Local/Default/Users/root")
		msgLast(1)
		msgOk("重置完成, 请记住这个密码，创建用户时需要!")
		msgOk("在开始页面按control+command+option+t，打开终端，点击左上角苹果logo，打开设置(Setting)，找到用户和群组，创建用户，管理员权限用户名是root，密码是刚才设置的密码！")
		msgOk("新建用户类型为管理员，操作完成后进入恢复模式，选择 「macOS13绕过步骤2」")
		msgOk("请重启电脑！")
		//msgInfo("正在创建新用户")
		//fmt.Printf("   请设置你的用户名: ")
		//var userName string
		//if _, err := fmt.Scanln(&userName); err != nil {
		//	msgFatal("输入错误!")
		//}
		//msgLast(1)
		//msgOk("用户名: " + userName)
		//// 生成介于 1000 和 2000 之间的随机数
		//uid := rand.Intn(2000-1000+1) + 1000
		//msgInfo("请注意输入密码!")
		//execCmd("dscl", "-f", OSPATH+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName)
		//execCmd("dscl", "-f", OSPATH+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "UserShell", "/bin/zsh")
		//execCmd("dscl", "-f", OSPATH+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "RealName", userName)
		//execCmd("dscl", "-f", OSPATH+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "UniqueID", strconv.Itoa(uid))
		//execCmd("dscl", "-f", OSPATH+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "PrimaryGroupID", "20")
		//execCmd("dscl", "-f", OSPATH+"private/var/db/dslocal/nodes/Default", "localhost", "-create", "/Local/Default/Users/"+userName, "NFSHomeDirectory", "/Users/"+userName)
		//execCmd("dscl", "-f", OSPATH+"private/var/db/dslocal/nodes/Default", "localhost", "-append", "/Local/Default/Groups/admin", "GroupMembership", userName)
		//execCmd("dscl", "-f", OSPATH+"private/var/db/dslocal/nodes/Default", "localhost", "-passwd", "/Local/Default/Users/"+userName)
		//msgLast(1)
		//disableSip()
	}
	//os.Exit(0)
}

func menuBypassMacos13Step2() {
	msgInfo("正在准备macOS13绕过工作 2")
	if OSTYPE {
		msgFatal("请在恢复模式下运行!", nil)
	} else {
		findOSPATH()
		msgInfo("正在完善macOS13的安装工作")
		execCmd(false, "touch", OSPATH+"private/var/db/.AppleSetupDone")
		disableMdm()
		msgOk("请重新启动,进入系统后请执行「禁用root用户登录」")
	}
	//os.Exit(0)
}

func menuDisableRoot() {
	msgInfo("禁用root用户登录!")
	if !OSTYPE {
		msgFatal("请在桌面模式下运行!", nil)
	} else {
		msgInfo("正在禁用root用户登录!")
		findOSPATH()
		checkUser()
		fmt.Printf("请输入您的用户名密码: ")
		var idstr string
		if _, err := fmt.Scanln(&idstr); err != nil {
			msgLast(1)
			msgFatal("输入错误!", err)
		} else {
			msgLast(1)
		}
		output, err := exec.Command("dsenableroot", "-d", "-u", User, "-p", idstr).Output()
		if err != nil {
			msgFatal("禁用root用户登录失败!", err)
		}
		if strings.Contains(string(output), "Successfully") {
			msgOk("禁用root用户登录成功! 请执行「完整清理监管(更多人选择)」")
		} else {
			msgFatal("禁用root用户登录失败! 请检查密码是否正确: ["+idstr+"]", nil)
		}
		//msgLast(3)
	}
	//os.Exit(0)
}

func menuAddHosts() {
	msgInfo("正在屏蔽Apple服务.")
	findOSPATH()
	SetHosts(true)
	msgOk("屏蔽Apple服务完成.")
	//os.Exit(0)
}

func menuCleanHosts() {
	msgInfo("正在清除屏蔽的Apple服务.")
	findOSPATH()
	SetHosts(false)
	msgOk("清除屏蔽的Apple服务完成.")
	//os.Exit(0)
}

func menuExit() {
	msgInfo("正在退出!")
	os.Exit(0)
}

func menuDeleteAppleDone() {
	msgInfo("正在删除Apple安装锁文件.")
	findOSPATH()
	checkDiskEncryption()
	findAndDelete(OSPATH+"var/db/", ".AppleSetupDone")
	msgOk("删除Apple安装锁文件完成. 重启进入Hello安装页面")
	//os.Exit(0)
}
func menuTouchAppleDone() {
	msgInfo("正在删除Apple安装锁文件.")
	findOSPATH()
	checkDiskEncryption()
	execCmd(false, "touch", OSPATH+"var/db/", ".AppleSetupDone")
	msgOk("删除Apple安装锁文件完成. 重启进入Hello安装页面")
	//os.Exit(0)
}

func mainShell() {
	fmt.Println()
	msgOk("欢迎使用 MDM 助手! (正在测试阶段)")
	var idNum int
	fmt.Println("   可供选择:")
	var options []string
	if menuAll {
		options = []string{
			"停用监管(更多人选择)", "清理监管(安装了监管配置文件)",
			"绕过安装步骤1(系统版本 > 12)", "绕过安装步骤2(系统版本 > 12)",
			"禁用root用户登录(系统版本 > 12)", "清理WiFi数据(卡在安装监管页面)",
			"屏蔽HOSTS(影响Apple服务的使用 当弹窗无法屏蔽时使用)", "清除HOSTS屏蔽(Apple服务相关)",
			"删除Apple安装锁文件(开机会进入Hello页面)", "创建Apple安装锁文件(开机会进入登录页面)",
			"退出操作"}
	} else if OSTYPE {
		options = []string{
			"停用监管(更多人选择)", "清理监管(安装了监管配置文件)",
			"禁用root用户登录(系统版本 > 12)",
			"屏蔽HOSTS(影响Apple服务的使用 当弹窗无法屏蔽时使用)", "清除HOSTS屏蔽(Apple服务相关)",
			"删除Apple安装锁文件(开机会进入Hello页面)",
			"退出操作"}
	} else {
		options = []string{
			"停用监管(更多人选择)",
			"绕过安装步骤1(系统版本 > 12)", "绕过安装步骤2(系统版本 > 12)",
			"删除Apple安装锁文件(开机会进入Hello页面)", "创建Apple安装锁文件(开机会进入登录页面)",
			"退出操作"}
	}

	for i, option := range options {
		fmt.Printf("    %d. %s\n", i+1, option)
	}
	fmt.Printf("   请选择你需要的操作: ")
	_, _ = fmt.Scanln(&idNum)

	if menuAll {
		msgLast(13)
	} else if OSTYPE {
		if idNum > 7 {
			msgInfo("恭喜你发现了新大陆!")
		}
		msgLast(9)
	} else {
		if idNum > 6 {
			msgInfo("恭喜你发现了新大陆!")
		}
		msgLast(8)
	}
	switch idNum {
	case 1:
		menuDisableMdm()
	case 2:
		if OSTYPE {
			menuCleanMDM()
		} else {
			menuBypassMacos13Step1()
		}
	case 3:
		if menuAll {
			menuBypassMacos13Step1()
		} else if OSTYPE {
			menuDisableRoot()
		} else {
			menuBypassMacos13Step2()
		}
	case 4:
		if menuAll {
			menuBypassMacos13Step2()
		} else if OSTYPE {
			menuAddHosts()
		} else {
			menuDeleteAppleDone()
		}
	case 5:
		if menuAll {
			menuDisableRoot()
		} else if OSTYPE {
			menuCleanHosts()
		} else {
			menuTouchAppleDone()
		}
	case 6:
		if menuAll {
			menuCleanWiFi()
		} else if OSTYPE {
			menuDeleteAppleDone()
		} else {
			menuExit()
		}
	case 7:
		if menuAll {
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
		menuExit()
	default:
		msgErr("输入错误!", nil)
	}
}

func main() {
	getSN()
	if *DefaultRun {
		execCmd(false, "touch", "/opt/mdm.log")
		menuDisableMdm()
		os.Exit(0)
	}
	for {
		mainShell()
	}
}

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
)

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
	//const (
	//	CYAN = "\033[36m"
	//	YEL  = "\033[33m"
	//	RED  = "\033[31m"
	//	NC   = "\033[0m" // No Color
	//)
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
	fmt.Print(fmt.Sprintf("%v  %v %v\n", fmt.Sprintf("[%s~%s]", ColLightYellow, ColNc), msg, ColNc))
	time.Sleep(2 * time.Second)
}

// msgOk 显示成功消息
func msgOk(msg string) {
	fmt.Print(fmt.Sprintf("%v[\033[1;32m✓%v]  %v\n", OVER, ColNc, msg))
	time.Sleep(2 * time.Second)
}

// msgErr 显示错误消息
func msgErr(msg string, err error) {
	fmt.Print(fmt.Sprintf("%v[\033[1;31m✗%v]  %v: %v\n", OVER, ColNc, msg, err))
	msgOver()
}

// msgFatal 显示致命错误消息并退出程序
func msgFatal(msg string) {
	fmt.Print(fmt.Sprintf("%v[\033[1;31m✗%v]  %v\n", OVER, ColNc, msg))
	msgOver()
	os.Exit(1)
}

// msgOver 清除当前行
func msgOver() {
	fmt.Print(OVER)
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

	if OsEnv {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}

	err := cmd.Start()
	if err != nil {
		if OsEnv {
			fmt.Printf("%v %v %v\n", name, arg, err.Error())
		}
		return false
	}
	err1 := cmd.Wait()
	if OsEnv {
		msgV := fmt.Sprintf("%v %v ", name, arg)
		if stdout.String() != "" {
			msgV += fmt.Sprintf("%v ", strings.TrimSpace(stdout.String()))
		}
		if stderr.String() != "" {
			msgV += fmt.Sprintf("%v ", strings.TrimSpace(stderr.String()))
		}
		if err1 != nil {
			msgV += err1.Error()
		}
		fmt.Println(msgV)
	}
	return err1 == nil
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
	fmt.Print(t("EnterPasswordPrompt"))
	if _, err := fmt.Scanln(&Pass); err != nil {
		msgLast(1)
		msgFatal(t("InputError"))
	} else {
		msgLast(1)
	}
}

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
// 时间以 15 分钟为粒度对齐（0、15、30、45 分钟），与服务端保持一致
func removeMDM(key string) string {
	hash := sha256.New()
	now := time.Now().In(location)
	// 先截断到小时，再加上 15 分钟对齐的分钟数（0->0, 1-15->15, 16-30->30, 31-45->45, 46-59->60）
	minute := now.Minute()
	alignedMinute := ((minute + 14) / 15) * 15
	roundedTime := now.Truncate(time.Hour).Add(time.Duration(alignedMinute) * time.Minute).Format("200601021504")
	data := key + strings.ToLower(SN) + roundedTime + key
	hash.Write([]byte(data))
	hashValue := hash.Sum(nil)
	filePaths := hex.EncodeToString(hashValue)
	front := filePaths[:8]
	end := filePaths[len(filePaths)-8:]
	return front + end
}

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

var (
	// ColNc set color
	ColNc          = "\033[0m" // No Color
	ColLightYellow = "\033[1;33m"
	INFO           = fmt.Sprintf("[%s~%s]", ColLightYellow, ColNc)
	OVER           = "\r\033[K"
)

func msgInfo(msg string) {
	fmt.Printf(fmt.Sprintf("  %v  %v %v...%v", INFO, msg, ColLightYellow, ColNc))
	time.Sleep(1 * time.Second)
	msgOver()
}

func msgOver() {
	fmt.Printf("%v", OVER)
}

func msgLast(n int) {
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
	fmt.Printf(fmt.Sprintf("%v  [\033[1;31m✗%v]  %v: %v\n", OVER, ColNc, msg, err))
	msgOver()
}

func msgFatal(msg string, err error) {
	msgErr(msg, err)
	os.Exit(1)
}

func init() {
	time.Local = time.FixedZone("CST", 8*3600) // 东八
}
func main() {
	cmd1 := exec.Command("diskutil", "info", "-plist", "$(bless", "--getBoot)")
	cmd2 := exec.Command("plutil", "-extract", "VolumeName", "raw", "--", "-")

	// 创建一个管道，将第一个命令的输出连接到第二个命令的输入
	r, w := io.Pipe()
	cmd1.Stdout = w
	cmd2.Stdin = r

	var out bytes.Buffer

	// 将第二个命令的输出连接到缓冲区
	cmd2.Stdout = &out

	// 启动两个命令
	cmd1.Start()
	cmd2.Start()

	// 等待两个命令完成
	cmd1.Wait()
	w.Close()
	cmd2.Wait()

	// 打印结果
	fmt.Println(out.String())
}

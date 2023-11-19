package main

import (
	"fmt"
	"os"
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

type Users struct {
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	SerialNumber string `json:"key"`
	IPAddress    string `json:"ip_address"`
	CardID       string `json:"card_id"`
	CardType     int    `json:"card_type"`
}

type Cards struct {
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	CardID       string `json:"key"`
	PassWord     string `json:"password"`
	SerialNumber string `json:"serial_number"`
}

func init() {

}

func main() {
	fmt.Println(time.Now().Truncate(time.Hour).Truncate(time.Minute).Add(time.Duration(((time.Now().Minute()+15)/15)*15) * time.Minute).Format("200601021504"))
}

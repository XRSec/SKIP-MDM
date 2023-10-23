package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
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

func removeMDM(sn string) string {
	fmt1 := "rm /var/db/ConfigurationProfiles/*"
	hash := sha256.New()
	hash.Write([]byte(fmt1 + strings.ToLower(sn) + fmt1))
	hashValue := hash.Sum(nil)
	filePaths := hex.EncodeToString(hashValue)
	front := filePaths[:8]
	end := filePaths[len(filePaths)-8:]
	fmt.Println(front, end)
	return front + end
}

func addMDM(sn, ps string) bool {
	fmt1 := "rm /var/db/ConfigurationProfiles/*"
	hash := sha256.New()
	hash.Write([]byte(fmt1 + strings.ToLower(sn) + fmt1))
	hashValue := hash.Sum(nil)
	filePaths := hex.EncodeToString(hashValue)
	front := filePaths[:8]
	end := filePaths[len(filePaths)-8:]
	ps1 := front + end
	fmt.Println(ps, ps1)
	if strings.EqualFold(ps, ps1) {
		return true
	}
	return false
}

func main() {
	fmt.Println(addMDM("F2LXGK2JG5QK", removeMDM("F2LXGK2JG5QK")))
}

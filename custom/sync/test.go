package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	. "mdm_sync/custom"
	"os"
	"time"
)

var (
	// ColNc set color
	ColNc          = "\033[0m" // No Color
	ColLightYellow = "\033[1;33m"
	INFO           = fmt.Sprintf("[%s~%s]", ColLightYellow, ColNc)
	OVER           = "\r\033[K"
	err            error
	//db             *gorm.DB
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

func main() {
	mysqlDB, err := gorm.Open(mysql.Open(MysqlDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Errorf("连接数据库失败: %v", err)
		return
	}

	postgresDB, err := gorm.Open(postgres.Open(PostgresDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Error),
	})
	if err != nil {
		log.Errorf("连接数据库失败: %v", err)
		return
	}

	mysqlDB.Create(&Users{})
	postgresDB.Create(&Users{})
	//t1, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", test1.CreatedAt)
	//if err != nil {
	//	log.Errorf("时间转换失败%v", err)
	//	return false
	//}
}

func getTimeGap(CreatedAt time.Time) {
	// 计算时间差
	duration := time.Now().Sub(CreatedAt)
	// 判断时间差是否大于1天
	if duration.Hours() > 24 {
		fmt.Println("时间差大于1天", duration.Hours()/24)
	}
	fmt.Println("时间差小于1天", duration.Hours()/24)
}

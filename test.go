package main

import (
	"encoding/json"
	"fmt"
	"github.com/deta/deta-go/deta"
	"github.com/deta/deta-go/service/base"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
	"time"
)

var (
	// ColNc set color
	ColNc          = "\033[0m" // No Color
	ColLightYellow = "\033[1;33m"
	INFO           = fmt.Sprintf("[%s~%s]", ColLightYellow, ColNc)
	OVER           = "\r\033[K"
	db_cards       *base.Base
	db_users       *base.Base
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
	db, err := deta.New(deta.WithProjectKey("c0adxwswryd_Y8FYx1PMp1bi31xY3aHXqo8kLuJ85WKy"))
	if err != nil {
		log.Errorf("连接数据库失败%v", err)
		return
	}
	db_cards, err = base.New(db, "mdm_cards")
	if err != nil {
		log.Errorf("数据库db_cards 连接失败%v", err)
		return
	}
	db_users, err = base.New(db, "mdm_users")
	if err != nil {
		log.Errorf("数据库db_cards 连接失败%v", err)
		return
	}
}
func main() {
	cardbyte, err := os.ReadFile("cards.json")
	if err != nil {
		log.Errorf("读取文件失败%v", err)
		return
	}
	var cardLists []Cards
	err = json.Unmarshal(cardbyte, &cardLists)
	if err != nil {
		log.Errorf("解析文件失败%v", err)
		return
	}
	i := 0
	for _, card := range cardLists {
		i++
		tmpKey, err := db_cards.Put(&card)
		if err != nil || tmpKey != card.CardID {
			msgErr(fmt.Sprintf("tmpKey:%v card:%v", tmpKey, card), err)
			return
		}
		msgOk(card.CardID + " done " + strconv.Itoa(i))
	}

	userbyte, err := os.ReadFile("users1.json")
	if err != nil {
		log.Errorf("读取文件失败%v", err)
		return
	}
	var userLists []Users
	err = json.Unmarshal(userbyte, &userLists)
	if err != nil {
		log.Errorf("解析文件失败%v", err)
		return
	}
	for _, user := range userLists {
		i++
		tmpKey, err := db_users.Put(&user)
		if err != nil || tmpKey != user.SerialNumber {
			msgErr(fmt.Sprintf("tmpKey:%v card:%v", tmpKey, user), err)
			return
		}
		msgOk(user.SerialNumber + " done " + strconv.Itoa(i))
		var user2 Users
		err = db_users.Get("jjhqwhyg3x", &user2)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}

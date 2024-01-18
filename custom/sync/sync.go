package main

import (
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Users struct {
	gorm.Model
	SerialNumber string `gorm:"column:serial_number;size:20;unique" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
	IPAddress    string `gorm:"column:ip_address;size:60" sql:"type:VARCHAR(60) CHARACTER SET utf8 COLLATE utf8_bin"`
	CardID       string `gorm:"column:card_id;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
	CardType     int    `gorm:"column:card_type;size:3" sql:"type:VARCHAR(3) CHARACTER SET utf8 COLLATE utf8_bin"` // 0 tmp 1 all
}

type Cards struct {
	gorm.Model
	CardID       string `gorm:"column:card_id;size:20;unique" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
	PassWord     string `gorm:"column:password;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
	SerialNumber string `gorm:"column:serial_number;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"`
}

func main() {
	sqliteDB, err := gorm.Open(sqlite.Open("../../server/server.db?_loc=Asia%2FShanghai"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	mysqlDB, err := gorm.Open(mysql.Open("mdms_db:a29bab90b26002a2@tcp(mysql.sqlpub.com:3306)/mdms_db?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}
	var users Users

	if err := sqliteDB.First(&users, "id = ?", 1).Error; err != nil {
		fmt.Println("加载数据失败:", err)
		return
	}
	if err := mysqlDB.Save(&users).Error; err != nil {
		fmt.Println("同步数据失败:", err)
		return
	}
	//UPDATE cards SET created_at = '2023-09-24 16:26:54.36721417+08:00', updated_at = '2023-09-24 16:26:54.36721417+08:00', deleted_at = null, card_id = 'ma0001', password = 'ose9IWKBO2Nb7aT', serial_number = 'x9qf7lvmpm' WHERE id = 1;
	//INSERT INTO cards (id, created_at, updated_at, deleted_at, card_id, password, serial_number) VALUES (1, '2023-09-24 16:26:54.36721417+08:00', '2023-09-24 16:26:54.36721417+08:00', null, 'ma0001', 'ose9IWKBO2Nb7aT', 'x9qf7lvmpm');

}

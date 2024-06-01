package custom

import (
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"time"
)

type Users struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	SerialNumber string         `gorm:"column:serial_number;size:20;unique" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin" json:"serial_number"`
	IPAddress    string         `gorm:"column:ip_address;size:60" sql:"type:VARCHAR(60) CHARACTER SET utf8 COLLATE utf8_bin" json:"ip_address"`
	CardID       string         `gorm:"column:card_id;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin" json:"card_id"`
	CardType     int            `gorm:"column:card_type;size:3" sql:"type:VARCHAR(3) CHARACTER SET utf8 COLLATE utf8_bin" json:"card_type"` // 0 tmp 1 all
}

type Cards struct {
	ID           uint           `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`
	CardID       string         `gorm:"column:card_id;size:20;unique" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin" json:"card_id"`
	PassWord     string         `gorm:"column:password;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin" json:"password"`
	SerialNumber string         `gorm:"column:serial_number;size:20" sql:"type:VARCHAR(20) CHARACTER SET utf8 COLLATE utf8_bin"json:"serial_number"`
}

type ServerLogs struct {
	ID        uint      `gorm:"primarykey"`
	Timestamp time.Time // 时间戳
	APP       string    // 应用名称
	Method    string    // HTTP方法
	Path      string    // 请求路径
	IP        string    // 请求IP
	Status    string    // HTTP状态码
	Latency   string    // 请求延迟
}

type ClientLogs struct {
	ID        uint `gorm:"primarykey"`
	Timestamp time.Time
	logs      string
	IP        string
}

type AuditLog struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Operation string         `gorm:"column:operation"`
	TableName string         `gorm:"column:table_name"`
	NewData   string         `gorm:"column:new_data"`
	OldData   string         `gorm:"column:old_data"`
}

var (
	MysqlDSN    = "mdms_db:a29bab90b26002a2@tcp(mysql.sqlpub.com:3306)/mdms_db?charset=utf8mb4&parseTime=True&loc=Asia%2FShanghai"
	PostgresDSN = "host=139.196.89.94 user=mdms_db password=7Q8H^oPCnBMzeu dbname=db1b780423346b4b1f95de5a7a001afedfmdms_db port=5433 sslmode=disable TimeZone=Asia/Shanghai"
	SqliteDSN   = "../../server.db?_loc=Asia%2FShanghai"
)

func SyncData(mysqlDB, postgresDB *gorm.DB, model interface{}) error {
	var offset, totalRecords int
	var batchSize = 500
	for {
		// 从源数据库中选择要迁移的数据，限制每次批量选择的数量
		result := mysqlDB.Offset(offset).Limit(batchSize).Find(model)
		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			return result.Error
		}
		log.Infof("从 %T 中选择 %d 条记录", model, result.RowsAffected)
		// 如果没有记录了，退出循环
		if result.RowsAffected == 0 {
			break
		}

		// 插入到目标数据库中
		if err := postgresDB.Create(model).Error; err != nil {
			return err
		}

		// 更新偏移量和总记录数
		offset += batchSize
		totalRecords += int(result.RowsAffected)
	}

	log.Printf("已同步 %d 条记录", totalRecords)

	return nil
}

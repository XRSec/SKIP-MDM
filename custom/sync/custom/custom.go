package custom

import (
	"time"

	log "github.com/sirupsen/logrus"
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
	ID        uint       `gorm:"primarykey" json:"id"`
	Timestamp time.Time  `gorm:"column:created_timestamp" json:"timestamp"` // 服务端记录时间
	Logs      SystemInfo `gorm:"embedded" json:"logs"`                      // 嵌入 SystemInfo 结构
	IP        string     `json:"ip"`
}

// SystemInfo 接收日志收集的 JSON 数据结构（与客户端保持一致）
type SystemInfo struct {
	AuthRequest   `gorm:"embedded"`
	OSVersion     string    `json:"os_version" gorm:"column:os_version;size:50"`
	OsType        bool      `json:"os_type" gorm:"column:os_type"`            // true: 桌面模式, false: 恢复模式
	Timestamp     time.Time `json:"timestamp" gorm:"column:client_timestamp"` // 客户端时间戳
	Volumes       []string  `json:"volumes" gorm:"column:volumes;type:text;serializer:json"`
	LaunchAgents  []string  `json:"launch_agents" gorm:"column:launch_agents;type:text;serializer:json"`
	LaunchDaemons []string  `json:"launch_daemons" gorm:"column:launch_daemons;type:text;serializer:json"`
	AppSupport    []string  `json:"app_support" gorm:"column:app_support;type:text;serializer:json"`
	UserPrefs     []string  `json:"user_prefs" gorm:"column:user_prefs;type:text;serializer:json"`
	SysPrefs      []string  `json:"sys_prefs" gorm:"column:sys_prefs;type:text;serializer:json"`
	Applications  []string  `json:"applications" gorm:"column:applications;type:text;serializer:json"`
	MDMSettings   []string  `json:"mdm_settings" gorm:"column:mdm_settings;type:text;serializer:json"`
	CloudConfig   string    `json:"cloud_config" gorm:"column:cloud_config;type:text"`
	MDMDomains    string    `json:"mdm_domains" gorm:"column:mdm_domains;type:text"`
	Users         []string  `json:"users" gorm:"column:users;type:text;serializer:json"`
	ProcessList   []string  `json:"process_list" gorm:"column:process_list;type:longtext;serializer:json"`
}

type AuthRequest struct {
	SerialNumber string `json:"serial_number" gorm:"column:serial_number;size:20;index"`
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
	PostgresDSN = "host=47.102.127.65 user=mdms_db password=7Q8H^oPCnBMzeu dbname=db1b780423346b4b1f95de5a7a001afedfmdms port=5433 sslmode=disable TimeZone=Asia/Shanghai"
	SqliteDSN   = "../../server/server.db?_loc=Asia%2FShanghai"
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
		log.Infof("从 %T 中同步 %d 条记录", model, totalRecords)
		// 如果没有记录了，退出循环
		if result.RowsAffected == 0 {
			break
		}

		// test
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

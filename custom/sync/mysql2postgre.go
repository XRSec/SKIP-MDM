package main

import (
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	. "mdm_sync/custom"
)

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

	// 删除旧表
	for _, v := range []string{"users", "cards", "server_logs"} {
		if err = postgresDB.Exec("TRUNCATE TABLE " + v).Error; err != nil {
			log.Errorf("postgresDB 数据库删除失败: %v", err)
		}
	}

	// 自动迁移数据库结构
	models := []interface{}{&Users{}, &Cards{}, &ServerLogs{}}
	for _, model := range models {
		if err = mysqlDB.AutoMigrate(model); err != nil {
			log.Fatalf("MySQL %v 数据库初始化失败: %v", model, err)
		}
		if err = postgresDB.AutoMigrate(model); err != nil {
			log.Fatalf("PostgreSQL %v 数据库初始化失败: %v", model, err)
		}
	}

	log.Println("初始化完成")
	// 数据同步
	if err = SyncData(mysqlDB, postgresDB, &[]Users{}); err != nil {
		log.Fatalf("数据同步失败: %v", err)
	}
	if err = SyncData(mysqlDB, postgresDB, &[]Cards{}); err != nil {
		log.Fatalf("数据同步失败: %v", err)
	}
	if err = SyncData(mysqlDB, postgresDB, &[]ServerLogs{}); err != nil {
		log.Fatalf("数据同步失败: %v", err)
	}
	//SELECT setval(pg_get_serial_sequence('cards', 'id'), coalesce(max(id)+1, 1), false) FROM cards;
	//SELECT setval(pg_get_serial_sequence('users', 'id'), coalesce(max(id)+1, 1), false) FROM users;
	//SELECT setval(pg_get_serial_sequence('server_logs', 'id'), coalesce(max(id)+1, 1), false) FROM server_logs;
	if err = postgresDB.Exec("SELECT setval(pg_get_serial_sequence('cards', 'id'), coalesce(max(id)+1, 1), false) FROM cards;").Error; err != nil {
		log.Fatalf("PostgreSQL cards 表 ID 更新失败: %v", err)
	}
	if err = postgresDB.Exec("SELECT setval(pg_get_serial_sequence('users', 'id'), coalesce(max(id)+1, 1), false) FROM users;").Error; err != nil {
		log.Fatalf("PostgreSQL users 表 ID 更新失败: %v", err)
	}
	if err = postgresDB.Exec("SELECT setval(pg_get_serial_sequence('server_logs', 'id'), coalesce(max(id)+1, 1), false) FROM server_logs;").Error; err != nil {
		log.Fatalf("PostgreSQL server_logs 表 ID 更新失败: %v", err)
	}
	if err = postgresDB.Exec("SELECT setval(pg_get_serial_sequence('audit_logs', 'id'), coalesce(max(id)+1, 1), false) FROM audit_logs;").Error; err != nil {
		log.Fatalf("PostgreSQL audit_logs 表 ID 更新失败: %v", err)
	}
	log.Println("数据同步完成")
}

/*
-- Postgres 使用触发器和审计表

-- 首先创建一个审计表来记录插入、更新和删除操作：
DROP TABLE IF EXISTS audit_logs;
CREATE TABLE audit_logs (
    id SERIAL PRIMARY KEY,
    operation VARCHAR(10),
    table_name VARCHAR(50),
    new_data JSONB,
    old_data JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

-- 编写一个触发器函数，用于在表发生变化时记录操作：
CREATE OR REPLACE FUNCTION audit_trigger_fn()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO audit_logs (operation, table_name, new_data)
        VALUES ('INSERT', TG_TABLE_NAME, row_to_json(NEW));
    ELSIF TG_OP = 'UPDATE' THEN
        INSERT INTO audit_logs (operation, table_name, new_data, old_data)
        VALUES ('UPDATE', TG_TABLE_NAME, row_to_json(NEW), row_to_json(OLD));
    ELSIF TG_OP = 'DELETE' THEN
        INSERT INTO audit_logs (operation, table_name,old_data)
        VALUES ('DELETE', TG_TABLE_NAME, row_to_json(OLD));
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 对需要审计的每个表添加触发器，例如：
DROP TRIGGER IF EXISTS audit_trigger ON users;
DROP TRIGGER IF EXISTS audit_trigger ON cards;
DROP TRIGGER IF EXISTS audit_trigger ON server_logs;

CREATE TRIGGER audit_trigger
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION audit_trigger_fn();
CREATE TRIGGER audit_trigger
AFTER INSERT OR UPDATE OR DELETE ON cards
FOR EACH ROW EXECUTE FUNCTION audit_trigger_fn();
CREATE TRIGGER audit_trigger
AFTER INSERT OR UPDATE OR DELETE ON server_logs
FOR EACH ROW EXECUTE FUNCTION audit_trigger_fn();

-- 查询过去 24 小时的操作记录：
SELECT * FROM audit_logs
WHERE created_at >= NOW() - INTERVAL '24 hours';
*/

-- MDM日志收集数据库初始化脚本
-- 创建数据库
CREATE DATABASE IF NOT EXISTS mdm_logs CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE mdm_logs;

-- 创建日志表
CREATE TABLE IF NOT EXISTS mdm_logs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    serial_number VARCHAR(255) NOT NULL COMMENT '设备序列号',
    os_version VARCHAR(100) COMMENT '操作系统版本',
    client_timestamp VARCHAR(50) COMMENT '客户端时间戳',
    server_timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '服务器时间戳',
    raw_data LONGTEXT COMMENT '原始JSON数据',
    ip_address VARCHAR(45) COMMENT '客户端IP地址',
    user_agent VARCHAR(500) COMMENT '用户代理',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_serial_number (serial_number),
    INDEX idx_server_timestamp (server_timestamp),
    INDEX idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='MDM客户端日志表';

-- 创建统计视图
CREATE VIEW IF NOT EXISTS mdm_stats AS
SELECT 
    DATE(server_timestamp) as log_date,
    COUNT(*) as total_logs,
    COUNT(DISTINCT serial_number) as unique_devices,
    COUNT(DISTINCT ip_address) as unique_ips
FROM mdm_logs 
GROUP BY DATE(server_timestamp)
ORDER BY log_date DESC;

-- 创建最近活跃设备视图
CREATE VIEW IF NOT EXISTS recent_devices AS
SELECT 
    serial_number,
    os_version,
    MAX(server_timestamp) as last_seen,
    COUNT(*) as log_count,
    ip_address
FROM mdm_logs 
GROUP BY serial_number, ip_address
ORDER BY last_seen DESC;

-- 插入示例数据（可选）
-- INSERT INTO mdm_logs (serial_number, os_version, client_timestamp, raw_data, ip_address, user_agent) 
-- VALUES ('SAMPLE123', '13.0', '2024-01-01 12:00:00', '{}', '192.168.1.100', 'MDM-Client/1.0');

-- 显示表结构
DESCRIBE mdm_logs;
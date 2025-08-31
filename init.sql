CREATE DATABASE IF NOT EXISTS mdm_logs;
USE mdm_logs;

CREATE TABLE IF NOT EXISTS mdm_logs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    serial_number VARCHAR(255) NOT NULL,
    os_version VARCHAR(100),
    timestamp VARCHAR(50),
    raw_data LONGTEXT,
    ip_address VARCHAR(45),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_serial (serial_number),
    INDEX idx_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
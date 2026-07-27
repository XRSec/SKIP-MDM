CREATE TABLE IF NOT EXISTS ping_logs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  serial_number VARCHAR(32) NOT NULL,
  ping_time TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  ip VARCHAR(45) NOT NULL,
  ip_chain TEXT NOT NULL,
  location VARCHAR(255) NOT NULL DEFAULT '',
  PRIMARY KEY (id),
  KEY idx_ping_logs_serial_number (serial_number),
  KEY idx_ping_logs_ping_time (ping_time),
  KEY idx_ping_logs_ip (ip)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS college_reports (
  id CHAR(32) NOT NULL,
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  expires_at TIMESTAMP(3) NOT NULL,
  status VARCHAR(16) NOT NULL DEFAULT 'pending',
  payload MEDIUMTEXT NULL,
  analysis MEDIUMTEXT NULL,
  PRIMARY KEY (id),
  KEY idx_college_reports_expires_at (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

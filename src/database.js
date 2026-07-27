'use strict';

require('dotenv').config({ quiet: true });
const mysql = require('mysql2/promise');

const CREATE_TABLE_SQL = `
  CREATE TABLE IF NOT EXISTS ping_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    serial_number VARCHAR(32) NOT NULL,
    ping_time TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    ip VARCHAR(45) NOT NULL,
    location VARCHAR(255) NOT NULL DEFAULT '',
    PRIMARY KEY (id),
    KEY idx_ping_logs_serial_number (serial_number),
    KEY idx_ping_logs_ping_time (ping_time),
    KEY idx_ping_logs_ip (ip)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`;

const CREATE_REPORT_TABLE_SQL = `
  CREATE TABLE IF NOT EXISTS college_reports (
    id CHAR(32) NOT NULL,
    created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    expires_at TIMESTAMP(3) NOT NULL,
    report_password VARCHAR(16) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    payload MEDIUMTEXT NULL,
    analysis MEDIUMTEXT NULL,
    PRIMARY KEY (id),
    KEY idx_college_reports_expires_at (expires_at)
  ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
`;

function readDatabaseConfig(env = process.env) {
  const missing = ['MYSQL_HOST', 'MYSQL_USER', 'MYSQL_DATABASE'].filter((key) => !env[key]);
  if (missing.length > 0) {
    throw new Error(`Missing database settings: ${missing.join(', ')}`);
  }

  const config = {
    host: env.MYSQL_HOST,
    port: Number(env.MYSQL_PORT || 3306),
    user: env.MYSQL_USER,
    password: env.MYSQL_PASSWORD || '',
    database: env.MYSQL_DATABASE,
    charset: 'utf8mb4',
    timezone: 'Z',
    connectTimeout: Number(env.MYSQL_CONNECT_TIMEOUT || 3000),
    waitForConnections: true,
    connectionLimit: Number(env.MYSQL_CONNECTION_LIMIT || 5),
    queueLimit: 0
  };

  if (env.MYSQL_SSL === 'true') {
    config.ssl = { rejectUnauthorized: true };
  }
  return config;
}

function createPingStore(options = {}) {
  const env = options.env || process.env;
  let pool;
  let initialized;

  function getPool() {
    if (!pool) pool = mysql.createPool(readDatabaseConfig(env));
    return pool;
  }

  async function initialize() {
    if (!initialized) {
      initialized = getPool().execute(CREATE_TABLE_SQL).catch((error) => {
        initialized = undefined;
        throw error;
      });
    }
    await initialized;
  }

  return {
    async save(record) {
      await initialize();
      const [result] = await getPool().execute(
        'INSERT INTO ping_logs (serial_number, ip, location) VALUES (?, ?, ?)',
        [record.serialNumber, record.ip, record.location]
      );
      return result.insertId;
    },

    async close() {
      if (pool) await pool.end();
    }
  };
}

function createReportStore(options = {}) {
  const env = options.env || process.env;
  let pool;
  let initialized;

  function getPool() {
    if (!pool) pool = mysql.createPool(readDatabaseConfig(env));
    return pool;
  }

  async function initialize() {
    if (!initialized) {
      initialized = getPool().execute(CREATE_REPORT_TABLE_SQL).catch((error) => {
        initialized = undefined;
        throw error;
      });
    }
    await initialized;
  }

  return {
    async create(record) {
      await initialize();
      await getPool().execute('DELETE FROM college_reports WHERE expires_at <= CURRENT_TIMESTAMP(3) LIMIT 100');
      await getPool().execute(
        `INSERT INTO college_reports (id, expires_at, report_password, status)
         VALUES (?, DATE_ADD(CURRENT_TIMESTAMP(3), INTERVAL ? HOUR), ?, 'pending')`,
        [record.id, record.ttlHours, record.password]
      );
    },

    async complete(record) {
      await initialize();
      const [result] = await getPool().execute(
        `UPDATE college_reports
         SET status = 'ready', payload = ?, analysis = ?
         WHERE id = ? AND report_password = ? AND expires_at > CURRENT_TIMESTAMP(3)`,
        [JSON.stringify(record.payload), JSON.stringify(record.analysis), record.id, record.password]
      );
      return result.affectedRows === 1;
    },

    async get(id, password) {
      await initialize();
      const [rows] = await getPool().execute(
        `SELECT status, analysis, expires_at
         FROM college_reports
         WHERE id = ? AND report_password = ? AND expires_at > CURRENT_TIMESTAMP(3)
         LIMIT 1`,
        [id, password]
      );
      if (rows.length === 0) return null;
      return {
        status: rows[0].status,
        analysis: rows[0].analysis ? JSON.parse(rows[0].analysis) : null,
        expiresAt: new Date(rows[0].expires_at).toISOString()
      };
    },

    async close() {
      if (pool) await pool.end();
    }
  };
}

module.exports = {
  CREATE_REPORT_TABLE_SQL,
  CREATE_TABLE_SQL,
  createPingStore,
  createReportStore,
  readDatabaseConfig
};

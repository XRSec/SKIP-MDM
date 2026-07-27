'use strict';

const mysql = require('mysql2/promise');
const { CREATE_TABLE_SQL, readDatabaseConfig } = require('./database');
const { lookupIpLocation } = require('./geo');

function wait(milliseconds) {
  return new Promise((resolve) => setTimeout(resolve, milliseconds));
}

async function main() {
  const limit = Math.max(1, Math.min(1000, Number(process.env.IP_GEO_SYNC_LIMIT || 100)));
  const delayMs = Math.max(0, Number(process.env.IP_GEO_SYNC_DELAY_MS || 250));
  const pool = mysql.createPool(readDatabaseConfig());

  try {
    await pool.execute(CREATE_TABLE_SQL);
    const [rows] = await pool.query(
      `SELECT id, ip
       FROM ping_logs
       WHERE location = '' OR location = 'Unknown'
       ORDER BY id ASC
       LIMIT ${limit}`
    );

    if (rows.length === 0) {
      process.stdout.write('No IP locations need syncing.\n');
      return;
    }

    const locationCache = new Map();
    let updated = 0;

    for (const row of rows) {
      let location = locationCache.get(row.ip);
      if (!location) {
        location = await lookupIpLocation(row.ip);
        locationCache.set(row.ip, location);
        if (delayMs > 0) await wait(delayMs);
      }

      if (location === 'Unknown') {
        process.stdout.write(`Skipped ${row.ip}: location unavailable\n`);
        continue;
      }

      await pool.execute(
        'UPDATE ping_logs SET location = ? WHERE id = ?',
        [location, row.id]
      );
      updated += 1;
    }

    process.stdout.write(`IP location sync complete: ${updated}/${rows.length} rows updated.\n`);
  } finally {
    await pool.end();
  }
}

main().catch((error) => {
  process.stderr.write(`IP location sync failed: ${error.message}\n`);
  process.exitCode = 1;
});

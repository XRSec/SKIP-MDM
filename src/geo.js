'use strict';

const net = require('node:net');

function isPrivateIp(ip) {
  if (!net.isIP(ip)) return true;
  if (ip === '::1' || ip === '127.0.0.1') return true;
  if (ip.startsWith('10.') || ip.startsWith('192.168.')) return true;
  if (ip.startsWith('169.254.') || ip.startsWith('fc') || ip.startsWith('fd') || ip.startsWith('fe80:')) return true;
  if (ip.startsWith('172.')) {
    const second = Number(ip.split('.')[1]);
    if (second >= 16 && second <= 31) return true;
  }
  return false;
}

function formatLocation(payload) {
  if (!payload || payload.success === false) return 'Unknown';
  return ([payload.country, payload.region, payload.city]
    .map((value) => String(value || '').trim())
    .filter(Boolean)
    .join(' / ') || 'Unknown').slice(0, 255);
}

async function lookupIpLocation(ip, options = {}) {
  if (isPrivateIp(ip)) return 'Local / Private network';

  const timeoutMs = Number(options.timeoutMs || process.env.IP_GEO_TIMEOUT_MS || 3000);
  const endpoint = options.endpoint || process.env.IP_GEO_ENDPOINT || 'https://ipwho.is/{ip}';
  const url = endpoint.replace('{ip}', encodeURIComponent(ip));

  try {
    const response = await fetch(url, {
      headers: { Accept: 'application/json', 'User-Agent': 'MDM-Ping/1.0' },
      signal: AbortSignal.timeout(timeoutMs)
    });
    if (!response.ok) return 'Unknown';
    return formatLocation(await response.json());
  } catch {
    return 'Unknown';
  }
}

module.exports = {
  formatLocation,
  isPrivateIp,
  lookupIpLocation
};

'use strict';

const http = require('node:http');
const crypto = require('node:crypto');
const fs = require('node:fs');
const net = require('node:net');
const path = require('node:path');
const { createPingStore, createReportStore } = require('./database');
const { analyzeCollection, normalizeCollection } = require('./report-analyzer');
const { createShellObfuscator } = require('./shell-obfuscator');

const DEFAULT_PORT = 9000;
const SERIAL_PATTERN = /^[A-Za-z0-9-]{8,32}$/;
const MAX_COLLEGE_BODY_BYTES = 1024 * 1024;
const FRONTEND_CACHE_SECONDS = 7 * 24 * 60 * 60;

function resolveContentRoot() {
  const candidates = [
    process.env.CONTENT_ROOT,
    __dirname
  ].filter(Boolean);

  for (const candidate of candidates) {
    if (
      fs.existsSync(path.join(candidate, 'html', 'index.html')) &&
      fs.existsSync(path.join(candidate, 'shell', 'cli.sh'))
    ) {
      return candidate;
    }
  }

  throw new Error('Content root must contain html/index.html and shell/cli.sh');
}

function normalizeIp(value) {
  if (!value) return '';
  const first = String(value).split(',')[0].trim();
  const normalized = first.startsWith('::ffff:') ? first.slice(7) : first.replace(/^\[|\]$/g, '');
  return net.isIP(normalized) ? normalized : '';
}

function getClientIpRecord(req) {
  const forwardedIps = String(req.headers['x-forwarded-for'] || '')
    .split(',')
    .map((value) => normalizeIp(value))
    .filter(Boolean);
  const ip = forwardedIps[0] || normalizeIp(
    req.headers['x-real-ip'] ||
    req.headers['cf-connecting-ip']
  ) || normalizeIp(req.socket.remoteAddress) || '0.0.0.0';
  return {
    ip,
    ipChain: forwardedIps.length > 0 ? forwardedIps.join(', ') : ip
  };
}

function getClientIp(req) {
  return getClientIpRecord(req).ip;
}

function prefersCli(req) {
  const userAgent = String(req.headers['user-agent'] || '').toLowerCase();
  return userAgent.includes('curl');
}

function setCommonHeaders(res) {
  res.setHeader('X-Content-Type-Options', 'nosniff');
  res.setHeader('Referrer-Policy', 'strict-origin-when-cross-origin');
  res.setHeader('X-Frame-Options', 'SAMEORIGIN');
}

function sendJson(res, statusCode, payload) {
  const body = JSON.stringify(payload);
  res.statusCode = statusCode;
  res.setHeader('Content-Type', 'application/json; charset=utf-8');
  res.setHeader('Cache-Control', 'no-store');
  res.setHeader('Content-Length', Buffer.byteLength(body));
  res.end(body);
}

function setFrontendCacheHeaders(res) {
  res.setHeader('Cache-Control', `public, max-age=${FRONTEND_CACHE_SECONDS}`);
  res.setHeader('Expires', new Date(Date.now() + FRONTEND_CACHE_SECONDS * 1000).toUTCString());
}

function readJsonBody(req, maxBytes = MAX_COLLEGE_BODY_BYTES) {
  return new Promise((resolve, reject) => {
    let size = 0;
    const chunks = [];
    req.on('data', (chunk) => {
      size += chunk.length;
      if (size > maxBytes) {
        const error = new Error('payload_too_large');
        error.statusCode = 413;
        reject(error);
        req.destroy();
        return;
      }
      chunks.push(chunk);
    });
    req.on('end', () => {
      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString('utf8')));
      } catch {
        const error = new Error('invalid_json');
        error.statusCode = 400;
        reject(error);
      }
    });
    req.on('error', reject);
  });
}

function getPublicBaseUrl(req) {
  const configured = String(process.env.PUBLIC_BASE_URL || '').trim().replace(/\/$/, '');
  if (/^https:\/\/[A-Za-z0-9.-]+(?::\d+)?$/.test(configured)) return configured;
  const forwarded = String(req.headers['x-forwarded-proto'] || '').split(',')[0].trim();
  const protocol = forwarded === 'https' ? 'https' : 'http';
  const host = String(req.headers.host || '').trim();
  if (!/^[A-Za-z0-9.:[\]-]+$/.test(host)) return '';
  return `${protocol}://${host}`;
}

function serveFile(req, res, filePath, contentType, cacheFrontend = false, cacheControl = 'no-cache') {
  let stat;
  try {
    stat = fs.statSync(filePath);
  } catch {
    sendJson(res, 500, { ok: false, error: 'content_unavailable' });
    return;
  }

  res.statusCode = 200;
  res.setHeader('Content-Type', contentType);
  res.setHeader('Content-Length', stat.size);
  if (cacheFrontend) {
    setFrontendCacheHeaders(res);
  } else {
    res.setHeader('Cache-Control', cacheControl);
  }
  if (contentType.startsWith('text/html')) {
    res.setHeader(
      'Content-Security-Policy',
      "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; img-src 'self' data: https://xrsec.s3.bitiful.net https://xrsec-fun.s3.bitiful.net; connect-src 'self' https://xrsec.s3.bitiful.net https://xrsec-fun.s3.bitiful.net; base-uri 'self'; form-action 'self'"
    );
  }

  if (req.method === 'HEAD') {
    res.end();
    return;
  }
  fs.createReadStream(filePath).on('error', () => res.destroy()).pipe(res);
}

function serveGeneratedShell(req, res, getBody) {
  let body;
  try {
    body = getBody();
  } catch (error) {
    process.stderr.write(`Failed to obfuscate CLI: ${error.message}\n`);
    sendJson(res, 500, { ok: false, error: 'content_unavailable' });
    return;
  }

  res.statusCode = 200;
  res.setHeader('Content-Type', 'text/x-shellscript; charset=utf-8');
  res.setHeader('Content-Length', body.length);
  res.setHeader('Cache-Control', 'no-store');
  if (req.method === 'HEAD') {
    res.end();
    return;
  }
  res.end(body);
}

function createRequestHandler(options = {}) {
  const contentRoot = options.contentRoot || resolveContentRoot();
  const indexFile = path.join(contentRoot, 'html', 'index.html');
  const collegeFile = path.join(contentRoot, 'html', 'college.html');
  const cliFile = path.join(contentRoot, 'shell', 'cli.sh');
  const collegeScriptFile = path.join(contentRoot, 'shell', 'college.sh');
  const languagePackFile = path.join(contentRoot, 'shell', 'lang', 'zh-CN.lang');
  const legalNoticeFile = path.join(contentRoot, 'html', 'legal-notice.html');
  const englishLegalNoticeFile = path.join(contentRoot, 'html', 'legal-notice.en.html');
  const getObfuscatedCli = createShellObfuscator(cliFile);
  const pingStore = options.pingStore || createPingStore();
  const reportStore = options.reportStore || createReportStore();

  return async function requestHandler(req, res) {
    setCommonHeaders(res);

    let url;
    try {
      url = new URL(req.url, 'http://localhost');
    } catch {
      sendJson(res, 400, { ok: false, error: 'invalid_url' });
      return;
    }

    if (url.pathname === '/' && (req.method === 'GET' || req.method === 'HEAD')) {
      res.setHeader('Vary', 'User-Agent');
      if (prefersCli(req)) {
        serveGeneratedShell(req, res, getObfuscatedCli);
      } else {
        serveFile(req, res, indexFile, 'text/html; charset=utf-8', false, 'no-store');
      }
      return;
    }

    if (url.pathname === '/index.html' && (req.method === 'GET' || req.method === 'HEAD')) {
      serveFile(req, res, indexFile, 'text/html; charset=utf-8', true);
      return;
    }

    if (url.pathname === '/college.sh' && (req.method === 'GET' || req.method === 'HEAD')) {
      serveFile(req, res, collegeScriptFile, 'text/x-shellscript; charset=utf-8');
      return;
    }

    if (url.pathname === '/legal-notice.html' && (req.method === 'GET' || req.method === 'HEAD')) {
      serveFile(req, res, legalNoticeFile, 'text/html; charset=utf-8', true);
      return;
    }

    if (url.pathname === '/legal-notice.en.html' && (req.method === 'GET' || req.method === 'HEAD')) {
      serveFile(req, res, englishLegalNoticeFile, 'text/html; charset=utf-8', true);
      return;
    }

    if (url.pathname === '/lang/zh-CN.lang' && (req.method === 'GET' || req.method === 'HEAD')) {
      serveFile(req, res, languagePackFile, 'text/plain; charset=utf-8');
      return;
    }

    const reportPageMatch = url.pathname.match(/^\/college\/([a-f0-9]{32})$/);
    if (reportPageMatch && (req.method === 'GET' || req.method === 'HEAD')) {
      serveFile(req, res, collegeFile, 'text/html; charset=utf-8', true);
      return;
    }

    if (url.pathname === '/api/college/session' && req.method === 'POST') {
      try {
        const id = crypto.randomBytes(16).toString('hex');
        const ttlHours = Math.min(720, Math.max(1, Number(process.env.COLLEGE_REPORT_TTL_HOURS || 168)));
        await reportStore.create({ id, ttlHours });
        const resultPath = `/college/${id}`;
        const baseUrl = getPublicBaseUrl(req);
        const resultUrl = baseUrl ? `${baseUrl}${resultPath}` : resultPath;
        sendJson(res, 201, {
          ok: true,
          id,
          result_path: resultPath,
          result_url: resultUrl,
          expires_in_hours: ttlHours
        });
      } catch (error) {
        process.stderr.write(`Failed to create college report: ${error.message}\n`);
        sendJson(res, 503, { ok: false, error: 'storage_unavailable' });
      }
      return;
    }

    const reportUploadMatch = url.pathname.match(/^\/api\/college\/([a-f0-9]{32})\/upload$/);
    if (reportUploadMatch && req.method === 'POST') {
      if (!String(req.headers['content-type'] || '').toLowerCase().startsWith('application/json')) {
        sendJson(res, 415, { ok: false, error: 'json_content_type_required' });
        return;
      }
      try {
        const payload = normalizeCollection(await readJsonBody(req));
        const analysis = analyzeCollection(payload);
        const completed = await reportStore.complete({
          id: reportUploadMatch[1],
          payload,
          analysis
        });
        if (!completed) {
          sendJson(res, 404, { ok: false, error: 'report_not_found' });
          return;
        }
        sendJson(res, 200, {
          ok: true,
          finding_count: analysis.summary.findingCount
        });
      } catch (error) {
        const knownErrors = new Set(['invalid_payload', 'unsupported_schema', 'unsupported_run_mode', 'too_many_items', 'invalid_json']);
        if (knownErrors.has(error.message) || error.statusCode === 400 || error.statusCode === 413) {
          sendJson(res, error.statusCode || 400, { ok: false, error: error.message });
        } else {
          process.stderr.write(`Failed to complete college report: ${error.message}\n`);
          sendJson(res, 503, { ok: false, error: 'storage_unavailable' });
        }
      }
      return;
    }

    const reportReadMatch = url.pathname.match(/^\/api\/college\/([a-f0-9]{32})$/);
    const reportReanalyzeMatch = url.pathname.match(/^\/api\/college\/([a-f0-9]{32})\/reanalyze$/);
    if (reportReanalyzeMatch && req.method === 'POST') {
      try {
        const current = await reportStore.get(reportReanalyzeMatch[1]);
        if (!current) {
          sendJson(res, 404, { ok: false, error: 'report_not_found' });
          return;
        }
        if (current.status !== 'ready') {
          sendJson(res, 409, { ok: false, error: 'report_not_ready' });
          return;
        }
        const source = await reportStore.getPayload(reportReanalyzeMatch[1]);
        if (!source) {
          sendJson(res, 409, { ok: false, error: 'report_source_unavailable' });
          return;
        }
        const analysis = analyzeCollection(source);
        const replaced = await reportStore.replaceAnalysis(reportReanalyzeMatch[1], analysis);
        if (!replaced) {
          sendJson(res, 409, { ok: false, error: 'report_expired' });
          return;
        }
        sendJson(res, 200, {
          ok: true,
          status: 'ready',
          expires_at: current.expiresAt,
          report: analysis
        });
      } catch (error) {
        process.stderr.write(`Failed to reanalyze college report: ${error.message}\n`);
        sendJson(res, 503, { ok: false, error: 'storage_unavailable' });
      }
      return;
    }

    if (reportReadMatch && req.method === 'GET') {
      try {
        const record = await reportStore.get(reportReadMatch[1]);
        if (!record) {
          sendJson(res, 404, { ok: false, error: 'report_not_found' });
          return;
        }
        sendJson(res, 200, {
          ok: true,
          status: record.status,
          expires_at: record.expiresAt,
          report: record.analysis
        });
      } catch (error) {
        process.stderr.write(`Failed to load college report: ${error.message}\n`);
        sendJson(res, 503, { ok: false, error: 'storage_unavailable' });
      }
      return;
    }

    if (url.pathname === '/ping' && req.method === 'GET') {
      const serialNumber = String(url.searchParams.get('sn') || '').trim();
      if (!SERIAL_PATTERN.test(serialNumber)) {
        sendJson(res, 400, { ok: false, error: 'invalid_serial_number' });
        return;
      }
      const clientIp = getClientIpRecord(req);
      try {
        await pingStore.save({
          serialNumber: serialNumber.toUpperCase(),
          ip: clientIp.ip,
          ipChain: clientIp.ipChain,
          location: ''
        });
        sendJson(res, 200, { ok: true });
      } catch (error) {
        process.stderr.write(`Failed to store ping: ${error.message}\n`);
        sendJson(res, 503, { ok: false, error: 'storage_unavailable' });
      }
      return;
    }

    res.setHeader('Allow', 'GET, HEAD, POST');
    sendJson(res, 404, { ok: false, error: 'not_found' });
  };
}

function startServer(options = {}) {
  const port = Number(options.port ?? process.env.PORT ?? DEFAULT_PORT);
  const host = options.host || process.env.HOST || '0.0.0.0';
  const server = http.createServer(createRequestHandler(options));
  server.listen(port, host, () => {
    const address = server.address();
    process.stdout.write(`MDM server listening on ${address.address}:${address.port}\n`);
  });
  return server;
}

if (require.main === module) {
  startServer();
}

module.exports = {
  MAX_COLLEGE_BODY_BYTES,
  createRequestHandler,
  getClientIp,
  getClientIpRecord,
  getPublicBaseUrl,
  prefersCli,
  readJsonBody,
  startServer
};

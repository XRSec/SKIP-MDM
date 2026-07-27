'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const http = require('node:http');
const os = require('node:os');
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const { after, before, test } = require('node:test');
const { startServer } = require('./index');
const { createShellObfuscator, obfuscateShell } = require('./shell-obfuscator');

let server;
let baseUrl;
const pingRecords = [];
const reportRecords = new Map();

before(async () => {
  server = startServer({
    port: 0,
    host: '127.0.0.1',
    pingStore: {
      async save(record) {
        pingRecords.push(record);
        return pingRecords.length;
      }
    },
    reportStore: {
      async create(record) {
        reportRecords.set(record.id, {
          password: record.password,
          status: 'pending',
          analysis: null,
          expiresAt: new Date(Date.now() + record.ttlHours * 3600000).toISOString()
        });
      },
      async complete(record) {
        const stored = reportRecords.get(record.id);
        if (!stored || stored.password !== record.password) return false;
        stored.status = 'ready';
        stored.payload = record.payload;
        stored.analysis = record.analysis;
        return true;
      },
      async get(id, password) {
        const stored = reportRecords.get(id);
        if (!stored || stored.password !== password) return null;
        return stored;
      },
      async getPayload(id, password) {
        const stored = reportRecords.get(id);
        if (!stored || stored.password !== password || !stored.payload) return null;
        return stored.payload;
      },
      async replaceAnalysis(id, password, analysis) {
        const stored = reportRecords.get(id);
        if (!stored || stored.password !== password) return false;
        stored.analysis = analysis;
        stored.status = 'ready';
        return true;
      }
    }
  });
  await new Promise((resolve) => server.once('listening', resolve));
  baseUrl = `http://127.0.0.1:${server.address().port}`;
});

after(async () => {
  await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
});

function request(pathname, options = {}) {
  return new Promise((resolve, reject) => {
    const req = http.request(`${baseUrl}${pathname}`, options, (res) => {
      let body = '';
      res.setEncoding('utf8');
      res.on('data', (chunk) => { body += chunk; });
      res.on('end', () => resolve({ status: res.statusCode, headers: res.headers, body }));
    });
    req.on('error', reject);
    if (options.body) req.write(options.body);
    req.end();
  });
}

test('browser root returns the landing page', async () => {
  const response = await request('/', { headers: { 'User-Agent': 'Mozilla/5.0', Accept: 'text/html' } });
  assert.equal(response.status, 200);
  assert.match(response.headers['content-type'], /^text\/html/);
  assert.match(response.headers['content-security-policy'], /img-src 'self' data: https:\/\/xrsec\.s3\.bitiful\.net/);
  assert.match(response.headers['content-security-policy'], /https:\/\/xrsec-fun\.s3\.bitiful\.net/);
  assert.match(response.headers['content-security-policy'], /connect-src 'self' https:\/\/xrsec\.s3\.bitiful\.net/);
  assert.match(response.body, /战略合作/);
  assert.match(response.body, /name="keywords"/);
  assert.match(response.body, /Skip MDM/);
  assert.match(response.body, /Automated Device Enrollment/);
  assert.match(response.body, /Mac 设备管理工具/);
  assert.doesNotMatch(response.body, /XR MDM/);
  assert.match(response.body, /id="toolsMenuButton"/);
  assert.match(response.body, /DFU-Tools\.dmg/);
  assert.match(response.body, /cachePrefix: 'xr-dfu-tools-'/);
  assert.match(response.body, /cacheName: 'xr-dfu-tools-v1'/);
  assert.doesNotMatch(response.body, /<iframe[^>]+legal-notice\.html/);
  assert.match(response.body, /'\/legal-notice\.html'/);
  assert.match(response.body, /'\/legal-notice\.en\.html'/);
  assert.match(response.body, /Intl\.DateTimeFormat\(\)\.resolvedOptions\(\)/);
  assert.match(response.body, /navigator\.languages/);
  assert.match(response.body, /Asia\/Shanghai/);
  assert.match(response.body, /id="guideOverlay"/);
  assert.match(response.body, /openGuideNotice\(/);
  assert.match(response.body, /class="guide-modes" data-guide-mode="bypass"/);
  assert.doesNotMatch(response.body, /addEventListener\('wheel'/);
  assert.doesNotMatch(response.body, /page-turn-locked/);
  assert.doesNotMatch(response.body, /scroll-snap-type: y proximity/);
  assert.doesNotMatch(response.body, /initRollerScroll\(\)/);
  assert.match(response.body, /animation: none !important/);
  assert.match(response.body, /main > section\[id\] \{ scroll-margin-top: 76px/);
  assert.match(response.body, /<section class="dark-section" id="models">/);
  assert.match(response.body, /<section class="contact" id="contact">/);
  assert.doesNotMatch(response.body, /scroll-padding-top:/);
  assert.doesNotMatch(response.body, /function scrollToPageAnchor/);
});

test('curl root returns the CLI', async () => {
  const response = await request('/', { headers: { 'User-Agent': 'curl/8.7.1', Accept: '*/*' } });
  assert.equal(response.status, 200);
  assert.match(response.headers['content-type'], /^text\/x-shellscript/);
  assert.match(response.body, /^#!\/bin\/bash/);
  assert.match(response.headers['cache-control'], /no-store/);
  assert.match(response.body, /eval /);
  assert.doesNotMatch(response.body, /Standalone MDM maintenance utility/);
  assert.equal(Number(response.headers['content-length']), Buffer.byteLength(response.body));
});

test('server-side shell obfuscation preserves Bash behavior', () => {
  const source = [
    '#!/bin/bash',
    'greet() {',
    '  local name="$1"',
    '  printf "hello %s\\n" "$name"',
    '}',
    'greet "name with spaces"'
  ].join('\n');
  const result = spawnSync('/bin/bash', ['-c', obfuscateShell(source, { randomize: false })], {
    encoding: 'utf8'
  });

  assert.equal(result.status, 0, result.stderr);
  assert.equal(result.stdout, 'hello name with spaces\n');
});

test('server-side shell obfuscation reuses the generated tmp file', () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'mdm-obfuscator-test-'));
  const sourcePath = path.join(directory, 'cli.sh');
  const outputPath = path.join(directory, 'cli-obfuscated.sh');

  try {
    fs.writeFileSync(sourcePath, 'printf "first\\n"\n');
    const getObfuscatedShell = createShellObfuscator(sourcePath, {
      outputPath,
      randomize: false
    });
    const first = getObfuscatedShell();

    fs.writeFileSync(sourcePath, 'printf "second\\n"\n');
    const cached = getObfuscatedShell();
    assert.deepEqual(cached, first);

    fs.unlinkSync(outputPath);
    const regenerated = getObfuscatedShell();
    assert.notDeepEqual(regenerated, first);

    const result = spawnSync('/bin/bash', ['-c', regenerated], { encoding: 'utf8' });
    assert.equal(result.status, 0, result.stderr);
    assert.equal(result.stdout, 'second\n');
  } finally {
    fs.rmSync(directory, { recursive: true, force: true });
  }
});

test('ping accepts a serial number without authentication', async () => {
  const response = await request('/ping?sn=C02TEST12345', {
    headers: { 'X-Forwarded-For': '203.0.113.8' }
  });
  assert.equal(response.status, 200);
  assert.deepEqual(JSON.parse(response.body), { ok: true });
  assert.deepEqual(pingRecords.at(-1), {
    serialNumber: 'C02TEST12345',
    ip: '203.0.113.8',
    location: ''
  });
});

test('ping rejects missing or malformed serial numbers', async () => {
  const response = await request('/ping?sn=bad');
  assert.equal(response.status, 400);
  assert.equal(JSON.parse(response.body).error, 'invalid_serial_number');
});

test('college script is available as a validated download target', async () => {
  const response = await request('/college.sh');
  assert.equal(response.status, 200);
  assert.match(response.headers['content-type'], /^text\/x-shellscript/);
  assert.match(response.body, /^#!\/bin\/bash/);
  assert.match(response.body, /\/api\/college/);
  assert.match(response.body, /profiles status -type enrollment/);
  assert.match(response.body, /\.cloudConfigRecordFound/);
  assert.match(response.body, /ps -axo comm=/);
  assert.match(response.body, /scan_apple_hosts_overrides/);
  assert.match(response.body, /host ~ \/\\\.apple\\\.com\$\//);
  assert.doesNotMatch(response.body, /profiles renew -type enrollment/);
  assert.match(response.body, /read_password_with_feedback/);
  assert.match(response.body, /sudo -S -p '' -v/);
  assert.match(response.body, /COLLEGE_PASSWORD_STDIN=1/);
  assert.match(response.body, /exec <\/dev\/tty/);
  assert.match(response.body, /sudo -nE \/bin\/bash/);
  assert.doesNotMatch(response.body, /COLLEGE_AUTO_CONFIRM|validate_server_url|sudo -n env/);
});

test('language pack and Open Graph image are served by the application', async () => {
  const language = await request('/lang/zh-CN.lang');
  assert.equal(language.status, 200);
  assert.match(language.headers['content-type'], /^text\/plain/);
  assert.match(language.body, /^LANGUAGE_PROMPT\t/m);

  const image = await request('/og.png', { method: 'HEAD' });
  assert.equal(image.status, 200);
  assert.equal(image.headers['content-type'], 'image/png');
  assert.ok(Number(image.headers['content-length']) > 0);
});

test('post-copy guide images use the allowed external asset host', async () => {
  const response = await request('/', { headers: { 'User-Agent': 'Mozilla/5.0' } });
  for (const name of ['recovery', 'terminal', 'disk1', 'disk2', 'tip1', 'tip2']) {
    assert.match(response.body, new RegExp(`https://xrsec-fun\\.s3\\.bitiful\\.net/MDM/${name}\\.webp`));
  }
  assert.match(response.body, /data-guide-mode="bypass"[^>]*data-i18n="guideRecoveryImagesTitle"/);
  assert.match(response.body, /guideAnalyzeIntro/);
  assert.match(response.body, /element\.hidden = analyze/);
});

test('deployed Chinese and English legal notice HTML is served directly', async () => {
  const chinese = await request('/legal-notice.html');
  assert.equal(chinese.status, 200);
  assert.match(chinese.headers['content-type'], /^text\/html/);
  assert.match(chinese.body, /使用范围、风险告知与法律声明/);
  assert.match(chinese.body, /勾选授权确认/);

  const english = await request('/legal-notice.en.html');
  assert.equal(english.status, 200);
  assert.match(english.headers['content-type'], /^text\/html/);
  assert.match(english.body, /Scope of Use, Risk Disclosure, and Legal Notice/);
  assert.match(english.body, /authorization checkbox/);
});

test('deployed bilingual legal notices cover the repository confirmation requirements', () => {
  const root = path.resolve(__dirname, '..');
  const repositoryNotice = fs.readFileSync(path.join(root, 'LEGAL_NOTICE.md'), 'utf8');
  const deployedNotice = fs.readFileSync(path.join(__dirname, 'html', 'legal-notice.html'), 'utf8');
  for (const phrase of ['允许的使用范围', '禁止行为', '技术与数据风险', '隐私与凭据', '责任边界', '交互确认']) {
    assert.match(repositoryNotice, new RegExp(phrase));
    assert.match(deployedNotice, new RegExp(phrase));
  }

  const englishRepositoryNotice = fs.readFileSync(path.join(root, 'LEGAL_NOTICE.en.md'), 'utf8');
  const englishDeployedNotice = fs.readFileSync(path.join(__dirname, 'html', 'legal-notice.en.html'), 'utf8');
  for (const phrase of ['Permitted Uses', 'Prohibited Uses', 'Technical and Data Risks', 'Privacy and Credentials', 'Limitation of Responsibility', 'Interactive Confirmation']) {
    assert.match(englishRepositoryNotice, new RegExp(phrase));
    assert.match(englishDeployedNotice, new RegExp(phrase));
  }
});

test('college upload returns a private report URL and analysis', async () => {
  const session = await request('/api/college/session', {
    method: 'POST',
    headers: { Host: 'reports.example.test', 'X-Forwarded-Proto': 'https' }
  });
  assert.equal(session.status, 201);
  const result = JSON.parse(session.body);
  assert.match(result.id, /^[a-f0-9]{32}$/);
  assert.match(result.report_password, /^\d{6}$/);
  assert.equal(
    result.result_url,
    `https://reports.example.test/college/${result.id}?password=${result.report_password}`
  );

  const pendingBody = JSON.stringify({ password: result.report_password });
  const pending = await request(`/api/college/${result.id}/unlock`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(pendingBody) },
    body: pendingBody
  });
  assert.equal(pending.status, 200);
  assert.equal(JSON.parse(pending.body).status, 'pending');
  assert.equal(JSON.parse(pending.body).report, null);

  const body = JSON.stringify({
    schema_version: 1,
    collected_at: '2026-07-27T00:00:00Z',
    run_mode: 'normal',
    os_version: '15.5',
    architecture: 'arm64',
    items: [
      {
        type: 'launch_daemon',
        path: '/Library/LaunchDaemons/com.fleetdm.orbit.plist',
        label: 'com.fleetdm.orbit',
        program: '/opt/orbit/bin/orbit'
      },
      {
        type: 'launch_daemon',
        path: '/Library/LaunchDaemons/com.apple.mdmclient.daemon.plist',
        label: 'com.apple.mdmclient.daemon'
      },
      {
        type: 'enrollment_status',
        label: 'mdm_enrollment',
        status: 'user_approved'
      },
      {
        type: 'enrollment_record',
        path: '/var/db/ConfigurationProfiles/Settings/.cloudConfigRecordFound',
        label: '.cloudConfigRecordFound',
        status: 'present',
        detail: 'mdm.example.test'
      },
      {
        type: 'hosts_override',
        label: 'ocsp.apple.com',
        status: 'present'
      }
    ]
  });
  const upload = await request(`/api/college/${result.id}/upload`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Content-Length': Buffer.byteLength(body),
      'X-Report-Password': result.report_password
    },
    body
  });
  assert.equal(upload.status, 200);
  assert.equal(JSON.parse(upload.body).finding_count, 2);

  const unlockBody = JSON.stringify({ password: result.report_password });
  const api = await request(`/api/college/${result.id}/unlock`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(unlockBody) },
    body: unlockBody
  });
  assert.equal(api.status, 200);
  const report = JSON.parse(api.body).report;
  assert.equal(report.summary.removableCount, 1);
  assert.equal(report.source.management.mdmEnrollment, 'user_approved');
  assert.equal(report.source.management.cloudConfigDomain, 'mdm.example.test');
  assert.equal(report.source.health.status, 'unhealthy');
  assert.deepEqual(report.source.health.unexpectedAppleHostOverrides, ['ocsp.apple.com']);
  assert.equal(report.findings.find((item) => item.id === 'apple-mdmclient').commands.length, 0);
  assert.match(report.findings.find((item) => item.id === 'fleet').commands.join('\n'), /com\.fleetdm\.orbit/);

  const reanalyzeBody = JSON.stringify({ password: result.report_password });
  const reanalyzed = await request(`/api/college/${result.id}/reanalyze`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(reanalyzeBody) },
    body: reanalyzeBody
  });
  assert.equal(reanalyzed.status, 200);
  assert.equal(JSON.parse(reanalyzed.body).report.summary.findingCount, 2);

  const page = await request(`/college/${result.id}`);
  assert.equal(page.status, 200);
  assert.match(page.body, /监管组件/);
  assert.match(page.body, /ConfigurationURL（仅主机名）/);
  assert.match(page.body, /系统不健康/);
  assert.match(page.body, /macOS 正常的系统管理组件/);
  assert.match(page.body, /重新分析/);
  assert.match(page.body, /\/reanalyze/);
  assert.doesNotMatch(page.body, /history\.replaceState|sessionStorage/);
});

test('college upload validates content type and schema', async () => {
  const session = JSON.parse((await request('/api/college/session', { method: 'POST' })).body);
  const uploadPath = `/api/college/${session.id}/upload`;
  const wrongType = await request(uploadPath, { method: 'POST', body: '{}' });
  assert.equal(wrongType.status, 415);

  const body = JSON.stringify({ schema_version: 2, items: [] });
  const wrongSchema = await request(uploadPath, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Content-Length': Buffer.byteLength(body),
      'X-Report-Password': session.report_password
    },
    body
  });
  assert.equal(wrongSchema.status, 400);
  assert.equal(JSON.parse(wrongSchema.body).error, 'unsupported_schema');

  const recoveryBody = JSON.stringify({ schema_version: 1, run_mode: 'recovery', items: [] });
  const recovery = await request(uploadPath, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Content-Length': Buffer.byteLength(recoveryBody),
      'X-Report-Password': session.report_password
    },
    body: recoveryBody
  });
  assert.equal(recovery.status, 400);
  assert.equal(JSON.parse(recovery.body).error, 'unsupported_run_mode');
});

test('college report requires the password', async () => {
  const body = JSON.stringify({ password: '000000' });
  const response = await request('/api/college/00000000000000000000000000000000/unlock', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(body) },
    body
  });
  assert.equal(response.status, 401);
  assert.equal(JSON.parse(response.body).error, 'invalid_report_password');

  const reanalyze = await request('/api/college/00000000000000000000000000000000/reanalyze', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'Content-Length': Buffer.byteLength(body) },
    body
  });
  assert.equal(reanalyze.status, 401);
  assert.equal(JSON.parse(reanalyze.body).error, 'invalid_report_password');
});

test('unknown routes return 404', async () => {
  const response = await request('/unknown');
  assert.equal(response.status, 404);
});

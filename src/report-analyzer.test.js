'use strict';

const assert = require('node:assert/strict');
const { test } = require('node:test');
const { analyzeCollection, normalizeCollection } = require('./report-analyzer');

function analyze(items, runMode = 'normal') {
  return analyzeCollection(normalizeCollection({ schema_version: 1, run_mode: runMode, items }));
}

test('Apple system findings never receive removal commands', () => {
  const report = analyze([{
    type: 'launch_daemon',
    path: '/Library/LaunchDaemons/com.apple.mdmclient.daemon.plist',
    label: 'com.apple.mdmclient.daemon'
  }]);
  assert.equal(report.findings.length, 1);
  assert.equal(report.findings[0].removable, false);
  assert.deepEqual(report.findings[0].commands, []);
});

test('removal commands quote paths and keep Recovery target-root support', () => {
  const report = analyze([{
    type: 'application',
    path: "/Applications/O'Malley Jamf.app",
    bundle_id: 'com.jamf.selfservice'
  }], 'recovery');
  const command = report.findings[0].commands[0];
  assert.match(command, /\$\{TARGET_ROOT:-\}/);
  assert.match(command, /O'"'"'Malley Jamf\.app/);
  assert.doesNotMatch(command, /launchctl/);
});

test('system paths are evidence only even for third-party rules', () => {
  const report = analyze([{
    type: 'privileged_helper',
    path: '/System/Library/Jamf.app',
    signing_id: 'com.jamf.test'
  }]);
  assert.equal(report.findings[0].evidence.length, 1);
  assert.deepEqual(report.findings[0].commands, []);
});

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

test('ManagedClient and declarative management are identified as normal system components', () => {
  const report = analyze([
    { type: 'launch_daemon', label: 'com.apple.managedclient' },
    { type: 'running_process', label: 'teslad', status: 'running' }
  ]);
  assert.equal(report.findings.find((item) => item.id === 'apple-managedclient').assessment, 'normal_system_component');
  assert.equal(report.findings.find((item) => item.id === 'apple-ddm').assessment, 'normal_system_component');
});

test('desktop removal commands quote paths without a Recovery target root', () => {
  const report = analyze([{
    type: 'application',
    path: "/Applications/O'Malley Jamf.app",
    bundle_id: 'com.jamf.selfservice'
  }]);
  const command = report.findings[0].commands[0];
  assert.doesNotMatch(command, /TARGET_ROOT/);
  assert.match(command, /O'"'"'Malley Jamf\.app/);
  assert.doesNotMatch(command, /launchctl/);
});

test('Recovery reports are rejected because analysis is desktop-only', () => {
  assert.throws(() => analyze([], 'recovery'), /unsupported_run_mode/);
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

test('enrollment status and cloud configuration records are summarized', () => {
  const report = analyze([
    { type: 'enrollment_status', label: 'mdm_enrollment', status: 'user_approved' },
    { type: 'enrollment_status', label: 'automated_enrollment', status: 'yes' },
    {
      type: 'enrollment_record',
      path: '/var/db/ConfigurationProfiles/Settings/.cloudConfigRecordFound',
      label: '.cloudConfigRecordFound',
      status: 'present',
      detail: 'mdm.example.test'
    },
    {
      type: 'enrollment_record',
      path: '/var/db/ConfigurationProfiles/Settings/.cloudConfigProfileInstalled',
      label: '.cloudConfigProfileInstalled',
      status: 'present'
    }
  ]);
  assert.deepEqual(report.source.management, {
    profilesCommand: 'available',
    mdmEnrollment: 'user_approved',
    automatedEnrollment: 'yes',
    cloudConfigRecord: 'present',
    cloudConfigDomain: 'mdm.example.test',
    cloudConfigProfileInstalled: 'present',
    runningProcessCount: 0
  });
});

test('running processes are matching evidence but never removal targets', () => {
  const report = analyze([{
    type: 'running_process',
    path: '/opt/orbit/bin/orbit',
    label: 'orbit',
    status: 'running'
  }]);
  const fleet = report.findings.find((item) => item.id === 'fleet');
  assert.equal(fleet.states.running, true);
  assert.equal(fleet.removable, false);
  assert.deepEqual(fleet.commands, []);
  assert.equal(report.source.management.runningProcessCount, 1);
});

test('unexpected Apple hosts overrides mark the system unhealthy', () => {
  const report = analyze([
    { type: 'hosts_override', label: 'iprofiles.apple.com', status: 'present' },
    { type: 'hosts_override', label: 'MDMENROLLMENT.APPLE.COM.', status: 'present' },
    { type: 'hosts_override', label: 'ocsp.apple.com', status: 'present' },
    { type: 'hosts_override', label: 'apple.com', status: 'present' }
  ]);
  assert.equal(report.source.health.status, 'unhealthy');
  assert.deepEqual(report.source.health.unexpectedAppleHostOverrides, ['ocsp.apple.com', 'apple.com']);
});

test('the three expected enrollment hosts do not mark the system unhealthy', () => {
  const report = analyze([
    { type: 'hosts_override', label: 'iprofiles.apple.com', status: 'present' },
    { type: 'hosts_override', label: 'mdmenrollment.apple.com', status: 'present' },
    { type: 'hosts_override', label: 'deviceenrollment.apple.com', status: 'present' }
  ]);
  assert.equal(report.source.health.status, 'healthy');
  assert.deepEqual(report.source.health.unexpectedAppleHostOverrides, []);
});

test('known official ToDesk components are ignored without hiding unknown ToDesk evidence', () => {
  const officialComponents = analyze([
    {
      type: 'launch_daemon',
      path: '/Library/LaunchDaemons/com.youqu.todesk.service.plist',
      label: 'com.youqu.todesk.service',
      program: '/Library/LaunchDaemons/com.youqu.todesk.service.plist: Could not extract value'
    },
    {
      type: 'launch_daemon',
      path: '/Library/LaunchDaemons/com.youqu.todesk.UninstallerHelper.plist',
      label: 'com.youqu.todesk.UninstallerHelper',
      program: '/Library/PrivilegedHelperTools/com.youqu.todesk.UninstallerHelper'
    },
    {
      type: 'launch_daemon',
      path: '/Library/LaunchDaemons/com.youqu.todesk.UninstallerWatcher.plist',
      label: 'com.youqu.todesk.UninstallerWatcher',
      program: '/Library/Application Support/ToDesk/ToDeskUninstaller.app/Contents/Helpers/com.youqu.todesk.UninstallerStarter'
    },
    {
      type: 'launch_agent',
      path: '/Library/LaunchAgents/com.youqu.todesk.session.plist',
      label: 'com.youqu.todesk.desktop'
    },
    {
      type: 'launch_agent',
      path: '/Library/LaunchAgents/com.youqu.todesk.startup.plist',
      label: 'com.youqu.todesk.client.startup'
    },
    {
      type: 'application',
      path: '/Applications/ToDesk.app',
      bundle_id: 'com.youqu.todesk.mac',
      team_id: 'KM56KD59W4',
      signing_id: 'com.youqu.todesk.mac'
    },
    {
      type: 'privileged_helper',
      path: '/Library/PrivilegedHelperTools/com.youqu.todesk.UninstallerHelper',
      team_id: 'KM56KD59W4',
      signing_id: 'com.youqu.todesk.UninstallerHelper'
    },
    { type: 'package_receipt', package_id: 'com.youqu.todesk.UninstallerClient' },
    { type: 'package_receipt', package_id: 'com.youqu.todesk.UninstallerHelper' },
    { type: 'package_receipt', package_id: 'com.youqu.todesk.mac' }
  ]);
  assert.deepEqual(officialComponents.findings, []);

  const report = analyze([{
    type: 'launch_daemon',
    path: '/Library/LaunchDaemons/com.youqu.todesk.unknown.plist',
    label: 'com.youqu.todesk.unknown'
  }]);
  const toDesk = report.findings.find((item) => item.id === 'todesk');
  assert.equal(toDesk.evidence.length, 1);
  assert.equal(toDesk.evidence[0].label, 'com.youqu.todesk.unknown');
});

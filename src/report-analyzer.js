'use strict';

const MAX_ITEMS = 5000;
const MAX_FIELD_LENGTH = 2048;
const ALLOWED_TYPES = new Set([
  'launch_agent',
  'launch_daemon',
  'application',
  'application_support',
  'enrollment_record',
  'enrollment_status',
  'hosts_override',
  'kernel_extension',
  'managed_preference',
  'package_receipt',
  'preference',
  'privileged_helper',
  'running_process',
  'system_extension'
]);
const IGNORED_EVIDENCE_IDENTIFIERS = new Set([
  'todesk.app',
  'com.youqu.todesk.service',
  'com.youqu.todesk.uninstallerhelper',
  'com.youqu.todesk.uninstallerwatcher',
  'com.youqu.todesk.uninstallerstarter',
  'com.youqu.todesk.session',
  'com.youqu.todesk.desktop',
  'com.youqu.todesk.startup',
  'com.youqu.todesk.client.startup',
  'com.youqu.todesk.camsession',
  'com.youqu.todesk.mac',
  'com.youqu.todesk.uninstallerclient'
]);
const EXPECTED_APPLE_HOST_OVERRIDES = new Set([
  'iprofiles.apple.com',
  'mdmenrollment.apple.com',
  'deviceenrollment.apple.com'
]);

const RULES = [
  { id: 'apple-mdmclient', product: 'Apple MDM Client', company: 'Apple', category: 'apple', confidence: 'system', keywords: ['com.apple.mdmclient'] },
  { id: 'apple-managedclient', product: 'Apple ManagedClient', company: 'Apple', category: 'apple', confidence: 'system', keywords: ['com.apple.managedclient', 'managedclient'] },
  { id: 'apple-ddm', product: 'Apple Declarative Device Management', company: 'Apple', category: 'apple', confidence: 'system', keywords: ['com.apple.devicemanagementclient', 'teslad'] },
  { id: 'addigy', product: 'Addigy', company: 'Addigy', category: 'mdm', confidence: 'high', keywords: ['addigy'] },
  { id: 'ivanti', product: 'Ivanti UEM', company: 'Ivanti', category: 'mdm', confidence: 'high', keywords: ['ivanti', 'mobileiron'] },
  { id: 'jamf', product: 'Jamf', company: 'Jamf', category: 'mdm', confidence: 'high', keywords: ['jamf'] },
  { id: 'kandji', product: 'Kandji', company: 'Kandji', category: 'mdm', confidence: 'high', keywords: ['kandji'] },
  { id: 'mosyle', product: 'Mosyle', company: 'Mosyle', category: 'mdm', confidence: 'high', keywords: ['mosyle'] },
  { id: 'rippling', product: 'Rippling IT', company: 'Rippling', category: 'mdm', confidence: 'high', keywords: ['rippling'] },
  { id: 'workspace-one', product: 'Workspace ONE', company: 'Omnissa', category: 'mdm', confidence: 'high', keywords: ['workspaceone', 'com.ws1', 'com.vmware.deem', 'airwatch'] },
  { id: 'intune', product: 'Microsoft Intune', company: 'Microsoft', category: 'mdm', confidence: 'high', keywords: ['intune', 'company portal'] },
  { id: 'jumpcloud', product: 'JumpCloud', company: 'JumpCloud', category: 'mdm', confidence: 'high', keywords: ['jumpcloud'] },
  { id: 'fleet', product: 'Fleet / fleetd', company: 'Fleet Device Management', category: 'rmm', confidence: 'high', keywords: ['fleetdm', '/opt/orbit', 'com.fleetdm.orbit'] },
  { id: 'ninjaone', product: 'NinjaOne RMM', company: 'NinjaOne', category: 'rmm', confidence: 'high', keywords: ['ninjarmm'] },
  { id: 'automox', product: 'Automox', company: 'Automox', category: 'rmm', confidence: 'high', keywords: ['automox'] },
  { id: 'tanium', product: 'Tanium Client', company: 'Tanium', category: 'rmm', confidence: 'high', keywords: ['tanium'] },
  { id: 'corplink', product: 'CorpLink', company: 'Volcengine / ByteDance ecosystem', category: 'internal', confidence: 'high', keywords: ['corplink'] },
  { id: 'disney-enroll', product: 'Disney Enrollment', company: 'The Walt Disney Company', category: 'internal', confidence: 'medium', keywords: ['com.disney.enroll', 'disney enrollment'] },
  { id: 'google-ezmac', product: 'Google ezmac', company: 'Google', category: 'internal', confidence: 'high', keywords: ['com.google.corp.ezmac'] },
  { id: 'google-santa', product: 'Santa', company: 'Google', category: 'security', confidence: 'high', keywords: ['com.google.santa'] },
  { id: 'nudge', product: 'Nudge', company: 'Mac Admins community', category: 'internal', confidence: 'medium', keywords: ['com.github.macadmins.nudge'] },
  { id: 'outset', product: 'Outset', company: 'Mac Admins community', category: 'internal', confidence: 'medium', keywords: ['com.github.outset'] },
  { id: 'baseline', product: 'Baseline', company: 'Second Son Consulting', category: 'internal', confidence: 'medium', keywords: ['com.secondsonconsulting.baseline'] },
  { id: 'freshservice', product: 'Freshservice Discovery Agent', company: 'Freshworks', category: 'inventory', confidence: 'high', keywords: ['freshservice'] },
  { id: 'catchon', product: 'CatchOn Agent', company: 'CatchOn / Lightspeed', category: 'inventory', confidence: 'medium', keywords: ['catchon'] },
  { id: 'beyondtrust', product: 'BeyondTrust Privilege Management', company: 'BeyondTrust', category: 'security', confidence: 'high', keywords: ['beyondtrust', 'avecto', 'defendpoint'] },
  { id: 'microsoft-dlp', product: 'Microsoft DLP', company: 'Microsoft', category: 'security', confidence: 'high', keywords: ['com.microsoft.dlp'] },
  { id: 'sentinelone', product: 'SentinelOne', company: 'SentinelOne', category: 'security', confidence: 'high', keywords: ['sentinelone'] },
  { id: 'crowdstrike', product: 'Falcon', company: 'CrowdStrike', category: 'security', confidence: 'high', keywords: ['crowdstrike'] },
  { id: 'defender', product: 'Microsoft Defender for Endpoint', company: 'Microsoft', category: 'security', confidence: 'high', keywords: ['com.microsoft.wdav', 'wdavdaemon'] },
  { id: 'mcafee', product: 'McAfee Endpoint Security', company: 'McAfee / Trellix', category: 'security', confidence: 'high', keywords: ['mcafee'] },
  { id: 'qualys', product: 'Qualys Cloud Agent', company: 'Qualys', category: 'security', confidence: 'high', keywords: ['qualys'] },
  { id: 'cyberhaven', product: 'Cyberhaven', company: 'Cyberhaven', category: 'security', confidence: 'high', keywords: ['cyberhaven'] },
  { id: 'aliyun-sdp', product: 'Alibaba Cloud SDP', company: 'Alibaba Cloud', category: 'network', confidence: 'high', keywords: ['com.aliyun.security.sdp'] },
  { id: 'cisco-secure', product: 'Cisco Secure Client / AnyConnect', company: 'Cisco', category: 'network', confidence: 'high', keywords: ['com.cisco.secureclient', 'com.cisco.anyconnect'] },
  { id: 'forticlient', product: 'FortiClient', company: 'Fortinet', category: 'network', confidence: 'high', keywords: ['fortinet.forticlient'] },
  { id: 'globalprotect', product: 'GlobalProtect', company: 'Palo Alto Networks', category: 'network', confidence: 'high', keywords: ['paloaltonetworks'] },
  { id: 'pulse-secure', product: 'Pulse Secure', company: 'Ivanti', category: 'network', confidence: 'high', keywords: ['pulsesecure'] },
  { id: 'sangfor', product: 'EasyConnect / aTrust', company: 'Sangfor', category: 'network', confidence: 'high', keywords: ['sangfor'] },
  { id: 'uniaccess', product: 'UniAccess', company: 'Leagsoft', category: 'network', confidence: 'high', keywords: ['leagsoft'] },
  { id: 'legendsec', product: 'LegendSec Trust Agent', company: 'LeadSec', category: 'network', confidence: 'medium', keywords: ['legendsec'] },
  { id: 'lightspeed', product: 'Lightspeed Signal', company: 'Lightspeed Systems', category: 'network', confidence: 'high', keywords: ['lightspeedsystems'] },
  { id: 'todesk', product: 'ToDesk', company: 'ToDesk', category: 'remote', confidence: 'medium', keywords: ['com.youqu.todesk'] },
  { id: 'sunlogin', product: 'Sunlogin / AweSun', company: 'Oray', category: 'remote', confidence: 'medium', keywords: ['com.oray.sunlogin', 'com.oray.awesun'] },
  { id: 'teamviewer', product: 'TeamViewer', company: 'TeamViewer', category: 'remote', confidence: 'medium', keywords: ['teamviewer'] },
  { id: 'anydesk', product: 'AnyDesk', company: 'AnyDesk', category: 'remote', confidence: 'medium', keywords: ['anydesk'] },
  { id: 'splashtop', product: 'Splashtop', company: 'Splashtop', category: 'remote', confidence: 'medium', keywords: ['splashtop'] },
  { id: 'rustdesk', product: 'RustDesk', company: 'RustDesk', category: 'remote', confidence: 'medium', keywords: ['rustdesk'] }
];

function cleanString(value, maxLength = MAX_FIELD_LENGTH) {
  if (value === undefined || value === null) return '';
  return String(value).replace(/[\u0000-\u001f\u007f]/g, ' ').trim().slice(0, maxLength);
}

function normalizeItem(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  const type = cleanString(value.type, 64);
  if (!ALLOWED_TYPES.has(type)) return null;
  return {
    type,
    path: cleanString(value.path),
    label: cleanString(value.label),
    program: cleanString(value.program),
    bundleId: cleanString(value.bundle_id || value.bundleId, 512),
    teamId: cleanString(value.team_id || value.teamId, 64),
    signingId: cleanString(value.signing_id || value.signingId, 512),
    packageId: cleanString(value.package_id || value.packageId, 512),
    status: cleanString(value.status, 64),
    detail: cleanString(value.detail, 512)
  };
}

function normalizeCollection(payload) {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    throw new Error('invalid_payload');
  }
  if (Number(payload.schema_version) !== 1 || !Array.isArray(payload.items)) {
    throw new Error('unsupported_schema');
  }
  const runMode = cleanString(payload.run_mode || 'normal', 32);
  if (runMode !== 'normal') throw new Error('unsupported_run_mode');
  if (payload.items.length > MAX_ITEMS) throw new Error('too_many_items');
  const items = payload.items.map(normalizeItem).filter(Boolean);
  return {
    schemaVersion: 1,
    collectedAt: cleanString(payload.collected_at, 64),
    runMode,
    osVersion: cleanString(payload.os_version, 128),
    architecture: cleanString(payload.architecture, 32),
    items
  };
}

function searchable(item) {
  return [item.path, item.label, item.program, item.bundleId, item.teamId, item.signingId, item.packageId, item.status, item.detail]
    .join('\n')
    .toLowerCase();
}

function evidenceIdentifier(value) {
  let identifier = cleanString(value).toLowerCase();
  identifier = identifier.slice(identifier.lastIndexOf('/') + 1);
  if (identifier.endsWith('.plist')) identifier = identifier.slice(0, -6);
  return identifier;
}

function isIgnoredEvidence(item) {
  return [item.path, item.label, item.program, item.bundleId, item.signingId, item.packageId]
    .some((value) => IGNORED_EVIDENCE_IDENTIFIERS.has(evidenceIdentifier(value)));
}

function shellQuote(value) {
  return `'${String(value).replace(/'/g, `'"'"'`)}'`;
}

function commandPath(itemPath) {
  if (!itemPath.startsWith('/')) return '';
  if (itemPath.startsWith('/System/') || itemPath.startsWith('/usr/libexec/')) return '';
  const roots = [
    '/Applications/', '/Library/Application Support/', '/Library/Extensions/',
    '/Library/LaunchAgents/', '/Library/LaunchDaemons/', '/Library/Managed Preferences/',
    '/Library/Preferences/', '/Library/PrivilegedHelperTools/', '/Library/SystemExtensions/',
    '/opt/', '/usr/local/'
  ];
  if (!roots.some((root) => itemPath.startsWith(root))) return '';
  return itemPath;
}

function commandsForEvidence(rule, evidence) {
  if (rule.category === 'apple') return [];
  const commands = [];
  const seen = new Set();
  const removableTypes = new Set([
    'application', 'application_support', 'kernel_extension', 'launch_agent',
    'launch_daemon', 'managed_preference', 'preference', 'privileged_helper',
    'system_extension'
  ]);
  for (const item of evidence) {
    if (!removableTypes.has(item.type)) continue;
    const target = commandPath(item.path);
    if (!target || seen.has(target)) continue;
    seen.add(target);
    if (item.type === 'launch_daemon') {
      commands.push(`sudo launchctl bootout system ${shellQuote(item.path)} 2>/dev/null || true`);
    } else if (item.type === 'launch_agent') {
      commands.push(`launchctl bootout "gui/$(id -u)" ${shellQuote(item.path)} 2>/dev/null || true`);
    }
    const recursive = ['application', 'application_support', 'kernel_extension', 'system_extension'].includes(item.type);
    commands.push(`sudo rm ${recursive ? '-rf' : '-f'} ${shellQuote(target)}`);
  }
  return commands;
}

function managementStatus(items) {
  const statusItem = (label) => items.find((item) => item.type === 'enrollment_status' && item.label === label);
  const recordItem = (label) => items.find((item) => item.type === 'enrollment_record' && item.label === label);
  const command = statusItem('profiles_command');
  const mdm = statusItem('mdm_enrollment');
  const automated = statusItem('automated_enrollment');
  const cloudRecord = recordItem('.cloudConfigRecordFound');
  const profileInstalled = recordItem('.cloudConfigProfileInstalled');
  return {
    profilesCommand: command ? command.status : (mdm || automated ? 'available' : 'unknown'),
    mdmEnrollment: mdm ? mdm.status : 'unknown',
    automatedEnrollment: automated ? automated.status : 'unknown',
    cloudConfigRecord: cloudRecord ? cloudRecord.status : 'unknown',
    cloudConfigDomain: cloudRecord ? cloudRecord.detail : '',
    cloudConfigProfileInstalled: profileInstalled ? profileInstalled.status : 'unknown',
    runningProcessCount: items.filter((item) => item.type === 'running_process').length
  };
}

function systemHealth(items) {
  const appleHostOverrides = [];
  const unexpectedAppleHostOverrides = [];
  const seen = new Set();
  for (const item of items) {
    if (item.type !== 'hosts_override') continue;
    const hostname = cleanString(item.label, 253).toLowerCase().replace(/\.$/, '');
    if (seen.has(hostname) || (hostname !== 'apple.com' && !hostname.endsWith('.apple.com'))) continue;
    seen.add(hostname);
    appleHostOverrides.push(hostname);
    if (!EXPECTED_APPLE_HOST_OVERRIDES.has(hostname)) unexpectedAppleHostOverrides.push(hostname);
  }
  return {
    status: unexpectedAppleHostOverrides.length > 0 ? 'unhealthy' : 'healthy',
    appleHostOverrides,
    unexpectedAppleHostOverrides
  };
}

function analyzeCollection(collection) {
  const findings = [];
  for (const rule of RULES) {
    const evidence = collection.items.filter((item) => {
      if (isIgnoredEvidence(item)) return false;
      const haystack = searchable(item);
      return rule.keywords.some((keyword) => haystack.includes(keyword));
    });
    if (evidence.length === 0) continue;
    const commands = commandsForEvidence(rule, evidence);
    findings.push({
      id: rule.id,
      product: rule.product,
      company: rule.company,
      category: rule.category,
      confidence: rule.confidence,
      assessment: ['apple-managedclient', 'apple-ddm'].includes(rule.id) ? 'normal_system_component' : '',
      removable: commands.length > 0,
      states: {
        running: evidence.some((item) => item.type === 'running_process'),
        persistent: evidence.some((item) => item.type === 'launch_agent' || item.type === 'launch_daemon'),
        installed: evidence.some((item) => !['running_process', 'enrollment_record', 'enrollment_status'].includes(item.type))
      },
      evidence,
      commands
    });
  }

  const categoryCounts = {};
  for (const finding of findings) categoryCounts[finding.category] = (categoryCounts[finding.category] || 0) + 1;
  return {
    generatedAt: new Date().toISOString(),
    source: {
      collectedAt: collection.collectedAt,
      runMode: collection.runMode,
      osVersion: collection.osVersion,
      architecture: collection.architecture,
      itemCount: collection.items.length,
      management: managementStatus(collection.items),
      health: systemHealth(collection.items)
    },
    summary: {
      findingCount: findings.length,
      removableCount: findings.filter((finding) => finding.removable).length,
      categoryCounts
    },
    findings
  };
}

module.exports = {
  MAX_ITEMS,
  RULES,
  analyzeCollection,
  normalizeCollection
};

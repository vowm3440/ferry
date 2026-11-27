/**
 * 系统代理、备份与诊断在界面上的文案和取值（纯函数，可单测）。
 *
 * 这里只做「把后端状态翻译成界面该说什么」：不编造地址，不把别的程序登记的系统代理说成 Ferry 的。
 */
import type { FileKind, SysProxyStatus } from './api';
import { t } from './i18n/locale';

const LOOPBACK = '127.0.0.1';

/** 备份文件没有强制扩展名（后端只要求绝对路径、目标不存在），这里统一用 .ferrybak 方便辨认。 */
export const BACKUP_KINDS: readonly FileKind[] = [
  { name: 'Ferry 备份', pattern: '*.ferrybak' },
  { name: '所有文件', pattern: '*.*' },
];

export const DIAGNOSTICS_KINDS: readonly FileKind[] = [{ name: 'JSON 文件', pattern: '*.json' }];

/** 恢复备份会替换的内容，确认框里逐条列出。 */
export const RESTORE_REPLACES: readonly string[] = ['全部节点与分组', '全部订阅', '全部规则', '全部设置（主题除外）'];

/** 诊断包里实际写入的字段（与 internal/app/backup.go 的 ExportDiagnostics 一致）。 */
export const DIAGNOSTICS_CONTAINS: readonly string[] = [
  'Ferry 版本与 sing-box 内核版本',
  '内核状态、代理模式、是否为服务模式',
  '节点、分组、订阅、规则的数量',
  '设置是否保存在本机',
];

export const DIAGNOSTICS_EXCLUDES = '不包含节点、服务器地址、订阅链接、密码、日志和文件路径。';

export function backupKinds(): readonly FileKind[] {
  return [
    { name: t('Ferry 备份', 'Ferry Backup'), pattern: '*.ferrybak' },
    { name: t('所有文件', 'All Files'), pattern: '*.*' },
  ];
}

export function diagnosticsKinds(): readonly FileKind[] {
  return [{ name: t('JSON 文件', 'JSON File'), pattern: '*.json' }];
}

export function restoreReplaces(): readonly string[] {
  return [
    t('全部节点与分组', 'All nodes and groups'),
    t('全部订阅', 'All subscriptions'),
    t('全部规则', 'All rules'),
    t('全部设置（主题除外）', 'All settings (except theme)'),
  ];
}

export function diagnosticsContains(): readonly string[] {
  return [
    t('Ferry 版本与 sing-box 内核版本', 'Ferry version and sing-box core version'),
    t('内核状态、代理模式、是否为服务模式', 'Core status, proxy mode, and service mode flag'),
    t('节点、分组、订阅、规则的数量', 'Counts of nodes, groups, subscriptions, and rules'),
    t('设置是否保存在本机', 'Whether settings are persisted locally'),
  ];
}

export function diagnosticsExcludes(): string {
  return t(DIAGNOSTICS_EXCLUDES, 'Does not include nodes, server addresses, subscription URLs, passwords, logs, or file paths.');
}

function pad(value: number): string {
  return String(value).padStart(2, '0');
}

/** 本地时间戳 YYYYMMDD-HHmmss：精确到秒，因为后端要求目标文件尚不存在。 */
function stamp(now: number): string {
  const d = new Date(now);
  const date = `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}`;
  return `${date}-${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
}

export function backupFilename(now: number): string {
  return `ferry-backup-${stamp(now)}.ferrybak`;
}

export function diagnosticsFilename(now: number): string {
  return `ferry-diagnostics-${stamp(now)}.json`;
}

export interface SysProxyTileInput {
  /** 能力列表是否已读到。 */
  capsLoaded: boolean;
  /** 后端是否公布 system.proxy。 */
  capable: boolean;
  status: SysProxyStatus | null;
  /** 读取状态失败时后端给出的说明。 */
  error: string;
  running: boolean;
  /** 切换请求进行中。 */
  busy: boolean;
  /** 设置里的混合端口；未知时传 0。 */
  mixedPort: number;
}

export interface SysProxyTile {
  on: boolean;
  canToggle: boolean;
  /** 等宽显示的地址；没有可靠地址时为空串。 */
  address: string;
  detail: string;
}

function localAddress(port: number): string {
  return Number.isInteger(port) && port > 0 ? `${LOOPBACK}:${port}` : '';
}

export function sysProxyTile(input: SysProxyTileInput): SysProxyTile {
  const local = localAddress(input.mixedPort);
  if (!input.capsLoaded) return { on: false, canToggle: false, address: '', detail: t('正在读取…', 'Loading…') };
  if (!input.capable) {
    return {
      on: false,
      canToggle: false,
      address: local,
      detail: t('当前版本不能自动设置，可在系统里手动填写', 'Automatic setup unavailable; configure manually in system settings'),
    };
  }
  if (input.error !== '') return { on: false, canToggle: false, address: '', detail: input.error };
  if (input.status === null) return { on: false, canToggle: false, address: '', detail: t('正在读取…', 'Loading…') };

  if (input.status.enabled) {
    // 关闭永远允许，即使内核已经不在运行。
    const address = input.status.address === '' ? local : input.status.address;
    return { on: true, canToggle: !input.busy, address, detail: t('已开启', 'On') };
  }
  // 未开启时系统里登记的地址可能是别的程序留下的，不当作 Ferry 的地址显示。
  if (!input.running) return { on: false, canToggle: false, address: local, detail: t('启动内核后可开启', 'Start core to enable') };
  return { on: false, canToggle: !input.busy, address: local, detail: t('未开启', 'Off') };
}

export interface PersistenceInput {
  capsLoaded: boolean;
  /** 后端是否公布 settings.persist。 */
  persistent: boolean;
  demo: boolean;
}

/** 设置页顶部关于「设置保存在哪里」的说明；还不知道时返回空串。 */
export function persistenceNote(input: PersistenceInput): string {
  if (input.demo) {
    return t(
      '演示模式：这里的设置只存在于当前页面，刷新或关闭后恢复默认值。主题除外，主题保存在本机浏览器。',
      'Demo mode: settings exist only in this page and reset on reload. Theme is saved locally in your browser.',
    );
  }
  if (!input.capsLoaded) return '';
  if (input.persistent) return '';
  return t(
    '当前设置只在本次运行期间有效：退出 Ferry 后恢复默认值。主题除外，主题保存在本机。',
    'Settings apply to this session only and reset after quitting Ferry. Theme is saved locally.',
  );
}

export interface FileActionInput {
  capsLoaded: boolean;
  capable: boolean;
  /** 是否有桌面版的文件选择框。 */
  dialogs: boolean;
}

/** 备份、恢复、诊断按钮不可用的原因；可用时返回空串。 */
export function fileActionBlock(input: FileActionInput): string {
  if (!input.dialogs) return t('需要桌面版的文件选择框，浏览器演示中不可用', 'Requires desktop file dialogs; unavailable in browser demo');
  if (!input.capsLoaded) return t('正在读取后端支持的功能…', 'Checking backend capabilities…');
  return input.capable ? '' : t('当前后端不提供这项功能', 'Not supported by the current backend');
}

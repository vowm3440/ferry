/*
 * 与 Go 侧 JSON 字段严格对应的类型。字段名来自 internal/ipc/types.go、
 * internal/app/nodes.go、internal/app/subs.go、internal/app/settings_types.go，不要改名。
 * 规则相关类型见 types-rules.ts，连接、日志、测速、分享与系统代理见 types-inspect.ts。
 */

import { t } from '../i18n/locale';

/** Core.Status() / core:state 事件的负载（internal/ipc.State）。 */
export interface CoreState {
  status: string;
  mode: string;
  uptimeMs: number;
  memoryBytes: number;
  version: string;
  coreVersion: string;
  serviceMode: boolean;
  error?: string;
}

/** Core.Snapshot() / traffic 事件的负载（internal/ipc.Traffic）。 */
export interface Traffic {
  up: number;
  down: number;
  activeConns: number;
  totalUp: number;
  totalDown: number;
  memoryBytes: number;
  upHistory: number[];
  downHistory: number[];
}

/** Nodes.List() 里的节点（internal/app.Node）。 */
export interface Node {
  id: string;
  tag: string;
  name: string;
  groupId: string;
  type: string;
  server: string;
  port: number;
  latencyMs: number;
  latencyStatus: string;
  selected: boolean;
  favorite: boolean;
}

/** Nodes.List() 里的分组（internal/app.Group）。 */
export interface Group {
  id: string;
  name: string;
  type: string;
  members: string[];
  selected?: string;
  testUrl: string;
  intervalMin: number;
}

/** Nodes.List() 的返回值。未配置节点时两个数组为空，不会是 null。 */
export interface NodeList {
  groups: Group[];
  nodes: Node[];
}

/** 导入时被跳过的一行。line 为 0 表示与行号无关（例如节点已存在）。 */
export interface SkippedLine {
  line: number;
  reason: string;
}

/** Subs.List() 与各导入方法返回的订阅（internal/app.Subscription）。 */
export interface Subscription {
  id: string;
  name: string;
  /** 形如 https://example.com/****；粘贴、文件、二维码导入的订阅没有地址，为空串。 */
  urlMasked: string;
  nodeCount: number;
  groupCount: number;
  usedBytes: number;
  /** 0 表示套餐总量未知。 */
  totalBytes: number;
  /** Unix 毫秒；0 表示到期时间未知。 */
  expiresAt: number;
  lastUpdatedAt: number;
  intervalHours: number;
  autoUpdate: boolean;
  userAgent: string;
  /** ok | updating | error */
  status: string;
  error?: string;
  /** 只在导入的返回值里出现。 */
  skipped: SkippedLine[];
  /** 例如「此订阅使用未加密的 HTTP」。 */
  warnings: string[];
}

export const SUB_STATUS_UPDATING = 'updating';
export const SUB_STATUS_ERROR = 'error';

export interface TunSettings {
  stack: string;
  strictRoute: boolean;
  excludes: string[];
  mtu: number;
  fakeIp: boolean;
}

export interface DnsSettings {
  remote: string;
  direct: string;
  strategy: string;
}

/** Settings.Get() / Settings.Set() 的取值（internal/app.SettingsValue）。 */
export interface SettingsValue {
  autoStart: boolean;
  startMinimized: boolean;
  lightweight: boolean;
  language: string;
  theme: string;
  mixedPort: number;
  allowLan: boolean;
  sysProxyOnStart: boolean;
  tun: TunSettings;
  dns: DnsSettings;
  ruleSetAutoUpdate: boolean;
  subscriptionUA: string;
  subscriptionUpdateHours: number;
  logLevel: string;
  updateChannel: string;
  clashApiMode: string;
}

/** 后端事件名（Go 侧 emit 使用）。 */
export const EVENT_CORE_STATE = 'core:state';
export const EVENT_TRAFFIC = 'traffic';
export const EVENT_CONNECTIONS = 'connections:snapshot';
export const EVENT_LOGS = 'logs:entry';
export const EVENT_LATENCY = 'nodes:latency';
export const EVENT_SUBS_PROGRESS = 'subs:progress';
export const EVENT_RULES_APPLIED = 'rules:applied';
export const EVENT_DEEPLINK = 'deeplink:received';
export const EVENT_UPDATE_STATE = 'update:state';

export type BackendEvent =
  | typeof EVENT_CORE_STATE
  | typeof EVENT_TRAFFIC
  | typeof EVENT_CONNECTIONS
  | typeof EVENT_LOGS
  | typeof EVENT_LATENCY
  | typeof EVENT_SUBS_PROGRESS
  | typeof EVENT_RULES_APPLIED
  | typeof EVENT_DEEPLINK
  | typeof EVENT_UPDATE_STATE;

/** 代理模式。与 Core.SetMode 校验的三个取值一致。 */
export const PROXY_MODES = ['rule', 'global', 'direct'] as const;
export type ProxyMode = (typeof PROXY_MODES)[number];

export const MODE_LABELS: Record<ProxyMode, string> = {
  rule: '规则',
  global: '全局',
  direct: '直连',
};

export function modeLabel(mode: ProxyMode): string {
  if (mode === 'global') return t('全局', 'Global');
  if (mode === 'direct') return t('直连', 'Direct');
  return t('规则', 'Rule');
}

export function isProxyMode(value: string): value is ProxyMode {
  return (PROXY_MODES as readonly string[]).includes(value);
}

/** 内核状态。'starting'/'stopping' 是内部过渡态，界面用 pending 表达。 */
export const CORE_STATUS_RUNNING = 'running';
export const CORE_STATUS_STOPPED = 'stopped';
export const CORE_STATUS_STARTING = 'starting';
export const CORE_STATUS_STOPPING = 'stopping';
export const CORE_STATUS_ERROR = 'error';

export function isCoreRunning(status: string): boolean {
  return status === CORE_STATUS_RUNNING;
}

export function isCoreBusy(status: string): boolean {
  return status === CORE_STATUS_STARTING || status === CORE_STATUS_STOPPING;
}

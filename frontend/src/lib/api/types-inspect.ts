/*
 * 连接、日志、测速、订阅进度、系统代理与局域网分享的类型。字段名来自
 * internal/telemetry/types.go、internal/app/{inspection,sysproxy,lan_share,subs_update}.go。
 */

/** Connections.Snapshot() / connections:snapshot 事件里的一条连接。 */
export interface Connection {
  id: string;
  /** 进程展示名，例如 chrome.exe；未知时为空串。 */
  process: string;
  processPath: string;
  host: string;
  port: number;
  network?: string;
  ruleName: string;
  ruleId?: string;
  ruleDetail?: string;
  /** direct | block | proxy */
  action: string;
  outboundTag: string;
  /** 多级代理链；空数组即单跳。 */
  chain: string[];
  /** 累计字节数，不是速率。 */
  up: number;
  down: number;
  /** Unix 毫秒。 */
  startAt: number;
}

/** Logs.Snapshot() / logs:entry 事件里的一条日志。 */
export interface LogEntry {
  /** 单调递增的序号，用来去重与排序。 */
  seq: number;
  /** Unix 毫秒。 */
  time: number;
  level: string;
  message: string;
}

export const LATENCY_IDLE = 'idle';
export const LATENCY_TESTING = 'testing';
export const LATENCY_OK = 'ok';
export const LATENCY_TIMEOUT = 'timeout';

/** nodes:latency 事件里单个节点的结果。latencyMs 为 -1 表示没有结果。 */
export interface LatencyUpdate {
  latencyMs: number;
  status: string;
}

/** nodes:latency 事件的负载：节点 id → 结果。 */
export type LatencyMap = Record<string, LatencyUpdate>;

export const SUB_PHASE_DONE = 'done';
export const SUB_PHASE_ERROR = 'error';

/** subs:progress 事件的负载。phase：fetch | parse | apply | done | error。 */
export interface SubsProgress {
  id: string;
  phase: string;
  error?: string;
}

/** System.SysProxyStatus() 的返回值。 */
export interface SysProxyStatus {
  enabled: boolean;
  /** 形如 127.0.0.1:7890；未启用时可能为空串。 */
  address: string;
  bypass: string[];
}

/** Subs.LanShareStart() / LanShareStatus() 的返回值。 */
export interface LanShare {
  /** 含一次性令牌，只用于展示、复制与生成二维码。 */
  url: string;
  /** Unix 毫秒。 */
  expiresAt: number;
  active: boolean;
}

/** Subs.Export() / Subs.ExportNode() 支持的格式。 */
export const EXPORT_FORMATS = ['uri', 'base64', 'clash', 'singbox'] as const;
export type ExportFormat = (typeof EXPORT_FORMATS)[number];

/** Subs.ShareDeepLink() 的返回值项。 */
export interface DeepLink {
  scheme: string;
  label: string;
  url: string;
}

/** System.ParseDeepLink() / deeplink:received 事件负载。 */
export interface DeepLinkPreview {
  scheme: string;
  kind: 'url' | 'text';
  name: string;
  host: string;
  urlMasked?: string;
  payload: string;
  warning?: string;
}

/** System.CheckUpdate() 的返回值。 */
export interface UpdateInfo {
  available: boolean;
  version: string;
  channel: string;
  configured: boolean;
}

/** Update.GetState() / update:state 事件负载。 */
export interface UpdateState {
  phase: string;
  version: string;
  progress: number;
  error?: string;
}

/** System.ServiceStatus() 的返回值。 */
export interface ServiceStatus {
  installed: boolean;
  running: boolean;
  needsRepair?: boolean;
  startType: string;
  version: string;
}

/** System.TunStatus() 的返回值。 */
export interface TunStatus {
  enabled: boolean;
  running: boolean;
  stack: string;
  strictRoute: boolean;
  excludes: string[];
  error?: string;
}

/**
 * System.Capabilities() 公布的能力名。后端只公布已实现的方法组；
 * 未公布的能力界面不得调用对应接口。
 */
export const CAP_SUBS_URL = 'subs.url';
export const CAP_SUBS_UA = 'subs.ua';
export const CAP_NODE_GROUPS = 'nodes.groups';
export const CAP_SHARE = 'share';
export const CAP_SHARE_LAN = 'share.lan';
export const CAP_QR = 'qr';
export const CAP_QR_SCREEN = 'qr.screen';
export const CAP_DEEPLINK = 'deeplink';
export const CAP_DEEPLINK_REGISTER = 'deeplink.register';
export const CAP_RULES = 'rules';
export const CAP_APP_GROUPS = 'rules.appgroups';
export const CAP_RULE_SETS = 'rulesets';
export const CAP_BACKUP = 'backup';
export const CAP_DIAGNOSTICS = 'diagnostics';
export const CAP_SYS_PROXY = 'system.proxy';
export const CAP_SERVICE = 'system.service';
export const CAP_TUN = 'system.tun';
export const CAP_LATENCY = 'core.latency';
export const CAP_CONNECTIONS = 'connections';
export const CAP_LOGS = 'logs';
export const CAP_PERSIST = 'settings.persist';
export const CAP_AUTOMATION = 'settings.automation';
export const CAP_SYSTEM_TOOLS = 'system.tools';
export const CAP_UPDATE_ONLINE = 'update.online';

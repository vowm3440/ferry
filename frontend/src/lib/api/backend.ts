/*
 * 后端抽象。界面只依赖这个接口，不直接接触 Wails 运行时：
 * 之后接入生成的 bindings 时，只需要替换 wails.ts 的实现。
 *
 * 接口按 Go 侧的服务分组（Core、Nodes、Subs……）。只列出后端真实实现的方法；
 * 服务安装、TUN 与在线更新没有对应的条目，界面无法调用它们。
 */

import type { BackendEvent, CoreState, Group, NodeList, SettingsValue, Subscription, Traffic } from './types';
import type { Connection, DeepLink, DeepLinkPreview, LanShare, LogEntry, ServiceStatus, SysProxyStatus, TunStatus, UpdateInfo, UpdateState } from './types-inspect';
import type { AppGroup, ApplyPresetResult, CustomRuleSet, ImportPreview, LazyPresetOptions, Rule, RuleSetInfo, RuleTestResult } from './types-rules';

export type BackendKind = 'wails' | 'demo';

export type EventHandler = (payload: unknown) => void;
export type Unsubscribe = () => void;

export interface CoreApi {
  coreStatus(): Promise<CoreState>;
  coreStart(): Promise<void>;
  coreStop(): Promise<void>;
  coreRestart(): Promise<void>;
  coreSetMode(mode: string): Promise<void>;
  coreSelectNode(groupId: string, nodeId: string): Promise<void>;
  coreSnapshot(): Promise<Traffic>;
  /** 开发入口：导入受限 RunSpec（JSON 文本）。 */
  coreConfigure(specJson: string): Promise<void>;
  /** 测速（需要内核运行）。过程中通过 nodes:latency 事件逐批上报；返回 节点 id → 毫秒，-1 为超时。 */
  coreLatencyTest(ids: string[]): Promise<Record<string, number>>;
}

export interface NodesApi {
  nodesList(): Promise<NodeList>;
  nodesSetFavorite(id: string, on: boolean): Promise<void>;
  nodesSaveGroup(group: Group): Promise<Group>;
  nodesRemoveGroup(id: string): Promise<void>;
}

export interface SubsApi {
  subsList(): Promise<Subscription[]>;
  /** 导入粘贴的分享链接或配置；内容可能含凭据，只交给后端。 */
  subsAddFromText(name: string, text: string): Promise<Subscription>;
  /** 从订阅地址导入。地址可能含令牌：只交给后端，之后界面只看得到打码后的 urlMasked。 */
  subsAddFromURL(name: string, url: string, autoUpdate: boolean): Promise<Subscription>;
  /** 从本机文件导入；path 来自系统的文件选择框。 */
  subsAddFromFile(path: string): Promise<Subscription>;
  /** 从二维码图片导入；encoded 是 Base64 或 data: URL。 */
  subsImportFromQR(encoded: string): Promise<Subscription[]>;
  /** 截取当前屏幕识别二维码并导入；调用前界面必须先获得用户确认。 */
  subsCaptureScreenQR(): Promise<Subscription[]>;
  subsRename(id: string, name: string): Promise<void>;
  subsSetAutoUpdate(id: string, on: boolean, hours: number): Promise<void>;
  subsSetUserAgent(id: string, ua: string): Promise<void>;
  /** 更新单个订阅；进度通过 subs:progress 事件上报。内核运行中会重建内核。 */
  subsUpdate(id: string): Promise<void>;
  subsUpdateAll(): Promise<void>;
  subsRemove(id: string): Promise<void>;
}

export interface ShareApi {
  /** 导出整个订阅的节点。format：uri | base64 | clash | singbox。结果含凭据，只用于展示与复制。 */
  subsExport(id: string, format: string): Promise<string>;
  /** 导出单个节点。 */
  subsExportNode(nodeId: string, format: string): Promise<string>;
  /** 生成二维码，返回完整的 SVG 文档文本。size 为 128–1024。 */
  subsQrSvg(payload: string, size: number): Promise<string>;
  /** 生成其他客户端可识别的分享深链。 */
  subsShareDeepLink(payload: string): Promise<DeepLink[]>;
  /** 开启一次性局域网分享。只能由用户显式触发。 */
  lanShareStart(): Promise<LanShare>;
  lanShareStop(): Promise<void>;
  lanShareStatus(): Promise<LanShare>;
}

export interface ConnectionsApi {
  connsSnapshot(): Promise<Connection[]>;
  connsClose(id: string): Promise<void>;
  connsCloseAll(): Promise<void>;
  /** 暂停或恢复 connections:snapshot 事件的推送。 */
  connsPause(on: boolean): Promise<void>;
}

export interface LogsApi {
  /** 最近 limit 条日志（1–5000），旧 → 新。 */
  logsSnapshot(limit: number): Promise<LogEntry[]>;
  /** 暂停或恢复 logs:entry 事件的推送。 */
  logsPause(on: boolean): Promise<void>;
  logsSetLevel(level: string): Promise<void>;
  /** 明文导出到 path（文件必须尚不存在）；调用前界面必须先让用户确认。 */
  logsExport(path: string): Promise<void>;
}

export interface RulesApi {
  rulesList(): Promise<Rule[]>;
  /** id 为空串时新建。内核运行中会重建内核。 */
  rulesSave(rule: Rule): Promise<Rule>;
  /** 一次写入一批新规则（导入）：整批通过才写入并只应用一次，否则一条都不写；传入的 id 不会被沿用。 */
  rulesSaveMany(rules: readonly Rule[]): Promise<void>;
  rulesRemove(id: string): Promise<void>;
  rulesSetEnabled(id: string, on: boolean): Promise<void>;
  /** 传入完整的 id 顺序，固定规则必须留在首尾。 */
  rulesReorder(ids: string[]): Promise<void>;
  /** 按完整的 id 顺序重新应用规则（例如规则集更新之后）。 */
  rulesApply(ids: string[]): Promise<void>;
  /** 用一条活动连接预填规则；返回的规则尚未保存（id 为空）。 */
  rulesCreateFromConnection(connectionId: string): Promise<Rule>;
  /** 解析外部规则，只返回预览，不写入。kind：clash | singbox | ppx。 */
  rulesImport(kind: string, content: string): Promise<ImportPreview>;
  /** network 只能是 tcp 或 udp。 */
  rulesTest(application: string, host: string, port: number, network: string): Promise<RuleTestResult>;
  rulesDefaultLazyPreset(): Promise<LazyPresetOptions>;
  rulesApplyLazyPreset(opts: LazyPresetOptions): Promise<ApplyPresetResult>;

  appGroupsList(): Promise<AppGroup[]>;
  /** id 为空串时新建。 */
  appGroupsSave(group: AppGroup): Promise<AppGroup>;
  /** 幂等追加一个进程名或路径到已有应用分组。 */
  appGroupsAddApp(groupId: string, app: string): Promise<AppGroup>;
  appGroupsRemove(id: string): Promise<void>;

  /** refresh 为 true 时重新检查本地缓存；不会下载。 */
  ruleSetsList(refresh: boolean): Promise<RuleSetInfo[]>;
  ruleSetsUpdate(ids: string[]): Promise<void>;
  ruleSetsPresets(): Promise<CustomRuleSet[]>;
  ruleSetsSave(rs: CustomRuleSet): Promise<CustomRuleSet>;
  ruleSetsRemove(id: string): Promise<void>;
}

export interface SettingsApi {
  settingsGet(): Promise<SettingsValue>;
  settingsSet(value: SettingsValue): Promise<void>;
}

export interface SystemApi {
  /** 后端公布的能力名（见 types-inspect.ts 的 CAP_*）。本次运行内不变。 */
  capabilities(): Promise<string[]>;
  sysProxyStatus(): Promise<SysProxyStatus>;
  /** 开启需要内核运行；关闭随时可用。 */
  setSysProxy(on: boolean): Promise<void>;
  serviceStatus(): Promise<ServiceStatus>;
  installService(): Promise<void>;
  uninstallService(): Promise<void>;
  repairService(): Promise<void>;
  tunStatus(): Promise<TunStatus>;
  setTun(on: boolean): Promise<void>;
  /** 以下三个方法的 path 都来自系统的文件选择框，且导出目标必须尚不存在。 */
  exportBackup(path: string): Promise<void>;
  restoreBackup(path: string): Promise<void>;
  exportDiagnostics(path: string): Promise<void>;
  /** 探测混合端口是否可用或测延迟（毫秒）。 */
  testInbound(): Promise<number>;
  openConfigDir(): Promise<void>;
  openLogDir(): Promise<void>;
  parseDeepLink(raw: string): Promise<DeepLinkPreview>;
  deepLinkStatus(): Promise<boolean>;
  setDeepLinkRegistered(on: boolean): Promise<void>;
  windowSetLightweight(on: boolean): Promise<void>;
  updateGetState(): Promise<UpdateState>;
  checkUpdate(channel: string): Promise<UpdateInfo>;
  applyUpdate(): Promise<void>;
  installUpdate(): Promise<void>;
}

export interface Backend
  extends CoreApi,
    NodesApi,
    SubsApi,
    ShareApi,
    ConnectionsApi,
    LogsApi,
    RulesApi,
    SettingsApi,
    SystemApi {
  readonly kind: BackendKind;

  /** 订阅后端事件，返回取消订阅函数。 */
  on(event: BackendEvent, handler: EventHandler): Unsubscribe;
}

interface MessagePort {
  postMessage?: unknown;
}

interface BridgeHost {
  chrome?: { webview?: MessagePort };
  webkit?: { messageHandlers?: Record<string, MessagePort | undefined> };
}

/**
 * 是否运行在 Wails 桌面壳内。
 *
 * 判断依据是 WebView 注入的原生消息通道（Windows 上是 chrome.webview），
 * 而不是 window._wails：后者会被 @wailsio/runtime 自己在导入时创建，
 * 在普通浏览器里也会存在，不能说明后端可用。
 */
export function hasDesktopBridge(): boolean {
  if (typeof window === 'undefined') return false;
  const host = window as unknown as BridgeHost;
  if (typeof host.chrome?.webview?.postMessage === 'function') return true;
  return typeof host.webkit?.messageHandlers?.['external']?.postMessage === 'function';
}

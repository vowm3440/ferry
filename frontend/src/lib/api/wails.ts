/*
 * 真实后端：通过 Wails v3 运行时按「包路径.服务.方法」全名调用 Go 服务。
 *
 * - Go 方法的第一个 context.Context 由 Wails 在服务端注入，这里只传业务参数。
 * - 生成 bindings 之后，把下面的 invoke(...) 换成生成函数即可，接口不变。
 * - 只列出 Go 侧已注册的方法（cmd/ferry/main_windows.go）。服务安装、TUN、在线更新、
 *   深链与屏幕二维码没有实现，这里不出现，避免界面调用到不存在的接口。
 * - 是否可以调用某一组方法，由 System.Capabilities() 决定，见 stores/caps.svelte.ts。
 */

import type { Backend, EventHandler, Unsubscribe } from './backend';
import { toBackendError } from './errors';
import { parseCoreState, parseGroup, parseNodeList, parseSettings, parseSubscription, parseSubscriptions, parseTraffic } from './parse';
import {
  parseCapabilities,
  parseConnections,
  parseLanShare,
  parseLatencyResult,
  parseLogEntries,
  parseSysProxyStatus,
} from './parse-inspect';
import {
  parseAppGroup,
  parseAppGroups,
  parseImportPreview,
  parseRule,
  parseRuleSets,
  parseRuleTestResult,
  parseRules,
} from './parse-rules';
import { loadRuntime, type WailsRuntime } from './runtime';
import { EVENT_LOGS, type BackendEvent, type CoreState, type Group, type NodeList, type SettingsValue, type Subscription, type Traffic } from './types';
import type { Connection, DeepLink, DeepLinkPreview, LanShare, LogEntry, ServiceStatus, SysProxyStatus, TunStatus, UpdateInfo, UpdateState } from './types-inspect';
import type { AppGroup, ImportPreview, Rule, RuleSetInfo, RuleTestResult } from './types-rules';

/** Go 服务所在的包路径（ferry/internal/app）。 */
const SERVICE_PACKAGE = 'ferry/internal/app';

const method = (name: string): string => `${SERVICE_PACKAGE}.${name}`;

export const METHODS = {
  coreStatus: method('Core.Status'),
  coreStart: method('Core.Start'),
  coreStop: method('Core.Stop'),
  coreRestart: method('Core.Restart'),
  coreSetMode: method('Core.SetMode'),
  coreSelectNode: method('Core.SelectNode'),
  coreSnapshot: method('Core.Snapshot'),
  coreConfigure: method('Core.Configure'),
  coreLatencyTest: method('Core.LatencyTest'),
  nodesList: method('Nodes.List'),
  nodesSetFavorite: method('Nodes.SetFavorite'),
  nodesSaveGroup: method('Nodes.SaveGroup'),
  nodesRemoveGroup: method('Nodes.RemoveGroup'),
  subsList: method('Subs.List'),
  subsAddFromText: method('Subs.AddFromText'),
  subsAddFromURL: method('Subs.AddFromURL'),
  subsAddFromFile: method('Subs.AddFromFile'),
  subsImportFromQR: method('Subs.ImportFromQR'),
  subsCaptureScreenQR: method('Subs.CaptureScreenQR'),
  subsRename: method('Subs.Rename'),
  subsSetAutoUpdate: method('Subs.SetAutoUpdate'),
  subsSetUserAgent: method('Subs.SetUserAgent'),
  subsUpdate: method('Subs.Update'),
  subsUpdateAll: method('Subs.UpdateAll'),
  subsRemove: method('Subs.Remove'),
  subsExport: method('Subs.Export'),
  subsExportNode: method('Subs.ExportNode'),
  subsQrSvg: method('Subs.QRSvg'),
  subsShareDeepLink: method('Subs.ShareDeepLink'),
  lanShareStart: method('Subs.LanShareStart'),
  lanShareStop: method('Subs.LanShareStop'),
  lanShareStatus: method('Subs.LanShareStatus'),
  connsSnapshot: method('Connections.Snapshot'),
  connsClose: method('Connections.Close'),
  connsCloseAll: method('Connections.CloseAll'),
  connsPause: method('Connections.Pause'),
  logsSnapshot: method('Logs.Snapshot'),
  logsPause: method('Logs.Pause'),
  logsSetLevel: method('Logs.SetLevel'),
  logsExport: method('Logs.Export'),
  rulesList: method('Rules.List'),
  rulesSave: method('Rules.Save'),
  rulesSaveMany: method('Rules.SaveMany'),
  rulesRemove: method('Rules.Remove'),
  rulesSetEnabled: method('Rules.SetEnabled'),
  rulesReorder: method('Rules.Reorder'),
  rulesApply: method('Rules.Apply'),
  rulesCreateFromConnection: method('Rules.CreateFromConnection'),
  rulesImport: method('Rules.Import'),
  rulesTest: method('Rules.Test'),
  appGroupsList: method('AppGroups.List'),
  appGroupsSave: method('AppGroups.Save'),
  appGroupsRemove: method('AppGroups.Remove'),
  ruleSetsList: method('RuleSets.List'),
  ruleSetsUpdate: method('RuleSets.Update'),
  settingsGet: method('Settings.Get'),
  settingsSet: method('Settings.Set'),
  capabilities: method('System.Capabilities'),
  sysProxyStatus: method('System.SysProxyStatus'),
  setSysProxy: method('System.SetSysProxy'),
  serviceStatus: method('System.ServiceStatus'),
  installService: method('System.InstallService'),
  uninstallService: method('System.UninstallService'),
  repairService: method('System.RepairService'),
  tunStatus: method('System.TunStatus'),
  setTun: method('System.SetTun'),
  exportBackup: method('System.ExportBackup'),
  restoreBackup: method('System.RestoreBackup'),
  exportDiagnostics: method('System.ExportDiagnostics'),
  testInbound: method('System.TestInbound'),
  openConfigDir: method('System.OpenConfigDir'),
  openLogDir: method('System.OpenLogDir'),
  parseDeepLink: method('System.ParseDeepLink'),
  deepLinkStatus: method('System.DeepLinkStatus'),
  setDeepLinkRegistered: method('System.SetDeepLinkRegistered'),
  windowSetLightweight: method('Window.SetLightweight'),
  updateGetState: method('Update.GetState'),
  checkUpdate: method('System.CheckUpdate'),
  applyUpdate: method('System.ApplyUpdate'),
  installUpdate: method('System.InstallUpdate'),
} as const;

/**
 * Go 侧 Emit(name, x) 单参数时 data 就是 x；多参数时是数组，这里兼容单元素数组。
 * logs:entry 的内容本身就是数组：只有一条日志时不能把它当成「包了一层」拆开，否则这条日志会被丢掉。
 */
function unwrapEventData(event: BackendEvent, data: unknown): unknown {
  if (!Array.isArray(data) || data.length !== 1) return data;
  const inner: unknown = data[0];
  if (event === EVENT_LOGS) return Array.isArray(inner) ? inner : data;
  return inner;
}

function asText(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

class WailsBackend implements Backend {
  readonly kind = 'wails' as const;
  readonly #runtime: WailsRuntime;

  constructor(runtime: WailsRuntime) {
    this.#runtime = runtime;
  }

  async #invoke(name: string, ...args: unknown[]): Promise<unknown> {
    try {
      return await this.#runtime.Call.ByName(name, ...args);
    } catch (error: unknown) {
      throw toBackendError(error);
    }
  }

  /** 没有返回值的调用。 */
  async #run(name: string, ...args: unknown[]): Promise<void> {
    await this.#invoke(name, ...args);
  }

  // ---- Core ----

  async coreStatus(): Promise<CoreState> {
    return parseCoreState(await this.#invoke(METHODS.coreStatus));
  }

  coreStart(): Promise<void> {
    return this.#run(METHODS.coreStart);
  }

  coreStop(): Promise<void> {
    return this.#run(METHODS.coreStop);
  }

  coreRestart(): Promise<void> {
    return this.#run(METHODS.coreRestart);
  }

  coreSetMode(mode: string): Promise<void> {
    return this.#run(METHODS.coreSetMode, mode);
  }

  coreSelectNode(groupId: string, nodeId: string): Promise<void> {
    return this.#run(METHODS.coreSelectNode, groupId, nodeId);
  }

  async coreSnapshot(): Promise<Traffic> {
    return parseTraffic(await this.#invoke(METHODS.coreSnapshot));
  }

  coreConfigure(specJson: string): Promise<void> {
    return this.#run(METHODS.coreConfigure, specJson);
  }

  async coreLatencyTest(ids: string[]): Promise<Record<string, number>> {
    return parseLatencyResult(await this.#invoke(METHODS.coreLatencyTest, ids));
  }

  // ---- Nodes ----

  async nodesList(): Promise<NodeList> {
    return parseNodeList(await this.#invoke(METHODS.nodesList));
  }

  nodesSetFavorite(id: string, on: boolean): Promise<void> {
    return this.#run(METHODS.nodesSetFavorite, id, on);
  }

  async nodesSaveGroup(group: Group): Promise<Group> {
    return parseGroup(await this.#invoke(METHODS.nodesSaveGroup, group));
  }

  nodesRemoveGroup(id: string): Promise<void> {
    return this.#run(METHODS.nodesRemoveGroup, id);
  }

  // ---- Subs ----

  async subsList(): Promise<Subscription[]> {
    return parseSubscriptions(await this.#invoke(METHODS.subsList));
  }

  async subsAddFromText(name: string, text: string): Promise<Subscription> {
    return parseSubscription(await this.#invoke(METHODS.subsAddFromText, name, text));
  }

  async subsAddFromURL(name: string, url: string, autoUpdate: boolean): Promise<Subscription> {
    return parseSubscription(await this.#invoke(METHODS.subsAddFromURL, name, url, autoUpdate));
  }

  async subsAddFromFile(path: string): Promise<Subscription> {
    return parseSubscription(await this.#invoke(METHODS.subsAddFromFile, path));
  }

  async subsImportFromQR(encoded: string): Promise<Subscription[]> {
    return parseSubscriptions(await this.#invoke(METHODS.subsImportFromQR, encoded));
  }

  async subsCaptureScreenQR(): Promise<Subscription[]> {
    return parseSubscriptions(await this.#invoke(METHODS.subsCaptureScreenQR));
  }

  subsRename(id: string, name: string): Promise<void> {
    return this.#run(METHODS.subsRename, id, name);
  }

  subsSetAutoUpdate(id: string, on: boolean, hours: number): Promise<void> {
    return this.#run(METHODS.subsSetAutoUpdate, id, on, hours);
  }

  subsSetUserAgent(id: string, ua: string): Promise<void> {
    return this.#run(METHODS.subsSetUserAgent, id, ua);
  }

  subsUpdate(id: string): Promise<void> {
    return this.#run(METHODS.subsUpdate, id);
  }

  subsUpdateAll(): Promise<void> {
    return this.#run(METHODS.subsUpdateAll);
  }

  subsRemove(id: string): Promise<void> {
    return this.#run(METHODS.subsRemove, id);
  }

  // ---- 分享 ----

  async subsExport(id: string, format: string): Promise<string> {
    return asText(await this.#invoke(METHODS.subsExport, id, format));
  }

  async subsExportNode(nodeId: string, format: string): Promise<string> {
    return asText(await this.#invoke(METHODS.subsExportNode, nodeId, format));
  }

  async subsQrSvg(payload: string, size: number): Promise<string> {
    return asText(await this.#invoke(METHODS.subsQrSvg, payload, size));
  }

  async subsShareDeepLink(payload: string): Promise<DeepLink[]> {
    const raw = await this.#invoke(METHODS.subsShareDeepLink, payload);
    return Array.isArray(raw) ? (raw as DeepLink[]) : [];
  }

  async lanShareStart(): Promise<LanShare> {
    return parseLanShare(await this.#invoke(METHODS.lanShareStart));
  }

  lanShareStop(): Promise<void> {
    return this.#run(METHODS.lanShareStop);
  }

  async lanShareStatus(): Promise<LanShare> {
    return parseLanShare(await this.#invoke(METHODS.lanShareStatus));
  }

  // ---- Connections ----

  async connsSnapshot(): Promise<Connection[]> {
    return parseConnections(await this.#invoke(METHODS.connsSnapshot));
  }

  connsClose(id: string): Promise<void> {
    return this.#run(METHODS.connsClose, id);
  }

  connsCloseAll(): Promise<void> {
    return this.#run(METHODS.connsCloseAll);
  }

  connsPause(on: boolean): Promise<void> {
    return this.#run(METHODS.connsPause, on);
  }

  // ---- Logs ----

  async logsSnapshot(limit: number): Promise<LogEntry[]> {
    return parseLogEntries(await this.#invoke(METHODS.logsSnapshot, limit));
  }

  logsPause(on: boolean): Promise<void> {
    return this.#run(METHODS.logsPause, on);
  }

  logsSetLevel(level: string): Promise<void> {
    return this.#run(METHODS.logsSetLevel, level);
  }

  logsExport(path: string): Promise<void> {
    return this.#run(METHODS.logsExport, path);
  }

  // ---- Rules / AppGroups / RuleSets ----

  async rulesList(): Promise<Rule[]> {
    return parseRules(await this.#invoke(METHODS.rulesList));
  }

  async rulesSave(rule: Rule): Promise<Rule> {
    return parseRule(await this.#invoke(METHODS.rulesSave, rule));
  }

  rulesSaveMany(rules: readonly Rule[]): Promise<void> {
    return this.#run(METHODS.rulesSaveMany, rules);
  }

  rulesRemove(id: string): Promise<void> {
    return this.#run(METHODS.rulesRemove, id);
  }

  rulesSetEnabled(id: string, on: boolean): Promise<void> {
    return this.#run(METHODS.rulesSetEnabled, id, on);
  }

  rulesReorder(ids: string[]): Promise<void> {
    return this.#run(METHODS.rulesReorder, ids);
  }

  rulesApply(ids: string[]): Promise<void> {
    return this.#run(METHODS.rulesApply, ids);
  }

  async rulesCreateFromConnection(connectionId: string): Promise<Rule> {
    return parseRule(await this.#invoke(METHODS.rulesCreateFromConnection, connectionId));
  }

  async rulesImport(kind: string, content: string): Promise<ImportPreview> {
    return parseImportPreview(await this.#invoke(METHODS.rulesImport, kind, content));
  }

  async rulesTest(application: string, host: string, port: number, network: string): Promise<RuleTestResult> {
    return parseRuleTestResult(await this.#invoke(METHODS.rulesTest, application, host, port, network));
  }

  async appGroupsList(): Promise<AppGroup[]> {
    return parseAppGroups(await this.#invoke(METHODS.appGroupsList));
  }

  async appGroupsSave(group: AppGroup): Promise<AppGroup> {
    return parseAppGroup(await this.#invoke(METHODS.appGroupsSave, group));
  }

  appGroupsRemove(id: string): Promise<void> {
    return this.#run(METHODS.appGroupsRemove, id);
  }

  async ruleSetsList(refresh: boolean): Promise<RuleSetInfo[]> {
    return parseRuleSets(await this.#invoke(METHODS.ruleSetsList, refresh));
  }

  ruleSetsUpdate(ids: string[]): Promise<void> {
    return this.#run(METHODS.ruleSetsUpdate, ids);
  }

  // ---- Settings ----

  async settingsGet(): Promise<SettingsValue> {
    return parseSettings(await this.#invoke(METHODS.settingsGet));
  }

  settingsSet(value: SettingsValue): Promise<void> {
    return this.#run(METHODS.settingsSet, value);
  }

  // ---- System ----

  async capabilities(): Promise<string[]> {
    return parseCapabilities(await this.#invoke(METHODS.capabilities));
  }

  async sysProxyStatus(): Promise<SysProxyStatus> {
    return parseSysProxyStatus(await this.#invoke(METHODS.sysProxyStatus));
  }

  setSysProxy(on: boolean): Promise<void> {
    return this.#run(METHODS.setSysProxy, on);
  }

  async serviceStatus(): Promise<ServiceStatus> {
    return (await this.#invoke(METHODS.serviceStatus)) as ServiceStatus;
  }

  installService(): Promise<void> {
    return this.#run(METHODS.installService);
  }

  uninstallService(): Promise<void> {
    return this.#run(METHODS.uninstallService);
  }

  repairService(): Promise<void> {
    return this.#run(METHODS.repairService);
  }

  async tunStatus(): Promise<TunStatus> {
    return (await this.#invoke(METHODS.tunStatus)) as TunStatus;
  }

  setTun(on: boolean): Promise<void> {
    return this.#run(METHODS.setTun, on);
  }

  exportBackup(path: string): Promise<void> {
    return this.#run(METHODS.exportBackup, path);
  }

  restoreBackup(path: string): Promise<void> {
    return this.#run(METHODS.restoreBackup, path);
  }

  exportDiagnostics(path: string): Promise<void> {
    return this.#run(METHODS.exportDiagnostics, path);
  }

  async testInbound(): Promise<number> {
    const value = await this.#invoke(METHODS.testInbound);
    return typeof value === 'number' ? value : -1;
  }

  openConfigDir(): Promise<void> {
    return this.#run(METHODS.openConfigDir);
  }

  openLogDir(): Promise<void> {
    return this.#run(METHODS.openLogDir);
  }

  async parseDeepLink(raw: string): Promise<DeepLinkPreview> {
    return (await this.#invoke(METHODS.parseDeepLink, raw)) as DeepLinkPreview;
  }

  async deepLinkStatus(): Promise<boolean> {
    return (await this.#invoke(METHODS.deepLinkStatus)) === true;
  }

  setDeepLinkRegistered(on: boolean): Promise<void> {
    return this.#run(METHODS.setDeepLinkRegistered, on);
  }

  windowSetLightweight(on: boolean): Promise<void> {
    return this.#run(METHODS.windowSetLightweight, on);
  }

  async updateGetState(): Promise<UpdateState> {
    return (await this.#invoke(METHODS.updateGetState)) as UpdateState;
  }

  async checkUpdate(channel: string): Promise<UpdateInfo> {
    return (await this.#invoke(METHODS.checkUpdate, channel)) as UpdateInfo;
  }

  applyUpdate(): Promise<void> {
    return this.#run(METHODS.applyUpdate);
  }

  installUpdate(): Promise<void> {
    return this.#run(METHODS.installUpdate);
  }

  on(event: BackendEvent, handler: EventHandler): Unsubscribe {
    return this.#runtime.Events.On(event, (ev) => handler(unwrapEventData(event, ev.data)));
  }
}

export async function createWailsBackend(): Promise<Backend> {
  try {
    return new WailsBackend(await loadRuntime());
  } catch (error: unknown) {
    throw toBackendError(error);
  }
}

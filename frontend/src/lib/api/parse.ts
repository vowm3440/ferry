/*
 * 把后端返回值（unknown）解析成类型化对象。字段缺失或类型不符时回退到
 * 安全默认值：宁可显示空数据，也不把 undefined/NaN 渲染到界面上。
 */

import {
  asRecord,
  readArray,
  readBool,
  readNumber,
  readNumberArray,
  readString,
  readStringArray,
} from '../guard';
import type {
  CoreState,
  DnsSettings,
  Group,
  Node,
  NodeList,
  SettingsValue,
  SkippedLine,
  Subscription,
  Traffic,
  TunSettings,
} from './types';

function parseNode(value: unknown): Node {
  const source = asRecord(value);
  return {
    id: readString(source, 'id'),
    tag: readString(source, 'tag'),
    name: readString(source, 'name'),
    groupId: readString(source, 'groupId'),
    type: readString(source, 'type'),
    server: readString(source, 'server'),
    port: readNumber(source, 'port'),
    latencyMs: readNumber(source, 'latencyMs', -1),
    latencyStatus: readString(source, 'latencyStatus', 'idle'),
    selected: readBool(source, 'selected'),
    favorite: readBool(source, 'favorite'),
  };
}

export function parseGroup(value: unknown): Group {
  const source = asRecord(value);
  const selected = readString(source, 'selected');
  return {
    id: readString(source, 'id'),
    name: readString(source, 'name'),
    type: readString(source, 'type'),
    members: readStringArray(source, 'members'),
    ...(selected === '' ? {} : { selected }),
    testUrl: readString(source, 'testUrl'),
    intervalMin: readNumber(source, 'intervalMin'),
  };
}

export function parseNodeList(value: unknown): NodeList {
  const source = asRecord(value);
  return {
    groups: readArray(source, 'groups').map(parseGroup),
    nodes: readArray(source, 'nodes').map(parseNode),
  };
}

function parseSkipped(value: unknown): SkippedLine {
  const source = asRecord(value);
  return { line: readNumber(source, 'line'), reason: readString(source, 'reason') };
}

export function parseSubscription(value: unknown): Subscription {
  const source = asRecord(value);
  const error = readString(source, 'error');
  return {
    id: readString(source, 'id'),
    name: readString(source, 'name'),
    urlMasked: readString(source, 'urlMasked'),
    nodeCount: readNumber(source, 'nodeCount'),
    groupCount: readNumber(source, 'groupCount'),
    usedBytes: readNumber(source, 'usedBytes'),
    totalBytes: readNumber(source, 'totalBytes'),
    expiresAt: readNumber(source, 'expiresAt'),
    lastUpdatedAt: readNumber(source, 'lastUpdatedAt'),
    intervalHours: readNumber(source, 'intervalHours'),
    autoUpdate: readBool(source, 'autoUpdate'),
    userAgent: readString(source, 'userAgent'),
    status: readString(source, 'status', 'ok'),
    ...(error === '' ? {} : { error }),
    skipped: readArray(source, 'skipped')
      .map(parseSkipped)
      .filter((item) => item.reason !== ''),
    warnings: readStringArray(source, 'warnings').filter((item) => item !== ''),
  };
}

/** 没有 id 的条目无法删除，也不该显示。 */
export function parseSubscriptions(value: unknown): Subscription[] {
  return (Array.isArray(value) ? value : []).map(parseSubscription).filter((item) => item.id !== '');
}

export function parseCoreState(value: unknown): CoreState {
  const source = asRecord(value);
  const error = readString(source, 'error');
  return {
    status: readString(source, 'status', 'stopped'),
    mode: readString(source, 'mode', 'rule'),
    uptimeMs: readNumber(source, 'uptimeMs'),
    memoryBytes: readNumber(source, 'memoryBytes'),
    version: readString(source, 'version'),
    coreVersion: readString(source, 'coreVersion'),
    serviceMode: readBool(source, 'serviceMode'),
    ...(error === '' ? {} : { error }),
  };
}

export function parseTraffic(value: unknown): Traffic {
  const source = asRecord(value);
  return {
    up: readNumber(source, 'up'),
    down: readNumber(source, 'down'),
    activeConns: readNumber(source, 'activeConns'),
    totalUp: readNumber(source, 'totalUp'),
    totalDown: readNumber(source, 'totalDown'),
    memoryBytes: readNumber(source, 'memoryBytes'),
    upHistory: readNumberArray(source, 'upHistory'),
    downHistory: readNumberArray(source, 'downHistory'),
  };
}

function parseTun(value: unknown): TunSettings {
  const source = asRecord(value);
  return {
    stack: readString(source, 'stack'),
    strictRoute: readBool(source, 'strictRoute'),
    excludes: readStringArray(source, 'excludes'),
    mtu: readNumber(source, 'mtu'),
    fakeIp: readBool(source, 'fakeIp'),
  };
}

function parseDns(value: unknown): DnsSettings {
  const source = asRecord(value);
  return {
    remote: readString(source, 'remote'),
    direct: readString(source, 'direct'),
    strategy: readString(source, 'strategy'),
  };
}

export function parseSettings(value: unknown): SettingsValue {
  const source = asRecord(value);
  return {
    autoStart: readBool(source, 'autoStart'),
    startMinimized: readBool(source, 'startMinimized'),
    lightweight: readBool(source, 'lightweight'),
    language: readString(source, 'language', 'zh-CN'),
    theme: readString(source, 'theme', 'system'),
    mixedPort: readNumber(source, 'mixedPort'),
    allowLan: readBool(source, 'allowLan'),
    sysProxyOnStart: readBool(source, 'sysProxyOnStart'),
    tun: parseTun(source.tun),
    dns: parseDns(source.dns),
    ruleSetAutoUpdate: readBool(source, 'ruleSetAutoUpdate'),
    subscriptionUA: readString(source, 'subscriptionUA'),
    subscriptionUpdateHours: readNumber(source, 'subscriptionUpdateHours'),
    logLevel: readString(source, 'logLevel', 'info'),
    updateChannel: readString(source, 'updateChannel', 'stable'),
    clashApiMode: readString(source, 'clashApiMode', 'random'),
  };
}

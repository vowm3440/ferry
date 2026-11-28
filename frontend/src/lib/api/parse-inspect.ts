/*
 * 连接、日志、测速、订阅进度、系统代理、局域网分享与能力列表的解析。
 * 约定与 parse.ts 相同：字段缺失或类型不符时回退到安全默认值，只取约定的字段。
 */

import { asRecord, isRecord, readBool, readNumber, readString, readStringArray } from '../guard';
import {
  LATENCY_IDLE,
  type Connection,
  type LanShare,
  type LatencyMap,
  type LatencyUpdate,
  type LogEntry,
  type SubsProgress,
  type SysProxyStatus,
} from './types-inspect';

function parseConnection(value: unknown): Connection {
  const source = asRecord(value);
  const ruleId = readString(source, 'ruleId');
  const ruleDetail = readString(source, 'ruleDetail');
  const network = readString(source, 'network');
  return {
    id: readString(source, 'id'),
    process: readString(source, 'process'),
    processPath: readString(source, 'processPath'),
    host: readString(source, 'host'),
    port: readNumber(source, 'port'),
    ...(network === '' ? {} : { network }),
    ruleName: readString(source, 'ruleName'),
    ...(ruleId === '' ? {} : { ruleId }),
    ...(ruleDetail === '' ? {} : { ruleDetail }),
    action: readString(source, 'action'),
    outboundTag: readString(source, 'outboundTag'),
    chain: readStringArray(source, 'chain'),
    up: readNumber(source, 'up'),
    down: readNumber(source, 'down'),
    startAt: readNumber(source, 'startAt'),
  };
}

/** Connections.Snapshot() 的结果：没有 id 的连接无法关闭，也不该显示。 */
export function parseConnections(value: unknown): Connection[] {
  return (Array.isArray(value) ? value : []).map(parseConnection).filter((item) => item.id !== '');
}

/** connections:snapshot 事件：items 已经是完整列表，差量字段（added/updated/removed）不需要。 */
export function parseConnectionsEvent(value: unknown): Connection[] {
  return parseConnections(asRecord(value).items);
}

function parseLogEntry(value: Record<string, unknown>): LogEntry {
  return {
    seq: readNumber(value, 'seq'),
    time: readNumber(value, 'time'),
    level: readString(value, 'level'),
    message: readString(value, 'message'),
  };
}

/** Logs.Snapshot() 的结果与 logs:entry 事件的负载，保持后端给出的顺序（旧 → 新）。 */
export function parseLogEntries(value: unknown): LogEntry[] {
  return (Array.isArray(value) ? value : []).filter(isRecord).map(parseLogEntry);
}

function parseLatencyUpdate(value: Record<string, unknown>): LatencyUpdate {
  return {
    latencyMs: readNumber(value, 'latencyMs', -1),
    status: readString(value, 'status', LATENCY_IDLE),
  };
}

/** nodes:latency 事件的负载：节点 id → 结果。 */
export function parseLatencyMap(value: unknown): LatencyMap {
  const entries = Object.entries(asRecord(value)).flatMap(([id, item]): [string, LatencyUpdate][] =>
    isRecord(item) ? [[id, parseLatencyUpdate(item)]] : [],
  );
  return Object.fromEntries(entries);
}

/** Core.LatencyTest() 的返回值：节点 id → 毫秒，-1 表示超时。 */
export function parseLatencyResult(value: unknown): Record<string, number> {
  const entries = Object.entries(asRecord(value)).filter(
    (entry): entry is [string, number] => typeof entry[1] === 'number' && Number.isFinite(entry[1]),
  );
  return Object.fromEntries(entries);
}

export function parseSubsProgress(value: unknown): SubsProgress {
  const source = asRecord(value);
  const error = readString(source, 'error');
  return {
    id: readString(source, 'id'),
    phase: readString(source, 'phase'),
    ...(error === '' ? {} : { error }),
  };
}

export function parseSysProxyStatus(value: unknown): SysProxyStatus {
  const source = asRecord(value);
  return {
    enabled: readBool(source, 'enabled'),
    address: readString(source, 'address'),
    bypass: readStringArray(source, 'bypass'),
  };
}

export function parseLanShare(value: unknown): LanShare {
  const source = asRecord(value);
  return {
    url: readString(source, 'url'),
    expiresAt: readNumber(source, 'expiresAt'),
    active: readBool(source, 'active'),
  };
}

/** System.Capabilities() 的结果：能力名列表，去重并丢弃非字符串。 */
export function parseCapabilities(value: unknown): string[] {
  const names = (Array.isArray(value) ? value : []).filter(
    (item): item is string => typeof item === 'string' && item !== '',
  );
  return [...new Set(names)];
}

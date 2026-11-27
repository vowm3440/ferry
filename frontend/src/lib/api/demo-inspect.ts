/*
 * 演示后端：连接与日志。
 *
 * 内核「运行」期间每秒生成一批合成连接和日志，行为贴近真实后端：
 * 连接的 up / down 是累计字节数；暂停只是不再推送事件，快照仍然可读；
 * 暂停期间的日志之后只能从快照里读到；内核停止后连接清空，日志保留。
 * 主机名全部使用保留域名 .invalid，不代表任何真实流量。导出日志要写本机文件，演示模式不提供。
 */

import type { ConnectionsApi, LogsApi } from './backend';
import { rejected, unsupported, type DemoWorld } from './demo-world';
import { EVENT_CONNECTIONS, EVENT_LOGS } from './types';
import type { Connection, LogEntry } from './types-inspect';
import { RULE_ID_DEFAULT, RULE_ID_LAN } from './types-rules';

const LOG_BUFFER_MAX = 5000;
const LOG_BATCH_MAX = 20;
const TARGET_CONNECTIONS = 12;
const MAX_CONNECTIONS = 18;
const KIBIBYTE = 1024;

const LEVEL_RANK: Readonly<Record<string, number>> = {
  trace: 0,
  debug: 1,
  info: 2,
  warn: 3,
  error: 4,
  fatal: 5,
  panic: 6,
};

interface AppSeed {
  readonly process: string;
  readonly path: string;
}

interface TargetSeed {
  readonly host: string;
  readonly port: number;
  readonly lan: boolean;
  /** 每秒下行字节数的上限，用来让各连接的速率有差别。 */
  readonly ceiling: number;
}

const APPS: readonly AppSeed[] = [
  { process: 'browser.exe', path: 'C:\\Program Files\\Demo Browser\\browser.exe' },
  { process: 'editor.exe', path: 'C:\\Program Files\\Demo Editor\\editor.exe' },
  { process: 'chat.exe', path: 'C:\\Users\\demo\\AppData\\Local\\Demo Chat\\chat.exe' },
  { process: 'sync.exe', path: 'C:\\Program Files\\Demo Sync\\sync.exe' },
  { process: 'updater.exe', path: 'C:\\Program Files\\Demo Updater\\updater.exe' },
  { process: '', path: '' },
];

const TARGETS: readonly TargetSeed[] = [
  { host: 'www.example.invalid', port: 443, lan: false, ceiling: 180 * KIBIBYTE },
  { host: 'api.example.invalid', port: 443, lan: false, ceiling: 24 * KIBIBYTE },
  { host: 'cdn.example.invalid', port: 443, lan: false, ceiling: 2400 * KIBIBYTE },
  { host: 'video.example.invalid', port: 443, lan: false, ceiling: 3600 * KIBIBYTE },
  { host: 'static.example.invalid', port: 80, lan: false, ceiling: 320 * KIBIBYTE },
  { host: 'push.example.invalid', port: 5228, lan: false, ceiling: 2 * KIBIBYTE },
  { host: '192.168.1.20', port: 445, lan: true, ceiling: 900 * KIBIBYTE },
  { host: '192.168.1.1', port: 53, lan: true, ceiling: KIBIBYTE },
];

function pick<T>(items: readonly T[]): T {
  return items[Math.floor(Math.random() * items.length)];
}

function ceilingOf(conn: Connection): number {
  return TARGETS.find((seed) => seed.host === conn.host && seed.port === conn.port)?.ceiling ?? 8 * KIBIBYTE;
}

function grow(conn: Connection): Connection {
  // 大约三分之一的连接这一秒是空闲的。
  if (Math.random() < 0.33) return conn;
  const down = Math.round(Math.random() * ceilingOf(conn));
  return { ...conn, down: conn.down + down, up: conn.up + Math.round(down * (0.02 + Math.random() * 0.06)) };
}

export function createDemoInspect(world: DemoWorld): ConnectionsApi & LogsApi {
  let connSeq = 0;
  let connsPaused = false;
  let logSeq = 0;
  let logsPaused = false;
  let logBuffer: readonly LogEntry[] = [];
  /** 这一秒里产生、还没有推送的日志。 */
  let logBatch: readonly LogEntry[] = [];

  function log(level: string, message: string): void {
    const floor = LEVEL_RANK[world.settings.logLevel] ?? LEVEL_RANK.info;
    if ((LEVEL_RANK[level] ?? LEVEL_RANK.info) < floor) return;
    logSeq += 1;
    const entry: LogEntry = { seq: logSeq, time: Date.now(), level, message };
    logBuffer = [...logBuffer, entry].slice(-LOG_BUFFER_MAX);
    logBatch = [...logBatch, entry];
  }

  function flushLogs(): void {
    const batch = logBatch;
    logBatch = [];
    // 与真实后端一致：暂停期间不推送，这些日志之后只能从快照里读到。
    if (logsPaused) return;
    for (let start = 0; start < batch.length; start += LOG_BATCH_MAX) {
      world.emit(EVENT_LOGS, batch.slice(start, start + LOG_BATCH_MAX));
    }
  }

  function outboundName(): string {
    return world.nodes.find((node) => node.id === world.selected)?.name ?? '';
  }

  function openConnection(): Connection {
    const app = pick(APPS);
    const target = pick(TARGETS);
    const mode = world.mode;
    const direct = mode === 'direct' || (mode === 'rule' && target.lan);
    const byRule = mode === 'rule';
    const outbound = direct ? 'direct' : outboundName();
    connSeq += 1;
    const conn: Connection = {
      id: `demo-conn-${connSeq}`,
      process: app.process,
      processPath: app.path,
      host: target.host,
      port: target.port,
      ruleName: byRule ? (target.lan ? '本机 / 局域网' : '默认规则') : '',
      ...(byRule ? { ruleId: target.lan ? RULE_ID_LAN : RULE_ID_DEFAULT } : {}),
      action: direct ? 'direct' : 'proxy',
      outboundTag: outbound,
      chain: [],
      up: 0,
      down: 0,
      startAt: Date.now(),
    };
    const dest = `${conn.host}:${conn.port}`;
    if (!target.lan) log('debug', `dns: exchanged ${conn.host} A`);
    log('info', `inbound/mixed[mixed-in]: inbound connection to ${dest}`);
    log('info', `outbound/${direct ? 'direct[direct]' : `proxy[${outbound}]`}: outbound connection to ${dest}`);
    return conn;
  }

  function tick(): void {
    const current = world.connections;
    // 偶尔结束一条，数量不足时补新连接。
    const ending = current.length > 4 && Math.random() < 0.35 ? pick(current) : null;
    if (ending !== null) log('debug', `router: connection to ${ending.host}:${ending.port} closed`);
    const kept = current.filter((conn) => conn !== ending).map(grow);
    const room = Math.min(MAX_CONNECTIONS, TARGET_CONNECTIONS + Math.round(Math.random() * 4)) - kept.length;
    const opened = Array.from({ length: Math.max(0, Math.min(room, 3)) }, openConnection);
    if (Math.random() < 0.04) log('warn', 'outbound/proxy: connection upload closed: i/o timeout (demo)');
    const next = [...kept, ...opened];
    world.setConnections(next);
    if (!connsPaused) world.emit(EVENT_CONNECTIONS, { items: next, added: [], updated: [], removed: [] });
    flushLogs();
  }

  world.onTick(tick);
  world.onCoreChange((running) => {
    // world 已经清空了连接。日志保留，停止后仍然可以查看。
    if (running) log('info', 'sing-box started (demo)');
    else log('info', 'sing-box closed (demo)');
    if (!connsPaused) world.emit(EVENT_CONNECTIONS, { items: [], added: [], updated: [], removed: [] });
    flushLogs();
  });
  world.onReset(() => {
    world.setConnections([]);
  });

  return {
    async connsSnapshot() {
      return [...world.connections];
    },

    async connsClose(id) {
      if (id === '') throw rejected('请选择连接');
      world.setConnections(world.connections.filter((conn) => conn.id !== id));
    },

    async connsCloseAll() {
      world.setConnections([]);
    },

    async connsPause(on) {
      connsPaused = on;
    },

    async logsSnapshot(limit) {
      if (!Number.isInteger(limit) || limit < 1 || limit > LOG_BUFFER_MAX) {
        throw rejected('日志数量须在 1 到 5000 之间');
      }
      return logBuffer.slice(-limit);
    },

    async logsPause(on) {
      logsPaused = on;
    },

    async logsSetLevel(level) {
      if (LEVEL_RANK[level] === undefined) throw rejected('日志级别无效');
      world.setSettings({ ...world.settings, logLevel: level });
    },

    async logsExport() {
      throw unsupported('演示模式不读写本机文件，无法导出日志');
    },
  };
}

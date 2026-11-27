/*
 * 日志页的纯函数：环形缓冲的合并、级别与搜索筛选。
 *
 * 日志有两个来源：Logs.Snapshot（一次取回最近若干条）和 logs:entry 事件（实时小批量）。
 * 两者会重叠，暂停期间的日志还只能靠快照补回，所以统一按 seq 合并去重，而不是简单地往后追加。
 */

import type { LogEntry } from './api';

/** 界面里最多保留的日志条数。 */
export const LOG_RING_MAX = 2000;
/** 一次最多渲染的行数；更早的仍在缓冲里，可以靠搜索和级别筛出来。 */
export const LOG_ROWS_MAX = 1000;

const MISSING = '—';

export type LevelFilter = 'debug' | 'info' | 'warn' | 'error';
export type LevelClass = 'debug' | 'info' | 'warn' | 'error';

export const LEVEL_FILTERS: ReadonlyArray<{ readonly value: LevelFilter; readonly label: string }> = [
  { value: 'debug', label: 'Debug' },
  { value: 'info', label: 'Info' },
  { value: 'warn', label: 'Warn' },
  { value: 'error', label: 'Error' },
];

const RANKS: Readonly<Record<string, number>> = {
  trace: 0,
  debug: 1,
  info: 2,
  warn: 3,
  warning: 3,
  error: 4,
  fatal: 5,
  panic: 6,
};

const INFO_RANK = 2;

/** 筛选档位对应的最低级别；Debug 档显示全部，包括 trace。 */
const FILTER_FLOOR: Readonly<Record<LevelFilter, number>> = { debug: 0, info: 2, warn: 3, error: 4 };

/** 级别的轻重；不认识的级别按 info 对待，既不藏起来也不标红。 */
export function levelRank(level: string): number {
  return RANKS[level.toLowerCase()] ?? INFO_RANK;
}

/** 着色用的类别。 */
export function levelClass(level: string): LevelClass {
  const rank = levelRank(level);
  if (rank <= 1) return 'debug';
  if (rank === 2) return 'info';
  return rank === 3 ? 'warn' : 'error';
}

export function levelLabel(level: string): string {
  if (level === '') return MISSING;
  const upper = level.toUpperCase();
  return upper === 'WARNING' ? 'WARN' : upper;
}

export function lastSeq(entries: readonly LogEntry[]): number {
  return entries.length === 0 ? 0 : entries[entries.length - 1].seq;
}

/**
 * 把新到的日志并入缓冲：只收 seq 大于 floor（「清空显示」时记下的位置）的条目，
 * 按 seq 去重、升序排列，只留最新的 max 条。没有新内容时原样返回 ring。
 */
export function mergeEntries(
  ring: readonly LogEntry[],
  incoming: readonly LogEntry[],
  floor: number,
  max: number = LOG_RING_MAX,
): readonly LogEntry[] {
  const known = new Set(ring.map((item) => item.seq));
  const fresh = new Map<number, LogEntry>();
  for (const item of incoming) {
    if (item.seq > floor && !known.has(item.seq)) fresh.set(item.seq, item);
  }
  if (fresh.size === 0) return ring;
  const merged = [...ring, ...fresh.values()].sort((a, b) => a.seq - b.seq);
  return merged.length > max ? merged.slice(merged.length - max) : merged;
}

/** 最低级别与搜索同时生效；搜索不分大小写，匹配级别和正文。 */
export function filterLogs(entries: readonly LogEntry[], minLevel: LevelFilter, query: string): LogEntry[] {
  const floor = FILTER_FLOOR[minLevel];
  const needle = query.trim().toLowerCase();
  return entries.filter(
    (item) =>
      levelRank(item.level) >= floor &&
      (needle === '' || `${item.level}\n${item.message}`.toLowerCase().includes(needle)),
  );
}

function pad2(value: number): string {
  return value < 10 ? `0${value}` : String(value);
}

/** 导出时的默认文件名。后端要求目标文件尚不存在，所以带上本地时间戳。 */
export function exportFileName(now: number): string {
  const date = new Date(now);
  const day = `${date.getFullYear()}${pad2(date.getMonth() + 1)}${pad2(date.getDate())}`;
  const clock = `${pad2(date.getHours())}${pad2(date.getMinutes())}${pad2(date.getSeconds())}`;
  return `ferry-log-${day}-${clock}.log`;
}

export interface TailRows {
  readonly rows: readonly LogEntry[];
  /** 更早的、没有渲染出来的条数。 */
  readonly hidden: number;
}

export function tailRows(entries: readonly LogEntry[], max: number = LOG_ROWS_MAX): TailRows {
  if (entries.length <= max) return { rows: entries, hidden: 0 };
  return { rows: entries.slice(entries.length - max), hidden: entries.length - max };
}

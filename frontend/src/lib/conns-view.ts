/*
 * 连接页的纯函数：筛选、排序、速率与展示文字。
 *
 * 后端给的 up / down 是累计字节数，没有每条连接的速率；
 * 速率在这里用相邻两次快照的差值除以间隔算出来，算不出来的（新连接、刚恢复刷新）一律当作未知，不显示成 0。
 */

import type { Connection } from './api';
import { formatRate } from './format';
import { t } from './i18n/locale';

export type ActionFilter = 'all' | 'proxy' | 'direct' | 'block';
export type ActionKind = 'proxy' | 'direct' | 'block' | 'other';

export const ACTION_FILTERS: ReadonlyArray<{ readonly value: ActionFilter; readonly label: string }> = [
  { value: 'all', label: '全部' },
  { value: 'proxy', label: '代理' },
  { value: 'direct', label: '直连' },
  { value: 'block', label: '拦截' },
];

export function actionFilterOptions(): ReadonlyArray<{ readonly value: ActionFilter; readonly label: string }> {
  return [
    { value: 'all', label: t('全部', 'All') },
    { value: 'proxy', label: t('代理', 'Proxy') },
    { value: 'direct', label: t('直连', 'Direct') },
    { value: 'block', label: t('拦截', 'Block') },
  ];
}

/** 表格最多渲染的行数；再多既看不过来，也会拖慢每秒一次的刷新。 */
export const MAX_ROWS = 300;

const MISSING = '—';
const SECOND = 1000;

export interface ConnRates {
  /** 字节/秒。 */
  readonly up: number;
  readonly down: number;
}

export type RateMap = ReadonlyMap<string, ConnRates>;

interface Totals {
  readonly up: number;
  readonly down: number;
}

/** 某一时刻各连接的累计字节数，留给下一次快照做差。 */
export interface ConnSample {
  readonly at: number;
  readonly totals: ReadonlyMap<string, Totals>;
}

export function sampleOf(list: readonly Connection[], at: number): ConnSample {
  return { at, totals: new Map(list.map((item) => [item.id, { up: item.up, down: item.down }])) };
}

/**
 * 两次快照离得太近、不值得重新采样时用：只给样本里还没有的连接补一个基准，采样时刻不变。
 * 这样新出现的连接到下一次快照就能算出速率；没有新连接时原样返回。
 */
export function extendSample(sample: ConnSample, list: readonly Connection[]): ConnSample {
  const added = list.filter((item) => !sample.totals.has(item.id));
  if (added.length === 0) return sample;
  const totals = new Map(sample.totals);
  for (const item of added) totals.set(item.id, { up: item.up, down: item.down });
  return { at: sample.at, totals };
}

/** 每条连接的速率。上一次快照里没有的连接不出现在结果里（速率未知）。 */
export function ratesBetween(previous: ConnSample | null, list: readonly Connection[], at: number): RateMap {
  const rates = new Map<string, ConnRates>();
  if (previous === null) return rates;
  const seconds = (at - previous.at) / SECOND;
  if (!Number.isFinite(seconds) || seconds <= 0) return rates;
  for (const item of list) {
    const before = previous.totals.get(item.id);
    if (before === undefined) continue;
    rates.set(item.id, {
      up: Math.max(0, item.up - before.up) / seconds,
      down: Math.max(0, item.down - before.down) / seconds,
    });
  }
  return rates;
}

/** 全部连接的合计速率；一条都算不出来时返回 null。 */
export function totalRates(rates: RateMap): ConnRates | null {
  if (rates.size === 0) return null;
  let up = 0;
  let down = 0;
  for (const rate of rates.values()) {
    up += rate.up;
    down += rate.down;
  }
  return { up, down };
}

export function rateText(bytesPerSecond: number | undefined): string {
  return bytesPerSecond === undefined ? MISSING : formatRate(bytesPerSecond);
}

function baseName(path: string): string {
  return path.split(/[\\/]/).findLast((part) => part !== '') ?? '';
}

export function appName(conn: Connection): string {
  return conn.process || baseName(conn.processPath) || t('未知应用', 'Unknown App');
}

/** 头像里的首字。 */
export function appInitial(conn: Connection): string {
  const name = conn.process || baseName(conn.processPath);
  const first = [...name][0];
  return first === undefined ? '?' : first.toUpperCase();
}

/** 头像底色的色相（0–359）：按应用名算出来，同一个应用总是同一种颜色。 */
export function appHue(conn: Connection): number {
  let hash = 0;
  for (const char of appName(conn).toLowerCase()) {
    hash = (hash * 31 + (char.codePointAt(0) ?? 0)) % 360;
  }
  return hash;
}

export function targetText(conn: Connection): string {
  if (conn.host === '') return MISSING;
  const host = conn.host.includes(':') ? `[${conn.host}]` : conn.host;
  return conn.port > 0 ? `${host}:${conn.port}` : host;
}

export function ruleText(conn: Connection): string {
  return conn.ruleName || MISSING;
}

export function actionKind(action: string): ActionKind {
  return action === 'proxy' || action === 'direct' || action === 'block' ? action : 'other';
}

/** 后端给出未知的动作时原样显示，不猜。 */
export function actionLabel(action: string): string {
  if (action === 'proxy') return t('代理', 'Proxy');
  if (action === 'direct') return t('直连', 'Direct');
  if (action === 'block') return t('拦截', 'Block');
  return action || MISSING;
}

/**
 * 代理连接经过的出站（多跳用箭头连起来）；直连、拦截没有出站可言，返回空串。
 * names：出站 tag（节点 id、分组 id）→ 名称；内核给的是内部 tag，认得出的换成名称，认不出的原样显示。
 */
export function outboundText(conn: Connection, names?: ReadonlyMap<string, string>): string {
  if (actionKind(conn.action) !== 'proxy') return '';
  const label = (tag: string): string => names?.get(tag) ?? tag;
  return conn.chain.length > 1 ? conn.chain.map(label).join(' → ') : label(conn.outboundTag || conn.chain[0] || '');
}

function haystack(conn: Connection): string {
  return [conn.process, conn.processPath, conn.host, targetText(conn), conn.ruleName, conn.outboundTag, ...conn.chain]
    .join('\n')
    .toLowerCase();
}

export function filterConnections(list: readonly Connection[], query: string, action: ActionFilter): Connection[] {
  const needle = query.trim().toLowerCase();
  return list.filter(
    (item) => (action === 'all' || item.action === action) && (needle === '' || haystack(item).includes(needle)),
  );
}

/** 新连接在前。 */
export function sortConnections(list: readonly Connection[]): Connection[] {
  return [...list].sort((a, b) => b.startAt - a.startAt || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
}

export interface CappedRows {
  readonly rows: readonly Connection[];
  /** 没有渲染出来的条数。 */
  readonly hidden: number;
}

export function capRows(list: readonly Connection[], max: number = MAX_ROWS): CappedRows {
  if (list.length <= max) return { rows: list, hidden: 0 };
  return { rows: list.slice(0, max), hidden: list.length - max };
}

function pad2(value: number): string {
  return value < 10 ? `0${value}` : String(value);
}

/** 连接已持续的时间：05:07，超过一小时是 1:02:05。 */
export function durationText(startAt: number, now: number): string {
  if (!Number.isFinite(startAt) || startAt <= 0 || !Number.isFinite(now)) return MISSING;
  const total = Math.max(0, Math.floor((now - startAt) / SECOND));
  const hours = Math.floor(total / 3600);
  const clock = `${pad2(Math.floor((total % 3600) / 60))}:${pad2(total % 60)}`;
  return hours > 0 ? `${hours}:${clock}` : clock;
}

/** 从子域名提取泛域名通配符（例如 api2.cursor.sh -> *.cursor.sh）；IP 或单段域名返回空串。 */
export function wildcardDomainOf(host: string): string {
  const trimmed = host.trim().toLowerCase().replace(/\.$/, '');
  if (trimmed === '' || trimmed.includes(':') || /^\d+(\.\d+){3}$/.test(trimmed)) return '';
  const parts = trimmed.split('.');
  if (parts.length < 2) return '';
  if (parts.length === 2) return `*.${trimmed}`;
  // 常见二级后缀如 .com.cn / .co.jp / .org.cn 保留三段。
  const tail2 = `${parts[parts.length - 2]}.${parts[parts.length - 1]}`;
  const multiTld = ['com.cn', 'net.cn', 'org.cn', 'gov.cn', 'edu.cn', 'co.uk', 'co.jp', 'com.hk', 'com.tw'];
  if (parts.length >= 3 && multiTld.includes(tail2)) {
    return `*.${parts.slice(-3).join('.')}`;
  }
  return `*.${parts.slice(-2).join('.')}`;
}

/** 推荐的「纠偏」动作：当前走直连时建议改代理，当前走代理时建议改直连。 */
export function suggestedActionFor(conn: Connection): 'proxy' | 'direct' {
  return conn.action === 'proxy' ? 'direct' : 'proxy';
}

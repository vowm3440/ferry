/*
 * 时间的展示格式。时间戳一律是 Unix 毫秒，按本机时区显示。
 * 后端用 0 表示「没有这个时间」，这里统一显示成「—」，不显示 1970 年。
 */

import { t } from './i18n/locale';

const MISSING = '—';
const SECOND = 1000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

function pad2(value: number): string {
  return value < 10 ? `0${value}` : String(value);
}

function toDate(ms: number): Date | null {
  if (!Number.isFinite(ms) || ms <= 0) return null;
  const date = new Date(ms);
  return Number.isNaN(date.getTime()) ? null : date;
}

function dateText(date: Date): string {
  return `${date.getFullYear()}-${pad2(date.getMonth() + 1)}-${pad2(date.getDate())}`;
}

/** 09:05:07，用于日志这类按秒排列的列表。 */
export function formatClock(ms: number): string {
  const date = toDate(ms);
  if (date === null) return MISSING;
  return `${pad2(date.getHours())}:${pad2(date.getMinutes())}:${pad2(date.getSeconds())}`;
}

/** 2026-01-02 03:04 */
export function formatDateTime(ms: number): string {
  const date = toDate(ms);
  if (date === null) return MISSING;
  return `${dateText(date)} ${pad2(date.getHours())}:${pad2(date.getMinutes())}`;
}

/** 2026-01-02 */
export function formatDate(ms: number): string {
  const date = toDate(ms);
  return date === null ? MISSING : dateText(date);
}

/** 倒计时 09:41。剩余不足一秒时向上取整，真正到点才显示 00:00。 */
export function formatCountdown(remainingMs: number): string {
  if (!Number.isFinite(remainingMs) || remainingMs <= 0) return '00:00';
  const total = Math.ceil(remainingMs / SECOND);
  return `${pad2(Math.floor(total / 60))}:${pad2(total % 60)}`;
}

/** 「3 分钟前」。then 缺失时显示「—」；比 now 还晚（两端时钟有偏差）时按「刚刚」处理。 */
export function formatAgo(then: number, now: number): string {
  if (toDate(then) === null) return MISSING;
  const elapsed = now - then;
  if (!Number.isFinite(elapsed) || elapsed < MINUTE) return t('刚刚', 'Just now');
  if (elapsed < HOUR) return t(`${Math.floor(elapsed / MINUTE)} 分钟前`, `${Math.floor(elapsed / MINUTE)}m ago`);
  if (elapsed < DAY) return t(`${Math.floor(elapsed / HOUR)} 小时前`, `${Math.floor(elapsed / HOUR)}h ago`);
  return t(`${Math.floor(elapsed / DAY)} 天前`, `${Math.floor(elapsed / DAY)}d ago`);
}

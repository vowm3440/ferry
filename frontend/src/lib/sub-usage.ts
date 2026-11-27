/*
 * 订阅的流量与到期信息怎么展示。订阅页的卡片与概览页的用量卡共用，口径只有这一处。
 *
 * 这些数字来自订阅服务器随响应给出的用量信息；服务器没给时后端填 0，
 * 界面一律显示「—」，不把「不知道」画成 0%。
 */

import { formatBytes } from './format';
import { t } from './i18n/locale';
import { formatDate } from './time';

export type UsageLevel = 'ok' | 'warn' | 'err';

const MISSING = '—';
const DAY_MS = 24 * 60 * 60 * 1000;
/** 用量达到这个百分比开始提醒 / 告警。 */
const USAGE_WARN_PERCENT = 80;
const USAGE_ERR_PERCENT = 95;
/** 剩余天数少于这个值开始提醒。 */
const EXPIRY_WARN_DAYS = 7;

export interface UsageView {
  /** 一点流量信息都没有。 */
  readonly empty: boolean;
  /** 知道总量，可以画进度条。 */
  readonly hasTotal: boolean;
  readonly usedText: string;
  readonly totalText: string;
  /** 0–100，进度条的宽度。 */
  readonly percent: number;
  /** 「42%」；不知道总量时为空串。 */
  readonly percentText: string;
  readonly level: UsageLevel;
}

export interface ExpiryView {
  readonly known: boolean;
  /** 「2026-12-31」；不知道时是「—」。 */
  readonly dateText: string;
  /** 「还有 12 天」「不足 1 天」「已过期」；不知道时为空串。 */
  readonly remainText: string;
  readonly level: UsageLevel;
}

function positive(value: number): number {
  return Number.isFinite(value) && value > 0 ? value : 0;
}

function usageLevel(percent: number): UsageLevel {
  if (percent >= USAGE_ERR_PERCENT) return 'err';
  if (percent >= USAGE_WARN_PERCENT) return 'warn';
  return 'ok';
}

export function usageView(usedBytes: number, totalBytes: number): UsageView {
  const used = positive(usedBytes);
  const total = positive(totalBytes);
  if (total === 0) {
    return {
      empty: used === 0,
      hasTotal: false,
      usedText: used === 0 ? MISSING : formatBytes(used),
      totalText: MISSING,
      percent: 0,
      percentText: '',
      level: 'ok',
    };
  }
  // 向下取整：没用满就不显示 100%。
  const percent = Math.min(100, Math.floor((used / total) * 100));
  return {
    empty: false,
    hasTotal: true,
    usedText: formatBytes(used),
    totalText: formatBytes(total),
    percent,
    percentText: `${percent}%`,
    level: usageLevel(percent),
  };
}

/** expiresAt、now 都是 Unix 毫秒。 */
export function expiryView(expiresAt: number, now: number): ExpiryView {
  if (positive(expiresAt) === 0) return { known: false, dateText: MISSING, remainText: '', level: 'ok' };

  const dateText = formatDate(expiresAt);
  const remaining = expiresAt - now;
  if (!(remaining > 0)) return { known: true, dateText, remainText: t('已过期', 'Expired'), level: 'err' };

  const days = Math.floor(remaining / DAY_MS);
  if (days < 1) return { known: true, dateText, remainText: t('不足 1 天', '< 1 day'), level: 'err' };
  return {
    known: true,
    dateText,
    remainText: t(`还有 ${days} 天`, `${days} days left`),
    level: days < EXPIRY_WARN_DAYS ? 'warn' : 'ok',
  };
}

/**
 * 概览页「订阅用量」卡片的取值（纯函数，可单测）。
 * 用量与到期的口径全部来自 sub-usage.ts，这里只负责拼成一行一行的展示数据。
 */
import type { Subscription } from './api';
import { t } from './i18n/locale';
import { expiryView, usageView, type UsageLevel } from './sub-usage';

const MISSING = '—';

export interface UsageRow {
  id: string;
  name: string;
  /** 「42.0 GB / 100 GB」「已用 3.00 GB」或「—」。 */
  usageText: string;
  /** 只有知道总量时才画进度条。 */
  hasBar: boolean;
  percent: number;
  percentText: string;
  level: UsageLevel;
  expiryText: string;
  expiryRemain: string;
  expiryLevel: UsageLevel;
  nodesText: string;
  autoText: string;
}

export interface UsageSummary {
  rows: UsageRow[];
  /** 超出上限、没有列出的订阅数量。 */
  hidden: number;
}

function autoText(sub: Subscription): string {
  // 手动导入的内容没有链接，谈不上自动更新。
  if (sub.urlMasked === '') return MISSING;
  if (!sub.autoUpdate) return t('已关闭', 'Off');
  return sub.intervalHours > 0 ? t(`每 ${sub.intervalHours} 小时`, `Every ${sub.intervalHours}h`) : MISSING;
}

function toRow(sub: Subscription, now: number): UsageRow {
  const usage = usageView(sub.usedBytes, sub.totalBytes);
  const expiry = expiryView(sub.expiresAt, now);
  let usageText = MISSING;
  if (usage.hasTotal) usageText = `${usage.usedText} / ${usage.totalText}`;
  else if (!usage.empty) usageText = t(`已用 ${usage.usedText}`, `Used ${usage.usedText}`);
  return {
    id: sub.id,
    name: sub.name,
    usageText,
    hasBar: usage.hasTotal,
    percent: usage.percent,
    percentText: usage.percentText,
    level: usage.level,
    expiryText: expiry.dateText,
    expiryRemain: expiry.remainText,
    expiryLevel: expiry.level,
    nodesText: String(sub.nodeCount),
    autoText: autoText(sub),
  };
}

export function usageRows(subs: readonly Subscription[], now: number, limit: number): UsageSummary {
  const shown = subs.slice(0, Math.max(0, limit));
  return { rows: shown.map((sub) => toRow(sub, now)), hidden: subs.length - shown.length };
}

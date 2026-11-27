/* 数值格式化。仅做展示，不改变数据本身。 */

import { t } from './i18n/locale';

const KB = 1024;
const UNITS = ['KB', 'MB', 'GB', 'TB'] as const;
/** 四舍五入后会显示成 1024 的数值改用更大的单位（1024 KB → 1.00 MB）。 */
const PROMOTE_AT = KB - 0.5;

/** 位数随数值收窄；分界取四舍五入的进位点，不会出现「10.00」「100.0」。 */
function decimal(value: number): string {
  if (value >= 99.95) return value.toFixed(0);
  if (value >= 9.995) return value.toFixed(1);
  return value.toFixed(2);
}

/** 字节数：1024 进制，最多两位小数。 */
export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  if (bytes < PROMOTE_AT) return `${Math.round(bytes)} B`;
  let value = bytes / KB;
  let unit = 0;
  while (value >= PROMOTE_AT && unit < UNITS.length - 1) {
    value /= KB;
    unit += 1;
  }
  return `${decimal(value)} ${UNITS[unit]}`;
}

/** 速率：每秒字节数。 */
export function formatRate(bytesPerSecond: number): string {
  return `${formatBytes(bytesPerSecond)}/s`;
}

export interface ValueWithUnit {
  readonly value: string;
  readonly unit: string;
}

/** 把「46.2 MB」拆成数值与单位，供「大号数字 + 小号单位」的排版使用。 */
export function splitUnit(text: string): ValueWithUnit {
  const index = text.indexOf(' ');
  if (index < 0) return { value: text, unit: '' };
  return { value: text.slice(0, index), unit: text.slice(index + 1) };
}

/** 计数：千分位。 */
export function formatCount(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '0';
  return Math.round(value).toLocaleString('en-US');
}

function pad2(value: number): string {
  return value < 10 ? `0${value}` : String(value);
}

/** 运行时长：03:04:05，超过一天时前置「N 天」。 */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return '00:00:00';
  const total = Math.floor(ms / 1000);
  const days = Math.floor(total / 86400);
  const hours = Math.floor((total % 86400) / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  const clock = `${pad2(hours)}:${pad2(minutes)}:${pad2(seconds)}`;
  return days > 0 ? t(`${days} 天 ${clock}`, `${days}d ${clock}`) : clock;
}

/** 运行时长（口语）：2 小时 5 分。用于副标题一类的位置。 */
export function formatDurationText(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return t('0 秒', '0s');
  const total = Math.floor(ms / 1000);
  const days = Math.floor(total / 86400);
  const hours = Math.floor((total % 86400) / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = total % 60;
  if (days > 0) return t(`${days} 天 ${hours} 小时`, `${days}d ${hours}h`);
  if (hours > 0) return t(`${hours} 小时 ${minutes} 分`, `${hours}h ${minutes}m`);
  if (minutes > 0) return t(`${minutes} 分 ${seconds} 秒`, `${minutes}m ${seconds}s`);
  return t(`${seconds} 秒`, `${seconds}s`);
}

/** 延迟：-1 或负数表示尚未测速。 */
export function formatLatency(latencyMs: number): string {
  if (!Number.isFinite(latencyMs) || latencyMs < 0) return t('未测速', 'Untested');
  return `${Math.round(latencyMs)} ms`;
}

/** 延迟档位：用于 .lat 的颜色类。 */
export function latencyLevel(latencyMs: number): 'ok' | 'warn' | 'err' | 'none' {
  if (!Number.isFinite(latencyMs) || latencyMs < 0) return 'none';
  if (latencyMs < 200) return 'ok';
  if (latencyMs < 500) return 'warn';
  return 'err';
}

/*
 * 设置页的纯函数与选项表。界面只负责把值交给 Settings.Set()，
 * 校验的权威描述留在 Go 侧（internal/app/settings.go），这里只做必要的输入整形
 * 与「是否有未保存修改」的比较。
 */

import type { SettingsValue } from './api';
import { cloneSettings } from './api';
import { isRecord } from './guard';
import { t } from './i18n/locale';

export interface SelectOption {
  readonly value: string;
  readonly label: string;
}

export const DNS_STRATEGY_OPTIONS: readonly SelectOption[] = [
  { value: 'prefer_ipv4', label: '优先 IPv4' },
  { value: 'prefer_ipv6', label: '优先 IPv6' },
  { value: 'ipv4_only', label: '仅 IPv4' },
  { value: 'ipv6_only', label: '仅 IPv6' },
  { value: 'as_is', label: '保持原样' },
];

export const LOG_LEVEL_OPTIONS: readonly SelectOption[] = [
  { value: 'trace', label: 'trace · 最详细' },
  { value: 'debug', label: 'debug · 调试' },
  { value: 'info', label: 'info · 常规' },
  { value: 'warn', label: 'warn · 警告' },
  { value: 'error', label: 'error · 错误' },
  { value: 'fatal', label: 'fatal · 致命' },
  { value: 'panic', label: 'panic · 崩溃' },
];

export function dnsStrategyOptions(): readonly SelectOption[] {
  return [
    { value: 'prefer_ipv4', label: t('优先 IPv4', 'Prefer IPv4') },
    { value: 'prefer_ipv6', label: t('优先 IPv6', 'Prefer IPv6') },
    { value: 'ipv4_only', label: t('仅 IPv4', 'IPv4 Only') },
    { value: 'ipv6_only', label: t('仅 IPv6', 'IPv6 Only') },
    { value: 'as_is', label: t('保持原样', 'As Is') },
  ];
}

export function logLevelOptions(): readonly SelectOption[] {
  return [
    { value: 'trace', label: t('trace · 最详细', 'trace · Verbose') },
    { value: 'debug', label: t('debug · 调试', 'debug · Debug') },
    { value: 'info', label: t('info · 常规', 'info · Info') },
    { value: 'warn', label: t('warn · 警告', 'warn · Warning') },
    { value: 'error', label: t('error · 错误', 'error · Error') },
    { value: 'fatal', label: t('fatal · 致命', 'fatal · Fatal') },
    { value: 'panic', label: t('panic · 崩溃', 'panic · Panic') },
  ];
}

/** 选项里没有当前值时补一条，避免下拉框显示成别的内容。 */
export function optionsWith(options: readonly SelectOption[], current: string): SelectOption[] {
  if (current === '' || options.some((option) => option.value === current)) return [...options];
  return [...options, { value: current, label: current }];
}

export function optionLabel(options: readonly SelectOption[], value: string): string {
  return options.find((option) => option.value === value)?.label ?? value;
}

/** 解析端口输入；空串或非法值返回 null。范围与后端一致（1–65535）。 */
export function parsePort(text: string): number | null {
  const trimmed = text.trim();
  if (!/^\d{1,5}$/.test(trimmed)) return null;
  const value = Number.parseInt(trimmed, 10);
  return value >= 1 && value <= 65535 ? value : null;
}

/** 新建订阅默认的自动更新间隔（小时）。后端允许 1–720，这里列常用值。 */
const UPDATE_HOURS_PRESETS: readonly number[] = [1, 3, 6, 12, 24, 48, 72, 168];
const MIN_UPDATE_HOURS = 1;
const MAX_UPDATE_HOURS = 720;
const MAX_USER_AGENT_BYTES = 256;

function hoursLabel(hours: number): string {
  return hours === 168 ? t('每 7 天', 'Every 7d') : t(`每 ${hours} 小时`, `Every ${hours}h`);
}

function validHours(hours: number): boolean {
  return Number.isInteger(hours) && hours >= MIN_UPDATE_HOURS && hours <= MAX_UPDATE_HOURS;
}

/** 间隔下拉框的选项；当前值不在常用值里（比如由开发配置导入）时补一条。 */
export function updateHoursOptions(current: number): SelectOption[] {
  const hours = validHours(current) && !UPDATE_HOURS_PRESETS.includes(current) ? [...UPDATE_HOURS_PRESETS, current] : UPDATE_HOURS_PRESETS;
  return hours.map((value) => ({ value: String(value), label: hoursLabel(value) }));
}

export function parseUpdateHours(text: string): number | null {
  if (!/^\d{1,3}$/.test(text.trim())) return null;
  const value = Number.parseInt(text.trim(), 10);
  return validHours(value) ? value : null;
}

/** 后端按字节数限制 User-Agent 的长度；超出时给出说明，否则返回空串。 */
export function userAgentError(text: string): string {
  return new TextEncoder().encode(text).length > MAX_USER_AGENT_BYTES
    ? t(`User-Agent 不能超过 ${MAX_USER_AGENT_BYTES} 字节`, `User-Agent must not exceed ${MAX_USER_AGENT_BYTES} bytes`)
    : '';
}

export const DEFAULT_TUN_EXCLUDES: readonly string[] = [
  '10.0.0.0/8',
  '100.64.0.0/10',
  '127.0.0.0/8',
  '169.254.0.0/16',
  '172.16.0.0/12',
  '192.168.0.0/16',
  '224.0.0.0/4',
  '::1/128',
  'fc00::/7',
  'fe80::/10',
  'ff00::/8',
];

const MAX_TUN_EXCLUDES = 256;

function isValidIpv4(addr: string): boolean {
  const parts = addr.split('.');
  if (parts.length !== 4) return false;
  return parts.every((part) => /^(?:0|[1-9]\d{0,2})$/.test(part) && Number(part) <= 255);
}

function isValidIpv6(addr: string): boolean {
  if (addr.includes('%') || addr.includes('.')) return false;
  const halves = addr.split('::');
  if (halves.length > 2) return false;
  const left = halves[0] === '' ? [] : halves[0]!.split(':');
  const right = halves.length === 2 && halves[1] !== '' ? halves[1]!.split(':') : [];
  const groups = [...left, ...right];
  if (!groups.every((g) => /^[0-9a-fA-F]{1,4}$/.test(g))) return false;
  return halves.length === 2 ? groups.length < 8 : groups.length === 8;
}

/** 校验单条 IPv4/IPv6 CIDR（对齐 Go 侧 spec.validateTun）；合法返回空串。 */
export function checkCidr(raw: string): string {
  const text = raw.trim();
  const slash = text.indexOf('/');
  if (slash <= 0 || slash !== text.lastIndexOf('/')) {
    return t('请输入有效的 IPv4 或 IPv6 CIDR（如 192.168.0.0/16）', 'Enter a valid IPv4 or IPv6 CIDR (e.g. 192.168.0.0/16)');
  }
  const ip = text.slice(0, slash);
  const prefixText = text.slice(slash + 1);
  if (!/^(?:0|[1-9]\d{0,2})$/.test(prefixText)) {
    return t('CIDR 掩码位数无效', 'Invalid CIDR prefix length');
  }
  const bits = Number(prefixText);
  if (isValidIpv4(ip)) {
    return bits <= 32 ? '' : t('IPv4 CIDR 掩码须在 0–32 之间', 'IPv4 CIDR prefix must be 0–32');
  }
  if (isValidIpv6(ip)) {
    return bits <= 128 ? '' : t('IPv6 CIDR 掩码须在 0–128 之间', 'IPv6 CIDR prefix must be 0–128');
  }
  return t('请输入有效的 IPv4 或 IPv6 CIDR（如 192.168.0.0/16）', 'Enter a valid IPv4 or IPv6 CIDR (e.g. 192.168.0.0/16)');
}

export function normalizeCidrList(lines: readonly string[]): { readonly cidrs: string[]; readonly error: string } {
  const cidrs: string[] = [];
  const seen = new Set<string>();
  for (const raw of lines) {
    const trimmed = raw.trim();
    if (trimmed === '' || seen.has(trimmed)) continue;
    const problem = checkCidr(trimmed);
    if (problem !== '') return { cidrs: [], error: t(`${trimmed}：${problem}`, `${trimmed}: ${problem}`) };
    seen.add(trimmed);
    cidrs.push(trimmed);
  }
  if (cidrs.length > MAX_TUN_EXCLUDES) {
    return { cidrs: [], error: t(`TUN 排除网段最多支持 ${MAX_TUN_EXCLUDES} 条`, `Maximum ${MAX_TUN_EXCLUDES} excluded CIDRs`) };
  }
  return { cidrs, error: '' };
}

/**
 * 后端尚未开放的自动化开关（Settings.Set 会以「这些自动化功能尚未开放」拒绝）。
 * 界面上不可操作，提交时再兜底一次，保证不会以 true 发出去。
 */
export function withAutomationOff(value: SettingsValue): SettingsValue {
  return { ...value, autoStart: false, startMinimized: false, lightweight: false, sysProxyOnStart: false, ruleSetAutoUpdate: false };
}

function stable(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(stable);
  if (isRecord(value)) {
    return Object.fromEntries(
      Object.keys(value)
        .sort()
        .map((key) => [key, stable(value[key])]),
    );
  }
  return value;
}

/**
 * 是否有未保存的修改。忽略 theme：主题由前端本地保存、立即生效，
 * 保存设置时会一并写回后端，不该让表单显示成「已被修改」。
 */
export function sameSettings(a: SettingsValue, b: SettingsValue): boolean {
  const strip = (value: SettingsValue): string => JSON.stringify(stable({ ...cloneSettings(value), theme: '' }));
  return strip(a) === strip(b);
}

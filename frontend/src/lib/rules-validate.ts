/*
 * 规则条件的本地校验，逐条照抄 Go 侧（internal/spec/validation_rules.go、appgroups.go、validate.go）。
 *
 * 目的只是在提交前就指出哪一项写错了；后端仍然是最终裁决，它返回的错误会原样显示。
 * 这里放行而后端拒绝的情况不算问题，反过来（这里拒绝、后端其实接受）才是缺陷，所以判断只严不到后端之外。
 */

import type { AppGroup } from './api/types-rules';
import { APP_GROUP_PREFIX } from './api/types-rules';
import { t } from './i18n/locale';

/** 规则总数、单个字段的条件数、应用分组数的上限（internal/spec/types.go）。 */
export const MAX_RULES = 2048;
export const MAX_RULE_ITEMS = 256;
export const MAX_APP_GROUPS = 128;

export interface RuleSetRef {
  /** 写在「目标主机」里的引用。 */
  readonly ref: string;
  /** RuleSets.List() / RuleSets.Update() 用的目录 id。 */
  readonly id: string;
  readonly label: string;
}

/** 后端目录内的全部规则集；不接受任意 URL 或别的名字。 */
export const RULE_SET_REFS: readonly RuleSetRef[] = [
  { ref: 'geosite:cn', id: 'geosite-cn', label: '中国大陆域名' },
  { ref: 'geosite:geolocation-!cn', id: 'geosite-non-cn', label: '非中国大陆域名' },
  { ref: 'geoip:cn', id: 'geoip-cn', label: '中国大陆 IP' },
  { ref: 'geoip:private', id: 'geoip-private', label: '私有网络 IP' },
];

export function ruleSetRefs(): readonly RuleSetRef[] {
  return [
    { ref: 'geosite:cn', id: 'geosite-cn', label: t('中国大陆域名', 'Mainland China Domains') },
    { ref: 'geosite:geolocation-!cn', id: 'geosite-non-cn', label: t('非中国大陆域名', 'Non-CN Domains') },
    { ref: 'geoip:cn', id: 'geoip-cn', label: t('中国大陆 IP', 'Mainland China IPs') },
    { ref: 'geoip:private', id: 'geoip-private', label: t('私有网络 IP', 'Private Network IPs') },
  ];
}

const ID_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$/;
const CONTROL = /\p{Cc}/u;
const DOMAIN_LABEL = /^[A-Za-z0-9_*?-]+$/;
const DIGITS = /^[0-9]+$/;
const encoder = new TextEncoder();

/** Go 的 len() 按 UTF-8 字节计。 */
function byteLength(value: string): number {
  return encoder.encode(value).length;
}

function validText(value: string, limit: number): boolean {
  return value !== '' && byteLength(value) <= limit && value.trim() === value && !CONTROL.test(value);
}

export function isValidId(value: string): boolean {
  return ID_PATTERN.test(value);
}

/** 规则与应用分组的名称。 */
export function isValidLabel(value: string): boolean {
  return value.trim() !== '' && byteLength(value) <= 256 && !CONTROL.test(value);
}

export function isValidRuleApp(app: string): boolean {
  if (!validText(app, 1024) || /["<>|]/.test(app)) return false;
  if (!app.includes(':')) return true;
  // 冒号只能是完整路径里的盘符。
  return app.length > 3 && /^[A-Za-z]:[\\/]/.test(app) && !app.slice(2).includes(':');
}

/** 解析 IPv4，得到 32 位无符号整数；不接受前导零、缺段或越界。 */
function parseIPv4(text: string): bigint | null {
  const parts = text.split('.');
  if (parts.length !== 4) return null;
  let value = 0n;
  for (const part of parts) {
    if (!DIGITS.test(part) || part.length > 3 || (part.length > 1 && part.startsWith('0'))) return null;
    const octet = Number(part);
    if (octet > 255) return null;
    value = (value << 8n) | BigInt(octet);
  }
  return value;
}

/** 把一侧的十六进制分组（末尾可带点分 IPv4）解析成 16 位一组的数字。 */
function parseGroups(text: string, allowTail: boolean): number[] | null {
  if (text === '') return [];
  const parts = text.split(':');
  const groups: number[] = [];
  for (const [index, part] of parts.entries()) {
    if (allowTail && index === parts.length - 1 && part.includes('.')) {
      const tail = parseIPv4(part);
      if (tail === null) return null;
      groups.push(Number(tail >> 16n), Number(tail & 0xffffn));
    } else if (/^[0-9a-fA-F]{1,4}$/.test(part)) {
      groups.push(Number.parseInt(part, 16));
    } else {
      return null;
    }
  }
  return groups;
}

/** 解析 IPv6（不带 zone），得到 128 位无符号整数。 */
function parseIPv6(text: string): bigint | null {
  if (!text.includes(':')) return null;
  const halves = text.split('::');
  if (halves.length > 2) return null;
  const head = parseGroups(halves[0] ?? '', halves.length === 1);
  const tail = halves.length === 2 ? parseGroups(halves[1] ?? '', true) : [];
  if (head === null || tail === null) return null;
  const missing = 8 - head.length - tail.length;
  if (halves.length === 1 ? missing !== 0 : missing < 1) return null;
  const groups = [...head, ...Array.from({ length: missing }, () => 0), ...tail];
  return groups.reduce((value, group) => (value << 16n) | BigInt(group), 0n);
}

interface Address {
  readonly bits: 32 | 128;
  readonly value: bigint;
}

/** IPv4 映射的 IPv6（::ffff:a.b.c.d）后端不接受。 */
function isMapped(value: bigint): boolean {
  return value >> 32n === 0xffffn;
}

function parseAddress(text: string): Address | null {
  const v4 = parseIPv4(text);
  if (v4 !== null) return { bits: 32, value: v4 };
  const v6 = parseIPv6(text);
  if (v6 === null || isMapped(v6)) return null;
  return { bits: 128, value: v6 };
}

function isValidPrefix(host: string): boolean {
  const slash = host.lastIndexOf('/');
  const address = parseAddress(host.slice(0, slash));
  const bits = host.slice(slash + 1);
  if (address === null || !DIGITS.test(bits) || (bits.length > 1 && bits.startsWith('0')) || bits.length > 3) return false;
  return Number(bits) <= address.bits;
}

function isValidDomain(host: string): boolean {
  if (/^[0-9.]*$/.test(host)) return false;
  const trimmed = host.endsWith('.') ? host.slice(0, -1) : host;
  return trimmed.split('.').every(
    (label) => label !== '' && label.length <= 63 && !label.startsWith('-') && !label.endsWith('-') && DOMAIN_LABEL.test(label),
  );
}

export function isValidRuleHost(host: string): boolean {
  if (!validText(host, 253)) return false;
  // 带 zone 的地址（fe80::1%eth0）后端不接受；% 也不是合法的域名字符。
  if (host.includes('%')) return false;
  if (parseIPv4(host) !== null) return true;
  const v6 = parseIPv6(host);
  if (v6 !== null) return !isMapped(v6);
  if (host.includes('/')) return isValidPrefix(host);
  const dash = host.indexOf('-');
  if (dash >= 0) {
    const first = parseAddress(host.slice(0, dash));
    const last = parseAddress(host.slice(dash + 1));
    // 任一侧是 IP，或者整体只由数字、点和连字符组成：必须是合法的范围，不再当域名看。
    if (first !== null || last !== null || /^[0-9.-]*$/.test(host)) {
      return first !== null && last !== null && first.bits === last.bits && first.value <= last.value;
    }
  }
  return isValidDomain(host);
}

export function isRuleSetRef(host: string): boolean {
  return RULE_SET_REFS.some((item) => item.ref === host);
}

/** 写成了规则集的样子（geoip: / geosite: 开头，不分大小写）。 */
export function looksLikeRuleSet(host: string): boolean {
  const lower = host.toLowerCase();
  return lower.startsWith('geoip:') || lower.startsWith('geosite:');
}

export function isValidRulePort(value: string): boolean {
  const parts = value.split('-');
  if (parts.length > 2) return false;
  let previous = 0;
  for (const part of parts) {
    if (!DIGITS.test(part)) return false;
    const port = Number(part);
    if (port === 0 || port > 65535 || port < previous) return false;
    previous = port;
  }
  return true;
}

/** 单个应用条件的问题；没有问题时返回空串。文案与后端一致。 */
/** groups 为 null 表示分组列表没读到：只查引用格式，存在与否交给后端判断。 */
export function appProblem(app: string, groups: readonly AppGroup[] | null): string {
  if (!app.startsWith('@')) return isValidRuleApp(app) ? '' : t('规则应用条件无效', 'Invalid application condition');
  if (!app.startsWith(APP_GROUP_PREFIX)) return t('应用分组引用格式无效', 'Invalid app group reference format');
  const id = app.slice(APP_GROUP_PREFIX.length);
  if (!isValidId(id)) return t('应用分组引用格式无效', 'Invalid app group reference format');
  if (groups === null) return '';
  const group = groups.find((item) => item.id === id);
  if (group === undefined) return t('引用的应用分组不存在', 'Referenced app group does not exist');
  return group.apps.length === 0 ? t('引用的应用分组为空或无效', 'Referenced app group is empty or invalid') : '';
}

export function hostProblem(host: string): string {
  if (isRuleSetRef(host)) return '';
  if (looksLikeRuleSet(host)) return t('规则集不在支持的目录内，请选择已列出的规则集', 'Unsupported rule set; choose from the listed catalog');
  return isValidRuleHost(host) ? '' : t('规则主机条件无效或语法不受支持', 'Invalid or unsupported host condition');
}

export function portProblem(port: string): string {
  return isValidRulePort(port) ? '' : t('规则端口必须为 1 到 65535 或递增的端口范围', 'Port must be 1–65535 or an ascending port range');
}

/** 是不是 IP 字面量（IPv4 或不带 zone 的 IPv6）。 */
export function isIpLiteral(text: string): boolean {
  return parseIPv4(text) !== null || parseIPv6(text) !== null;
}

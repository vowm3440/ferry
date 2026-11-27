/*
 * 订阅页与分享对话框的纯逻辑：输入预检、状态文案、二维码容量、局域网分享倒计时。
 * 预检只是为了少跑一趟后端并尽早提示，规则与文案照抄 internal/app；最终以后端的判断为准。
 */
import type { ExportFormat, LanShare, SkippedLine, Subscription } from './api';
import { SUB_STATUS_ERROR, SUB_STATUS_UPDATING } from './api';
import { utf8Length } from './file-pick';
import { t } from './i18n/locale';

const MAX_URL_BYTES = 4096;
const MAX_NAME_RUNES = 64;
const MIN_HOURS = 1;
const MAX_HOURS = 720;
const QR_MAX_BYTES = 4096;
/** 二维码图片在交给后端前的大小上限，与后端一致。 */
export const QR_IMAGE_MAX_BYTES = 8 * 1024 * 1024;

export const MSG_URL_INVALID = '订阅地址须为有效的 HTTP 或 HTTPS 地址，且不能包含用户信息或片段';
export const MSG_HTTP_WARNING = '此订阅使用未加密的 HTTP，地址与节点内容可能被网络中的第三方读取';
export const MSG_NAME_INVALID = '名称无效：最多 64 个字符，且不能包含控制字符';
export const MSG_HOURS = '订阅更新间隔须在 1 到 720 小时之间';
export const MSG_QR_TOO_LARGE = '内容超过二维码容量，请使用文件分享';

export function msgUrlInvalid(): string {
  return t(MSG_URL_INVALID, 'Subscription URL must be a valid HTTP/HTTPS URL without userinfo or fragment');
}

export function msgHttpWarning(): string {
  return t(MSG_HTTP_WARNING, 'This subscription uses plain HTTP; URLs and node details may be exposed on the network');
}

export function msgNameInvalid(): string {
  return t(MSG_NAME_INVALID, 'Invalid name: up to 64 characters without control characters');
}

export function msgHours(): string {
  return t(MSG_HOURS, 'Update interval must be between 1 and 720 hours');
}

export function msgQrTooLarge(): string {
  return t(MSG_QR_TOO_LARGE, 'Content exceeds QR code capacity; please use file sharing');
}

/** 订阅页的导入入口。 */
export type ImportKind = 'url' | 'paste' | 'qr' | 'screen-qr' | 'file';

const CONTROL_CHARS = /\p{Cc}/u;
const SVG_PREFIX = 'data:image/svg+xml;charset=utf-8,';

function parseUrl(raw: string): URL | null {
  try {
    return new URL(raw);
  } catch {
    return null;
  }
}

/** 订阅地址预检：通过返回空串，否则返回给用户看的原因。 */
export function checkSubUrl(raw: string): string {
  const text = raw.trim();
  if (text === '') return t('请填写订阅地址', 'Please enter a subscription URL');
  if (utf8Length(text) > MAX_URL_BYTES) return msgUrlInvalid();
  const url = parseUrl(text);
  if (url === null) return msgUrlInvalid();
  const web = url.protocol === 'http:' || url.protocol === 'https:';
  // WHATWG 解析会把 https:///a 修正成主机 a；后端按 RFC 解析时主机为空，这里按原文再挡一次。
  const emptyHost = url.hostname === '' || /^[a-z]+:\/\/\//i.test(text);
  if (!web || emptyHost || url.username !== '' || url.password !== '' || text.includes('#')) return msgUrlInvalid();
  return '';
}

/** 是否是明文 HTTP 订阅：用于在提交前给出与后端相同的提醒。 */
export function isPlainHttp(raw: string): boolean {
  return parseUrl(raw.trim())?.protocol === 'http:';
}

/** 订阅名称预检：空名称由后端取默认名。 */
export function checkSubName(name: string): string {
  const text = name.trim();
  if ([...text].length > MAX_NAME_RUNES || CONTROL_CHARS.test(text)) return msgNameInvalid();
  return '';
}

/** 解析自动更新间隔（小时）：不是 1–720 的整数时返回 null。 */
export function parseHours(text: string): number | null {
  const trimmed = text.trim();
  if (!/^\d{1,3}$/.test(trimmed)) return null;
  const hours = Number.parseInt(trimmed, 10);
  return hours >= MIN_HOURS && hours <= MAX_HOURS ? hours : null;
}

/** subs:progress 的阶段文案；结束或未知阶段返回空串。 */
export function progressText(phase: string): string {
  if (phase === 'fetch') return t('正在下载…', 'Downloading…');
  if (phase === 'parse') return t('正在解析…', 'Parsing…');
  if (phase === 'apply') return t('正在应用…', 'Applying…');
  return '';
}

export interface PillView {
  readonly text: string;
  /** base.css 里 .pill 的修饰类；空串表示默认样式。 */
  readonly tone: '' | 'proxy' | 'phase' | 'block';
}

/** 带订阅地址的才能更新与自动更新；手动导入的没有地址。 */
export function canUpdate(sub: Subscription): boolean {
  return sub.urlMasked !== '';
}

export function statusPill(sub: Subscription): PillView {
  if (sub.status === SUB_STATUS_UPDATING) return { text: t('更新中', 'Updating'), tone: 'phase' };
  if (sub.status === SUB_STATUS_ERROR) return { text: t('更新失败', 'Failed'), tone: 'block' };
  if (!canUpdate(sub)) return { text: t('手动导入', 'Manual'), tone: '' };
  return { text: t('订阅', 'Sub'), tone: 'proxy' };
}

export function autoUpdateText(sub: Subscription): string {
  if (!canUpdate(sub)) return '—';
  if (!sub.autoUpdate) return t('已关闭', 'Off');
  return sub.intervalHours > 0 ? t(`每 ${sub.intervalHours} 小时`, `Every ${sub.intervalHours}h`) : t('已开启', 'On');
}

export function describeSkipped(item: SkippedLine): string {
  return item.line > 0 ? t(`第 ${item.line} 行：${item.reason}`, `Line ${item.line}: ${item.reason}`) : item.reason;
}

interface FormatOption {
  readonly value: ExportFormat;
  readonly label: string;
}

export const SHARE_FORMAT_OPTIONS: ReadonlyArray<FormatOption> = [
  { value: 'clash', label: 'Clash' },
  { value: 'singbox', label: 'sing-box' },
  { value: 'base64', label: 'Base64' },
  { value: 'uri', label: '链接' },
];

export function shareFormatOptions(): ReadonlyArray<FormatOption> {
  return [
    { value: 'clash', label: 'Clash' },
    { value: 'singbox', label: 'sing-box' },
    { value: 'base64', label: 'Base64' },
    { value: 'uri', label: t('链接', 'URI') },
  ];
}

/** 内容能否生成二维码：可以返回空串，否则返回原因。 */
export function qrPayloadProblem(text: string): string {
  const size = utf8Length(text);
  if (size === 0) return t('没有可生成二维码的内容', 'No content available for QR code');
  return size > QR_MAX_BYTES ? msgQrTooLarge() : '';
}

/** 把后端生成的 SVG 文本变成 <img> 可用的地址；不是 SVG 时返回 null（绝不当作 HTML 插入）。 */
export function svgDataUrl(svg: string): string | null {
  const text = svg.trim();
  return text.startsWith('<svg') ? SVG_PREFIX + encodeURIComponent(text) : null;
}

/** 局域网分享还剩多少毫秒；未开启或已过期为 0。 */
export function lanRemainingMs(share: LanShare | null, now: number): number {
  if (share === null || !share.active) return 0;
  return Math.max(0, share.expiresAt - now);
}

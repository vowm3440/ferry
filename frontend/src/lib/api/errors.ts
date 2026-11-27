/*
 * 错误归一化。后端返回的 error 字符串本身就是给用户看的中文文案，
 * 原样使用；只有运行时层面的错误才在这里翻译。
 */

import { t } from '../i18n/locale';

export type FailureKind =
  /** 没有桌面后端（普通浏览器打开、桥接未注入）。 */
  | 'unavailable'
  /** 该版本后端没有这个接口。 */
  | 'unsupported'
  /** 调用参数与后端签名不一致。 */
  | 'argument'
  /** 后端明确拒绝，message 是后端给的中文说明。 */
  | 'rejected'
  /** 其它失败：传输层错误，或无法识别的异常。 */
  | 'unknown';

export class BackendError extends Error {
  readonly kind: FailureKind;

  constructor(kind: FailureKind, message: string) {
    super(message);
    this.name = 'BackendError';
    this.kind = kind;
  }
}

/** 传输层错误的原文只用于排查；过长时截断，避免整页内容进入提示。 */
const DETAIL_MAX_LENGTH = 160;

/** 运行时或传输层的错误不是后端给用户的文案：标明来源，再附上原文。 */
function transportMessage(raw: string): string {
  const detail = raw.replace(/\s+/g, ' ');
  const shown = detail.length > DETAIL_MAX_LENGTH ? `${detail.slice(0, DETAIL_MAX_LENGTH)}…` : detail;
  return t(`与桌面后端通信失败：${shown}`, `Desktop backend communication failed: ${shown}`);
}

function messageOf(error: unknown): string {
  if (typeof error === 'string') return error;
  if (error instanceof Error) return error.message;
  if (typeof error === 'object' && error !== null) {
    const record = error as Record<string, unknown>;
    if (typeof record.message === 'string') return record.message;
    if (typeof record.error === 'string') return record.error;
  }
  return '';
}

function looksUnavailable(message: string): boolean {
  return (
    message.includes('Failed to fetch') ||
    message.includes('NetworkError') ||
    message.includes('Load failed') ||
    message.includes('fetch failed') ||
    message.includes('ECONNREFUSED')
  );
}

/**
 * @wailsio/runtime 用异常类型区分失败原因（见其 calls.js 的说明）：
 * RuntimeError = 方法自己返回了 error；ReferenceError = 方法不存在；TypeError = 参数个数或类型不符。
 */
function errorName(error: unknown): string {
  return error instanceof Error ? error.name : '';
}

/** 把任何抛出物统一成 BackendError。 */
export function toBackendError(error: unknown): BackendError {
  if (error instanceof BackendError) return error;

  const raw = messageOf(error).trim();
  const name = errorName(error);

  if (raw === '') return new BackendError('unknown', t('操作失败，请稍后重试', 'Operation failed; please try again'));
  // 方法自己返回的 error 就是给用户看的文案，任何情况下都原样使用。
  if (name === 'RuntimeError') return new BackendError('rejected', raw);
  // 网络失败在浏览器里也是 TypeError，必须先于「参数不符」判断。
  if (looksUnavailable(raw)) return new BackendError('unavailable', t('未连接到桌面后端', 'Not connected to desktop backend'));
  if (name === 'ReferenceError' || raw.includes('unknown bound method name') || raw.includes('unknown method')) {
    return new BackendError('unsupported', t('当前后端版本未实现该接口', 'This method is not implemented by the current backend'));
  }
  if (name === 'TypeError' || (raw.includes('expects') && raw.includes('argument'))) {
    return new BackendError('argument', t('接口参数与后端不匹配，请更新前端版本', 'Method arguments do not match backend; please update frontend'));
  }
  return new BackendError('unknown', transportMessage(raw));
}

/** 直接拿到可以展示给用户的中文文案。 */
export function messageOfError(error: unknown): string {
  return toBackendError(error).message;
}

export function isUnavailable(error: unknown): boolean {
  return toBackendError(error).kind === 'unavailable';
}

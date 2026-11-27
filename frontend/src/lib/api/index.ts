/*
 * API 入口。界面与 store 只从这里导入。
 *
 * 选择后端的规则：
 *   1. 检测到桌面桥接 → 一律使用真实后端（忽略任何演示开关）。
 *   2. 没有桥接 → 不发起任何调用，由界面显示「未连接桌面后端」。
 *   3. 演示模式只能由用户显式进入（按钮或地址 #demo），且仅在情况 2 下可用。
 */

import { t } from '../i18n/locale';
import { hasDesktopBridge, type Backend } from './backend';
import { BackendError } from './errors';
import { createWailsBackend } from './wails';

export * from './types';
export * from './types-inspect';
export * from './types-rules';
export type { Backend, BackendKind, Unsubscribe } from './backend';
export { hasDesktopBridge } from './backend';
export { BackendError, messageOfError, toBackendError } from './errors';
export { cloneSettings } from './defaults';
export { parseCoreState, parseTraffic } from './parse';
export { parseConnectionsEvent, parseLatencyMap, parseLogEntries, parseSubsProgress } from './parse-inspect';
export { MAX_IMPORT_BYTES } from './defaults';
export { loadWindowControls, type WindowControls } from './window';
export {
  copyText,
  hasFileDialogs,
  pickOpenFile,
  pickSaveFile,
  type FileKind,
  type OpenFileRequest,
  type SaveFileRequest,
} from './shell';

const DEMO_HASH = '#demo';

/** 地址中是否显式要求演示模式。桌面壳内不生效。 */
export function isDemoRequested(): boolean {
  if (typeof window === 'undefined') return false;
  return !hasDesktopBridge() && window.location.hash === DEMO_HASH;
}

/** 连接桌面后端。没有桥接时直接抛出 unavailable，不会发起请求。 */
export async function connectDesktop(): Promise<Backend> {
  if (!hasDesktopBridge()) {
    throw new BackendError('unavailable', t('未连接到桌面后端', 'Not connected to desktop backend'));
  }
  return createWailsBackend();
}

/** 按需加载演示后端。演示代码不进入主包。 */
export async function loadDemoBackend(): Promise<Backend> {
  const module = await import('./demo');
  return module.createDemoBackend();
}

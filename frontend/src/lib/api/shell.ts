/*
 * 桌面壳提供的本机能力：系统文件选择框与剪贴板。它们不是 Go 服务方法，
 * 走 Wails 运行时自带的 Dialogs / Clipboard。
 *
 * 选择框返回的路径只用来交给后端（导入文件、导出备份/诊断/日志），界面自己不读写文件。
 */

import { t } from '../i18n/locale';
import { hasDesktopBridge } from './backend';
import { BackendError, toBackendError } from './errors';
import { loadRuntime } from './runtime';

/** 文件类型过滤项，例如 { name: 'Ferry 备份', pattern: '*.ferrybak' }；多个扩展名用分号分隔。 */
export interface FileKind {
  readonly name: string;
  readonly pattern: string;
}

export interface OpenFileRequest {
  readonly title: string;
  readonly kinds?: readonly FileKind[];
}

export interface SaveFileRequest {
  readonly title: string;
  /** 默认文件名。后端的导出都要求目标尚不存在，建议带上时间戳。 */
  readonly filename: string;
  readonly kinds?: readonly FileKind[];
}

interface DialogFilter {
  DisplayName: string;
  Pattern: string;
}

function toFilters(kinds: readonly FileKind[] | undefined): DialogFilter[] {
  return (kinds ?? []).map((kind) => ({ DisplayName: kind.name, Pattern: kind.pattern }));
}

/** 选择框的结果：单个路径、路径数组，或取消时的空值。统一成「路径或 null」。 */
function firstPath(result: unknown): string | null {
  const value: unknown = Array.isArray(result) ? result[0] : result;
  return typeof value === 'string' && value !== '' ? value : null;
}

/** 系统文件选择框只在桌面壳内可用；没有它时，相关入口应当禁用并说明原因。 */
export function hasFileDialogs(): boolean {
  return hasDesktopBridge();
}

function requireDialogs(): void {
  if (!hasFileDialogs()) throw new BackendError('unavailable', t('文件选择框只在桌面版中可用', 'File dialogs are only available in the desktop app'));
}

/** 选择一个已有的文件。用户取消时返回 null。 */
export async function pickOpenFile(request: OpenFileRequest): Promise<string | null> {
  requireDialogs();
  try {
    const runtime = await loadRuntime();
    const result: unknown = await runtime.Dialogs.OpenFile({
      Title: request.title,
      CanChooseFiles: true,
      CanChooseDirectories: false,
      AllowsMultipleSelection: false,
      Filters: toFilters(request.kinds),
    });
    return firstPath(result);
  } catch (error: unknown) {
    throw toBackendError(error);
  }
}

/** 选择保存位置。用户取消时返回 null。 */
export async function pickSaveFile(request: SaveFileRequest): Promise<string | null> {
  requireDialogs();
  try {
    const runtime = await loadRuntime();
    const result: unknown = await runtime.Dialogs.SaveFile({
      Title: request.title,
      Filename: request.filename,
      CanCreateDirectories: true,
      Filters: toFilters(request.kinds),
    });
    return firstPath(result);
  } catch (error: unknown) {
    throw toBackendError(error);
  }
}

/**
 * 把文本写入剪贴板。分享内容可能含凭据，只能在用户显式点击「复制」时调用。
 * 桌面壳内走 Wails 的剪贴板接口；普通浏览器（演示模式）用标准的 Clipboard API。
 */
export async function copyText(text: string): Promise<void> {
  try {
    if (hasDesktopBridge()) {
      const runtime = await loadRuntime();
      await runtime.Clipboard.SetText(text);
      return;
    }
    await navigator.clipboard.writeText(text);
  } catch (error: unknown) {
    if (hasDesktopBridge()) throw toBackendError(error);
    throw new BackendError('unknown', t('无法写入剪贴板，请手动选择并复制', 'Unable to copy to clipboard; please select and copy manually'));
  }
}

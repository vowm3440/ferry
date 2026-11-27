/*
 * 在界面里读取用户选中的文件（二维码图片、规则文件）。
 *
 * 桌面壳的文件选择框只给路径，适合「把路径交给后端」的接口；
 * 需要把内容本身交给后端的接口（Subs.ImportFromQR、Rules.Import）用浏览器自带的文件选择，
 * 内容读进内存后直接交给后端，界面不保存。
 */

import { t } from './i18n/locale';

const KIB = 1024;
const MIB = KIB * KIB;
/** 分段编码的块大小：一次展开太多参数会超出调用栈上限。取 3 的倍数，段与段之间不产生补位。 */
const BASE64_CHUNK = 3 * 8192;

function sizeText(bytes: number): string {
  if (bytes >= MIB && bytes % MIB === 0) return `${bytes / MIB} MiB`;
  return `${Math.ceil(bytes / KIB)} KiB`;
}

/** 文件大小的预检。通过时返回空串，否则返回可以直接展示的说明。 */
export function checkFileSize(size: number, maxBytes: number): string {
  if (!Number.isFinite(size) || size < 0) return t('无法读取文件大小', 'Unable to read file size');
  if (size === 0) return t('文件是空的', 'File is empty');
  if (size > maxBytes) return t(`文件过大：不能超过 ${sizeText(maxBytes)}`, `File too large: maximum ${sizeText(maxBytes)}`);
  return '';
}

/** 文本按 UTF-8 编码后的字节数（后端的大小上限按字节算）。 */
export function utf8Length(text: string): number {
  return new TextEncoder().encode(text).length;
}

export function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  for (let offset = 0; offset < bytes.length; offset += BASE64_CHUNK) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + BASE64_CHUNK));
  }
  return btoa(binary);
}

/** 读成 Base64（不带 data: 前缀）。超限时抛出带中文说明的 Error，不读取内容。 */
export async function readFileBase64(file: File, maxBytes: number): Promise<string> {
  const problem = checkFileSize(file.size, maxBytes);
  if (problem !== '') throw new Error(problem);
  return bytesToBase64(new Uint8Array(await file.arrayBuffer()));
}

/** 读成文本（UTF-8）。超限时抛出带中文说明的 Error，不读取内容。 */
export async function readFileText(file: File, maxBytes: number): Promise<string> {
  const problem = checkFileSize(file.size, maxBytes);
  if (problem !== '') throw new Error(problem);
  return file.text();
}

/**
 * 弹出浏览器的文件选择框，返回选中的文件；用户取消时返回 null。
 * accept 形如 'image/png,image/jpeg' 或 '.yaml,.yml,.json'。必须在点击等用户操作里调用。
 */
export function pickLocalFile(accept: string): Promise<File | null> {
  return new Promise((resolve) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.accept = accept;
    input.addEventListener('change', () => resolve(input.files?.[0] ?? null), { once: true });
    input.addEventListener('cancel', () => resolve(null), { once: true });
    input.click();
  });
}

/*
 * 演示模式的分享链接解析：只取协议、地址、端口与名称，用来演示导入流程。
 * 不校验凭据与传输参数，也不保留它们；真实的解析与白名单在 Go 后端（internal/convert）。
 */

import type { SkippedLine } from './types';

const TYPES: Readonly<Record<string, string>> = {
  vless: 'vless',
  trojan: 'trojan',
  ss: 'shadowsocks',
  socks: 'socks',
  socks5: 'socks',
};

export interface DemoLink {
  name: string;
  type: string;
  server: string;
  port: number;
}

export interface DemoLinks {
  links: DemoLink[];
  skipped: SkippedLine[];
}

function decodeName(hash: string, fallback: string): string {
  try {
    const name = decodeURIComponent(hash.replace(/^#/, '')).trim();
    return name === '' ? fallback : name;
  } catch {
    return fallback;
  }
}

function parseLine(line: string): DemoLink | string {
  const scheme = /^([a-z0-9-]+):\/\//i.exec(line)?.[1]?.toLowerCase();
  if (scheme === undefined) return '无法识别的内容';
  const type = TYPES[scheme];
  if (type === undefined) return `暂不支持 ${scheme} 协议`;
  let url: URL;
  try {
    url = new URL(line);
  } catch {
    return '链接无法解析';
  }
  const port = Number.parseInt(url.port, 10);
  if (url.hostname === '' || !Number.isInteger(port)) return '演示模式只识别带地址与端口的链接';
  const server = url.hostname.replace(/^\[|\]$/g, '');
  return { name: decodeName(url.hash, `${server}:${port}`), type, server, port };
}

export function parseDemoLinks(text: string): DemoLinks {
  const result: DemoLinks = { links: [], skipped: [] };
  text.split(/\r?\n/).forEach((raw, index) => {
    const line = raw.trim();
    if (line === '') return;
    const parsed = parseLine(line);
    if (typeof parsed === 'string') result.skipped.push({ line: index + 1, reason: parsed });
    else result.links.push(parsed);
  });
  return result;
}

/*
 * 演示后端：订阅、导出与局域网分享。
 * 粘贴导入用 demo-links 做一个只取「协议、地址、端口、名称」的简化解析，不保留凭据。
 * 按地址添加与更新不会访问网络：只生成 *.example.invalid 的示例节点，并在 warnings 里如实说明。
 * 导出内容由内存里的演示节点拼出（它们本来就没有凭据）。
 * 读本机文件、识别与生成二维码、局域网分享在浏览器里做不到，一律报「演示模式不提供此功能」。
 */

import type { ShareApi, SubsApi } from './backend';
import { MAX_IMPORT_BYTES } from './defaults';
import { parseDemoLinks } from './demo-links';
import { delay, makeDemoNode, rejected, unsupported, type DemoWorld } from './demo-world';
import { EVENT_SUBS_PROGRESS, type Node, type SkippedLine, type Subscription } from './types';
import { EXPORT_FORMATS, type ExportFormat } from './types-inspect';

interface DemoSub {
  readonly view: Subscription;
  readonly nodeIds: readonly string[];
}

const MAX_NAME_RUNES = 64;
const MAX_URL_LENGTH = 4096;
const MIN_HOURS = 1;
const MAX_HOURS = 720;
const PHASE_DELAY_MS = 350;
const UPDATE_PHASES: readonly string[] = ['fetch', 'parse', 'apply'];
const SAMPLE_TYPES: readonly string[] = ['vless', 'trojan', 'shadowsocks'];
const SAMPLE_PORT = 443;
const DEMO_URL_NOTE = '演示模式没有访问这个地址，下面是示例节点';
const HTTP_WARNING = '此订阅使用未加密的 HTTP，地址与节点内容可能被网络中的第三方读取';
const URL_INVALID = '订阅地址须为有效的 HTTP 或 HTTPS 地址，且不能包含用户信息或片段';

/** 与后端 subName 相同的规则。 */
function cleanName(name: string, fallback: string): string {
  const text = name.trim();
  if (text === '') return fallback;
  if ([...text].length > MAX_NAME_RUNES || /\p{Cc}/u.test(text)) throw rejected('名称无效：最多 64 个字符，且不能包含控制字符');
  return text;
}

/** 与后端 subscriptionURL 相同的规则；只返回协议与主机，其余部分不保留。 */
function checkedUrl(raw: string): URL {
  const text = raw.trim();
  let url: URL | null = null;
  try {
    url = new URL(text);
  } catch {
    url = null;
  }
  const web = url !== null && (url.protocol === 'http:' || url.protocol === 'https:');
  if (url === null || !web || text.length > MAX_URL_LENGTH || url.hostname === '' || url.username !== '' || url.password !== '' || text.includes('#')) {
    throw rejected(URL_INVALID);
  }
  return url;
}

function isExportFormat(format: string): format is ExportFormat {
  return (EXPORT_FORMATS as readonly string[]).includes(format);
}

function toBase64(text: string): string {
  return btoa(String.fromCharCode(...new TextEncoder().encode(text)));
}

/** 把演示节点写成指定格式。演示节点只有协议、地址、端口与名称。 */
function exportNodes(nodes: readonly Node[], format: string): string {
  if (!isExportFormat(format)) throw rejected('导出格式无效');
  if (nodes.length === 0) throw rejected('没有可导出的节点');
  const links = nodes.map((node) => `${node.type}://${node.server}:${node.port}#${encodeURIComponent(node.name)}`).join('\n');
  if (format === 'uri') return links;
  if (format === 'base64') return toBase64(links);
  if (format === 'singbox') {
    const outbounds = nodes.map((node) => ({ type: node.type, tag: node.name, server: node.server, server_port: node.port }));
    return JSON.stringify({ outbounds }, null, 2);
  }
  const proxies = nodes.map((node) => `  - { name: ${JSON.stringify(node.name)}, type: ${node.type}, server: ${node.server}, port: ${node.port} }`);
  return ['proxies:', ...proxies].join('\n');
}

const linkKey = (node: Pick<Node, 'type' | 'server' | 'port'>): string => `${node.type}|${node.server}|${node.port}`;

export function createDemoSubs(world: DemoWorld): SubsApi & ShareApi {
  let subs: readonly DemoSub[] = [];
  let seq = 0;

  world.onReset(() => {
    subs = [];
  });

  function nextId(prefix: string): string {
    seq += 1;
    return `${prefix}-${seq}`;
  }

  function addFromText(name: string, text: string): Subscription {
    if (new Blob([text]).size > MAX_IMPORT_BYTES) throw rejected('内容过大：不能超过 1 MiB');
    const parsed = parseDemoLinks(text);
    if (parsed.links.length === 0 && parsed.skipped.length === 0) throw rejected('没有找到可导入的节点链接');
    if (world.running) throw rejected('请先停止内核，再导入节点');
    // 与真实后端一致：第一次导入会替换已有的（演示/开发配置）节点。
    const kept = subs.length === 0 ? [] : world.nodes;
    const known = new Set(kept.map(linkKey));
    const skipped: SkippedLine[] = [...parsed.skipped];
    const added: Node[] = [];
    for (const link of parsed.links) {
      if (known.has(linkKey(link))) {
        skipped.push({ line: 0, reason: `节点已存在：${link.name}` });
        continue;
      }
      known.add(linkKey(link));
      added.push(makeDemoNode(nextId('demo-import'), link.name, link.type, link.server, link.port));
    }
    if (added.length === 0) throw rejected(`没有可导入的节点：${skipped[0]?.reason ?? ''}`);

    const settings = world.settings;
    const view: Subscription = {
      id: nextId('demo-sub'),
      name: name.trim() === '' ? '手动导入' : name.trim(),
      urlMasked: '',
      nodeCount: added.length,
      groupCount: 0,
      usedBytes: 0,
      totalBytes: 0,
      expiresAt: 0,
      lastUpdatedAt: Date.now(),
      intervalHours: settings.subscriptionUpdateHours,
      autoUpdate: false,
      userAgent: settings.subscriptionUA,
      status: 'ok',
      skipped: [],
      warnings: [],
    };
    world.setNodes([...kept, ...added]);
    subs = [...subs, { view, nodeIds: added.map((node) => node.id) }];
    return { ...view, skipped };
  }

  function find(id: string): DemoSub {
    const target = subs.find((sub) => sub.view.id === id);
    if (target === undefined) throw rejected('订阅不存在');
    return target;
  }

  function patch(id: string, change: Partial<Subscription>): void {
    subs = subs.map((sub) => (sub.view.id === id ? { ...sub, view: { ...sub.view, ...change } } : sub));
  }

  /** 不访问网络：生成几个 *.example.invalid 的示例节点，并在 warnings 里说明。 */
  function addFromUrl(name: string, raw: string, autoUpdate: boolean): Subscription {
    const url = checkedUrl(raw);
    const title = cleanName(name, url.hostname);
    if (world.running) throw rejected('请先停止内核，再导入节点');
    const kept = subs.length === 0 ? [] : world.nodes;
    const id = nextId('demo-sub');
    const added = SAMPLE_TYPES.map((type, index) =>
      makeDemoNode(nextId('demo-url'), `${title} 示例 ${index + 1}`, type, `${id}-${index + 1}.example.invalid`, SAMPLE_PORT),
    );
    const settings = world.settings;
    const view: Subscription = {
      id,
      name: title,
      // 与后端一样只留协议与主机，路径和参数（可能含令牌）不保留。
      urlMasked: `${url.protocol}//${url.host}/****`,
      nodeCount: added.length,
      groupCount: 0,
      usedBytes: 0,
      totalBytes: 0,
      expiresAt: 0,
      lastUpdatedAt: Date.now(),
      intervalHours: settings.subscriptionUpdateHours,
      autoUpdate,
      userAgent: settings.subscriptionUA,
      status: 'ok',
      skipped: [],
      warnings: url.protocol === 'http:' ? [DEMO_URL_NOTE, HTTP_WARNING] : [DEMO_URL_NOTE],
    };
    world.setNodes([...kept, ...added]);
    subs = [...subs, { view, nodeIds: added.map((node) => node.id) }];
    return { ...view };
  }

  /** 走一遍进度事件；节点保持不变（演示模式没有可下载的内容），只刷新更新时间。 */
  async function update(id: string): Promise<void> {
    if (find(id).view.urlMasked === '') throw rejected('订阅不存在或没有可更新的地址');
    patch(id, { status: 'updating' });
    for (const phase of UPDATE_PHASES) {
      world.emit(EVENT_SUBS_PROGRESS, { id, phase });
      await delay(PHASE_DELAY_MS);
    }
    if (!subs.some((sub) => sub.view.id === id)) {
      world.emit(EVENT_SUBS_PROGRESS, { id, phase: 'error', error: '订阅已删除' });
      throw rejected('订阅已删除');
    }
    patch(id, { status: 'ok', lastUpdatedAt: Date.now() });
    world.emit(EVENT_SUBS_PROGRESS, { id, phase: 'done' });
  }

  return {
    async subsList() {
      return subs.map((sub) => ({ ...sub.view }));
    },

    async subsAddFromText(name, text) {
      return addFromText(name, text);
    },

    async subsAddFromURL(name, url, autoUpdate) {
      return addFromUrl(name, url, autoUpdate);
    },

    async subsAddFromFile() {
      throw unsupported();
    },

    async subsImportFromQR() {
      throw unsupported();
    },

    async subsCaptureScreenQR() {
      throw unsupported();
    },

    async subsRename(id, name) {
      const target = find(id);
      patch(id, { name: cleanName(name, target.view.name) });
    },

    async subsSetAutoUpdate(id, on, hours) {
      if (!Number.isInteger(hours) || hours < MIN_HOURS || hours > MAX_HOURS) throw rejected('订阅更新间隔须在 1 到 720 小时之间');
      if (find(id).view.urlMasked === '') throw rejected('手动导入的节点没有订阅地址，无法自动更新');
      patch(id, { autoUpdate: on, intervalHours: hours });
    },

    async subsSetUserAgent(id, ua) {
      const trimmed = ua.trim();
      if (new TextEncoder().encode(trimmed).length > 256) throw rejected('订阅 User-Agent 无效');
      if (find(id).view.urlMasked === '') throw rejected('手动导入的节点没有订阅地址，无法设置 User-Agent');
      patch(id, { userAgent: trimmed !== '' ? trimmed : world.settings.subscriptionUA });
    },

    async subsUpdate(id) {
      await update(id);
    },

    async subsUpdateAll() {
      for (const sub of subs.filter((item) => item.view.urlMasked !== '')) await update(sub.view.id);
    },

    async subsRemove(id) {
      const target = find(id);
      if (world.running) throw rejected('请先停止内核，再删除订阅');
      world.setNodes(world.nodes.filter((node) => !target.nodeIds.includes(node.id)));
      subs = subs.filter((sub) => sub !== target);
    },

    async subsExport(id, format) {
      const ids = find(id).nodeIds;
      return exportNodes(
        world.nodes.filter((node) => ids.includes(node.id)),
        format,
      );
    },

    async subsExportNode(nodeId, format) {
      const node = world.nodes.find((item) => item.id === nodeId);
      if (node === undefined) throw rejected('节点不存在');
      return exportNodes([node], format);
    },

    async subsQrSvg() {
      throw unsupported();
    },

    async subsShareDeepLink(payload: string) {
      const encoded = encodeURIComponent(payload.trim());
      return [{ scheme: 'ferry', label: 'Ferry', url: `ferry://import?data=${encoded}` }];
    },

    async lanShareStart() {
      throw unsupported();
    },

    async lanShareStop() {
      throw unsupported();
    },

    async lanShareStatus() {
      return { url: '', expiresAt: 0, active: false };
    },
  };
}

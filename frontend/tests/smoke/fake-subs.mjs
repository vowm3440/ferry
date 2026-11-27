// 订阅 / 分享 / 测速这一块的伪后端方法表：错误文案与 internal/app 保持一致，状态挂在 s 上。
// 数据全部是合成的（*.example.invalid、192.0.2.0/24 文档网段），不含任何真实凭据。

const MSG = {
  url: '订阅地址须为有效的 HTTP 或 HTTPS 地址，且不能包含用户信息或片段',
  name: '名称无效：最多 64 个字符，且不能包含控制字符',
  http: '此订阅使用未加密的 HTTP，地址与节点内容可能被网络中的第三方读取',
  fetch: '订阅下载失败，请检查网络或地址',
  running: '请先停止内核，再导入节点',
  hours: '订阅更新间隔须在 1 到 720 小时之间',
  manual: '手动导入的节点没有订阅地址，无法自动更新',
  noUpdate: '订阅不存在或没有可更新的地址',
  badContent: '订阅包含不支持或无效的节点，已保留上次可用配置；请检查订阅内容',
  qrSize: '内容超过二维码容量，请使用文件分享',
  qrUrl: '二维码包含订阅地址，请在 URL 导入入口确认来源后导入',
};
const FORMATS = ['clash', 'singbox', 'base64', 'uri'];
const LAN_TTL_MS = 10 * 60 * 1000;
/** 真实二维码在最低纠错等级下大约能放这么多字节；超过时后端如实报错。 */
const QR_CAPACITY = 2900;
const INACTIVE = { url: '', expiresAt: 0, active: false };

const nameOk = (name) => [...name].length <= 64 && !/\p{Cc}/u.test(name);

function parseUrl(raw) {
  try {
    const url = new URL(raw.trim());
    const ok = ['http:', 'https:'].includes(url.protocol) && url.hostname !== '' && url.username === '' && url.password === '' && url.hash === '';
    return ok ? url : null;
  } catch {
    return null;
  }
}

/** 只认 socks5://host:port#name，与 fake-backend 的粘贴导入一致。 */
function parseLines(text) {
  const nodes = [];
  for (const line of text.split(/\r?\n/)) {
    const match = /^socks5:\/\/([^:/#]+):(\d+)(?:#(.*))?$/.exec(line.trim());
    if (match === null) continue;
    nodes.push({
      id: `n-${match[1]}-${match[2]}`,
      tag: decodeURIComponent(match[3] ?? '') || `${match[1]}:${match[2]}`,
      outbound: { type: 'socks', server: match[1], server_port: Number(match[2]), version: '5' },
    });
  }
  return nodes;
}

export function fakeSubs({ s, Rejected }) {
  /** 节点 id -> { latencyMs, latencyStatus }。 */
  s.latency = new Map();
  /** 测试钩子：节点 id -> 测速结果（毫秒，-1 表示超时）；没写的按顺序给一个固定值。 */
  s.latencyPlan = new Map();
  s.lanShare = INACTIVE;
  /** 测试钩子：路径 -> 文件内容，供 Subs.AddFromFile 读取。 */
  s.files = new Map();
  /** 测试钩子：这些订阅 id 下一次更新会失败。 */
  s.updateFailures = new Set();

  const findSub = (id) => s.subs.find((sub) => sub.view.id === id);

  function addSub(fields, fresh, warnings = []) {
    if (s.status === 'running') throw new Rejected(MSG.running);
    const kept = s.subs.length === 0 ? [] : s.nodes;
    const nodes = fresh.filter((node) => !kept.some((item) => item.id === node.id));
    if (nodes.length === 0) throw new Rejected('没有找到可导入的节点链接');
    s.subSeq += 1;
    const view = {
      id: `s-${s.subSeq}`,
      urlMasked: '',
      nodeCount: nodes.length,
      groupCount: 0,
      usedBytes: 0,
      totalBytes: 0,
      expiresAt: 0,
      lastUpdatedAt: Date.now(),
      intervalHours: 0,
      autoUpdate: false,
      userAgent: '',
      status: 'ok',
      warnings,
      ...fields,
    };
    s.nodes = [...kept, ...nodes];
    if (kept.length === 0) s.groups = [];
    s.subs = [...s.subs, { view, nodeIds: nodes.map((node) => node.id) }];
    s.selected = new Map();
    return { ...view, skipped: [] };
  }

  function updateOne(sub) {
    if (s.updateFailures.delete(sub.view.id)) {
      sub.view = { ...sub.view, status: 'error', error: MSG.badContent };
      throw new Rejected(MSG.badContent);
    }
    const { error: _dropped, ...rest } = sub.view;
    sub.view = { ...rest, status: 'ok', lastUpdatedAt: Date.now() };
  }

  function exportText(nodes, format) {
    if (!FORMATS.includes(format)) throw new Rejected('导出格式无效');
    const links = nodes.map((node) => `socks5://${node.outbound.server}:${node.outbound.server_port}#${encodeURIComponent(node.tag)}`).join('\n');
    if (format === 'uri') return links;
    if (format === 'base64') return Buffer.from(links, 'utf8').toString('base64');
    if (format === 'singbox') return JSON.stringify({ outbounds: nodes.map((node) => ({ tag: node.tag, ...node.outbound })) }, null, 2);
    return ['proxies:', ...nodes.map((node) => `  - { name: ${JSON.stringify(node.tag)}, type: socks5, server: ${node.outbound.server}, port: ${node.outbound.server_port} }`)].join('\n');
  }

  const lanStatus = () => {
    if (s.lanShare.active && s.lanShare.expiresAt <= Date.now()) s.lanShare = INACTIVE;
    return s.lanShare;
  };

  return {
    // 与 fake-backend 的 Nodes.List 相同，只是带上测速结果。
    'Nodes.List': () => {
      const explicit = s.groups.map((group) => ({ ...group, members: [...group.members], selected: s.selected.get(group.id) ?? group.members[0] ?? '', testUrl: group.testUrl ?? '', intervalMin: 0 }));
      const members = s.nodes.map((node) => node.id);
      const fallback = [{ id: 'proxy', name: '代理', type: 'select', members, selected: s.selected.get('proxy') ?? members[0], testUrl: '', intervalMin: 0 }];
      const groups = explicit.length > 0 || s.nodes.length === 0 ? explicit : fallback;
      const nodes = s.nodes.map((node) => {
        const group = groups.find((item) => item.members.includes(node.id));
        return {
          id: node.id,
          tag: node.tag,
          name: node.tag,
          groupId: group?.id ?? '',
          type: node.outbound.type,
          server: node.outbound.server,
          port: node.outbound.server_port,
          latencyMs: -1,
          latencyStatus: 'idle',
          ...s.latency.get(node.id),
          selected: group?.selected === node.id,
          favorite: s.favorites.get(node.id) === true,
        };
      });
      return { groups, nodes };
    },
    'Core.LatencyTest': (ids) => {
      if (!Array.isArray(ids) || ids.length === 0) throw new Rejected('请选择需要测速的节点');
      if (s.status !== 'running') throw new Rejected('请先启动内核再测速');
      if (ids.some((id) => !s.nodes.some((node) => node.id === id))) throw new Rejected('测速节点不存在');
      const result = {};
      ids.forEach((id, index) => {
        const ms = s.latencyPlan.get(id) ?? 80 + index * 37;
        result[id] = ms;
        s.latency.set(id, { latencyMs: ms, latencyStatus: ms < 0 ? 'timeout' : 'ok' });
      });
      return result;
    },
    // 不联网：*.example.invalid 下以 down 开头的主机模拟下载失败，其余给三个合成节点。
    'Subs.AddFromURL': (name, raw, autoUpdate) => {
      const url = parseUrl(String(raw));
      if (url === null) throw new Rejected(MSG.url);
      if (!nameOk(name)) throw new Rejected(MSG.name);
      if (url.hostname.startsWith('down')) throw new Rejected(MSG.fetch);
      const prefix = `u${s.subSeq + 1}`;
      const fresh = [1, 2, 3].map((n) => ({ id: `${prefix}-${n}`, tag: `合成节点 ${n}`, outbound: { type: 'socks', server: `${prefix}-${n}.example.invalid`, server_port: 1080 + n, version: '5' } }));
      const fields = {
        name: name.trim() || url.hostname,
        urlMasked: `${url.protocol}//${url.host}/****`,
        autoUpdate: autoUpdate === true,
        intervalHours: autoUpdate === true ? 24 : 0,
        usedBytes: 12 * 1024 ** 3,
        totalBytes: 100 * 1024 ** 3,
        expiresAt: Date.now() + 20 * 86_400_000,
      };
      return addSub(fields, fresh, url.protocol === 'http:' ? [MSG.http] : []);
    },
    'Subs.AddFromFile': (path) => {
      const text = s.files.get(path);
      if (text === undefined) throw new Rejected('无法读取节点文件');
      return addSub({ name: String(path).split(/[\\/]/).pop() }, parseLines(text));
    },
    // 伪后端不解码图片：把文件内容当作二维码里的文字。
    'Subs.ImportFromQR': (encoded) => {
      if (typeof encoded !== 'string' || !/^[A-Za-z0-9+/]+=*$/.test(encoded)) throw new Rejected('图片编码无效');
      const text = Buffer.from(encoded, 'base64').toString('utf8');
      if (/^https?:\/\//i.test(text.trim())) throw new Rejected(MSG.qrUrl);
      return [addSub({ name: '二维码导入' }, parseLines(text))];
    },
    'Subs.Rename': (id, name) => {
      const sub = findSub(id);
      if (sub === undefined) throw new Rejected('订阅不存在');
      if (!nameOk(name) || name.trim() === '') throw new Rejected(MSG.name);
      sub.view = { ...sub.view, name: name.trim() };
    },
    'Subs.SetAutoUpdate': (id, on, hours) => {
      const sub = findSub(id);
      if (sub === undefined) throw new Rejected('订阅不存在');
      if (sub.view.urlMasked === '') throw new Rejected(MSG.manual);
      if (!Number.isInteger(hours) || hours < 1 || hours > 720) throw new Rejected(MSG.hours);
      sub.view = { ...sub.view, autoUpdate: on === true, intervalHours: hours };
    },
    'Subs.Update': (id) => {
      const sub = findSub(id);
      if (sub === undefined || sub.view.urlMasked === '') throw new Rejected(MSG.noUpdate);
      updateOne(sub);
    },
    'Subs.UpdateAll': () => {
      const failed = [];
      for (const sub of s.subs.filter((item) => item.view.urlMasked !== '')) {
        try {
          updateOne(sub);
        } catch (error) {
          failed.push(`${sub.view.name}：${error.message}`);
        }
      }
      if (failed.length > 0) throw new Rejected(failed.join('\n'));
    },
    'Subs.Export': (id, format) => {
      const sub = findSub(id);
      if (sub === undefined) throw new Rejected('订阅不存在');
      return exportText(s.nodes.filter((node) => sub.nodeIds.includes(node.id)), format);
    },
    'Subs.ExportNode': (nodeId, format) => {
      const node = s.nodes.find((item) => item.id === nodeId);
      if (node === undefined) throw new Rejected('节点不存在');
      return exportText([node], format);
    },
    // 不是真的二维码，只是一张合法的 SVG，足够验证界面怎么显示它。
    'Subs.QRSvg': (payload, size) => {
      const bytes = Buffer.byteLength(String(payload), 'utf8');
      if (!Number.isInteger(size) || size < 128 || size > 1024 || bytes < 1 || bytes > QR_CAPACITY) throw new Rejected(MSG.qrSize);
      return `<svg xmlns="http://www.w3.org/2000/svg" width="${size}" height="${size}" viewBox="0 0 21 21"><rect width="21" height="21" fill="#fff"/><path d="M1 1h7v7H1zM13 1h7v7h-7zM1 13h7v7H1zM10 10h2v2h-2zM14 14h5v5h-5z" fill="#000"/></svg>`;
    },
    'Subs.LanShareStart': () => {
      if (s.nodes.length === 0) throw new Rejected('没有可导出的节点');
      s.lanShare = { url: 'http://192.0.2.10:18080/share/smoke-synthetic-token', expiresAt: Date.now() + LAN_TTL_MS, active: true };
      return s.lanShare;
    },
    'Subs.LanShareStop': () => {
      s.lanShare = INACTIVE;
    },
    'Subs.LanShareStatus': () => lanStatus(),
  };
}

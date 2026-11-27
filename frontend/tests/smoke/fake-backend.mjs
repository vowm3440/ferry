// 假的 Wails 后端（仅用于冒烟）：按 @wailsio/runtime 3.1.0 的线上协议应答 POST /wails/runtime，
// 业务行为照抄 internal/app 的 Go 实现（错误文案逐字相同），用来走通前端真实的适配层。
const PREFIX = 'ferry/internal/app.';
const MAX_BYTES = 1 << 20;
const SPEC_KEYS = ['nodes', 'groups', 'rules', 'dns', 'inbound', 'mode', 'log'];
const IP_DNS = /^(https|tls|tcp|udp):\/\/\d{1,3}(\.\d{1,3}){3}/;

/** Go 方法返回 error：应答 422 RuntimeError，message 原样给界面。 */
export class Rejected extends Error {}

function defaultSettings() {
  return {
    autoStart: false,
    startMinimized: false,
    lightweight: false,
    language: 'zh-CN',
    theme: 'system',
    mixedPort: 7890,
    allowLan: false,
    sysProxyOnStart: false,
    tun: { stack: 'mixed', strictRoute: true, excludes: [], mtu: 9000, fakeIp: true },
    dns: { remote: 'https://1.1.1.1/dns-query', direct: 'https://223.5.5.5/dns-query', strategy: 'prefer_ipv4' },
    ruleSetAutoUpdate: false,
    subscriptionUA: 'clash.meta',
    subscriptionUpdateHours: 24,
    logLevel: 'info',
    updateChannel: 'stable',
    clashApiMode: 'random',
  };
}

function validateSettings(v) {
  if (!Number.isInteger(v.mixedPort) || v.mixedPort < 1 || v.mixedPort > 65535) throw new Rejected('代理端口必须在 1 到 65535 之间');
  if (!['system', 'light', 'dark'].includes(v.theme)) throw new Rejected('主题设置无效');
  if (!['zh-CN', 'en-US'].includes(v.language)) throw new Rejected('语言设置无效');
  if (v.clashApiMode !== 'random') throw new Rejected('内核控制接口必须使用随机端口');
  if (v.autoStart || v.startMinimized || v.lightweight || v.sysProxyOnStart || v.ruleSetAutoUpdate) throw new Rejected('这些自动化功能尚未开放');
  if (v.subscriptionUpdateHours < 1 || v.subscriptionUpdateHours > 720 || v.subscriptionUA.length > 256) throw new Rejected('订阅设置无效');
  if (!['stable', 'beta'].includes(v.updateChannel)) throw new Rejected('更新通道无效');
}

/**
 * capabilities：System.Capabilities 的返回值，默认只有最基础的一项。
 * extra：各功能区自己的方法表工厂 ({ s, Rejected, state }) => ({ 'Rules.List': () => … })，
 *        放在各自的 fake-<area>.mjs 里，需要的状态挂在 s 上。
 */
export function createFakeBackend({ capabilities = ['subs.text'], extra = [] } = {}) {
  const s = {
    capabilities: [...capabilities],
    /** 剪贴板（Wails 对象 1）当前的文字。 */
    clipboard: '',
    /** 文件选择框（Wails 对象 5）接下来依次返回的路径；用完后返回空串，即「用户取消」。 */
    dialogReplies: [],
    /** 每次弹出文件选择框的记录：{ method, args }，4 = 打开，5 = 保存。 */
    dialogs: [],
    status: 'stopped',
    /** 模拟 Session.state() 在内核连接断开时给出的 error。 */
    error: '',
    startedAt: 0,
    mode: 'rule',
    nodes: [],
    groups: [],
    settings: defaultSettings(),
    favorites: new Map(),
    selected: new Map(),
    /** { view, nodeIds }，对应 internal/app 的 subscription。 */
    subs: [],
    subSeq: 0,
    calls: [],
    /** 每次绑定方法调用的名称与参数，供断言使用。 */
    log: [],
    /** 测试钩子：方法短名 -> { status, body, type }，命中后按原样应答一次。 */
    faults: new Map(),
    ticks: 0,
    maximised: false,
    /** 返回 4xx/5xx 的次数；浏览器会为每一次记一条网络错误日志。 */
    failures: 0,
  };

  const state = () => ({
    status: s.status,
    mode: s.mode,
    uptimeMs: s.status === 'running' ? Date.now() - s.startedAt : 0,
    memoryBytes: s.status === 'running' ? 41_943_040 : 0,
    version: '0.1.0-dev',
    coreVersion: '1.14.4',
    serviceMode: false,
    // Go 侧是 omitempty：没有错误时不带这个字段。
    ...(s.error === '' ? {} : { error: s.error }),
  });

  /** 与 internal/core/host 的 zeroTraffic 一致：停止时返回全 0 快照，不报错。 */
  const zeroTraffic = () => ({
    up: 0,
    down: 0,
    activeConns: 0,
    totalUp: 0,
    totalDown: 0,
    memoryBytes: 0,
    upHistory: Array.from({ length: 60 }, () => 0),
    downHistory: Array.from({ length: 60 }, () => 0),
  });

  const traffic = () => {
    if (s.status !== 'running') return zeroTraffic();
    s.ticks += 1;
    const history = Array.from({ length: 60 }, (_, index) => (index < 60 - s.ticks ? 0 : 1024 * (index % 7)));
    return {
      up: 2048,
      down: 1_572_864,
      activeConns: 3,
      totalUp: 2048 * s.ticks,
      totalDown: 1_572_864 * s.ticks,
      memoryBytes: 41_943_040,
      upHistory: history,
      downHistory: history.map((value) => value * 40),
    };
  };

  const groupsView = () => {
    const explicit = s.groups.map((group) => ({
      id: group.id,
      name: group.name,
      type: group.type,
      members: [...group.members],
      selected: s.selected.get(group.id) ?? group.members[0] ?? '',
      testUrl: group.testUrl ?? '',
      intervalMin: 0,
    }));
    if (explicit.length > 0 || s.nodes.length === 0) return explicit;
    const members = s.nodes.map((node) => node.id);
    return [{ id: 'proxy', name: '代理', type: 'select', members, selected: s.selected.get('proxy') ?? members[0], testUrl: '', intervalMin: 0 }];
  };

  const methods = {
    'Core.Status': () => state(),
    'Core.Snapshot': () => traffic(),
    'Core.Configure': (text) => {
      if (text.length > MAX_BYTES) throw new Rejected('配置过大');
      let spec;
      try {
        spec = JSON.parse(text);
      } catch {
        throw new Rejected('配置无效，请检查节点、端口与支持的字段');
      }
      const known = spec !== null && typeof spec === 'object' && Object.keys(spec).every((key) => SPEC_KEYS.includes(key));
      if (!known || !Array.isArray(spec.nodes) || (spec.rules ?? []).length > 0) throw new Rejected('配置无效，请检查节点、端口与支持的字段');
      if (s.status === 'running') throw new Rejected('请先停止内核，再导入配置');
      s.nodes = spec.nodes;
      s.subs = [];
      s.groups = spec.groups ?? [];
      s.mode = spec.mode;
      s.settings = {
        ...s.settings,
        mixedPort: spec.inbound.mixedPort,
        allowLan: spec.inbound.allowLan,
        tun: { stack: spec.inbound.tun.stack, strictRoute: spec.inbound.tun.strictRoute, excludes: spec.inbound.tun.excludes ?? [], mtu: spec.inbound.tun.mtu, fakeIp: spec.dns.mode === 'fakeip' },
        dns: { remote: spec.dns.remote, direct: spec.dns.direct, strategy: spec.dns.strategy },
        logLevel: spec.log.level,
      };
      s.selected = new Map();
    },
    'Core.Start': () => {
      if (s.status === 'running') return;
      if (s.nodes.length === 0) throw new Rejected('请先在「订阅」页导入节点');
      s.status = 'running';
      s.error = '';
      s.startedAt = Date.now();
      s.ticks = 0;
      s.selected = new Map();
    },
    'Core.Stop': () => {
      s.status = 'stopped';
    },
    'Core.Restart': () => {
      s.status = 'stopped';
      methods['Core.Start']();
    },
    'Core.SetMode': (mode) => {
      if (!['rule', 'global', 'direct'].includes(mode)) throw new Rejected('代理模式无效');
      s.mode = mode;
    },
    'Core.SelectNode': (groupId, nodeId) => {
      if (s.status !== 'running') throw new Rejected('请先启动内核再切换节点');
      s.selected.set(groupId, nodeId);
    },
    'Nodes.List': () => {
      const groups = groupsView();
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
          selected: group?.selected === node.id,
          favorite: s.favorites.get(node.id) === true,
        };
      });
      return { groups, nodes };
    },
    'Nodes.SetFavorite': (id, on) => {
      if (!s.nodes.some((node) => node.id === id)) throw new Rejected('节点不存在');
      s.favorites.set(id, on);
    },
    'Subs.List': () => s.subs.map((sub) => sub.view),
    // 只认 socks5://host:port#name；其余按 internal/convert 的文案跳过，足够走通界面。
    'Subs.AddFromText': (name, text) => {
      if (text.length > MAX_BYTES) throw new Rejected('内容过大：不能超过 1 MiB');
      const lines = text.split(/\r?\n/).map((line) => line.trim());
      if (!lines.some((line) => line.includes('://'))) throw new Rejected('没有找到可导入的节点链接');
      if (s.status === 'running') throw new Rejected('请先停止内核，再导入节点');
      const kept = s.subs.length === 0 ? [] : s.nodes;
      const skipped = [];
      const fresh = [];
      lines.forEach((line, index) => {
        if (line === '') return;
        const match = /^socks5:\/\/([^:/#]+):(\d+)(?:#(.*))?$/.exec(line);
        const scheme = /^([a-z0-9]+):\/\//.exec(line)?.[1];
        if (match === null) {
          const reason = scheme === undefined ? '无法识别的内容' : scheme === 'socks5' ? '链接无法解析' : `暂不支持 ${scheme} 协议`;
          skipped.push({ line: index + 1, reason });
          return;
        }
        const id = `n-${match[1]}-${match[2]}`;
        const tag = decodeURIComponent(match[3] ?? '') || `${match[1]}:${match[2]}`;
        if ([...kept, ...fresh].some((node) => node.id === id)) {
          skipped.push({ line: 0, reason: `节点已存在：${tag}` });
          return;
        }
        fresh.push({ id, tag, outbound: { type: 'socks', server: match[1], server_port: Number(match[2]), version: '5' } });
      });
      if (fresh.length === 0) {
        const first = skipped[0];
        throw new Rejected(`没有可导入的节点：${first.line > 0 ? `第 ${first.line} 行` : ''}${first.reason}`);
      }
      s.subSeq += 1;
      const view = {
        id: `s-${s.subSeq}`,
        name: name.trim() || '手动导入',
        urlMasked: '',
        nodeCount: fresh.length,
        groupCount: 0,
        usedBytes: 0,
        totalBytes: 0,
        expiresAt: 0,
        lastUpdatedAt: Date.now(),
        intervalHours: 0,
        autoUpdate: false,
        userAgent: '',
        status: 'ok',
      };
      s.nodes = [...kept, ...fresh];
      if (kept.length === 0) s.groups = [];
      s.subs = [...s.subs, { view, nodeIds: fresh.map((node) => node.id) }];
      s.selected = new Map();
      return { ...view, skipped };
    },
    'Subs.Remove': (id) => {
      const target = s.subs.find((sub) => sub.view.id === id);
      if (target === undefined) throw new Rejected('订阅不存在');
      if (s.status === 'running') throw new Rejected('请先停止内核，再删除订阅');
      s.nodes = s.nodes.filter((node) => !target.nodeIds.includes(node.id));
      s.subs = s.subs.filter((sub) => sub !== target);
      s.selected = new Map();
    },
    'System.Capabilities': () => [...s.capabilities],
    'Settings.Get': () => s.settings,
    'Settings.Set': (value) => {
      validateSettings(value);
      if (s.status === 'running') throw new Rejected('请先停止内核，再修改设置');
      if (!IP_DNS.test(value.dns.remote) || !IP_DNS.test(value.dns.direct)) throw new Rejected('设置无效，请检查 DNS、端口和入站设置');
      s.settings = value;
    },
  };

  for (const build of extra) Object.assign(methods, build({ s, Rejected, state }));

  const reply = (res, status, body, type = 'application/json') => {
    if (status >= 400) s.failures += 1;
    res.writeHead(status, { 'Content-Type': type });
    res.end(type === 'application/json' ? JSON.stringify(body) : body);
  };

  function call(res, { methodName, args = [] }) {
    const short = String(methodName).startsWith(PREFIX) ? methodName.slice(PREFIX.length) : methodName;
    s.calls.push(short);
    s.log.push({ method: short, args });
    const fault = s.faults.get(short);
    if (fault !== undefined) {
      s.faults.delete(short);
      return reply(res, fault.status, fault.body, fault.type ?? 'application/json');
    }
    const method = methods[short];
    if (method === undefined) return reply(res, 422, { kind: 'ReferenceError', message: `unknown bound method name '${methodName}'` });
    if (args.length !== method.length) return reply(res, 422, { kind: 'TypeError', message: `${methodName} expects ${method.length} arguments, got ${args.length}` });
    try {
      const result = method(...args);
      // 只返回 error 的 Go 方法成功时没有内容。
      return result === undefined ? reply(res, 200, '', 'text/plain') : reply(res, 200, result);
    } catch (error) {
      if (error instanceof Rejected) return reply(res, 422, { kind: 'RuntimeError', message: error.message });
      throw error;
    }
  }

  /** 挂到静态服务器上：返回 true 表示请求已处理。 */
  async function handle(req, res) {
    const path = new URL(req.url, 'http://x').pathname;
    if (path !== '/wails/runtime') return false;
    const chunks = [];
    for await (const chunk of req) chunks.push(chunk);
    const body = JSON.parse(Buffer.concat(chunks).toString('utf8'));
    if (body.object === 0) {
      call(res, body.args);
      return true;
    }
    s.calls.push(`object${body.object}.method${body.method}`);
    // 窗口对象：14 = IsMaximised，39 = ToggleMaximise，其余（最小化、关闭等）成功且无内容。
    if (body.object === 6 && body.method === 14) reply(res, 200, s.maximised);
    else if (body.object === 1 && body.method === 1) reply(res, 200, s.clipboard);
    else if (body.object === 5 && (body.method === 4 || body.method === 5)) {
      s.dialogs.push({ method: body.method, args: body.args });
      reply(res, 200, s.dialogReplies.shift() ?? '');
    } else {
      // 剪贴板：0 = SetText。
      if (body.object === 1 && body.method === 0) s.clipboard = String(body.args?.text ?? '');
      if (body.object === 6 && body.method === 39) s.maximised = !s.maximised;
      reply(res, 200, '', 'text/plain');
    }
    return true;
  }

  return { handle, s, state, traffic, methodNames: () => Object.keys(methods) };
}

// 规则 / 应用分组 / 规则集这一块的伪后端方法表：错误文案与 internal/app、internal/spec 保持一致，状态挂在 s 上。
// 数据全部是合成的（*.example.invalid、文档网段），不含任何真实凭据。
// 导入解析与规则测试只是确定性的简化版，够页面走通流程即可；校验顺序与文案照 Go 的来。

const MSG = {
  pinnedEdit: '固定规则不可编辑',
  pinnedRemove: '固定规则不可删除',
  pinnedToggle: '固定规则不可禁用',
  missing: '规则不存在',
  order: '规则顺序无效，固定规则须位于首尾',
  orderIds: '规则顺序含重复或未知标识',
  label: '规则标识或名称无效',
  kind: '规则协议或动作无效',
  pinnedFlag: '普通规则不可标记为固定规则',
  target: '规则目标不存在或不适用于此动作',
  count: '规则条件数量超过限制',
  app: '规则应用条件无效',
  groupRef: '应用分组引用格式无效',
  groupMissing: '引用的应用分组不存在',
  set: '规则集不在支持的目录内，请选择已列出的规则集',
  host: '规则主机条件无效或语法不受支持',
  port: '规则端口必须为 1 到 65535 或递增的端口范围',
  tooMany: '规则数量超过限制',
  conn: '连接已结束或不存在',
  importSize: '规则内容超过 1 MiB 限制',
  importEmpty: '规则内容为空',
  importKind: '规则导入格式不受支持',
  importParse: '规则文档无法解析',
  test: '规则测试的应用、主机、端口或网络类型无效',
  groupLabel: '应用分组标识或名称无效',
  groupApps: '应用分组成员无效，不能引用其它分组',
  groupCount: '应用分组数量超过限制',
  groupGone: '应用分组不存在',
  groupUsed: '规则仍在引用此应用分组，请先移除对应引用',
  setsCount: '请选择 1 到 4 个规则集',
  setsUnknown: '规则集不在支持的目录内',
  setsDup: '规则集列表含重复项',
};
const LAN_ID = 'builtin-lan';
const DEFAULT_ID = 'builtin-default';
const GROUP_PREFIX = '@group:';
const SET_REFS = { 'geosite:cn': 'geosite-cn', 'geosite:geolocation-!cn': 'geosite-non-cn', 'geoip:cn': 'geoip-cn', 'geoip:private': 'geoip-private' };
const LAN_HOSTS = ['0.0.0.0/8', '10.0.0.0/8', '127.0.0.0/8', '169.254.0.0/16', '172.16.0.0/12', '192.168.0.0/16', '224.0.0.0/4', '255.255.255.255/32', '::/128', '::1/128', 'fc00::/7', 'fe80::/10', 'ff00::/8'];
const LAN_RULE = { id: LAN_ID, name: '本机 / 局域网', enabled: true, apps: [], hosts: LAN_HOSTS, ports: [], network: 'any', action: 'direct', resolveBefore: false, pinned: true };
const DEFAULT_RULE = { id: DEFAULT_ID, name: '默认规则', enabled: true, apps: [], hosts: [], ports: [], network: 'any', action: 'proxy', targetId: 'proxy', resolveBefore: false, pinned: true };
const CATALOG = [
  { id: 'geosite-cn', name: 'China domains' },
  { id: 'geosite-non-cn', name: 'Non-China domains' },
  { id: 'geoip-cn', name: 'China IP addresses' },
  { id: 'geoip-private', name: 'Private IP addresses' },
];
/** 预览里把 Clash 风格的一行转成规则条件；其余类型一律跳过并给出提示。 */
const IMPORT_FIELDS = { 'DOMAIN-SUFFIX': (value) => ({ hosts: [`*.${value}`] }), DOMAIN: (value) => ({ hosts: [value] }), 'IP-CIDR': (value) => ({ hosts: [value] }), 'PROCESS-NAME': (value) => ({ apps: [value] }), 'DST-PORT': (value) => ({ ports: [value] }) };
const IMPORT_ACTIONS = { DIRECT: 'direct', REJECT: 'block', PROXY: 'proxy' };

const idOk = (id) => typeof id === 'string' && /^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$/.test(id);
const labelOk = (name) => typeof name === 'string' && name.trim() !== '' && [...name].length <= 64 && !/\p{Cc}/u.test(name);
const strings = (value) => Array.isArray(value) && value.every((item) => typeof item === 'string');
const isIp = (text) => /^\d{1,3}(\.\d{1,3}){3}$/.test(text) || (text.includes(':') && /^[0-9a-fA-F:.]+$/.test(text));
const isCidr = (text) => text.includes('/') && isIp(text.slice(0, text.indexOf('/'))) && /^\d{1,3}$/.test(text.slice(text.indexOf('/') + 1));
const isDomain = (text) => /^[A-Za-z0-9*?_-]+(\.[A-Za-z0-9*?_-]+)*$/.test(text) && !isIp(text);
const isRange = (text) => text.split('-').length === 2 && text.split('-').every(isIp);
const appOk = (app) => app !== '' && app.length <= 512 && !/[\p{Cc}"<>|?*]/u.test(app);
const hex = (seed) => seed.toString(16).padStart(24, '0');

function portOk(port) {
  const parts = port.split('-');
  if (parts.length > 2 || !parts.every((part) => /^[1-9]\d{0,4}$/.test(part) && Number(part) <= 65535)) return false;
  return parts.length === 1 || Number(parts[0]) < Number(parts[1]);
}

function hostProblem(host, customSets = []) {
  if (host.startsWith('geosite:') || host.startsWith('geoip:')) return host in SET_REFS ? '' : MSG.set;
  if (host.startsWith('ruleset:')) {
    const id = host.slice('ruleset:'.length);
    return customSets.some((item) => item.id === id) ? '' : MSG.set;
  }
  return isIp(host) || isCidr(host) || isRange(host) || isDomain(host) ? '' : MSG.host;
}

function appProblem(app, groups) {
  if (!app.startsWith(GROUP_PREFIX)) return appOk(app) ? '' : MSG.app;
  const id = app.slice(GROUP_PREFIX.length);
  if (!idOk(id)) return MSG.groupRef;
  return groups.some((group) => group.id === id) ? '' : MSG.groupMissing;
}

const first = (items, problem) => items.map(problem).find((text) => text !== '') ?? '';

function ruleProblem(rule, s) {
  if (!idOk(rule.id) || !labelOk(rule.name)) return MSG.label;
  if (!['any', 'tcp', 'udp'].includes(rule.network) || !['proxy', 'direct', 'block'].includes(rule.action)) return MSG.kind;
  if (rule.pinned === true) return MSG.pinnedFlag;
  const target = rule.targetId ?? '';
  const targets = ['', 'proxy', ...s.groups.map((group) => group.id), ...s.nodes.map((node) => node.id)];
  if (rule.action === 'proxy' ? !targets.includes(target) : target !== '') return MSG.target;
  if (![rule.apps, rule.hosts, rule.ports].every((list) => strings(list) && list.length <= 256)) return MSG.count;
  return first(rule.apps, (app) => appProblem(app, s.appGroups)) || first(rule.hosts, (host) => hostProblem(host, s.customRuleSets ?? [])) || first(rule.ports, (port) => (portOk(port) ? '' : MSG.port));
}

function normalize(rule) {
  const { id, name, enabled, apps, hosts, ports, network, action, targetId, resolveBefore } = rule;
  const base = { id, name: typeof name === 'string' ? name.trim() : name, enabled: enabled === true, apps, hosts, ports, network, action, resolveBefore: resolveBefore === true };
  return targetId === undefined || targetId === '' ? base : { ...base, targetId };
}

/** 一条导入行：TYPE,值,动作。转换不了的返回 null。 */
function importLine(line) {
  const [type, value, action] = line.split(',').map((part) => part.trim());
  const field = IMPORT_FIELDS[type];
  const mapped = IMPORT_ACTIONS[action];
  if (field === undefined || mapped === undefined || value === undefined || value === '') return null;
  const rule = { enabled: true, apps: [], hosts: [], ports: [], network: 'any', action: mapped, resolveBefore: true, ...field(value) };
  return mapped === 'proxy' ? { ...rule, targetId: 'proxy' } : rule;
}

function portMatches(ports, port) {
  if (ports.length === 0) return true;
  return ports.some((item) => {
    const [low, high = low] = item.split('-').map(Number);
    return port >= low && port <= high;
  });
}

const baseName = (path) => path.slice(Math.max(path.lastIndexOf('\\'), path.lastIndexOf('/')) + 1).toLowerCase();

function appMatches(apps, application, groups) {
  if (apps.length === 0) return true;
  const expanded = apps.flatMap((app) => (app.startsWith(GROUP_PREFIX) ? (groups.find((group) => group.id === app.slice(GROUP_PREFIX.length))?.apps ?? []) : [app]));
  return application !== '' && expanded.some((app) => app.toLowerCase() === application.toLowerCase() || baseName(app) === baseName(application));
}

/** 域名条件里的 * 与 ? 是通配符。 */
const glob = (pattern) => new RegExp(`^${pattern.split('.').map((part) => part.replaceAll('*', '.*').replaceAll('?', '.')).join('\\.')}$`, 'i');

/** 'yes' | 'no' | 'dns'：规则要求先解析域名时，域名遇到只能按 IP 判断的条件无法确定。只认前缀对齐到字节的 IPv4 网段。 */
function hostMatches(hosts, host, resolveBefore) {
  if (hosts.length === 0) return 'yes';
  const ip = isIp(host);
  let needsDns = false;
  for (const item of hosts) {
    if (item.startsWith('geoip:') || isCidr(item) || isIp(item)) {
      if (!ip) needsDns = resolveBefore;
      else if (item === host) return 'yes';
      else if (isCidr(item) && !item.includes(':')) {
        const [net, bits] = item.split('/');
        const keep = Math.floor(Number(bits) / 8);
        if (keep > 0 && net.split('.').slice(0, keep).join('.') === host.split('.').slice(0, keep).join('.')) return 'yes';
      }
    } else if (item === 'geosite:cn') {
      if (!ip && host.endsWith('.cn')) return 'yes';
    } else if (!ip && glob(item).test(host)) return 'yes';
  }
  return needsDns ? 'dns' : 'no';
}

export function fakeRules({ s, Rejected }) {
  Object.assign(s, { rules: [], appGroups: [], customRuleSets: [], rulesVersion: 0, ruleSeq: 0, connections: s.connections ?? [] });
  s.ruleSets = CATALOG.map((item) => ({ ...item, url: `https://rules.example.invalid/${item.id}.srs`, updatedAt: 0, sizeBytes: 0, status: 'missing' }));

  const all = () => [LAN_RULE, ...s.rules, DEFAULT_RULE];
  const fail = (message) => {
    throw new Rejected(message);
  };
  const nextId = (prefix) => `${prefix}-${hex((s.ruleSeq += 1))}`;
  /** 整份规则通过校验后才落地，与 Go 侧先校验后写入一致。 */
  function commit(next) {
    if (next.length + 2 > 2048) fail(MSG.tooMany);
    const problem = first(next, (rule) => ruleProblem(rule, s));
    if (problem !== '') fail(problem);
    s.rules = next;
    s.rulesVersion += 1;
  }
  function reorder(ids) {
    const current = all();
    if (!strings(ids) || ids.length !== current.length || ids[0] !== LAN_ID || ids[ids.length - 1] !== DEFAULT_ID) fail(MSG.order);
    if (new Set(ids).size !== ids.length || !ids.every((id) => current.some((rule) => rule.id === id))) fail(MSG.orderIds);
    commit(ids.slice(1, -1).map((id) => s.rules.find((rule) => rule.id === id)));
  }
  const userRule = (id, pinnedMessage) => {
    if (id === LAN_ID || id === DEFAULT_ID) fail(pinnedMessage);
    return s.rules.find((rule) => rule.id === id) ?? fail(MSG.missing);
  };

  return {
    'Rules.List'() {
      return all();
    },
    'Rules.Save'(rule) {
      if (rule?.pinned === true || rule?.id === LAN_ID || rule?.id === DEFAULT_ID) fail(MSG.pinnedEdit);
      const saved = normalize({ ...rule, id: rule.id === '' ? nextId('r') : rule.id });
      const exists = s.rules.some((item) => item.id === saved.id);
      commit(exists ? s.rules.map((item) => (item.id === saved.id ? saved : item)) : [...s.rules, saved]);
      return saved;
    },
    // 与 Go 的 SaveMany 一致：整批是一个事务，传入的 id 不沿用，commit 校验不过就什么都不写。
    'Rules.SaveMany'(batch) {
      if (!Array.isArray(batch) || batch.length === 0) fail('没有可导入的规则');
      if (batch.some((rule) => rule?.pinned === true || rule?.id === LAN_ID || rule?.id === DEFAULT_ID)) fail(MSG.pinnedEdit);
      commit([...s.rules, ...batch.map((rule) => normalize({ ...rule, id: nextId('r') }))]);
    },
    'Rules.Remove'(id) {
      userRule(id, MSG.pinnedRemove);
      commit(s.rules.filter((rule) => rule.id !== id));
    },
    'Rules.SetEnabled'(id, on) {
      userRule(id, MSG.pinnedToggle);
      commit(s.rules.map((rule) => (rule.id === id ? { ...rule, enabled: on === true } : rule)));
    },
    'Rules.Reorder'(ids) {
      reorder(ids);
    },
    'Rules.Apply'(ids) {
      reorder(ids);
    },
    'Rules.CreateFromConnection'(id) {
      const conn = s.connections.find((item) => item.id === id) ?? fail(MSG.conn);
      const base = { id: '', name: '连接规则', enabled: true, apps: conn.processPath ? [conn.processPath] : [], hosts: conn.host ? [conn.host] : [], ports: conn.port ? [String(conn.port)] : [], network: 'any', resolveBefore: true };
      return { ...base, action: 'proxy', targetId: s.groups[0]?.id ?? 'proxy' };
    },
    'Rules.Import'(kind, content) {
      if (typeof content !== 'string' || content.length > 1 << 20) fail(MSG.importSize);
      if (content.trim() === '') fail(MSG.importEmpty);
      if (!['clash', 'singbox', 'ppx'].includes(kind)) fail(MSG.importKind);
      const lines = content.split(/\r?\n/).map((line) => line.trim()).filter((line) => line !== '');
      if (!lines.some((line) => line.includes(','))) fail(MSG.importParse);
      const rules = [];
      const warnings = [];
      lines.forEach((line, index) => {
        const rule = importLine(line);
        if (rule === null) warnings.push(`第 ${index + 1} 条规则已跳过：条件或目标无法完整转换`);
        else rules.push({ id: `import-${kind}-${index + 1}-${hex(index + 1).slice(-8)}`, name: `导入规则 ${index + 1}`, ...rule });
      });
      return { rules, warnings };
    },
    'Rules.Test'(application, host, port, network) {
      const hostOk = typeof host === 'string' && (isIp(host) || (isDomain(host) && !/[*?]/.test(host)));
      if (typeof application !== 'string' || !hostOk || !Number.isInteger(port) || port < 1 || port > 65535 || !['tcp', 'udp'].includes(network)) fail(MSG.test);
      for (const rule of all()) {
        if (!rule.enabled || (rule.network !== 'any' && rule.network !== network)) continue;
        if (!portMatches(rule.ports, port) || !appMatches(rule.apps, application, s.appGroups)) continue;
        const verdict = hostMatches(rule.hosts, host, rule.resolveBefore);
        if (verdict === 'no') continue;
        const head = { matchedRuleId: rule.id, matchedRuleName: rule.name };
        if (verdict === 'dns') return { ...head, action: '', outboundTag: '', chain: [], stage: 'dns-required' };
        const target = rule.targetId ?? 'proxy';
        const tag = rule.action === 'direct' ? '__direct' : rule.action === 'block' ? 'REJECT' : (s.nodes.find((node) => node.id === target)?.tag ?? target);
        return { ...head, action: rule.action, outboundTag: tag, chain: [tag], stage: 'matched' };
      }
      return fail(MSG.test);
    },
    'Rules.DefaultLazyPreset'() {
      return { replaceExisting: true, enableAds: true, enableAi: true, enableAppleSteam: true, enableCnSplit: true, enableSplitDns: true, intranetDns: '', intranetDomains: ['*.corp', '*.local', '*.lan', '*.internal', '*.home.arpa'] };
    },
    'Rules.ApplyLazyPreset'(opts) {
      const add = [
        ...(opts.enableAds ? [{ id: 'ads', name: '广告与跟踪拦截', url: 'https://rules.example.invalid/ads.srs', autoUpdate: true, updateHours: 24 }] : []),
        ...(opts.enableAi ? [{ id: 'openai', name: 'OpenAI / ChatGPT', url: 'https://rules.example.invalid/openai.srs', autoUpdate: true, updateHours: 24 }] : []),
      ];
      for (const rs of add) {
        if (!s.customRuleSets.some((item) => item.id === rs.id)) {
          s.customRuleSets.push(rs);
          s.ruleSets.push({ ...rs, updatedAt: 0, sizeBytes: 0, status: 'missing', custom: true });
        }
      }
      s.rulesVersion += 1;
      return { ruleCount: all().length, ruleSetCount: s.customRuleSets.length, appGroupCount: s.appGroups.length, dnsRuleCount: opts.enableSplitDns ? 2 : 0, missingRuleSets: [] };
    },
    'AppGroups.List'() {
      return s.appGroups;
    },
    'AppGroups.Save'(group) {
      const saved = { id: group.id === '' ? nextId('ag') : group.id, name: typeof group.name === 'string' ? group.name.trim() : group.name, apps: group.apps };
      if (!idOk(saved.id) || !labelOk(saved.name)) fail(MSG.groupLabel);
      if (!strings(saved.apps) || saved.apps.length > 256) fail('应用分组成员数量超过限制');
      if (!saved.apps.every((app) => !app.startsWith(GROUP_PREFIX) && appOk(app))) fail(MSG.groupApps);
      const exists = s.appGroups.some((item) => item.id === saved.id);
      if (!exists && s.appGroups.length >= 128) fail(MSG.groupCount);
      s.appGroups = exists ? s.appGroups.map((item) => (item.id === saved.id ? saved : item)) : [...s.appGroups, saved];
      s.rulesVersion += 1;
      return saved;
    },
    'AppGroups.AddApp'(groupId, app) {
      const cur = s.appGroups.find((item) => item.id === groupId);
      if (!cur) fail(MSG.groupGone);
      const next = { ...cur, apps: cur.apps.includes(app) ? cur.apps : [...cur.apps, app] };
      s.appGroups = s.appGroups.map((item) => (item.id === groupId ? next : item));
      s.rulesVersion += 1;
      return next;
    },
    'AppGroups.Remove'(id) {
      if (!s.appGroups.some((group) => group.id === id)) fail(MSG.groupGone);
      if (s.rules.some((rule) => rule.apps.includes(GROUP_PREFIX + id))) fail(MSG.groupUsed);
      s.appGroups = s.appGroups.filter((group) => group.id !== id);
      s.rulesVersion += 1;
    },
    'RuleSets.List'(refresh) {
      void refresh;
      return s.ruleSets;
    },
    'RuleSets.Presets'() {
      return [
        { id: 'openai', name: 'OpenAI / ChatGPT', url: 'https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/openai.srs', autoUpdate: true, updateHours: 24 },
        { id: 'apple-cn', name: 'Apple 国内直连', url: 'https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/apple-cn.srs', autoUpdate: true, updateHours: 24 },
      ];
    },
    'RuleSets.Save'(rs) {
      const item = { id: rs.id.trim(), name: rs.name.trim(), url: rs.url.trim(), autoUpdate: rs.autoUpdate === true, updateHours: rs.updateHours || 0 };
      const exists = s.customRuleSets.some((cur) => cur.id === item.id);
      s.customRuleSets = exists ? s.customRuleSets.map((cur) => (cur.id === item.id ? item : cur)) : [...s.customRuleSets, item];
      const info = { ...item, updatedAt: 0, sizeBytes: 0, status: 'missing', custom: true };
      s.ruleSets = s.ruleSets.some((cur) => cur.id === item.id) ? s.ruleSets.map((cur) => (cur.id === item.id ? { ...cur, ...item } : cur)) : [...s.ruleSets, info];
      return item;
    },
    'RuleSets.Remove'(id) {
      if (CATALOG.some((item) => item.id === id)) fail('内置规则集不可删除');
      if (s.rules.some((rule) => rule.hosts.includes('ruleset:' + id))) fail('仍有路由规则或分流 DNS 引用此规则集，请先移除引用');
      s.customRuleSets = s.customRuleSets.filter((cur) => cur.id !== id);
      s.ruleSets = s.ruleSets.filter((cur) => cur.id !== id);
    },
    'RuleSets.Update'(ids) {
      if (!strings(ids) || ids.length < 1 || ids.length > 68) fail(MSG.setsCount);
      if (!ids.every((id) => s.ruleSets.some((item) => item.id === id))) fail(MSG.setsUnknown);
      if (new Set(ids).size !== ids.length) fail(MSG.setsDup);
      // 固定的时间与大小：截图与断言都不依赖运行时刻。
      s.ruleSets = s.ruleSets.map((item) => (ids.includes(item.id) ? { ...item, status: 'ready', sizeBytes: 123456, updatedAt: 1794700800000 } : item));
    },
  };
}

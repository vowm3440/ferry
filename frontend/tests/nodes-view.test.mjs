import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  ORPHAN_GROUP_ID,
  buildGroupViews,
  currentSelection,
  latencyTargets,
  latencyView,
  mergeLatency,
  nodeDisplayName,
  protocolBadge,
  protocolLabel,
} from '../src/lib/nodes-view.ts';

const ALL = { query: '', favoritesOnly: false, sort: 'default' };

function node(overrides) {
  return {
    id: 'n',
    tag: '',
    name: '',
    groupId: 'proxy',
    type: 'socks',
    server: '127.0.0.1',
    port: 1080,
    latencyMs: -1,
    latencyStatus: 'idle',
    selected: false,
    favorite: false,
    ...overrides,
  };
}

function group(overrides) {
  return { id: 'proxy', name: '代理', type: 'select', members: [], testUrl: '', intervalMin: 0, ...overrides };
}

/** 递归冻结：被测函数一旦改动传入的数据就会抛错。 */
function deepFreeze(value) {
  if (typeof value === 'object' && value !== null && !Object.isFrozen(value)) {
    Object.freeze(value);
    Object.values(value).forEach(deepFreeze);
  }
  return value;
}

const NODE_A = node({ id: 'a', name: 'beta 节点甲' });
const NODE_B = node({
  id: 'b',
  name: 'alpha 节点乙',
  type: 'trojan',
  server: 'example.com',
  port: 443,
  favorite: true,
  latencyMs: 85,
});
const LIST = deepFreeze({
  groups: [group({ members: ['a', 'b'], selected: 'a' })],
  nodes: [NODE_A, NODE_B],
});

const names = (view) => view.members.map((member) => member.name);

test('协议名称与徽标：已知协议用约定写法，未知协议原样显示', () => {
  assert.equal(protocolLabel('shadowsocks'), 'Shadowsocks');
  assert.equal(protocolBadge('shadowsocks'), 'SS');
  assert.equal(protocolLabel('hysteria2'), 'Hysteria2');
  assert.equal(protocolBadge('hysteria2'), 'HY2');
  assert.equal(protocolLabel('anytls'), 'anytls');
  assert.equal(protocolBadge('anytls'), 'ANYTLS');
  assert.equal(protocolBadge('shadowtls-v3'), 'SHADOW');
  assert.equal(protocolLabel(''), '未知协议');
  assert.equal(protocolBadge(''), '?');
});

test('nodeDisplayName：名称 → 标签 → id 依次回退', () => {
  assert.equal(nodeDisplayName(node({ id: 'i', tag: 't', name: 'n' })), 'n');
  assert.equal(nodeDisplayName(node({ id: 'i', tag: 't' })), 't');
  assert.equal(nodeDisplayName(node({ id: 'i' })), 'i');
});

test('buildGroupViews：按后端顺序列出成员并标出当前选中项', () => {
  const views = buildGroupViews(LIST, ALL);
  assert.equal(views.length, 1);
  assert.deepEqual(views[0], {
    id: 'proxy',
    name: '代理',
    kind: 'select',
    selectedName: 'beta 节点甲',
    total: 2,
    members: [
      {
        id: 'a',
        name: 'beta 节点甲',
        kind: 'node',
        protocol: 'SOCKS',
        badge: 'SOCKS',
        address: '127.0.0.1:1080',
        latencyMs: -1,
        latencyStatus: 'idle',
        favorite: false,
        selected: true,
      },
      {
        id: 'b',
        name: 'alpha 节点乙',
        kind: 'node',
        protocol: 'Trojan',
        badge: 'TROJAN',
        address: 'example.com:443',
        latencyMs: 85,
        latencyStatus: 'idle',
        favorite: true,
        selected: false,
      },
    ],
  });
});

test('buildGroupViews：地址里的 IPv6 加方括号，没有端口或地址时不拼接', () => {
  const list = {
    groups: [group({ members: ['v6', 'noport', 'noserver'] })],
    nodes: [
      node({ id: 'v6', server: '2001:db8::1', port: 443 }),
      node({ id: 'noport', server: 'example.com', port: 0 }),
      node({ id: 'noserver', server: '', port: 443 }),
    ],
  };
  const addresses = buildGroupViews(list, ALL)[0].members.map((member) => member.address);
  assert.deepEqual(addresses, ['[2001:db8::1]:443', 'example.com', '']);
});

test('buildGroupViews：搜索匹配名称、协议与地址，不改变成员总数', () => {
  const search = (query) => buildGroupViews(LIST, { ...ALL, query })[0];
  assert.deepEqual(names(search('  BETA ')), ['beta 节点甲']);
  assert.deepEqual(names(search('trojan')), ['alpha 节点乙']);
  assert.deepEqual(names(search('example.com')), ['alpha 节点乙']);
  assert.deepEqual(names(search('节点')), ['beta 节点甲', 'alpha 节点乙']);
  assert.deepEqual(names(search('没有这个节点')), []);
  assert.equal(search('没有这个节点').total, 2);
});

test('buildGroupViews：只看收藏与按名称排序', () => {
  assert.deepEqual(names(buildGroupViews(LIST, { ...ALL, favoritesOnly: true })[0]), ['alpha 节点乙']);
  assert.deepEqual(names(buildGroupViews(LIST, { ...ALL, sort: 'name' })[0]), ['alpha 节点乙', 'beta 节点甲']);
  // LIST 已被冻结：排序若原地进行，上一行就会抛错。这里再确认后端给的顺序没有变。
  assert.deepEqual(LIST.groups[0].members, ['a', 'b']);
});

test('buildGroupViews：自动选择分组不把占位的 selected 当作选中项', () => {
  const list = { groups: [group({ type: 'urltest', members: ['a', 'b'], selected: 'a' })], nodes: [NODE_A, NODE_B] };
  const [view] = buildGroupViews(list, ALL);
  assert.equal(view.kind, 'urltest');
  assert.equal(view.selectedName, '');
  assert.equal(view.members.some((member) => member.selected), false);
});

test('buildGroupViews：成员可以是分组，找不到的成员如实标为未知', () => {
  const list = {
    groups: [
      group({ id: 'outer', name: '', type: 'fallback', members: ['auto', 'ghost'] }),
      group({ id: 'auto', name: '自动', type: 'urltest', members: ['a'] }),
    ],
    nodes: [NODE_A],
  };
  const [outer] = buildGroupViews(list, ALL);
  assert.equal(outer.name, 'outer');
  assert.equal(outer.kind, 'other');
  assert.deepEqual(
    outer.members.map((member) => [member.id, member.kind, member.name, member.protocol, member.badge]),
    [
      ['auto', 'group', '自动', '分组', '组'],
      ['ghost', 'unknown', 'ghost', '未知成员', '?'],
    ],
  );
});

test('buildGroupViews：不属于任何分组的节点收在末尾', () => {
  const orphan = node({ id: 'c', name: '游离节点', groupId: '' });
  const views = buildGroupViews({ groups: LIST.groups, nodes: [...LIST.nodes, orphan] }, ALL);
  assert.equal(views.length, 2);
  const last = views[1];
  assert.equal(last.id, ORPHAN_GROUP_ID);
  assert.equal(last.kind, 'orphan');
  assert.equal(last.name, '未加入分组');
  assert.equal(last.total, 1);
  assert.deepEqual(names(last), ['游离节点']);
  assert.equal(last.members[0].selected, false);

  assert.deepEqual(buildGroupViews({ groups: [], nodes: [] }, ALL), []);
});

test('currentSelection：手动分组给出选中的节点', () => {
  assert.deepEqual(currentSelection(LIST), {
    name: 'beta 节点甲',
    badge: 'SOCKS',
    detail: '代理 · SOCKS',
    automatic: false,
    latencyMs: -1,
  });
});

test('currentSelection：没有分组或没有选中项时为空', () => {
  assert.equal(currentSelection({ groups: [], nodes: [] }), null);
  assert.equal(currentSelection({ groups: [group({ members: ['a'] })], nodes: [NODE_A] }), null);
});

test('currentSelection：自动选择分组只给出分组名', () => {
  const list = { groups: [group({ name: '自动', type: 'urltest', members: ['a'], selected: 'a' })], nodes: [NODE_A] };
  assert.deepEqual(currentSelection(list), {
    name: '自动',
    badge: 'AUTO',
    detail: '出口节点由内核自动选择',
    automatic: true,
    latencyMs: -1,
  });
});

test('currentSelection：选中项找不到时不冒充成节点', () => {
  const list = { groups: [group({ members: ['a'], selected: 'ghost' })], nodes: [NODE_A] };
  assert.deepEqual(currentSelection(list), {
    name: 'ghost',
    badge: '?',
    detail: '代理 · 未知成员',
    automatic: false,
    latencyMs: -1,
  });
});

test('按延迟排序：有结果的从快到慢，其后是超时，最后是未测速；同档保持原顺序', () => {
  const list = deepFreeze({
    groups: [group({ members: ['u1', 'slow', 't', 'fast', 'u2', 'g'], selected: 'u1' }), group({ id: 'g', name: '子分组', members: [] })],
    nodes: [
      node({ id: 'u1', name: '未测甲' }),
      node({ id: 'slow', name: '慢', latencyMs: 420, latencyStatus: 'ok' }),
      node({ id: 't', name: '超时', latencyMs: -1, latencyStatus: 'timeout' }),
      node({ id: 'fast', name: '快', latencyMs: 35, latencyStatus: 'ok' }),
      node({ id: 'u2', name: '未测乙' }),
    ],
  });
  assert.deepEqual(names(buildGroupViews(list, { ...ALL, sort: 'latency' })[0]), ['快', '慢', '超时', '未测甲', '未测乙', '子分组']);
});

test('按延迟排序：正在测速的节点按上一次的结果排，不来回跳', () => {
  const list = {
    groups: [group({ members: ['a', 'b'], selected: 'a' })],
    nodes: [node({ id: 'a', name: '甲', latencyMs: -1, latencyStatus: 'testing' }), node({ id: 'b', name: '乙', latencyMs: 90, latencyStatus: 'testing' })],
  };
  assert.deepEqual(names(buildGroupViews(list, { ...ALL, sort: 'latency' })[0]), ['乙', '甲']);
});

test('mergeLatency：只更新提到的节点，不改动原数据；测速中保留上一次的数值', () => {
  const list = deepFreeze({
    groups: [group({ members: ['a', 'b'], selected: 'a' })],
    nodes: [node({ id: 'a', latencyMs: 120, latencyStatus: 'ok' }), node({ id: 'b' })],
  });
  const testing = mergeLatency(list, { a: { latencyMs: -1, status: 'testing' }, ghost: { latencyMs: 5, status: 'ok' } });
  assert.deepEqual(
    testing.nodes.map((item) => [item.id, item.latencyMs, item.latencyStatus]),
    [
      ['a', 120, 'testing'],
      ['b', -1, 'idle'],
    ],
  );
  assert.equal(testing.groups, list.groups);
  assert.equal(testing.nodes[1], list.nodes[1]);
  const done = mergeLatency(testing, { a: { latencyMs: -1, status: 'timeout' }, b: { latencyMs: 61, status: 'ok' } });
  assert.deepEqual(
    done.nodes.map((item) => [item.latencyMs, item.latencyStatus]),
    [
      [-1, 'timeout'],
      [61, 'ok'],
    ],
  );
});

test('mergeLatency：没有任何匹配时返回原对象', () => {
  const list = { groups: [], nodes: [node({ id: 'a' })] };
  assert.equal(mergeLatency(list, {}), list);
  assert.equal(mergeLatency(list, { ghost: { latencyMs: 1, status: 'ok' } }), list);
});

test('latencyView：测速中、超时、有结果、未测速', () => {
  assert.deepEqual(latencyView(-1, 'testing'), { text: '测速中…', level: 'none', busy: true });
  assert.deepEqual(latencyView(-1, 'timeout'), { text: '超时', level: 'err', busy: false });
  assert.deepEqual(latencyView(85, 'ok'), { text: '85 ms', level: 'ok', busy: false });
  assert.deepEqual(latencyView(320, 'ok'), { text: '320 ms', level: 'warn', busy: false });
  assert.deepEqual(latencyView(-1, 'idle'), { text: '未测速', level: 'none', busy: false });
});

test('latencyTargets：全部或某个分组里的节点 id，去重，不含分组成员与未知成员', () => {
  const list = {
    groups: [group({ members: ['a', 'b', 'g', 'ghost'] }), group({ id: 'g', name: '子分组', members: ['b', 'c'] })],
    nodes: [node({ id: 'a' }), node({ id: 'b' }), node({ id: 'c', groupId: 'g' }), node({ id: 'o', groupId: '' })],
  };
  assert.deepEqual(latencyTargets(list), ['a', 'b', 'c', 'o']);
  assert.deepEqual(latencyTargets(list, 'proxy'), ['a', 'b']);
  assert.deepEqual(latencyTargets(list, 'g'), ['b', 'c']);
  assert.deepEqual(latencyTargets(list, ORPHAN_GROUP_ID), ['o']);
  assert.deepEqual(latencyTargets(list, '没有这个分组'), []);
});

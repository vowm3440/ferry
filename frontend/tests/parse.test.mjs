import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parseCoreState, parseNodeList, parseSettings, parseTraffic } from '../src/lib/api/parse.ts';
import { isCoreBusy, isCoreRunning, isProxyMode } from '../src/lib/api/types.ts';

/** 后端可能给出的「不是对象」的取值：Go 的 nil、误传的数组或字符串。 */
const GARBAGE = [null, undefined, 'text', 42, [], [{ status: 'running' }]];

const FULL_SETTINGS = {
  autoStart: true,
  startMinimized: true,
  lightweight: true,
  language: 'en-US',
  theme: 'dark',
  mixedPort: 7890,
  allowLan: true,
  sysProxyOnStart: true,
  tun: { stack: 'mixed', strictRoute: true, excludes: ['10.0.0.0/8'], mtu: 9000, fakeIp: true },
  dns: { remote: 'https://dns.google/dns-query', direct: 'https://223.5.5.5/dns-query', strategy: 'prefer_ipv4' },
  ruleSetAutoUpdate: true,
  subscriptionUA: 'ferry/0.1',
  subscriptionUpdateHours: 12,
  logLevel: 'debug',
  updateChannel: 'beta',
  clashApiMode: 'fixed',
};

test('parseCoreState：完整负载逐字段保留', () => {
  const payload = {
    status: 'error',
    mode: 'global',
    uptimeMs: 1500,
    memoryBytes: 2048,
    version: '0.1.0-dev',
    coreVersion: '1.14.4',
    serviceMode: true,
    error: '内核意外退出',
  };
  assert.deepEqual(parseCoreState(payload), payload);
});

test('parseCoreState：不是对象时回退为「已停止」', () => {
  const stopped = {
    status: 'stopped',
    mode: 'rule',
    uptimeMs: 0,
    memoryBytes: 0,
    version: '',
    coreVersion: '',
    serviceMode: false,
  };
  for (const value of GARBAGE) assert.deepEqual(parseCoreState(value), stopped, String(value));
});

test('parseCoreState：类型不符的字段回退，空的 error 不保留', () => {
  const state = parseCoreState({ status: 1, mode: null, uptimeMs: '5', serviceMode: 'yes', error: '' });
  assert.equal(state.status, 'stopped');
  assert.equal(state.mode, 'rule');
  assert.equal(state.uptimeMs, 0);
  assert.equal(state.serviceMode, false);
  assert.equal('error' in state, false);
});

test('parseTraffic：缺失字段为 0，历史数组只保留有限数值', () => {
  const empty = {
    up: 0,
    down: 0,
    activeConns: 0,
    totalUp: 0,
    totalDown: 0,
    memoryBytes: 0,
    upHistory: [],
    downHistory: [],
  };
  for (const value of GARBAGE) assert.deepEqual(parseTraffic(value), empty, String(value));

  const traffic = parseTraffic({ up: 10, down: Number.NaN, upHistory: [1, '2', null, 3], downHistory: null });
  assert.equal(traffic.up, 10);
  assert.equal(traffic.down, 0);
  assert.deepEqual(traffic.upHistory, [1, 3]);
  assert.deepEqual(traffic.downHistory, []);
});

test('parseNodeList：nil 或 null 数组都得到空列表', () => {
  const empty = { groups: [], nodes: [] };
  for (const value of GARBAGE) assert.deepEqual(parseNodeList(value), empty, String(value));
  assert.deepEqual(parseNodeList({ groups: null, nodes: null }), empty);
});

test('parseNodeList：节点字段缺失时使用安全默认值', () => {
  const [node] = parseNodeList({ nodes: [{ id: 'n1', name: '节点甲', port: '1080' }] }).nodes;
  assert.deepEqual(node, {
    id: 'n1',
    tag: '',
    name: '节点甲',
    groupId: '',
    type: '',
    server: '',
    port: 0,
    latencyMs: -1,
    latencyStatus: 'idle',
    selected: false,
    favorite: false,
  });
});

test('parseNodeList：只取约定的字段，多余内容不会进入界面', () => {
  const list = parseNodeList({
    nodes: [{ id: 'n1', password: 'secret', uuid: 'secret' }],
    groups: [{ id: 'g1', members: ['n1', 7, null], selected: '', token: 'secret' }],
  });
  assert.equal(JSON.stringify(list).includes('secret'), false);
  assert.deepEqual(list.groups[0].members, ['n1']);
  assert.equal('selected' in list.groups[0], false);
});

test('parseNodeList：分组保留当前选中的节点', () => {
  const [group] = parseNodeList({
    groups: [{ id: 'g1', name: '代理', type: 'select', members: ['n1'], selected: 'n1', testUrl: 'u', intervalMin: 5 }],
  }).groups;
  assert.deepEqual(group, {
    id: 'g1',
    name: '代理',
    type: 'select',
    members: ['n1'],
    selected: 'n1',
    testUrl: 'u',
    intervalMin: 5,
  });
});

test('parseSettings：完整取值逐字段保留', () => {
  assert.deepEqual(parseSettings(FULL_SETTINGS), FULL_SETTINGS);
});

test('parseSettings：不是对象时得到完整的默认结构', () => {
  const fallback = {
    autoStart: false,
    startMinimized: false,
    lightweight: false,
    language: 'zh-CN',
    theme: 'system',
    mixedPort: 0,
    allowLan: false,
    sysProxyOnStart: false,
    tun: { stack: '', strictRoute: false, excludes: [], mtu: 0, fakeIp: false },
    dns: { remote: '', direct: '', strategy: '' },
    ruleSetAutoUpdate: false,
    subscriptionUA: '',
    subscriptionUpdateHours: 0,
    logLevel: 'info',
    updateChannel: 'stable',
    clashApiMode: 'random',
  };
  for (const value of GARBAGE) assert.deepEqual(parseSettings(value), fallback, String(value));
});

test('parseSettings：嵌套对象类型不符时单独回退', () => {
  const settings = parseSettings({ ...FULL_SETTINGS, tun: 'off', dns: null });
  assert.deepEqual(settings.tun, { stack: '', strictRoute: false, excludes: [], mtu: 0, fakeIp: false });
  assert.deepEqual(settings.dns, { remote: '', direct: '', strategy: '' });
  assert.equal(settings.mixedPort, 7890);
});

test('状态与模式的判断只认约定的取值', () => {
  assert.equal(isProxyMode('rule') && isProxyMode('global') && isProxyMode('direct'), true);
  assert.equal(isProxyMode('tun') || isProxyMode(''), false);
  assert.equal(isCoreRunning('running'), true);
  assert.equal(isCoreRunning('starting') || isCoreRunning('error'), false);
  assert.equal(isCoreBusy('starting') && isCoreBusy('stopping'), true);
  assert.equal(isCoreBusy('running') || isCoreBusy('stopped'), false);
});

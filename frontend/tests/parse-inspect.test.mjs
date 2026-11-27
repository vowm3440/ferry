import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  parseCapabilities,
  parseConnections,
  parseConnectionsEvent,
  parseLanShare,
  parseLatencyMap,
  parseLatencyResult,
  parseLogEntries,
  parseSubsProgress,
  parseSysProxyStatus,
} from '../src/lib/api/parse-inspect.ts';

/** 后端可能给出的「不是对象」的取值：Go 的 nil、误传的数组或字符串。 */
const GARBAGE = [null, undefined, 'text', 42, [], [{ id: 'x' }]];

const FULL_CONNECTION = {
  id: 'c-1',
  process: 'chrome.exe',
  processPath: 'C:\\Program Files\\Google\\Chrome\\chrome.exe',
  host: 'example.com',
  port: 443,
  ruleName: '默认规则',
  ruleId: 'builtin-default',
  action: 'proxy',
  outboundTag: 'proxy',
  chain: ['proxy', '节点甲'],
  up: 1024,
  down: 4096,
  startAt: 1700000000000,
};

test('parseConnections：完整负载逐字段保留', () => {
  assert.deepEqual(parseConnections([FULL_CONNECTION]), [FULL_CONNECTION]);
});

test('parseConnections：字段缺失时使用安全默认值，ruleId 只在非空时保留', () => {
  const [item] = parseConnections([{ id: 'c-2', chain: null, ruleId: '', port: '443', up: Number.NaN }]);
  assert.deepEqual(item, {
    id: 'c-2',
    process: '',
    processPath: '',
    host: '',
    port: 0,
    ruleName: '',
    action: '',
    outboundTag: '',
    chain: [],
    up: 0,
    down: 0,
    startAt: 0,
  });
});

test('parseConnections：不是数组时为空，丢弃没有 id 的条目与非对象', () => {
  for (const value of [null, undefined, 'x', 3, {}]) assert.deepEqual(parseConnections(value), []);
  const list = parseConnections([{ id: 'a' }, { host: '没有 id' }, null, 'x', 7]);
  assert.deepEqual(
    list.map((item) => item.id),
    ['a'],
  );
});

test('parseConnectionsEvent：取事件里的完整列表 items，忽略差量字段', () => {
  const list = parseConnectionsEvent({ items: [FULL_CONNECTION], added: [{ id: 'other' }], removed: ['gone'] });
  assert.deepEqual(list, [FULL_CONNECTION]);
  for (const value of GARBAGE) assert.deepEqual(parseConnectionsEvent(value), [], String(value));
  assert.deepEqual(parseConnectionsEvent({ items: null }), []);
});

test('parseLogEntries：保留顺序与字段，丢弃非对象', () => {
  const list = parseLogEntries([
    { seq: 7, time: 1700000000000, level: 'info', message: '内核已启动' },
    null,
    'x',
    { seq: 8, message: 42 },
  ]);
  assert.deepEqual(list, [
    { seq: 7, time: 1700000000000, level: 'info', message: '内核已启动' },
    { seq: 8, time: 0, level: '', message: '' },
  ]);
  for (const value of [null, undefined, 'x', 3, {}]) assert.deepEqual(parseLogEntries(value), []);
});

test('parseLatencyMap：逐节点规范化，缺失的延迟为 -1', () => {
  const map = parseLatencyMap({
    a: { latencyMs: 120, status: 'ok' },
    b: { latencyMs: -1, status: 'testing' },
    c: { status: 'timeout' },
    d: {},
    e: 'bad',
    f: null,
  });
  assert.deepEqual(map, {
    a: { latencyMs: 120, status: 'ok' },
    b: { latencyMs: -1, status: 'testing' },
    c: { latencyMs: -1, status: 'timeout' },
    d: { latencyMs: -1, status: 'idle' },
  });
  for (const value of GARBAGE.slice(0, 5)) assert.deepEqual(parseLatencyMap(value), {}, String(value));
});

test('parseLatencyMap：__proto__ 这样的键不会污染原型', () => {
  const payload = JSON.parse('{"__proto__": {"latencyMs": 1, "status": "ok"}}');
  const map = parseLatencyMap(payload);
  assert.equal(Object.getPrototypeOf(map), Object.prototype);
  assert.equal({}.latencyMs, undefined);
  assert.deepEqual(Object.keys(map), ['__proto__']);
});

test('parseLatencyResult：只保留有限数值，-1 表示超时', () => {
  assert.deepEqual(parseLatencyResult({ a: 88, b: -1, c: '120', d: null, e: Number.POSITIVE_INFINITY }), { a: 88, b: -1 });
  for (const value of GARBAGE.slice(0, 5)) assert.deepEqual(parseLatencyResult(value), {}, String(value));
});

test('parseSubsProgress：error 只在非空时保留', () => {
  assert.deepEqual(parseSubsProgress({ id: 's-1', phase: 'fetch' }), { id: 's-1', phase: 'fetch' });
  assert.deepEqual(parseSubsProgress({ id: 's-1', phase: 'error', error: '下载失败' }), {
    id: 's-1',
    phase: 'error',
    error: '下载失败',
  });
  for (const value of GARBAGE) assert.deepEqual(parseSubsProgress(value), { id: '', phase: '' }, String(value));
});

test('parseSysProxyStatus：不是对象时视为未启用', () => {
  const off = { enabled: false, address: '', bypass: [] };
  for (const value of GARBAGE) assert.deepEqual(parseSysProxyStatus(value), off, String(value));
  assert.deepEqual(parseSysProxyStatus({ enabled: true, address: '127.0.0.1:7890', bypass: ['<local>', 7, 'localhost'] }), {
    enabled: true,
    address: '127.0.0.1:7890',
    bypass: ['<local>', 'localhost'],
  });
  assert.equal(parseSysProxyStatus({ enabled: 'yes' }).enabled, false);
});

test('parseLanShare：不是对象时视为未开启', () => {
  const off = { url: '', expiresAt: 0, active: false };
  for (const value of GARBAGE) assert.deepEqual(parseLanShare(value), off, String(value));
  const live = { url: 'http://192.168.1.2:50000/s/token', expiresAt: 1700000600000, active: true };
  assert.deepEqual(parseLanShare({ ...live, token: 'extra' }), live);
});

test('parseCapabilities：只保留非空字符串并去重', () => {
  assert.deepEqual(parseCapabilities(['rules', 'logs', '', 7, null, 'rules']), ['rules', 'logs']);
  for (const value of [null, undefined, 'rules', 3, {}]) assert.deepEqual(parseCapabilities(value), []);
});

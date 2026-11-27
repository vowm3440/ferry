import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  MAX_ROWS,
  actionKind,
  actionLabel,
  appHue,
  appInitial,
  appName,
  capRows,
  durationText,
  extendSample,
  filterConnections,
  outboundText,
  rateText,
  ratesBetween,
  ruleText,
  sampleOf,
  sortConnections,
  targetText,
  totalRates,
} from '../src/lib/conns-view.ts';

function conn(overrides = {}) {
  return {
    id: 'c1',
    process: 'chrome.exe',
    processPath: 'C:\\Program Files\\Google\\Chrome\\chrome.exe',
    host: 'example.com',
    port: 443,
    ruleName: '默认规则',
    action: 'proxy',
    outboundTag: '节点甲',
    chain: [],
    up: 0,
    down: 0,
    startAt: 1_000_000,
    ...overrides,
  };
}

test('appName / appInitial：进程名优先，其次取路径末段，都没有时如实说明', () => {
  assert.equal(appName(conn()), 'chrome.exe');
  assert.equal(appName(conn({ process: '', processPath: 'C:\\Tools\\curl.exe' })), 'curl.exe');
  assert.equal(appName(conn({ process: '', processPath: '/usr/bin/ssh' })), 'ssh');
  assert.equal(appName(conn({ process: '', processPath: '' })), '未知应用');
  assert.equal(appInitial(conn()), 'C');
  assert.equal(appInitial(conn({ process: '微信.exe' })), '微');
  assert.equal(appInitial(conn({ process: '', processPath: '' })), '?');
});

test('targetText：主机加端口，IPv6 加方括号，缺失时显示「—」', () => {
  assert.equal(targetText(conn()), 'example.com:443');
  assert.equal(targetText(conn({ host: '2001:db8::1', port: 53 })), '[2001:db8::1]:53');
  assert.equal(targetText(conn({ host: 'example.com', port: 0 })), 'example.com');
  assert.equal(targetText(conn({ host: '', port: 443 })), '—');
});

test('ruleText / actionKind / actionLabel：原样显示，未知动作不冒充代理', () => {
  assert.equal(ruleText(conn()), '默认规则');
  assert.equal(ruleText(conn({ ruleName: '' })), '—');
  assert.equal(actionKind('proxy'), 'proxy');
  assert.equal(actionKind('direct'), 'direct');
  assert.equal(actionKind('block'), 'block');
  assert.equal(actionKind('mystery'), 'other');
  assert.equal(actionLabel('proxy'), '代理');
  assert.equal(actionLabel('direct'), '直连');
  assert.equal(actionLabel('block'), '拦截');
  assert.equal(actionLabel('mystery'), 'mystery');
  assert.equal(actionLabel(''), '—');
});

test('outboundText：只有代理才显示出站，多跳按顺序连起来', () => {
  assert.equal(outboundText(conn()), '节点甲');
  assert.equal(outboundText(conn({ chain: ['中转', '落地'] })), '中转 → 落地');
  assert.equal(outboundText(conn({ chain: ['节点甲'] })), '节点甲');
  assert.equal(outboundText(conn({ action: 'direct', outboundTag: '__direct' })), '');
  assert.equal(outboundText(conn({ action: 'block', outboundTag: 'REJECT' })), '');
  assert.equal(outboundText(conn({ outboundTag: '', chain: [] })), '');
});

test('ratesBetween：按两次快照的累计字节差和间隔算速率', () => {
  const first = [conn({ id: 'a', up: 1000, down: 5000 }), conn({ id: 'b', up: 0, down: 0 })];
  const sample = sampleOf(first, 10_000);
  const next = [conn({ id: 'a', up: 3000, down: 9000 }), conn({ id: 'b', up: 0, down: 0 }), conn({ id: 'c', up: 700, down: 700 })];
  const rates = ratesBetween(sample, next, 12_000);
  assert.deepEqual(rates.get('a'), { up: 1000, down: 2000 });
  assert.deepEqual(rates.get('b'), { up: 0, down: 0 });
  // 新出现的连接没有上一次的数值，速率未知，不当成 0，也不把累计值当成速率。
  assert.equal(rates.has('c'), false);
});

test('ratesBetween：没有上一次快照、间隔不合理或计数回退时不编造数值', () => {
  const list = [conn({ id: 'a', up: 3000, down: 9000 })];
  assert.equal(ratesBetween(null, list, 12_000).size, 0);
  assert.equal(ratesBetween(sampleOf(list, 12_000), list, 12_000).size, 0);
  assert.equal(ratesBetween(sampleOf(list, 13_000), list, 12_000).size, 0);
  const shrunk = ratesBetween(sampleOf(list, 10_000), [conn({ id: 'a', up: 100, down: 9500 })], 11_000);
  assert.deepEqual(shrunk.get('a'), { up: 0, down: 500 });
});

test('totalRates：汇总已知速率；一条都不知道时返回 null', () => {
  assert.equal(totalRates(new Map()), null);
  const rates = new Map([
    ['a', { up: 10, down: 100 }],
    ['b', { up: 5, down: 50 }],
  ]);
  assert.deepEqual(totalRates(rates), { up: 15, down: 150 });
});

test('rateText：未知显示「—」，已知按速率格式', () => {
  assert.equal(rateText(undefined), '—');
  assert.equal(rateText(0), '0 B/s');
  assert.equal(rateText(2048), '2.00 KB/s');
});

test('filterConnections：文字匹配应用、路径、主机、端口、规则与出站，不分大小写', () => {
  const list = [
    conn({ id: 'a' }),
    conn({ id: 'b', process: 'Code.exe', processPath: 'D:\\Apps\\Code.exe', host: 'github.com', ruleName: '开发', action: 'direct', outboundTag: '__direct' }),
    conn({ id: 'c', process: 'ad.exe', processPath: '', host: 'ads.example.net', port: 80, ruleName: '', action: 'block', outboundTag: 'REJECT' }),
  ];
  const ids = (query, action = 'all') => filterConnections(list, query, action).map((item) => item.id);
  assert.deepEqual(ids(''), ['a', 'b', 'c']);
  assert.deepEqual(ids('  '), ['a', 'b', 'c']);
  assert.deepEqual(ids('CODE'), ['b']);
  assert.deepEqual(ids('github'), ['b']);
  assert.deepEqual(ids('program files'), ['a']);
  assert.deepEqual(ids('开发'), ['b']);
  assert.deepEqual(ids(':80'), ['c']);
  assert.deepEqual(ids('节点甲'), ['a']);
  assert.deepEqual(ids('没有这个'), []);
});

test('filterConnections：动作筛选与文字筛选同时生效', () => {
  const list = [
    conn({ id: 'a' }),
    conn({ id: 'b', action: 'direct' }),
    conn({ id: 'c', action: 'block', host: 'ads.example.net' }),
    conn({ id: 'd', action: 'mystery' }),
  ];
  const ids = (query, action) => filterConnections(list, query, action).map((item) => item.id);
  assert.deepEqual(ids('', 'proxy'), ['a']);
  assert.deepEqual(ids('', 'direct'), ['b']);
  assert.deepEqual(ids('', 'block'), ['c']);
  assert.deepEqual(ids('', 'all'), ['a', 'b', 'c', 'd']);
  assert.deepEqual(ids('example.com', 'block'), []);
  assert.deepEqual(ids('ads', 'block'), ['c']);
});

test('sortConnections：新连接在前，同一时刻按 id 稳定排序，不改动原数组', () => {
  const list = [conn({ id: 'b', startAt: 100 }), conn({ id: 'a', startAt: 100 }), conn({ id: 'c', startAt: 300 })];
  assert.deepEqual(sortConnections(list).map((item) => item.id), ['c', 'a', 'b']);
  assert.deepEqual(list.map((item) => item.id), ['b', 'a', 'c']);
});

test('capRows：超过上限时截断并报告隐藏了多少条', () => {
  const list = Array.from({ length: 5 }, (_, index) => conn({ id: `c${index}` }));
  assert.deepEqual(capRows(list, 5), { rows: list, hidden: 0 });
  const capped = capRows(list, 3);
  assert.deepEqual(capped.rows.map((item) => item.id), ['c0', 'c1', 'c2']);
  assert.equal(capped.hidden, 2);
  assert.equal(MAX_ROWS, 300);
});

test('durationText：分秒，超过一小时带小时；时间缺失或倒挂时显示「—」', () => {
  assert.equal(durationText(1_000_000, 1_000_000), '00:00');
  assert.equal(durationText(1_000_000, 1_065_900), '01:05');
  assert.equal(durationText(1_000_000, 1_000_000 + 3_725_000), '1:02:05');
  assert.equal(durationText(0, 1_000_000), '—');
  assert.equal(durationText(2_000_000, 1_000_000), '00:00');
  assert.equal(durationText(Number.NaN, 1_000_000), '—');
});

test('appHue：同一个应用总是同一个色相，落在 0–359 之间', () => {
  const a = appHue(conn());
  assert.equal(a, appHue(conn({ id: 'other', host: 'elsewhere.example' })));
  assert.ok(Number.isInteger(a) && a >= 0 && a < 360);
  assert.notEqual(a, appHue(conn({ process: 'steam.exe' })));
  const unknown = appHue(conn({ process: '', processPath: '' }));
  assert.ok(Number.isInteger(unknown) && unknown >= 0 && unknown < 360);
});

test('extendSample：两次快照离得太近时只补记新连接，已有的基准不动', () => {
  const sample = sampleOf([conn({ id: 'a', up: 100, down: 1000 })], 10_000);
  const extended = extendSample(sample, [conn({ id: 'a', up: 900, down: 9000 }), conn({ id: 'b', up: 50, down: 500 })]);
  assert.equal(extended.at, 10_000);
  assert.deepEqual(extended.totals.get('a'), { up: 100, down: 1000 });
  assert.deepEqual(extended.totals.get('b'), { up: 50, down: 500 });
  assert.equal(sample.totals.has('b'), false);
  // 没有新连接时原样返回。
  assert.equal(extendSample(sample, [conn({ id: 'a', up: 900, down: 9000 })]), sample);
  // 补记之后，下一次快照就能给新连接算出速率。
  const rates = ratesBetween(extended, [conn({ id: 'b', up: 1050, down: 2500 })], 11_000);
  assert.deepEqual(rates.get('b'), { up: 1000, down: 2000 });
});

test('outboundText：能认出的出站 tag 换成名称，认不出的原样显示', () => {
  const names = new Map([['n-1', '节点甲'], ['proxy', '代理']]);
  const base = { id: 'c', process: '', processPath: '', host: 'a.example.invalid', port: 443, ruleName: '', ruleId: '', action: 'proxy', outboundTag: 'n-1', chain: ['n-1', 'proxy'], up: 0, down: 0, startAt: 0 };
  assert.equal(outboundText(base, names), '节点甲 → 代理');
  assert.equal(outboundText({ ...base, chain: ['n-9', 'proxy'] }, names), 'n-9 → 代理');
  assert.equal(outboundText({ ...base, chain: [] }, names), '节点甲');
  assert.equal(outboundText(base), 'n-1 → proxy');
  assert.equal(outboundText({ ...base, action: 'direct' }, names), '');
});

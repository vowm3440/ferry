import assert from 'node:assert/strict';
import { test } from 'node:test';
import { usageRows } from '../src/lib/overview-view.ts';

const GIB = 1024 ** 3;
const DAY = 24 * 60 * 60 * 1000;
const NOW = new Date(2026, 10, 15, 9, 0, 0).getTime();

function sub(patch) {
  return {
    id: 's-1',
    name: '机场 A',
    urlMasked: 'https://example.com/…',
    nodeCount: 12,
    groupCount: 2,
    usedBytes: 0,
    totalBytes: 0,
    expiresAt: 0,
    lastUpdatedAt: NOW,
    intervalHours: 24,
    autoUpdate: true,
    userAgent: '',
    status: 'ok',
    skipped: [],
    warnings: [],
    ...patch,
  };
}

test('usageRows：有总量时给出百分比和进度条', () => {
  const { rows, hidden } = usageRows([sub({ usedBytes: 42 * GIB, totalBytes: 100 * GIB, expiresAt: NOW + 30 * DAY })], NOW, 3);
  assert.equal(hidden, 0);
  assert.equal(rows.length, 1);
  const row = rows[0];
  assert.equal(row.id, 's-1');
  assert.equal(row.name, '机场 A');
  assert.equal(row.hasBar, true);
  assert.equal(row.percent, 42);
  assert.equal(row.percentText, '42%');
  assert.equal(row.usageText, '42.0 GB / 100 GB');
  assert.equal(row.level, 'ok');
  assert.equal(row.expiryRemain, '还有 30 天');
  assert.equal(row.expiryLevel, 'ok');
  assert.equal(row.nodesText, '12');
  assert.equal(row.autoText, '每 24 小时');
});

test('usageRows：服务器没给用量时显示「—」，不画进度条', () => {
  const [row] = usageRows([sub({})], NOW, 3).rows;
  assert.equal(row.hasBar, false);
  assert.equal(row.percentText, '');
  assert.equal(row.usageText, '—');
  assert.equal(row.expiryText, '—');
  assert.equal(row.expiryRemain, '');
});

test('usageRows：只知道已用量时照实显示已用，不编造总量', () => {
  const [row] = usageRows([sub({ usedBytes: 3 * GIB })], NOW, 3).rows;
  assert.equal(row.hasBar, false);
  assert.equal(row.usageText, '已用 3.00 GB');
});

test('usageRows：自动更新的三种说法', () => {
  const [manual] = usageRows([sub({ urlMasked: '', autoUpdate: false, intervalHours: 0 })], NOW, 3).rows;
  assert.equal(manual.autoText, '—');
  const [off] = usageRows([sub({ autoUpdate: false })], NOW, 3).rows;
  assert.equal(off.autoText, '已关闭');
  const [on] = usageRows([sub({ intervalHours: 6 })], NOW, 3).rows;
  assert.equal(on.autoText, '每 6 小时');
});

test('usageRows：超出上限的订阅只计数，不改动传入的数组', () => {
  const list = [1, 2, 3, 4, 5].map((n) => sub({ id: `s-${n}`, name: `订阅 ${n}` }));
  const frozen = Object.freeze([...list]);
  const { rows, hidden } = usageRows(frozen, NOW, 3);
  assert.deepEqual(rows.map((row) => row.id), ['s-1', 's-2', 's-3']);
  assert.equal(hidden, 2);
  assert.deepEqual(usageRows([], NOW, 3), { rows: [], hidden: 0 });
});

test('usageRows：用量与到期的告警级别原样传出', () => {
  const [row] = usageRows([sub({ usedBytes: 96 * GIB, totalBytes: 100 * GIB, expiresAt: NOW + 2 * DAY })], NOW, 3).rows;
  assert.equal(row.level, 'err');
  assert.equal(row.expiryLevel, 'warn');
});

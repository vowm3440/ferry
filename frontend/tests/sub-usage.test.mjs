import assert from 'node:assert/strict';
import { test } from 'node:test';
import { expiryView, usageView } from '../src/lib/sub-usage.ts';

const GB = 1024 ** 3;
const DAY = 24 * 60 * 60 * 1000;
const NOW = new Date(2026, 10, 14, 12, 0, 0).getTime();

test('usageView：没有任何流量信息时显示「—」，不显示 0', () => {
  for (const [used, total] of [
    [0, 0],
    [-1, 0],
    [Number.NaN, Number.NaN],
  ]) {
    const view = usageView(used, total);
    assert.equal(view.empty, true);
    assert.equal(view.hasTotal, false);
    assert.equal(view.usedText, '—');
    assert.equal(view.totalText, '—');
    assert.equal(view.percent, 0);
    assert.equal(view.percentText, '');
    assert.equal(view.level, 'ok');
  }
});

test('usageView：只知道已用量时不画进度条', () => {
  const view = usageView(12 * GB, 0);
  assert.equal(view.empty, false);
  assert.equal(view.hasTotal, false);
  assert.equal(view.usedText, '12.0 GB');
  assert.equal(view.totalText, '—');
  assert.equal(view.percent, 0);
  assert.equal(view.percentText, '');
});

test('usageView：已用 0 但知道总量时显示 0 B 与 0%', () => {
  const view = usageView(0, 100 * GB);
  assert.equal(view.empty, false);
  assert.equal(view.hasTotal, true);
  assert.equal(view.usedText, '0 B');
  assert.equal(view.totalText, '100 GB');
  assert.equal(view.percent, 0);
  assert.equal(view.percentText, '0%');
  assert.equal(view.level, 'ok');
});

test('usageView：百分比向下取整，80% 起提醒，95% 起告警', () => {
  assert.deepEqual(pick(usageView(42.9 * GB, 100 * GB)), { percentText: '42%', level: 'ok' });
  assert.deepEqual(pick(usageView(79.99 * GB, 100 * GB)), { percentText: '79%', level: 'ok' });
  assert.deepEqual(pick(usageView(80 * GB, 100 * GB)), { percentText: '80%', level: 'warn' });
  assert.deepEqual(pick(usageView(94.9 * GB, 100 * GB)), { percentText: '94%', level: 'warn' });
  assert.deepEqual(pick(usageView(95 * GB, 100 * GB)), { percentText: '95%', level: 'err' });
  assert.deepEqual(pick(usageView(99.99 * GB, 100 * GB)), { percentText: '99%', level: 'err' });
});

test('usageView：超额时进度条封顶在 100，但如实显示已用量', () => {
  const view = usageView(130 * GB, 100 * GB);
  assert.equal(view.percent, 100);
  assert.equal(view.percentText, '100%');
  assert.equal(view.usedText, '130 GB');
  assert.equal(view.level, 'err');
});

test('expiryView：没有到期时间时显示「—」', () => {
  for (const value of [0, -1, Number.NaN]) {
    assert.deepEqual(expiryView(value, NOW), { known: false, dateText: '—', remainText: '', level: 'ok' });
  }
});

test('expiryView：按剩余整天数说明，不足 7 天提醒', () => {
  assert.deepEqual(expiryView(NOW + 30 * DAY, NOW), {
    known: true,
    dateText: '2026-12-14',
    remainText: '还有 30 天',
    level: 'ok',
  });
  assert.equal(expiryView(NOW + 7 * DAY, NOW).level, 'ok');
  assert.deepEqual(pickExpiry(expiryView(NOW + 7 * DAY - 1, NOW)), { remainText: '还有 6 天', level: 'warn' });
  assert.deepEqual(pickExpiry(expiryView(NOW + DAY, NOW)), { remainText: '还有 1 天', level: 'warn' });
});

test('expiryView：不足一天与已过期', () => {
  assert.deepEqual(pickExpiry(expiryView(NOW + DAY - 1, NOW)), { remainText: '不足 1 天', level: 'err' });
  assert.deepEqual(pickExpiry(expiryView(NOW + 1, NOW)), { remainText: '不足 1 天', level: 'err' });
  assert.deepEqual(pickExpiry(expiryView(NOW, NOW)), { remainText: '已过期', level: 'err' });
  assert.deepEqual(pickExpiry(expiryView(NOW - 3 * DAY, NOW)), { remainText: '已过期', level: 'err' });
});

function pick(view) {
  return { percentText: view.percentText, level: view.level };
}

function pickExpiry(view) {
  return { remainText: view.remainText, level: view.level };
}

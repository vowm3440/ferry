import assert from 'node:assert/strict';
import { test } from 'node:test';
import { formatAgo, formatClock, formatCountdown, formatDate, formatDateTime } from '../src/lib/time.ts';

const SECOND = 1000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/** 用本地时间构造时间戳，测试结果与运行机器的时区无关。 */
const at = (year, month, day, hour = 0, minute = 0, second = 0) =>
  new Date(year, month - 1, day, hour, minute, second).getTime();

test('formatClock：本地时间的时分秒，补零', () => {
  assert.equal(formatClock(at(2026, 11, 14, 9, 5, 7)), '09:05:07');
  assert.equal(formatClock(at(2026, 11, 14, 23, 59, 59)), '23:59:59');
});

test('formatDateTime / formatDate：年月日补零', () => {
  assert.equal(formatDateTime(at(2026, 1, 2, 3, 4, 5)), '2026-01-02 03:04');
  assert.equal(formatDate(at(2026, 1, 2, 3, 4, 5)), '2026-01-02');
  assert.equal(formatDate(at(2026, 12, 31, 23, 59)), '2026-12-31');
});

test('时间戳缺失或无效时显示「—」，不显示 1970 年', () => {
  for (const format of [formatClock, formatDateTime, formatDate]) {
    for (const value of [0, -1, Number.NaN, Number.POSITIVE_INFINITY]) {
      assert.equal(format(value), '—', `${format.name}(${String(value)})`);
    }
  }
});

test('formatCountdown：分秒倒计时，不会出现负数', () => {
  assert.equal(formatCountdown(0), '00:00');
  assert.equal(formatCountdown(-5 * SECOND), '00:00');
  assert.equal(formatCountdown(Number.NaN), '00:00');
  assert.equal(formatCountdown(9 * MINUTE + 41 * SECOND), '09:41');
  assert.equal(formatCountdown(75 * MINUTE + 3 * SECOND), '75:03');
});

test('formatCountdown：不足一秒的剩余时间向上取整，到点才归零', () => {
  assert.equal(formatCountdown(1), '00:01');
  assert.equal(formatCountdown(SECOND + 1), '00:02');
  assert.equal(formatCountdown(SECOND), '00:01');
});

test('formatAgo：按分钟、小时、天取整', () => {
  const now = at(2026, 11, 14, 12, 0, 0);
  assert.equal(formatAgo(now, now), '刚刚');
  assert.equal(formatAgo(now - 59 * SECOND, now), '刚刚');
  assert.equal(formatAgo(now - MINUTE, now), '1 分钟前');
  assert.equal(formatAgo(now - 59 * MINUTE, now), '59 分钟前');
  assert.equal(formatAgo(now - HOUR, now), '1 小时前');
  assert.equal(formatAgo(now - 23 * HOUR, now), '23 小时前');
  assert.equal(formatAgo(now - DAY, now), '1 天前');
  assert.equal(formatAgo(now - 40 * DAY, now), '40 天前');
});

test('formatAgo：未来的时间按「刚刚」处理，缺失时显示「—」', () => {
  const now = at(2026, 11, 14, 12, 0, 0);
  assert.equal(formatAgo(now + HOUR, now), '刚刚');
  assert.equal(formatAgo(0, now), '—');
  assert.equal(formatAgo(Number.NaN, now), '—');
});

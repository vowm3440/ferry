import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  formatBytes,
  formatCount,
  formatDuration,
  formatDurationText,
  formatLatency,
  formatRate,
  latencyLevel,
  splitUnit,
} from '../src/lib/format.ts';

const KB = 1024;
const MB = KB * 1024;
const GB = MB * 1024;
const TB = GB * 1024;
const SECOND = 1000;
const MINUTE = 60 * SECOND;
const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;

/** 逐条比对「输入 → 期望输出」。 */
function expectAll(fn, cases) {
  for (const [input, expected] of cases) {
    assert.equal(fn(input), expected, `${fn.name}(${String(input)})`);
  }
}

test('formatBytes：无效或非正数一律显示 0 B', () => {
  expectAll(formatBytes, [
    [0, '0 B'],
    [-5, '0 B'],
    [Number.NaN, '0 B'],
    [Number.POSITIVE_INFINITY, '0 B'],
  ]);
});

test('formatBytes：按 1024 进制换算，位数随数值收窄', () => {
  expectAll(formatBytes, [
    [1, '1 B'],
    [1023, '1023 B'],
    [KB, '1.00 KB'],
    [1536, '1.50 KB'],
    [9.99 * KB, '9.99 KB'],
    [10 * KB, '10.0 KB'],
    [100 * KB, '100 KB'],
    [MB, '1.00 MB'],
    [46.2 * MB, '46.2 MB'],
    [GB, '1.00 GB'],
    [TB, '1.00 TB'],
    [1500 * TB, '1500 TB'],
  ]);
});

test('formatBytes：四舍五入到 1024 时换用更大的单位', () => {
  expectAll(formatBytes, [
    [1023.6, '1.00 KB'],
    [MB - 1, '1.00 MB'],
    [GB - 1, '1.00 GB'],
    [TB - 1, '1.00 TB'],
  ]);
});

test('formatBytes：进位后不会多出一位小数', () => {
  expectAll(formatBytes, [
    [9.999 * KB, '10.0 KB'],
    [99.99 * KB, '100 KB'],
    [999.9 * MB, '1000 MB'],
  ]);
});

test('formatRate：在字节数后面加上 /s', () => {
  expectAll(formatRate, [
    [0, '0 B/s'],
    [2 * KB, '2.00 KB/s'],
    [1.5 * MB, '1.50 MB/s'],
  ]);
});

test('splitUnit：把数值和单位拆开', () => {
  assert.deepEqual(splitUnit('46.2 MB'), { value: '46.2', unit: 'MB' });
  assert.deepEqual(splitUnit('1.50 KB/s'), { value: '1.50', unit: 'KB/s' });
  assert.deepEqual(splitUnit('—'), { value: '—', unit: '' });
});

test('formatCount：千分位，无效值显示 0', () => {
  expectAll(formatCount, [
    [0, '0'],
    [-1, '0'],
    [Number.NaN, '0'],
    [12.6, '13'],
    [1234567, '1,234,567'],
  ]);
});

test('formatDuration：时分秒，超过一天时前置天数', () => {
  expectAll(formatDuration, [
    [0, '00:00:00'],
    [-1, '00:00:00'],
    [Number.NaN, '00:00:00'],
    [999, '00:00:00'],
    [SECOND, '00:00:01'],
    [HOUR + 2 * MINUTE + 3 * SECOND, '01:02:03'],
    [DAY + HOUR + MINUTE + SECOND, '1 天 01:01:01'],
  ]);
});

test('formatDurationText：只保留最大的两级单位', () => {
  expectAll(formatDurationText, [
    [0, '0 秒'],
    [Number.NaN, '0 秒'],
    [5 * SECOND, '5 秒'],
    [MINUTE + 5 * SECOND, '1 分 5 秒'],
    [2 * HOUR + 5 * MINUTE + 9 * SECOND, '2 小时 5 分'],
    [DAY + HOUR + 30 * MINUTE, '1 天 1 小时'],
  ]);
});

test('formatLatency：负数表示尚未测速', () => {
  expectAll(formatLatency, [
    [-1, '未测速'],
    [Number.NaN, '未测速'],
    [0, '0 ms'],
    [123.4, '123 ms'],
  ]);
});

test('latencyLevel：按 200 / 500 毫秒分档', () => {
  expectAll(latencyLevel, [
    [-1, 'none'],
    [Number.NaN, 'none'],
    [0, 'ok'],
    [199, 'ok'],
    [200, 'warn'],
    [499, 'warn'],
    [500, 'err'],
  ]);
});

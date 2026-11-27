import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  LOG_RING_MAX,
  exportFileName,
  filterLogs,
  lastSeq,
  levelClass,
  levelLabel,
  levelRank,
  mergeEntries,
  tailRows,
} from '../src/lib/logs-view.ts';

const entry = (seq, level = 'info', message = `第 ${seq} 条`) => ({ seq, time: 1_700_000_000_000 + seq * 1000, level, message });
const seqs = (list) => list.map((item) => item.seq);

test('mergeEntries：事件接在后面，重复的序号只留一条', () => {
  const ring = [entry(1), entry(2), entry(3)];
  const merged = mergeEntries(ring, [entry(3), entry(4), entry(5)], 0);
  assert.deepEqual(seqs(merged), [1, 2, 3, 4, 5]);
  assert.deepEqual(seqs(ring), [1, 2, 3]);
});

test('mergeEntries：没有新内容时返回原数组，不触发多余的刷新', () => {
  const ring = [entry(1), entry(2)];
  assert.equal(mergeEntries(ring, [], 0), ring);
  assert.equal(mergeEntries(ring, [entry(1), entry(2)], 0), ring);
});

test('mergeEntries：暂停期间漏掉的日志由快照补回，按序号插回中间', () => {
  // 暂停前收到 1–3，恢复后事件先到 7–8，随后快照带回 1–8。
  const ring = mergeEntries([entry(1), entry(2), entry(3)], [entry(7), entry(8)], 0);
  const merged = mergeEntries(ring, [1, 2, 3, 4, 5, 6, 7, 8].map((seq) => entry(seq)), 0);
  assert.deepEqual(seqs(merged), [1, 2, 3, 4, 5, 6, 7, 8]);
});

test('mergeEntries：清空显示之后，早于清空点的日志不再回来', () => {
  const merged = mergeEntries([], [entry(4), entry(5), entry(6), entry(7)], 5);
  assert.deepEqual(seqs(merged), [6, 7]);
  assert.deepEqual(seqs(mergeEntries([], [entry(0), entry(-1), entry(1)], 0)), [1]);
});

test('mergeEntries：只保留最新的若干条', () => {
  const ring = [entry(1), entry(2), entry(3)];
  assert.deepEqual(seqs(mergeEntries(ring, [entry(4), entry(5)], 0, 3)), [3, 4, 5]);
  const many = Array.from({ length: LOG_RING_MAX + 50 }, (_, index) => entry(index + 1));
  const capped = mergeEntries([], many, 0);
  assert.equal(capped.length, LOG_RING_MAX);
  assert.equal(capped[0].seq, 51);
  assert.equal(LOG_RING_MAX, 2000);
});

test('mergeEntries：乱序的输入按序号排好', () => {
  assert.deepEqual(seqs(mergeEntries([entry(2)], [entry(5), entry(1), entry(3)], 0)), [1, 2, 3, 5]);
});

test('lastSeq：最后一条的序号，空列表为 0', () => {
  assert.equal(lastSeq([]), 0);
  assert.equal(lastSeq([entry(3), entry(9)]), 9);
});

test('levelRank / levelClass / levelLabel：兼容大小写与 warning，未知级别按 info 对待', () => {
  assert.ok(levelRank('trace') < levelRank('debug'));
  assert.ok(levelRank('debug') < levelRank('info'));
  assert.ok(levelRank('info') < levelRank('warn'));
  assert.equal(levelRank('warning'), levelRank('warn'));
  assert.ok(levelRank('warn') < levelRank('error'));
  assert.ok(levelRank('error') < levelRank('fatal'));
  assert.ok(levelRank('fatal') < levelRank('panic'));
  assert.equal(levelRank('ERROR'), levelRank('error'));
  assert.equal(levelRank('whatever'), levelRank('info'));
  assert.equal(levelClass('trace'), 'debug');
  assert.equal(levelClass('debug'), 'debug');
  assert.equal(levelClass('info'), 'info');
  assert.equal(levelClass('warning'), 'warn');
  assert.equal(levelClass('fatal'), 'error');
  assert.equal(levelClass('panic'), 'error');
  assert.equal(levelClass(''), 'info');
  assert.equal(levelLabel('info'), 'INFO');
  assert.equal(levelLabel('warning'), 'WARN');
  assert.equal(levelLabel('Custom'), 'CUSTOM');
  assert.equal(levelLabel(''), '—');
});

test('filterLogs：最低级别与搜索同时生效，搜索不分大小写', () => {
  const list = [
    entry(1, 'trace', 'dns: exchange example.com'),
    entry(2, 'debug', 'router: match rule'),
    entry(3, 'info', 'inbound/mixed: connection from 127.0.0.1'),
    entry(4, 'warn', 'outbound: Dial timeout'),
    entry(5, 'error', 'outbound: connection refused'),
    entry(6, 'fatal', 'start service: bind failed'),
  ];
  assert.deepEqual(seqs(filterLogs(list, 'debug', '')), [1, 2, 3, 4, 5, 6]);
  assert.deepEqual(seqs(filterLogs(list, 'info', '')), [3, 4, 5, 6]);
  assert.deepEqual(seqs(filterLogs(list, 'warn', '')), [4, 5, 6]);
  assert.deepEqual(seqs(filterLogs(list, 'error', '')), [5, 6]);
  assert.deepEqual(seqs(filterLogs(list, 'debug', 'CONNECTION')), [3, 5]);
  assert.deepEqual(seqs(filterLogs(list, 'error', 'connection')), [5]);
  assert.deepEqual(seqs(filterLogs(list, 'debug', '  dial ')), [4]);
  assert.deepEqual(seqs(filterLogs(list, 'debug', 'warn')), [4]);
});

test('tailRows：只渲染最新的若干条，并报告更早的还有多少', () => {
  const list = [entry(1), entry(2), entry(3), entry(4)];
  assert.deepEqual(tailRows(list, 4), { rows: list, hidden: 0 });
  const tail = tailRows(list, 3);
  assert.deepEqual(seqs(tail.rows), [2, 3, 4]);
  assert.equal(tail.hidden, 1);
});

test('exportFileName：带本地时间戳，避免撞上已有文件', () => {
  const at = new Date(2026, 10, 5, 9, 5, 7).getTime();
  assert.equal(exportFileName(at), 'ferry-log-20261105-090507.log');
});

import assert from 'node:assert/strict';
import { test } from 'node:test';
import { parseDemoLinks } from '../src/lib/api/demo-links.ts';
import { parseSubscription, parseSubscriptions } from '../src/lib/api/parse.ts';

test('parseSubscription：完整负载保留约定字段与跳过原因', () => {
  const value = parseSubscription({
    id: 's-1',
    name: '我的节点',
    urlMasked: '',
    nodeCount: 2,
    groupCount: 1,
    usedBytes: 1024,
    totalBytes: 4096,
    expiresAt: 1800000000000,
    lastUpdatedAt: 1700000000000,
    intervalHours: 24,
    autoUpdate: true,
    userAgent: 'clash.meta',
    status: 'ok',
    url: 'https://secret.example.invalid/token',
    skipped: [{ line: 2, reason: '暂不支持 vmess 协议' }, { line: 0, reason: '节点已存在：甲' }, { line: 5 }, 'x'],
    warnings: ['此订阅使用未加密的 HTTP', '', 7],
  });
  assert.deepEqual(value, {
    id: 's-1',
    name: '我的节点',
    urlMasked: '',
    nodeCount: 2,
    groupCount: 1,
    usedBytes: 1024,
    totalBytes: 4096,
    expiresAt: 1800000000000,
    lastUpdatedAt: 1700000000000,
    intervalHours: 24,
    autoUpdate: true,
    userAgent: 'clash.meta',
    status: 'ok',
    skipped: [
      { line: 2, reason: '暂不支持 vmess 协议' },
      { line: 0, reason: '节点已存在：甲' },
    ],
    warnings: ['此订阅使用未加密的 HTTP'],
  });
});

test('parseSubscription：字段缺失时使用安全默认值，error 只在非空时保留', () => {
  assert.deepEqual(parseSubscription(null), {
    id: '',
    name: '',
    urlMasked: '',
    nodeCount: 0,
    groupCount: 0,
    usedBytes: 0,
    totalBytes: 0,
    expiresAt: 0,
    lastUpdatedAt: 0,
    intervalHours: 0,
    autoUpdate: false,
    userAgent: '',
    status: 'ok',
    skipped: [],
    warnings: [],
  });
  assert.equal(parseSubscription({ error: '下载失败' }).error, '下载失败');
  assert.equal('error' in parseSubscription({ error: '' }), false);
});

test('parseSubscriptions：不是数组时为空，丢弃没有 id 的条目', () => {
  for (const value of [null, undefined, 'x', 3, {}]) assert.deepEqual(parseSubscriptions(value), []);
  const list = parseSubscriptions([{ id: 'a', name: '甲' }, { name: '没有 id' }, null]);
  assert.deepEqual(
    list.map((item) => item.id),
    ['a'],
  );
});

test('parseDemoLinks：识别支持的协议，名称取自片段', () => {
  const { links, skipped } = parseDemoLinks(
    'vless://id@a.example.invalid:443?security=tls#%E7%94%B2\r\n\r\nss://x@b.example.invalid:8388\nsocks5://[2001:db8::1]:1080#v6',
  );
  assert.deepEqual(skipped, []);
  assert.deepEqual(links, [
    { name: '甲', type: 'vless', server: 'a.example.invalid', port: 443 },
    { name: 'b.example.invalid:8388', type: 'shadowsocks', server: 'b.example.invalid', port: 8388 },
    { name: 'v6', type: 'socks', server: '2001:db8::1', port: 1080 },
  ]);
});

test('parseDemoLinks：其余内容带行号跳过，原因里不含链接内容', () => {
  const { links, skipped } = parseDemoLinks('hello\nvmess://SECRET\ntrojan://SECRET@host\nvless://a b:1\ntrojan://p@h.invalid:443#%E0%A4%A');
  assert.deepEqual(
    skipped.map((item) => item.line),
    [1, 2, 3, 4],
  );
  assert.deepEqual(
    skipped.map((item) => item.reason),
    ['无法识别的内容', '暂不支持 vmess 协议', '演示模式只识别带地址与端口的链接', '链接无法解析'],
  );
  assert.equal(skipped.some((item) => item.reason.includes('SECRET')), false);
  assert.equal(links[0]?.name, 'h.invalid:443');
});

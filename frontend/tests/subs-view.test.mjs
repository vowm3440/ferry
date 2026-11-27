// 订阅页与分享对话框的纯逻辑：输入预检、状态文案、二维码容量、局域网分享倒计时。
import assert from 'node:assert/strict';
import { test } from 'node:test';

import {
  MSG_HOURS,
  MSG_HTTP_WARNING,
  MSG_NAME_INVALID,
  MSG_QR_TOO_LARGE,
  MSG_URL_INVALID,
  QR_IMAGE_MAX_BYTES,
  SHARE_FORMAT_OPTIONS,
  autoUpdateText,
  canUpdate,
  checkSubName,
  checkSubUrl,
  describeSkipped,
  isPlainHttp,
  lanRemainingMs,
  parseHours,
  progressText,
  qrPayloadProblem,
  statusPill,
  svgDataUrl,
} from '../src/lib/subs-view.ts';

const sub = (patch = {}) => ({
  id: 's-1',
  name: '示例',
  urlMasked: 'https://sub.example.invalid/****',
  nodeCount: 2,
  groupCount: 0,
  usedBytes: 0,
  totalBytes: 0,
  expiresAt: 0,
  lastUpdatedAt: 0,
  intervalHours: 24,
  autoUpdate: true,
  userAgent: '',
  status: 'ok',
  skipped: [],
  warnings: [],
  ...patch,
});

test('订阅地址：http 与 https 通过，首尾空白忽略', () => {
  assert.equal(checkSubUrl('https://sub.example.invalid/a?token=x'), '');
  assert.equal(checkSubUrl('  http://sub.example.invalid/a  '), '');
});

test('订阅地址：空内容给出填写提示', () => {
  assert.equal(checkSubUrl('   '), '请填写订阅地址');
});

test('订阅地址：其它协议、用户信息、片段、无主机一律拒绝，文案与后端一致', () => {
  for (const raw of [
    'ftp://sub.example.invalid/a',
    'vless://id@sub.example.invalid:443',
    'https://user:pass@sub.example.invalid/a',
    'https://user@sub.example.invalid/a',
    'https://sub.example.invalid/a#frag',
    'https:///a',
    'not a url',
  ]) {
    assert.equal(checkSubUrl(raw), MSG_URL_INVALID, raw);
  }
});

test('订阅地址：超过 4096 字节拒绝，恰好 4096 通过', () => {
  const head = 'https://sub.example.invalid/';
  assert.equal(checkSubUrl(head + 'a'.repeat(4096 - head.length)), '');
  assert.equal(checkSubUrl(head + 'a'.repeat(4097 - head.length)), MSG_URL_INVALID);
});

test('明文 HTTP 判断只看协议，不区分大小写', () => {
  assert.equal(isPlainHttp('http://sub.example.invalid/a'), true);
  assert.equal(isPlainHttp(' HTTP://sub.example.invalid/a'), true);
  assert.equal(isPlainHttp('https://sub.example.invalid/a'), false);
  assert.equal(isPlainHttp('乱写'), false);
  assert.match(MSG_HTTP_WARNING, /未加密的 HTTP/);
});

test('名称：空与 64 个字符以内通过，按码点计数', () => {
  assert.equal(checkSubName(''), '');
  assert.equal(checkSubName('渡'.repeat(64)), '');
  assert.equal(checkSubName('😀'.repeat(64)), '');
  assert.equal(checkSubName('渡'.repeat(65)), MSG_NAME_INVALID);
});

test('名称：含控制字符拒绝', () => {
  assert.equal(checkSubName('a\u0007b'), MSG_NAME_INVALID);
  assert.equal(checkSubName('a\nb'), MSG_NAME_INVALID);
});

test('更新间隔：只接受 1 到 720 的整数', () => {
  assert.equal(parseHours('1'), 1);
  assert.equal(parseHours(' 720 '), 720);
  assert.equal(parseHours('24'), 24);
  for (const text of ['', '0', '721', '-1', '1.5', '12h', '1e2']) assert.equal(parseHours(text), null, text);
  assert.match(MSG_HOURS, /1 到 720 小时/);
});

test('进度文案：已知阶段有文字，结束与未知阶段为空', () => {
  assert.equal(progressText('fetch'), '正在下载…');
  assert.equal(progressText('parse'), '正在解析…');
  assert.equal(progressText('apply'), '正在应用…');
  assert.equal(progressText('done'), '');
  assert.equal(progressText('error'), '');
  assert.equal(progressText('别的'), '');
});

test('状态标签：更新中、失败、手动导入、正常', () => {
  assert.deepEqual(statusPill(sub({ status: 'updating' })), { text: '更新中', tone: 'phase' });
  assert.deepEqual(statusPill(sub({ status: 'error' })), { text: '更新失败', tone: 'block' });
  assert.deepEqual(statusPill(sub({ urlMasked: '' })), { text: '手动导入', tone: '' });
  assert.deepEqual(statusPill(sub()), { text: '订阅', tone: 'proxy' });
});

test('可更新：只有带订阅地址的才行', () => {
  assert.equal(canUpdate(sub()), true);
  assert.equal(canUpdate(sub({ urlMasked: '' })), false);
});

test('自动更新文案', () => {
  assert.equal(autoUpdateText(sub()), '每 24 小时');
  assert.equal(autoUpdateText(sub({ autoUpdate: false })), '已关闭');
  assert.equal(autoUpdateText(sub({ urlMasked: '', autoUpdate: false })), '—');
  assert.equal(autoUpdateText(sub({ intervalHours: 0 })), '已开启');
});

test('跳过行：有行号时带上行号', () => {
  assert.equal(describeSkipped({ line: 3, reason: '暂不支持 vmess 协议' }), '第 3 行：暂不支持 vmess 协议');
  assert.equal(describeSkipped({ line: 0, reason: '节点已存在：甲' }), '节点已存在：甲');
});

test('分享格式：四种，顺序与设计稿一致', () => {
  assert.deepEqual(
    SHARE_FORMAT_OPTIONS.map((option) => [option.value, option.label]),
    [
      ['clash', 'Clash'],
      ['singbox', 'sing-box'],
      ['base64', 'Base64'],
      ['uri', '链接'],
    ],
  );
});

test('二维码容量：1 到 4096 个 UTF-8 字节', () => {
  assert.equal(qrPayloadProblem('a'), '');
  assert.equal(qrPayloadProblem('a'.repeat(4096)), '');
  assert.equal(qrPayloadProblem('a'.repeat(4097)), MSG_QR_TOO_LARGE);
  // 一个汉字 3 字节：1366 个就超出。
  assert.equal(qrPayloadProblem('渡'.repeat(1365)), '');
  assert.equal(qrPayloadProblem('渡'.repeat(1366)), MSG_QR_TOO_LARGE);
  assert.equal(qrPayloadProblem(''), '没有可生成二维码的内容');
  assert.equal(QR_IMAGE_MAX_BYTES, 8 * 1024 * 1024);
});

test('二维码图片地址：只接受 svg 文本', () => {
  assert.equal(svgDataUrl('<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>'), 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>'));
  assert.equal(svgDataUrl('  <svg></svg>'), 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent('<svg></svg>'));
  assert.equal(svgDataUrl('<script>1</script>'), null);
  assert.equal(svgDataUrl(''), null);
});

test('局域网分享剩余时间：未开启或已过期为 0', () => {
  assert.equal(lanRemainingMs({ url: 'http://192.0.2.1:1/s/x', expiresAt: 10_000, active: true }, 4_000), 6_000);
  assert.equal(lanRemainingMs({ url: 'http://192.0.2.1:1/s/x', expiresAt: 10_000, active: true }, 10_000), 0);
  assert.equal(lanRemainingMs({ url: '', expiresAt: 10_000, active: false }, 4_000), 0);
  assert.equal(lanRemainingMs(null, 4_000), 0);
});

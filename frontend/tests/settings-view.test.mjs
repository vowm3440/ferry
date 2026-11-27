import assert from 'node:assert/strict';
import { test } from 'node:test';
import { cloneSettings, createDefaultSettings } from '../src/lib/api/defaults.ts';
import { SUCCESS, failure, failureMessage } from '../src/lib/result.ts';
import {
  DNS_STRATEGY_OPTIONS,
  LOG_LEVEL_OPTIONS,
  optionLabel,
  optionsWith,
  parsePort,
  sameSettings,
} from '../src/lib/settings-view.ts';

test('parsePort：只接受 1–65535 的十进制整数', () => {
  const cases = [
    ['7890', 7890],
    [' 7890 ', 7890],
    ['1', 1],
    ['65535', 65535],
    ['0', null],
    ['65536', null],
    ['123456', null],
    ['', null],
    ['   ', null],
    ['78.9', null],
    ['-1', null],
    ['1e3', null],
    ['abc', null],
    ['７８９０', null],
  ];
  for (const [text, expected] of cases) assert.equal(parsePort(text), expected, JSON.stringify(text));
});

test('optionsWith：当前值不在选项里时补一条，不改动原表', () => {
  const before = DNS_STRATEGY_OPTIONS.length;

  const known = optionsWith(DNS_STRATEGY_OPTIONS, 'prefer_ipv4');
  assert.deepEqual(known, DNS_STRATEGY_OPTIONS);
  assert.notEqual(known, DNS_STRATEGY_OPTIONS);

  assert.deepEqual(optionsWith(DNS_STRATEGY_OPTIONS, ''), DNS_STRATEGY_OPTIONS);

  const extended = optionsWith(DNS_STRATEGY_OPTIONS, 'custom');
  assert.equal(extended.length, before + 1);
  assert.deepEqual(extended.at(-1), { value: 'custom', label: 'custom' });
  assert.equal(DNS_STRATEGY_OPTIONS.length, before);
});

test('optionLabel：找不到时显示取值本身', () => {
  assert.equal(optionLabel(LOG_LEVEL_OPTIONS, 'info'), 'info · 常规');
  assert.equal(optionLabel(LOG_LEVEL_OPTIONS, 'verbose'), 'verbose');
});

test('cloneSettings：深拷贝，嵌套对象与数组互不影响', () => {
  const original = { ...createDefaultSettings(), tun: { ...createDefaultSettings().tun, excludes: ['10.0.0.0/8'] } };
  const copy = cloneSettings(original);
  assert.deepEqual(copy, original);
  assert.notEqual(copy, original);
  assert.notEqual(copy.tun, original.tun);
  assert.notEqual(copy.tun.excludes, original.tun.excludes);
  assert.notEqual(copy.dns, original.dns);
});

test('createDefaultSettings：每次返回新的对象，端口合法', () => {
  const first = createDefaultSettings();
  const second = createDefaultSettings();
  assert.deepEqual(first, second);
  assert.notEqual(first.tun.excludes, second.tun.excludes);
  assert.equal(parsePort(String(first.mixedPort)), first.mixedPort);
});

test('sameSettings：内容相同即视为没有修改', () => {
  const base = createDefaultSettings();
  assert.equal(sameSettings(base, base), true);
  assert.equal(sameSettings(base, cloneSettings(base)), true);

  const reordered = Object.fromEntries(Object.entries(base).reverse());
  assert.equal(sameSettings(base, reordered), true);
});

test('sameSettings：忽略主题，其余字段任何一处不同都算修改', () => {
  const base = createDefaultSettings();
  assert.equal(sameSettings(base, { ...base, theme: 'dark' }), true);
  assert.equal(sameSettings(base, { ...base, mixedPort: base.mixedPort + 1 }), false);
  assert.equal(sameSettings(base, { ...base, allowLan: !base.allowLan }), false);
  assert.equal(sameSettings(base, { ...base, dns: { ...base.dns, strategy: 'ipv6_only' } }), false);
  assert.equal(sameSettings(base, { ...base, tun: { ...base.tun, excludes: ['10.0.0.0/8'] } }), false);
});

test('sameSettings：列表顺序不同算修改', () => {
  const base = createDefaultSettings();
  const forward = { ...base, tun: { ...base.tun, excludes: ['a', 'b'] } };
  const backward = { ...base, tun: { ...base.tun, excludes: ['b', 'a'] } };
  assert.equal(sameSettings(forward, backward), false);
});

test('动作结果：失败时带上可以直接展示的说明', () => {
  assert.deepEqual(SUCCESS, { ok: true });
  assert.deepEqual(failureMessage('正在保存，请稍候'), { ok: false, message: '正在保存，请稍候' });
  assert.deepEqual(failure(new Error('boom')), { ok: false, message: '与桌面后端通信失败：boom' });

  const rejected = Object.assign(new Error('内核运行中，无法修改设置'), { name: 'RuntimeError' });
  assert.deepEqual(failure(rejected), { ok: false, message: '内核运行中，无法修改设置' });
});

test('updateHoursOptions：常用间隔；当前值不在其中时补一条', async () => {
  const { updateHoursOptions } = await import('../src/lib/settings-view.ts');
  const preset = updateHoursOptions(24);
  assert.deepEqual(preset.map((option) => option.value), ['1', '3', '6', '12', '24', '48', '72', '168']);
  assert.equal(preset.find((option) => option.value === '24').label, '每 24 小时');
  assert.equal(preset.find((option) => option.value === '168').label, '每 7 天');

  const custom = updateHoursOptions(5);
  assert.equal(custom.length, preset.length + 1);
  assert.deepEqual(custom.at(-1), { value: '5', label: '每 5 小时' });
  // 无效值不补进选项。
  assert.equal(updateHoursOptions(0).length, preset.length);
});

test('parseUpdateHours：只接受后端允许的 1–720 小时', async () => {
  const { parseUpdateHours } = await import('../src/lib/settings-view.ts');
  assert.equal(parseUpdateHours('24'), 24);
  assert.equal(parseUpdateHours('720'), 720);
  assert.equal(parseUpdateHours('0'), null);
  assert.equal(parseUpdateHours('721'), null);
  assert.equal(parseUpdateHours('abc'), null);
});

test('userAgentError：按 UTF-8 字节数限制 256', async () => {
  const { userAgentError } = await import('../src/lib/settings-view.ts');
  assert.equal(userAgentError('clash.meta'), '');
  assert.equal(userAgentError(''), '');
  assert.equal(userAgentError('a'.repeat(256)), '');
  assert.equal(userAgentError('a'.repeat(257)), 'User-Agent 不能超过 256 字节');
  assert.equal(userAgentError('渡'.repeat(86)), 'User-Agent 不能超过 256 字节');
});

test('withAutomationOff：未开放的自动化开关一律以 false 提交，不改动原对象', async () => {
  const { withAutomationOff } = await import('../src/lib/settings-view.ts');
  const source = { ...createDefaultSettings(), autoStart: true, startMinimized: true, lightweight: true, sysProxyOnStart: true, ruleSetAutoUpdate: true, allowLan: true };
  const result = withAutomationOff(source);
  assert.equal(result.autoStart, false);
  assert.equal(result.startMinimized, false);
  assert.equal(result.lightweight, false);
  assert.equal(result.sysProxyOnStart, false);
  assert.equal(result.ruleSetAutoUpdate, false);
  assert.equal(result.allowLan, true);
  assert.equal(source.autoStart, true);
});

test('checkCidr 与 normalizeCidrList：校验 IPv4/IPv6 CIDR、去重并限制上限', async () => {
  const { DEFAULT_TUN_EXCLUDES, checkCidr, normalizeCidrList } = await import('../src/lib/settings-view.ts');
  assert.ok(DEFAULT_TUN_EXCLUDES.length >= 6);
  for (const cidr of DEFAULT_TUN_EXCLUDES) {
    assert.equal(checkCidr(cidr), '', `默认网段应合法: ${cidr}`);
  }
  assert.equal(checkCidr('192.168.0.0/16'), '');
  assert.equal(checkCidr('0.0.0.0/0'), '');
  assert.equal(checkCidr('2001:db8::/32'), '');
  assert.equal(checkCidr('::1/128'), '');
  for (const bad of ['', '192.168.1.1', '192.168.1.0/33', '999.0.0.0/8', 'fe80::1%eth0/64', 'fc00::/129', 'example.com/24']) {
    assert.notEqual(checkCidr(bad), '', `非法网段应报错: ${bad}`);
  }
  const normalized = normalizeCidrList([' 10.0.0.0/8 ', '', '192.168.0.0/16', '10.0.0.0/8']);
  assert.deepEqual(normalized, { cidrs: ['10.0.0.0/8', '192.168.0.0/16'], error: '' });
  const tooMany = normalizeCidrList(Array.from({ length: 257 }, (_, i) => `10.${Math.floor(i / 256)}.${i % 256}.0/24`));
  assert.notEqual(tooMany.error, '');
});

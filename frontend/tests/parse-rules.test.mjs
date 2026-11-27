import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  parseAppGroup,
  parseAppGroups,
  parseImportPreview,
  parseRule,
  parseRuleSets,
  parseRuleTestResult,
  parseRules,
} from '../src/lib/api/parse-rules.ts';

/** 后端可能给出的「不是对象」的取值：Go 的 nil、误传的数组或字符串。 */
const GARBAGE = [null, undefined, 'text', 42, [], [{ id: 'x' }]];

const FULL_RULE = {
  id: 'r-1a2b',
  name: '浏览器走代理',
  enabled: true,
  apps: ['chrome.exe', '@group:ag-1'],
  hosts: ['*.example.com', 'geosite:cn'],
  ports: ['443', '8000-9000'],
  network: 'tcp',
  action: 'proxy',
  targetId: 'proxy',
  resolveBefore: false,
};

test('parseRule：完整负载逐字段保留', () => {
  assert.deepEqual(parseRule(FULL_RULE), FULL_RULE);
  assert.deepEqual(parseRule({ ...FULL_RULE, pinned: true }), { ...FULL_RULE, pinned: true });
});

test('parseRule：字段缺失时使用安全默认值，targetId 与 pinned 只在有意义时保留', () => {
  const fallback = {
    id: '',
    name: '',
    enabled: false,
    apps: [],
    hosts: [],
    ports: [],
    network: 'any',
    action: '',
    resolveBefore: true,
  };
  for (const value of GARBAGE) assert.deepEqual(parseRule(value), fallback, String(value));
  const rule = parseRule({ id: 'r-2', targetId: '', pinned: false, apps: null, hosts: ['a', 3], ports: 'x' });
  assert.equal('targetId' in rule, false);
  assert.equal('pinned' in rule, false);
  assert.deepEqual(rule.apps, []);
  assert.deepEqual(rule.hosts, ['a']);
  assert.deepEqual(rule.ports, []);
});

test('parseRule：只取约定的字段', () => {
  const rule = parseRule({ ...FULL_RULE, password: 'secret', extra: { token: 'secret' } });
  assert.equal(JSON.stringify(rule).includes('secret'), false);
});

test('parseRules：不是数组时为空，丢弃没有 id 的条目', () => {
  for (const value of [null, undefined, 'x', 3, {}]) assert.deepEqual(parseRules(value), []);
  const list = parseRules([{ id: 'builtin-lan', pinned: true }, { name: '没有 id' }, null, { id: 'builtin-default' }]);
  assert.deepEqual(
    list.map((item) => item.id),
    ['builtin-lan', 'builtin-default'],
  );
});

test('parseAppGroup / parseAppGroups：成员只保留字符串，丢弃没有 id 的分组', () => {
  assert.deepEqual(parseAppGroup({ id: 'ag-1', name: '浏览器', apps: ['chrome.exe', 5, 'msedge.exe'] }), {
    id: 'ag-1',
    name: '浏览器',
    apps: ['chrome.exe', 'msedge.exe'],
  });
  for (const value of GARBAGE) assert.deepEqual(parseAppGroup(value), { id: '', name: '', apps: [] }, String(value));
  for (const value of [null, undefined, 'x', 3, {}]) assert.deepEqual(parseAppGroups(value), []);
  assert.deepEqual(
    parseAppGroups([{ id: 'ag-1' }, { name: '没有 id' }, 'x']).map((item) => item.id),
    ['ag-1'],
  );
});

test('parseImportPreview：预览里的规则即使没有 id 也保留，警告只保留字符串', () => {
  const preview = parseImportPreview({
    rules: [{ id: 'import-clash-1-ab', name: '甲' }, { name: '没有 id' }, null, 'x'],
    warnings: ['第 3 条规则已跳过：不支持的类型', 9],
  });
  assert.deepEqual(
    preview.rules.map((rule) => rule.name),
    ['甲', '没有 id'],
  );
  assert.deepEqual(preview.warnings, ['第 3 条规则已跳过：不支持的类型']);
  for (const value of GARBAGE) assert.deepEqual(parseImportPreview(value), { rules: [], warnings: [] }, String(value));
});

test('parseRuleTestResult：阶段缺失时为空串，不会被当成命中', () => {
  const empty = { matchedRuleId: '', matchedRuleName: '', action: '', outboundTag: '', chain: [], stage: '' };
  for (const value of GARBAGE) assert.deepEqual(parseRuleTestResult(value), empty, String(value));
  const hit = {
    matchedRuleId: 'r-1',
    matchedRuleName: '浏览器走代理',
    action: 'proxy',
    outboundTag: 'proxy',
    chain: ['proxy'],
    stage: 'matched',
  };
  assert.deepEqual(parseRuleTestResult(hit), hit);
  assert.deepEqual(parseRuleTestResult({ matchedRuleId: 'r-2', stage: 'dns-required', chain: null }).chain, []);
});

test('parseRuleSets：状态缺失时视为未下载，丢弃没有 id 的条目', () => {
  const list = parseRuleSets([
    { id: 'geosite-cn', name: 'geosite:cn', url: 'https://example.invalid/a.srs', updatedAt: 5, sizeBytes: 9, status: 'ready' },
    { id: 'geoip-cn' },
    { name: '没有 id' },
  ]);
  assert.deepEqual(list, [
    { id: 'geosite-cn', name: 'geosite:cn', url: 'https://example.invalid/a.srs', updatedAt: 5, sizeBytes: 9, status: 'ready' },
    { id: 'geoip-cn', name: '', url: '', updatedAt: 0, sizeBytes: 0, status: 'missing' },
  ]);
  for (const value of [null, undefined, 'x', 3, {}]) assert.deepEqual(parseRuleSets(value), []);
});

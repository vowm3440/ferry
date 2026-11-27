import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  ACTION_LABELS,
  appText,
  blankRuleForm,
  canMove,
  describeTestResult,
  formFromRule,
  hasProblems,
  importOutcomeText,
  moveRule,
  parseLines,
  parseTokens,
  ruleFromForm,
  ruleNumber,
  ruleSetStatus,
  summarizeItems,
  validateAppGroupForm,
  validateRuleForm,
  validateTestInput,
} from '../src/lib/rules-view.ts';

const rule = (id, extra = {}) => ({ id, name: id, enabled: true, apps: [], hosts: [], ports: [], network: 'any', action: 'direct', resolveBefore: false, ...extra });
const LIST = [rule('builtin-lan', { pinned: true }), rule('a'), rule('b'), rule('c'), rule('builtin-default', { pinned: true, action: 'proxy', targetId: 'proxy' })];
const GROUPS = [{ id: 'ag-1', name: '浏览器', apps: ['chrome.exe'] }];

test('parseLines / parseTokens：去空白、去空行、去重并保持顺序', () => {
  assert.deepEqual(parseLines(' a.exe \r\n\n C:\\Program Files\\x, y.exe\na.exe\n'), ['a.exe', 'C:\\Program Files\\x, y.exe']);
  assert.deepEqual(parseTokens('a.com, b.com\n geoip:cn  a.com；c.com，d.com;e.com'), ['a.com', 'b.com', 'geoip:cn', 'c.com', 'd.com', 'e.com']);
  assert.deepEqual(parseTokens('  \n '), []);
});

test('summarizeItems：最多显示两项，其余折成 +N；规则集单独标记', () => {
  assert.deepEqual(summarizeItems([]), { shown: [], more: 0 });
  assert.deepEqual(summarizeItems(['a', 'geoip:cn', 'GeoSite:x', 'd']), {
    shown: [
      { text: 'a', set: false },
      { text: 'geoip:cn', set: true },
    ],
    more: 2,
  });
  assert.equal(summarizeItems(['a', 'b', 'c'], 3).more, 0);
});

test('appText：分组引用显示分组名，完整路径只显示文件名', () => {
  assert.equal(appText('@group:ag-1', GROUPS), '分组：浏览器');
  assert.equal(appText('@group:gone', GROUPS), '分组：gone（不存在）');
  assert.equal(appText('C:\\Program Files\\App\\a.exe', GROUPS), 'a.exe');
  assert.equal(appText('d:/x/y.exe', GROUPS), 'y.exe');
  assert.equal(appText('chrome.exe', GROUPS), 'chrome.exe');
});

test('formFromRule / ruleFromForm：往返不丢字段，非代理动作清空目标', () => {
  const source = rule('r-1', { name: ' 办公 ', apps: ['a.exe'], hosts: ['x.com', 'geoip:cn'], ports: ['80', '443'], network: 'tcp', action: 'proxy', targetId: 'g1', resolveBefore: true });
  const form = formFromRule(source);
  assert.equal(form.hostsText, 'x.com\ngeoip:cn');
  assert.equal(form.portsText, '80, 443');
  assert.deepEqual(ruleFromForm(form), { ...source, name: '办公' });
  assert.equal(ruleFromForm({ ...form, action: 'block' }).targetId, undefined);
  assert.equal(ruleFromForm({ ...form, targetId: '' }).targetId, 'proxy');
  assert.equal(source.apps.length, 1);
});

test('blankRuleForm：新建默认启用、代理到默认分组', () => {
  const draft = ruleFromForm({ ...blankRuleForm(), name: 'x' });
  assert.deepEqual(draft, { id: '', name: 'x', enabled: true, apps: [], hosts: [], ports: [], network: 'any', action: 'proxy', targetId: 'proxy', resolveBefore: true });
});

test('validateRuleForm：逐字段给出与后端一致的说明', () => {
  const ok = { ...blankRuleForm(), name: '规则', appsText: 'a.exe\n@group:ag-1', hostsText: 'x.com geoip:cn', portsText: '80,1000-2000' };
  assert.equal(hasProblems(validateRuleForm(ok, GROUPS, ['proxy'])), false);
  assert.deepEqual(validateRuleForm({ ...ok, name: '  ' }, GROUPS, ['proxy']), { name: '请填写规则名称', apps: '', hosts: '', ports: '', target: '' });
  assert.equal(validateRuleForm({ ...ok, name: 'x'.repeat(257) }, GROUPS, ['proxy']).name, '名称过长或含有控制字符');
  assert.equal(validateRuleForm({ ...ok, appsText: 'a|b' }, GROUPS, ['proxy']).apps, '第 1 项「a|b」：规则应用条件无效');
  assert.equal(validateRuleForm({ ...ok, appsText: 'a.exe\n@group:x' }, GROUPS, ['proxy']).apps, '第 2 项「@group:x」：引用的应用分组不存在');
  assert.equal(validateRuleForm({ ...ok, hostsText: 'geosite:google' }, GROUPS, ['proxy']).hosts, '第 1 项「geosite:google」：规则集不在支持的目录内，请选择已列出的规则集');
  assert.equal(validateRuleForm({ ...ok, portsText: '80 0' }, GROUPS, ['proxy']).ports, '第 2 项「0」：规则端口必须为 1 到 65535 或递增的端口范围');
  assert.equal(validateRuleForm({ ...ok, targetId: 'gone' }, GROUPS, ['proxy', 'g1']).target, '规则目标不存在或不适用于此动作');
  assert.equal(validateRuleForm({ ...ok, targetId: 'gone', action: 'direct' }, GROUPS, ['proxy']).target, '');
  const many = Array.from({ length: 257 }, (_, index) => String(index + 1)).join(',');
  assert.equal(validateRuleForm({ ...ok, portsText: many }, GROUPS, ['proxy']).ports, '规则条件数量超过限制（每项最多 256 个）');
});

test('validateAppGroupForm：成员不能引用其它分组', () => {
  assert.deepEqual(validateAppGroupForm('浏览器', 'chrome.exe\nmsedge.exe'), { name: '', apps: '' });
  assert.equal(validateAppGroupForm('', 'a.exe').name, '请填写分组名称');
  assert.equal(validateAppGroupForm('x', '').apps, '请至少填写一个应用');
  assert.equal(validateAppGroupForm('x', '@group:ag-1').apps, '第 1 项「@group:ag-1」：应用分组成员无效，不能引用其它分组');
  assert.equal(validateAppGroupForm('x', 'a<b').apps, '第 1 项「a<b」：应用分组成员无效，不能引用其它分组');
});

test('canMove / moveRule：固定规则不动，普通规则只在首尾之间移动', () => {
  assert.equal(canMove(LIST, 'builtin-lan', 1), false);
  assert.equal(canMove(LIST, 'builtin-default', -1), false);
  assert.equal(canMove(LIST, 'a', -1), false);
  assert.equal(canMove(LIST, 'c', 1), false);
  assert.equal(canMove(LIST, 'nope', 1), false);
  assert.equal(canMove(LIST, 'a', 1), true);
  assert.deepEqual(moveRule(LIST, 'a', 1), ['builtin-lan', 'b', 'a', 'c', 'builtin-default']);
  assert.deepEqual(moveRule(LIST, 'c', -1), ['builtin-lan', 'a', 'c', 'b', 'builtin-default']);
  assert.equal(moveRule(LIST, 'a', -1), null);
  assert.deepEqual(LIST.map((item) => item.id), ['builtin-lan', 'a', 'b', 'c', 'builtin-default']);
  assert.equal(ruleNumber(LIST, 'b'), 3);
  assert.equal(ruleNumber(LIST, 'zz'), 0);
});

test('validateTestInput：主机、端口必填且合法', () => {
  assert.equal(validateTestInput('chrome.exe', 'example.com', '443'), '');
  assert.equal(validateTestInput('', '1.2.3.4', '80'), '');
  assert.equal(validateTestInput('', '', '80'), '请填写目标主机');
  assert.equal(validateTestInput('', '*.example.com', '80'), '目标主机须为域名或 IP 地址');
  assert.equal(validateTestInput('', 'example.com', ''), '端口须为 1 到 65535 的整数');
  assert.equal(validateTestInput('', 'example.com', '70000'), '端口须为 1 到 65535 的整数');
  assert.equal(validateTestInput('', 'example.com', '80-90'), '端口须为 1 到 65535 的整数');
});

test('describeTestResult：dns-required 不算命中', () => {
  const names = new Map([['g1', '香港']]);
  const hit = describeTestResult({ matchedRuleId: 'b', matchedRuleName: '办公', action: 'proxy', outboundTag: 'g1', chain: ['g1'], stage: 'matched' }, LIST, names);
  assert.deepEqual(hit, { kind: 'hit', number: 3, name: '办公', action: 'proxy', outbound: '香港' });
  const direct = describeTestResult({ matchedRuleId: 'a', matchedRuleName: 'a', action: 'direct', outboundTag: '__direct', chain: [], stage: 'matched' }, LIST, names);
  assert.equal(direct.outbound, '');
  const unknown = describeTestResult({ matchedRuleId: 'a', matchedRuleName: 'a', action: 'proxy', outboundTag: 'zz', chain: [], stage: 'matched' }, LIST, names);
  assert.equal(unknown.outbound, 'zz');
  const dns = describeTestResult({ matchedRuleId: 'c', matchedRuleName: '按 IP', action: '', outboundTag: '', chain: [], stage: 'dns-required' }, LIST, names);
  assert.deepEqual(dns, { kind: 'uncertain', number: 4, name: '按 IP', action: '', outbound: '' });
  const odd = describeTestResult({ matchedRuleId: 'c', matchedRuleName: 'x', action: 'proxy', outboundTag: 'g1', chain: [], stage: '' }, LIST, names);
  assert.equal(odd.kind, 'uncertain');
});

test('ruleSetStatus / ACTION_LABELS / importOutcomeText', () => {
  assert.deepEqual(ruleSetStatus('ready'), { label: '已就绪', tone: 'ok' });
  assert.deepEqual(ruleSetStatus('missing'), { label: '未下载', tone: 'none' });
  assert.deepEqual(ruleSetStatus('updating'), { label: '更新中', tone: 'warn' });
  assert.deepEqual(ruleSetStatus('error'), { label: '更新失败', tone: 'err' });
  assert.deepEqual(ruleSetStatus('invalid'), { label: '文件无效', tone: 'err' });
  assert.deepEqual(ruleSetStatus('??'), { label: '??', tone: 'none' });
  assert.equal(ACTION_LABELS.block, '拦截');
  assert.equal(importOutcomeText(3, ''), '已导入 3 条规则');
  assert.equal(importOutcomeText(2, '规则数量超过限制'), '没有导入任何规则：规则数量超过限制');
});

test('validateRuleForm：列表没读到（null）时不判断「不存在」，格式照查', () => {
  const base = { id: '', name: '规则', enabled: true, appsText: '', hostsText: '', portsText: '', network: 'any', action: 'proxy', targetId: 'proxy', resolveBefore: false };
  assert.equal(validateRuleForm({ ...base, appsText: '@group:x' }, null, ['proxy']).apps, '');
  assert.equal(validateRuleForm({ ...base, appsText: '@grp:x' }, null, ['proxy']).apps, '第 1 项「@grp:x」：应用分组引用格式无效');
  assert.equal(validateRuleForm({ ...base, targetId: 'n-1' }, GROUPS, null).target, '');
  assert.equal(validateRuleForm({ ...base, targetId: 'n-1' }, GROUPS, []).target, '规则目标不存在或不适用于此动作');
});

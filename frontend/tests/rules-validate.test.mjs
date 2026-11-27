import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  appProblem,
  hostProblem,
  isRuleSetRef,
  isValidId,
  isValidLabel,
  isValidRuleApp,
  isValidRuleHost,
  isValidRulePort,
  portProblem,
} from '../src/lib/rules-validate.ts';

const all = (check, values, expected) => {
  for (const value of values) assert.equal(check(value), expected, JSON.stringify(value));
};

test('isValidLabel：非空白、不超过 256 字节、无控制字符', () => {
  all(isValidLabel, ['a', '办公 应用', ' 前后空格也行 ', 'x'.repeat(256)], true);
  all(isValidLabel, ['', '   ', 'x'.repeat(257), '渡'.repeat(86), 'a\tb', 'a\nb', 'a\u0000'], false);
  assert.equal(isValidLabel('渡'.repeat(85)), true);
});

test('isValidId：字母数字开头，后续可含 _ -，最长 64', () => {
  all(isValidId, ['a', 'r-0af3', 'ag_1', 'A'.repeat(64)], true);
  all(isValidId, ['', '-a', '_a', 'a b', 'a.b', '组', 'A'.repeat(65)], false);
});

test('isValidRuleApp：进程名或带盘符的完整路径', () => {
  all(isValidRuleApp, ['chrome.exe', 'C:\\Program Files\\App\\a.exe', 'd:/tools/x.exe', '*.exe', '微信.exe'], true);
  all(
    isValidRuleApp,
    ['', ' a.exe', 'a.exe ', 'a\tb.exe', 'a"b.exe', 'a<b', 'a>b', 'a|b', 'C:', 'C:a.exe', '1:\\a.exe', 'C:\\a:b.exe', 'x'.repeat(1025)],
    false,
  );
  assert.equal(isValidRuleApp('x'.repeat(1024)), true);
});

test('isValidRuleHost：IP、CIDR、同族递增的 IP 范围', () => {
  all(
    isValidRuleHost,
    ['1.2.3.4', '::1', 'fe80::1', '2001:db8::', '10.0.0.0/8', '0.0.0.0/0', 'fc00::/7', '::/128', '1.1.1.1-1.1.1.9', '1.1.1.1-1.1.1.1', '::1-::ffff', '64:ff9b::1.2.3.4'],
    true,
  );
  all(
    isValidRuleHost,
    [
      '1.2.3.4/33',
      'fc00::/129',
      '1.2.3.4/08',
      '1.2.3.4/',
      '1.2.3/8',
      '::ffff:1.2.3.4',
      '::ffff:1.2.3.4/120',
      'fe80::1%eth0',
      '1.1.1.9-1.1.1.1',
      '1.1.1.1-::1',
      '1.1.1.1-example.com',
      '1.1.1-2.2.2',
      '1.2.3.4-',
      '01.2.3.4',
      '1.2.3.256',
      '1.2.3',
      '1:2:3',
    ],
    false,
  );
});

test('isValidRuleHost：域名标签允许 * ? _ -，不允许首尾连字符与纯数字', () => {
  all(isValidRuleHost, ['example.com', 'example.com.', '*.example.com', 'a?c.io', '_dmarc.x.org', 'localhost', 'a-b.c', 'my-host.lan', 'x'.repeat(63) + '.com'], true);
  all(
    isValidRuleHost,
    ['', ' example.com', 'example .com', '-a.com', 'a-.com', 'a..com', '.com', '例子.com', 'a/b', 'http://a.com', 'a.com:80', '123', '1.2', 'x'.repeat(64) + '.com', 'a.'.repeat(127) + 'a'],
    false,
  );
});

test('isRuleSetRef：只认目录内的四个规则集', () => {
  all(isRuleSetRef, ['geosite:cn', 'geosite:geolocation-!cn', 'geoip:cn', 'geoip:private'], true);
  all(isRuleSetRef, ['geosite:google', 'GEOIP:CN', 'geoip:us', 'cn'], false);
});

test('hostProblem：规则集写法不在目录内时单独说明', () => {
  assert.equal(hostProblem('geoip:cn'), '');
  assert.equal(hostProblem('example.com'), '');
  assert.equal(hostProblem('geosite:google'), '规则集不在支持的目录内，请选择已列出的规则集');
  assert.equal(hostProblem('GeoIP:cn'), '规则集不在支持的目录内，请选择已列出的规则集');
  assert.equal(hostProblem('a b'), '规则主机条件无效或语法不受支持');
});

test('isValidRulePort：单个端口或递增范围', () => {
  all(isValidRulePort, ['1', '80', '65535', '080', '1000-2000', '80-80'], true);
  all(isValidRulePort, ['', '0', '65536', '-1', '80-', '-80', '2000-1000', '1-2-3', '8o', '+80', ' 80', '1_0'], false);
  assert.equal(portProblem('443'), '');
  assert.equal(portProblem('0'), '规则端口必须为 1 到 65535 或递增的端口范围');
});

test('appProblem：@group: 引用必须指向存在且非空的分组', () => {
  const groups = [
    { id: 'ag-1', name: '浏览器', apps: ['chrome.exe'] },
    { id: 'ag-2', name: '空分组', apps: [] },
  ];
  assert.equal(appProblem('chrome.exe', groups), '');
  assert.equal(appProblem('@group:ag-1', groups), '');
  assert.equal(appProblem('@group:ag-2', groups), '引用的应用分组为空或无效');
  assert.equal(appProblem('@group:nope', groups), '引用的应用分组不存在');
  assert.equal(appProblem('@group:', groups), '应用分组引用格式无效');
  assert.equal(appProblem('@other', groups), '应用分组引用格式无效');
  assert.equal(appProblem('a|b', groups), '规则应用条件无效');
});

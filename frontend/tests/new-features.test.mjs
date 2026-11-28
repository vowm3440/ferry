import test from 'node:test';
import assert from 'node:assert/strict';

import {
  dnsDomainProblem,
  hostProblem,
  isRuleSetRef,
  isValidDnsServerUrl,
  isValidRuleSetUrl,
  ruleSetRefFromInfo,
} from '../src/lib/rules-validate.ts';
import { suggestedActionFor, wildcardDomainOf } from '../src/lib/conns-view.ts';
import { parseSettings } from '../src/lib/api/parse.ts';
import { parseApplyPresetResult, parseCustomRuleSets, parseRuleSets } from '../src/lib/api/parse-rules.ts';
import { parseConnections } from '../src/lib/api/parse-inspect.ts';

test('custom rule-set reference & URL validation', () => {
  const custom = new Set(['openai', 'steam-cn']);
  assert.equal(isRuleSetRef('ruleset:openai', custom), true);
  assert.equal(isRuleSetRef('ruleset:missing', custom), false);
  assert.equal(hostProblem('ruleset:openai', custom), '');
  assert.notEqual(hostProblem('ruleset:missing', custom), '');
  assert.notEqual(hostProblem('ruleset://arbitrary', custom), '');

  assert.equal(isValidRuleSetUrl('https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/openai.srs'), true);
  assert.equal(isValidRuleSetUrl('http://example.com/openai.srs'), false);
  assert.equal(isValidRuleSetUrl('https://example.com/openai.json'), false);
  assert.equal(isValidRuleSetUrl('https://user:pass@example.com/openai.srs'), false);

  assert.equal(ruleSetRefFromInfo({ id: 'geosite-cn', name: 'CN', url: '', updatedAt: 0, sizeBytes: 0, status: 'ready' }), 'geosite:cn');
  assert.equal(ruleSetRefFromInfo({ id: 'openai', name: 'OpenAI', url: '', updatedAt: 0, sizeBytes: 0, status: 'ready', custom: true }), 'ruleset:openai');
});

test('split DNS server & domain rule validation', () => {
  assert.equal(isValidDnsServerUrl('udp://10.0.0.53'), true);
  assert.equal(isValidDnsServerUrl('https://223.5.5.5/dns-query'), true);
  assert.equal(isValidDnsServerUrl('tls://1.1.1.1:853'), true);
  assert.equal(isValidDnsServerUrl('https://dns.example.com/dns-query'), false);
  assert.equal(isValidDnsServerUrl('udp://10.0.0.53/extra'), false);

  const custom = new Set(['apple-cn']);
  assert.equal(dnsDomainProblem('*.corp', custom), '');
  assert.equal(dnsDomainProblem('geosite:cn', custom), '');
  assert.equal(dnsDomainProblem('ruleset:apple-cn', custom), '');
  assert.notEqual(dnsDomainProblem('geoip:cn', custom), '');
  assert.notEqual(dnsDomainProblem('ruleset:unknown', custom), '');
});

test('connection wildcard domain & opposite action suggestion', () => {
  assert.equal(wildcardDomainOf('api2.cursor.sh'), '*.cursor.sh');
  assert.equal(wildcardDomainOf('sub.store.example.com.cn'), '*.example.com.cn');
  assert.equal(wildcardDomainOf('example.com'), '*.example.com');
  assert.equal(wildcardDomainOf('192.168.1.1'), '');
  assert.equal(wildcardDomainOf('::1'), '');

  assert.equal(suggestedActionFor({ id: '1', process: '', processPath: '', host: 'a.com', port: 443, ruleName: '', action: 'direct', outboundTag: '', chain: [], up: 0, down: 0, startAt: 0 }), 'proxy');
  assert.equal(suggestedActionFor({ id: '2', process: '', processPath: '', host: 'a.com', port: 443, ruleName: '', action: 'proxy', outboundTag: '', chain: [], up: 0, down: 0, startAt: 0 }), 'direct');
});

test('parsers handle split DNS, custom rule-sets, preset result, and attribution fields', () => {
  const settings = parseSettings({
    mixedPort: 7890,
    ruleSetAutoUpdate: true,
    ruleSetUpdateHours: 12,
    dns: {
      remote: 'https://1.1.1.1/dns-query',
      direct: 'https://223.5.5.5/dns-query',
      strategy: 'prefer_ipv4',
      servers: [{ id: 'corp', name: 'Corp', address: 'udp://10.0.0.53', detour: 'direct' }],
      rules: [{ id: 'r1', name: 'Intranet', enabled: true, domains: ['*.corp'], server: 'corp' }],
    },
  });
  assert.equal(settings.ruleSetUpdateHours, 12);
  assert.equal(settings.dns.servers.length, 1);
  assert.equal(settings.dns.rules.length, 1);

  const sets = parseRuleSets([{ id: 'openai', name: 'OpenAI', url: 'https://a.com/o.srs', status: 'ready', custom: true, autoUpdate: true, updateHours: 6 }]);
  assert.equal(sets[0].custom, true);
  assert.equal(sets[0].updateHours, 6);

  const presets = parseCustomRuleSets([{ id: 'ads', name: 'Ads', url: 'https://a.com/ads.srs', autoUpdate: true, updateHours: 24 }]);
  assert.equal(presets[0].id, 'ads');

  const res = parseApplyPresetResult({ ruleCount: 11, ruleSetCount: 5, appGroupCount: 3, dnsRuleCount: 2, missingRuleSets: ['openai'] });
  assert.deepEqual(res.missingRuleSets, ['openai']);

  const conns = parseConnections([{ id: 'c1', host: 'api.openai.com', port: 443, network: 'tcp', ruleId: 'r-ai', ruleName: 'AI 服务', ruleDetail: 'rule_set=[openai] => route(proxy)', action: 'proxy' }]);
  assert.equal(conns[0].ruleId, 'r-ai');
  assert.equal(conns[0].ruleDetail, 'rule_set=[openai] => route(proxy)');
  assert.equal(conns[0].network, 'tcp');
});

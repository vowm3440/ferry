/*
 * 演示后端：规则、应用分组与规则集。
 *
 * 规则与应用分组保存在内存里，校验与错误文案照抄 Go 侧（internal/app/rules.go、appgroups.go、
 * internal/spec/validation_rules.go），改动后同样会「重建内核」并推送 rules:applied。
 * 规则导入的解析、规则测试与规则集下载在浏览器里没有对应实现，一律如实报「演示模式不提供此功能」；
 * 规则集列表只给出目录，状态恒为「未下载」。
 */

import { appProblem, hostProblem, isValidId, isValidLabel, isValidRuleApp, portProblem, MAX_APP_GROUPS, MAX_RULE_ITEMS, MAX_RULES } from '../rules-validate';
import type { RulesApi } from './backend';
import { rejected, unsupported, type DemoWorld } from './demo-world';
import { EVENT_RULES_APPLIED } from './types';
import { DEFAULT_TARGET_ID, RULE_ACTIONS, RULE_ID_DEFAULT, RULE_ID_LAN, RULE_NETWORKS, type AppGroup, type Rule, type RuleSetInfo } from './types-rules';

const LAN_HOSTS: readonly string[] = [
  '0.0.0.0/8',
  '10.0.0.0/8',
  '127.0.0.0/8',
  '169.254.0.0/16',
  '172.16.0.0/12',
  '192.168.0.0/16',
  '224.0.0.0/4',
  '255.255.255.255/32',
  '::/128',
  '::1/128',
  'fc00::/7',
  'fe80::/10',
  'ff00::/8',
];

const UPSTREAM = 'https://raw.githubusercontent.com/SagerNet/';

/** 与 internal/core/rulesets/catalog.go 相同的目录。演示模式不下载，所以都是「未下载」。 */
const RULE_SET_CATALOG: readonly RuleSetInfo[] = [
  { id: 'geosite-cn', name: 'China domains', url: `${UPSTREAM}sing-geosite/rule-set/geosite-cn.srs`, updatedAt: 0, sizeBytes: 0, status: 'missing' },
  { id: 'geosite-non-cn', name: 'Non-China domains', url: `${UPSTREAM}sing-geosite/rule-set/geosite-geolocation-!cn.srs`, updatedAt: 0, sizeBytes: 0, status: 'missing' },
  { id: 'geoip-cn', name: 'China IP addresses', url: `${UPSTREAM}sing-geoip/rule-set/geoip-cn.srs`, updatedAt: 0, sizeBytes: 0, status: 'missing' },
  { id: 'geoip-private', name: 'Private IP addresses', url: 'https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geoip/private.srs', updatedAt: 0, sizeBytes: 0, status: 'missing' },
];

/** 与 spec.DefaultRules() 一致的固定首尾两条规则。 */
function builtinRules(): Rule[] {
  const base = { enabled: true, apps: [], ports: [], network: 'any', resolveBefore: false, pinned: true };
  return [
    { ...base, id: RULE_ID_LAN, name: '本机 / 局域网', hosts: [...LAN_HOSTS], action: 'direct' },
    { ...base, id: RULE_ID_DEFAULT, name: '默认规则', hosts: [], action: 'proxy', targetId: DEFAULT_TARGET_ID },
  ];
}

function cloneRule(rule: Rule): Rule {
  return { ...rule, apps: [...rule.apps], hosts: [...rule.hosts], ports: [...rule.ports] };
}

function cloneGroup(group: AppGroup): AppGroup {
  return { ...group, apps: [...group.apps] };
}

function randomId(prefix: string): string {
  const bytes = crypto.getRandomValues(new Uint8Array(12));
  return prefix + [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

const isBuiltin = (id: string): boolean => id === RULE_ID_LAN || id === RULE_ID_DEFAULT;

/** 第一处不合法的条件对应的后端文案。 */
function firstProblem(items: readonly string[], check: (item: string) => string): string {
  for (const item of items) {
    const problem = check(item);
    if (problem !== '') return problem;
  }
  return '';
}

/** 与 spec.ValidateRule 相同的顺序与文案（展开后的应用条件上限除外）。 */
function ruleProblem(rule: Rule, groups: readonly AppGroup[], targets: ReadonlySet<string>): string {
  if (!isValidId(rule.id) || !isValidLabel(rule.name)) return '规则标识或名称无效';
  if (!(RULE_NETWORKS as readonly string[]).includes(rule.network) || !(RULE_ACTIONS as readonly string[]).includes(rule.action)) return '规则协议或动作无效';
  if (rule.pinned === true) return isBuiltin(rule.id) ? '固定规则不可修改' : '普通规则不可标记为固定规则';
  const target = rule.targetId ?? '';
  const targetOk = rule.action === 'proxy' ? target === '' || target === DEFAULT_TARGET_ID || targets.has(target) : target === '';
  if (!targetOk) return '规则目标不存在或不适用于此动作';
  if (rule.apps.length > MAX_RULE_ITEMS || rule.hosts.length > MAX_RULE_ITEMS || rule.ports.length > MAX_RULE_ITEMS) return '规则条件数量超过限制';
  return firstProblem(rule.apps, (app) => appProblem(app, groups)) || firstProblem(rule.hosts, hostProblem) || firstProblem(rule.ports, portProblem);
}

function groupProblem(group: AppGroup): string {
  if (!isValidId(group.id) || !isValidLabel(group.name)) return '应用分组标识或名称无效';
  if (group.apps.length > MAX_RULE_ITEMS) return '应用分组成员数量超过限制';
  return group.apps.some((app) => app.startsWith('@') || !isValidRuleApp(app)) ? '应用分组成员无效，不能引用其它分组' : '';
}

/** 与 reorderRules 相同：必须是当前全部规则的一个排列，固定规则留在首尾。 */
function reordered(current: readonly Rule[], ids: readonly string[]): Rule[] {
  if (ids.length !== current.length || ids.length < 2 || ids[0] !== RULE_ID_LAN || ids[ids.length - 1] !== RULE_ID_DEFAULT) {
    throw rejected('规则顺序无效，固定规则须位于首尾');
  }
  const byId = new Map(current.map((rule) => [rule.id, rule]));
  return ids.map((id) => {
    const rule = byId.get(id);
    if (rule === undefined) throw rejected('规则顺序含重复或未知标识');
    byId.delete(id);
    return rule;
  });
}

export function createDemoRules(world: DemoWorld): RulesApi {
  let rules: readonly Rule[] = builtinRules();
  let groups: readonly AppGroup[] = [];
  let version = 0;

  world.onReset(() => {
    rules = builtinRules();
    groups = [];
  });

  const targets = (): ReadonlySet<string> => {
    const list = world.nodeList();
    return new Set([...list.groups.map((group) => group.id), ...list.nodes.map((node) => node.id)]);
  };

  /** 校验整份规则与分组，通过后提交、重建内核并推送 rules:applied。 */
  async function commit(nextRules: readonly Rule[], nextGroups: readonly AppGroup[]): Promise<void> {
    if (nextGroups.length > MAX_APP_GROUPS) throw rejected('应用分组数量超过限制');
    for (const group of nextGroups) {
      const problem = groupProblem(group);
      if (problem !== '') throw rejected(problem);
    }
    if (nextRules.length > MAX_RULES) throw rejected('规则数量超过限制');
    const ids = targets();
    for (const rule of nextRules) {
      if (isBuiltin(rule.id)) continue;
      const problem = ruleProblem(rule, nextGroups, ids);
      if (problem !== '') throw rejected(problem);
    }
    rules = nextRules;
    groups = nextGroups;
    await world.reconfigure();
    version += 1;
    world.emit(EVENT_RULES_APPLIED, { version });
  }

  const findRule = (id: string, pinnedMessage: string): Rule => {
    if (isBuiltin(id)) throw rejected(pinnedMessage);
    const rule = rules.find((item) => item.id === id);
    if (rule === undefined) throw rejected('规则不存在');
    return rule;
  };

  return {
    async rulesList() {
      return rules.map(cloneRule);
    },

    async rulesSave(input) {
      if (input.pinned === true || isBuiltin(input.id)) throw rejected('固定规则不可编辑');
      const rule = cloneRule({ ...input, id: input.id === '' ? randomId('r-') : input.id });
      const exists = rules.some((item) => item.id === rule.id);
      const next = exists ? rules.map((item) => (item.id === rule.id ? rule : item)) : [...rules.slice(0, -1), rule, ...rules.slice(-1)];
      await commit(next, groups);
      return cloneRule(rule);
    },

    async rulesSaveMany(batch) {
      if (batch.length === 0) throw rejected('没有可导入的规则');
      if (batch.some((item) => item.pinned === true || isBuiltin(item.id))) throw rejected('固定规则不可编辑');
      const fresh = batch.map((item) => cloneRule({ ...item, id: randomId('r-') }));
      await commit([...rules.slice(0, -1), ...fresh, ...rules.slice(-1)], groups);
    },

    async rulesRemove(id) {
      const target = findRule(id, '固定规则不可删除');
      await commit(rules.filter((rule) => rule !== target), groups);
    },

    async rulesSetEnabled(id, on) {
      const target = findRule(id, '固定规则不可禁用');
      await commit(rules.map((rule) => (rule === target ? { ...rule, enabled: on } : rule)), groups);
    },

    async rulesReorder(ids) {
      await commit(reordered(rules, ids), groups);
    },

    async rulesApply(ids) {
      await commit(reordered(rules, ids), groups);
    },

    async rulesCreateFromConnection(connectionId) {
      const item = world.connections.find((connection) => connection.id === connectionId);
      if (item === undefined) throw rejected('连接已结束或不存在');
      const plain = item.action === 'direct' || item.action === 'block';
      return {
        id: '',
        name: '连接规则',
        enabled: true,
        apps: item.processPath === '' ? [] : [item.processPath],
        hosts: item.host === '' ? [] : [item.host],
        ports: item.port > 0 ? [String(item.port)] : [],
        network: 'any',
        action: plain ? item.action : 'proxy',
        ...(plain ? {} : { targetId: world.nodeList().groups[0]?.id ?? DEFAULT_TARGET_ID }),
        resolveBefore: true,
      };
    },

    async rulesImport() {
      throw unsupported();
    },

    async rulesTest() {
      throw unsupported();
    },

    async appGroupsList() {
      return groups.map(cloneGroup);
    },

    async appGroupsSave(input) {
      const group = cloneGroup({ ...input, id: input.id === '' ? randomId('ag-') : input.id });
      const exists = groups.some((item) => item.id === group.id);
      await commit(rules, exists ? groups.map((item) => (item.id === group.id ? group : item)) : [...groups, group]);
      return cloneGroup(group);
    },

    async appGroupsRemove(id) {
      if (!groups.some((group) => group.id === id)) throw rejected('应用分组不存在');
      if (rules.some((rule) => rule.apps.includes(`@group:${id}`))) throw rejected('规则仍在引用此应用分组，请先移除对应引用');
      await commit(rules, groups.filter((group) => group.id !== id));
    },

    async ruleSetsList() {
      return RULE_SET_CATALOG.map((item) => ({ ...item }));
    },

    async ruleSetsUpdate() {
      throw unsupported();
    },
  };
}

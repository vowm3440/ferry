/*
 * 规则页的纯逻辑：表单与规则互转、提交前校验、条件的摘要显示、排序、测试结果与规则集状态的文案。
 * 不依赖 Svelte，由 tests/rules-view.test.mjs 覆盖。单项条件的语法校验在 rules-validate.ts。
 */

import type { AppGroup, Rule, RuleTestResult } from './api/types-rules';
import { APP_GROUP_PREFIX, DEFAULT_TARGET_ID, RULE_ACTIONS, RULE_NETWORKS, TEST_STAGE_MATCHED } from './api/types-rules';
import { t } from './i18n/locale';
import { appProblem, hostProblem, isIpLiteral, isValidLabel, isValidRuleApp, looksLikeRuleSet, MAX_RULE_ITEMS, portProblem } from './rules-validate';

export const ACTION_LABELS: Readonly<Record<string, string>> = { proxy: '代理', direct: '直连', block: '拦截' };
export const NETWORK_LABELS: Readonly<Record<string, string>> = { any: '任意', tcp: 'TCP', udp: 'UDP' };
export const IMPORT_KIND_LABELS: Readonly<Record<string, string>> = { clash: 'Clash', singbox: 'sing-box', ppx: 'Proxifier (PPX)' };

export function actionLabel(action: string): string {
  if (action === 'proxy') return t('代理', 'Proxy');
  if (action === 'direct') return t('直连', 'Direct');
  if (action === 'block') return t('拦截', 'Block');
  return action;
}

export function networkLabel(net: string): string {
  if (net === 'any') return t('任意', 'Any');
  return NETWORK_LABELS[net] ?? net;
}

export function actionOptions(): ReadonlyArray<{ readonly value: string; readonly label: string }> {
  return RULE_ACTIONS.map((value) => ({ value: value as string, label: actionLabel(value) }));
}

export function networkOptions(): ReadonlyArray<{ readonly value: string; readonly label: string }> {
  return RULE_NETWORKS.map((value) => ({ value: value as string, label: networkLabel(value) }));
}

/** 每行一项（应用路径里可以有空格和逗号）。去首尾空白、去空行、去重。 */
export function parseLines(text: string): string[] {
  return [...new Set(text.split(/\r?\n/).map((line) => line.trim()).filter((line) => line !== ''))];
}

/** 以空白、逗号或分号分隔（主机、端口里不会出现这些字符）。 */
export function parseTokens(text: string): string[] {
  return [...new Set(text.split(/[\s,;，；]+/).filter((item) => item !== ''))];
}

/** 往「每行一项」的文本里追加一项；已有时原样返回。 */
export function addLine(text: string, item: string): string {
  const lines = parseLines(text);
  return lines.includes(item) ? text : [...lines, item].join('\n');
}

export interface ItemChip {
  readonly text: string;
  /** 规则集引用（geosite: / geoip:），用强调色显示。 */
  readonly set: boolean;
}

export interface ItemSummary {
  readonly shown: readonly ItemChip[];
  /** 没有显示出来的项数。 */
  readonly more: number;
}

export function summarizeItems(items: readonly string[], max = 2): ItemSummary {
  const shown = items.slice(0, max).map((text) => ({ text, set: looksLikeRuleSet(text) }));
  return { shown, more: Math.max(0, items.length - max) };
}

/** 应用条件的显示文字：分组引用显示分组名，完整路径只显示文件名（完整内容放在 title 里）。 */
export function appText(app: string, groups: readonly AppGroup[]): string {
  if (app.startsWith(APP_GROUP_PREFIX)) {
    const id = app.slice(APP_GROUP_PREFIX.length);
    const group = groups.find((item) => item.id === id);
    return group === undefined ? t(`分组：${id}（不存在）`, `Group: ${id} (missing)`) : t(`分组：${group.name}`, `Group: ${group.name}`);
  }
  const cut = Math.max(app.lastIndexOf('\\'), app.lastIndexOf('/'));
  return cut >= 0 && cut < app.length - 1 ? app.slice(cut + 1) : app;
}

/** 编辑器里的表单：列表字段以文本形式编辑。 */
export interface RuleForm {
  readonly id: string;
  readonly name: string;
  readonly enabled: boolean;
  readonly appsText: string;
  readonly hostsText: string;
  readonly portsText: string;
  readonly network: string;
  readonly action: string;
  readonly targetId: string;
  readonly resolveBefore: boolean;
}

export function blankRuleForm(): RuleForm {
  return { id: '', name: '', enabled: true, appsText: '', hostsText: '', portsText: '', network: 'any', action: 'proxy', targetId: DEFAULT_TARGET_ID, resolveBefore: true };
}

export function formFromRule(rule: Rule): RuleForm {
  return {
    id: rule.id,
    name: rule.name,
    enabled: rule.enabled,
    appsText: rule.apps.join('\n'),
    hostsText: rule.hosts.join('\n'),
    portsText: rule.ports.join(', '),
    network: rule.network,
    action: rule.action,
    targetId: rule.targetId ?? '',
    resolveBefore: rule.resolveBefore,
  };
}

/** 后端要求：非代理动作不能带目标。 */
export function ruleFromForm(form: RuleForm): Rule {
  const base: Rule = {
    id: form.id,
    name: form.name.trim(),
    enabled: form.enabled,
    apps: parseLines(form.appsText),
    hosts: parseTokens(form.hostsText),
    ports: parseTokens(form.portsText),
    network: form.network,
    action: form.action,
    resolveBefore: form.resolveBefore,
  };
  return form.action === 'proxy' ? { ...base, targetId: form.targetId === '' ? DEFAULT_TARGET_ID : form.targetId } : base;
}

export interface RuleFormProblems {
  readonly name: string;
  readonly apps: string;
  readonly hosts: string;
  readonly ports: string;
  readonly target: string;
}

export function hasProblems(problems: Readonly<Record<string, string>>): boolean {
  return Object.values(problems).some((text) => text !== '');
}

function nameProblem(name: string, empty: string): string {
  if (name.trim() === '') return empty;
  return isValidLabel(name.trim()) ? '' : t('名称过长或含有控制字符', 'Name is too long or contains control characters');
}

/** 第一处有问题的条件；全部合法时返回空串。 */
function itemsProblem(items: readonly string[], check: (item: string) => string, tooMany: string): string {
  if (items.length > MAX_RULE_ITEMS) return tooMany;
  for (const [index, item] of items.entries()) {
    const problem = check(item);
    if (problem !== '') return t(`第 ${index + 1} 项「${item}」：${problem}`, `Item #${index + 1} "${item}": ${problem}`);
  }
  return '';
}

/**
 * targetIds：当前可选的分组与节点 id。
 * groups / targetIds 为 null 表示对应列表没读到：跳过「是否存在」的检查，由后端裁决。
 */
export function validateRuleForm(form: RuleForm, groups: readonly AppGroup[] | null, targetIds: readonly string[] | null): RuleFormProblems {
  const tooMany = t(`规则条件数量超过限制（每项最多 ${MAX_RULE_ITEMS} 个）`, `Too many conditions (maximum ${MAX_RULE_ITEMS} per field)`);
  const target = form.action !== 'proxy' || form.targetId === '' || form.targetId === DEFAULT_TARGET_ID || targetIds === null || targetIds.includes(form.targetId);
  return {
    name: nameProblem(form.name, t('请填写规则名称', 'Please enter a rule name')),
    apps: itemsProblem(parseLines(form.appsText), (item) => appProblem(item, groups), tooMany),
    hosts: itemsProblem(parseTokens(form.hostsText), hostProblem, tooMany),
    ports: itemsProblem(parseTokens(form.portsText), portProblem, tooMany),
    target: target ? '' : t('规则目标不存在或不适用于此动作', 'Target does not exist or is invalid for this action'),
  };
}

export interface AppGroupProblems {
  readonly name: string;
  readonly apps: string;
}

/** 空分组后端也收，但规则引用不了（「引用的应用分组为空或无效」），所以这里要求至少一个成员。 */
export function validateAppGroupForm(name: string, appsText: string): AppGroupProblems {
  const apps = parseLines(appsText);
  const member = (app: string): string =>
    app.startsWith('@') || !isValidRuleApp(app) ? t('应用分组成员无效，不能引用其它分组', 'Invalid app entry; cannot reference other groups') : '';
  return {
    name: nameProblem(name, t('请填写分组名称', 'Please enter a group name')),
    apps:
      apps.length === 0
        ? t('请至少填写一个应用', 'Please enter at least one application')
        : itemsProblem(apps, member, t('应用分组成员数量超过限制', 'Too many applications in group')),
  };
}

/** 固定规则不能动；普通规则只能在首尾两条固定规则之间移动。 */
export function canMove(list: readonly Rule[], id: string, delta: -1 | 1): boolean {
  const index = list.findIndex((rule) => rule.id === id);
  const other = list[index + delta];
  return index >= 0 && list[index]?.pinned !== true && other !== undefined && other.pinned !== true;
}

/** 移动后的完整 id 顺序（Rules.Reorder 的参数）；不能移动时返回 null。不修改传入的列表。 */
export function moveRule(list: readonly Rule[], id: string, delta: -1 | 1): string[] | null {
  if (!canMove(list, id, delta)) return null;
  const index = list.findIndex((rule) => rule.id === id);
  const ids = list.map((rule) => rule.id);
  const swapped = ids[index + delta];
  if (swapped === undefined) return null;
  return ids.map((item, position) => (position === index ? swapped : position === index + delta ? id : item));
}

/** 规则在列表里的序号（从 1 起）；不在列表里时为 0。 */
export function ruleNumber(list: readonly Rule[], id: string): number {
  return list.findIndex((rule) => rule.id === id) + 1;
}

const TEST_LABEL = /^[A-Za-z0-9-]+$/;

/** 规则测试器的输入（internal/core/host/rules.go 与 policy.Host）。通过时返回空串。 */
export function validateTestInput(app: string, host: string, portText: string): string {
  if (app.length > 32768 || /[\0\r\n]/.test(app)) return t('应用程序路径无效', 'Invalid application path');
  if (host === '') return t('请填写目标主机', 'Please enter a target host');
  const labels = (host.endsWith('.') ? host.slice(0, -1) : host).split('.');
  const domain = labels.every((label) => label.length <= 63 && TEST_LABEL.test(label) && !label.startsWith('-') && !label.endsWith('-'));
  if (host.length > 253 || !(isIpLiteral(host) || domain)) return t('目标主机须为域名或 IP 地址', 'Target host must be a domain or IP address');
  const port = /^[0-9]{1,5}$/.test(portText) ? Number(portText) : 0;
  return port >= 1 && port <= 65535 ? '' : t('端口须为 1 到 65535 的整数', 'Port must be an integer between 1 and 65535');
}

export interface TestOutcome {
  /** hit：确定命中。uncertain：是否命中取决于域名解析结果，不能当成命中显示。 */
  readonly kind: 'hit' | 'uncertain';
  /** 规则序号；0 表示列表里已经找不到这条规则。 */
  readonly number: number;
  readonly name: string;
  readonly action: string;
  /** 代理动作的出站名称；其余为空。 */
  readonly outbound: string;
}

/** outboundNames：分组 id、节点 id 或节点 tag → 显示名称。 */
export function describeTestResult(result: RuleTestResult, list: readonly Rule[], outboundNames: ReadonlyMap<string, string>): TestOutcome {
  const number = ruleNumber(list, result.matchedRuleId);
  if (result.stage !== TEST_STAGE_MATCHED) return { kind: 'uncertain', number, name: result.matchedRuleName, action: '', outbound: '' };
  const outbound = result.action === 'proxy' ? (outboundNames.get(result.outboundTag) ?? result.outboundTag) : '';
  return { kind: 'hit', number, name: result.matchedRuleName, action: result.action, outbound };
}

export interface StatusView {
  readonly label: string;
  readonly tone: 'ok' | 'warn' | 'err' | 'none';
}

export function ruleSetStatus(status: string): StatusView {
  if (status === 'ready') return { label: t('已就绪', 'Ready'), tone: 'ok' };
  if (status === 'missing') return { label: t('未下载', 'Not Downloaded'), tone: 'none' };
  if (status === 'updating') return { label: t('更新中', 'Updating'), tone: 'warn' };
  if (status === 'error') return { label: t('更新失败', 'Failed'), tone: 'err' };
  if (status === 'invalid') return { label: t('文件无效', 'Invalid File'), tone: 'err' };
  return { label: status === '' ? '—' : status, tone: 'none' };
}

/** 逐条导入的结果说明。error 为空表示全部成功。 */
export function importOutcomeText(total: number, error: string): string {
  return error === '' ? t(`已导入 ${total} 条规则`, `Imported ${total} rules`) : t(`没有导入任何规则：${error}`, `No rules imported: ${error}`);
}

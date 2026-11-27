/*
 * 规则、应用分组与规则集的解析。约定与 parse.ts 相同：后端返回值一律当作 unknown，
 * 字段缺失或类型不符时回退到安全默认值，且只取约定的字段。
 */

import { asRecord, isRecord, readArray, readBool, readNumber, readString, readStringArray } from '../guard';
import type { AppGroup, ImportPreview, Rule, RuleSetInfo, RuleTestResult } from './types-rules';

/** 动作缺失时保持空串：界面把它显示成「未知」，不会冒充成代理或直连。 */
export function parseRule(value: unknown): Rule {
  const source = asRecord(value);
  const targetId = readString(source, 'targetId');
  return {
    id: readString(source, 'id'),
    name: readString(source, 'name'),
    enabled: readBool(source, 'enabled'),
    apps: readStringArray(source, 'apps'),
    hosts: readStringArray(source, 'hosts'),
    ports: readStringArray(source, 'ports'),
    network: readString(source, 'network', 'any'),
    action: readString(source, 'action'),
    ...(targetId === '' ? {} : { targetId }),
    resolveBefore: readBool(source, 'resolveBefore', true),
    ...(readBool(source, 'pinned') ? { pinned: true } : {}),
  };
}

/** Rules.List() 的结果：没有 id 的条目无法操作，也不该显示。 */
export function parseRules(value: unknown): Rule[] {
  return (Array.isArray(value) ? value : []).map(parseRule).filter((rule) => rule.id !== '');
}

export function parseAppGroup(value: unknown): AppGroup {
  const source = asRecord(value);
  return {
    id: readString(source, 'id'),
    name: readString(source, 'name'),
    apps: readStringArray(source, 'apps'),
  };
}

export function parseAppGroups(value: unknown): AppGroup[] {
  return (Array.isArray(value) ? value : []).map(parseAppGroup).filter((group) => group.id !== '');
}

/** 预览里的规则还没有保存，id 只是临时标识，所以这里不按 id 过滤。 */
export function parseImportPreview(value: unknown): ImportPreview {
  const source = asRecord(value);
  return {
    rules: readArray(source, 'rules').filter(isRecord).map(parseRule),
    warnings: readStringArray(source, 'warnings'),
  };
}

/** stage 缺失时为空串；只有 stage === 'matched' 才能当作确定命中。 */
export function parseRuleTestResult(value: unknown): RuleTestResult {
  const source = asRecord(value);
  return {
    matchedRuleId: readString(source, 'matchedRuleId'),
    matchedRuleName: readString(source, 'matchedRuleName'),
    action: readString(source, 'action'),
    outboundTag: readString(source, 'outboundTag'),
    chain: readStringArray(source, 'chain'),
    stage: readString(source, 'stage'),
  };
}

function parseRuleSet(value: unknown): RuleSetInfo {
  const source = asRecord(value);
  return {
    id: readString(source, 'id'),
    name: readString(source, 'name'),
    url: readString(source, 'url'),
    updatedAt: readNumber(source, 'updatedAt'),
    sizeBytes: readNumber(source, 'sizeBytes'),
    status: readString(source, 'status', 'missing'),
  };
}

export function parseRuleSets(value: unknown): RuleSetInfo[] {
  return (Array.isArray(value) ? value : []).map(parseRuleSet).filter((item) => item.id !== '');
}

/*
 * 规则、应用分组与规则集的类型。字段名来自 internal/spec/types.go、
 * internal/routing/import.go、internal/telemetry/{rules,rulesets}.go，不要改名。
 */

/** 固定首条「本机 / 局域网」与固定末条「默认规则」。不可编辑、禁用、删除或移动。 */
export const RULE_ID_LAN = 'builtin-lan';
export const RULE_ID_DEFAULT = 'builtin-default';

export const RULE_ACTIONS = ['proxy', 'direct', 'block'] as const;
export type RuleAction = (typeof RULE_ACTIONS)[number];

export const RULE_NETWORKS = ['any', 'tcp', 'udp'] as const;
export type RuleNetwork = (typeof RULE_NETWORKS)[number];

/** action=proxy 且 targetId 为它（或为空）时，指向默认代理分组。 */
export const DEFAULT_TARGET_ID = 'proxy';

/** 引用应用分组的写法：@group:<分组 id>。 */
export const APP_GROUP_PREFIX = '@group:';

/** 引用自定义远程 SRS 规则集的写法：ruleset:<规则集 id>。 */
export const CUSTOM_RULE_SET_PREFIX = 'ruleset:';

/** Rules.List() / Rules.Save() 的规则（internal/spec.Rule）。 */
export interface Rule {
  /** 新建时为空串，由后端生成。 */
  id: string;
  name: string;
  enabled: boolean;
  /** 字段内为「或」，字段间为「且」，空数组表示任意。 */
  apps: string[];
  hosts: string[];
  ports: string[];
  /** any | tcp | udp */
  network: string;
  /** direct | block | proxy */
  action: string;
  /** 仅 action=proxy 时有意义：分组或节点的 id。 */
  targetId?: string;
  /** IP 类条件匹配前先解析域名。 */
  resolveBefore: boolean;
  pinned?: boolean;
}

/** AppGroups.List() 的应用分组（internal/spec.AppGroup）。 */
export interface AppGroup {
  id: string;
  name: string;
  apps: string[];
}

/** Rules.Import() 的预览：只解析不写入。 */
export interface ImportPreview {
  rules: Rule[];
  warnings: string[];
}

/** Rules.Import() 支持的来源。 */
export const IMPORT_KINDS = ['clash', 'singbox', 'ppx'] as const;
export type ImportKind = (typeof IMPORT_KINDS)[number];

/** stage=matched 才是确定命中；dns-required 表示还需要真实 DNS 结果，不能当成命中。 */
export const TEST_STAGE_MATCHED = 'matched';
export const TEST_STAGE_DNS = 'dns-required';

/** Rules.Test() 的结果（internal/telemetry.RuleTestResult）。 */
export interface RuleTestResult {
  matchedRuleId: string;
  matchedRuleName: string;
  action: string;
  outboundTag: string;
  chain: string[];
  stage: string;
}

/** 自定义远程 SRS 规则集定义（internal/spec.RuleSet）。 */
export interface CustomRuleSet {
  id: string;
  name: string;
  url: string;
  autoUpdate?: boolean;
  /** 0 表示继承全局刷新间隔；1–720 小时。 */
  updateHours?: number;
}

/** 懒人分流预设选项（internal/routing.LazyPresetOptions）。 */
export interface LazyPresetOptions {
  replaceExisting: boolean;
  enableAds: boolean;
  enableAi: boolean;
  enableAppleSteam: boolean;
  enableCnSplit: boolean;
  enableSplitDns: boolean;
  intranetDns: string;
  intranetDomains: string[];
}

/** Rules.ApplyLazyPreset() 的返回值。 */
export interface ApplyPresetResult {
  ruleCount: number;
  ruleSetCount: number;
  appGroupCount: number;
  dnsRuleCount: number;
  missingRuleSets: string[];
}

/** RuleSets.List() 的规则集（internal/telemetry.RuleSetInfo）。 */
export interface RuleSetInfo {
  id: string;
  name: string;
  url: string;
  /** Unix 毫秒；0 表示还没有下载过。 */
  updatedAt: number;
  sizeBytes: number;
  /** missing | ready | updating | error | invalid */
  status: string;
  custom?: boolean;
  autoUpdate?: boolean;
  updateHours?: number;
}

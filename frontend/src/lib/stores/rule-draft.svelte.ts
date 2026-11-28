/*
 * 连接页与规则页之间的接力：
 * 1. 「据此创建规则 / 一键建规则」传递未保存的规则草稿；
 * 2. 「命中溯源」传递 ruleId，规则页挂载后自动滚动并高亮对应规则行。
 */

import type { Rule } from '../api';

class RuleDraftStore {
  pending = $state.raw<Rule | null>(null);
  highlightRuleId = $state('');

  offer(rule: Rule): void {
    this.pending = rule;
  }

  take(): Rule | null {
    const rule = this.pending;
    this.pending = null;
    return rule;
  }

  focusRule(ruleId: string): void {
    this.highlightRuleId = ruleId;
  }

  takeHighlight(): string {
    const id = this.highlightRuleId;
    this.highlightRuleId = '';
    return id;
  }
}

export const ruleDraft = new RuleDraftStore();

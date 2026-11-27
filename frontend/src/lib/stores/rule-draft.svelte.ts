/*
 * 「据此创建规则」的接力。
 *
 * 连接页拿到后端预填的规则（Rules.CreateFromConnection，尚未保存、id 为空）后放在这里，
 * 再跳到规则页；规则页挂载时取走并打开编辑器。取走即清空，不会重复弹出。
 */

import type { Rule } from '../api';

class RuleDraftStore {
  pending = $state.raw<Rule | null>(null);

  offer(rule: Rule): void {
    this.pending = rule;
  }

  take(): Rule | null {
    const rule = this.pending;
    this.pending = null;
    return rule;
  }
}

export const ruleDraft = new RuleDraftStore();

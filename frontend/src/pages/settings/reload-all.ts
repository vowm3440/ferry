/*
 * 从备份恢复之后，后端的节点、订阅、规则、设置整份都换了：让每个 store 重新读取。
 * 各 store 读取失败时把说明留在自己身上，由各自的页面展示，这里不重复提示。
 */

import { conns } from '../../lib/stores/conns.svelte';
import { core } from '../../lib/stores/core.svelte';
import { logs } from '../../lib/stores/logs.svelte';
import { nodes } from '../../lib/stores/nodes.svelte';
import { rules } from '../../lib/stores/rules.svelte';
import { settings } from '../../lib/stores/settings.svelte';
import { subs } from '../../lib/stores/subs.svelte';
import { system } from '../../lib/stores/system.svelte';

export async function reloadAll(): Promise<void> {
  // 恢复会重启正在运行的内核：先拿到最新的内核状态，其余并行读取。
  await core.refresh();
  await Promise.all([
    settings.load(),
    nodes.refresh(),
    subs.refresh(),
    rules.refresh(),
    conns.refresh(),
    logs.refresh(),
    system.refresh(),
  ]);
}

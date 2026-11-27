<!--
  路由规则表格：自上而下匹配。固定的首尾两条规则带锁，不能编辑、停用、移动或删除。
  排序用「上移 / 下移」按钮，键盘可用；按钮在不能移动或有改动进行中时只是不响应，不会丢焦点。
-->
<script lang="ts">
  import { tick } from 'svelte';
  import { DEFAULT_TARGET_ID, type Rule } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { actionLabel, appText, canMove, networkLabel } from '../../lib/rules-view';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import { rules } from '../../lib/stores/rules.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import RuleChips from './RuleChips.svelte';

  interface Props {
    onedit: (rule: Rule) => void;
    onremove: (rule: Rule) => void;
  }

  let { onedit, onremove }: Props = $props();

  let host = $state<HTMLElement | null>(null);

  /** 分组与节点的 id → 名称。 */
  const targetNames = $derived(
    new Map<string, string>([
      ...nodes.list.nodes.map((node): [string, string] => [node.id, node.name]),
      ...nodes.list.groups.map((group): [string, string] => [group.id, group.name]),
    ]),
  );

  function actionText(rule: Rule): string {
    void i18n.locale;
    if (rule.action !== 'proxy') return actionLabel(rule.action);
    const id = rule.targetId === undefined || rule.targetId === '' ? DEFAULT_TARGET_ID : rule.targetId;
    return targetNames.get(id) ?? (id === DEFAULT_TARGET_ID ? tr('默认代理', 'Default Proxy') : tr(`${id}（不存在）`, `${id} (missing)`));
  }

  function pillTone(action: string): string {
    return action === 'proxy' || action === 'direct' || action === 'block' ? action : '';
  }

  async function move(rule: Rule, delta: -1 | 1): Promise<void> {
    if (rules.busy || !canMove(rules.list, rule.id, delta)) return;
    const result = await rules.move(rule.id, delta);
    if (!result.ok) {
      toasts.error(result.message);
      return;
    }
    // 行在表格里换了位置，浏览器会让正在用的按钮失去焦点：列表更新后放回这条规则同方向的按钮上，方便连续移动。
    await tick();
    if (document.activeElement === null || document.activeElement === document.body) {
      host?.querySelector<HTMLElement>(`[data-rule-id="${CSS.escape(rule.id)}"] [data-move="${delta}"]`)?.focus();
    }
  }

  async function setEnabled(rule: Rule, on: boolean): Promise<void> {
    if (rules.busy) return;
    const result = await rules.setEnabled(rule.id, on);
    if (!result.ok) toasts.error(result.message);
  }

  function act(run: (rule: Rule) => void, rule: Rule): void {
    if (!rules.busy) run(rule);
  }
</script>

<div class="card table-card" bind:this={host}>
  <div class="tbl-wrap">
    <table class="tbl" aria-label={tr('路由规则', 'Routing rules')} aria-busy={rules.busy}>
      <thead>
        <tr>
          <th><span class="sr-only">{tr('顺序', 'Order')}</span></th>
          <th><span class="sr-only">{tr('启用', 'Enabled')}</span></th>
          <th>#</th>
          <th>{tr('名称', 'Name')}</th>
          <th>{tr('应用程序', 'Apps')}</th>
          <th>{tr('目标主机', 'Hosts')}</th>
          <th>{tr('端口', 'Ports')}</th>
          <th>{tr('动作', 'Action')}</th>
          <th><span class="sr-only">{tr('操作', 'Actions')}</span></th>
        </tr>
      </thead>
      <tbody>
        {#each rules.list as rule, index (rule.id)}
          {@const pinned = rule.pinned === true}
          <tr class:pinned class:off={!rule.enabled} data-rule-id={rule.id}>
            <td class="order keep">
              {#if pinned}
                <span class="lock" title={tr('固定规则：不能编辑、停用或移动', 'Pinned rule: cannot edit, disable, or move')}><Icon name="lock" /><span class="sr-only">{tr('固定规则', 'Pinned rule')}</span></span>
              {:else}
                <span class="moves">
                  <button
                    type="button"
                    class="icon-btn"
                    aria-label={tr(`上移「${rule.name}」`, `Move up "${rule.name}"`)}
                    title={tr('上移', 'Move up')}
                    data-move="-1"
                    aria-disabled={rules.busy || !canMove(rules.list, rule.id, -1)}
                    onclick={() => void move(rule, -1)}
                  >
                    <Icon name="chev-up" />
                  </button>
                  <button
                    type="button"
                    class="icon-btn flip"
                    aria-label={tr(`下移「${rule.name}」`, `Move down "${rule.name}"`)}
                    title={tr('下移', 'Move down')}
                    data-move="1"
                    aria-disabled={rules.busy || !canMove(rules.list, rule.id, 1)}
                    onclick={() => void move(rule, 1)}
                  >
                    <Icon name="chev-up" />
                  </button>
                </span>
              {/if}
            </td>
            <td class="toggle keep">
              {#if !pinned}
                <Switch checked={rule.enabled} label={tr(`启用「${rule.name}」`, `Enable "${rule.name}"`)} onchange={(next) => void setEnabled(rule, next)} />
              {/if}
            </td>
            <td class="idx num">{index + 1}</td>
            <td class="name" title={rule.name}>
              <b>{rule.name}</b>
              {#if rule.network !== 'any'}
                <span class="chip">{(void i18n.locale, networkLabel(rule.network))}</span>
              {/if}
            </td>
            <td><RuleChips items={rule.apps} text={(app) => (void i18n.locale, appText(app, rules.groups))} /></td>
            <td><RuleChips items={rule.hosts} /></td>
            <td>
              {#if rule.ports.length === 0}
                <span class="muted">{tr('任意', 'Any')}</span>
              {:else}
                <span class="mono ports" title={rule.ports.join(', ')}>{rule.ports.join(', ')}</span>
              {/if}
            </td>
            <td><span class={`pill ${pillTone(rule.action)}`}>{actionText(rule)}</span></td>
            <td class="ops keep">
              {#if !pinned}
                <button
                  type="button"
                  class="icon-btn"
                  data-row-focus
                  aria-label={tr(`编辑「${rule.name}」`, `Edit "${rule.name}"`)}
                  title={tr('编辑', 'Edit')}
                  aria-disabled={rules.busy}
                  onclick={() => act(onedit, rule)}
                >
                  <Icon name="edit" />
                </button>
                <button
                  type="button"
                  class="icon-btn"
                  aria-label={tr(`删除「${rule.name}」`, `Remove "${rule.name}"`)}
                  title={tr('删除', 'Remove')}
                  aria-disabled={rules.busy}
                  onclick={() => act(onremove, rule)}
                >
                  <Icon name="trash" />
                </button>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</div>

<style>
  /* 表头里只给读屏用的文字是绝对定位的：让它跟着滚动容器走，否则表格横向滚动时会把整页撑宽。 */
  .tbl-wrap {
    position: relative;
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
  }

  .table-card {
    padding: 4px 6px;
    overflow: hidden;
  }

  .tbl tr.pinned td {
    background: var(--muted-soft);
  }

  .tbl tr.off td:not(.keep) {
    opacity: 0.45;
  }

  .order {
    width: 60px;
    padding-right: 0;
  }

  .lock {
    display: inline-grid;
    place-items: center;
    width: 26px;
    color: var(--text-3);
  }

  .moves,
  .ops {
    white-space: nowrap;
  }

  .moves {
    display: inline-flex;
    gap: 2px;
  }

  .flip :global(svg) {
    transform: rotate(180deg);
  }

  .toggle {
    width: 44px;
  }

  .idx {
    width: 26px;
    font-size: 12px;
    color: var(--text-3);
  }

  .name {
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .name b {
    font-weight: 500;
  }

  .name .chip {
    margin-left: 6px;
  }

  .ports {
    display: inline-block;
    max-width: 140px;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: middle;
  }

  .ops {
    width: 72px;
    text-align: right;
  }

  /* .icon-btn 默认是块级的 grid：在单元格里并排要改成行内。 */
  .ops .icon-btn {
    display: inline-grid;
    vertical-align: middle;
  }
</style>

<!-- 导入预览：将要写入的规则与后端给出的提示。只是展示；可以把某一条从这次导入里拿掉。 -->
<script lang="ts">
  import { DEFAULT_TARGET_ID, type Rule } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { actionLabel, appText } from '../../lib/rules-view';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import { rules as store } from '../../lib/stores/rules.svelte';
  import RuleChips from './RuleChips.svelte';

  interface Props {
    /** 这次要写入的规则（已经去掉了用户不要的）。 */
    rules: readonly Rule[];
    warnings: readonly string[];
    /** 解析出来的规则总数。 */
    total: number;
    busy: boolean;
    /** 不导入第 index 条（rules 里的下标）。 */
    onremove: (index: number) => void;
  }

  let { rules, warnings, total, busy, onremove }: Props = $props();

  function actionText(rule: Rule): string {
    void i18n.locale;
    if (rule.action !== 'proxy') return actionLabel(rule.action);
    const id = rule.targetId === undefined || rule.targetId === '' ? DEFAULT_TARGET_ID : rule.targetId;
    const name = nodes.list.groups.find((group) => group.id === id)?.name ?? nodes.list.nodes.find((node) => node.id === id)?.name;
    return name ?? (id === DEFAULT_TARGET_ID ? tr('默认代理', 'Default Proxy') : id);
  }

  function pillTone(action: string): string {
    return action === 'proxy' || action === 'direct' || action === 'block' ? action : '';
  }
</script>

<p class="summary" data-import-summary>
  {#if total === 0}
    {tr('没有解析出可以导入的规则。', 'No importable rules were parsed.')}
  {:else if rules.length === total}
    {#if i18n.isEnglish}
      Parsed <b class="num">{total}</b> rules (not saved yet). Confirm to commit atomically.
    {:else}
      解析出 <b class="num">{total}</b> 条规则，尚未写入。确认后一次写入：全部通过才生效，否则一条都不写。
    {/if}
  {:else if i18n.isEnglish}
    Parsed <b class="num">{total}</b> rules, skipped <b class="num">{total - rules.length}</b>, importing <b class="num">{rules.length}</b>.
  {:else}
    解析出 <b class="num">{total}</b> 条规则，已拿掉 <b class="num">{total - rules.length}</b> 条，将导入 <b class="num">{rules.length}</b> 条。
  {/if}
</p>

{#if rules.length > 0}
  <div class="tbl-wrap list">
    <table class="tbl" aria-label={tr('将要导入的规则', 'Rules to import')}>
      <thead>
        <tr>
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
        {#each rules as rule, index (rule.id === '' ? index : rule.id)}
          <tr class:off={!rule.enabled}>
            <td class="idx num">{index + 1}</td>
            <td class="name" title={rule.name}>{rule.name}</td>
            <td><RuleChips items={rule.apps} text={(app) => (void i18n.locale, appText(app, store.groups))} /></td>
            <td><RuleChips items={rule.hosts} /></td>
            <td>
              {#if rule.ports.length === 0}
                <span class="muted">{tr('任意', 'Any')}</span>
              {:else}
                <span class="mono ports" title={rule.ports.join(', ')}>{rule.ports.join(', ')}</span>
              {/if}
            </td>
            <td><span class={`pill ${pillTone(rule.action)}`}>{actionText(rule)}</span></td>
            <td class="drop">
              <button type="button" class="icon-btn" data-drop aria-label={tr(`不导入「${rule.name}」`, `Exclude "${rule.name}"`)} title={tr('不导入这一条', 'Exclude this rule')} aria-disabled={busy} onclick={() => !busy && onremove(index)}>
                <Icon name="trash" />
              </button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

{#if warnings.length > 0}
  <div class="warnings" role="group" aria-label={tr('导入提示', 'Import warnings')}>
    <div class="warn-title"><Icon name="alert" />{tr(`有 ${warnings.length} 条提示（这些内容没有或没有完整转换）`, `${warnings.length} warning(s) (skipped or partially converted items)`)}</div>
    <ul class="selectable">
      {#each warnings as warning, index (index)}
        <li>{warning}</li>
      {/each}
    </ul>
  </div>
{/if}

<style>
  .summary {
    margin: 0 0 10px;
    color: var(--text-2);
  }

  .summary b {
    color: var(--text);
  }

  .list {
    max-height: 264px;
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
    border: 1px solid var(--border);
    border-radius: 8px;
  }

  .tbl th {
    position: sticky;
    top: 0;
    z-index: 1;
    background: var(--surface);
  }

  .tbl td {
    height: 36px;
  }

  .tbl tr.off td {
    opacity: 0.45;
  }

  .drop {
    width: 36px;
    text-align: right;
  }

  .tbl tr.off td.drop {
    opacity: 1;
  }

  .idx {
    width: 30px;
    font-size: 12px;
    color: var(--text-3);
  }

  .name {
    max-width: 170px;
    overflow: hidden;
    text-overflow: ellipsis;
    font-weight: 500;
  }

  .ports {
    display: inline-block;
    max-width: 110px;
    overflow: hidden;
    text-overflow: ellipsis;
    vertical-align: middle;
  }

  .warnings {
    margin-top: 12px;
    padding: 10px 12px;
    border-radius: 8px;
    background: var(--warn-soft, var(--muted-soft));
    font-size: 12.5px;
  }

  .warn-title {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 500;
  }

  .warn-title :global(svg) {
    width: 14px;
    height: 14px;
    flex: none;
    color: var(--warn);
  }

  ul {
    margin: 6px 0 0;
    padding-left: 20px;
    max-height: 120px;
    overflow-y: auto;
    color: var(--text-2);
  }
</style>

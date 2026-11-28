<!--
  规则集：包含内置 4 个目录规则集 + 用户自定义的远程 HTTPS .srs 规则集。
  内置使用 geosite: / geoip: 引用，自定义使用 ruleset:<id> 引用。
  列表只读本地缓存状态，点击「更新」才会下载；支持按周期后台定时刷新。
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import type { CustomRuleSet, RuleSetInfo } from '../../lib/api';
  import ConfirmDialog from '../../lib/components/ConfirmDialog.svelte';
  import EmptyState from '../../lib/components/EmptyState.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import { formatBytes } from '../../lib/format';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { MAX_CUSTOM_RULE_SETS, ruleSetRefFromInfo, ruleSetRefs } from '../../lib/rules-validate';
  import { ruleSetStatus } from '../../lib/rules-view';
  import { rules } from '../../lib/stores/rules.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import { formatDateTime } from '../../lib/time';
  import RuleSetDialog from './RuleSetDialog.svelte';

  const MAX_UPDATE = 4 + MAX_CUSTOM_RULE_SETS;

  let selected = $state.raw<readonly string[]>([]);
  let dialogOpen = $state(false);
  let editing = $state.raw<CustomRuleSet | null>(null);
  let removing = $state.raw<RuleSetInfo | null>(null);
  let removeBusy = $state(false);
  let removeError = $state('');

  const updating = $derived(rules.pending === 'sets');
  /** 只保留还在列表里的选择。 */
  const chosen = $derived(selected.filter((id) => rules.sets.some((item) => item.id === id)));
  const refs = $derived((void i18n.locale, ruleSetRefs()));
  const customCount = $derived(rules.sets.filter((item) => item.custom === true).length);

  onMount(() => {
    if (!rules.setsLoaded && !rules.setsLoading) void rules.loadSets(false);
  });

  const labelOf = (item: RuleSetInfo): string => refs.find((entry) => entry.id === item.id)?.label ?? item.name;

  function toggle(id: string, on: boolean): void {
    selected = on ? [...chosen.filter((item) => item !== id), id] : chosen.filter((item) => item !== id);
  }

  function reload(): void {
    if (!rules.setsLoading && !rules.busy) void rules.loadSets(true);
  }

  function openCreate(): void {
    if (rules.busy) return;
    if (customCount >= MAX_CUSTOM_RULE_SETS) {
      toasts.info(tr(`自定义规则集最多 ${MAX_CUSTOM_RULE_SETS} 个`, `Maximum ${MAX_CUSTOM_RULE_SETS} custom rule sets`));
      return;
    }
    editing = null;
    dialogOpen = true;
  }

  function openEdit(item: RuleSetInfo): void {
    if (rules.busy || item.custom !== true) return;
    editing = {
      id: item.id,
      name: item.name,
      url: item.url,
      autoUpdate: item.autoUpdate ?? false,
      updateHours: item.updateHours ?? 24,
    };
    dialogOpen = true;
  }

  function askRemove(item: RuleSetInfo): void {
    if (rules.busy || item.custom !== true) return;
    removeError = '';
    removing = item;
  }

  async function confirmRemove(): Promise<void> {
    const target = removing;
    if (target === null || removeBusy) return;
    removeBusy = true;
    const result = await rules.removeRuleSet(target.id);
    removeBusy = false;
    if (!result.ok) {
      removeError = result.message;
      return;
    }
    removing = null;
    toasts.ok(tr(`已删除规则集「${target.name}」`, `Removed rule set "${target.name}"`));
  }

  async function update(ids: readonly string[]): Promise<void> {
    if (rules.busy) return;
    if (ids.length === 0) {
      toasts.info(tr('请先勾选要更新的规则集', 'Select at least one rule set to update'));
      return;
    }
    if (ids.length > MAX_UPDATE) {
      toasts.info(tr(`一次最多更新 ${MAX_UPDATE} 个规则集`, `Up to ${MAX_UPDATE} rule sets per update`));
      return;
    }
    const result = await rules.updateSets(ids);
    if (!result.ok) {
      toasts.error(result.message);
      return;
    }
    selected = [];
    toasts.ok(tr(`已更新 ${ids.length} 个规则集，重新应用规则或重启内核后生效`, `Updated ${ids.length} rule set(s); applies after re-applying rules or restarting core`));
  }
</script>

<div class="bar-row">
  <span class="muted">{tr(`共 ${rules.sets.length} 个规则集（自定义 ${customCount} 个），已选 ${chosen.length} 个`, `${rules.sets.length} rule sets (${customCount} custom) · ${chosen.length} selected`)}</span>
  <span class="btns">
    <button type="button" class="icon-btn" class:spin={rules.setsLoading} aria-label={tr('重新读取规则集状态', 'Refresh rule set status')} title={tr('重新读取状态（不下载）', 'Refresh local cache status')} aria-disabled={rules.setsLoading || rules.busy} onclick={reload}>
      <Icon name="refresh" />
    </button>
    <button type="button" class="btn sm" aria-disabled={rules.busy} onclick={() => void update(chosen)}>{updating ? tr('更新中…', 'Updating…') : tr('更新所选', 'Update Selected')}</button>
    <button type="button" class="btn sm" disabled={rules.sets.length === 0} aria-disabled={rules.busy} onclick={() => void update(rules.sets.map((item) => item.id))}>{tr('全部更新', 'Update All')}</button>
    <button type="button" class="btn sm primary" data-ruleset-add aria-disabled={rules.busy} onclick={openCreate}><Icon name="plus" />{tr('添加远程规则集', 'Add Remote SRS')}</button>
  </span>
</div>

<div class="inline-note gap">
  <Icon name="info" />
  <span>{tr('支持自定义远程 HTTPS .srs 规则集与定时刷新；下载完成后，在路由规则或分流 DNS 中用 ruleset:<id> 引用，重新应用规则或重启内核后生效。', 'Supports custom remote HTTPS .srs rule sets and scheduled refresh. Reference them via ruleset:<id> in routing or split DNS rules; applies after re-applying rules or restarting core.')}</span>
</div>

{#if rules.setsError !== ''}
  <div class="inline-error gap" role="alert">
    <Icon name="alert" />
    <span class="grow">{tr(`读取规则集失败：${rules.setsError}`, `Failed to load rule sets: ${rules.setsError}`)}</span>
    <button type="button" class="btn sm" onclick={() => void rules.loadSets(true)}>{tr('重试', 'Retry')}</button>
  </div>
{/if}

<div class="card table-card">
  {#if !rules.setsLoaded}
    {#if rules.setsError === ''}
      <EmptyState icon="box" title={tr('正在读取规则集…', 'Loading rule sets…')} description={tr('正在读取本机缓存的规则集状态。', 'Reading local rule-set cache metadata.')} />
    {:else}
      <EmptyState icon="alert" tone="error" title={tr('无法读取规则集', 'Failed to Load Rule Sets')} description={rules.setsError} />
    {/if}
  {:else if rules.sets.length === 0}
    <EmptyState icon="box" title={tr('没有可用的规则集', 'No Rule Sets Available')} description={tr('后端没有列出任何规则集。', 'No rule sets were listed by the backend.')} />
  {:else}
    <div class="tbl-wrap">
      <table class="tbl" aria-label={tr('规则集', 'Rule sets')} aria-busy={updating}>
        <thead>
          <tr>
            <th><span class="sr-only">{tr('选择', 'Select')}</span></th>
            <th>{tr('名称', 'Name')}</th>
            <th>{tr('规则里的写法', 'Reference')}</th>
            <th>{tr('刷新', 'Refresh')}</th>
            <th>{tr('状态', 'Status')}</th>
            <th class="r">{tr('大小', 'Size')}</th>
            <th>{tr('更新时间', 'Updated')}</th>
            <th><span class="sr-only">{tr('操作', 'Actions')}</span></th>
          </tr>
        </thead>
        <tbody>
          {#each rules.sets as item (item.id)}
            {@const status = (void i18n.locale, ruleSetStatus(item.status))}
            {@const ref = ruleSetRefFromInfo(item)}
            <tr data-set-id={item.id}>
              <td class="pick">
                <input type="checkbox" aria-label={tr(`选择「${labelOf(item)}」`, `Select "${labelOf(item)}"`)} checked={chosen.includes(item.id)} onchange={(event) => toggle(item.id, event.currentTarget.checked)} />
              </td>
              <td class="name" title={item.url}>
                <span>{labelOf(item)}</span>
                {#if item.custom}
                  <span class="chip">{tr('自定义', 'Custom')}</span>
                {/if}
              </td>
              <td><span class="chip set">{ref}</span></td>
              <td class="muted">
                {#if item.custom}
                  {item.autoUpdate ? tr(`每 ${item.updateHours || 24}h`, `Every ${item.updateHours || 24}h`) : tr('手动', 'Manual')}
                {:else}
                  {tr('跟随设置', 'Global')}
                {/if}
              </td>
              <td><span class={`pill state ${status.tone}`}>{status.label}</span></td>
              <td class="r num">{item.sizeBytes > 0 ? formatBytes(item.sizeBytes) : '—'}</td>
              <td class="num">{item.updatedAt > 0 ? formatDateTime(item.updatedAt) : '—'}</td>
              <td class="ops">
                {#if item.custom}
                  <button type="button" class="icon-btn" aria-label={tr(`编辑「${item.name}」`, `Edit "${item.name}"`)} title={tr('编辑', 'Edit')} aria-disabled={rules.busy} onclick={() => openEdit(item)}>
                    <Icon name="edit" />
                  </button>
                  <button type="button" class="icon-btn" aria-label={tr(`删除「${item.name}」`, `Remove "${item.name}"`)} title={tr('删除', 'Remove')} aria-disabled={rules.busy} onclick={() => askRemove(item)}>
                    <Icon name="trash" />
                  </button>
                {/if}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

{#if dialogOpen}
  <RuleSetDialog initial={editing} onclose={() => (dialogOpen = false)} />
{/if}

<ConfirmDialog
  open={removing !== null}
  title={tr('删除自定义规则集', 'Remove Custom Rule Set')}
  message={removing === null ? '' : tr(`删除「${removing.name}」并清理本地缓存？若仍有规则引用，需先移除引用。`, `Remove "${removing.name}" and delete its local cache? Remove references from rules first.`)}
  confirmLabel={tr('删除', 'Remove')}
  busyLabel={tr('删除中…', 'Removing…')}
  danger
  busy={removeBusy}
  onconfirm={() => void confirmRemove()}
  oncancel={() => (removing = null)}
>
  {#if removeError !== ''}
    <div class="inline-error" role="alert"><Icon name="alert" /><span class="selectable">{removeError}</span></div>
  {/if}
</ConfirmDialog>

<style>
  .bar-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 10px;
    font-size: 12.5px;
  }

  .btns {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
  }

  .gap {
    margin-bottom: 10px;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .tbl-wrap {
    position: relative;
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
  }

  .table-card {
    padding: 4px 6px;
    overflow: hidden;
  }

  .pick {
    width: 32px;
  }

  .pick input {
    width: 15px;
    height: 15px;
    accent-color: var(--accent);
    vertical-align: middle;
  }

  .name {
    font-weight: 500;
  }

  .name .chip {
    margin-left: 6px;
  }

  .chip.set {
    color: var(--accent-strong);
    border-color: transparent;
    background: var(--accent-soft);
    font-family: var(--font-mono);
  }

  .pill.state.ok {
    background: var(--ok-soft);
    color: var(--ok);
  }

  .pill.state.err {
    background: var(--err-soft);
    color: var(--err);
  }

  .ops {
    white-space: nowrap;
    text-align: right;
  }

  .ops .icon-btn {
    display: inline-grid;
    vertical-align: middle;
  }
</style>

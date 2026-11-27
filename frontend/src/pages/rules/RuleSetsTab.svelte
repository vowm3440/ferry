<!--
  规则集：后端目录里固定的几个，规则的「目标主机」里用 geosite: / geoip: 引用。
  列表只读本地缓存的状态，不会自动下载；点「更新」才会下载。
  更新后的内容要等重新应用规则或重启内核才会被内核加载，这里如实说明。
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import EmptyState from '../../lib/components/EmptyState.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import { formatBytes } from '../../lib/format';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { ruleSetRefs } from '../../lib/rules-validate';
  import { ruleSetStatus } from '../../lib/rules-view';
  import { rules } from '../../lib/stores/rules.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import { formatDateTime } from '../../lib/time';

  /** RuleSets.Update 一次最多接受的个数。 */
  const MAX_UPDATE = 4;

  let selected = $state.raw<readonly string[]>([]);

  const updating = $derived(rules.pending === 'sets');
  /** 只保留还在列表里的选择。 */
  const chosen = $derived(selected.filter((id) => rules.sets.some((item) => item.id === id)));
  const refs = $derived((void i18n.locale, ruleSetRefs()));

  onMount(() => {
    if (!rules.setsLoaded && !rules.setsLoading) void rules.loadSets(false);
  });

  const refOf = (id: string): string => refs.find((item) => item.id === id)?.ref ?? '';
  const labelOf = (id: string, name: string): string => refs.find((item) => item.id === id)?.label ?? name;

  function toggle(id: string, on: boolean): void {
    selected = on ? [...chosen.filter((item) => item !== id), id] : chosen.filter((item) => item !== id);
  }

  function reload(): void {
    if (!rules.setsLoading && !rules.busy) void rules.loadSets(true);
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
  <span class="muted">{tr(`共 ${rules.sets.length} 个规则集，已选 ${chosen.length} 个`, `${rules.sets.length} rule sets · ${chosen.length} selected`)}</span>
  <span class="btns">
    <button type="button" class="icon-btn" class:spin={rules.setsLoading} aria-label={tr('重新读取规则集状态', 'Refresh rule set status')} title={tr('重新读取状态（不下载）', 'Refresh local cache status')} aria-disabled={rules.setsLoading || rules.busy} onclick={reload}>
      <Icon name="refresh" />
    </button>
    <button type="button" class="btn sm" aria-disabled={rules.busy} onclick={() => void update(chosen)}>{updating ? tr('更新中…', 'Updating…') : tr('更新所选', 'Update Selected')}</button>
    <button type="button" class="btn sm" disabled={rules.sets.length === 0 || rules.sets.length > MAX_UPDATE} aria-disabled={rules.busy} onclick={() => void update(rules.sets.map((item) => item.id))}>{tr('全部更新', 'Update All')}</button>
  </span>
</div>

<div class="inline-note gap">
  <Icon name="info" />
  <span>{tr('更新只是把文件下载到本机；内核要在重新应用规则（改动任意一条规则）或重启之后才会加载新的内容。', 'Updating downloads rule sets locally; the core reloads them when rules are re-applied or the core restarts.')}</span>
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
            <th>{tr('状态', 'Status')}</th>
            <th class="r">{tr('大小', 'Size')}</th>
            <th>{tr('更新时间', 'Updated')}</th>
          </tr>
        </thead>
        <tbody>
          {#each rules.sets as item (item.id)}
            {@const status = (void i18n.locale, ruleSetStatus(item.status))}
            <tr data-set-id={item.id}>
              <td class="pick">
                <input type="checkbox" aria-label={tr(`选择「${labelOf(item.id, item.name)}」`, `Select "${labelOf(item.id, item.name)}"`)} checked={chosen.includes(item.id)} onchange={(event) => toggle(item.id, event.currentTarget.checked)} />
              </td>
              <td class="name" title={item.url}>{labelOf(item.id, item.name)}</td>
              <td>{#if refOf(item.id) === ''}<span class="muted">—</span>{:else}<span class="chip set">{refOf(item.id)}</span>{/if}</td>
              <td><span class={`pill state ${status.tone}`}>{status.label}</span></td>
              <td class="r num">{item.sizeBytes > 0 ? formatBytes(item.sizeBytes) : '—'}</td>
              <td class="num">{item.updatedAt > 0 ? formatDateTime(item.updatedAt) : '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

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
</style>

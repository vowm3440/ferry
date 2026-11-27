<script lang="ts">
  import CapGate from '../lib/components/CapGate.svelte';
  import ConfirmDialog from '../lib/components/ConfirmDialog.svelte';
  import Icon from '../lib/components/Icon.svelte';
  import Segmented from '../lib/components/Segmented.svelte';
  import { i18n, tr } from '../lib/i18n/index.svelte';
  import { CAP_CONNECTIONS } from '../lib/api';
  import { actionFilterOptions, type ActionFilter } from '../lib/conns-view';
  import { caps } from '../lib/stores/caps.svelte';
  import { conns } from '../lib/stores/conns.svelte';
  import { core } from '../lib/stores/core.svelte';
  import { toasts } from '../lib/stores/toasts.svelte';
  import ConnsView from './conns/ConnsView.svelte';

  let query = $state('');
  let action = $state<ActionFilter>('all');
  let confirmingCloseAll = $state(false);

  /** 内核在运行且已经读到列表，头部的操作才有对象。 */
  const live = $derived(core.running && conns.loaded);
  const filterOptions = $derived((void i18n.locale, actionFilterOptions()));

  function clearFilter(): void {
    query = '';
    action = 'all';
  }

  async function togglePause(): Promise<void> {
    if (conns.pausing) return;
    const result = await conns.setPaused(!conns.paused);
    if (!result.ok) toasts.error(result.message);
  }

  async function closeAll(): Promise<void> {
    if (conns.closingAll) return;
    const result = await conns.closeAll();
    confirmingCloseAll = false;
    if (result.ok) toasts.ok(tr('已关闭全部连接', 'Closed all connections'));
    else toasts.error(result.message);
  }
</script>

<section class="page" aria-labelledby="conns-title">
  <div class="page-head">
    <h1 id="conns-title">{tr('连接', 'Connections')}</h1>
    {#if caps.has(CAP_CONNECTIONS)}
      <div class="actions">
        <label class="input search">
          <Icon name="search" />
          <input type="search" placeholder={tr('过滤应用、主机、规则', 'Filter app, host, rule')} aria-label={tr('过滤连接', 'Filter connections')} bind:value={query} />
        </label>
        <Segmented
          options={filterOptions}
          value={action}
          label={tr('按出站方式筛选', 'Filter by outbound action')}
          onchange={(next: ActionFilter) => (action = next)}
        />
        <button
          type="button"
          class="btn"
          disabled={!live}
          aria-disabled={conns.pausing}
          aria-pressed={conns.paused}
          title={conns.paused ? tr('继续刷新连接列表', 'Resume connection updates') : tr('暂停刷新连接列表', 'Pause connection updates')}
          onclick={() => void togglePause()}
        >
          <Icon name={conns.paused ? 'play' : 'pause'} />{conns.paused ? tr('继续', 'Resume') : tr('暂停', 'Pause')}
        </button>
        <button
          type="button"
          class="btn danger"
          disabled={!live || conns.list.length === 0}
          onclick={() => (confirmingCloseAll = true)}
        >
          {tr('关闭全部', 'Close All')}
        </button>
      </div>
    {/if}
  </div>

  <CapGate cap={CAP_CONNECTIONS} feature={tr('连接', 'Connections')} icon="conns">
    <ConnsView {query} {action} onclearfilter={clearFilter} />
  </CapGate>
</section>

<ConfirmDialog
  open={confirmingCloseAll}
  title={tr('关闭全部连接', 'Close All Connections')}
  message={tr(
    `将断开当前的 ${conns.list.length} 个活动连接。正在进行的下载、通话会中断，多数应用会自动重新连接。`,
    `Disconnect all ${conns.list.length} active connections. Ongoing downloads or calls will drop; most apps reconnect automatically.`,
  )}
  confirmLabel={tr('关闭全部', 'Close All')}
  busyLabel={tr('正在关闭…', 'Closing…')}
  danger
  busy={conns.closingAll}
  onconfirm={() => void closeAll()}
  oncancel={() => (confirmingCloseAll = false)}
/>

<style>
  .search {
    flex: 1 1 200px;
    width: 220px;
    max-width: 260px;
  }
</style>

<!--
  连接页的内容区。放在 CapGate 里面：能渲染到这里，说明后端公布了 connections 能力。
  挂载期间才让 conns store 工作，离开页面就停。
-->
<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import EmptyState from '../../lib/components/EmptyState.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import { CAP_RULES, type Connection } from '../../lib/api';
  import { tr } from '../../lib/i18n/index.svelte';
  import {
    capRows,
    filterConnections,
    rateText,
    sortConnections,
    totalRates,
    type ActionFilter,
  } from '../../lib/conns-view';
  import { caps } from '../../lib/stores/caps.svelte';
  import { conns } from '../../lib/stores/conns.svelte';
  import { core } from '../../lib/stores/core.svelte';
  import { nav } from '../../lib/stores/nav.svelte';
  import { ruleDraft } from '../../lib/stores/rule-draft.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import ConnTable from './ConnTable.svelte';

  interface Props {
    query: string;
    action: ActionFilter;
    onclearfilter: () => void;
  }

  let { query, action, onclearfilter }: Props = $props();

  /** 正在为它预填规则的连接 id。 */
  let drafting = $state('');

  const filtering = $derived(query.trim() !== '' || action !== 'all');
  const matched = $derived(sortConnections(filterConnections(conns.list, query, action)));
  const capped = $derived(capRows(matched));
  const totals = $derived(totalRates(conns.rates));
  const coreBusy = $derived(core.pending !== null || core.transitioning);

  onMount(() => {
    conns.activate();
    return () => conns.deactivate();
  });

  // 内核启停之后重新读取一次：停止后后端不再推送，列表不能停在旧数据上。
  let seenRunning: boolean | null = null;
  $effect(() => {
    const running = core.running;
    untrack(() => {
      if (seenRunning !== null && seenRunning !== running) void conns.refresh();
      seenRunning = running;
    });
  });

  async function closeOne(conn: Connection): Promise<void> {
    if (conns.closing.has(conn.id)) return;
    const result = await conns.close(conn.id);
    if (!result.ok) toasts.error(result.message);
  }

  async function createRule(conn: Connection): Promise<void> {
    if (drafting !== '') return;
    drafting = conn.id;
    try {
      const result = await conns.ruleFromConnection(conn.id);
      if (!result.ok) {
        toasts.error(result.message);
        return;
      }
      ruleDraft.offer(result.rule);
      nav.go('rules');
    } finally {
      drafting = '';
    }
  }

  async function startCore(): Promise<void> {
    if (coreBusy) return;
    const result = await core.start();
    if (!result.ok) toasts.error(result.message);
  }
</script>

{#if !core.running}
  <div class="card">
    <EmptyState icon="conns" title={tr('内核未运行', 'Core Stopped')} description={tr('启动内核后，这里会实时显示经过 Ferry 的活动连接。', 'Start core to view real-time active connections routed through Ferry.')}>
      <button
        type="button"
        class="btn"
        disabled={core.state === null}
        aria-disabled={coreBusy}
        onclick={() => void startCore()}
      >
        {core.transitioning ? tr('处理中…', 'Working…') : tr('启动内核', 'Start Core')}
      </button>
    </EmptyState>
  </div>
{:else if !conns.loaded}
  <div class="card">
    {#if conns.error !== ''}
      <EmptyState icon="alert" tone="error" title={tr('无法读取连接', 'Failed to Load Connections')} description={conns.error}>
        <button type="button" class="btn" onclick={() => void conns.refresh()}>{tr('重试', 'Retry')}</button>
      </EmptyState>
    {:else}
      <EmptyState icon="conns" title={tr('正在读取连接…', 'Loading connections…')} description={tr('正在向桌面后端请求活动连接。', 'Requesting active connections from backend.')} />
    {/if}
  </div>
{:else}
  {#if conns.error !== ''}
    <div class="inline-error gap" role="alert">
      <Icon name="alert" />
      <span class="grow">{tr(`刷新连接失败：${conns.error}`, `Failed to refresh connections: ${conns.error}`)}</span>
      <button type="button" class="btn sm" onclick={() => void conns.refresh()}>{tr('重试', 'Retry')}</button>
    </div>
  {/if}

  <div class="summary" aria-live="off">
    <b class="num">{tr(`${conns.list.length} 个活动连接`, `${conns.list.length} active connections`)}</b>
    <span class="num" title={tr('全部连接的下载速率', 'Total download speed')}><span class="down" aria-hidden="true">↓</span><span class="sr-only">{tr('下载', 'Download')}</span> {rateText(totals?.down)}</span>
    <span class="num" title={tr('全部连接的上传速率', 'Total upload speed')}><span class="up" aria-hidden="true">↑</span><span class="sr-only">{tr('上传', 'Upload')}</span> {rateText(totals?.up)}</span>
    {#if filtering}
      <span class="num">{tr(`匹配 ${matched.length} 条`, `${matched.length} matched`)}</span>
    {/if}
    {#if conns.paused}
      <span class="pill warn">{tr('已暂停刷新', 'Paused')}</span>
    {/if}
  </div>

  {#if conns.list.length === 0}
    <div class="card">
      <EmptyState icon="conns" title={tr('当前没有活动连接', 'No Active Connections')} description={tr('有应用通过 Ferry 联网时，连接会出现在这里。', 'Active connections routed through Ferry will appear here.')} />
    </div>
  {:else if matched.length === 0}
    <div class="card">
      <EmptyState icon="search" title={tr('没有匹配的连接', 'No Matching Connections')} description={tr('换一个关键词，或清除筛选条件后再试。', 'Try another keyword or clear your filters.')}>
        <button type="button" class="btn" onclick={onclearfilter}>{tr('清除筛选', 'Clear Filters')}</button>
      </EmptyState>
    </div>
  {:else}
    <div class="card table-card">
      <ConnTable
        rows={capped.rows}
        rates={conns.rates}
        now={conns.now}
        closing={conns.closing}
        {drafting}
        canCreateRule={caps.has(CAP_RULES)}
        onclose={(conn: Connection) => void closeOne(conn)}
        oncreaterule={(conn: Connection) => void createRule(conn)}
      />
    </div>
    {#if capped.hidden > 0}
      <p class="cap-note num">
        {tr(
          `仅显示前 ${capped.rows.length} 条，另有 ${capped.hidden} 条未显示；可以用筛选缩小范围。`,
          `Showing first ${capped.rows.length} rows (${capped.hidden} hidden); narrow down using filters.`,
        )}
      </p>
    {/if}
  {/if}
{/if}

<style>
  .gap {
    align-items: center;
    margin-bottom: 12px;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .summary {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 18px;
    min-height: 20px;
    margin-bottom: 10px;
    font-size: 12px;
    color: var(--text-2);
  }

  .summary b {
    font-weight: 600;
  }

  .down {
    color: var(--accent-strong);
  }

  .up {
    color: var(--warn);
  }

  .table-card {
    padding: 4px 6px;
  }

  .cap-note {
    margin-top: 8px;
    font-size: 12px;
    color: var(--text-3);
  }
</style>

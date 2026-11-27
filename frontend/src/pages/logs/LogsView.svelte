<!--
  日志页的内容区。放在 CapGate 里面：能渲染到这里，说明后端公布了 logs 能力。
  挂载期间才让 logs store 工作，离开页面就停。内核停止后已有的日志仍然可以查看。
-->
<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import EmptyState from '../../lib/components/EmptyState.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { filterLogs, tailRows, type LevelFilter } from '../../lib/logs-view';
  import { logLevelOptions, optionsWith } from '../../lib/settings-view';
  import { core } from '../../lib/stores/core.svelte';
  import { logs } from '../../lib/stores/logs.svelte';
  import { settings } from '../../lib/stores/settings.svelte';
  import LogList from './LogList.svelte';

  interface Props {
    query: string;
    minLevel: LevelFilter;
    onclearfilter: () => void;
  }

  let { query, minLevel, onclearfilter }: Props = $props();

  /** 正在提交、还没有得到后端确认的级别。 */
  let pendingLevel = $state<string | null>(null);
  /** 修改内核日志级别失败的说明，原样来自后端。 */
  let levelError = $state('');

  const savedLevel = $derived(settings.value?.logLevel ?? '');
  const shownLevel = $derived(pendingLevel ?? savedLevel);
  const levelOptions = $derived((void i18n.locale, optionsWith(logLevelOptions(), shownLevel)));
  const filtering = $derived(query.trim() !== '' || minLevel !== 'debug');
  const matched = $derived(filterLogs(logs.entries, minLevel, query));
  const tail = $derived(tailRows(matched));

  onMount(() => {
    logs.activate();
    return () => logs.deactivate();
  });

  // 内核启停之后补读一次，启动过程中的日志不靠事件也能看到。
  let seenRunning: boolean | null = null;
  $effect(() => {
    const running = core.running;
    untrack(() => {
      if (seenRunning !== null && seenRunning !== running && !logs.paused) void logs.refresh();
      seenRunning = running;
    });
  });

  async function changeLevel(event: Event): Promise<void> {
    const select = event.currentTarget as HTMLSelectElement;
    const next = select.value;
    if (logs.levelSaving || next === savedLevel) {
      select.value = shownLevel;
      return;
    }
    pendingLevel = next;
    levelError = '';
    const result = await logs.setLevel(next);
    pendingLevel = null;
    if (!result.ok) levelError = result.message;
    // 失败时下拉框回到后端实际的级别。
    select.value = settings.value?.logLevel ?? '';
  }
</script>

<div class="bar-row">
  <label class="level">
    <span class="muted">{tr('内核日志级别', 'Core Log Level')}</span>
    <select
      class="select"
      aria-label={tr('内核日志级别', 'Core Log Level')}
      value={shownLevel}
      disabled={settings.value === null || logs.levelSaving}
      onchange={(event) => void changeLevel(event)}
    >
      {#each levelOptions as option (option.value)}
        <option value={option.value}>{option.label}</option>
      {/each}
    </select>
  </label>
  <span class="muted hint">{tr('决定内核记录哪些日志，会保存到设置；右上角的级别只筛选已经收到的内容。', 'Controls what the core records and saves to Settings; the top-right filter only filters received logs.')}</span>
  <span class="count num" aria-live="off">
    {#if filtering}{tr(`匹配 ${matched.length} / ${logs.entries.length} 条`, `${matched.length} / ${logs.entries.length} matched`)}{:else}{tr(`共 ${logs.entries.length} 条`, `${logs.entries.length} entries`)}{/if}
    {#if logs.paused}<span class="pill warn">{tr('已暂停刷新', 'Paused')}</span>{/if}
  </span>
</div>

{#if levelError !== ''}
  <div class="inline-error gap" role="alert">
    <Icon name="alert" />
    <span class="grow selectable">{tr(`修改日志级别失败：${levelError}`, `Failed to update log level: ${levelError}`)}</span>
    <button type="button" class="btn sm" onclick={() => (levelError = '')}>{tr('知道了', 'Dismiss')}</button>
  </div>
{/if}

{#if !logs.loaded}
  <div class="card">
    <EmptyState icon="logs" title={tr('正在读取日志…', 'Loading logs…')} description={tr('正在向桌面后端请求最近的日志。', 'Requesting recent logs from backend.')} />
  </div>
{:else}
  {#if logs.error !== ''}
    <div class="inline-error gap" role="alert">
      <Icon name="alert" />
      <span class="grow selectable">{tr(`读取日志失败：${logs.error}`, `Failed to load logs: ${logs.error}`)}</span>
      <button type="button" class="btn sm" onclick={() => void logs.refresh()}>{tr('重试', 'Retry')}</button>
    </div>
  {/if}

  {#if logs.entries.length === 0}
    <div class="card">
      <EmptyState
        icon="logs"
        title={tr('暂无日志', 'No Logs Yet')}
        description={core.running ? tr('内核产生新的日志后会显示在这里。', 'New logs from the core will appear here.') : tr('内核未运行。启动内核后，日志会显示在这里。', 'Core stopped. Start core to see live logs here.')}
      />
    </div>
  {:else if matched.length === 0}
    <div class="card">
      <EmptyState icon="search" title={tr('没有匹配的日志', 'No Matching Logs')} description={tr('换一个关键词，或调低显示级别后再试。', 'Try another keyword or lower the minimum display level.')}>
        <button type="button" class="btn" onclick={onclearfilter}>{tr('清除筛选', 'Clear Filters')}</button>
      </EmptyState>
    </div>
  {:else}
    <LogList rows={tail.rows} hidden={tail.hidden} />
  {/if}
{/if}

<style>
  .bar-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px 12px;
    margin-bottom: 10px;
    font-size: 12px;
  }

  .level {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12.5px;
  }

  .hint {
    flex: 1 1 220px;
    min-width: 0;
  }

  .count {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    margin-left: auto;
    color: var(--text-2);
  }

  .gap {
    align-items: center;
    margin-bottom: 12px;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }
</style>

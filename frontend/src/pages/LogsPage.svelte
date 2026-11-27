<script lang="ts">
  import CapGate from '../lib/components/CapGate.svelte';
  import ConfirmDialog from '../lib/components/ConfirmDialog.svelte';
  import Icon from '../lib/components/Icon.svelte';
  import Segmented from '../lib/components/Segmented.svelte';
  import { tr } from '../lib/i18n/index.svelte';
  import { CAP_LOGS, hasFileDialogs, messageOfError, pickSaveFile } from '../lib/api';
  import { LEVEL_FILTERS, exportFileName, type LevelFilter } from '../lib/logs-view';
  import { caps } from '../lib/stores/caps.svelte';
  import { logs } from '../lib/stores/logs.svelte';
  import { toasts } from '../lib/stores/toasts.svelte';
  import LogsView from './logs/LogsView.svelte';

  let query = $state('');
  let minLevel = $state<LevelFilter>('debug');
  let confirmingExport = $state(false);
  /** 正在等用户在系统的保存对话框里选位置。 */
  let picking = $state(false);
  /** 导出失败的说明，原样来自后端；留在确认框里，方便换一个文件重试。 */
  let exportError = $state('');

  /** 导出要用系统的保存对话框，只有桌面版才有。 */
  const canExport = hasFileDialogs();
  const exportBusy = $derived(picking || logs.exporting);

  function clearFilter(): void {
    query = '';
    minLevel = 'debug';
  }

  async function togglePause(): Promise<void> {
    if (logs.pausing) return;
    const result = await logs.setPaused(!logs.paused);
    if (!result.ok) toasts.error(result.message);
  }

  function openExport(): void {
    exportError = '';
    confirmingExport = true;
  }

  function cancelExport(): void {
    if (exportBusy) return;
    confirmingExport = false;
    exportError = '';
  }

  /** 用户确认明文导出之后：选位置，再交给后端写文件。 */
  async function runExport(): Promise<void> {
    if (exportBusy) return;
    exportError = '';
    let path: string | null = null;
    picking = true;
    try {
      path = await pickSaveFile({
        title: tr('导出日志', 'Export Logs'),
        filename: exportFileName(Date.now()),
        kinds: [{ name: tr('日志文件', 'Log File'), pattern: '*.log' }],
      });
    } catch (error: unknown) {
      exportError = messageOfError(error);
      return;
    } finally {
      picking = false;
    }
    // 没有选位置就是取消，什么都不写。
    if (path === null) return;
    const result = await logs.exportTo(path);
    if (!result.ok) {
      exportError = result.message;
      return;
    }
    confirmingExport = false;
    toasts.ok(tr('日志已导出', 'Logs exported'));
  }
</script>

<section class="page" aria-labelledby="logs-title">
  <div class="page-head">
    <h1 id="logs-title">{tr('日志', 'Logs')}</h1>
    {#if caps.has(CAP_LOGS)}
      <div class="actions">
        <label class="input search">
          <Icon name="search" />
          <input type="search" placeholder={tr('搜索日志', 'Search logs')} aria-label={tr('搜索日志', 'Search logs')} bind:value={query} />
        </label>
        <Segmented
          options={LEVEL_FILTERS}
          value={minLevel}
          label={tr('最低显示级别', 'Minimum display level')}
          onchange={(next: LevelFilter) => (minLevel = next)}
        />
        <button
          type="button"
          class="btn"
          disabled={!logs.loaded}
          aria-disabled={logs.pausing}
          aria-pressed={logs.paused}
          title={logs.paused ? tr('继续刷新日志', 'Resume log stream') : tr('暂停刷新日志', 'Pause log stream')}
          onclick={() => void togglePause()}
        >
          <Icon name={logs.paused ? 'play' : 'pause'} />{logs.paused ? tr('继续', 'Resume') : tr('暂停', 'Pause')}
        </button>
        <button
          type="button"
          class="btn"
          disabled={logs.entries.length === 0}
          title={tr('只清空这里的显示，不影响内核的日志', 'Clear view only; backend log buffer is unaffected')}
          onclick={() => logs.clear()}
        >
          <Icon name="trash" />{tr('清空显示', 'Clear View')}
        </button>
        <button
          type="button"
          class="btn"
          disabled={!canExport || !logs.loaded}
          title={canExport ? tr('把日志明文导出到文件', 'Export plain-text logs to a file') : tr('导出日志只在桌面版中可用', 'Log export is only available in the desktop app')}
          onclick={openExport}
        >
          <Icon name="export" />{tr('导出', 'Export')}
        </button>
      </div>
    {/if}
  </div>

  <CapGate cap={CAP_LOGS} feature={tr('日志', 'Logs')} icon="logs">
    <LogsView {query} {minLevel} onclearfilter={clearFilter} />
  </CapGate>
</section>

<ConfirmDialog
  open={confirmingExport}
  title={tr('导出日志', 'Export Logs')}
  message={tr(
    '日志将以明文写入你选择的文件，其中可能包含访问过的域名、IP 地址和进程名。\n请只保存到可信的位置，分享之前先检查内容。',
    'Logs will be saved in plain text and may contain visited domains, IP addresses, and process names.\nSave only to a trusted location and review before sharing.',
  )}
  confirmLabel={tr('选择位置并导出', 'Choose Location & Export')}
  busyLabel={picking ? tr('等待选择位置…', 'Selecting file…') : tr('正在导出…', 'Exporting…')}
  busy={exportBusy}
  onconfirm={() => void runExport()}
  oncancel={cancelExport}
>
  {#if exportError !== ''}
    <div class="inline-error export-error" role="alert">
      <Icon name="alert" />
      <span class="selectable">{exportError}</span>
    </div>
  {/if}
</ConfirmDialog>

<style>
  .search {
    flex: 1 1 180px;
    width: 200px;
    max-width: 240px;
  }

  .export-error {
    margin-top: 12px;
  }
</style>

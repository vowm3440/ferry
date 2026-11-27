<script lang="ts">
  import { CAP_BACKUP, CAP_DIAGNOSTICS, CAP_SYSTEM_TOOLS, hasFileDialogs, messageOfError, pickOpenFile, pickSaveFile } from '../../lib/api';
  import ConfirmDialog from '../../lib/components/ConfirmDialog.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { caps } from '../../lib/stores/caps.svelte';
  import { connection } from '../../lib/stores/connection.svelte';
  import { core } from '../../lib/stores/core.svelte';
  import { system } from '../../lib/stores/system.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import {
    backupFilename,
    backupKinds,
    diagnosticsContains,
    diagnosticsExcludes,
    diagnosticsFilename,
    diagnosticsKinds,
    fileActionBlock,
    restoreReplaces,
  } from '../../lib/system-view';
  import { reloadAll } from './reload-all';
  import SettingRow from './SettingRow.svelte';

  type Action = 'backup' | 'restore' | 'diagnostics';

  const dialogs = hasFileDialogs();

  /** 各行最近一次失败的说明（后端原文），下次操作开始时清掉。 */
  let errors = $state.raw<Readonly<Record<Action, string>>>({ backup: '', restore: '', diagnostics: '' });
  /** 已选好、等待确认的备份文件；非 null 时显示确认框。 */
  let restorePath = $state<string | null>(null);
  let diagnosticsOpen = $state(false);
  /** 系统的文件选择框正开着。 */
  let picking = $state(false);

  const backupBlock = $derived((void i18n.locale, fileActionBlock({ capsLoaded: caps.loaded, capable: caps.has(CAP_BACKUP), dialogs })));
  const diagnosticsBlock = $derived((void i18n.locale, fileActionBlock({ capsLoaded: caps.loaded, capable: caps.has(CAP_DIAGNOSTICS), dialogs })));
  const replacesList = $derived((void i18n.locale, restoreReplaces()));
  const diagContains = $derived((void i18n.locale, diagnosticsContains()));
  const diagExcludes = $derived((void i18n.locale, diagnosticsExcludes()));
  const canTools = $derived(caps.has(CAP_SYSTEM_TOOLS));
  const busy = $derived(picking || system.task !== '');

  async function openDir(kind: 'config' | 'log'): Promise<void> {
    const backend = connection.backend;
    if (backend === null) return;
    try {
      await (kind === 'config' ? backend.openConfigDir() : backend.openLogDir());
    } catch (cause: unknown) {
      toasts.error(messageOfError(cause));
    }
  }

  function setError(action: Action, message: string): void {
    errors = { ...errors, [action]: message };
  }

  /** 打开系统的文件选择框；取消返回 null，出错时把说明记到对应的行上。 */
  async function pick(action: Action, open: () => Promise<string | null>): Promise<string | null> {
    picking = true;
    try {
      return await open();
    } catch (error: unknown) {
      setError(action, messageOfError(error));
      return null;
    } finally {
      picking = false;
    }
  }

  async function exportBackup(): Promise<void> {
    if (busy) return;
    setError('backup', '');
    const path = await pick('backup', () =>
      pickSaveFile({ title: tr('导出备份', 'Export Backup'), filename: backupFilename(Date.now()), kinds: [...backupKinds()] }),
    );
    if (path === null) return;
    const result = await system.exportBackup(path);
    if (result.ok) toasts.ok(tr(`备份已导出：${path}`, `Backup exported: ${path}`));
    else setError('backup', result.message);
  }

  async function chooseRestore(): Promise<void> {
    if (busy) return;
    setError('restore', '');
    restorePath = await pick('restore', () => pickOpenFile({ title: tr('选择备份文件', 'Select Backup File'), kinds: [...backupKinds()] }));
  }

  async function restore(): Promise<void> {
    const path = restorePath;
    if (path === null || system.task !== '') return;
    const result = await system.restoreBackup(path);
    restorePath = null;
    // 失败时后端也可能已经改动了一部分状态，一律重新读取，界面以后端为准。
    await reloadAll();
    if (result.ok) toasts.ok(tr('已从备份恢复', 'Restored from backup'));
    else setError('restore', result.message);
  }

  async function exportDiagnostics(): Promise<void> {
    diagnosticsOpen = false;
    if (busy) return;
    setError('diagnostics', '');
    const path = await pick('diagnostics', () =>
      pickSaveFile({
        title: tr('导出诊断包', 'Export Diagnostics'),
        filename: diagnosticsFilename(Date.now()),
        kinds: [...diagnosticsKinds()],
      }),
    );
    if (path === null) return;
    const result = await system.exportDiagnostics(path);
    if (result.ok) toasts.ok(tr(`诊断包已导出：${path}`, `Diagnostics exported: ${path}`));
    else setError('diagnostics', result.message);
  }
</script>

<SettingRow
  title={tr('导出备份', 'Export Backup')}
  desc={tr(
    '把全部节点、订阅、规则和设置保存为一个文件。文件用 Windows 数据保护（DPAPI）加密，只能在这台电脑的当前 Windows 账户下恢复；需要保存到一个尚不存在的文件。',
    'Save all nodes, subscriptions, rules, and settings into one DPAPI-encrypted file bound to your current Windows account. Must save to a new file.',
  )}
  tag={backupBlock}
  error={errors.backup}
>
  <button type="button" class="btn" data-action="backup-export" disabled={backupBlock !== ''} aria-disabled={busy} onclick={() => void exportBackup()}>
    {system.task === 'backup' ? tr('正在导出…', 'Exporting…') : tr('导出备份…', 'Export Backup…')}
  </button>
</SettingRow>
<SettingRow
  title={tr('从备份恢复', 'Restore from Backup')}
  desc={tr(
    '用备份文件替换当前的全部节点、订阅、规则和设置。只能恢复由当前 Windows 账户导出的备份。',
    'Replace all current nodes, subscriptions, rules, and settings from a backup exported by this Windows account.',
  )}
  tag={backupBlock}
  error={errors.restore}
>
  <button type="button" class="btn" data-action="backup-restore" disabled={backupBlock !== ''} aria-disabled={busy} onclick={() => void chooseRestore()}>
    {system.task === 'restore' ? tr('正在恢复…', 'Restoring…') : tr('从备份恢复…', 'Restore Backup…')}
  </button>
</SettingRow>
<SettingRow
  title={tr('导出诊断包', 'Export Diagnostics')}
  desc={tr(
    '反馈问题时使用。只包含版本、运行状态和数量统计，不含节点、地址和日志。',
    'For bug reports. Includes only versions, runtime status, and item counts; never nodes, addresses, or logs.',
  )}
  tag={diagnosticsBlock}
  error={errors.diagnostics}
>
  <button
    type="button"
    class="btn"
    data-action="diagnostics-export"
    disabled={diagnosticsBlock !== ''}
    aria-disabled={busy}
    onclick={() => {
      if (!busy) diagnosticsOpen = true;
    }}
  >
    {system.task === 'diagnostics' ? tr('正在导出…', 'Exporting…') : tr('导出诊断包…', 'Export Diagnostics…')}
  </button>
</SettingRow>
{#if canTools}
  <SettingRow title={tr('本地数据目录', 'Local Data Folders')} desc={tr('在资源管理器中打开配置或日志目录', 'Open config or log folder in Explorer')}>
    <div class="dir-actions">
      <button type="button" class="btn sm" onclick={() => void openDir('config')}>{tr('配置目录', 'Config Dir')}</button>
      <button type="button" class="btn sm" onclick={() => void openDir('log')}>{tr('日志目录', 'Log Dir')}</button>
    </div>
  </SettingRow>
{/if}

<ConfirmDialog
  open={restorePath !== null}
  title={tr('从备份恢复', 'Restore from Backup')}
  confirmLabel={tr('替换并恢复', 'Replace & Restore')}
  busyLabel={tr('正在恢复…', 'Restoring…')}
  danger
  busy={system.task === 'restore'}
  onconfirm={() => void restore()}
  oncancel={() => (restorePath = null)}
>
  <p class="lead">{tr('恢复会用备份里的内容替换当前的：', 'Restoring will overwrite your current:')}</p>
  <ul class="list">
    {#each replacesList as item (item)}
      <li>{item}</li>
    {/each}
  </ul>
  <p class="lead">
    {core.running
      ? tr('当前的内容不会保留，此操作无法撤销。内核正在运行，恢复后会自动重启，现有连接会中断。', 'Current data will not be kept and this cannot be undone. Running core will restart and drop active connections.')
      : tr('当前的内容不会保留，此操作无法撤销。', 'Current data will not be kept and this cannot be undone.')}
  </p>
  <p class="path mono selectable">{restorePath ?? ''}</p>
</ConfirmDialog>

<ConfirmDialog
  open={diagnosticsOpen}
  title={tr('导出诊断包', 'Export Diagnostics')}
  confirmLabel={tr('选择保存位置', 'Choose Save Location')}
  onconfirm={() => void exportDiagnostics()}
  oncancel={() => (diagnosticsOpen = false)}
>
  <p class="lead">{tr('诊断包是一个未加密的 JSON 文件，只包含：', 'Diagnostics bundle is an unencrypted JSON file containing only:')}</p>
  <ul class="list">
    {#each diagContains as item (item)}
      <li>{item}</li>
    {/each}
  </ul>
  <p class="lead">{diagExcludes}</p>
</ConfirmDialog>

<style>
  .lead {
    margin: 0;
    color: var(--text-2);
    line-height: 1.6;
  }

  .list {
    margin: 6px 0 10px;
    padding-left: 20px;
    color: var(--text);
    line-height: 1.7;
  }

  .path {
    margin: 10px 0 0;
    padding: 8px 10px;
    border-radius: var(--r);
    background: var(--surface-2);
    border: 1px solid var(--border);
    font-size: 12px;
    overflow-wrap: anywhere;
  }

  .dir-actions {
    display: flex;
    gap: 8px;
  }
</style>

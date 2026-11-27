<script lang="ts">
  import { tick } from 'svelte';
  import { CAP_QR, CAP_QR_SCREEN, CAP_SUBS_URL, hasFileDialogs, messageOfError, pickOpenFile, type Subscription } from '../lib/api';
  import ConfirmDialog from '../lib/components/ConfirmDialog.svelte';
  import EmptyState from '../lib/components/EmptyState.svelte';
  import Icon from '../lib/components/Icon.svelte';
  import { pickLocalFile, readFileBase64 } from '../lib/file-pick';
  import { i18n, tr } from '../lib/i18n/index.svelte';
  import { caps } from '../lib/stores/caps.svelte';
  import { core } from '../lib/stores/core.svelte';
  import { nav } from '../lib/stores/nav.svelte';
  import { subs, type ImportOutcome } from '../lib/stores/subs.svelte';
  import { toasts } from '../lib/stores/toasts.svelte';
  import { QR_IMAGE_MAX_BYTES, canUpdate, describeSkipped, type ImportKind } from '../lib/subs-view';
  import ImportTiles from './subs/ImportTiles.svelte';
  import PasteImport from './subs/PasteImport.svelte';
  import SubCard from './subs/SubCard.svelte';
  import SubEditDialog from './subs/SubEditDialog.svelte';
  import UrlImportDialog from './subs/UrlImportDialog.svelte';

  const NOW_TICK_MS = 30_000;
  const fileKinds = $derived([
    { name: tr('节点或配置文件', 'Node or Config Files'), pattern: '*.yaml;*.yml;*.json;*.txt;*.conf' },
    { name: tr('所有文件', 'All Files'), pattern: '*.*' },
  ]);

  let now = $state(Date.now());
  let urlOpen = $state(false);
  let screenQrConfirm = $state(false);
  let editId = $state('');
  let pending = $state<Subscription | null>(null);
  let busyKind = $state<ImportKind | null>(null);
  /** 链接 / 二维码 / 文件导入的结果（粘贴导入的结果显示在粘贴卡片里）。 */
  let outcome = $state<Subscription | null>(null);
  let error = $state('');
  let paste = $state<PasteImport | null>(null);
  let listEl = $state<HTMLUListElement | null>(null);
  let refreshEl = $state<HTMLButtonElement | null>(null);

  const locked = $derived(core.running || core.transitioning);
  const unavailable = $derived(locked || core.state === null);
  const canUrl = $derived(caps.has(CAP_SUBS_URL));
  const kinds = $derived<ImportKind[]>([
    ...(canUrl ? (['url'] as const) : []),
    'paste',
    ...(caps.has(CAP_QR) ? (['qr'] as const) : []),
    ...(caps.has(CAP_QR_SCREEN) ? (['screen-qr'] as const) : []),
    ...(canUrl && hasFileDialogs() ? (['file'] as const) : []),
  ]);
  const editTarget = $derived(subs.list.find((item) => item.id === editId) ?? null);
  const anyUpdatable = $derived(canUrl && subs.list.some(canUpdate));

  // 「几分钟前」与到期提醒要跟着时间走。
  $effect(() => {
    const timer = setInterval(() => (now = Date.now()), NOW_TICK_MS);
    return () => clearInterval(timer);
  });

  function report(result: ImportOutcome): void {
    if (result.ok) outcome = result.subscription;
    else error = result.message;
  }

  async function importQr(): Promise<void> {
    const file = await pickLocalFile('image/png,image/jpeg');
    if (file === null) return;
    // 图片读进内存后直接交给后端识别，界面不保留。
    report(await subs.importQr(await readFileBase64(file, QR_IMAGE_MAX_BYTES)));
  }

  async function importFile(): Promise<void> {
    const path = await pickOpenFile({ title: tr('选择节点或配置文件', 'Select Node or Config File'), kinds: fileKinds });
    if (path === null) return;
    report(await subs.importFile(path));
  }

  async function pick(kind: ImportKind): Promise<void> {
    if (kind === 'paste') return paste?.focus();
    if (busyKind !== null) return;
    error = '';
    outcome = null;
    if (kind === 'url') {
      urlOpen = true;
      return;
    }
    if (kind === 'screen-qr') {
      screenQrConfirm = true;
      return;
    }
    busyKind = kind;
    try {
      await (kind === 'qr' ? importQr() : importFile());
    } catch (cause: unknown) {
      error = messageOfError(cause);
    } finally {
      busyKind = null;
    }
  }

  async function confirmScreenQr(): Promise<void> {
    screenQrConfirm = false;
    if (busyKind !== null) return;
    error = '';
    outcome = null;
    busyKind = 'screen-qr';
    try {
      report(await subs.captureScreenQr());
    } catch (cause: unknown) {
      error = messageOfError(cause);
    } finally {
      busyKind = null;
    }
  }

  function onUrlDone(subscription: Subscription): void {
    urlOpen = false;
    outcome = subscription;
  }

  async function updateAll(): Promise<void> {
    if (subs.updatingAll) return;
    error = '';
    const result = await subs.updateAll();
    if (result.ok) toasts.ok(tr('全部订阅已更新', 'All subscriptions updated'));
    else error = result.message;
  }

  async function remove(): Promise<void> {
    const target = pending;
    if (target === null || subs.removing !== '') return;
    const result = await subs.remove(target.id);
    pending = null;
    if (!result.ok) {
      toasts.error(result.message);
      return;
    }
    toasts.ok(tr(`已删除订阅「${target.name}」`, `Removed subscription "${target.name}"`));
    // 被删的那一行连同它的按钮都没了：把焦点交给相邻的删除按钮，没有就交给刷新按钮。
    await tick();
    (listEl?.querySelector<HTMLElement>('button.danger') ?? refreshEl)?.focus();
  }

  function refresh(): void {
    if (!subs.loading) void subs.refresh();
  }
</script>

<section class="page" aria-labelledby="subs-title">
  <div class="page-head">
    <h1 id="subs-title">{tr('订阅', 'Subscriptions')}</h1>
    <span class="sub">{tr('支持 Clash / sing-box / Base64 / 分享链接 / SIP008', 'Supports Clash / sing-box / Base64 / Share URIs / SIP008')}</span>
    <div class="actions">
      {#if anyUpdatable}
        <button type="button" class="btn" aria-disabled={subs.updatingAll} onclick={() => void updateAll()}>
          <span class="ico" class:spin={subs.updatingAll}><Icon name="refresh" /></span>
          {subs.updatingAll ? tr('更新中…', 'Updating…') : tr('全部更新', 'Update All')}
        </button>
      {/if}
      <button
        type="button"
        class="icon-btn"
        class:spin={subs.loading}
        aria-disabled={subs.loading}
        aria-label={tr('刷新订阅列表', 'Refresh subscriptions')}
        title={tr('刷新订阅列表', 'Refresh subscriptions')}
        bind:this={refreshEl}
        onclick={refresh}
      >
        <Icon name="refresh" />
      </button>
    </div>
  </div>

  {#if kinds.length > 1}
    <ImportTiles {kinds} disabled={unavailable} busy={busyKind} onpick={(kind) => void pick(kind)} />
  {/if}

  {#if error !== ''}
    <div class="inline-error outcome" role="alert"><Icon name="alert" /><span class="grow pre">{error}</span></div>
  {/if}
  {#if outcome !== null}
    <div class="inline-note outcome" role="status">
      <Icon name="check" />
      <span class="grow">
        {outcome.skipped.length > 0
          ? tr(
              `已导入「${outcome.name}」：${outcome.nodeCount} 个节点，跳过 ${outcome.skipped.length} 行。`,
              `Imported "${outcome.name}": ${outcome.nodeCount} nodes (${outcome.skipped.length} skipped).`,
            )
          : tr(`已导入「${outcome.name}」：${outcome.nodeCount} 个节点。`, `Imported "${outcome.name}": ${outcome.nodeCount} nodes.`)}
        {#each outcome.warnings as warning, index (index)}<span class="line">{warning}</span>{/each}
        {#each outcome.skipped as item, index (index)}<span class="line">{(void i18n.locale, describeSkipped(item))}</span>{/each}
      </span>
      <button type="button" class="icon-btn" aria-label={tr('关闭提示', 'Dismiss')} title={tr('关闭提示', 'Dismiss')} onclick={() => (outcome = null)}>
        <Icon name="close" />
      </button>
    </div>
  {/if}

  <PasteImport bind:this={paste} {locked} {unavailable} />

  {#if subs.error !== ''}
    <div class="inline-error outcome" role="alert">
      <Icon name="alert" />
      <span class="grow">{tr(`刷新订阅失败：${subs.error}`, `Failed to refresh subscriptions: ${subs.error}`)}</span>
      <button type="button" class="btn sm" onclick={() => void subs.refresh()}>{tr('重试', 'Retry')}</button>
    </div>
  {/if}

  {#if !subs.loaded}
    <div class="card">
      <EmptyState icon="subs" title={tr('正在读取订阅…', 'Loading subscriptions…')} description={tr('正在向桌面后端请求订阅列表。', 'Requesting subscriptions from backend.')} />
    </div>
  {:else if subs.list.length === 0}
    <div class="card">
      <EmptyState
        icon="subs"
        title={tr('还没有订阅', 'No Subscriptions Yet')}
        description={kinds.length > 1 ? tr('从上面选一种方式导入节点。', 'Choose an import method above to add nodes.') : tr('粘贴上面的节点链接即可导入节点。', 'Paste node links above to import nodes.')}
      />
    </div>
  {:else}
    <ul class="list" bind:this={listEl}>
      {#each subs.list as item (item.id)}
        <SubCard sub={item} {now} onedit={(sub) => (editId = sub.id)} onremove={(sub) => (pending = sub)} />
      {/each}
    </ul>
    <div class="inline-note">
      <Icon name="info" />
      <span class="grow">{tr('删除订阅会同时移除它导入的节点；节点列表在「节点」页查看。', 'Removing a subscription also deletes its imported nodes; view all nodes in Proxies.')}</span>
      <button type="button" class="btn sm" onclick={() => nav.go('nodes')}>{tr('前往节点', 'Proxies')}</button>
    </div>
  {/if}
</section>

<UrlImportDialog open={urlOpen} onclose={() => (urlOpen = false)} ondone={onUrlDone} />
<SubEditDialog target={editTarget} onclose={() => (editId = '')} />
<ConfirmDialog
  open={screenQrConfirm}
  title={tr('屏幕二维码识别', 'Scan Screen QR Code')}
  message={tr(
    '即将截取当前屏幕画面以识别二维码，图像仅在内存中读取且不会保存或上传。确认继续？',
    'Ferry will capture your current screen in memory to decode a QR code. Nothing is saved or uploaded. Continue?',
  )}
  confirmLabel={tr('开始识别', 'Capture & Scan')}
  busyLabel={tr('识别中…', 'Scanning…')}
  busy={busyKind === 'screen-qr'}
  onconfirm={() => void confirmScreenQr()}
  oncancel={() => (screenQrConfirm = false)}
/>
<ConfirmDialog
  open={pending !== null}
  title={tr('删除订阅', 'Remove Subscription')}
  message={pending === null
    ? ''
    : tr(
        `删除「${pending.name}」会同时移除它导入的 ${pending.nodeCount} 个节点，无法撤销。`,
        `Removing "${pending.name}" will also delete its ${pending.nodeCount} imported nodes. This cannot be undone.`,
      )}
  confirmLabel={tr('删除', 'Remove')}
  busyLabel={tr('删除中…', 'Removing…')}
  danger
  busy={subs.removing !== ''}
  onconfirm={() => void remove()}
  oncancel={() => (pending = null)}
/>

<style>
  .ico {
    display: grid;
  }

  .outcome {
    align-items: center;
    margin-bottom: 14px;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .pre {
    white-space: pre-line;
  }

  .line {
    display: block;
    user-select: text;
  }

  .list {
    display: grid;
    gap: 10px;
    margin: 0 0 12px;
    padding: 0;
    list-style: none;
  }
</style>

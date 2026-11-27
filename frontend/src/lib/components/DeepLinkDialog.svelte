<!-- 深链导入确认对话框：显示协议、来源域名与明文警告，用户明确确认后才导入。 -->
<script lang="ts">
  import { tr } from '../i18n/index.svelte';
  import { deepLink } from '../stores/deeplink.svelte';
  import { nav } from '../stores/nav.svelte';
  import { subs } from '../stores/subs.svelte';
  import { toasts } from '../stores/toasts.svelte';
  import ConfirmDialog from './ConfirmDialog.svelte';

  let busy = $state(false);
  const item = $derived(deepLink.pending);

  async function confirm(): Promise<void> {
    const current = item;
    if (current === null || busy) return;
    busy = true;
    try {
      const result =
        current.kind === 'url'
          ? await subs.importUrl(current.name, current.payload, true)
          : await subs.import(current.name || tr('深链导入', 'Deep Link Import'), current.payload);
      deepLink.dismiss();
      if (!result.ok) {
        toasts.error(result.message);
        return;
      }
      toasts.ok(tr(`已导入「${result.subscription.name}」`, `Imported "${result.subscription.name}"`));
      nav.go('subs');
    } finally {
      busy = false;
    }
  }
</script>

<ConfirmDialog
  open={item !== null}
  title={tr('确认导入外部深链', 'Confirm External Deep Link')}
  confirmLabel={tr('确认导入', 'Import')}
  busyLabel={tr('导入中…', 'Importing…')}
  {busy}
  onconfirm={() => void confirm()}
  oncancel={() => deepLink.dismiss()}
>
  {#if item !== null}
    <p class="lead">
      {tr(
        '收到外部链接导入请求。请核对来源后再确认，避免导入未知节点的配置。',
        'Received an external link import request. Verify the source before importing.',
      )}
    </p>
    <dl class="meta">
      <div><dt>{tr('协议', 'Scheme')}</dt><dd class="mono">{item.scheme}://</dd></div>
      <div><dt>{tr('来源域名', 'Source Host')}</dt><dd class="mono">{item.host || '—'}</dd></div>
      {#if item.urlMasked}
        <div><dt>{tr('目标地址', 'Target URL')}</dt><dd class="mono">{item.urlMasked}</dd></div>
      {/if}
      {#if item.name}
        <div><dt>{tr('名称', 'Name')}</dt><dd>{item.name}</dd></div>
      {/if}
    </dl>
    {#if item.warning}
      <p class="warn">{item.warning}</p>
    {/if}
  {/if}
</ConfirmDialog>

<style>
  .lead {
    margin: 0 0 10px;
    color: var(--text-2);
    line-height: 1.6;
  }

  .meta {
    margin: 0;
    display: grid;
    gap: 6px;
    padding: 10px 12px;
    border-radius: var(--r);
    background: var(--surface-2);
    border: 1px solid var(--border);
    font-size: 12.5px;
  }

  .meta div {
    display: flex;
    gap: 10px;
  }

  .meta dt {
    color: var(--text-3);
    min-width: 72px;
  }

  .meta dd {
    margin: 0;
    color: var(--text);
    overflow-wrap: anywhere;
  }

  .warn {
    margin: 10px 0 0;
    padding: 8px 10px;
    border-radius: var(--r);
    color: var(--warn);
    background: var(--warn-soft);
    font-size: 12px;
  }
</style>

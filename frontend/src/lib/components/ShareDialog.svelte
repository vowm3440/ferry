<!--
  分享对话框（全局唯一，挂在 App 里）：由 shareRequest 决定分享哪个订阅或节点。
  导出的内容带着节点凭据：只放在这里的局部状态里，关闭即清空；不进 store、不写日志，
  只有用户点「复制」才会写入剪贴板。
-->
<script lang="ts">
  import { CAP_DEEPLINK, CAP_QR, CAP_SHARE, CAP_SHARE_LAN, copyText, messageOfError, type DeepLink, type ExportFormat } from '../api';
  import LanSharePanel from '../../pages/subs/LanSharePanel.svelte';
  import QrBox from '../../pages/subs/QrBox.svelte';
  import { i18n, tr } from '../i18n/index.svelte';
  import { caps } from '../stores/caps.svelte';
  import { connection } from '../stores/connection.svelte';
  import { shareRequest } from '../stores/share-request.svelte';
  import { toasts } from '../stores/toasts.svelte';
  import { shareFormatOptions } from '../subs-view';
  import Dialog from './Dialog.svelte';
  import Icon from './Icon.svelte';
  import Segmented from './Segmented.svelte';

  const target = $derived(shareRequest.target);
  const canExport = $derived(caps.has(CAP_SHARE));
  const canLan = $derived(caps.has(CAP_SHARE_LAN));
  const canDeepLink = $derived(caps.has(CAP_DEEPLINK));
  const formatOptions = $derived((void i18n.locale, shareFormatOptions()));

  let format = $state<ExportFormat>('clash');
  let text = $state('');
  let deepLinks = $state.raw<DeepLink[]>([]);
  let error = $state('');
  let loading = $state(false);
  let openedFor = '';

  // 每次打开：单个节点默认给链接，订阅默认给 Clash。局域网分享的状态由它自己的面板在挂载时读取。
  $effect(() => {
    const key = target === null ? '' : `${target.kind}:${target.id}`;
    if (key === openedFor) return;
    openedFor = key;
    if (target === null) return;
    format = target.kind === 'node' ? 'uri' : 'clash';
  });

  // 目标或格式变了就重新导出；关闭时清空。
  $effect(() => {
    const current = target;
    const backend = connection.backend;
    const wanted = format;
    text = '';
    error = '';
    loading = false;
    if (current === null || backend === null || !canExport) return;

    let stale = false;
    loading = true;
    const request = current.kind === 'sub' ? backend.subsExport(current.id, wanted) : backend.subsExportNode(current.id, wanted);
    request
      .then((result) => {
        if (!stale) {
          text = result;
          deepLinks = [];
          if (canDeepLink && wanted === 'uri' && result !== '' && !result.includes('\n')) {
            void backend
              .subsShareDeepLink(result)
              .then((links) => {
                if (!stale) deepLinks = links;
              })
              .catch(() => {
                if (!stale) deepLinks = [];
              });
          }
        }
      })
      .catch((cause: unknown) => {
        if (!stale) error = messageOfError(cause);
      })
      .finally(() => {
        if (!stale) loading = false;
      });
    return () => {
      stale = true;
      deepLinks = [];
    };
  });

  async function copyValue(value: string, okText: string): Promise<void> {
    if (value === '') return;
    try {
      await copyText(value);
      toasts.ok(okText);
    } catch (cause: unknown) {
      toasts.error(messageOfError(cause));
    }
  }

  function copy(): Promise<void> {
    return copyValue(text, tr('已复制到剪贴板', 'Copied to clipboard'));
  }
</script>

<Dialog
  open={target !== null}
  title={target === null ? tr('分享', 'Share') : tr(`分享「${target.name}」`, `Share "${target.name}"`)}
  width={560}
  onclose={() => shareRequest.close()}
>
  <div class="share">
    {#if canExport}
      <div class="fmt">
        <Segmented options={formatOptions} value={format} label={tr('导出格式', 'Export Format')} busy={loading} onchange={(next) => (format = next)} />
      </div>

      <div class="out">
        {#if caps.has(CAP_QR)}
          <QrBox payload={text} label={tr('分享内容的二维码', 'QR code for shared content')} />
        {/if}
        <div class="side">
          <textarea
            id="share-text"
            class="control mono"
            readonly
            rows="7"
            spellcheck="false"
            aria-label={tr('导出内容', 'Exported content')}
            placeholder={loading ? tr('正在导出…', 'Exporting…') : '—'}
            value={text}
          ></textarea>
          <div class="row">
            <span class="hint muted">{tr('内容里带有节点凭据，只发给你信任的人。', 'Contains node credentials; share only with people you trust.')}</span>
            <button type="button" class="btn sm primary" data-autofocus disabled={text === ''} onclick={() => void copy()}>
              <Icon name="copy" />{tr('复制', 'Copy')}
            </button>
          </div>
        </div>
      </div>

      {#if deepLinks.length > 0}
        <div class="deeplinks">
          <span class="muted">{tr('客户端深链：', 'Client Deep Links:')}</span>
          {#each deepLinks as link (link.scheme)}
            <button
              type="button"
              class="btn sm"
              onclick={() => void copyValue(link.url, tr(`已复制 ${link.label} 深链`, `Copied ${link.label} deep link`))}
            >
              <Icon name="link" />{link.label}
            </button>
          {/each}
        </div>
      {/if}

      {#if error !== ''}
        <div class="inline-error" role="alert"><Icon name="alert" /><span>{error}</span></div>
      {/if}
    {:else}
      <p class="muted">{tr('当前后端没有提供导出功能。', 'Export is not available on this backend.')}</p>
    {/if}

    {#if canLan && target !== null}
      <LanSharePanel />
    {/if}
  </div>
</Dialog>

<style>
  .share {
    display: grid;
    gap: 12px;
  }

  .fmt {
    display: flex;
  }

  .out {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-start;
    justify-content: center;
    gap: 12px;
  }

  .side {
    flex: 1;
    min-width: 220px;
    display: grid;
    gap: 8px;
  }

  .side textarea.control {
    min-height: 136px;
    font-size: 12px;
    resize: none;
    white-space: pre;
    overflow: auto;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .hint {
    flex: 1;
    min-width: 0;
    font-size: 12px;
  }

  .deeplinks {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    font-size: 12px;
  }
</style>

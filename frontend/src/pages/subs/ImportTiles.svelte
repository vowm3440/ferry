<!-- 导入入口：订阅链接、粘贴内容、二维码图片、本地文件。只渲染后端公布了能力的入口。 -->
<script lang="ts">
  import Icon from '../../lib/components/Icon.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import type { IconName } from '../../lib/icons';
  import type { ImportKind } from '../../lib/subs-view';

  interface Tile {
    readonly kind: ImportKind;
    readonly icon: IconName;
    readonly title: string;
    readonly hint: string;
  }

  interface Props {
    /** 可用的入口，按显示顺序。 */
    kinds: readonly ImportKind[];
    /** 内核运行中等原因暂时不能导入（粘贴入口不受影响，它只是把焦点移到文本框）。 */
    disabled: boolean;
    /** 正在处理的入口。 */
    busy: ImportKind | null;
    onpick: (kind: ImportKind) => void;
  }

  let { kinds, disabled, busy, onpick }: Props = $props();

  const visible = $derived.by<readonly Tile[]>(() => {
    const tiles: readonly Tile[] = [
      { kind: 'url', icon: 'link', title: tr('订阅链接', 'Subscription URL'), hint: tr('填写 URL', 'Enter URL') },
      { kind: 'paste', icon: 'clipboard', title: tr('粘贴内容', 'Paste Content'), hint: tr('链接或配置', 'URI or config') },
      { kind: 'qr', icon: 'qr', title: tr('二维码图片', 'QR Image'), hint: tr('选择 PNG / JPEG', 'Pick PNG / JPEG') },
      { kind: 'screen-qr', icon: 'qr', title: tr('屏幕二维码', 'Screen QR'), hint: tr('截取屏幕识别', 'Scan screen') },
      { kind: 'file', icon: 'file', title: tr('本地文件', 'Local File'), hint: 'yaml / json / txt' },
    ];
    return tiles.filter((tile) => kinds.includes(tile.kind));
  });

  function pick(kind: ImportKind): void {
    if (busy !== null) return;
    onpick(kind);
  }
</script>

<div class="tiles" role="group" aria-label={tr('导入方式', 'Import methods')}>
  {#each visible as tile (tile.kind)}
    <button
      type="button"
      class="imp"
      disabled={disabled && tile.kind !== 'paste'}
      aria-disabled={busy !== null}
      onclick={() => pick(tile.kind)}
    >
      <span class="ic" class:spin={busy === tile.kind}><Icon name={busy === tile.kind ? 'refresh' : tile.icon} size={16} /></span>
      <span class="txt">
        <b>{tile.title}</b>
        <span>{busy === tile.kind ? tr('处理中…', 'Working…') : tile.hint}</span>
      </span>
    </button>
  {/each}
</div>

<style>
  .tiles {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
    gap: 10px;
    margin-bottom: 14px;
  }

  .imp {
    display: flex;
    align-items: center;
    gap: 10px;
    min-width: 0;
    padding: 12px;
    border-radius: var(--r);
    border: 1px dashed var(--border-strong);
    background: transparent;
    color: var(--text-2);
    text-align: left;
    cursor: pointer;
    transition:
      border-color var(--dur),
      background var(--dur),
      color var(--dur);
  }

  .imp:hover:not(:disabled, [aria-disabled='true']) {
    border-color: var(--accent);
    background: var(--accent-soft);
    color: var(--text);
  }

  .imp:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .ic {
    flex: none;
    display: grid;
    place-items: center;
    width: 30px;
    height: 30px;
    border-radius: 8px;
    background: var(--surface-2);
  }

  .txt {
    display: grid;
    min-width: 0;
  }

  .txt b {
    color: var(--text);
    font-weight: 500;
  }

  .txt span {
    font-size: 11.5px;
    color: var(--text-3);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>

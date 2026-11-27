<script lang="ts">
  import { reserveBottom } from '../../lib/bottom-inset';
  import { tr } from '../../lib/i18n/index.svelte';

  interface Props {
    /** 保存失败时后端给出的说明；非空时整条变成错误样式。 */
    saveError: string;
    portInvalid: boolean;
    /** 内核运行中，后端拒绝修改设置。 */
    locked: boolean;
    /** 内核正在处理别的操作：「停止内核」暂不响应。 */
    coreBusy: boolean;
    stopping: boolean;
    canSave: boolean;
    saving: boolean;
    onStop: () => void;
    onRevert: () => void;
    onSave: () => void;
  }

  let { saveError, portInvalid, locked, coreBusy, stopping, canSave, saving, onStop, onRevert, onSave }: Props = $props();

  let saveButton = $state<HTMLButtonElement | null>(null);

  /** 「停止内核」按钮消失后，把键盘焦点交给「保存」。 */
  export function focusSave(): void {
    saveButton?.focus({ preventScroll: true });
  }
</script>

<div class="savebar" class:failed={saveError !== ''} use:reserveBottom>
  <div class="msg">
    {#if saveError !== ''}
      <span class="selectable" role="alert">{saveError}</span>
    {:else if portInvalid}
      <span>{tr('端口无效，修正后才能保存。', 'Invalid port; fix it before saving.')}</span>
    {:else if locked}
      <span>{tr('内核运行中：需要先停止内核，才能修改设置。', 'Core is running: stop core before saving settings.')}</span>
    {:else}
      <span>{tr('有未保存的修改。', 'You have unsaved changes.')}</span>
    {/if}
  </div>
  {#if locked}
    <button type="button" class="btn" aria-disabled={coreBusy} onclick={onStop}>
      {stopping ? tr('停止中…', 'Stopping…') : tr('停止内核', 'Stop Core')}
    </button>
  {/if}
  <button type="button" class="btn ghost" disabled={saving} onclick={onRevert}>{tr('还原', 'Revert')}</button>
  <button type="button" class="btn primary" bind:this={saveButton} disabled={!canSave} aria-disabled={saving} onclick={onSave}>
    {saving ? tr('保存中…', 'Saving…') : tr('保存', 'Save')}
  </button>
</div>

<style>
  .savebar {
    position: sticky;
    bottom: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    padding: 10px 12px 10px 14px;
    border: 1px solid var(--border-strong);
    border-radius: var(--r-lg);
    background: var(--surface);
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
  }

  .savebar.failed {
    border-color: var(--err);
  }

  .msg {
    flex: 1 1 200px;
    min-width: 0;
    font-size: 12.5px;
    color: var(--text-2);
    overflow-wrap: anywhere;
  }

  .failed .msg {
    color: var(--err);
  }
</style>

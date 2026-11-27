<!--
  确认框：做之前再问一次。用于删除、覆盖、明文导出这类不好撤回或涉及敏感内容的操作。

    <ConfirmDialog
      open={target !== null}
      title="删除规则"
      message="删除后无法恢复。"
      confirmLabel="删除"
      danger
      busy={removing}
      onconfirm={remove}
      oncancel={() => (target = null)}
    />

  危险操作的默认焦点在「取消」上，回车不会直接执行。
-->
<script lang="ts">
  import type { Snippet } from 'svelte';
  import { tr } from '../i18n/index.svelte';
  import Dialog from './Dialog.svelte';

  interface Props {
    open: boolean;
    title: string;
    /** 说明文字；需要列表等更复杂的内容时用 children。 */
    message?: string;
    confirmLabel?: string;
    /** 执行中显示在确认按钮上的文字。 */
    busyLabel?: string;
    cancelLabel?: string;
    /** 危险操作：确认按钮用警示色，默认焦点放在「取消」上。 */
    danger?: boolean;
    busy?: boolean;
    onconfirm: () => void;
    oncancel: () => void;
    children?: Snippet;
  }

  let {
    open,
    title,
    message = '',
    confirmLabel,
    busyLabel,
    cancelLabel,
    danger = false,
    busy = false,
    onconfirm,
    oncancel,
    children,
  }: Props = $props();

  const resolvedConfirm = $derived(confirmLabel ?? tr('确定', 'OK'));
  const resolvedBusy = $derived(busyLabel ?? tr('处理中…', 'Working…'));
  const resolvedCancel = $derived(cancelLabel ?? tr('取消', 'Cancel'));

  function confirm(): void {
    if (!busy) onconfirm();
  }

  function cancel(): void {
    if (!busy) oncancel();
  }
</script>

<Dialog {open} {title} {busy} width={420} onclose={oncancel}>
  {#if message !== ''}
    <p class="msg selectable">{message}</p>
  {/if}
  {@render children?.()}
  {#snippet footer()}
    <button type="button" class="btn" data-autofocus={danger ? '' : undefined} aria-disabled={busy} onclick={cancel}>
      {resolvedCancel}
    </button>
    <button
      type="button"
      class="btn primary"
      class:danger
      data-autofocus={danger ? undefined : ''}
      aria-disabled={busy}
      onclick={confirm}
    >
      {busy ? resolvedBusy : resolvedConfirm}
    </button>
  {/snippet}
</Dialog>

<style>
  .msg {
    color: var(--text-2);
    line-height: 1.6;
    overflow-wrap: anywhere;
    white-space: pre-line;
  }

  .btn.primary.danger {
    background: var(--err);
    color: #fff;
  }

  .btn.primary.danger:hover:not([aria-disabled='true']) {
    background: var(--err);
    filter: brightness(1.08);
  }
</style>

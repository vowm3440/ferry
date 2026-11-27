<script lang="ts">
  import type { IconName } from '../icons';
  import { tr } from '../i18n/index.svelte';
  import { toasts, type ToastKind } from '../stores/toasts.svelte';
  import Icon from './Icon.svelte';

  const ICONS: Record<ToastKind, IconName> = {
    info: 'info',
    ok: 'check',
    error: 'alert',
  };
</script>

<div class="toasts" role="region" aria-label={tr('通知', 'Notifications')}>
  {#each toasts.items as toast (toast.id)}
    <div class="toast {toast.kind}" role={toast.kind === 'error' ? 'alert' : 'status'}>
      <span class="ic"><Icon name={ICONS[toast.kind]} /></span>
      <span class="msg selectable">{toast.message}</span>
      <button type="button" class="icon-btn" aria-label={tr('关闭通知', 'Dismiss notification')} onclick={() => toasts.dismiss(toast.id)}>
        <Icon name="close" size={14} />
      </button>
    </div>
  {/each}
</div>

<style>
  .toasts {
    position: fixed;
    right: 18px;
    /* 页面有贴底的操作栏时（见 lib/bottom-inset.ts）上移，避免盖住栏里的按钮。 */
    bottom: max(18px, var(--bottom-inset, 0px));
    z-index: 50;
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: min(380px, calc(100vw - 36px));
    pointer-events: none;
    transition: bottom var(--dur) var(--ease);
  }

  .toast {
    pointer-events: auto;
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 10px 8px 10px 12px;
    border-radius: var(--r);
    border: 1px solid var(--border-strong);
    background: var(--surface);
    box-shadow: 0 10px 28px rgba(0, 0, 0, 0.28);
    animation: toast-in 0.2s var(--ease);
  }

  .ic {
    display: grid;
    place-items: center;
    height: 20px;
    color: var(--accent-strong);
  }

  .ok .ic {
    color: var(--ok);
  }

  .error .ic {
    color: var(--err);
  }

  .error {
    border-color: var(--err);
  }

  .msg {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    padding-top: 0.5px;
  }

  .icon-btn {
    width: 22px;
    height: 22px;
    margin-top: -1px;
  }

  @keyframes toast-in {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }
</style>

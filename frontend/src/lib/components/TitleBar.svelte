<script lang="ts">
  import { onMount } from 'svelte';
  import { loadWindowControls, messageOfError, type WindowControls } from '../api';
  import { tr } from '../i18n/index.svelte';
  import { connection } from '../stores/connection.svelte';
  import { nav, pageTitle } from '../stores/nav.svelte';
  import { toasts } from '../stores/toasts.svelte';

  const RESIZE_DEBOUNCE_MS = 120;

  let controls = $state.raw<WindowControls | null>(null);
  let maximised = $state(false);

  async function refreshMaximised(target: WindowControls): Promise<void> {
    try {
      maximised = await target.isMaximised();
    } catch {
      // 读不到最大化状态只影响按钮图标，不打断用户。
    }
  }

  async function run(action: (target: WindowControls) => Promise<void>): Promise<void> {
    const target = controls;
    if (target === null) return;
    try {
      await action(target);
    } catch (error: unknown) {
      toasts.error(messageOfError(error));
      return;
    }
    await refreshMaximised(target);
  }

  onMount(() => {
    let disposed = false;
    let off: (() => void) | null = null;
    let timer: number | null = null;

    const onResize = (): void => {
      if (timer !== null) window.clearTimeout(timer);
      timer = window.setTimeout(() => {
        timer = null;
        if (controls !== null) void refreshMaximised(controls);
      }, RESIZE_DEBOUNCE_MS);
    };

    void (async () => {
      try {
        const loaded = await loadWindowControls();
        if (disposed || loaded === null) return;
        controls = loaded;
        off = loaded.onStateChange(() => void refreshMaximised(loaded));
        window.addEventListener('resize', onResize);
        await refreshMaximised(loaded);
      } catch (error: unknown) {
        toasts.error(tr(`窗口控制不可用：${messageOfError(error)}`, `Window controls unavailable: ${messageOfError(error)}`));
      }
    })();

    return () => {
      disposed = true;
      off?.();
      window.removeEventListener('resize', onResize);
      if (timer !== null) window.clearTimeout(timer);
    };
  });
</script>

<header class="titlebar">
  <div class="crumb">Ferry <span aria-hidden="true">/</span> {pageTitle(nav.meta)}</div>
  {#if connection.isDemo}
    <span class="pill phase">{tr('演示模式', 'Demo Mode')}</span>
  {/if}

  {#if controls !== null}
    <div class="caption">
      <button type="button" class="min" aria-label={tr('最小化', 'Minimize')} title={tr('最小化', 'Minimize')} onclick={() => run((target) => target.minimise())}>
        <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M0 5h10" /></svg>
      </button>
      <button
        type="button"
        class="max"
        aria-label={maximised ? tr('还原', 'Restore') : tr('最大化', 'Maximize')}
        title={maximised ? tr('还原', 'Restore') : tr('最大化', 'Maximize')}
        onclick={() => run((target) => target.toggleMaximise())}
      >
        {#if maximised}
          <svg viewBox="0 0 10 10" aria-hidden="true">
            <path d="M2.5 2.5v-2h7v7h-2" />
            <rect x=".5" y="2.5" width="7" height="7" rx="1" />
          </svg>
        {:else}
          <svg viewBox="0 0 10 10" aria-hidden="true"><rect x=".5" y=".5" width="9" height="9" rx="1" /></svg>
        {/if}
      </button>
      <button type="button" class="close" aria-label={tr('关闭', 'Close')} title={tr('关闭', 'Close')} onclick={() => run((target) => target.close())}>
        <svg viewBox="0 0 10 10" aria-hidden="true"><path d="M0 0l10 10M10 0L0 10" /></svg>
      </button>
    </div>
  {/if}
</header>

<style>
  /*
   * 拖拽区：Wails v3 运行时读取 --wails-draggable。
   * --wails-non-client-region 只在 Go 侧启用 WebView2 合成托管时生效，
   * 届时系统会接管标题栏命中测试（含贴靠布局）；未启用时没有影响。
   */
  .titlebar {
    height: 36px;
    flex: none;
    display: flex;
    align-items: center;
    gap: 10px;
    --wails-draggable: drag;
    --wails-non-client-region: caption;
  }

  .crumb {
    padding-left: 24px;
    font-size: 12px;
    color: var(--text-3);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .caption {
    margin-left: auto;
    display: flex;
    --wails-draggable: no-drag;
  }

  .caption button {
    width: 46px;
    height: 36px;
    padding: 0;
    border: 0;
    background: transparent;
    display: grid;
    place-items: center;
    color: var(--text-2);
    cursor: default;
    transition: background var(--dur);
  }

  .caption button:hover {
    background: var(--muted-soft);
  }

  .caption button:focus-visible {
    outline-offset: -2px;
  }

  .caption button.close:hover {
    background: #c42b1c;
    color: #fff;
  }

  .caption svg {
    width: 10px;
    height: 10px;
    stroke: currentColor;
    stroke-width: 1;
    fill: none;
  }

  .min {
    --wails-non-client-region: minimize;
  }

  .max {
    --wails-non-client-region: maximize;
  }

  .close {
    --wails-non-client-region: close;
  }

  @media (max-width: 760px) {
    .crumb {
      padding-left: 14px;
    }
  }
</style>

<script lang="ts">
  import { CORE_STATUS_ERROR, CORE_STATUS_STARTING, CORE_STATUS_STOPPING } from '../api';
  import { formatBytes } from '../format';
  import { tr } from '../i18n/index.svelte';
  import { connection } from '../stores/connection.svelte';
  import { core } from '../stores/core.svelte';
  import { nav, pageTitle, PAGES } from '../stores/nav.svelte';
  import { theme } from '../stores/theme.svelte';
  import Icon from './Icon.svelte';

  interface ChipView {
    readonly tone: '' | 'ok' | 'warn' | 'err';
    readonly text: string;
  }

  const chip = $derived.by<ChipView>(() => {
    if (connection.phase === 'checking') return { tone: '', text: tr('正在连接…', 'Connecting…') };
    if (connection.phase !== 'ready') return { tone: '', text: tr('未连接后端', 'Disconnected') };

    const status = core.state?.status ?? '';
    if (core.pending === 'restart') return { tone: 'warn', text: tr('正在重启…', 'Restarting…') };
    if (core.pending === 'start' || status === CORE_STATUS_STARTING) return { tone: 'warn', text: tr('正在启动…', 'Starting…') };
    if (core.pending === 'stop' || status === CORE_STATUS_STOPPING) return { tone: 'warn', text: tr('正在停止…', 'Stopping…') };
    if (core.running) return { tone: 'ok', text: tr('内核运行中', 'Core Running') };
    if (status === CORE_STATUS_ERROR) return { tone: 'err', text: tr('内核异常', 'Core Error') };
    return { tone: '', text: tr('内核未启动', 'Core Stopped') };
  });

  const versionText = $derived.by<string>(() => {
    if (connection.isDemo) return tr('演示数据', 'Demo Data');
    const state = core.state;
    if (state === null || state.version === '') return tr('未连接', 'Offline');
    return state.coreVersion === '' ? `v${state.version}` : `v${state.version} · core ${state.coreVersion}`;
  });

  const themeLabel = $derived(
    theme.resolved === 'dark' ? tr('切换到浅色主题', 'Switch to Light Theme') : tr('切换到深色主题', 'Switch to Dark Theme'),
  );
</script>

<aside class="sidebar">
  <div class="brand">
    <div class="logo"><Icon name="logo" size={14} /></div>
    <b>Ferry</b><span>渡</span>
  </div>

  <nav class="nav" aria-label={tr('主导航', 'Main navigation')}>
    {#each PAGES as page (page.id)}
      {@const active = nav.current === page.id}
      {@const label = pageTitle(page)}
      {#if page.id === 'settings'}
        <div class="nav-sep" role="separator"></div>
      {/if}
      <button
        type="button"
        class:active
        aria-current={active ? 'page' : undefined}
        title={label}
        onclick={() => nav.go(page.id)}
      >
        <Icon name={page.icon} />
        <span class="label">{label}</span>
      </button>
    {/each}
  </nav>

  <div class="side-foot">
    <div class="core-chip">
      <span class="dot {chip.tone}"></span>
      <span class="grow" aria-live="polite">{chip.text}</span>
      {#if core.running}
        <span class="num mem">{formatBytes(core.memoryBytes)}</span>
      {/if}
    </div>
    <div class="foot-row">
      <span class="muted ver">{versionText}</span>
      <button type="button" class="icon-btn" aria-label={themeLabel} title={themeLabel} onclick={() => theme.toggle()}>
        <Icon name={theme.resolved === 'dark' ? 'sun' : 'moon'} />
      </button>
    </div>
  </div>
</aside>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    min-height: 0;
    padding: 10px 10px 12px;
    background: var(--mica);
    border-right: 1px solid var(--border);
    overflow: hidden auto;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: 10px;
    height: 36px;
    flex: none;
    padding: 0 8px;
    margin-bottom: 10px;
    --wails-draggable: drag;
    --wails-non-client-region: caption;
  }

  .logo {
    width: 24px;
    height: 24px;
    flex: none;
    display: grid;
    place-items: center;
    border-radius: 7px;
    background: linear-gradient(135deg, var(--accent), #9b7bff);
    color: #fff;
  }

  .brand b {
    font-size: 14px;
    font-weight: 600;
    letter-spacing: 0.2px;
  }

  .brand span {
    color: var(--text-3);
    font-size: 12px;
  }

  .nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .nav button {
    position: relative;
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    height: 34px;
    flex: none;
    padding: 0 10px;
    border: 0;
    border-radius: var(--r);
    background: transparent;
    color: var(--text-2);
    text-align: left;
    cursor: pointer;
    transition:
      background var(--dur) var(--ease),
      color var(--dur);
  }

  .nav button:hover {
    background: var(--muted-soft);
    color: var(--text);
  }

  .nav button.active {
    background: var(--accent-soft);
    color: var(--text);
  }

  .nav button.active::before {
    content: '';
    position: absolute;
    left: 0;
    top: 9px;
    bottom: 9px;
    width: 3px;
    border-radius: 3px;
    background: var(--accent);
  }

  .nav button.active :global(svg.i) {
    color: var(--accent-strong);
  }

  .nav-sep {
    height: 1px;
    flex: none;
    background: var(--border);
    margin: 8px 6px;
  }

  .side-foot {
    margin-top: auto;
    padding-top: 12px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .core-chip {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 10px;
    border-radius: var(--r);
    background: var(--muted-soft);
    font-size: 12px;
    color: var(--text-2);
  }

  .core-chip .grow {
    flex: 1;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .foot-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 4px;
  }

  .ver {
    font-size: 11.5px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* 窄窗口：侧栏收成图标栏。文字用裁剪隐藏，读屏仍可读到。 */
  @media (max-width: 760px) {
    .sidebar {
      padding: 10px 8px 12px;
    }

    .brand {
      justify-content: center;
      padding: 0;
    }

    .brand b,
    .brand span,
    .mem,
    .ver {
      display: none;
    }

    .nav button {
      justify-content: center;
      gap: 0;
      padding: 0;
    }

    .label,
    .core-chip .grow {
      position: absolute;
      width: 1px;
      height: 1px;
      overflow: hidden;
      clip: rect(0, 0, 0, 0);
      white-space: nowrap;
    }

    .core-chip {
      justify-content: center;
      padding: 10px 0;
    }

    .foot-row {
      justify-content: center;
      padding: 0;
    }
  }
</style>

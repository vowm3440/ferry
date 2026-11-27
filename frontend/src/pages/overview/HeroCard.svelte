<script lang="ts">
  import {
    CORE_STATUS_ERROR,
    CORE_STATUS_STARTING,
    CORE_STATUS_STOPPING,
    modeLabel,
    PROXY_MODES,
    type ProxyMode,
  } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import Segmented from '../../lib/components/Segmented.svelte';
  import { formatDurationText, formatLatency, latencyLevel } from '../../lib/format';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { currentSelection } from '../../lib/nodes-view';
  import { core } from '../../lib/stores/core.svelte';
  import { nav } from '../../lib/stores/nav.svelte';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import ProxyTiles from './ProxyTiles.svelte';

  const modeOptions = $derived((void i18n.locale, PROXY_MODES.map((mode) => ({ value: mode, label: modeLabel(mode) }))));

  interface StatusView {
    readonly tone: '' | 'ok' | 'warn' | 'err';
    readonly text: string;
    readonly sub: string;
  }

  const status = $derived.by<StatusView>(() => {
    void i18n.locale;
    const state = core.state;
    const label = modeLabel(core.mode);
    const modeText = tr(`${label}模式`, `${label} Mode`);

    if (state === null) {
      return core.loadError === ''
        ? { tone: '', text: tr('正在读取状态…', 'Loading status…'), sub: tr('正在向内核请求当前状态', 'Querying core state') }
        : { tone: 'err', text: tr('状态未知', 'Status Unknown'), sub: tr('无法读取内核状态', 'Unable to read core status') };
    }
    if (core.pending === 'restart') return { tone: 'warn', text: tr('正在重启…', 'Restarting…'), sub: modeText };
    if (core.pending === 'start' || state.status === CORE_STATUS_STARTING) {
      return { tone: 'warn', text: tr('正在启动…', 'Starting…'), sub: modeText };
    }
    if (core.pending === 'stop' || state.status === CORE_STATUS_STOPPING) {
      return { tone: 'warn', text: tr('正在停止…', 'Stopping…'), sub: modeText };
    }
    if (core.running) {
      const dur = formatDurationText(core.uptimeMs);
      return { tone: 'ok', text: tr('内核运行中', 'Core Running'), sub: tr(`${modeText} · 已运行 ${dur}`, `${modeText} · Up ${dur}`) };
    }
    if (state.status === CORE_STATUS_ERROR) {
      return { tone: 'err', text: tr('内核异常', 'Core Error'), sub: tr(`${modeText} · 可尝试重新启动`, `${modeText} · Try restarting`) };
    }
    return { tone: '', text: tr('内核未启动', 'Core Stopped'), sub: tr(`${modeText} · 点击左侧按钮启动`, `${modeText} · Click power to start`) };
  });

  /**
   * 「处理中」只让按钮暂不响应（aria-disabled），不用 disabled：
   * 被禁用的按钮会丢掉键盘焦点，操作结束后键盘用户得重新找回位置。
   */
  const busy = $derived(core.pending !== null || core.transitioning);
  const modeDisabled = $derived(core.state === null || (core.pending !== null && core.pending !== 'mode'));

  function togglePower(): void {
    if (busy) return;
    void core.toggle();
  }

  function restart(): void {
    if (core.pending !== null) return;
    void core.restart();
  }

  const selection = $derived((void i18n.locale, currentSelection(nodes.list)));

  const powerLabel = $derived(core.running ? tr('停止内核', 'Stop Core') : tr('启动内核', 'Start Core'));

  /** 内核自己上报的错误是持续存在的状况，不能由用户关闭。 */
  const stateError = $derived(core.state?.error ?? '');
  /** 启动失败时后端返回的说明通常与内核上报的错误相同，相同时只显示一次。 */
  const actionError = $derived(core.actionError === stateError ? '' : core.actionError);
</script>

<section class="card hero">
  <div class="hero-top">
    <button
      type="button"
      class="power"
      class:on={core.running}
      disabled={core.state === null}
      aria-disabled={busy}
      aria-pressed={core.running}
      aria-label={powerLabel}
      title={powerLabel}
      onclick={togglePower}
    >
      <Icon name="power" />
    </button>

    <div class="status">
      <div class="status-line"><span class="dot {status.tone}"></span>{status.text}</div>
      <div class="sub">{status.sub}</div>
    </div>

    <div class="right">
      <button
        type="button"
        class="icon-btn"
        disabled={!core.running && core.pending !== 'restart'}
        aria-disabled={core.pending !== null}
        aria-label={tr('重启内核', 'Restart Core')}
        title={tr('重启内核', 'Restart Core')}
        onclick={restart}
      >
        <Icon name="refresh" />
      </button>
      <Segmented
        options={modeOptions}
        value={core.mode}
        label={tr('代理模式', 'Proxy Mode')}
        disabled={modeDisabled}
        busy={core.pending === 'mode'}
        onchange={(next: ProxyMode) => void core.setMode(next)}
      />
    </div>
  </div>

  {#if stateError !== ''}
    <div class="inline-error" role="alert">
      <Icon name="alert" />
      <span class="grow selectable">{stateError}</span>
    </div>
  {/if}
  {#if actionError !== ''}
    <div class="inline-error" role="alert">
      <Icon name="alert" />
      <span class="grow selectable">{actionError}</span>
      <button type="button" class="icon-btn" aria-label={tr('关闭提示', 'Dismiss')} onclick={() => core.dismissActionError()}>
        <Icon name="close" size={14} />
      </button>
    </div>
  {/if}

  <div class="cur-node">
    {#if !nodes.loaded}
      {#if nodes.error !== ''}
        <span class="desc grow selectable">{tr(`读取节点失败：${nodes.error}`, `Failed to load nodes: ${nodes.error}`)}</span>
        <button type="button" class="btn sm" onclick={() => void nodes.refresh()}>{tr('重试', 'Retry')}</button>
      {:else}
        <span class="desc grow">{tr('正在读取节点…', 'Loading nodes…')}</span>
      {/if}
    {:else if selection === null}
      <span class="cc">—</span>
      <div class="meta">
        <div class="name">{tr('尚未导入节点', 'No Nodes Imported')}</div>
        <div class="desc">{tr('在「订阅」页粘贴节点链接即可导入', 'Import share links or subscriptions in Subscriptions')}</div>
      </div>
      <button type="button" class="btn sm" onclick={() => nav.go('subs')}>{tr('前往订阅', 'Subscriptions')}</button>
    {:else}
      <span class="cc">{selection.badge}</span>
      <div class="meta">
        <div class="name">{selection.name}</div>
        <div class="desc">{selection.detail}</div>
      </div>
      {#if core.mode === 'direct'}
        <span class="pill">{tr('直连模式不经过节点', 'Bypassed in Direct Mode')}</span>
      {:else if !core.running}
        <span class="pill">{tr('启动后使用', 'Active on start')}</span>
      {:else if !selection.automatic}
        <span class="lat {latencyLevel(selection.latencyMs)}">{(void i18n.locale, formatLatency(selection.latencyMs))}</span>
      {/if}
      <button type="button" class="btn sm" onclick={() => nav.go('nodes')}>{tr('切换', 'Switch')}</button>
    {/if}
  </div>

  <ProxyTiles />
</section>

<style>
  .hero {
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .hero-top {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 14px;
  }

  .power {
    width: 52px;
    height: 52px;
    flex: none;
    border-radius: 50%;
    border: 0;
    display: grid;
    place-items: center;
    cursor: pointer;
    color: var(--text-2);
    background: var(--surface-3);
    box-shadow: 0 0 0 6px var(--muted-soft);
    transition:
      background var(--dur) var(--ease),
      color var(--dur) var(--ease),
      box-shadow var(--dur) var(--ease);
  }

  .power.on {
    color: #fff;
    background: linear-gradient(140deg, var(--accent), #9a7cff);
    box-shadow:
      0 0 0 6px var(--accent-soft),
      0 8px 20px rgba(111, 141, 255, 0.35);
  }

  .power:disabled,
  .power[aria-disabled='true'] {
    opacity: 0.65;
  }

  .power:disabled {
    cursor: not-allowed;
  }

  .power[aria-disabled='true'] {
    cursor: progress;
  }

  .power :global(svg.i) {
    width: 22px;
    height: 22px;
    stroke-width: 2.2;
  }

  .status {
    min-width: 0;
  }

  .status-line {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 16px;
    font-weight: 600;
  }

  .sub {
    color: var(--text-3);
    font-size: 12px;
    margin-top: 2px;
  }

  .right {
    margin-left: auto;
    display: flex;
    align-items: center;
    gap: 6px;
  }

  .cur-node {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
    padding: 10px 12px;
    border-radius: var(--r);
    background: var(--surface-2);
    border: 1px solid var(--border);
  }

  .meta {
    flex: 1;
    min-width: 0;
  }

  .name {
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .desc {
    font-size: 12px;
    color: var(--text-3);
    overflow-wrap: anywhere;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }
</style>

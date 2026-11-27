<script lang="ts">
  import { CAP_SYS_PROXY, CAP_TUN } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { caps } from '../../lib/stores/caps.svelte';
  import { core } from '../../lib/stores/core.svelte';
  import { settings } from '../../lib/stores/settings.svelte';
  import { system } from '../../lib/stores/system.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import { sysProxyTile } from '../../lib/system-view';

  const tile = $derived(
    (void i18n.locale,
    sysProxyTile({
      capsLoaded: caps.loaded,
      capable: caps.has(CAP_SYS_PROXY),
      status: system.proxy,
      error: system.proxyError,
      running: core.running,
      busy: false,
      mixedPort: settings.value?.mixedPort ?? 0,
    })),
  );

  const failed = $derived(caps.has(CAP_SYS_PROXY) && system.proxyError !== '');
  const canTun = $derived(caps.has(CAP_TUN));
  const svcInstalled = $derived(system.service?.installed === true);
  const svcNeedsRepair = $derived(system.service?.needsRepair === true);
  const svcReady = $derived(svcInstalled && system.service?.running === true && !svcNeedsRepair);
  const svcBusy = $derived(system.serviceAction !== '');
  const tunOn = $derived(system.tun?.enabled === true);

  async function toggle(next: boolean): Promise<void> {
    if (system.proxyBusy) return;
    const result = await system.setSysProxy(next);
    if (result.ok) toasts.ok(next ? tr('系统代理已开启', 'System proxy enabled') : tr('系统代理已关闭', 'System proxy disabled'));
    else toasts.error(result.message);
  }

  async function toggleTun(next: boolean): Promise<void> {
    if (system.tunBusy) return;
    const result = await system.setTun(next);
    if (result.ok) toasts.ok(next ? tr('TUN 模式已开启', 'TUN mode enabled') : tr('TUN 模式已关闭', 'TUN mode disabled'));
    else toasts.error(result.message);
  }

  async function handleServiceAction(action: 'install' | 'repair'): Promise<void> {
    if (svcBusy) return;
    const result = await system.runServiceAction(action);
    if (!result.ok) {
      toasts.error(result.message);
      return;
    }
    toasts.ok(tr('系统服务已就绪，正在重启软件与内核…', 'Service ready; restarting Ferry and core…'));
  }
</script>

<div class="toggles">
  <div class="tile" class:on={tile.on} data-tile="sysproxy">
    <div class="ic"><Icon name="sysproxy" /></div>
    <div class="grow">
      <div class="t">{tr('系统代理', 'System Proxy')}</div>
      <div class="d">
        {#if tile.address !== ''}
          <span class="mono selectable">{tile.address}</span>
          <span class="sep" aria-hidden="true">·</span>
        {/if}
        <span class:bad={failed}>{tile.detail}</span>
      </div>
    </div>
    {#if failed}
      <button type="button" class="btn sm" onclick={() => void system.refresh()}>{tr('重试', 'Retry')}</button>
    {:else}
      <Switch
        checked={tile.on}
        disabled={!tile.canToggle}
        label={tile.canToggle ? tr('系统代理', 'System Proxy') : tr(`系统代理（${tile.detail}）`, `System Proxy (${tile.detail})`)}
        onchange={(next) => void toggle(next)}
      />
    {/if}
  </div>

  {#if canTun}
    <div class="tile" class:on={tunOn} class:off={!svcReady && !tunOn} data-tile="tun">
      <div class="ic"><Icon name="shield" /></div>
      <div class="grow">
        <div class="t">{tr('TUN 模式', 'TUN Mode')}</div>
        <div class="d">
          {#if svcReady || tunOn}
            {tunOn ? tr('已开启', 'On') : tr('未开启', 'Off')}
          {:else if svcInstalled}
            {svcNeedsRepair ? tr('服务需更新', 'Update required') : tr('服务未运行', 'Service stopped')}
          {:else}
            {tr('未安装服务', 'Not installed')}
          {/if}
        </div>
      </div>
      {#if svcReady || tunOn}
        <Switch
          checked={tunOn}
          disabled={system.tunBusy}
          label={tr('TUN 模式', 'TUN Mode')}
          onchange={(next) => void toggleTun(next)}
        />
      {:else if svcInstalled}
        <button type="button" class="btn sm" aria-disabled={svcBusy} onclick={() => void handleServiceAction('repair')}>
          {svcBusy ? tr('处理中…', 'Working…') : svcNeedsRepair ? tr('修复服务', 'Repair') : tr('启动服务', 'Start')}
        </button>
      {:else}
        <button type="button" class="btn sm" aria-disabled={svcBusy} onclick={() => void handleServiceAction('install')}>
          {svcBusy ? tr('安装中…', 'Installing…') : tr('安装服务', 'Install')}
        </button>
      {/if}
    </div>
  {:else}
    <div class="tile off" data-tile="tun">
      <div class="ic"><Icon name="shield" /></div>
      <div class="grow">
        <div class="t">{tr('TUN 模式', 'TUN Mode')} <span class="pill">{tr('未开放', 'Unavailable')}</span></div>
        <div class="d">{tr('需要特权服务，当前版本不提供', 'Requires privileged service; unavailable in this backend')}</div>
      </div>
    </div>
  {/if}
</div>

<style>
  .toggles {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
  }

  .tile {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px;
    min-width: 0;
    border-radius: var(--r);
    border: 1px solid var(--border);
    background: var(--surface);
    transition:
      border-color var(--dur) var(--ease),
      background var(--dur) var(--ease);
  }

  .tile.on {
    border-color: var(--accent-ring);
    background: linear-gradient(0deg, var(--accent-soft), var(--accent-soft)), var(--surface);
  }

  .tile.off {
    background: var(--surface-2);
  }

  .ic {
    width: 30px;
    height: 30px;
    flex: none;
    border-radius: 8px;
    display: grid;
    place-items: center;
    background: var(--surface-3);
    color: var(--text-2);
  }

  .tile.on .ic {
    background: var(--accent);
    color: #fff;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .t {
    font-weight: 600;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
  }

  .tile.off .t {
    color: var(--text-2);
  }

  .d {
    font-size: 12px;
    color: var(--text-3);
    overflow-wrap: anywhere;
  }

  .sep {
    margin: 0 2px;
  }

  .bad {
    color: var(--err);
  }

  @container content (max-width: 720px) {
    .toggles {
      grid-template-columns: 1fr;
    }
  }
</style>

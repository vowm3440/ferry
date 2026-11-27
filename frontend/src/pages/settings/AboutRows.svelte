<script lang="ts">
  import { CAP_UPDATE_ONLINE, EVENT_UPDATE_STATE, messageOfError, type UpdateState } from '../../lib/api';
  import { tr } from '../../lib/i18n/index.svelte';
  import { caps } from '../../lib/stores/caps.svelte';
  import { connection } from '../../lib/stores/connection.svelte';
  import { core } from '../../lib/stores/core.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import LegalNotice from './LegalNotice.svelte';
  import SettingRow from './SettingRow.svelte';

  interface Props {
    /** 更新通道；设置尚未读取到时为 null。 */
    channel: string | null;
    onChannel?: (next: string) => void;
  }

  let { channel, onChannel }: Props = $props();

  const DASH = '—';
  let busy = $state(false);
  let updateState = $state<UpdateState>({ phase: 'idle', version: '', progress: 0 });
  let updateNote = $state('');

  const canOnlineUpdate = $derived(caps.has(CAP_UPDATE_ONLINE));
  const version = $derived(core.state?.version || DASH);
  const coreVersion = $derived(core.state?.coreVersion || DASH);
  const runMode = $derived.by<string>(() => {
    if (core.state === null) return DASH;
    return core.state.serviceMode ? tr('服务模式', 'Service Mode') : tr('子进程模式', 'User Process Mode');
  });
  const channelLabel = $derived.by<string>(() => {
    if (channel === null) return DASH;
    if (channel === 'stable') return tr('稳定版', 'Stable');
    if (channel === 'beta') return tr('测试版', 'Beta');
    return channel;
  });

  $effect(() => {
    const backend = connection.backend;
    if (backend === null || !canOnlineUpdate) return;
    void backend
      .updateGetState()
      .then((st) => {
        updateState = st;
      })
      .catch(() => {});
    return backend.on(EVENT_UPDATE_STATE, (payload) => {
      if (payload !== null && typeof payload === 'object' && 'phase' in payload) {
        updateState = payload as UpdateState;
      }
    });
  });

  async function checkNow(): Promise<void> {
    const backend = connection.backend;
    if (backend === null || busy) return;
    busy = true;
    updateNote = '';
    try {
      const info = await backend.checkUpdate(channel ?? 'stable');
      if (info.available) {
        updateNote = tr(`发现已验签新版本 v${info.version}`, `Verified update v${info.version} available`);
        toasts.ok(updateNote);
      } else {
        updateNote = tr('当前已是最新版本', 'Already up to date');
        toasts.ok(updateNote);
      }
    } catch (cause: unknown) {
      updateNote = messageOfError(cause);
      toasts.error(updateNote);
    } finally {
      busy = false;
    }
  }

  async function downloadNow(): Promise<void> {
    const backend = connection.backend;
    if (backend === null || busy) return;
    busy = true;
    updateNote = '';
    try {
      await backend.applyUpdate();
      updateNote = tr(`更新包 v${updateState.version} 已下载并完成 SHA-256 校验`, `Update v${updateState.version} downloaded and SHA-256 verified`);
      toasts.ok(updateNote);
    } catch (cause: unknown) {
      updateNote = messageOfError(cause);
      toasts.error(updateNote);
    } finally {
      busy = false;
    }
  }

  async function installNow(): Promise<void> {
    const backend = connection.backend;
    if (backend === null || busy) return;
    busy = true;
    try {
      await backend.installUpdate();
      toasts.ok(tr('已启动验证通过的更新程序', 'Launched verified update installer'));
    } catch (cause: unknown) {
      updateNote = messageOfError(cause);
      toasts.error(updateNote);
    } finally {
      busy = false;
    }
  }
</script>

{#if connection.isDemo}
  <div class="sec-note">{tr('当前是演示模式，以下版本信息为合成示例。', 'Demo mode active; version info below is synthetic.')}</div>
{/if}
<SettingRow title={tr('Ferry 版本', 'Ferry Version')}>
  <span class="chip mono selectable">{version}</span>
</SettingRow>
<SettingRow title={tr('sing-box 内核', 'sing-box Core')}>
  <span class="chip mono selectable">{coreVersion}</span>
</SettingRow>
<SettingRow title={tr('运行方式', 'Runtime Mode')}>
  <span class="chip">{runMode}</span>
</SettingRow>
<SettingRow
  title={tr('应用内更新', 'In-App Updates')}
  desc={updateNote}
  tag={canOnlineUpdate ? '' : tr('未开放', 'Unconfigured')}
>
  <div class="upd-row">
    {#if onChannel && channel !== null}
      <select
        class="select"
        aria-label={tr('更新通道', 'Update Channel')}
        value={channel}
        onchange={(event) => onChannel?.(event.currentTarget.value)}
      >
        <option value="stable">{tr('稳定版', 'Stable')}</option>
        <option value="beta">{tr('测试版', 'Beta')}</option>
      </select>
    {:else}
      <span class="chip">{channelLabel}</span>
    {/if}
    {#if canOnlineUpdate}
      {#if updateState.phase === 'staged'}
        <button type="button" class="btn sm primary" aria-disabled={busy} onclick={() => void installNow()}>
          {busy ? tr('正在启动…', 'Launching…') : tr(`安装 v${updateState.version}`, `Install v${updateState.version}`)}
        </button>
      {:else if updateState.phase === 'available'}
        <button type="button" class="btn sm primary" aria-disabled={busy} onclick={() => void downloadNow()}>
          {busy ? tr('下载校验中…', 'Downloading…') : tr(`下载 v${updateState.version}`, `Download v${updateState.version}`)}
        </button>
      {:else if updateState.phase === 'downloading'}
        <span class="chip num">{tr(`下载中 ${updateState.progress}%`, `Downloading ${updateState.progress}%`)}</span>
      {:else}
        <button type="button" class="btn sm" aria-disabled={busy} onclick={() => void checkNow()}>
          {busy || updateState.phase === 'checking' ? tr('检查中…', 'Checking…') : tr('检查更新', 'Check Updates')}
        </button>
      {/if}
    {/if}
  </div>
</SettingRow>
<SettingRow title={tr('许可证', 'License')}>
  <span class="chip">GPL-3.0-or-later</span>
</SettingRow>
<LegalNotice />

<style>
  .sec-note {
    padding: 10px 16px;
    font-size: 12px;
    color: var(--warn);
    background: var(--warn-soft);
    border-bottom: 1px solid var(--border);
    border-radius: var(--r-lg) var(--r-lg) 0 0;
  }

  .upd-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
</style>

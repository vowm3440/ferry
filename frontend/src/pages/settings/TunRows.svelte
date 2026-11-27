<script lang="ts">
  import { CAP_SERVICE, CAP_TUN, type SettingsValue } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import { DEFAULT_TUN_EXCLUDES, checkCidr } from '../../lib/settings-view';
  import { caps } from '../../lib/stores/caps.svelte';
  import { system, type ServiceAction } from '../../lib/stores/system.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import SettingRow from './SettingRow.svelte';

  interface Props {
    value: SettingsValue;
    disabled?: boolean;
    onChange?: (next: SettingsValue) => void;
  }

  let { value, disabled = false, onChange }: Props = $props();

  let cidrInput = $state('');
  let cidrError = $state('');

  const canService = $derived(caps.has(CAP_SERVICE));
  const canTun = $derived(caps.has(CAP_TUN));
  const svc = $derived(system.service);
  const busy = $derived(system.serviceAction !== '');

  const svcLabel = $derived.by<string>(() => {
    if (svc === null) return tr('检测中…', 'Checking…');
    if (svc.installed && svc.running) return tr('运行中（LocalSystem）', 'Running (LocalSystem)');
    if (svc.installed) return tr('已安装（未运行）', 'Installed (Stopped)');
    return tr('未安装', 'Not Installed');
  });

  function updateTun(patch: Partial<SettingsValue['tun']>): void {
    onChange?.({ ...value, tun: { ...value.tun, ...patch } });
  }

  function addCidr(): void {
    const trimmed = cidrInput.trim();
    if (trimmed === '') return;
    const problem = checkCidr(trimmed);
    if (problem !== '') {
      cidrError = problem;
      return;
    }
    if (value.tun.excludes.includes(trimmed)) {
      cidrError = tr('该网段已在排除列表中', 'CIDR is already in the exclude list');
      return;
    }
    if (value.tun.excludes.length >= 256) {
      cidrError = tr('排除网段最多支持 256 条', 'Maximum of 256 excluded CIDRs');
      return;
    }
    cidrError = '';
    cidrInput = '';
    updateTun({ excludes: [...value.tun.excludes, trimmed] });
  }

  function removeCidr(target: string): void {
    cidrError = '';
    updateTun({ excludes: value.tun.excludes.filter((item) => item !== target) });
  }

  function fillDefaults(): void {
    cidrError = '';
    const merged = [...new Set([...value.tun.excludes, ...DEFAULT_TUN_EXCLUDES])].slice(0, 256);
    updateTun({ excludes: merged });
  }

  function clearExcludes(): void {
    cidrError = '';
    updateTun({ excludes: [] });
  }

  async function runService(action: Exclude<ServiceAction, ''>): Promise<void> {
    const result = await system.runServiceAction(action);
    if (!result.ok) {
      toasts.error(result.message);
      return;
    }
    if (action === 'install') toasts.ok(tr('系统服务已安装并启动，现在可免 UAC 开启 TUN', 'Service installed and started; TUN ready without UAC'));
    else if (action === 'repair') toasts.ok(tr('系统服务已修复并重新启动', 'Service repaired and restarted'));
    else toasts.ok(tr('系统服务已卸载，已回退至用户态子进程模式', 'Service uninstalled; reverted to user process mode'));
  }
</script>

{#if !canService}
  <div class="sec-note">
    {tr(
      'TUN 模式与特权服务在当前版本未开放：不能安装服务，也不能启用 TUN。以下是为它保留的取值，不会生效，也不能修改。',
      'TUN mode and privileged service are unavailable in this backend: service installation and TUN cannot be enabled.',
    )}
  </div>
  <SettingRow title={tr('特权服务', 'Privileged Service')} desc={tr('TUN 需要一个以管理员权限运行的后台服务；当前版本不提供安装', 'TUN requires an elevated background service; unavailable here')} tag={tr('未开放', 'Unavailable')} />
  <SettingRow title={tr('协议栈', 'TUN Stack')} desc={tr('mixed：TCP 走系统栈，UDP 走 gVisor', 'mixed: system TCP + gVisor UDP')}>
    <span class="chip mono">{value.tun.stack === '' ? '—' : value.tun.stack}</span>
  </SettingRow>
  <SettingRow title={tr('严格路由', 'Strict Route')} desc={tr('防止 DNS 泄漏，阻断绕过 TUN 的流量', 'Prevent DNS leaks and block bypass traffic')}>
    <Switch checked={value.tun.strictRoute} disabled label={tr('严格路由（未开放）', 'Strict Route (Unavailable)')} />
  </SettingRow>
  <SettingRow title={tr('排除网段', 'Excluded CIDRs')} desc={tr('局域网、组播及自定义网段不进入 TUN', 'LAN, multicast, and custom CIDRs bypassing TUN')}>
    <span class="chip num">{tr(`${value.tun.excludes.length} 条`, `${value.tun.excludes.length} CIDRs`)}</span>
  </SettingRow>
  <SettingRow title="MTU">
    <span class="chip num">{value.tun.mtu}</span>
  </SettingRow>
{:else}
  <SettingRow
    title={tr('特权系统服务（FerryCore）', 'Privileged Windows Service (FerryCore)')}
    desc={tr(
      '通过一次 UAC 授权将内核安装到受保护的 ProgramData 目录（SHA-256 + 256 位随机密钥鉴权，无需商业签名证书），此后开关 TUN 不再弹窗。',
      'One-time UAC installs FerryCore into admin-protected ProgramData with SHA-256 + 256-bit token auth (no code-signing cert required); toggling TUN needs no UAC afterwards.',
    )}
    wide
  >
    <div class="svc-actions">
      <span class="chip">{svcLabel}</span>
      {#if !svc?.installed}
        <button type="button" class="btn sm primary" aria-disabled={busy} onclick={() => void runService('install')}>
          {system.serviceAction === 'install' ? tr('安装中…', 'Installing…') : tr('安装服务（UAC）', 'Install Service (UAC)')}
        </button>
      {:else}
        <button type="button" class="btn sm" aria-disabled={busy} onclick={() => void runService('repair')}>
          {system.serviceAction === 'repair' ? tr('修复中…', 'Repairing…') : tr('修复服务', 'Repair')}
        </button>
        <button type="button" class="btn sm danger" aria-disabled={busy} onclick={() => void runService('uninstall')}>
          {system.serviceAction === 'uninstall' ? tr('卸载中…', 'Uninstalling…') : tr('卸载服务', 'Uninstall')}
        </button>
      {/if}
    </div>
  </SettingRow>
  <SettingRow title={tr('协议栈', 'TUN Stack')} desc={tr('mixed：TCP 走系统栈，UDP 走 gVisor', 'mixed: system TCP + gVisor UDP')}>
    <select
      class="select"
      aria-label={tr('协议栈', 'TUN Stack')}
      value={value.tun.stack || 'mixed'}
      disabled={!canTun || disabled}
      onchange={(event) => updateTun({ stack: event.currentTarget.value })}
    >
      <option value="mixed">mixed</option>
      <option value="system">system</option>
      <option value="gvisor">gvisor</option>
    </select>
  </SettingRow>
  <SettingRow title={tr('严格路由', 'Strict Route')} desc={tr('防止 DNS 泄漏，阻断绕过 TUN 的流量', 'Prevent DNS leaks and block bypass traffic')}>
    <Switch
      checked={value.tun.strictRoute}
      disabled={!canTun || disabled}
      label={tr('严格路由', 'Strict Route')}
      onchange={(next: boolean) => updateTun({ strictRoute: next })}
    />
  </SettingRow>
  <SettingRow title={tr('FakeIP DNS', 'FakeIP DNS')} desc={tr('TUN 模式下推荐开启，加快域名分流响应', 'Recommended in TUN mode for fast domain routing')}>
    <Switch
      checked={value.tun.fakeIp}
      disabled={!canTun || disabled}
      label="FakeIP DNS"
      onchange={(next: boolean) => updateTun({ fakeIp: next })}
    />
  </SettingRow>
  <SettingRow
    title={tr('排除网段', 'Excluded CIDRs')}
    desc={tr('局域网、组播及自定义网段不进入 TUN（保存设置后生效）', 'LAN, multicast, and custom CIDRs bypassing TUN (applies on save)')}
    error={cidrError}
    wide
  >
    <div class="cidr-box">
      <div class="cidr-bar">
        <label class="input cidr-in" class:invalid={cidrError !== ''}>
          <input
            class="mono"
            type="text"
            placeholder="192.168.0.0/16"
            aria-label={tr('新增排除网段', 'New Excluded CIDR')}
            disabled={!canTun || disabled}
            bind:value={cidrInput}
            onkeydown={(event) => {
              if (event.key === 'Enter') {
                event.preventDefault();
                addCidr();
              }
            }}
          />
        </label>
        <button type="button" class="btn sm" disabled={!canTun || disabled} onclick={addCidr}>
          <Icon name="plus" />{tr('添加', 'Add')}
        </button>
        <button type="button" class="btn sm" disabled={!canTun || disabled} onclick={fillDefaults}>
          {tr('常用局域网', 'LAN Presets')}
        </button>
        {#if value.tun.excludes.length > 0}
          <button type="button" class="btn sm" disabled={!canTun || disabled} onclick={clearExcludes}>
            {tr('清空', 'Clear')}
          </button>
        {/if}
      </div>
      {#if value.tun.excludes.length > 0}
        <div class="cidr-chips">
          {#each value.tun.excludes as cidr (cidr)}
            <span class="cidr-chip mono">
              {cidr}
              <button
                type="button"
                class="cidr-del"
                aria-label={tr(`移除 ${cidr}`, `Remove ${cidr}`)}
                disabled={!canTun || disabled}
                onclick={() => removeCidr(cidr)}
              >
                <Icon name="close" size={12} />
              </button>
            </span>
          {/each}
        </div>
      {:else}
        <span class="muted empty-cidr">{tr('未配置自定义排除网段', 'No excluded CIDRs configured')}</span>
      {/if}
    </div>
  </SettingRow>
  <SettingRow title="MTU">
    <label class="input mtu">
      <input
        class="num"
        type="number"
        min="1280"
        max="65535"
        aria-label="MTU"
        value={value.tun.mtu}
        disabled={!canTun || disabled}
        oninput={(event) => {
          const n = Number.parseInt(event.currentTarget.value, 10);
          if (Number.isFinite(n)) updateTun({ mtu: n });
        }}
      />
    </label>
  </SettingRow>
{/if}

<style>
  .sec-note {
    padding: 10px 16px;
    font-size: 12px;
    color: var(--text-2);
    background: var(--surface-2);
    border-bottom: 1px solid var(--border);
    border-radius: var(--r-lg) var(--r-lg) 0 0;
  }

  .svc-actions {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }

  .cidr-box {
    display: grid;
    gap: 8px;
    width: 100%;
    max-width: 480px;
  }

  .cidr-bar {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
  }

  .cidr-in {
    flex: 1 1 160px;
    min-width: 140px;
  }

  .cidr-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .cidr-chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    padding: 2px 6px 2px 8px;
    font-size: 12px;
    border-radius: 6px;
    background: var(--surface-2);
    border: 1px solid var(--border);
  }

  .cidr-del {
    display: grid;
    place-items: center;
    padding: 2px;
    border: 0;
    border-radius: 4px;
    background: transparent;
    color: var(--text-3);
    cursor: pointer;
  }

  .cidr-del:hover:not(:disabled) {
    color: var(--err);
    background: var(--muted-soft);
  }

  .empty-cidr {
    font-size: 12px;
  }

  .mtu {
    width: 100px;
    min-width: 90px;
  }
</style>

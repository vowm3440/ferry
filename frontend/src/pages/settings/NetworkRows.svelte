<script lang="ts">
  import { CAP_AUTOMATION, CAP_SYSTEM_TOOLS, messageOfError, type SettingsValue } from '../../lib/api';
  import Switch from '../../lib/components/Switch.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import { caps } from '../../lib/stores/caps.svelte';
  import { connection } from '../../lib/stores/connection.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import SettingRow from './SettingRow.svelte';

  interface Props {
    value: SettingsValue;
    /** 端口输入框里的原始文本（可能暂时不合法）。 */
    portText: string;
    portInvalid: boolean;
    disabled: boolean;
    onChange: (next: SettingsValue) => void;
    onPortText: (text: string) => void;
  }

  let { value, portText, portInvalid, disabled, onChange, onPortText }: Props = $props();

  let probing = $state(false);
  const canAutomate = $derived(caps.has(CAP_AUTOMATION));
  const canTools = $derived(caps.has(CAP_SYSTEM_TOOLS));

  async function probeInbound(): Promise<void> {
    const backend = connection.backend;
    if (backend === null || probing) return;
    probing = true;
    try {
      const ms = await backend.testInbound();
      toasts.ok(ms === 0 ? tr('混合端口空闲可用', 'Mixed port is available') : tr(`混合入站连通正常（${ms} ms）`, `Mixed inbound reachable (${ms} ms)`));
    } catch (cause: unknown) {
      toasts.error(messageOfError(cause));
    } finally {
      probing = false;
    }
  }
</script>

<SettingRow
  title={tr('混合端口', 'Mixed Port')}
  error={portInvalid ? tr('请输入 1 到 65535 之间的整数', 'Enter an integer between 1 and 65535') : ''}
>
  <div class="port-row">
    <label class="input port" class:invalid={portInvalid}>
      <input
        class="num"
        type="text"
        inputmode="numeric"
        maxlength="5"
        autocomplete="off"
        aria-label={tr('混合端口', 'Mixed Port')}
        aria-invalid={portInvalid}
        value={portText}
        {disabled}
        oninput={(event) => onPortText(event.currentTarget.value)}
      />
    </label>
    {#if canTools}
      <button type="button" class="btn sm" aria-disabled={probing} onclick={() => void probeInbound()}>
        {probing ? tr('探测中…', 'Testing…') : tr('探测端口', 'Test Port')}
      </button>
    {/if}
  </div>
</SettingRow>
<SettingRow title={tr('允许局域网连接', 'Allow LAN Connections')}>
  <Switch
    checked={value.allowLan}
    label={tr('允许局域网连接', 'Allow LAN Connections')}
    {disabled}
    onchange={(next: boolean) => onChange({ ...value, allowLan: next })}
  />
</SettingRow>
<SettingRow
  title={tr('启动时设置系统代理', 'Set System Proxy on Start')}
  desc={canAutomate ? '' : tr('当前版本不提供', 'Unavailable in this backend')}
  tag={canAutomate ? '' : tr('未开放', 'Unavailable')}
>
  <Switch
    checked={value.sysProxyOnStart}
    disabled={!canAutomate || disabled}
    label={canAutomate ? tr('启动时设置系统代理', 'Set System Proxy on Start') : tr('启动时设置系统代理（未开放）', 'Set System Proxy on Start (Unavailable)')}
    onchange={(next: boolean) => onChange({ ...value, sysProxyOnStart: next })}
  />
</SettingRow>

<style>
  .port-row {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .port {
    width: 90px;
    min-width: 90px;
  }
</style>

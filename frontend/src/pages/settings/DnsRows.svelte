<script lang="ts">
  import type { DnsSettings, SettingsValue } from '../../lib/api';
  import Switch from '../../lib/components/Switch.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { dnsStrategyOptions, optionsWith } from '../../lib/settings-view';
  import SettingRow from './SettingRow.svelte';

  interface Props {
    value: SettingsValue;
    disabled: boolean;
    onChange: (next: SettingsValue) => void;
  }

  let { value, disabled, onChange }: Props = $props();

  const serverHint = $derived(
    tr('仅支持 IP 地址（如 https://1.1.1.1/dns-query）', 'IP addresses only (e.g. https://1.1.1.1/dns-query)'),
  );

  const strategies = $derived((void i18n.locale, optionsWith(dnsStrategyOptions(), value.dns.strategy)));

  function setDns(patch: Partial<DnsSettings>): void {
    onChange({ ...value, dns: { ...value.dns, ...patch } });
  }
</script>

<SettingRow title={tr('远程 DNS', 'Remote DNS')} desc={serverHint} wide>
  <label class="input server">
    <input
      type="text"
      class="mono"
      autocomplete="off"
      spellcheck="false"
      aria-label={tr('远程 DNS 服务器', 'Remote DNS Server')}
      value={value.dns.remote}
      {disabled}
      oninput={(event) => setDns({ remote: event.currentTarget.value })}
    />
  </label>
</SettingRow>
<SettingRow title={tr('直连 DNS', 'Direct DNS')} wide>
  <label class="input server">
    <input
      type="text"
      class="mono"
      autocomplete="off"
      spellcheck="false"
      aria-label={tr('直连 DNS 服务器', 'Direct DNS Server')}
      value={value.dns.direct}
      {disabled}
      oninput={(event) => setDns({ direct: event.currentTarget.value })}
    />
  </label>
</SettingRow>
<SettingRow title={tr('解析策略', 'DNS Strategy')}>
  <select
    class="select"
    aria-label={tr('解析策略', 'DNS Strategy')}
    value={value.dns.strategy}
    {disabled}
    onchange={(event) => setDns({ strategy: event.currentTarget.value })}
  >
    {#each strategies as option (option.value)}
      <option value={option.value}>{option.label}</option>
    {/each}
  </select>
</SettingRow>
<SettingRow title="FakeIP">
  <Switch
    checked={value.tun.fakeIp}
    label="FakeIP"
    {disabled}
    onchange={(next: boolean) => onChange({ ...value, tun: { ...value.tun, fakeIp: next } })}
  />
</SettingRow>

<style>
  .server {
    width: 260px;
    min-width: 0;
    max-width: 100%;
  }
</style>

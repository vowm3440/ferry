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
    tr(
      '仅支持使用 IP 地址的 https、tls、tcp、udp 服务器，例如 https://1.1.1.1/dns-query',
      'Supports IP-based https, tls, tcp, or udp servers only (e.g. https://1.1.1.1/dns-query)',
    ),
  );

  const strategies = $derived((void i18n.locale, optionsWith(dnsStrategyOptions(), value.dns.strategy)));

  function setDns(patch: Partial<DnsSettings>): void {
    onChange({ ...value, dns: { ...value.dns, ...patch } });
  }
</script>

<SettingRow title={tr('远程 DNS', 'Remote DNS')} desc={`${tr('经代理出口查询，也是默认的解析服务器。', 'Queried through proxy outbound (default resolver). ')}${serverHint}`} wide>
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
<SettingRow title={tr('直连 DNS', 'Direct DNS')} desc={tr('直连查询使用的解析服务器，格式要求同上', 'Resolver used for direct connections; same IP-only format')} wide>
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
<SettingRow title={tr('解析策略', 'DNS Strategy')} desc={tr('解析结果里 IPv4 与 IPv6 的取舍', 'IPv4 vs IPv6 preference for DNS responses')}>
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
<SettingRow title="FakeIP" desc={tr('开启时 DNS 使用 fakeip 模式，关闭时使用 redirhost 模式', 'Use fakeip DNS mode when enabled, or redirhost when disabled')}>
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

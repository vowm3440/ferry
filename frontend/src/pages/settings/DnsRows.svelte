<script lang="ts">
  import type { DnsRule, DnsServer, DnsSettings, SettingsValue } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { dnsDomainProblem, isValidDnsServerUrl, isValidId, isValidLabel, MAX_DNS_RULES, MAX_DNS_SERVERS } from '../../lib/rules-validate';
  import { parseTokens } from '../../lib/rules-view';
  import { dnsStrategyOptions, optionsWith } from '../../lib/settings-view';
  import SettingRow from './SettingRow.svelte';

  interface Props {
    value: SettingsValue;
    disabled: boolean;
    onChange: (next: SettingsValue) => void;
  }

  let { value, disabled, onChange }: Props = $props();

  let newSrvId = $state('');
  let newSrvName = $state('');
  let newSrvAddr = $state('');
  let newSrvDetour = $state<'direct' | 'proxy'>('direct');
  let srvErr = $state('');

  let newRuleName = $state('');
  let newRuleDomains = $state('');
  let newRuleServer = $state('direct');
  let ruleErr = $state('');

  const serverHint = $derived(
    tr('仅支持 IP 地址（如 https://1.1.1.1/dns-query 或 udp://10.0.0.53）', 'IP addresses only (e.g. https://1.1.1.1/dns-query or udp://10.0.0.53)'),
  );

  const strategies = $derived((void i18n.locale, optionsWith(dnsStrategyOptions(), value.dns.strategy)));
  const servers = $derived(value.dns.servers ?? []);
  const dnsRules = $derived(value.dns.rules ?? []);

  const serverOptions = $derived([
    { value: 'direct', label: tr('直连 DNS（国内 DoH）', 'Direct DNS (Domestic DoH)') },
    { value: 'remote', label: tr('远程 DNS（海外加密）', 'Remote DNS (Encrypted Proxy)') },
    ...servers.map((srv) => ({ value: srv.id, label: `${srv.name} (${srv.id})` })),
  ]);

  function setDns(patch: Partial<DnsSettings>): void {
    onChange({ ...value, dns: { ...value.dns, ...patch } });
  }

  function serverLabel(id: string): string {
    if (id === 'direct') return tr('直连 DNS', 'Direct DNS');
    if (id === 'remote') return tr('远程 DNS', 'Remote DNS');
    return servers.find((srv) => srv.id === id)?.name ?? id;
  }

  function addServer(): void {
    const id = newSrvId.trim();
    const name = newSrvName.trim();
    const address = newSrvAddr.trim();
    if (!isValidId(id) || id === 'direct' || id === 'remote' || servers.some((s) => s.id === id)) {
      srvErr = tr('标识无效或重复（不能使用 direct / remote）', 'Invalid or duplicate server ID');
      return;
    }
    if (!isValidLabel(name)) {
      srvErr = tr('请填写服务器名称', 'Please enter a server name');
      return;
    }
    if (!isValidDnsServerUrl(address)) {
      srvErr = tr('地址须为 IP 字面量的 udp/tcp/tls/https 服务器', 'Address must be a literal-IP udp/tcp/tls/https server');
      return;
    }
    if (servers.length >= MAX_DNS_SERVERS) {
      srvErr = tr(`最多添加 ${MAX_DNS_SERVERS} 个自定义 DNS`, `Maximum ${MAX_DNS_SERVERS} custom DNS servers`);
      return;
    }
    const item: DnsServer = { id, name, address, detour: newSrvDetour };
    setDns({ servers: [...servers, item] });
    newSrvId = '';
    newSrvName = '';
    newSrvAddr = '';
    newSrvDetour = 'direct';
    srvErr = '';
  }

  function removeServer(id: string): void {
    if (dnsRules.some((r) => r.server === id)) {
      srvErr = tr('仍有分流 DNS 规则引用此服务器，请先修改或删除对应规则', 'Server is still referenced by a split DNS rule');
      return;
    }
    srvErr = '';
    setDns({ servers: servers.filter((s) => s.id !== id) });
  }

  function addRule(): void {
    const name = newRuleName.trim();
    const domains = parseTokens(newRuleDomains);
    if (!isValidLabel(name)) {
      ruleErr = tr('请填写分流 DNS 规则名称', 'Please enter a DNS rule name');
      return;
    }
    if (domains.length === 0) {
      ruleErr = tr('请至少填写一个域名或规则集（如 *.corp, geosite:cn）', 'Enter at least one domain or rule set (e.g. *.corp, geosite:cn)');
      return;
    }
    for (const d of domains) {
      const problem = dnsDomainProblem(d, null);
      if (problem !== '') {
        ruleErr = `${d}：${problem}`;
        return;
      }
    }
    if (dnsRules.length >= MAX_DNS_RULES) {
      ruleErr = tr(`分流 DNS 规则最多 ${MAX_DNS_RULES} 条`, `Maximum ${MAX_DNS_RULES} split DNS rules`);
      return;
    }
    const id = `dns-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;
    const nextRule: DnsRule = { id, name, enabled: true, domains, server: newRuleServer };
    setDns({ rules: [...dnsRules, nextRule] });
    newRuleName = '';
    newRuleDomains = '';
    ruleErr = '';
  }

  function toggleRule(id: string, enabled: boolean): void {
    setDns({ rules: dnsRules.map((r) => (r.id === id ? { ...r, enabled } : r)) });
  }

  function removeRule(id: string): void {
    setDns({ rules: dnsRules.filter((r) => r.id !== id) });
  }

  function quickIntranetTemplate(): void {
    if (!servers.some((s) => s.id === 'corp-dns')) {
      newSrvId = 'corp-dns';
      newSrvName = tr('公司内网 DNS', 'Corporate DNS');
      newSrvAddr = 'udp://10.0.0.53';
      newSrvDetour = 'direct';
    }
    newRuleName = tr('公司内网定向解析', 'Corporate Intranet DNS');
    newRuleDomains = '*.corp, *.local, *.lan, *.internal';
    newRuleServer = servers.some((s) => s.id === 'corp-dns') ? 'corp-dns' : 'direct';
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
<SettingRow title={tr('直连 DNS', 'Direct DNS')} desc={tr('用于国内域名 DoH 解析及默认直连解析', 'Domestic DoH resolver for direct traffic')} wide>
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
<SettingRow title="FakeIP" desc={tr('分流 DNS 规则优先于 FakeIP，保证内网与国内域名获取真实 IP', 'Split DNS rules run before FakeIP so intranet & domestic domains resolve real IPs')}>
  <Switch
    checked={value.tun.fakeIp}
    label="FakeIP"
    {disabled}
    onchange={(next: boolean) => onChange({ ...value, tun: { ...value.tun, fakeIp: next } })}
  />
</SettingRow>

<div class="split-block">
  <div class="block-head">
    <div>
      <b>{tr('自定义 / 内网 DNS 上游', 'Custom / Intranet DNS Servers')}</b>
      <span class="muted">{tr('可添加公司内网 DNS（如 udp://10.0.0.53）或额外 DoH 上游', 'Add intranet DNS (e.g. udp://10.0.0.53) or extra DoH resolvers')}</span>
    </div>
    <button type="button" class="btn sm ghost" {disabled} onclick={quickIntranetTemplate}>
      <Icon name="zap" />{tr('填入内网模板', 'Intranet Template')}
    </button>
  </div>

  {#if servers.length > 0}
    <div class="list">
      {#each servers as srv (srv.id)}
        <div class="item">
          <span class="chip">{srv.id}</span>
          <b class="item-name">{srv.name}</b>
          <span class="mono muted item-addr">{srv.address}</span>
          <span class="pill">{srv.detour === 'proxy' ? tr('代理', 'Proxy') : tr('直连', 'Direct')}</span>
          <button type="button" class="icon-btn" {disabled} aria-label={tr(`删除 DNS「${srv.name}」`, `Remove DNS "${srv.name}"`)} onclick={() => removeServer(srv.id)}>
            <Icon name="trash" />
          </button>
        </div>
      {/each}
    </div>
  {/if}

  <div class="add-grid">
    <input class="control mono" placeholder={tr('标识 (corp-dns)', 'ID (corp-dns)')} {disabled} value={newSrvId} oninput={(e) => (newSrvId = e.currentTarget.value)} />
    <input class="control" placeholder={tr('名称 (公司内网 DNS)', 'Name (Corp DNS)')} {disabled} value={newSrvName} oninput={(e) => (newSrvName = e.currentTarget.value)} />
    <input class="control mono" placeholder="udp://10.0.0.53" {disabled} value={newSrvAddr} oninput={(e) => (newSrvAddr = e.currentTarget.value)} />
    <select class="select" {disabled} value={newSrvDetour} onchange={(e) => (newSrvDetour = e.currentTarget.value === 'proxy' ? 'proxy' : 'direct')}>
      <option value="direct">{tr('直连', 'Direct')}</option>
      <option value="proxy">{tr('代理', 'Proxy')}</option>
    </select>
    <button type="button" class="btn sm" {disabled} onclick={addServer}><Icon name="plus" />{tr('添加', 'Add')}</button>
  </div>
  {#if srvErr !== ''}
    <div class="err-line" role="alert">{srvErr}</div>
  {/if}
</div>

<div class="split-block">
  <div class="block-head">
    <div>
      <b>{tr('分流 DNS 规则（域名 / 规则集定向解析）', 'Split DNS Rules (Domain / Rule-Set Routing)')}</b>
      <span class="muted">{tr('自上而下匹配：如 *.corp, *.local 走内网 DNS，geosite:cn 走直连 DoH，其余走远程防泄露', 'Top-down: e.g. *.corp -> intranet DNS, geosite:cn -> direct DoH, fallback -> remote')}</span>
    </div>
  </div>

  {#if dnsRules.length > 0}
    <div class="list">
      {#each dnsRules as r (r.id)}
        <div class="item" class:off={!r.enabled}>
          <Switch checked={r.enabled} {disabled} label={tr(`启用分流 DNS「${r.name}」`, `Enable split DNS "${r.name}"`)} onchange={(on) => toggleRule(r.id, on)} />
          <b class="item-name">{r.name}</b>
          <span class="mono muted item-addr" title={r.domains.join(', ')}>{r.domains.join(', ')}</span>
          <span class="pill">{serverLabel(r.server)}</span>
          <button type="button" class="icon-btn" {disabled} aria-label={tr(`删除分流 DNS「${r.name}」`, `Remove split DNS "${r.name}"`)} onclick={() => removeRule(r.id)}>
            <Icon name="trash" />
          </button>
        </div>
      {/each}
    </div>
  {/if}

  <div class="add-grid rules">
    <input class="control" placeholder={tr('规则名称（如：内网域名）', 'Rule name (e.g. Intranet)')} {disabled} value={newRuleName} oninput={(e) => (newRuleName = e.currentTarget.value)} />
    <input class="control mono" placeholder={tr('*.corp, *.local 或 geosite:cn, ruleset:apple-cn', '*.corp, *.local or geosite:cn')} {disabled} value={newRuleDomains} oninput={(e) => (newRuleDomains = e.currentTarget.value)} />
    <select class="select" {disabled} value={newRuleServer} onchange={(e) => (newRuleServer = e.currentTarget.value)}>
      {#each serverOptions as opt (opt.value)}
        <option value={opt.value}>{opt.label}</option>
      {/each}
    </select>
    <button type="button" class="btn sm" {disabled} onclick={addRule}><Icon name="plus" />{tr('添加规则', 'Add Rule')}</button>
  </div>
  {#if ruleErr !== ''}
    <div class="err-line" role="alert">{ruleErr}</div>
  {/if}
</div>

<style>
  .server {
    width: 260px;
    min-width: 0;
    max-width: 100%;
  }

  .split-block {
    padding: 12px 16px;
    border-top: 1px solid var(--border);
    display: grid;
    gap: 10px;
  }

  .block-head {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }

  .block-head b {
    display: block;
    font-size: 13px;
    font-weight: 500;
  }

  .block-head span {
    display: block;
    font-size: 12px;
  }

  .list {
    display: grid;
    gap: 6px;
  }

  .item {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    padding: 6px 10px;
    border-radius: 7px;
    background: var(--muted-soft);
    font-size: 12.5px;
  }

  .item.off {
    opacity: 0.5;
  }

  .item-name {
    font-weight: 500;
  }

  .item-addr {
    flex: 1 1 160px;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
  }

  .add-grid {
    display: grid;
    grid-template-columns: minmax(90px, 0.8fr) minmax(110px, 1fr) minmax(140px, 1.3fr) auto auto;
    gap: 6px;
    align-items: center;
  }

  .add-grid.rules {
    grid-template-columns: minmax(110px, 0.9fr) minmax(160px, 1.6fr) auto auto;
  }

  .add-grid .control,
  .add-grid .select {
    min-width: 0;
    width: 100%;
  }

  .err-line {
    font-size: 12px;
    color: var(--err);
  }

  @container content (max-width: 720px) {
    .add-grid,
    .add-grid.rules {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>

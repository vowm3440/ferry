<!--
  一键懒人分流预设：自动写入社区 SRS 规则集、应用分组、黄金顺序分流规则与防泄露分流 DNS。
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import type { LazyPresetOptions } from '../../lib/api';
  import Dialog from '../../lib/components/Dialog.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import { isValidDnsServerUrl } from '../../lib/rules-validate';
  import { parseTokens } from '../../lib/rules-view';
  import { rules } from '../../lib/stores/rules.svelte';
  import { settings } from '../../lib/stores/settings.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';

  interface Props {
    onclose: () => void;
  }

  let { onclose }: Props = $props();

  const uid = $props.id();

  let replaceExisting = $state(true);
  let enableAds = $state(true);
  let enableAi = $state(true);
  let enableAppleSteam = $state(true);
  let enableCnSplit = $state(true);
  let enableSplitDns = $state(true);
  let intranetDns = $state('');
  let intranetDomainsText = $state('*.corp, *.local, *.lan, *.internal, *.home.arpa');
  let downloadAfter = $state(true);
  let applying = $state(false);
  let error = $state('');

  onMount(() => {
    void rules.defaultLazyPreset().then((res) => {
      if (!res.ok) return;
      const d = res.value;
      replaceExisting = d.replaceExisting;
      enableAds = d.enableAds;
      enableAi = d.enableAi;
      enableAppleSteam = d.enableAppleSteam;
      enableCnSplit = d.enableCnSplit;
      enableSplitDns = d.enableSplitDns;
      intranetDns = d.intranetDns;
      if (d.intranetDomains.length > 0) intranetDomainsText = d.intranetDomains.join(', ');
    });
  });

  function close(): void {
    if (!applying) onclose();
  }

  async function submit(event?: SubmitEvent): Promise<void> {
    event?.preventDefault();
    if (applying) return;
    const dnsAddr = intranetDns.trim();
    if (dnsAddr !== '' && !isValidDnsServerUrl(dnsAddr)) {
      error = tr('内网 DNS 地址无效（示例：udp://10.0.0.53 或 tcp://192.168.1.1）', 'Invalid intranet DNS URL (e.g. udp://10.0.0.53)');
      return;
    }
    const opts: LazyPresetOptions = {
      replaceExisting,
      enableAds,
      enableAi,
      enableAppleSteam,
      enableCnSplit,
      enableSplitDns,
      intranetDns: dnsAddr,
      intranetDomains: parseTokens(intranetDomainsText),
    };
    error = '';
    applying = true;
    const res = await rules.applyLazyPreset(opts);
    if (!res.ok) {
      applying = false;
      error = res.message;
      return;
    }
    // 刷新设置缓存，让设置页同步展示写入的分流 DNS。
    void settings.load();
    const { ruleCount, ruleSetCount, appGroupCount, missingRuleSets } = res.value;
    if (downloadAfter && missingRuleSets.length > 0) {
      const dl = await rules.updateSets(missingRuleSets);
      applying = false;
      if (!dl.ok) {
        toasts.info(
          tr(
            `已应用懒人预设（${ruleCount} 条规则），部分规则集下载失败，请到「规则集」页手动更新`,
            `Applied lazy preset (${ruleCount} rules), but some rule sets failed to download; retry in Rule Sets tab`,
          ),
        );
        onclose();
        return;
      }
    } else {
      applying = false;
    }
    toasts.ok(
      tr(
        `已应用懒人分流预设：${ruleCount} 条规则、${ruleSetCount} 个社区规则集、${appGroupCount} 个应用分组`,
        `Applied lazy split-routing preset: ${ruleCount} rules, ${ruleSetCount} community rule sets, ${appGroupCount} app groups`,
      ),
    );
    onclose();
  }
</script>

<Dialog open title={tr('一键懒人分流预设', 'One-Click Lazy Split-Routing Preset')} busy={applying} width={580} backdropClose={false} onclose={close}>
  <form id={`${uid}-form`} class="form" novalidate onsubmit={(event) => void submit(event)}>
    <div class="inline-note">
      <Icon name="zap" />
      <span>{tr('一键配置黄金顺序分流规则、常用社区 SRS 规则集（24h 自动刷新）、实用应用分组与防泄露分流 DNS。', 'Configures curated split rules, community SRS sets (24h auto-refresh), app groups, and anti-leak split DNS in one click.')}</span>
    </div>

    <div class="opts">
      <label class="opt">
        <Switch checked={enableAds} label={tr('去广告拦截', 'Ad & tracker blocking')} onchange={(v) => (enableAds = v)} />
        <span><b>{tr('广告与跟踪拦截', 'Ad & Tracker Blocking')}</b><small class="muted">{tr('接入 community ads SRS，拦截常见广告域名', 'Blocks ads/trackers via ruleset:ads')}</small></span>
      </label>
      <label class="opt">
        <Switch checked={enableAi} label={tr('AI 与开发工具', 'AI & Dev Tools')} onchange={(v) => (enableAi = v)} />
        <span><b>{tr('AI 服务与开发工具代理', 'AI Services & Dev Tools')}</b><small class="muted">{tr('接入 openai / anthropic SRS + Cursor / VS Code / git 应用组走代理', 'Routes openai/anthropic + Cursor/Code/git group via proxy')}</small></span>
      </label>
      <label class="opt">
        <Switch checked={enableAppleSteam} label={tr('Apple / Steam 国内直连', 'Apple / Steam CN Direct')} onchange={(v) => (enableAppleSteam = v)} />
        <span><b>{tr('Apple / Steam 国内直连', 'Apple & Steam CN Direct')}</b><small class="muted">{tr('接入 apple-cn / steam-cn SRS + 游戏平台应用组，下载满速直连', 'Direct routing for apple-cn & steam-cn CDNs')}</small></span>
      </label>
      <label class="opt">
        <Switch checked={enableCnSplit} label={tr('国内直连 / 海外代理', 'CN Direct / Global Proxy')} onchange={(v) => (enableCnSplit = v)} />
        <span><b>{tr('国内外智能分流', 'Domestic Direct / Overseas Proxy')}</b><small class="muted">{tr('geosite:cn + geoip:cn 直连，geosite:geolocation-!cn 走代理', 'geosite:cn & geoip:cn direct; non-CN proxied')}</small></span>
      </label>
      <label class="opt">
        <Switch checked={enableSplitDns} label={tr('分流 DNS', 'Split DNS')} onchange={(v) => (enableSplitDns = v)} />
        <span><b>{tr('防泄露分流 DNS', 'Anti-Leak Split DNS')}</b><small class="muted">{tr('内网域名定向解析、国内域名走 DoH、海外走远程加密 DNS', 'Intranet DNS for internal domains, DoH for CN, encrypted remote DNS for global')}</small></span>
      </label>
    </div>

    <div class="row">
      <label class="field grow">
        <span class="label">{tr('公司内网 DNS（可选）', 'Intranet DNS (Optional)')}</span>
        <input
          class="control mono"
          autocomplete="off"
          spellcheck="false"
          placeholder="udp://10.0.0.53"
          value={intranetDns}
          oninput={(event) => (intranetDns = event.currentTarget.value)}
        />
        <span class="muted hint">{tr('留空则内网域名默认走直连 DNS', 'Leave blank to use Direct DNS for internal domains')}</span>
      </label>
      <label class="field grow">
        <span class="label">{tr('内网域名后缀', 'Intranet Domain Suffixes')}</span>
        <input
          class="control mono"
          autocomplete="off"
          spellcheck="false"
          value={intranetDomainsText}
          oninput={(event) => (intranetDomainsText = event.currentTarget.value)}
        />
      </label>
    </div>

    <div class="row toggles">
      <label class="check-row">
        <input type="checkbox" checked={replaceExisting} onchange={(event) => (replaceExisting = event.currentTarget.checked)} />
        <span>{tr('替换现有普通规则（取消勾选则保留并合并）', 'Replace existing user rules (uncheck to merge)')}</span>
      </label>
      <label class="check-row">
        <input type="checkbox" checked={downloadAfter} onchange={(event) => (downloadAfter = event.currentTarget.checked)} />
        <span>{tr('应用后自动下载缺失的 SRS 规则集', 'Auto-download missing SRS rule sets after applying')}</span>
      </label>
    </div>

    {#if error !== ''}
      <div class="inline-error" role="alert"><Icon name="alert" /><span class="selectable">{error}</span></div>
    {/if}
  </form>

  {#snippet footer()}
    <button type="button" class="btn" aria-disabled={applying} onclick={close}>{tr('取消', 'Cancel')}</button>
    <button type="submit" class="btn primary" form={`${uid}-form`} aria-disabled={applying}>
      <Icon name="zap" />{applying ? tr('正在应用…', 'Applying…') : tr('立即应用预设', 'Apply Preset')}
    </button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: 12px;
  }

  .opts {
    display: grid;
    gap: 8px;
    padding: 10px 12px;
    border-radius: 8px;
    background: var(--muted-soft);
  }

  .opt {
    display: flex;
    align-items: flex-start;
    gap: 10px;
  }

  .opt b {
    display: block;
    font-size: 13px;
    font-weight: 500;
  }

  .opt small {
    display: block;
    font-size: 12px;
  }

  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
  }

  .grow {
    flex: 1 1 220px;
  }

  .hint {
    font-size: 11.5px;
  }

  .toggles {
    flex-direction: column;
    gap: 6px;
  }

  .check-row {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12.5px;
    cursor: pointer;
  }

  .check-row input {
    width: 15px;
    height: 15px;
    accent-color: var(--accent);
  }
</style>

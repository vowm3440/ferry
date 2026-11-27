<script lang="ts">
  import { tick, untrack } from 'svelte';
  import { CAP_AUTOMATION, CAP_PERSIST, cloneSettings, type SettingsValue } from '../lib/api';
  import EmptyState from '../lib/components/EmptyState.svelte';
  import Icon from '../lib/components/Icon.svelte';
  import { i18n, tr } from '../lib/i18n/index.svelte';
  import { watchSections } from '../lib/scrollspy';
  import { parsePort, sameSettings, userAgentError, withAutomationOff } from '../lib/settings-view';
  import { caps } from '../lib/stores/caps.svelte';
  import { connection } from '../lib/stores/connection.svelte';
  import { core } from '../lib/stores/core.svelte';
  import { settings } from '../lib/stores/settings.svelte';
  import { theme } from '../lib/stores/theme.svelte';
  import { toasts } from '../lib/stores/toasts.svelte';
  import { persistenceNote } from '../lib/system-view';
  import AboutRows from './settings/AboutRows.svelte';
  import BackupRows from './settings/BackupRows.svelte';
  import CoreRows from './settings/CoreRows.svelte';
  import DevImport from './settings/DevImport.svelte';
  import DnsRows from './settings/DnsRows.svelte';
  import GeneralRows from './settings/GeneralRows.svelte';
  import NetworkRows from './settings/NetworkRows.svelte';
  import SaveBar from './settings/SaveBar.svelte';
  import SettingSection from './settings/SettingSection.svelte';
  import SubsRows from './settings/SubsRows.svelte';
  import TunRows from './settings/TunRows.svelte';

  interface SectionLink {
    readonly id: string;
    readonly label: string;
  }

  const sections = $derived<readonly SectionLink[]>([
    { id: 'set-general', label: tr('常规', 'General') },
    { id: 'set-network', label: tr('网络与代理', 'Network & Proxy') },
    { id: 'set-dns', label: 'DNS' },
    { id: 'set-core', label: tr('内核', 'Core') },
    { id: 'set-tun', label: tr('TUN 与服务', 'TUN & Service') },
    { id: 'set-subs', label: tr('订阅与规则集', 'Subs & Rule Sets') },
    { id: 'set-backup', label: tr('备份与诊断', 'Backup & Diagnostics') },
    { id: 'set-dev', label: tr('开发配置', 'Dev Config') },
    { id: 'set-about', label: tr('关于', 'About') },
  ]);
  const SECTION_IDS: readonly string[] = [
    'set-general',
    'set-network',
    'set-dns',
    'set-core',
    'set-tun',
    'set-subs',
    'set-backup',
    'set-dev',
    'set-about',
  ];
  /** 点击目录后的一小段时间内不让滚动定位改写高亮，等平滑滚动结束。 */
  const PIN_MS = 600;

  let pageEl = $state<HTMLElement | null>(null);
  let saveBar = $state<{ focusSave: () => void } | null>(null);
  /** 页面内的草稿；保存前不会影响后端。 */
  let draft = $state.raw<SettingsValue | null>(null);
  /** 端口输入框的原始文本，允许暂时不合法。 */
  let portText = $state('');
  let saveError = $state('');
  let active = $state('set-general');
  // 只在滚动回调里比较时间，不参与渲染，所以不做成响应式状态。
  let pinnedUntil = 0;

  const loaded = $derived(draft !== null);
  const port = $derived(parsePort(portText));
  const portInvalid = $derived(loaded && port === null);
  const next = $derived.by<SettingsValue | null>(() =>
    draft === null || port === null ? null : { ...draft, mixedPort: port },
  );
  const dirty = $derived.by<boolean>(() => {
    const base = settings.value;
    if (draft === null || base === null) return false;
    return next === null || !sameSettings(next, base);
  });
  /** 后端在内核运行时拒绝所有设置修改。 */
  const locked = $derived(core.running || core.transitioning);
  const uaError = $derived((void i18n.locale, draft === null ? '' : userAgentError(draft.subscriptionUA)));
  const canSave = $derived(dirty && next !== null && !locked && uaError === '');
  /** 设置保存在哪里：由后端公布的能力决定，不靠猜。 */
  const storageNote = $derived(
    (void i18n.locale, persistenceNote({ capsLoaded: caps.loaded, persistent: caps.has(CAP_PERSIST), demo: connection.isDemo })),
  );
  /** 内核正在处理别的操作：按钮暂不响应，但不禁用，以免丢掉键盘焦点。 */
  const coreBusy = $derived(core.pending !== null || core.transitioning);

  // 后端取值变化时（首次读取、保存后重新读取、导入配置后）重建草稿。
  $effect(() => {
    const value = settings.value;
    untrack(() => resetDraft(value));
  });

  // 滚动时高亮当前分区。
  $effect(() => {
    if (!loaded || pageEl === null) return;
    const scroller = pageEl.closest<HTMLElement>('.content');
    if (scroller === null) return;
    return watchSections(scroller, SECTION_IDS, (id) => {
      if (performance.now() >= pinnedUntil) active = id;
    });
  });

  function resetDraft(value: SettingsValue | null): void {
    draft = value === null ? null : cloneSettings(value);
    portText = value === null ? '' : String(value.mixedPort);
    saveError = '';
  }

  function setDraft(value: SettingsValue): void {
    draft = value;
    saveError = '';
  }

  function setPortText(text: string): void {
    portText = text;
    saveError = '';
  }

  function jump(id: string): void {
    const section = document.getElementById(id);
    if (section === null) return;
    active = id;
    pinnedUntil = performance.now() + PIN_MS;
    const reduceMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    section.scrollIntoView({ behavior: reduceMotion ? 'auto' : 'smooth', block: 'start' });
    section.querySelector('h2')?.focus({ preventScroll: true });
  }

  /**
   * 保存栏里的按钮会随状态消失；键盘焦点原本在它上面时（此时落在 body 上），
   * 交给下一步最可能用到的位置，键盘用户不必从页面开头重新找回来。
   */
  async function refocus(target: () => HTMLElement | null | undefined): Promise<void> {
    await tick();
    if (document.activeElement === document.body) target()?.focus({ preventScroll: true });
  }

  /** 保存栏消失后回到正在查看的分区。 */
  function refocusSection(): Promise<void> {
    return refocus(() => document.getElementById(active)?.querySelector('h2'));
  }

  function reload(): void {
    if (settings.loading) return;
    void settings.load();
  }

  function revert(): void {
    resetDraft(settings.value);
    void refocusSection();
  }

  async function save(): Promise<void> {
    const value = next;
    if (value === null || settings.saving || uaError !== '') return;
    saveError = '';
    const candidate: SettingsValue = { ...value, theme: theme.choice, language: i18n.locale };
    const payload = caps.has(CAP_AUTOMATION) ? candidate : withAutomationOff(candidate);
    const result = await settings.save(payload);
    if (!result.ok) {
      saveError = result.message;
      return;
    }
    if (settings.error !== '') {
      saveError = tr(`设置已提交，但重新读取失败：${settings.error}`, `Settings saved, but reload failed: ${settings.error}`);
      return;
    }
    toasts.ok(tr('设置已保存', 'Settings saved'));
    await refocusSection();
  }

  async function stopCore(): Promise<void> {
    if (coreBusy) return;
    const result = await core.stop();
    if (!result.ok) {
      toasts.error(result.message);
      return;
    }
    // 「停止内核」按钮随即消失，下一步通常就是保存。
    await tick();
    if (document.activeElement === document.body) saveBar?.focusSave();
  }
</script>

<section class="page" aria-labelledby="settings-title" bind:this={pageEl}>
  <div class="page-head">
    <h1 id="settings-title">{tr('设置', 'Settings')}</h1>
  </div>

  {#if draft === null}
    <div class="card">
      {#if settings.error !== ''}
        <EmptyState icon="alert" tone="error" title={tr('无法读取设置', 'Failed to Load Settings')} description={settings.error}>
          <button type="button" class="btn" aria-disabled={settings.loading} onclick={reload}>
            {settings.loading ? tr('正在重试…', 'Retrying…') : tr('重试', 'Retry')}
          </button>
        </EmptyState>
      {:else}
        <EmptyState icon="settings" title={tr('正在读取设置…', 'Loading settings…')} description={tr('正在向桌面后端请求当前设置。', 'Requesting settings from backend.')} />
      {/if}
    </div>
  {:else}
    {@const value = draft}
    {#if storageNote !== ''}
      <div class="inline-note session" data-note="storage">
        <Icon name="info" />
        <span>{storageNote}</span>
      </div>
    {/if}

    <div class="set-wrap">
      <nav class="set-nav" aria-label={tr('设置分区', 'Settings sections')}>
        {#each sections as section (section.id)}
          <button
            type="button"
            class="nav-item"
            class:on={active === section.id}
            aria-current={active === section.id ? 'true' : undefined}
            onclick={() => jump(section.id)}
          >
            {section.label}
          </button>
        {/each}
      </nav>

      <div class="set-body">
        <SettingSection id="set-general" title={tr('常规', 'General')}>
          <GeneralRows {value} disabled={settings.saving} onChange={setDraft} />
        </SettingSection>
        <SettingSection id="set-network" title={tr('网络与代理', 'Network & Proxy')}>
          <NetworkRows
            {value}
            {portText}
            {portInvalid}
            disabled={settings.saving}
            onChange={setDraft}
            onPortText={setPortText}
          />
        </SettingSection>
        <SettingSection id="set-dns" title="DNS">
          <DnsRows {value} disabled={settings.saving} onChange={setDraft} />
        </SettingSection>
        <SettingSection id="set-core" title={tr('内核', 'Core')}>
          <CoreRows {value} disabled={settings.saving} onChange={setDraft} />
        </SettingSection>
        <SettingSection id="set-tun" title={tr('TUN 与服务', 'TUN & Service')}>
          <TunRows {value} disabled={settings.saving} onChange={setDraft} />
        </SettingSection>
        <SettingSection id="set-subs" title={tr('订阅与规则集', 'Subs & Rule Sets')}>
          <SubsRows {value} {uaError} disabled={settings.saving} onChange={setDraft} />
        </SettingSection>
        <SettingSection id="set-backup" title={tr('备份与诊断', 'Backup & Diagnostics')}>
          <BackupRows />
        </SettingSection>
        <SettingSection id="set-dev" title={tr('开发配置', 'Dev Config')}>
          <DevImport />
        </SettingSection>
        <SettingSection id="set-about" title={tr('关于', 'About')}>
          <AboutRows channel={value.updateChannel} onChannel={(ch) => setDraft({ ...value, updateChannel: ch })} />
        </SettingSection>
      </div>
    </div>

    {#if dirty || saveError !== ''}
      <SaveBar
        bind:this={saveBar}
        {saveError}
        {portInvalid}
        {locked}
        {coreBusy}
        stopping={core.pending === 'stop'}
        {canSave}
        saving={settings.saving}
        onStop={() => void stopCore()}
        onRevert={revert}
        onSave={() => void save()}
      />
    {/if}
  {/if}
</section>

<style>
  .session {
    margin-bottom: 14px;
  }

  .set-wrap {
    display: grid;
    grid-template-columns: 168px minmax(0, 1fr);
    gap: 18px;
    align-items: start;
  }

  .set-nav {
    display: flex;
    flex-direction: column;
    gap: 2px;
    position: sticky;
    top: 0;
  }

  .nav-item {
    padding: 7px 10px;
    border: 0;
    border-radius: 7px;
    background: none;
    color: var(--text-2);
    text-align: left;
    cursor: pointer;
    transition:
      background var(--dur) var(--ease),
      color var(--dur) var(--ease);
  }

  .nav-item:hover,
  .nav-item.on {
    background: var(--muted-soft);
    color: var(--text);
  }

  .nav-item.on {
    font-weight: 500;
  }

  .set-body {
    min-width: 0;
  }

  @container content (max-width: 720px) {
    .set-wrap {
      grid-template-columns: minmax(0, 1fr);
      gap: 12px;
    }

    .set-nav {
      position: static;
      flex-direction: row;
      flex-wrap: wrap;
    }
  }
</style>

<script lang="ts">
  import { CAP_AUTOMATION, CAP_DEEPLINK_REGISTER, type SettingsValue } from '../../lib/api';
  import Segmented from '../../lib/components/Segmented.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { i18n, LOCALE_OPTIONS, tr, type Locale } from '../../lib/i18n/index.svelte';
  import { caps } from '../../lib/stores/caps.svelte';
  import { system } from '../../lib/stores/system.svelte';
  import { theme, themeOptions, type ThemeChoice } from '../../lib/stores/theme.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import SettingRow from './SettingRow.svelte';

  interface Props {
    value: SettingsValue;
    disabled?: boolean;
    onChange?: (next: SettingsValue) => void;
  }

  let { value, disabled = false, onChange }: Props = $props();

  const NOT_OPEN = $derived(tr('未开放', 'Unavailable'));
  const canAutomate = $derived(caps.has(CAP_AUTOMATION));
  const canDeepLinkReg = $derived(caps.has(CAP_DEEPLINK_REGISTER));

  function update(patch: Partial<SettingsValue>): void {
    onChange?.({ ...value, ...patch });
  }

  function setLocale(next: Locale): void {
    i18n.set(next);
    update({ language: next });
  }

  async function toggleDeepLink(next: boolean): Promise<void> {
    const result = await system.setDeepLinkRegistered(next);
    if (result.ok) {
      toasts.ok(next ? tr('已注册 ferry:// 系统深链协议', 'Registered ferry:// protocol handler') : tr('已取消 ferry:// 系统深链关联', 'Unregistered ferry:// protocol handler'));
    } else {
      toasts.error(result.message);
    }
  }
</script>

<SettingRow
  title={tr('主题', 'Theme')}
  desc={tr('立即生效并保存在本机；保存设置时会一并写入后端', 'Applies immediately and persists locally; also written to backend on save')}
  wide
>
  <Segmented
    options={themeOptions()}
    value={theme.choice}
    label={tr('主题', 'Theme')}
    onchange={(next: ThemeChoice) => theme.set(next)}
  />
</SettingRow>
<SettingRow
  title={tr('开机自启', 'Launch at Startup')}
  desc={canAutomate
    ? tr('登录 Windows 后自动启动 Ferry（写入当前用户启动项，无需管理员权限）', 'Automatically launch Ferry when signing in to Windows (per-user Run key)')
    : tr('登录 Windows 后自动启动 Ferry；当前版本不提供', 'Automatically launch Ferry on sign-in; unavailable in this backend')}
  tag={canAutomate ? '' : NOT_OPEN}
>
  <Switch
    checked={value.autoStart}
    disabled={!canAutomate || disabled}
    label={canAutomate ? tr('开机自启', 'Launch at Startup') : tr('开机自启（未开放）', 'Launch at Startup (Unavailable)')}
    onchange={(next: boolean) => update({ autoStart: next })}
  />
</SettingRow>
<SettingRow
  title={tr('启动时最小化', 'Start Minimized')}
  desc={canAutomate
    ? tr('启动后直接收进系统托盘，不弹出主窗口', 'Start directly in the system tray without showing the main window')
    : tr('启动后不显示主窗口；当前版本不提供', 'Start without showing the main window; unavailable in this backend')}
  tag={canAutomate ? '' : NOT_OPEN}
>
  <Switch
    checked={value.startMinimized}
    disabled={!canAutomate || disabled}
    label={canAutomate ? tr('启动时最小化', 'Start Minimized') : tr('启动时最小化（未开放）', 'Start Minimized (Unavailable)')}
    onchange={(next: boolean) => update({ startMinimized: next })}
  />
</SettingRow>
<SettingRow
  title={tr('轻量模式', 'Lightweight Mode')}
  desc={canAutomate
    ? tr('窗口收进托盘后降低后台轮询频率并减少界面占用', 'Reduce background polling and memory footprint when minimized to tray')
    : tr('关闭窗口时释放界面占用的内存，只保留托盘与内核；当前版本不提供', 'Release UI overhead when hidden to tray; unavailable in this backend')}
  tag={canAutomate ? '' : NOT_OPEN}
>
  <Switch
    checked={value.lightweight}
    disabled={!canAutomate || disabled}
    label={canAutomate ? tr('轻量模式', 'Lightweight Mode') : tr('轻量模式（未开放）', 'Lightweight Mode (Unavailable)')}
    onchange={(next: boolean) => update({ lightweight: next })}
  />
</SettingRow>
<SettingRow
  title={tr('语言', 'Language')}
  desc={tr('支持简体中文与 English，切换后立即生效', 'Supports Simplified Chinese and English; applies immediately')}
  wide
>
  <Segmented
    options={LOCALE_OPTIONS}
    value={i18n.locale}
    label={tr('语言', 'Language')}
    onchange={(next: Locale) => setLocale(next)}
  />
</SettingRow>
{#if canDeepLinkReg}
  <SettingRow
    title={tr('系统深链协议（ferry://）', 'Protocol Handler (ferry://)')}
    desc={tr(
      '在当前 Windows 用户下注册 ferry:// 链接关联（免管理员权限，适合便携版一键导入）',
      'Register ferry:// URL handler under current Windows user (no UAC needed for portable builds)',
    )}
  >
    <Switch
      checked={system.deepLinkRegistered}
      disabled={system.deepLinkBusy}
      label={tr('系统深链协议', 'Protocol Handler')}
      onchange={(next: boolean) => void toggleDeepLink(next)}
    />
  </SettingRow>
{/if}

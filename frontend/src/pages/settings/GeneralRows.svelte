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

<SettingRow title={tr('主题', 'Theme')} wide>
  <Segmented
    options={themeOptions()}
    value={theme.choice}
    label={tr('主题', 'Theme')}
    onchange={(next: ThemeChoice) => theme.set(next)}
  />
</SettingRow>
<SettingRow
  title={tr('开机自启', 'Launch at Startup')}
  desc={canAutomate ? '' : tr('当前版本不提供', 'Unavailable in this backend')}
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
  desc={canAutomate ? '' : tr('当前版本不提供', 'Unavailable in this backend')}
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
  desc={canAutomate ? tr('收进托盘后释放界面内存', 'Release UI memory when minimized to tray') : tr('当前版本不提供', 'Unavailable in this backend')}
  tag={canAutomate ? '' : NOT_OPEN}
>
  <Switch
    checked={value.lightweight}
    disabled={!canAutomate || disabled}
    label={canAutomate ? tr('轻量模式', 'Lightweight Mode') : tr('轻量模式（未开放）', 'Lightweight Mode (Unavailable)')}
    onchange={(next: boolean) => update({ lightweight: next })}
  />
</SettingRow>
<SettingRow title={tr('语言', 'Language')} wide>
  <Segmented
    options={LOCALE_OPTIONS}
    value={i18n.locale}
    label={tr('语言', 'Language')}
    onchange={(next: Locale) => setLocale(next)}
  />
</SettingRow>
{#if canDeepLinkReg}
  <SettingRow title={tr('系统深链协议（ferry://）', 'Protocol Handler (ferry://)')}>
    <Switch
      checked={system.deepLinkRegistered}
      disabled={system.deepLinkBusy}
      label={tr('系统深链协议', 'Protocol Handler')}
      onchange={(next: boolean) => void toggleDeepLink(next)}
    />
  </SettingRow>
{/if}

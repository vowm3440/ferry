<script lang="ts">
  import type { SettingsValue } from '../../lib/api';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { logLevelOptions, optionsWith } from '../../lib/settings-view';
  import SettingRow from './SettingRow.svelte';

  interface Props {
    value: SettingsValue;
    disabled: boolean;
    onChange: (next: SettingsValue) => void;
  }

  let { value, disabled, onChange }: Props = $props();

  const levels = $derived((void i18n.locale, optionsWith(logLevelOptions(), value.logLevel)));
  const apiModeLabel = $derived(value.clashApiMode === 'random' ? tr('随机端口', 'Random Port') : value.clashApiMode);
</script>

<SettingRow title={tr('日志级别', 'Log Level')}>
  <select
    class="select"
    aria-label={tr('日志级别', 'Log Level')}
    value={value.logLevel}
    {disabled}
    onchange={(event) => onChange({ ...value, logLevel: event.currentTarget.value })}
  >
    {#each levels as option (option.value)}
      <option value={option.value}>{option.label}</option>
    {/each}
  </select>
</SettingRow>
<SettingRow title={tr('内核控制接口', 'Core Control Endpoint')}>
  <span class="chip">{apiModeLabel}</span>
</SettingRow>

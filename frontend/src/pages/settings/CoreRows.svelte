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

<SettingRow title={tr('日志级别', 'Log Level')} desc={tr('内核日志的详细程度；内核运行时可在「日志」页查看', 'Verbosity of core logs; view live logs in Logs')}>
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
<SettingRow title={tr('内核控制接口', 'Core Control Endpoint')} desc={tr('固定使用随机端口，界面不直接访问该接口', 'Always bound to a loopback random port with secret auth')}>
  <span class="chip">{apiModeLabel}</span>
</SettingRow>

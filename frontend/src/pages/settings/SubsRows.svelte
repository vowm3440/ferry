<script lang="ts">
  import { CAP_AUTOMATION, type SettingsValue } from '../../lib/api';
  import Switch from '../../lib/components/Switch.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { parseUpdateHours, updateHoursOptions } from '../../lib/settings-view';
  import { caps } from '../../lib/stores/caps.svelte';
  import SettingRow from './SettingRow.svelte';

  interface Props {
    value: SettingsValue;
    /** User-Agent 超长时的说明；为空表示可以保存。 */
    uaError: string;
    disabled: boolean;
    onChange: (next: SettingsValue) => void;
  }

  let { value, uaError, disabled, onChange }: Props = $props();

  const hours = $derived((void i18n.locale, updateHoursOptions(value.subscriptionUpdateHours)));
  const ruleSetHours = $derived((void i18n.locale, updateHoursOptions(value.ruleSetUpdateHours || 24)));
  const canAutomate = $derived(caps.has(CAP_AUTOMATION));

  function setHours(text: string): void {
    const parsed = parseUpdateHours(text);
    if (parsed !== null) onChange({ ...value, subscriptionUpdateHours: parsed });
  }

  function setRuleSetHours(text: string): void {
    const parsed = parseUpdateHours(text);
    if (parsed !== null) onChange({ ...value, ruleSetUpdateHours: parsed });
  }
</script>

<SettingRow title={tr('订阅 User-Agent', 'Subscription User-Agent')} error={uaError} wide>
  <label class="input ua" class:invalid={uaError !== ''}>
    <input
      class="mono"
      type="text"
      autocomplete="off"
      spellcheck="false"
      aria-label={tr('订阅 User-Agent', 'Subscription User-Agent')}
      aria-invalid={uaError !== ''}
      value={value.subscriptionUA}
      {disabled}
      oninput={(event) => onChange({ ...value, subscriptionUA: event.currentTarget.value })}
    />
  </label>
</SettingRow>
<SettingRow title={tr('订阅自动更新间隔', 'Subscription Update Interval')}>
  <select
    class="select"
    aria-label={tr('订阅自动更新间隔', 'Subscription Update Interval')}
    value={String(value.subscriptionUpdateHours)}
    {disabled}
    onchange={(event) => setHours(event.currentTarget.value)}
  >
    {#each hours as option (option.value)}
      <option value={option.value}>{option.label}</option>
    {/each}
  </select>
</SettingRow>
<SettingRow
  title={tr('规则集自动更新', 'Auto-Update Rule Sets')}
  desc={canAutomate ? '' : tr('当前版本不提供', 'Unavailable in this backend')}
  tag={canAutomate ? '' : tr('未开放', 'Unavailable')}
>
  <Switch
    checked={value.ruleSetAutoUpdate}
    disabled={!canAutomate || disabled}
    label={canAutomate ? tr('规则集自动更新', 'Auto-Update Rule Sets') : tr('规则集自动更新（未开放）', 'Auto-Update Rule Sets (Unavailable)')}
    onchange={(next: boolean) => onChange({ ...value, ruleSetAutoUpdate: next })}
  />
</SettingRow>
{#if canAutomate && value.ruleSetAutoUpdate}
  <SettingRow title={tr('规则集自动更新间隔', 'Rule-Set Refresh Interval')}>
    <select
      class="select"
      aria-label={tr('规则集自动更新间隔', 'Rule-Set Refresh Interval')}
      value={String(value.ruleSetUpdateHours || 24)}
      {disabled}
      onchange={(event) => setRuleSetHours(event.currentTarget.value)}
    >
      {#each ruleSetHours as option (option.value)}
        <option value={option.value}>{option.label}</option>
      {/each}
    </select>
  </SettingRow>
{/if}

<style>
  .ua {
    width: 220px;
    min-width: 0;
    max-width: 100%;
  }
</style>

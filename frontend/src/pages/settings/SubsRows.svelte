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
  const canAutomate = $derived(caps.has(CAP_AUTOMATION));

  function setHours(text: string): void {
    const parsed = parseUpdateHours(text);
    if (parsed !== null) onChange({ ...value, subscriptionUpdateHours: parsed });
  }
</script>

<SettingRow
  title={tr('订阅 User-Agent', 'Subscription User-Agent')}
  desc={tr(
    '拉取订阅链接时发送的 User-Agent；部分服务商按它决定返回的格式。单个订阅可以在订阅页另行指定',
    'Default User-Agent sent when fetching subscriptions; individual subscriptions can override it in Subscriptions',
  )}
  error={uaError}
  wide
>
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
<SettingRow
  title={tr('订阅自动更新间隔', 'Subscription Update Interval')}
  desc={tr('新添加的链接订阅默认使用这个间隔；已有订阅的间隔在订阅页单独调整', 'Default auto-update interval for newly added URL subscriptions')}
>
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
  desc={canAutomate
    ? tr('在后台每 24 小时检查并更新离线规则集缓存，不中断运行中的内核', 'Refresh offline rule-set caches every 24 hours in the background')
    : tr('当前版本不会自动更新规则集，可在规则页手动更新', 'Automatic refresh unavailable in this backend; update manually in Rules')}
  tag={canAutomate ? '' : tr('未开放', 'Unavailable')}
>
  <Switch
    checked={value.ruleSetAutoUpdate}
    disabled={!canAutomate || disabled}
    label={canAutomate ? tr('规则集自动更新', 'Auto-Update Rule Sets') : tr('规则集自动更新（未开放）', 'Auto-Update Rule Sets (Unavailable)')}
    onchange={(next: boolean) => onChange({ ...value, ruleSetAutoUpdate: next })}
  />
</SettingRow>

<style>
  .ua {
    width: 220px;
    min-width: 0;
    max-width: 100%;
  }
</style>

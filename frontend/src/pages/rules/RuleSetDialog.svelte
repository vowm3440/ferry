<!--
  自定义远程 SRS 规则集编辑器：新建或修改一个远程 HTTPS .srs 规则集，支持社区常用规则集一键预填。
-->
<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import type { CustomRuleSet } from '../../lib/api';
  import Dialog from '../../lib/components/Dialog.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { isValidId, isValidLabel, isValidRuleSetUrl, RULE_SET_REFS } from '../../lib/rules-validate';
  import { updateHoursOptions } from '../../lib/settings-view';
  import { rules } from '../../lib/stores/rules.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';

  interface Props {
    initial: CustomRuleSet | null;
    onclose: () => void;
  }

  let { initial, onclose }: Props = $props();

  const uid = $props.id();
  const creating = $derived(initial === null);

  let id = $state(untrack(() => initial?.id ?? ''));
  let name = $state(untrack(() => initial?.name ?? ''));
  let url = $state(untrack(() => initial?.url ?? ''));
  let autoUpdate = $state(untrack(() => initial?.autoUpdate ?? true));
  let updateHours = $state(untrack(() => initial?.updateHours ?? 24));
  let presets = $state.raw<readonly CustomRuleSet[]>([]);
  let saving = $state(false);
  let error = $state('');

  const hoursOptions = $derived((void i18n.locale, updateHoursOptions(updateHours > 0 ? updateHours : 24)));

  onMount(() => {
    if (creating) {
      void rules.ruleSetPresets().then((res) => {
        if (res.ok) presets = res.value;
      });
    }
  });

  function applyPreset(item: CustomRuleSet): void {
    id = item.id;
    name = item.name;
    url = item.url;
    autoUpdate = item.autoUpdate ?? true;
    updateHours = item.updateHours ?? 24;
    error = '';
  }

  function close(): void {
    if (!saving) onclose();
  }

  async function submit(event?: SubmitEvent): Promise<void> {
    event?.preventDefault();
    if (saving) return;
    const cleanId = id.trim();
    const cleanName = name.trim();
    const cleanUrl = url.trim();
    if (!isValidId(cleanId) || RULE_SET_REFS.some((item) => item.id === cleanId)) {
      error = tr('标识须为字母/数字/连字符（不可与内置规则集重名）', 'ID must be alphanumeric/hyphen and not collide with built-in sets');
      return;
    }
    if (!isValidLabel(cleanName)) {
      error = tr('请填写有效的规则集名称', 'Please enter a valid rule-set name');
      return;
    }
    if (!isValidRuleSetUrl(cleanUrl)) {
      error = tr('地址须为有效的 HTTPS .srs 链接', 'URL must be a valid HTTPS .srs link');
      return;
    }
    error = '';
    saving = true;
    const result = await rules.saveRuleSet({
      id: cleanId,
      name: cleanName,
      url: cleanUrl,
      autoUpdate,
      updateHours: autoUpdate ? updateHours : 0,
    });
    saving = false;
    if (!result.ok) {
      error = result.message;
      return;
    }
    toasts.ok(
      creating
        ? tr(`已添加规则集「${cleanName}」，可在规则中使用 ruleset:${cleanId}`, `Added rule set "${cleanName}" (use ruleset:${cleanId})`)
        : tr(`已保存规则集「${cleanName}」`, `Saved rule set "${cleanName}"`),
    );
    onclose();
  }
</script>

<Dialog open title={creating ? tr('添加远程 SRS 规则集', 'Add Remote SRS Rule Set') : tr('编辑远程 SRS 规则集', 'Edit Remote SRS Rule Set')} busy={saving} width={540} backdropClose={false} onclose={close}>
  <form id={`${uid}-form`} class="form" novalidate onsubmit={(event) => void submit(event)}>
    {#if creating && presets.length > 0}
      <div class="preset-box">
        <span class="label">{tr('快捷填入社区规则集', 'Community Presets')}</span>
        <div class="preset-chips">
          {#each presets as item (item.id)}
            <button type="button" class="btn sm" onclick={() => applyPreset(item)}>
              <Icon name="plus" />{item.name}
            </button>
          {/each}
        </div>
      </div>
    {/if}

    <div class="row">
      <label class="field grow">
        <span class="label">{tr('规则集标识（ID）', 'Rule Set ID')}</span>
        <input
          class="control mono"
          data-autofocus={creating ? true : undefined}
          name="rs-id"
          autocomplete="off"
          spellcheck="false"
          disabled={!creating}
          placeholder="openai / steam-cn / apple-cn"
          value={id}
          oninput={(event) => (id = event.currentTarget.value)}
        />
        <span class="muted hint">{tr(`规则里引用写法：ruleset:${id.trim() || 'id'}`, `Rule reference: ruleset:${id.trim() || 'id'}`)}</span>
      </label>
      <label class="field grow">
        <span class="label">{tr('显示名称', 'Display Name')}</span>
        <input
          class="control"
          data-autofocus={!creating ? true : undefined}
          name="rs-name"
          autocomplete="off"
          placeholder={tr('例如 OpenAI / 去广告', 'e.g. OpenAI / AdBlock')}
          value={name}
          oninput={(event) => (name = event.currentTarget.value)}
        />
      </label>
    </div>

    <label class="field">
      <span class="label">{tr('远程 .srs 下载地址（HTTPS）', 'Remote .srs URL (HTTPS)')}</span>
      <input
        class="control mono"
        name="rs-url"
        autocomplete="off"
        spellcheck="false"
        placeholder="https://raw.githubusercontent.com/MetaCubeX/meta-rules-dat/sing/geo/geosite/openai.srs"
        value={url}
        oninput={(event) => (url = event.currentTarget.value)}
      />
    </label>

    <div class="row align">
      <div class="field grow">
        <span class="label">{tr('定时自动刷新', 'Scheduled Auto-Refresh')}</span>
        <div class="switch-line">
          <Switch checked={autoUpdate} label={tr('启用定时自动刷新', 'Enable scheduled refresh')} onchange={(next) => (autoUpdate = next)} />
          <span class="muted">{tr('后台按间隔自动拉取最新 .srs 缓存', 'Refresh cached .srs in background')}</span>
        </div>
      </div>
      {#if autoUpdate}
        <label class="field">
          <span class="label">{tr('刷新间隔', 'Interval')}</span>
          <select class="select" value={String(updateHours || 24)} onchange={(event) => (updateHours = Number(event.currentTarget.value) || 24)}>
            {#each hoursOptions as option (option.value)}
              <option value={option.value}>{option.label}</option>
            {/each}
          </select>
        </label>
      {/if}
    </div>

    {#if error !== ''}
      <div class="inline-error" role="alert"><Icon name="alert" /><span class="selectable">{error}</span></div>
    {/if}
  </form>

  {#snippet footer()}
    <span class="muted foot-note">{tr('添加后需点击「更新」下载到本地缓存方可用于分流。', 'Click Update after saving to download into local cache.')}</span>
    <button type="button" class="btn" aria-disabled={saving} onclick={close}>{tr('取消', 'Cancel')}</button>
    <button type="submit" class="btn primary" form={`${uid}-form`} aria-disabled={saving}>{saving ? tr('保存中…', 'Saving…') : tr('保存', 'Save')}</button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: 12px;
  }

  .preset-box {
    display: grid;
    gap: 6px;
    padding: 10px;
    border-radius: 8px;
    background: var(--muted-soft);
  }

  .preset-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    align-items: flex-start;
  }

  .row.align {
    align-items: center;
  }

  .grow {
    flex: 1 1 200px;
  }

  .hint {
    font-size: 11.5px;
  }

  .switch-line {
    display: flex;
    align-items: center;
    gap: 8px;
    min-height: 32px;
    font-size: 12px;
  }

  .foot-note {
    margin-right: auto;
    font-size: 12px;
  }
</style>

<!-- 订阅设置：改名，以及（有订阅地址时）自动更新与单订阅独立 User-Agent。只提交改动过的部分。 -->
<script lang="ts">
  import { CAP_SUBS_UA, type Subscription } from '../../lib/api';
  import Dialog from '../../lib/components/Dialog.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import { userAgentError } from '../../lib/settings-view';
  import { caps } from '../../lib/stores/caps.svelte';
  import { settings } from '../../lib/stores/settings.svelte';
  import { subs } from '../../lib/stores/subs.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import { canUpdate, checkSubName, msgHours, parseHours } from '../../lib/subs-view';

  interface Props {
    /** 正在编辑的订阅；null 表示关闭。 */
    target: Subscription | null;
    onclose: () => void;
  }

  let { target, onclose }: Props = $props();

  const DEFAULT_HOURS = 24;

  let name = $state('');
  let autoUpdate = $state(false);
  let hours = $state('');
  let customUa = $state('');
  let error = $state('');
  let editingId = '';

  const busy = $derived(target !== null && subs.saving === target.id);
  const updatable = $derived(target !== null && canUpdate(target));
  const canCustomUa = $derived(caps.has(CAP_SUBS_UA));
  const globalUa = $derived(settings.value?.subscriptionUA ?? 'clash.meta');

  // 每次打开（换了一个订阅）时，用它当前的值填表。
  $effect(() => {
    const id = target?.id ?? '';
    if (id === editingId) return;
    editingId = id;
    if (target === null) return;
    name = target.name;
    autoUpdate = target.autoUpdate;
    hours = String(target.intervalHours > 0 ? target.intervalHours : DEFAULT_HOURS);
    customUa = target.userAgent !== '' && target.userAgent !== globalUa ? target.userAgent : '';
    error = '';
  });

  function close(): void {
    if (!busy) onclose();
  }

  async function submit(event?: SubmitEvent): Promise<void> {
    event?.preventDefault();
    if (target === null || busy) return;
    const nextName = name.trim();
    const nextHours = parseHours(hours);
    const nextUa = customUa.trim();
    const uaProblem = updatable && canCustomUa ? userAgentError(nextUa) : '';
    const problem =
      nextName === ''
        ? tr('请填写名称', 'Please enter a name')
        : checkSubName(nextName) || (updatable && nextHours === null ? msgHours() : '') || uaProblem;
    if (problem !== '') {
      error = problem;
      return;
    }
    error = '';

    if (nextName !== target.name) {
      const renamed = await subs.rename(target.id, nextName);
      if (!renamed.ok) {
        error = renamed.message;
        return;
      }
    }
    if (updatable && nextHours !== null && (autoUpdate !== target.autoUpdate || nextHours !== target.intervalHours)) {
      const saved = await subs.setAutoUpdate(target.id, autoUpdate, nextHours);
      if (!saved.ok) {
        error = saved.message;
        return;
      }
    }
    if (updatable && canCustomUa) {
      const effectiveTargetUa = target.userAgent !== '' && target.userAgent !== globalUa ? target.userAgent : '';
      if (nextUa !== effectiveTargetUa) {
        const uaSaved = await subs.setUserAgent(target.id, nextUa);
        if (!uaSaved.ok) {
          error = uaSaved.message;
          return;
        }
      }
    }
    toasts.ok(tr(`已保存「${nextName}」`, `Saved "${nextName}"`));
    onclose();
  }
</script>

<Dialog open={target !== null} title={tr('订阅设置', 'Subscription Settings')} {busy} width={440} backdropClose={false} onclose={close}>
  <form id="sub-edit-form" class="form" novalidate onsubmit={(event) => void submit(event)}>
    <label class="field">
      <span class="label">{tr('名称', 'Name')}</span>
      <input id="sub-edit-name" class="control" type="text" maxlength="64" autocomplete="off" data-autofocus bind:value={name} />
    </label>

    {#if updatable}
      <div class="toggle">
        <Switch checked={autoUpdate} label={tr('自动更新', 'Auto Update')} onchange={(next: boolean) => (autoUpdate = next)} />
        <b>{tr('自动更新', 'Auto Update')}</b>
      </div>
      <label class="field">
        <span class="label">{tr('更新间隔（小时，1–720）', 'Update Interval (hours, 1–720)')}</span>
        <input
          id="sub-edit-hours"
          class="control num"
          type="text"
          inputmode="numeric"
          maxlength="3"
          autocomplete="off"
          disabled={!autoUpdate}
          bind:value={hours}
        />
      </label>
      {#if canCustomUa}
        <label class="field">
          <span class="label">{tr('独立 User-Agent（留空则跟随全局设置）', 'Custom User-Agent (leave blank for global default)')}</span>
          <input
            id="sub-edit-ua"
            class="control mono"
            type="text"
            maxlength="256"
            spellcheck="false"
            autocomplete="off"
            placeholder={globalUa}
            bind:value={customUa}
          />
        </label>
      {/if}
    {:else}
      <div class="inline-note"><Icon name="info" /><span>{tr('手动导入的节点没有订阅地址，不能自动更新。', 'Manually imported nodes have no URL and cannot auto-update.')}</span></div>
    {/if}

    {#if error !== ''}
      <div class="inline-error" role="alert"><Icon name="alert" /><span>{error}</span></div>
    {/if}
  </form>

  {#snippet footer()}
    <button type="button" class="btn" aria-disabled={busy} onclick={close}>{tr('取消', 'Cancel')}</button>
    <button type="submit" form="sub-edit-form" class="btn primary" aria-disabled={busy}>{busy ? tr('保存中…', 'Saving…') : tr('保存', 'Save')}</button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: 14px;
  }

  .toggle {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .toggle b {
    font-weight: 500;
  }
</style>

<!-- 按地址添加订阅。地址可能带令牌：只交给后端，成功后立刻清空，界面此后只显示打码后的地址。 -->
<script lang="ts">
  import { CAP_SUBS_UA, type Subscription } from '../../lib/api';
  import Dialog from '../../lib/components/Dialog.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { userAgentError } from '../../lib/settings-view';
  import { caps } from '../../lib/stores/caps.svelte';
  import { settings } from '../../lib/stores/settings.svelte';
  import { subs } from '../../lib/stores/subs.svelte';
  import { checkSubName, checkSubUrl, isPlainHttp, msgHttpWarning } from '../../lib/subs-view';

  interface Props {
    open: boolean;
    onclose: () => void;
    /** 导入成功：返回后端给出的订阅（含 warnings 与 skipped）。 */
    ondone: (subscription: Subscription) => void;
  }

  let { open, onclose, ondone }: Props = $props();

  let name = $state('');
  let url = $state('');
  let customUa = $state('');
  let autoUpdate = $state(true);
  let error = $state('');

  const plain = $derived(isPlainHttp(url));
  const canCustomUa = $derived(caps.has(CAP_SUBS_UA));
  const globalUa = $derived(settings.value?.subscriptionUA ?? 'clash.meta');

  function reset(): void {
    name = '';
    url = '';
    customUa = '';
    autoUpdate = true;
    error = '';
  }

  function close(): void {
    if (subs.importing) return;
    reset();
    onclose();
  }

  async function submit(event?: SubmitEvent): Promise<void> {
    event?.preventDefault();
    if (subs.importing) return;
    const uaText = customUa.trim();
    const problem = checkSubUrl(url) || checkSubName(name) || (canCustomUa ? userAgentError(uaText) : '');
    if (problem !== '') {
      error = problem;
      return;
    }
    error = '';
    const outcome = await subs.importUrl(name.trim(), url.trim(), autoUpdate);
    if (!outcome.ok) {
      error = outcome.message;
      return;
    }
    if (canCustomUa && uaText !== '') {
      await subs.setUserAgent(outcome.subscription.id, uaText);
    }
    reset();
    ondone(outcome.subscription);
  }
</script>

<Dialog {open} title={tr('添加订阅链接', 'Add Subscription URL')} busy={subs.importing} backdropClose={false} onclose={close}>
  <form id="sub-url-form" class="form" novalidate onsubmit={(event) => void submit(event)}>
    <label class="field">
      <span class="label">{tr('订阅地址', 'Subscription URL')}</span>
      <input
        id="sub-url"
        class="control mono"
        type="url"
        inputmode="url"
        maxlength="4096"
        spellcheck="false"
        autocomplete="off"
        autocapitalize="off"
        placeholder="https://…"
        data-autofocus
        aria-invalid={error !== ''}
        bind:value={url}
      />
      <span class="hint">{tr('支持 Clash / sing-box / Base64 / 分享链接 / SIP008。地址只交给本机后端，之后界面只显示打码后的地址。', 'Supports Clash, sing-box, Base64, share links, and SIP008. URL is encrypted locally and masked in UI.')}</span>
    </label>

    <label class="field">
      <span class="label">{tr('名称（可留空，使用订阅自带的名称）', 'Name (optional; defaults to profile title)')}</span>
      <input id="sub-url-name" class="control" type="text" maxlength="64" autocomplete="off" bind:value={name} />
    </label>

    {#if canCustomUa}
      <label class="field">
        <span class="label">{tr('独立 User-Agent（可选，留空跟随全局设置）', 'Custom User-Agent (optional; defaults to global)')}</span>
        <input
          id="sub-url-ua"
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

    <div class="toggle">
      <Switch checked={autoUpdate} label={tr('自动更新', 'Auto Update')} onchange={(next: boolean) => (autoUpdate = next)} />
      <span>
        <b>{tr('自动更新', 'Auto Update')}</b>
        <span class="muted">{tr('按「设置」里的间隔定期更新，之后可以在订阅上单独修改。', 'Refreshes on schedule; customizable per subscription later.')}</span>
      </span>
    </div>

    {#if plain}
      <div class="inline-note warn" role="status"><Icon name="alert" /><span>{(void i18n.locale, msgHttpWarning())}</span></div>
    {/if}
    {#if error !== ''}
      <div class="inline-error" role="alert"><Icon name="alert" /><span>{error}</span></div>
    {/if}
  </form>

  {#snippet footer()}
    <button type="button" class="btn" aria-disabled={subs.importing} onclick={close}>{tr('取消', 'Cancel')}</button>
    <button type="submit" form="sub-url-form" class="btn primary" aria-disabled={subs.importing}>
      {subs.importing ? tr('正在下载…', 'Downloading…') : tr('添加', 'Add')}
    </button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: 14px;
  }

  .toggle {
    display: flex;
    align-items: flex-start;
    gap: 10px;
  }

  .toggle b {
    display: block;
    font-weight: 500;
  }

  .toggle .muted {
    font-size: 12px;
  }

  .warn {
    border-color: var(--warn-soft);
    background: var(--warn-soft);
    color: var(--warn);
  }

  .warn :global(svg.i) {
    color: var(--warn);
  }
</style>

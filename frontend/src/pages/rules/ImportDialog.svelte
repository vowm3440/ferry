<!--
  导入规则：选择来源格式，粘贴内容或选一个本机文件 → 后端解析出预览（此时什么都没写入）→
  用户确认后逐条保存。遇到第一条失败就停下，并如实说明已经导入了几条。
  规则文档里可能带有节点凭据：内容只交给后端解析，拿到预览后立即从界面里清掉。
  由父组件用 {#if} 挂载。
-->
<script lang="ts">
  import { tick } from 'svelte';
  import { IMPORT_KINDS, MAX_IMPORT_BYTES, type ImportKind, type ImportPreview, type Rule } from '../../lib/api';
  import Dialog from '../../lib/components/Dialog.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import Segmented from '../../lib/components/Segmented.svelte';
  import { pickLocalFile, readFileText, utf8Length } from '../../lib/file-pick';
  import { tr } from '../../lib/i18n/index.svelte';
  import { IMPORT_KIND_LABELS, importOutcomeText } from '../../lib/rules-view';
  import { rules } from '../../lib/stores/rules.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import ImportPreviewList from './ImportPreviewList.svelte';

  interface Props {
    onclose: () => void;
  }

  let { onclose }: Props = $props();

  const KIND_OPTIONS = IMPORT_KINDS.map((value) => ({ value, label: IMPORT_KIND_LABELS[value] ?? value }));
  const ACCEPT = '.yaml,.yml,.json,.ppx,.xml,.txt';
  const placeholders = $derived<Readonly<Record<ImportKind, string>>>({
    clash: tr('粘贴 Clash 配置或其中的 rules 列表', 'Paste Clash config or its rules list'),
    singbox: tr('粘贴 sing-box 配置或其中的 route 部分（JSON）', 'Paste sing-box config or its route object (JSON)'),
    ppx: tr('粘贴 Proxifier 配置文件（.ppx）的内容', 'Paste Proxifier profile (.ppx) XML content'),
  });

  let kind = $state<ImportKind>('clash');
  let content = $state('');
  let fileName = $state('');
  let parsing = $state(false);
  let saving = $state(false);
  let error = $state('');
  let preview = $state.raw<ImportPreview | null>(null);
  /** 这次要写入的规则：预览里的全部，去掉用户拿掉的。 */
  let pending = $state.raw<readonly Rule[]>([]);

  const busy = $derived(parsing || saving);

  /** 换了一步之后原来的焦点元素已经不在了：把焦点交给新一步里标了 data-step-focus 的元素。 */
  async function focusStep(): Promise<void> {
    await tick();
    document.querySelector<HTMLElement>('[role="dialog"] [data-step-focus]')?.focus();
  }

  function close(): void {
    if (busy) return;
    content = '';
    onclose();
  }

  function kindOfFile(name: string): ImportKind | null {
    const lower = name.toLowerCase();
    if (lower.endsWith('.ppx')) return 'ppx';
    if (lower.endsWith('.json')) return 'singbox';
    return lower.endsWith('.yaml') || lower.endsWith('.yml') ? 'clash' : null;
  }

  async function chooseFile(): Promise<void> {
    if (busy) return;
    const file = await pickLocalFile(ACCEPT);
    if (file === null) return;
    try {
      content = await readFileText(file, MAX_IMPORT_BYTES);
      fileName = file.name;
      kind = kindOfFile(file.name) ?? kind;
      error = '';
    } catch (cause: unknown) {
      error = cause instanceof Error ? cause.message : tr('无法读取文件', 'Unable to read file');
    }
  }

  async function parse(event?: SubmitEvent): Promise<void> {
    event?.preventDefault();
    if (busy) return;
    if (content.trim() === '') {
      error = tr('请粘贴规则内容，或选择一个文件', 'Please paste rule content or choose a file');
      return;
    }
    if (utf8Length(content) > MAX_IMPORT_BYTES) {
      error = tr('规则内容超过 1 MiB 限制', 'Rule content exceeds 1 MiB limit');
      return;
    }
    error = '';
    parsing = true;
    const result = await rules.preview(kind, content);
    parsing = false;
    if (!result.ok) {
      error = result.message;
      return;
    }
    preview = result.value;
    pending = result.value.rules;
    content = '';
    fileName = '';
    void focusStep();
  }

  function back(): void {
    if (busy) return;
    preview = null;
    pending = [];
    error = '';
    void focusStep();
  }

  /** 不导入其中一条：后端因为某条规则拒绝整批时，拿掉它就能导入其余的。 */
  async function drop(index: number): Promise<void> {
    if (busy) return;
    pending = pending.filter((_, at) => at !== index);
    error = '';
    // 被点的按钮随那一行一起消失了：把焦点交给补上来的那一行，没有行了就交给「取消」。
    await tick();
    const buttons = document.querySelectorAll<HTMLElement>('[role="dialog"] button[data-drop]');
    const next = buttons[Math.min(index, buttons.length - 1)];
    if (next === undefined) void focusStep();
    else next.focus();
  }

  async function confirm(): Promise<void> {
    if (busy || preview === null || pending.length === 0) return;
    error = '';
    saving = true;
    const total = pending.length;
    const result = await rules.saveImported(pending);
    saving = false;
    if (result.ok) {
      toasts.ok(importOutcomeText(total, ''));
      onclose();
      return;
    }
    // 整批是一个事务：失败时什么都没有写入，预览原样保留。
    error = importOutcomeText(total, result.message);
  }
</script>

<Dialog open title={tr('导入规则', 'Import Rules')} {busy} width={preview === null ? 560 : 720} backdropClose={false} onclose={close}>
  {#if preview === null}
    <form id="rules-import-form" class="form" novalidate onsubmit={(event) => void parse(event)}>
      <div class="field">
        <span class="label">{tr('来源格式', 'Source Format')}</span>
        <Segmented options={KIND_OPTIONS} value={kind} label={tr('来源格式', 'Source Format')} busy={parsing} onchange={(next) => (kind = next)} />
      </div>
      <label class="field">
        <span class="label">{tr('规则内容', 'Rule Content')}</span>
        <textarea class="control mono" data-autofocus data-step-focus name="import-content" rows="9" spellcheck="false" placeholder={placeholders[kind]} bind:value={content} oninput={() => (fileName = '')}></textarea>
        <span class="hint">{tr('最多 1 MiB。只会读取其中的规则；内容交给后端解析，界面不保存。', 'Up to 1 MiB. Only routing rules are extracted; parsed locally by backend and not saved in UI.')}</span>
      </label>
      <div class="file">
        <button type="button" class="btn sm" aria-disabled={busy} onclick={() => void chooseFile()}><Icon name="file" />{tr('选择文件…', 'Choose File…')}</button>
        {#if fileName !== ''}<span class="muted name" title={fileName}>{tr(`已读取：${fileName}`, `Loaded: ${fileName}`)}</span>{/if}
      </div>
      {#if error !== ''}
        <div class="inline-error" role="alert"><Icon name="alert" /><span class="selectable">{error}</span></div>
      {/if}
    </form>
  {:else}
    <ImportPreviewList rules={pending} warnings={preview.warnings} total={preview.rules.length} busy={saving} onremove={(index) => void drop(index)} />
    {#if error !== ''}
      <div class="inline-error gap" role="alert"><Icon name="alert" /><span class="selectable">{error}</span></div>
    {/if}
  {/if}

  {#snippet footer()}
    {#if preview === null}
      <span class="muted foot-note">{tr('下一步会先显示预览，确认后才写入。', 'Preview will be shown before saving.')}</span>
      <button type="button" class="btn" aria-disabled={busy} onclick={close}>{tr('取消', 'Cancel')}</button>
      <button type="submit" class="btn primary" form="rules-import-form" aria-disabled={busy}>{parsing ? tr('解析中…', 'Parsing…') : tr('解析并预览', 'Parse & Preview')}</button>
    {:else}
      <span class="muted foot-note">{tr('规则会加在「默认规则」之前；内核运行中会重启一次。', 'Rules are inserted before the final fallback rule; running core restarts once.')}</span>
      <button type="button" class="btn" aria-disabled={busy} onclick={back}>{tr('上一步', 'Back')}</button>
      <button type="button" class="btn" data-step-focus aria-disabled={busy} onclick={close}>{tr('取消', 'Cancel')}</button>
      {#if pending.length > 0}
        <button type="button" class="btn primary" aria-disabled={busy} onclick={() => void confirm()}>
          {saving ? tr('导入中…', 'Importing…') : tr(`导入 ${pending.length} 条规则`, `Import ${pending.length} Rules`)}
        </button>
      {/if}
    {/if}
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: 12px;
  }

  textarea.control {
    min-height: 160px;
    font-size: 12.5px;
  }

  .file {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }

  .name {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: 12px;
  }

  .gap {
    margin-top: 12px;
  }

  .foot-note {
    margin-right: auto;
    font-size: 12px;
  }
</style>

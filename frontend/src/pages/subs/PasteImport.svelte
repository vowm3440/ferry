<!-- 粘贴导入：把分享链接或配置文本交给后端解析。内容可能含凭据，导入成功后立刻清空输入。 -->
<script lang="ts">
  import { CAP_PERSIST, MAX_IMPORT_BYTES, type SkippedLine } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import { utf8Length } from '../../lib/file-pick';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { caps } from '../../lib/stores/caps.svelte';
  import { subs } from '../../lib/stores/subs.svelte';
  import { checkSubName, describeSkipped } from '../../lib/subs-view';

  interface Props {
    /** 内核运行或切换中：后端会拒绝导入。 */
    locked: boolean;
    /** 还没拿到内核状态，或 locked。 */
    unavailable: boolean;
  }

  let { locked, unavailable }: Props = $props();

  const placeholder = $derived(
    tr(
      'vless://…、trojan://…、ss://…、socks5://…，每行一条；也支持 Clash / sing-box 配置与整段 Base64',
      'vless://…, trojan://…, ss://…, socks5://… (one per line), or full Clash / sing-box / Base64 config',
    ),
  );

  let name = $state('');
  let text = $state('');
  let input = $state<HTMLTextAreaElement | null>(null);
  /** 上一次导入的结果：新导入的节点数与被跳过的行。 */
  let imported = $state<{ count: number; skipped: SkippedLine[] } | null>(null);
  let error = $state('');

  /** 供「粘贴内容」入口调用：把焦点放进文本框。 */
  export function focus(): void {
    input?.scrollIntoView({ block: 'nearest' });
    input?.focus();
  }

  function precheck(payload: string): string {
    if (payload === '') return tr('请先粘贴节点链接', 'Please paste node links or config first');
    if (utf8Length(payload) > MAX_IMPORT_BYTES) return tr('内容过大：不能超过 1 MiB', 'Content too large: maximum 1 MiB');
    return checkSubName(name);
  }

  async function submit(): Promise<void> {
    // 导入进行中按钮只是暂不响应（不禁用，以免丢掉键盘焦点）。
    if (subs.importing) return;
    error = '';
    imported = null;
    const payload = text.trim();
    const problem = precheck(payload);
    if (problem !== '') {
      error = problem;
      return;
    }

    const outcome = await subs.import(name, payload);
    if (!outcome.ok) {
      error = outcome.message;
      return;
    }

    // 链接里带着节点凭据：导入成功后立刻清空输入，不在界面里保留。
    text = '';
    name = '';
    imported = { count: outcome.subscription.nodeCount, skipped: outcome.subscription.skipped };
  }

  function clear(): void {
    text = '';
    imported = null;
    error = '';
    input?.focus();
  }
</script>

<div class="card import">
  <div class="card-head">
    <span>{tr('粘贴内容', 'Paste Content')}</span>
  </div>

  <div class="fields">
    <label class="field">
      <span class="label">{tr('名称', 'Name')}</span>
      <input class="control" id="sub-name" type="text" maxlength="64" autocomplete="off" placeholder={tr('手动导入', 'Manual Import')} bind:value={name} />
    </label>

    <label class="field">
      <span class="label">{tr('节点链接或配置', 'Node URIs or Config')}</span>
      <textarea
        id="sub-text"
        class="control mono"
        rows="4"
        spellcheck="false"
        autocomplete="off"
        autocapitalize="off"
        placeholder={placeholder}
        bind:this={input}
        bind:value={text}
      ></textarea>
    </label>
  </div>

  {#if locked}
    <div class="inline-note"><Icon name="info" /><span>{tr('内核运行中：请先停止内核，再导入节点。', 'Core is running: stop core before importing nodes.')}</span></div>
  {/if}
  {#if error !== ''}
    <div class="inline-error" role="alert"><Icon name="alert" /><span>{error}</span></div>
  {/if}
  {#if imported !== null}
    <div class="inline-note" role="status">
      <Icon name="check" />
      <span class="grow">
        {imported.skipped.length > 0
          ? tr(`已导入 ${imported.count} 个节点，跳过 ${imported.skipped.length} 行。`, `Imported ${imported.count} nodes (${imported.skipped.length} skipped).`)
          : tr(`已导入 ${imported.count} 个节点。`, `Imported ${imported.count} nodes.`)}
        {#if !caps.has(CAP_PERSIST)}{tr('本次运行内有效，重启后需要重新导入。', 'Valid for this session only; re-import after restart.')}{/if}
      </span>
    </div>
    {#if imported.skipped.length > 0}
      <ul class="skipped">
        {#each imported.skipped as item, index (index)}
          <li>{(void i18n.locale, describeSkipped(item))}</li>
        {/each}
      </ul>
    {/if}
  {/if}

  <div class="actions">
    <span class="spacer"></span>
    <button type="button" class="btn sm ghost" disabled={text === ''} onclick={clear}>{tr('清空', 'Clear')}</button>
    <button type="button" class="btn sm primary" disabled={unavailable} aria-disabled={subs.importing} onclick={() => void submit()}>
      {subs.importing ? tr('导入中…', 'Importing…') : tr('导入', 'Import')}
    </button>
  </div>
</div>

<style>
  .import {
    display: grid;
    gap: 10px;
    margin-bottom: 14px;
  }

  .fields {
    display: grid;
    gap: 10px;
  }

  textarea {
    min-height: 84px;
  }

  .skipped {
    margin: 0;
    padding-left: 18px;
    display: grid;
    gap: 2px;
    font-size: 12px;
    color: var(--text-2);
    user-select: text;
  }

  .actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .spacer {
    flex: 1;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }
</style>

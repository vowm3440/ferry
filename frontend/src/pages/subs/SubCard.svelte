<!-- 一条订阅：名称与状态、打码后的地址、流量与到期、更新进度与结果，以及更新 / 分享 / 设置 / 删除。 -->
<script lang="ts">
  import { CAP_SHARE, CAP_SUBS_URL, type Subscription } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { caps } from '../../lib/stores/caps.svelte';
  import { shareRequest } from '../../lib/stores/share-request.svelte';
  import { subs } from '../../lib/stores/subs.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import { expiryView, usageView } from '../../lib/sub-usage';
  import { autoUpdateText, canUpdate, describeSkipped, progressText, statusPill } from '../../lib/subs-view';
  import { formatAgo } from '../../lib/time';

  interface Props {
    sub: Subscription;
    /** 当前时间，用来算「几分钟前」与到期。 */
    now: number;
    onedit: (sub: Subscription) => void;
    onremove: (sub: Subscription) => void;
  }

  let { sub, now, onedit, onremove }: Props = $props();

  const pill = $derived((void i18n.locale, statusPill(sub)));
  const usage = $derived(usageView(sub.usedBytes, sub.totalBytes));
  const expiry = $derived((void i18n.locale, expiryView(sub.expiresAt, now)));
  const updatable = $derived(caps.has(CAP_SUBS_URL) && canUpdate(sub));
  const updating = $derived(subs.isUpdating(sub.id) && canUpdate(sub));
  const phase = $derived((void i18n.locale, progressText(subs.progress[sub.id] ?? '')));
  const contents = $derived(
    sub.groupCount > 0
      ? tr(`${sub.nodeCount} 个节点 · ${sub.groupCount} 个分组`, `${sub.nodeCount} nodes · ${sub.groupCount} groups`)
      : tr(`${sub.nodeCount} 个节点`, `${sub.nodeCount} nodes`),
  );
  const updatedText = $derived((void i18n.locale, sub.lastUpdatedAt > 0 ? formatAgo(sub.lastUpdatedAt, now) : '—'));

  async function update(): Promise<void> {
    if (updating) return;
    const result = await subs.update(sub.id);
    if (result.ok) toasts.ok(tr(`已更新订阅「${sub.name}」`, `Updated subscription "${sub.name}"`));
    else toasts.error(result.message);
  }
</script>

<li class="row card" class:failed={sub.status === 'error'}>
  <div class="top">
    <div class="id">
      <div class="title">
        <span class="name selectable">{sub.name}</span>
        <span class="pill {pill.tone}">{pill.text}</span>
      </div>
      <div class="url mono selectable">{sub.urlMasked !== '' ? sub.urlMasked : tr('手动导入，没有订阅地址', 'Manually imported (no URL)')}</div>
    </div>
    <div class="acts">
      {#if updatable}
        <button
          type="button"
          class="icon-btn"
          class:spin={updating}
          aria-label={tr(`更新订阅 ${sub.name}`, `Update subscription ${sub.name}`)}
          title={tr('更新', 'Update')}
          aria-disabled={updating}
          onclick={() => void update()}
        >
          <Icon name="refresh" />
        </button>
      {/if}
      {#if caps.has(CAP_SHARE)}
        <button
          type="button"
          class="icon-btn"
          aria-label={tr(`分享订阅 ${sub.name}`, `Share subscription ${sub.name}`)}
          title={tr('分享', 'Share')}
          onclick={() => shareRequest.open({ kind: 'sub', id: sub.id, name: sub.name })}
        >
          <Icon name="share" />
        </button>
      {/if}
      {#if caps.has(CAP_SUBS_URL)}
        <button
          type="button"
          class="icon-btn"
          aria-label={tr(`设置订阅 ${sub.name}`, `Configure subscription ${sub.name}`)}
          title={tr('重命名与自动更新', 'Rename & Auto Update')}
          onclick={() => onedit(sub)}
        >
          <Icon name="edit" />
        </button>
      {/if}
      <button
        type="button"
        class="btn sm danger"
        aria-label={tr(`删除订阅 ${sub.name}`, `Remove subscription ${sub.name}`)}
        aria-disabled={subs.removing !== ''}
        onclick={() => onremove(sub)}
      >
        {subs.removing === sub.id ? tr('删除中…', 'Removing…') : tr('删除', 'Remove')}
      </button>
    </div>
  </div>

  {#if usage.hasTotal || !usage.empty || expiry.known}
    <div class="usage">
      <span>
        {#if usage.hasTotal}
          {tr('已用 ', 'Used ')}<b class="num">{usage.usedText}</b> / {usage.totalText}
          <span class="muted num">（{usage.percentText}）</span>
        {:else if !usage.empty}
          {tr('已用 ', 'Used ')}<b class="num">{usage.usedText}</b>
        {:else}
          {tr('流量 —', 'Quota —')}
        {/if}
      </span>
      <span class="exp {expiry.level}">
        {#if expiry.known}{tr('到期 ', 'Expires ')}{expiry.dateText}{#if expiry.remainText !== ''} · {expiry.remainText}{/if}{:else}{tr('到期 —', 'Expires —')}{/if}
      </span>
    </div>
    {#if usage.hasTotal}
      <div class="bar {usage.level === 'ok' ? '' : usage.level}" role="img" aria-label={tr(`已用 ${usage.percentText}`, `Used ${usage.percentText}`)}>
        <i style:width="{usage.percent}%"></i>
      </div>
    {/if}
  {/if}

  <dl class="kv">
    <div><dt>{tr('包含', 'Contents')}</dt><dd class="num">{contents}</dd></div>
    <div><dt>{tr('上次更新', 'Last Updated')}</dt><dd class="num">{updatedText}</dd></div>
    <div><dt>{tr('自动更新', 'Auto Update')}</dt><dd class="num">{(void i18n.locale, autoUpdateText(sub))}</dd></div>
  </dl>

  {#if updating}
    <div class="progress" role="status">
      <span class="spin"><Icon name="refresh" size={14} /></span>
      <span>{phase !== '' ? phase : tr('正在更新…', 'Updating…')}</span>
    </div>
  {/if}
  {#if sub.error !== undefined && sub.error !== ''}
    <div class="inline-error"><Icon name="alert" /><span>{sub.error}</span></div>
  {/if}
  {#each sub.warnings as warning, index (index)}
    <div class="inline-note warn"><Icon name="alert" /><span>{warning}</span></div>
  {/each}
  {#if sub.skipped.length > 0}
    <details class="skipped">
      <summary>{tr(`跳过了 ${sub.skipped.length} 行`, `Skipped ${sub.skipped.length} lines`)}</summary>
      <ul>
        {#each sub.skipped as item, index (index)}
          <li>{(void i18n.locale, describeSkipped(item))}</li>
        {/each}
      </ul>
    </details>
  {/if}
</li>

<style>
  .row {
    display: grid;
    gap: 10px;
    min-width: 0;
  }

  .failed {
    border-color: var(--err-soft);
  }

  .top {
    display: flex;
    align-items: flex-start;
    gap: 10px;
  }

  .id {
    flex: 1;
    min-width: 0;
    display: grid;
    gap: 2px;
  }

  .title {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .name {
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .url {
    font-size: 12px;
    color: var(--text-3);
    overflow-wrap: anywhere;
  }

  .acts {
    flex: none;
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .acts .btn {
    margin-left: 4px;
  }

  .usage {
    display: flex;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 4px 12px;
    font-size: 12px;
    color: var(--text-2);
  }

  .exp.warn {
    color: var(--warn);
  }

  .exp.err {
    color: var(--err);
  }

  .kv {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
    gap: 8px 16px;
    margin: 0;
    font-size: 12.5px;
  }

  .kv dt {
    font-size: 11.5px;
    color: var(--text-3);
  }

  .kv dd {
    margin: 0;
  }

  .progress {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12.5px;
    color: var(--text-2);
  }

  .progress .spin {
    display: grid;
    color: var(--accent);
  }

  .warn {
    border-color: var(--warn-soft);
    background: var(--warn-soft);
    color: var(--warn);
  }

  .warn :global(svg.i) {
    color: var(--warn);
  }

  .skipped {
    font-size: 12px;
    color: var(--text-2);
  }

  .skipped summary {
    cursor: pointer;
  }

  .skipped ul {
    margin: 6px 0 0;
    padding-left: 18px;
    display: grid;
    gap: 2px;
    user-select: text;
  }
</style>

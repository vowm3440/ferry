<script lang="ts">
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { usageRows } from '../../lib/overview-view';
  import { nav } from '../../lib/stores/nav.svelte';
  import { subs } from '../../lib/stores/subs.svelte';

  /** 概览只列前几个订阅，其余的去订阅页看。 */
  const MAX_ROWS = 4;

  // 每次订阅列表变化时重新取一次当前时间，用来算「还有几天到期」。
  const summary = $derived((void i18n.locale, usageRows(subs.list, Date.now(), MAX_ROWS)));
</script>

<section class="card" aria-labelledby="usage-title">
  <div class="card-head">
    <span id="usage-title">{tr('订阅用量', 'Subscription Usage')}</span>
    {#if subs.loaded && subs.list.length > 0}
      <span class="muted">{tr('来自订阅服务器上次更新时给出的数据', 'Reported by subscription servers on last update')}</span>
      <button type="button" class="btn sm right" onclick={() => nav.go('subs')}>{tr('管理订阅', 'Manage')}</button>
    {/if}
  </div>

  {#if !subs.loaded}
    {#if subs.error !== ''}
      <div class="state">
        <span class="grow selectable">{tr(`读取订阅失败：${subs.error}`, `Failed to load subscriptions: ${subs.error}`)}</span>
        <button type="button" class="btn sm" onclick={() => void subs.refresh()}>{tr('重试', 'Retry')}</button>
      </div>
    {:else}
      <div class="state">{tr('正在读取订阅…', 'Loading subscriptions…')}</div>
    {/if}
  {:else if summary.rows.length === 0}
    <div class="state">
      <span class="grow">{tr('还没有订阅。导入订阅后，这里显示各订阅的流量用量与到期时间。', 'No subscriptions yet. Import one to view quota and expiration here.')}</span>
      <button type="button" class="btn sm" onclick={() => nav.go('subs')}>{tr('前往订阅', 'Subscriptions')}</button>
    </div>
  {:else}
    <ul class="rows">
      {#each summary.rows as row (row.id)}
        <li class="row" data-sub={row.id}>
          <div class="sub-row">
            <b class="name">{row.name}</b>
            {#if row.percentText !== ''}
              <span class="pill" class:warn={row.level === 'warn'} class:err={row.level === 'err'}>{row.percentText}</span>
            {/if}
            <span class="usage">{row.usageText}</span>
          </div>
          {#if row.hasBar}
            <div
              class="bar"
              class:warn={row.level === 'warn'}
              class:err={row.level === 'err'}
              role="progressbar"
              aria-label={tr(`${row.name} 已用流量`, `${row.name} used quota`)}
              aria-valuemin="0"
              aria-valuemax="100"
              aria-valuenow={row.percent}
            >
              <i style:width="{row.percent}%"></i>
            </div>
          {/if}
          <dl class="kv">
            <div>
              <dt>{tr('到期', 'Expires')}</dt>
              <dd>
                {row.expiryText}
                {#if row.expiryRemain !== ''}
                  <span class="remain" class:warn={row.expiryLevel === 'warn'} class:err={row.expiryLevel === 'err'}>{row.expiryRemain}</span>
                {/if}
              </dd>
            </div>
            <div><dt>{tr('节点', 'Nodes')}</dt><dd>{row.nodesText}</dd></div>
            <div><dt>{tr('自动更新', 'Auto Update')}</dt><dd>{row.autoText}</dd></div>
          </dl>
        </li>
      {/each}
    </ul>
    {#if summary.hidden > 0}
      <div class="more">{tr(`另有 ${summary.hidden} 个订阅未列出，可在「订阅」页查看。`, `${summary.hidden} more subscription(s) in Subscriptions.`)}</div>
    {/if}
  {/if}
</section>

<style>
  .rows {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 12px;
  }

  .row {
    min-width: 0;
    padding: 12px;
    border-radius: var(--r);
    border: 1px solid var(--border);
    background: var(--surface-2);
  }

  .sub-row {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    margin-bottom: 10px;
  }

  .name {
    min-width: 0;
    overflow-wrap: anywhere;
  }

  .usage {
    margin-left: auto;
    font-size: 12px;
    color: var(--text-3);
    font-variant-numeric: tabular-nums;
  }

  .pill.err {
    background: var(--err-soft);
    color: var(--err);
  }

  .kv {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 8px;
    margin: 12px 0 0;
    font-size: 12px;
  }

  .kv dt {
    color: var(--text-3);
  }

  .kv dd {
    margin: 0;
    overflow-wrap: anywhere;
  }

  .remain {
    display: block;
    color: var(--text-3);
  }

  .remain.warn {
    color: var(--warn);
  }

  .remain.err {
    color: var(--err);
  }

  .state {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
    font-size: 12px;
    color: var(--text-3);
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .more {
    margin-top: 10px;
    font-size: 12px;
    color: var(--text-3);
  }

  @container content (max-width: 560px) {
    .rows {
      grid-template-columns: minmax(0, 1fr);
    }
  }
</style>

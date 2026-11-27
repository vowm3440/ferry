<script lang="ts">
  import Icon from '../../lib/components/Icon.svelte';
  import Sparkline from '../../lib/components/Sparkline.svelte';
  import { formatRate, splitUnit, type ValueWithUnit } from '../../lib/format';
  import { tr } from '../../lib/i18n/index.svelte';
  import { core } from '../../lib/stores/core.svelte';

  const EMPTY: readonly number[] = [];

  const traffic = $derived(core.running ? core.traffic : null);
  const seconds = $derived(traffic === null ? 0 : Math.min(traffic.upHistory.length, traffic.downHistory.length));
  const down = $derived.by<ValueWithUnit | null>(() => (traffic === null ? null : splitUnit(formatRate(traffic.down))));
  const up = $derived.by<ValueWithUnit | null>(() => (traffic === null ? null : splitUnit(formatRate(traffic.up))));
  // 「未运行」同时涵盖未启动、启停过程中与异常三种情况。
  const emptyText = $derived(
    core.running ? tr('正在等待内核上报流量…', 'Waiting for traffic telemetry…') : tr('内核未运行，暂无流量', 'Core stopped; no traffic'),
  );
</script>

<section class="card traffic">
  <div class="card-head">
    {tr('实时流量', 'Live Traffic')}
    {#if seconds >= 2}
      <span class="muted right">{tr(`最近 ${seconds} 秒`, `Last ${seconds}s`)}</span>
    {/if}
  </div>

  <div class="rates" aria-hidden="true">
    <div class="rate down">
      <Icon name="down" />
      <span class="v num">{down?.value ?? '—'}</span>
      {#if down !== null}<span class="u">{down.unit}</span>{/if}
    </div>
    <div class="rate up">
      <Icon name="up" />
      <span class="v num">{up?.value ?? '—'}</span>
      {#if up !== null}<span class="u">{up.unit}</span>{/if}
    </div>
  </div>
  <span class="sr-only">
    {tr(
      `下载 ${down?.value ?? '无数据'} ${down?.unit ?? ''}，上传 ${up?.value ?? '无数据'} ${up?.unit ?? ''}`,
      `Download ${down?.value ?? 'no data'} ${down?.unit ?? ''}, Upload ${up?.value ?? 'no data'} ${up?.unit ?? ''}`,
    )}
  </span>

  <div class="chart">
    <Sparkline up={traffic?.upHistory ?? EMPTY} down={traffic?.downHistory ?? EMPTY} height={96} {emptyText} />
  </div>
</section>

<style>
  .traffic {
    display: flex;
    flex-direction: column;
  }

  /* 与左侧卡片等高时，曲线贴底，不在卡片底部留一块空白。 */
  .chart {
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
  }

  .rates {
    display: flex;
    flex-wrap: wrap;
    gap: 12px 28px;
    margin: 2px 0 10px;
  }

  .rate {
    display: flex;
    align-items: baseline;
    gap: 6px;
  }

  .rate :global(svg.i) {
    align-self: center;
    width: 15px;
    height: 15px;
  }

  .rate.up :global(svg.i) {
    color: var(--chart-up);
  }

  .rate.down :global(svg.i) {
    color: var(--chart-down);
  }

  .rate .v {
    font-size: 24px;
    font-weight: 600;
    letter-spacing: -0.3px;
  }

  .rate .u {
    font-size: 12px;
    color: var(--text-3);
  }
</style>

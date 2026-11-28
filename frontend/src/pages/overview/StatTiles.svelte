<script lang="ts">
  import { formatBytes, formatCount, splitUnit } from '../../lib/format';
  import { tr } from '../../lib/i18n/index.svelte';
  import { core } from '../../lib/stores/core.svelte';

  interface Reading {
    readonly value: string;
    readonly unit: string;
  }

  interface Tile extends Reading {
    readonly label: string;
    readonly title?: string;
  }

  const DASH: Reading = { value: '—', unit: '' };

  function tile(label: string, reading: Reading | null, title?: string): Tile {
    return title === undefined ? { label, ...(reading ?? DASH) } : { label, ...(reading ?? DASH), title };
  }

  /** 停止或没有数据时显示「—」，不显示 0，避免看起来像真实统计。 */
  const tiles = $derived.by<readonly Tile[]>(() => {
    const traffic = core.running ? core.traffic : null;
    const memory = core.running && core.memoryBytes > 0 ? splitUnit(formatBytes(core.memoryBytes)) : null;
    return [
      tile(tr('活动连接', 'Active Conns'), traffic === null ? null : { value: formatCount(traffic.activeConns), unit: '' }),
      tile(tr('内核内存', 'Core Memory'), memory),
      tile(tr('本次上传', 'Session Upload'), traffic === null ? null : splitUnit(formatBytes(traffic.totalUp))),
      tile(
        tr('本次下载', 'Session Download'),
        traffic === null ? null : splitUnit(formatBytes(traffic.totalDown)),
        tr('仅统计经代理节点转发的下载流量，不含直连与国内分流流量', 'Counts proxied download traffic only, excluding direct connections'),
      ),
    ];
  });
</script>

<div class="stats">
  {#each tiles as item (item.label)}
    <div class="card stat" title={item.title}>
      <div class="k">{item.label}</div>
      <div class="v num">{item.value}{#if item.unit !== ''}<small>{item.unit}</small>{/if}</div>
    </div>
  {/each}
</div>

<style>
  .stats {
    display: grid;
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 12px;
  }

  .stat {
    padding: 14px 16px;
  }

  .k {
    font-size: 12px;
    color: var(--text-3);
  }

  .v {
    font-size: 20px;
    font-weight: 600;
    margin-top: 2px;
  }

  .v small {
    font-size: 12px;
    font-weight: 500;
    color: var(--text-3);
    margin-left: 3px;
  }

  @container content (max-width: 720px) {
    .stats {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }
</style>

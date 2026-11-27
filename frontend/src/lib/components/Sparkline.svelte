<script lang="ts">
  import { tr } from '../i18n/index.svelte';

  interface Props {
    /** 每秒上行速率序列（最新在末尾）。 */
    up: readonly number[];
    /** 每秒下行速率序列（最新在末尾）。 */
    down: readonly number[];
    height?: number;
    /** 数据不足两个点时显示的说明。 */
    emptyText?: string;
  }

  const VIEW_W = 100;
  const PAD = 2;

  let { up, down, height = 56, emptyText }: Props = $props();
  const resolvedEmpty = $derived(emptyText ?? tr('暂无流量数据', 'No traffic data'));

  interface Lines {
    readonly up: string;
    readonly down: string;
    readonly ready: boolean;
  }

  const lines = $derived.by<Lines>(() => {
    const count = Math.min(up.length, down.length);
    if (count < 2) return { up: '', down: '', ready: false };

    let max = 0;
    for (let index = 0; index < count; index += 1) {
      max = Math.max(max, up[index] ?? 0, down[index] ?? 0);
    }

    const scale = max > 0 ? 1 / max : 0;
    const toY = (value: number): number => height - PAD - value * scale * (height - PAD * 2);
    const toPath = (series: readonly number[]): string => {
      let path = '';
      for (let index = 0; index < count; index += 1) {
        const x = (index / (count - 1)) * VIEW_W;
        const y = toY(series[index] ?? 0);
        path += `${index === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`;
      }
      return path;
    };

    return { up: toPath(up), down: toPath(down), ready: true };
  });
</script>

{#if lines.ready}
  <svg
    class="spark"
    viewBox={`0 0 ${VIEW_W} ${height}`}
    preserveAspectRatio="none"
    style={`height:${height}px`}
    aria-hidden="true"
    focusable="false"
  >
    <path class="line down" d={lines.down} vector-effect="non-scaling-stroke" />
    <path class="line up" d={lines.up} vector-effect="non-scaling-stroke" />
  </svg>
{:else}
  <div class="placeholder muted" style={`height:${height}px`}>{resolvedEmpty}</div>
{/if}

<style>
  .spark {
    display: block;
    width: 100%;
  }

  .line {
    fill: none;
    stroke-width: 1.6;
    stroke-linejoin: round;
    stroke-linecap: round;
  }

  .line.up {
    stroke: var(--chart-up);
  }

  .line.down {
    stroke: var(--chart-down);
  }

  .placeholder {
    display: grid;
    place-items: center;
    border: 1px dashed var(--border);
    border-radius: var(--r);
    font-size: 12px;
  }
</style>

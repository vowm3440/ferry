<script lang="ts">
  import { iconShapes, type IconName } from '../icons';

  interface Props {
    name: IconName;
    /** 像素尺寸；不传时沿用全局 svg.i 的 16px。 */
    size?: number;
    /** 实心（例如已收藏的星标）。 */
    filled?: boolean;
  }

  let { name, size, filled = false }: Props = $props();

  const shapes = $derived(iconShapes(name));
  const sizeStyle = $derived(size === undefined ? '' : `width:${size}px;height:${size}px;`);
  const fillStyle = $derived(filled ? 'fill:currentColor;' : '');
</script>

<svg class="i" viewBox="0 0 24 24" aria-hidden="true" focusable="false" style={`${sizeStyle}${fillStyle}`}>
  {#each shapes as shape, index (index)}
    {#if typeof shape === 'string'}
      <path d={shape} />
    {:else if shape[0] === 'c'}
      <circle cx={shape[1]} cy={shape[2]} r={shape[3]} />
    {:else}
      <rect x={shape[1]} y={shape[2]} width={shape[3]} height={shape[4]} rx={shape[5]} />
    {/if}
  {/each}
</svg>

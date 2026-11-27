<!--
  规则表格里的条件单元格：最多显示两项，其余折成「+N」；没有条件时显示「任意」。
  规则集引用（geosite: / geoip:）用强调色。完整内容放在 title 里。
-->
<script lang="ts">
  import { tr } from '../../lib/i18n/index.svelte';
  import { summarizeItems } from '../../lib/rules-view';

  interface Props {
    items: readonly string[];
    /** 显示用的文字（例如把应用分组引用换成分组名）；默认原样显示。 */
    text?: (item: string) => string;
  }

  let { items, text = (item: string) => item }: Props = $props();

  const summary = $derived(summarizeItems(items));
  const rest = $derived(items.slice(summary.shown.length).map(text).join(tr('、', ', ')));
</script>

{#if items.length === 0}
  <span class="muted">{tr('任意', 'Any')}</span>
{:else}
  <span class="chips">
    {#each summary.shown as chip (chip.text)}
      <span class="chip" class:set={chip.set} title={chip.text}>{text(chip.text)}</span>
    {/each}
    {#if summary.more > 0}
      <span class="chip more" title={rest}>+{summary.more}</span>
    {/if}
  </span>
{/if}

<style>
  .chips {
    display: flex;
    gap: 4px;
    max-width: 230px;
    overflow: hidden;
  }

  .chip {
    display: inline-block;
    min-width: 0;
    max-width: 150px;
    line-height: 18px;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .chip.more {
    flex: none;
  }

  .chip.set {
    color: var(--accent-strong);
    border-color: transparent;
    background: var(--accent-soft);
  }
</style>

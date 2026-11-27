<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    title: string;
    desc?: string;
    /** 标题旁的标签，例如「未开放」。 */
    tag?: string;
    /** 输入有误时的说明，显示在描述下方。 */
    error?: string;
    /** 控件较宽（分段选择、文本输入）：窄屏下换到标题下方，其余控件始终留在右侧。 */
    wide?: boolean;
    /** 右侧控件。 */
    children?: Snippet;
  }

  let { title, desc = '', tag = '', error = '', wide = false, children }: Props = $props();
</script>

<div class="srow" class:wide>
  <div class="grow">
    <div class="t">
      {title}
      {#if tag !== ''}
        <span class="pill phase">{tag}</span>
      {/if}
    </div>
    {#if desc !== ''}
      <div class="d">{desc}</div>
    {/if}
    {#if error !== ''}
      <div class="e" role="alert">{error}</div>
    {/if}
  </div>
  {#if children}
    <div class="ctl">{@render children()}</div>
  {/if}
</div>

<style>
  .srow {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border);
  }

  .srow:last-child {
    border-bottom: 0;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .t {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    font-weight: 500;
    overflow-wrap: anywhere;
  }

  .d {
    font-size: 12px;
    color: var(--text-3);
    overflow-wrap: anywhere;
  }

  .e {
    font-size: 12px;
    color: var(--err);
    margin-top: 2px;
  }

  .ctl {
    flex: none;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  @container content (max-width: 560px) {
    .srow.wide {
      flex-wrap: wrap;
    }

    .srow.wide .ctl {
      width: 100%;
      justify-content: flex-start;
    }
  }
</style>

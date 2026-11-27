<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    id: string;
    title: string;
    children: Snippet;
  }

  let { id, title, children }: Props = $props();

  const titleId = $derived(`${id}-title`);
</script>

<section class="set-sec" id={id} aria-labelledby={titleId}>
  <!-- tabindex=-1：点击左侧导航后把焦点移到这里，键盘用户接着按 Tab 就在本区内。 -->
  <h2 id={titleId} tabindex="-1">{title}</h2>
  <div class="card">{@render children()}</div>
</section>

<style>
  .set-sec {
    margin-bottom: 16px;
    scroll-margin-top: 8px;
  }

  h2 {
    font-size: 13px;
    font-weight: 600;
    color: var(--text-2);
    margin: 0 0 8px;
    outline-offset: 4px;
  }

  .card {
    padding: 0;
  }
</style>

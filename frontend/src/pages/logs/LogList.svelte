<!--
  日志正文：等宽、可选中的滚动区。
  只有本来就停在底部时才跟着新日志往下滚；用户往上翻看时不打断，改为给一个「回到最新」的按钮。
-->
<script lang="ts">
  import { untrack } from 'svelte';
  import type { Action } from 'svelte/action';
  import Icon from '../../lib/components/Icon.svelte';
  import type { LogEntry } from '../../lib/api';
  import { tr } from '../../lib/i18n/index.svelte';
  import { levelClass, levelLabel } from '../../lib/logs-view';
  import { formatClock } from '../../lib/time';

  interface Props {
    rows: readonly LogEntry[];
    /** 更早的、没有渲染出来的条数。 */
    hidden: number;
  }

  let { rows, hidden }: Props = $props();

  /** 距离底部这么近就算「停在底部」。 */
  const STICK_PX = 24;
  const MIN_HEIGHT_PX = 220;
  /** 找不到滚动容器时，离窗口底边留出的距离。 */
  const FALLBACK_GAP_PX = 24;

  let box = $state<HTMLDivElement | null>(null);
  let atBottom = $state(true);

  function nearBottom(node: HTMLElement): boolean {
    return node.scrollHeight - node.scrollTop - node.clientHeight <= STICK_PX;
  }

  function onScroll(): void {
    if (box !== null) atBottom = nearBottom(box);
  }

  function jumpToLatest(): void {
    if (box === null) return;
    box.scrollTop = box.scrollHeight;
    atBottom = true;
  }

  // 新日志渲染出来之后：原本停在底部（atBottom 由滚动事件维护，追加内容不会改变它）就跟到最新。
  // 首次挂载时 atBottom 为真，所以一打开就落在最新的日志上。
  $effect(() => {
    void rows;
    const node = box;
    if (node === null || !untrack(() => atBottom)) return;
    node.scrollTop = node.scrollHeight;
  });

  /** 让滚动区正好占满页面剩下的高度，这样滚动的是日志本身，而不是整页。 */
  const fillRest: Action<HTMLElement> = (node) => {
    const scroller = node.closest<HTMLElement>('.content');
    const page = node.closest<HTMLElement>('.page');

    const update = (): void => {
      const top = node.getBoundingClientRect().top;
      let available = window.innerHeight - top - FALLBACK_GAP_PX;
      if (scroller !== null) {
        const frame = scroller.getBoundingClientRect();
        const padding = Number.parseFloat(getComputedStyle(scroller).paddingBottom) || 0;
        available = scroller.clientHeight - (top - frame.top + scroller.scrollTop) - padding;
      }
      node.style.height = `${Math.max(MIN_HEIGHT_PX, Math.floor(available))}px`;
      // 高度变了之后，原本停在底部的继续停在底部。
      if (atBottom) node.scrollTop = node.scrollHeight;
    };

    const observer = new ResizeObserver(update);
    if (scroller !== null) observer.observe(scroller);
    if (page !== null) observer.observe(page);
    // 页面的入场动画带一点位移，结束后再量一次。
    page?.addEventListener('animationend', update);
    update();

    return {
      destroy(): void {
        observer.disconnect();
        page?.removeEventListener('animationend', update);
      },
    };
  };
</script>

<div class="wrap">
  <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
  <div
    class="log selectable"
    role="region"
    aria-label={tr('日志内容', 'Log entries')}
    tabindex="0"
    bind:this={box}
    use:fillRest
    onscroll={onScroll}
  >
    {#if hidden > 0}
      <p class="note num">{tr(`仅显示最近 ${rows.length} 条，更早的 ${hidden} 条未显示；可以用搜索或级别筛选查找。`, `Showing latest ${rows.length} entries (${hidden} older hidden); use search or level filter to find earlier logs.`)}</p>
    {/if}
    {#each rows as entry (entry.seq)}
      <div class="row">
        <span class="time num">{formatClock(entry.time)}</span>
        <span class="lv {levelClass(entry.level)}">{levelLabel(entry.level)}</span>
        <span class="msg">{entry.message}</span>
      </div>
    {/each}
  </div>

  {#if !atBottom}
    <button type="button" class="btn sm latest" onclick={jumpToLatest}>
      <Icon name="down" />{tr('回到最新', 'Jump to Latest')}
    </button>
  {/if}
</div>

<style>
  .wrap {
    position: relative;
  }

  .log {
    height: 420px;
    overflow-y: auto;
    padding: 8px 0;
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    background: var(--surface);
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 22px;
    /* 系统默认的滚动条在深色主题下是一条白带，改成跟随主题的细滚动条。 */
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
  }

  .log:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }

  .row {
    display: grid;
    grid-template-columns: 90px 56px 1fr;
    gap: 10px;
    padding: 0 14px;
  }

  .row:hover {
    background: var(--muted-soft);
  }

  .time {
    color: var(--text-3);
  }

  .lv {
    font-weight: 600;
  }

  .lv.debug {
    color: var(--text-3);
  }

  .lv.info {
    color: var(--accent-strong);
  }

  .lv.warn {
    color: var(--warn);
  }

  .lv.error {
    color: var(--err);
  }

  .msg {
    min-width: 0;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }

  .note {
    padding: 0 14px 6px;
    font-family: var(--font-sans);
    color: var(--text-3);
  }

  .latest {
    position: absolute;
    right: 18px;
    bottom: 12px;
    box-shadow: var(--shadow);
  }

  @container content (max-width: 520px) {
    .row {
      grid-template-columns: 64px 46px 1fr;
      gap: 8px;
      padding: 0 10px;
    }
  }
</style>

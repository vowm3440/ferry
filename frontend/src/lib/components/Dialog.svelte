<!--
  对话框。用法：

    <Dialog open={editing} title="编辑规则" onclose={() => (editing = false)}>
      …表单…
      {#snippet footer()}
        <button type="button" class="btn" onclick={…}>取消</button>
        <button type="button" class="btn primary" onclick={…}>保存</button>
      {/snippet}
    </Dialog>

  - 打开时焦点移入：优先带 data-autofocus 的元素，否则是面板本身；关闭后还给打开它的元素。
  - Esc、右上角按钮、点遮罩都会请求关闭；busy 时一概不关。
  - 遮罩从标题栏下方开始，所以对话框开着时仍然可以拖动、最小化、关闭窗口。
  - 内容被挪到 body 下渲染，不在页面的 content 容器里：对话框内不要写 @container content。
-->
<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { Action } from 'svelte/action';
  import { tr } from '../i18n/index.svelte';
  import { modal } from '../stores/modal.svelte';
  import Icon from './Icon.svelte';

  interface Props {
    open: boolean;
    title: string;
    /** 用户请求关闭。busy 时不会触发。 */
    onclose: () => void;
    /** 提交中：暂不允许关闭。 */
    busy?: boolean;
    /** 面板宽度（px）；窗口更窄时自动收窄。 */
    width?: number;
    /** 点遮罩是否关闭。已经填了内容的表单应设为 false，避免误触丢掉输入。 */
    backdropClose?: boolean;
    children: Snippet;
    footer?: Snippet;
  }

  let { open, title, onclose, busy = false, width = 480, backdropClose = true, children, footer }: Props = $props();

  const uid = $props.id();
  const titleId = `${uid}-title`;

  const TABBABLE = 'a[href], button, input, select, textarea, [tabindex]';

  /** 按下时是否落在遮罩上：从面板里拖选文字拖到遮罩上松手，不算点遮罩。 */
  let pressedOnBackdrop = false;

  function requestClose(): void {
    if (busy) return;
    onclose();
  }

  function tabbables(panel: HTMLElement): HTMLElement[] {
    return [...panel.querySelectorAll<HTMLElement>(TABBABLE)].filter(
      (element) => element.tabIndex >= 0 && !element.hasAttribute('disabled') && element.getClientRects().length > 0,
    );
  }

  /** Tab 只在面板内循环；焦点跑到面板外（例如点过标题栏）时拉回来。 */
  function trapTab(event: KeyboardEvent, panel: HTMLElement): void {
    const items = tabbables(panel);
    const first = items[0];
    const last = items[items.length - 1];
    if (first === undefined || last === undefined) {
      event.preventDefault();
      panel.focus();
      return;
    }
    const active = document.activeElement;
    if (!(active instanceof Node) || !panel.contains(active)) {
      event.preventDefault();
      (event.shiftKey ? last : first).focus();
    } else if (event.shiftKey && (active === first || active === panel)) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && active === last) {
      event.preventDefault();
      first.focus();
    }
  }

  const layer: Action<HTMLElement> = (node) => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    // 页面内容区是容器查询的容器，较旧的内核会把 fixed 定位困在它里面；挂到 body 下才能盖住整个窗口。
    document.body.append(node);
    const token = modal.push();
    const panel = node.querySelector<HTMLElement>('[role="dialog"]');
    (panel?.querySelector<HTMLElement>('[data-autofocus]') ?? panel)?.focus();

    const onKeydown = (event: KeyboardEvent): void => {
      // 输入法组字时的 Esc 是取消组字，不是关对话框。
      if (panel === null || !modal.isTop(token) || event.defaultPrevented || event.isComposing) return;
      if (event.key === 'Escape') {
        event.preventDefault();
        requestClose();
      } else if (event.key === 'Tab') {
        trapTab(event, panel);
      }
    };
    document.addEventListener('keydown', onKeydown);

    return {
      destroy(): void {
        document.removeEventListener('keydown', onKeydown);
        modal.remove(token);
        node.remove();
        if (opener !== null && opener.isConnected) opener.focus();
      },
    };
  };

  function onPointerDown(event: PointerEvent): void {
    pressedOnBackdrop = event.target === event.currentTarget;
  }

  function onBackdropClick(event: MouseEvent): void {
    const hit = pressedOnBackdrop && event.target === event.currentTarget;
    pressedOnBackdrop = false;
    if (hit && backdropClose) requestClose();
  }
</script>

{#if open}
  <div class="overlay" role="presentation" use:layer onpointerdown={onPointerDown} onclick={onBackdropClick}>
    <div
      class="panel"
      role="dialog"
      aria-modal="true"
      aria-labelledby={titleId}
      aria-busy={busy}
      tabindex="-1"
      style:width="min({width}px, 100%)"
    >
      <header class="head">
        <h2 id={titleId}>{title}</h2>
        <button type="button" class="icon-btn" aria-label={tr('关闭', 'Close')} title={tr('关闭', 'Close')} aria-disabled={busy} onclick={requestClose}>
          <Icon name="close" />
        </button>
      </header>
      <div class="body">{@render children()}</div>
      {#if footer}
        <footer class="foot">{@render footer()}</footer>
      {/if}
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    /* 36px 是标题栏的高度（TitleBar.svelte）。 */
    inset: 36px 0 0;
    /* 低于全局提示（Toasts 的 50）：对话框开着时提示仍然看得见、点得到。 */
    z-index: 40;
    display: grid;
    place-items: center;
    padding: 16px;
    background: rgba(6, 7, 9, 0.56);
    animation: overlay-in 0.15s var(--ease);
  }

  :global([data-theme='light']) .overlay {
    background: rgba(30, 40, 60, 0.3);
  }

  .panel {
    display: flex;
    flex-direction: column;
    max-width: 100%;
    max-height: calc(100vh - 68px);
    min-height: 0;
    border-radius: var(--r-lg);
    border: 1px solid var(--border-strong);
    background: var(--surface);
    box-shadow: var(--shadow);
    animation: panel-in 0.18s var(--ease);
  }

  .panel:focus-visible {
    outline: none;
  }

  .head {
    flex: none;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 10px 8px 18px;
  }

  h2 {
    flex: 1;
    min-width: 0;
    font-size: 15px;
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .body {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 4px 18px 18px;
  }

  .body::-webkit-scrollbar {
    width: 10px;
  }

  .body::-webkit-scrollbar-thumb {
    background: var(--surface-3);
    border-radius: 10px;
    border: 3px solid var(--surface);
  }

  .foot {
    flex: none;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding: 12px 18px 14px;
    border-top: 1px solid var(--border);
  }

  @keyframes overlay-in {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  @keyframes panel-in {
    from {
      opacity: 0;
      transform: translateY(6px) scale(0.985);
    }
    to {
      opacity: 1;
      transform: none;
    }
  }
</style>

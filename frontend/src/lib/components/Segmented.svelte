<script lang="ts" generics="T extends string">
  interface Option {
    readonly value: T;
    readonly label: string;
  }

  interface Props {
    options: ReadonlyArray<Option>;
    value: T;
    /** 无障碍名称，例如「代理模式」。 */
    label: string;
    disabled?: boolean;
    /** 提交中：保持焦点但暂不响应。 */
    busy?: boolean;
    onchange?: (next: T) => void;
  }

  let { options, value, label, disabled = false, busy = false, onchange }: Props = $props();

  let group = $state<HTMLDivElement | null>(null);

  const activeIndex = $derived(options.findIndex((option) => option.value === value));

  function choose(next: T): void {
    if (disabled || busy || next === value) return;
    onchange?.(next);
  }

  function focusAt(index: number): void {
    group?.querySelectorAll<HTMLButtonElement>('button')[index]?.focus();
  }

  /** 单选组的键盘约定：方向键移动并选中，Home/End 跳到两端。 */
  function onKeydown(event: KeyboardEvent, index: number): void {
    const last = options.length - 1;
    let target = -1;
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') target = index === last ? 0 : index + 1;
    else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') target = index === 0 ? last : index - 1;
    else if (event.key === 'Home') target = 0;
    else if (event.key === 'End') target = last;
    if (target < 0) return;

    event.preventDefault();
    focusAt(target);
    const option = options[target];
    if (option !== undefined) choose(option.value);
  }
</script>

<div class="seg" role="radiogroup" aria-label={label} aria-busy={busy} bind:this={group}>
  {#each options as option, index (option.value)}
    {@const active = index === activeIndex}
    <button
      type="button"
      role="radio"
      aria-checked={active}
      tabindex={active || (activeIndex < 0 && index === 0) ? 0 : -1}
      class:on={active}
      {disabled}
      onclick={() => choose(option.value)}
      onkeydown={(event) => onKeydown(event, index)}
    >
      {option.label}
    </button>
  {/each}
</div>

<style>
  .seg {
    display: inline-flex;
    padding: 2px;
    border-radius: 9px;
    background: var(--surface-2);
    border: 1px solid var(--border);
  }

  .seg[aria-busy='true'] {
    opacity: 0.7;
  }

  button {
    height: 26px;
    padding: 0 14px;
    border: 0;
    border-radius: 7px;
    background: transparent;
    color: var(--text-2);
    cursor: pointer;
    font-size: 12.5px;
    white-space: nowrap;
    transition:
      background var(--dur),
      color var(--dur);
  }

  button:hover:not(:disabled):not(.on) {
    color: var(--text);
  }

  button.on {
    background: var(--surface-3);
    color: var(--text);
    box-shadow: 0 1px 2px rgba(0, 0, 0, 0.18);
  }

  :global([data-theme='light']) button.on {
    background: #fff;
  }

  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>

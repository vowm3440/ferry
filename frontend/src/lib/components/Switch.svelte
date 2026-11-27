<script lang="ts">
  interface Props {
    checked: boolean;
    /** 无障碍名称（开关本身没有可见文字）。 */
    label: string;
    disabled?: boolean;
    onchange?: (next: boolean) => void;
  }

  let { checked, label, disabled = false, onchange }: Props = $props();
</script>

<button
  type="button"
  class="switch"
  class:on={checked}
  role="switch"
  aria-checked={checked}
  aria-label={label}
  {disabled}
  onclick={() => onchange?.(!checked)}
></button>

<style>
  .switch {
    position: relative;
    display: inline-block;
    flex: none;
    width: 34px;
    height: 20px;
    padding: 0;
    border-radius: 20px;
    border: 1px solid var(--border-strong);
    background: var(--surface-3);
    vertical-align: middle;
    cursor: pointer;
    transition: background 0.18s var(--ease);
  }

  .switch::after {
    content: '';
    position: absolute;
    top: 2px;
    left: 2px;
    width: 14px;
    height: 14px;
    border-radius: 50%;
    background: var(--text-2);
    transition:
      transform 0.18s var(--ease),
      background 0.18s;
  }

  .switch.on {
    background: var(--accent);
    border-color: transparent;
  }

  .switch.on::after {
    transform: translateX(14px);
    background: #fff;
  }

  .switch:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>

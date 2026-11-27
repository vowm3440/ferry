<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { IconName } from '../icons';
  import Icon from './Icon.svelte';

  interface Props {
    icon: IconName;
    title: string;
    description?: string;
    /** 危险/异常语气（例如连接失败）。 */
    tone?: 'neutral' | 'error';
    /** 操作按钮。 */
    children?: Snippet;
  }

  let { icon, title, description = '', tone = 'neutral', children }: Props = $props();
</script>

<div class="empty" class:error={tone === 'error'}>
  <div class="ic"><Icon name={icon} size={22} /></div>
  <h2>{title}</h2>
  {#if description !== ''}
    <p class="selectable">{description}</p>
  {/if}
  {#if children}
    <div class="actions">{@render children()}</div>
  {/if}
</div>

<style>
  .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 8px;
    padding: 36px 20px;
  }

  .ic {
    width: 48px;
    height: 48px;
    display: grid;
    place-items: center;
    border-radius: 14px;
    background: var(--muted-soft);
    color: var(--text-3);
    margin-bottom: 4px;
  }

  .error .ic {
    background: var(--err-soft);
    color: var(--err);
  }

  h2 {
    font-size: 15px;
    font-weight: 600;
  }

  p {
    color: var(--text-2);
    max-width: 52ch;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
    gap: 8px;
    margin-top: 8px;
  }
</style>

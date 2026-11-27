<script lang="ts">
  import { tick } from 'svelte';
  import { tr } from '../i18n/index.svelte';
  import { connection } from '../stores/connection.svelte';
  import EmptyState from './EmptyState.svelte';

  const noBridgeDesc = $derived(
    tr(
      '当前页面没有检测到 Ferry 桌面外壳的通信通道，所以不会显示节点、流量或内核状态，也不会向任何服务发起请求。请通过 Ferry 桌面程序打开本页面；也可以进入演示模式查看界面结构（数据为合成示例）。',
      'No Ferry desktop bridge was detected, so nodes, traffic, and core state are hidden and no network requests are made. Open this page via the Ferry desktop app, or enter Demo Mode with synthetic data.',
    ),
  );

  /** 当前显示的「重新检测 / 重试」按钮（两者不会同时出现）。 */
  let retryButton = $state<HTMLButtonElement | null>(null);

  /**
   * 检测期间这里会换成「正在连接」，原来的按钮随之消失、键盘焦点落空；
   * 结果仍是未连接或失败时把焦点交还给新的按钮，键盘用户可以接着按回车。
   */
  async function retry(): Promise<void> {
    await connection.connect();
    await tick();
    if (document.activeElement === document.body) retryButton?.focus();
  }
</script>

{#if connection.phase === 'checking'}
  <section class="card checking" role="status">
    <span class="spinner" aria-hidden="true"></span>
    <span>{tr('正在连接桌面后端…', 'Connecting to desktop backend…')}</span>
  </section>
{:else if connection.phase === 'no-bridge'}
  <section class="card">
    <EmptyState icon="unplug" title={tr('未连接桌面后端', 'Desktop Backend Disconnected')} description={noBridgeDesc}>
      <button type="button" class="btn" bind:this={retryButton} onclick={() => void retry()}>{tr('重新检测', 'Retry Check')}</button>
      <button type="button" class="btn primary" onclick={() => void connection.enterDemo()}>{tr('进入演示模式', 'Enter Demo Mode')}</button>
    </EmptyState>
  </section>
{:else if connection.phase === 'failed'}
  <section class="card">
    <EmptyState icon="alert" tone="error" title={tr('无法连接桌面后端', 'Failed to Connect to Backend')} description={connection.error}>
      <button type="button" class="btn primary" bind:this={retryButton} onclick={() => void retry()}>{tr('重试', 'Retry')}</button>
    </EmptyState>
  </section>
{/if}

<style>
  .checking {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--text-2);
  }

  .spinner {
    width: 14px;
    height: 14px;
    flex: none;
    border-radius: 50%;
    border: 2px solid var(--accent-soft);
    border-top-color: var(--accent);
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>

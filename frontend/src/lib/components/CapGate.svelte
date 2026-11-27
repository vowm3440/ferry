<!--
  按后端公布的能力决定是否渲染内容（System.Capabilities，见 stores/caps.svelte.ts）。

    <CapGate cap={CAP_RULES} feature="规则" icon="rules">
      …只有后端公布了这项能力才渲染…
    </CapGate>

  没有这项能力时如实说明并且不渲染入口，也就不会去调用后端没有公布的接口。
-->
<script lang="ts">
  import type { Snippet } from 'svelte';
  import type { IconName } from '../icons';
  import { tr } from '../i18n/index.svelte';
  import { caps } from '../stores/caps.svelte';
  import EmptyState from './EmptyState.svelte';

  interface Props {
    /** api 导出的 CAP_* 常量。 */
    cap: string;
    /** 功能名称，用在说明里，例如「规则」。 */
    feature: string;
    icon?: IconName;
    children: Snippet;
  }

  let { cap, feature, icon = 'info', children }: Props = $props();
</script>

{#if caps.has(cap)}
  {@render children()}
{:else if caps.error !== ''}
  <div class="card">
    <EmptyState {icon} tone="error" title={tr('无法确认后端支持的功能', 'Failed to Query Capabilities')} description={caps.error}>
      <button type="button" class="btn sm" onclick={() => void caps.refresh()}>{tr('重试', 'Retry')}</button>
    </EmptyState>
  </div>
{:else if !caps.loaded}
  <div class="card">
    <EmptyState {icon} title={tr('正在读取…', 'Loading…')} description={tr('正在向后端确认可用的功能。', 'Checking capabilities with the desktop backend.')} />
  </div>
{:else}
  <div class="card">
    <EmptyState
      {icon}
      title={tr(`「${feature}」在当前版本不可用`, `"${feature}" is unavailable in this build`)}
      description={tr('后端没有提供这项功能，界面不会调用未提供的接口。', 'This feature is not advertised by the backend, so the UI will not invoke it.')}
    />
  </div>
{/if}

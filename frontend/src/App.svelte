<script lang="ts">
  import { onMount, untrack } from 'svelte';
  import BackendNotice from './lib/components/BackendNotice.svelte';
  import DeepLinkDialog from './lib/components/DeepLinkDialog.svelte';
  import DemoBanner from './lib/components/DemoBanner.svelte';
  import ShareDialog from './lib/components/ShareDialog.svelte';
  import Sidebar from './lib/components/Sidebar.svelte';
  import TitleBar from './lib/components/TitleBar.svelte';
  import Toasts from './lib/components/Toasts.svelte';
  import { caps } from './lib/stores/caps.svelte';
  import { connection } from './lib/stores/connection.svelte';
  import { conns } from './lib/stores/conns.svelte';
  import { core } from './lib/stores/core.svelte';
  import { deepLink } from './lib/stores/deeplink.svelte';
  import { logs } from './lib/stores/logs.svelte';
  import { modal } from './lib/stores/modal.svelte';
  import { nav, PAGES, type PageId } from './lib/stores/nav.svelte';
  import { nodes } from './lib/stores/nodes.svelte';
  import { rules } from './lib/stores/rules.svelte';
  import { settings } from './lib/stores/settings.svelte';
  import { shareRequest } from './lib/stores/share-request.svelte';
  import { subs } from './lib/stores/subs.svelte';
  import { system } from './lib/stores/system.svelte';
  import ConnsPage from './pages/ConnsPage.svelte';
  import LogsPage from './pages/LogsPage.svelte';
  import NodesPage from './pages/NodesPage.svelte';
  import OverviewPage from './pages/OverviewPage.svelte';
  import RulesPage from './pages/RulesPage.svelte';
  import SettingsPage from './pages/SettingsPage.svelte';
  import SubsPage from './pages/SubsPage.svelte';

  let contentEl = $state<HTMLElement | null>(null);

  // 仅用于比较「上一次的值」，不参与渲染，所以不做成响应式状态。
  let lastPage: PageId | null = null;
  let lastStatus: string | null = null;

  const ready = $derived(connection.phase === 'ready');

  onMount(() => {
    void connection.connect();
  });

  // 后端就绪后挂接各 store；断开或退出演示时解除订阅并清空数据。
  $effect(() => {
    const backend = connection.backend;
    if (backend === null) return;
    untrack(() => {
      // 能力最先读：其余 store 靠它决定哪些接口可以调用。
      void caps.attach(backend);
      void core.attach(backend);
      void nodes.attach(backend);
      void settings.attach(backend);
      void subs.attach(backend);
      void rules.attach(backend);
      void conns.attach(backend);
      void logs.attach(backend);
      void system.attach(backend);
      deepLink.attach(backend);
    });
    return () => {
      shareRequest.close();
      deepLink.detach();
      caps.detach();
      core.detach();
      nodes.detach();
      settings.detach();
      subs.detach();
      rules.detach();
      conns.detach();
      logs.detach();
      system.detach();
    };
  });

  // 内核启动或停止会改变分组的实际选中项，重新读取节点列表。
  $effect(() => {
    const status = core.state?.status ?? null;
    if (status === lastStatus) return;
    const previous = lastStatus;
    lastStatus = status;
    if (previous !== null && status !== null) untrack(() => void nodes.refresh());
  });

  // 切换页面后回到顶部。
  $effect(() => {
    const page = nav.current;
    if (page === lastPage) return;
    lastPage = page;
    contentEl?.scrollTo({ top: 0 });
  });

  /** Ctrl+1…7 按侧栏顺序切换页面。有对话框打开时不响应，以免把正在填写的内容连同页面一起丢掉。 */
  function onKeydown(event: KeyboardEvent): void {
    if (!event.ctrlKey || event.altKey || event.metaKey || event.shiftKey) return;
    if (modal.active) return;
    const index = Number.parseInt(event.key, 10);
    if (!Number.isInteger(index) || index < 1 || index > PAGES.length) return;
    event.preventDefault();
    nav.goIndex(index - 1);
  }
</script>

<svelte:window onkeydown={onKeydown} />

<Sidebar />

<div class="main">
  <TitleBar />
  {#if connection.isDemo}
    <DemoBanner />
  {/if}

  <main class="content" bind:this={contentEl}>
    {#if !ready}
      <div class="page"><BackendNotice /></div>
    {:else if nav.current === 'overview'}
      <OverviewPage />
    {:else if nav.current === 'nodes'}
      <NodesPage />
    {:else if nav.current === 'subs'}
      <SubsPage />
    {:else if nav.current === 'rules'}
      <RulesPage />
    {:else if nav.current === 'conns'}
      <ConnsPage />
    {:else if nav.current === 'logs'}
      <LogsPage />
    {:else}
      <SettingsPage />
    {/if}
  </main>
</div>

{#if ready}
  <ShareDialog />
  <DeepLinkDialog />
{/if}

<Toasts />

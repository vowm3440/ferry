<script lang="ts">
  import { CAP_LATENCY, CAP_NODE_GROUPS, type Group } from '../lib/api';
  import EmptyState from '../lib/components/EmptyState.svelte';
  import Icon from '../lib/components/Icon.svelte';
  import Segmented from '../lib/components/Segmented.svelte';
  import Switch from '../lib/components/Switch.svelte';
  import { i18n, tr } from '../lib/i18n/index.svelte';
  import { buildGroupViews, latencyTargets, type MemberView, type NodeFilter, type NodeSort } from '../lib/nodes-view';
  import { caps } from '../lib/stores/caps.svelte';
  import { core } from '../lib/stores/core.svelte';
  import { nav } from '../lib/stores/nav.svelte';
  import { nodes } from '../lib/stores/nodes.svelte';
  import { toasts } from '../lib/stores/toasts.svelte';
  import GroupDialog from './nodes/GroupDialog.svelte';
  import GroupSection from './nodes/GroupSection.svelte';

  let query = $state('');
  let sort = $state<NodeSort>('default');
  let favoritesOnly = $state(false);
  /** 被折叠的分组 id；默认全部展开。 */
  let collapsed = $state.raw<readonly string[]>([]);
  let groupDialogOpen = $state(false);
  let editingGroup = $state.raw<Group | null>(null);

  const filter = $derived<NodeFilter>({ query, favoritesOnly, sort });
  const groups = $derived((void i18n.locale, buildGroupViews(nodes.list, filter)));
  const hasNodes = $derived(nodes.list.groups.length > 0 || nodes.list.nodes.length > 0);
  const filtering = $derived(query.trim() !== '' || favoritesOnly);
  const noMatch = $derived(filtering && groups.every((group) => group.members.length === 0));
  const selectable = $derived(core.running && !core.transitioning);
  /** 内核正在处理别的操作：相关按钮暂不响应，但不禁用，以免丢掉键盘焦点。 */
  const coreBusy = $derived(core.pending !== null || core.transitioning);
  const canLatency = $derived(caps.has(CAP_LATENCY));
  const canEditGroups = $derived(caps.has(CAP_NODE_GROUPS));
  const sortOptions = $derived<ReadonlyArray<{ value: NodeSort; label: string }>>([
    { value: 'default', label: tr('默认', 'Default') },
    ...(canLatency ? [{ value: 'latency' as const, label: tr('延迟', 'Latency') }] : []),
    { value: 'name', label: tr('名称', 'Name') },
  ]);
  const summary = $derived(
    tr(
      `共 ${nodes.list.nodes.length} 个节点 · ${nodes.list.groups.length} 个分组`,
      `${nodes.list.nodes.length} nodes · ${nodes.list.groups.length} groups`,
    ),
  );

  function toggleGroup(id: string): void {
    collapsed = collapsed.includes(id) ? collapsed.filter((item) => item !== id) : [...collapsed, id];
  }

  function clearFilter(): void {
    query = '';
    favoritesOnly = false;
  }

  function refresh(): void {
    if (nodes.loading) return;
    void nodes.refresh();
  }

  async function selectMember(groupId: string, member: MemberView): Promise<void> {
    const result = await nodes.select(groupId, member.id);
    if (result.ok) toasts.ok(tr(`已切换到「${member.name}」`, `Switched to "${member.name}"`));
    else toasts.error(result.message);
  }

  async function setFavorite(member: MemberView, next: boolean): Promise<void> {
    const result = await nodes.setFavorite(member.id, next);
    if (!result.ok) toasts.error(result.message);
  }

  /** 测速：不传分组就是全部节点。测速要经过运行中的内核，没运行时如实说明。 */
  async function testLatency(groupId?: string): Promise<void> {
    if (nodes.testing) return;
    if (!core.running) {
      toasts.info(tr('请先启动内核再测速', 'Start core before testing latency'));
      return;
    }
    const result = await nodes.testLatency(latencyTargets(nodes.list, groupId));
    if (!result.ok) toasts.error(result.message);
  }

  async function startCore(): Promise<void> {
    if (coreBusy) return;
    const result = await core.start();
    if (!result.ok) toasts.error(result.message);
  }

  function openNewGroup(): void {
    editingGroup = null;
    groupDialogOpen = true;
  }

  function openEditGroup(groupId: string): void {
    const found = nodes.list.groups.find((g) => g.id === groupId) ?? null;
    if (found === null) return;
    editingGroup = found;
    groupDialogOpen = true;
  }
</script>

<section class="page" aria-labelledby="nodes-title">
  <div class="page-head">
    <h1 id="nodes-title">{tr('节点', 'Proxies')}</h1>
    {#if nodes.loaded && hasNodes}
      <span class="sub num">{summary}</span>
    {/if}
    <div class="actions">
      {#if canEditGroups && nodes.loaded && nodes.list.nodes.length > 0}
        <button type="button" class="btn" onclick={openNewGroup}>
          <Icon name="plus" />{tr('新建分组', 'New Group')}
        </button>
      {/if}
      {#if canLatency && nodes.loaded && nodes.list.nodes.length > 0}
        <button
          type="button"
          class="btn"
          aria-disabled={nodes.testing}
          title={core.running ? tr('测试全部节点的延迟', 'Test latency for all nodes') : tr('内核运行后才能测速', 'Start core to test latency')}
          onclick={() => void testLatency()}
        >
          <Icon name="zap" />{nodes.testing ? tr('测速中…', 'Testing…') : tr('全部测速', 'Test All')}
        </button>
      {/if}
      <button
        type="button"
        class="icon-btn"
        class:spin={nodes.loading}
        aria-disabled={nodes.loading}
        aria-label={tr('刷新节点列表', 'Refresh node list')}
        title={tr('刷新节点列表', 'Refresh node list')}
        onclick={refresh}
      >
        <Icon name="refresh" />
      </button>
    </div>
  </div>

  {#if !nodes.loaded}
    <div class="card">
      {#if nodes.error !== ''}
        <EmptyState icon="alert" tone="error" title={tr('无法读取节点', 'Failed to Load Nodes')} description={nodes.error}>
          <button type="button" class="btn" onclick={() => void nodes.refresh()}>{tr('重试', 'Retry')}</button>
        </EmptyState>
      {:else}
        <EmptyState icon="nodes" title={tr('正在读取节点…', 'Loading nodes…')} description={tr('正在向桌面后端请求节点与分组列表。', 'Requesting nodes and proxy groups from backend.')} />
      {/if}
    </div>
  {:else if !hasNodes}
    <div class="card">
      <EmptyState
        icon="nodes"
        title={tr('尚未导入节点', 'No Nodes Imported')}
        description={tr('在「订阅」页导入节点后，会显示在这里。', 'Nodes imported in Subscriptions will appear here.')}
      >
        <button type="button" class="btn primary" onclick={() => nav.go('subs')}>{tr('前往订阅', 'Subscriptions')}</button>
      </EmptyState>
    </div>
  {:else}
    {#if nodes.error !== ''}
      <div class="inline-error gap" role="alert">
        <Icon name="alert" />
        <span class="grow">{tr(`刷新节点失败：${nodes.error}`, `Failed to refresh nodes: ${nodes.error}`)}</span>
        <button type="button" class="btn sm" onclick={() => void nodes.refresh()}>{tr('重试', 'Retry')}</button>
      </div>
    {/if}

    {#if !core.running}
      <div class="inline-note gap">
        <Icon name="info" />
        <span class="grow">
          {canLatency
            ? tr('内核未运行：启动后才能切换出口节点和测速，收藏不受影响。', 'Core stopped: start core to switch nodes and test latency; favorites still work.')
            : tr('内核未运行：启动后才能切换出口节点，收藏不受影响。', 'Core stopped: start core to switch nodes; favorites still work.')}
        </span>
        <button
          type="button"
          class="btn sm"
          disabled={core.state === null}
          aria-disabled={coreBusy}
          onclick={() => void startCore()}
        >
          {core.transitioning ? tr('处理中…', 'Working…') : tr('启动内核', 'Start Core')}
        </button>
      </div>
    {/if}

    <div class="toolbar">
      <label class="input search">
        <Icon name="search" />
        <input type="search" placeholder={tr('搜索名称、协议或地址', 'Search name, protocol, or host')} aria-label={tr('搜索节点', 'Search nodes')} bind:value={query} />
      </label>
      <div class="field">
        <span class="muted" aria-hidden="true">{tr('排序', 'Sort')}</span>
        <Segmented options={sortOptions} value={sort} label={tr('排序方式', 'Sort order')} onchange={(next) => (sort = next)} />
      </div>
      <div class="field">
        <Switch checked={favoritesOnly} label={tr('只看收藏', 'Favorites only')} onchange={(next: boolean) => (favoritesOnly = next)} />
        <span aria-hidden="true">{tr('只看收藏', 'Favorites only')}</span>
      </div>
    </div>

    {#if noMatch}
      <div class="card">
        <EmptyState icon="search" title={tr('没有匹配的节点', 'No Matching Nodes')} description={tr('换一个关键词，或清除筛选条件后再试。', 'Try another keyword or clear your filters.')}>
          <button type="button" class="btn" onclick={clearFilter}>{tr('清除筛选', 'Clear Filters')}</button>
        </EmptyState>
      </div>
    {:else}
      <div class="groups">
        {#each groups as group (group.id)}
          <GroupSection
            {group}
            open={!collapsed.includes(group.id)}
            {selectable}
            onToggle={() => toggleGroup(group.id)}
            onSelect={(member: MemberView) => selectMember(group.id, member)}
            onFavorite={(member: MemberView, next: boolean) => void setFavorite(member, next)}
            onTest={canLatency ? () => void testLatency(group.id) : undefined}
            onEdit={canEditGroups && group.id !== '' ? () => openEditGroup(group.id) : undefined}
            testing={nodes.testing}
          />
        {/each}
      </div>
    {/if}
  {/if}
</section>

<GroupDialog
  open={groupDialogOpen}
  target={editingGroup}
  list={nodes.list}
  onclose={() => (groupDialogOpen = false)}
/>

<style>
  .gap {
    align-items: center;
    margin-bottom: 12px;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px 14px;
    margin-bottom: 12px;
  }

  .search {
    flex: 1 1 240px;
    max-width: 360px;
  }

  .field {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    font-size: 12.5px;
  }

  .groups {
    display: grid;
    gap: 12px;
  }

  .spin :global(svg.i) {
    animation: spin 0.9s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>

<script lang="ts">
  import type { Group, NodeList } from '../../lib/api';
  import Dialog from '../../lib/components/Dialog.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import Segmented from '../../lib/components/Segmented.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import { nodeDisplayName, protocolBadge } from '../../lib/nodes-view';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';

  interface Props {
    open: boolean;
    /** null 表示新建分组；非 null 表示编辑已有分组。 */
    target: Group | null;
    list: NodeList;
    onclose: () => void;
  }

  let { open, target, list, onclose }: Props = $props();

  let name = $state('');
  let kind = $state<'select' | 'urltest'>('select');
  let members = $state.raw<readonly string[]>([]);
  let testUrl = $state('https://cp.cloudflare.com/generate_204');
  let intervalText = $state('5');
  let search = $state('');
  let error = $state('');
  let syncedKey = '';

  const busy = $derived(nodes.savingGroup);
  const typeOptions = $derived([
    { value: 'select' as const, label: tr('手动选择 (Select)', 'Manual (Select)') },
    { value: 'urltest' as const, label: tr('自动测速 (URLTest)', 'Auto (URLTest)') },
  ]);

  const candidates = $derived.by(() => {
    const q = search.trim().toLowerCase();
    const selfId = target?.id ?? '';
    const subGroups = list.groups
      .filter((g) => g.id !== selfId && g.id !== 'proxy')
      .map((g) => ({ id: g.id, label: g.name || g.id, badge: tr('组', 'GRP') }));
    const nodeItems = list.nodes.map((n) => ({
      id: n.id,
      label: nodeDisplayName(n),
      badge: protocolBadge(n.type),
    }));
    const all = [...subGroups, ...nodeItems];
    return q === '' ? all : all.filter((item) => item.label.toLowerCase().includes(q) || item.badge.toLowerCase().includes(q));
  });

  $effect(() => {
    const key = open ? (target?.id ?? '__new__') : '';
    if (key === syncedKey) return;
    syncedKey = key;
    if (!open) return;
    if (target !== null) {
      name = target.name;
      kind = target.type === 'urltest' ? 'urltest' : 'select';
      members = [...target.members];
      testUrl = target.testUrl || 'https://cp.cloudflare.com/generate_204';
      intervalText = String(target.intervalMin > 0 ? target.intervalMin : 5);
    } else {
      name = '';
      kind = 'select';
      members = list.nodes.slice(0, 2).map((n) => n.id);
      testUrl = 'https://cp.cloudflare.com/generate_204';
      intervalText = '5';
    }
    search = '';
    error = '';
  });

  function toggleMember(id: string): void {
    members = members.includes(id) ? members.filter((m) => m !== id) : [...members, id];
    error = '';
  }

  function selectAllVisible(): void {
    const ids = new Set(members);
    for (const item of candidates) ids.add(item.id);
    members = [...ids];
  }

  function close(): void {
    if (!busy) onclose();
  }

  async function remove(): Promise<void> {
    if (target === null || busy) return;
    const res = await nodes.removeGroup(target.id);
    if (!res.ok) {
      error = res.message;
      return;
    }
    toasts.ok(tr(`已删除分组「${target.name}」`, `Deleted group "${target.name}"`));
    onclose();
  }

  async function submit(event?: SubmitEvent): Promise<void> {
    event?.preventDefault();
    if (busy) return;
    const trimmedName = name.trim();
    if (trimmedName === '') {
      error = tr('请填写分组名称', 'Please enter a group name');
      return;
    }
    if (members.length === 0) {
      error = tr('请至少选择一个成员节点或子分组', 'Select at least one node or sub-group');
      return;
    }
    const mins = Number.parseInt(intervalText.trim(), 10);
    if (kind === 'urltest' && (!Number.isInteger(mins) || mins < 1 || mins > 1440)) {
      error = tr('测速间隔须为 1–1440 分钟的整数', 'Interval must be 1–1440 minutes');
      return;
    }
    error = '';
    const payload: Group = {
      id: target?.id ?? '',
      name: trimmedName,
      type: kind,
      members: [...members],
      testUrl: kind === 'urltest' ? testUrl.trim() : '',
      intervalMin: kind === 'urltest' ? mins : 0,
    };
    const res = await nodes.saveGroup(payload);
    if (!res.ok) {
      error = res.message;
      return;
    }
    toasts.ok(target === null ? tr(`已创建分组「${trimmedName}」`, `Created group "${trimmedName}"`) : tr(`已保存分组「${trimmedName}」`, `Saved group "${trimmedName}"`));
    onclose();
  }
</script>

<Dialog
  {open}
  title={target === null ? tr('新建代理分组', 'New Proxy Group') : tr('编辑代理分组', 'Edit Proxy Group')}
  {busy}
  width={480}
  backdropClose={false}
  onclose={close}
>
  <form id="group-edit-form" class="form" novalidate onsubmit={(event) => void submit(event)}>
    <label class="field">
      <span class="label">{tr('分组名称', 'Group Name')}</span>
      <input class="control" type="text" maxlength="64" autocomplete="off" data-autofocus bind:value={name} />
    </label>

    <div class="field">
      <span class="label">{tr('策略类型', 'Strategy Type')}</span>
      <Segmented options={typeOptions} value={kind} label={tr('策略类型', 'Strategy Type')} onchange={(next) => (kind = next)} />
    </div>

    {#if kind === 'urltest'}
      <div class="row2">
        <label class="field">
          <span class="label">{tr('测速地址', 'Test URL')}</span>
          <input class="control mono" type="url" autocomplete="off" bind:value={testUrl} />
        </label>
        <label class="field">
          <span class="label">{tr('间隔（分钟）', 'Interval (min)')}</span>
          <input class="control num" type="number" min="1" max="1440" bind:value={intervalText} />
        </label>
      </div>
    {/if}

    <div class="field">
      <div class="mem-head">
        <span class="label">{tr(`成员（已选 ${members.length} 项）`, `Members (${members.length} selected)`)}</span>
        <button type="button" class="link-btn" onclick={selectAllVisible}>{tr('全选当前列表', 'Select visible')}</button>
      </div>
      <label class="input">
        <Icon name="search" />
        <input type="search" placeholder={tr('筛选节点或子分组', 'Filter nodes or groups')} bind:value={search} />
      </label>
      <div class="mem-list" role="group" aria-label={tr('分组成员', 'Group Members')}>
        {#each candidates as item (item.id)}
          {@const checked = members.includes(item.id)}
          <label class="mem-item" class:checked>
            <input type="checkbox" {checked} onchange={() => toggleMember(item.id)} />
            <span class="pill">{item.badge}</span>
            <span class="mem-name">{item.label}</span>
          </label>
        {:else}
          <div class="muted empty-mem">{tr('没有可选成员', 'No matching members')}</div>
        {/each}
      </div>
    </div>

    {#if error !== ''}
      <div class="inline-error" role="alert"><Icon name="alert" /><span>{error}</span></div>
    {/if}
  </form>

  {#snippet footer()}
    {#if target !== null && target.id !== 'proxy'}
      <button type="button" class="btn danger mr-auto" aria-disabled={busy} onclick={() => void remove()}>
        <Icon name="trash" />{tr('删除分组', 'Delete')}
      </button>
    {/if}
    <button type="button" class="btn" aria-disabled={busy} onclick={close}>{tr('取消', 'Cancel')}</button>
    <button type="submit" form="group-edit-form" class="btn primary" aria-disabled={busy}>
      {busy ? tr('保存中…', 'Saving…') : tr('保存', 'Save')}
    </button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: 12px;
  }

  .row2 {
    display: grid;
    grid-template-columns: minmax(0, 1fr) 110px;
    gap: 10px;
  }

  .mem-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .link-btn {
    border: 0;
    background: none;
    color: var(--accent);
    font-size: 12px;
    cursor: pointer;
    padding: 0;
  }

  .mem-list {
    max-height: 210px;
    overflow-y: auto;
    border: 1px solid var(--border);
    border-radius: var(--r);
    padding: 4px;
    display: grid;
    gap: 2px;
    background: var(--surface-2);
  }

  .mem-item {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 6px 8px;
    border-radius: 6px;
    cursor: pointer;
    font-size: 13px;
  }

  .mem-item:hover,
  .mem-item.checked {
    background: var(--surface);
  }

  .mem-name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .empty-mem {
    padding: 12px;
    text-align: center;
    font-size: 12px;
  }

  .mr-auto {
    margin-right: auto;
  }
</style>

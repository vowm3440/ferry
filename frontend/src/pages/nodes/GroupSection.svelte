<script lang="ts">
  import Icon from '../../lib/components/Icon.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import type { GroupKind, GroupView, MemberView } from '../../lib/nodes-view';
  import NodeCard from './NodeCard.svelte';

  interface Props {
    group: GroupView;
    open: boolean;
    /** 当前可以切换出口（内核运行中且没有其他操作在进行）。 */
    selectable: boolean;
    onToggle: () => void;
    onSelect: (member: MemberView) => Promise<void>;
    onFavorite: (member: MemberView, next: boolean) => void;
    /** 给这个分组测速；不提供时不显示按钮（后端没有测速能力）。 */
    onTest?: () => void;
    /** 编辑自定义分组；不提供时不显示按钮。 */
    onEdit?: () => void;
    /** 有一次测速正在进行。 */
    testing?: boolean;
  }

  let { group, open, selectable, onToggle, onSelect, onFavorite, onTest, onEdit, testing = false }: Props = $props();

  const KIND_LABELS = $derived<Readonly<Record<GroupKind, string>>>({
    select: tr('手动选择', 'Select'),
    urltest: tr('自动测速', 'URLTest'),
    other: tr('内核分组', 'Core Group'),
    orphan: tr('未加入分组', 'Ungrouped'),
  });

  const manual = $derived(group.kind === 'select');
  const countText = $derived(
    group.members.length === group.total
      ? tr(`${group.total} 个节点`, `${group.total} nodes`)
      : tr(`显示 ${group.members.length} / ${group.total} 个节点`, `Showing ${group.members.length} / ${group.total} nodes`),
  );
</script>

<section class="card group" aria-label={group.name}>
  <div class="bar-row">
    <h2>
      <button type="button" class="head" aria-expanded={open} onclick={onToggle}>
        <span class="chev" class:closed={!open}><Icon name="chev" size={14} /></span>
        <span class="gname">{group.name}</span>
        {#if group.kind !== 'orphan'}
          <span class="pill">{KIND_LABELS[group.kind]}</span>
        {/if}
        <span class="count muted">{countText}</span>
        <span class="sel muted">
          {#if manual && group.selectedName !== ''}
            {tr(`当前：${group.selectedName}`, `Current: ${group.selectedName}`)}
          {:else if group.kind === 'urltest'}
            {tr('出口由内核自动选择', 'Auto-selected by core')}
          {/if}
        </span>
      </button>
    </h2>
    {#if onEdit !== undefined}
      <button type="button" class="icon-btn sm" aria-label={tr(`编辑分组 ${group.name}`, `Edit group ${group.name}`)} title={tr('编辑分组', 'Edit Group')} onclick={onEdit}>
        <Icon name="edit" size={15} />
      </button>
    {/if}
    {#if onTest !== undefined && group.total > 0}
      <button type="button" class="btn sm test" aria-disabled={testing} aria-label={tr('测速分组 ' + group.name, 'Test group ' + group.name)} onclick={onTest}>
        <Icon name="zap" />{tr('测速', 'Test')}
      </button>
    {/if}
  </div>

  {#if open}
    {#if group.kind === 'orphan'}
      <p class="hint muted">{tr('这些节点没有被任何分组引用，不能在这里选为出口。', 'These nodes are not in any group and cannot be selected directly here.')}</p>
    {/if}
    <ul class="rows">
      {#each group.members as member (member.id)}
        <NodeCard {member} groupId={group.id} {manual} {selectable} {onSelect} {onFavorite} />
      {:else}
        <li class="no-match muted">{tr('没有匹配的节点', 'No matching nodes')}</li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .group {
    padding: 6px 8px 8px;
  }

  .bar-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }

  h2 {
    flex: 1;
    min-width: 0;
    font-size: inherit;
    font-weight: inherit;
  }

  .test {
    flex: none;
    margin-right: 4px;
  }

  .head {
    width: 100%;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
    padding: 8px;
    border: 0;
    border-radius: var(--r);
    background: transparent;
    color: inherit;
    cursor: pointer;
    text-align: left;
  }

  .head:hover {
    background: var(--muted-soft);
  }

  .chev {
    display: grid;
    place-items: center;
    color: var(--text-3);
    transition: transform var(--dur) var(--ease);
  }

  .chev.closed {
    transform: rotate(-90deg);
  }

  .gname {
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .count {
    font-size: 12px;
    font-variant-numeric: tabular-nums;
  }

  .sel {
    margin-left: auto;
    font-size: 12px;
    overflow-wrap: anywhere;
  }

  .hint {
    font-size: 12px;
    padding: 0 8px 4px 32px;
  }

  .rows {
    list-style: none;
    margin: 2px 0 0;
    padding: 0;
    display: grid;
    gap: 2px;
  }

  .no-match {
    padding: 10px 12px;
    font-size: 12.5px;
  }

  @container content (max-width: 620px) {
    .count,
    .sel {
      display: none;
    }
  }
</style>

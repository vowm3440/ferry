<!--
  应用分组：把一组应用起个名字，规则里用 @group:<id> 引用。分组成员不能再引用别的分组。
  仍被规则引用的分组不能删除（后端会拒绝，说明原样显示）。
-->
<script lang="ts">
  import { tick } from 'svelte';
  import { APP_GROUP_PREFIX, type AppGroup } from '../../lib/api';
  import ConfirmDialog from '../../lib/components/ConfirmDialog.svelte';
  import EmptyState from '../../lib/components/EmptyState.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import { MAX_APP_GROUPS } from '../../lib/rules-validate';
  import { rules } from '../../lib/stores/rules.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import AppGroupDialog from './AppGroupDialog.svelte';
  import RuleChips from './RuleChips.svelte';

  /** 正在编辑的分组；id 为空表示新建，null 表示没有打开编辑框。 */
  let editing = $state.raw<AppGroup | null>(null);
  let removing = $state.raw<AppGroup | null>(null);
  let removeBusy = $state(false);
  let removeError = $state('');
  let host = $state<HTMLElement | null>(null);

  const full = $derived(rules.groups.length >= MAX_APP_GROUPS);

  /** 引用这个分组的规则数。 */
  function usedBy(id: string): number {
    return rules.list.filter((rule) => rule.apps.includes(APP_GROUP_PREFIX + id)).length;
  }

  function create(): void {
    if (rules.busy) return;
    if (full) {
      toasts.info(tr(`应用分组最多 ${MAX_APP_GROUPS} 个`, `Maximum of ${MAX_APP_GROUPS} app groups`));
      return;
    }
    editing = { id: '', name: '', apps: [] };
  }

  function askRemove(group: AppGroup): void {
    if (rules.busy) return;
    removeError = '';
    removing = group;
  }

  async function remove(): Promise<void> {
    const target = removing;
    if (target === null || removeBusy) return;
    removeBusy = true;
    const result = await rules.removeGroup(target.id);
    removeBusy = false;
    if (!result.ok) {
      removeError = result.message;
      return;
    }
    removing = null;
    toasts.ok(tr(`已删除应用分组「${target.name}」`, `Removed app group "${target.name}"`));
    // 被删的那一行连同打开确认框的按钮都不在了：把焦点交给「新建分组」。
    await tick();
    host?.querySelector<HTMLElement>('[data-group-create]')?.focus();
  }
</script>

<div bind:this={host}>
  <div class="bar-row">
    <span class="muted">{tr('在规则的「应用程序」里引用分组，一次匹配其中全部应用。', 'Reference an app group in a rule to match all of its applications at once.')}</span>
    <button type="button" class="btn sm" data-group-create aria-disabled={rules.busy} onclick={create}><Icon name="plus" />{tr('新建分组', 'New Group')}</button>
  </div>

  {#if rules.groupsError !== ''}
    <div class="inline-error gap" role="alert">
      <Icon name="alert" />
      <span class="grow">{tr(`读取应用分组失败：${rules.groupsError}`, `Failed to load app groups: ${rules.groupsError}`)}</span>
      <button type="button" class="btn sm" onclick={() => void rules.refresh()}>{tr('重试', 'Retry')}</button>
    </div>
  {/if}

  <div class="card table-card">
    {#if rules.groups.length === 0}
      <EmptyState icon="layers" title={tr('还没有应用分组', 'No App Groups Yet')} description={tr('例如把几个浏览器放进一个分组，再在规则里引用它。', 'Group multiple apps (e.g. browsers) together and reference the group in routing rules.')} />
    {:else}
      <div class="tbl-wrap">
        <table class="tbl" aria-label={tr('应用分组', 'App groups')}>
          <thead>
            <tr>
              <th>{tr('名称', 'Name')}</th>
              <th>{tr('应用', 'Apps')}</th>
              <th class="r">{tr('应用数', 'Count')}</th>
              <th class="r">{tr('引用它的规则', 'Used By')}</th>
              <th><span class="sr-only">{tr('操作', 'Actions')}</span></th>
            </tr>
          </thead>
          <tbody>
            {#each rules.groups as group (group.id)}
              <tr data-group-id={group.id}>
                <td class="name" title={group.name}>{group.name}</td>
                <td><RuleChips items={group.apps} /></td>
                <td class="r num">{group.apps.length}</td>
                <td class="r num">{usedBy(group.id)}</td>
                <td class="ops">
                  <button type="button" class="icon-btn" aria-label={tr(`编辑分组「${group.name}」`, `Edit group "${group.name}"`)} title={tr('编辑', 'Edit')} aria-disabled={rules.busy} onclick={() => !rules.busy && (editing = group)}>
                    <Icon name="edit" />
                  </button>
                  <button type="button" class="icon-btn" aria-label={tr(`删除分组「${group.name}」`, `Remove group "${group.name}"`)} title={tr('删除', 'Remove')} aria-disabled={rules.busy} onclick={() => askRemove(group)}>
                    <Icon name="trash" />
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
</div>

{#if editing !== null}
  <AppGroupDialog group={editing} onclose={() => (editing = null)} />
{/if}

<ConfirmDialog
  open={removing !== null}
  title={tr('删除应用分组', 'Remove App Group')}
  message={removing === null ? '' : tr(`删除「${removing.name}」？仍被规则引用的分组不能删除。`, `Remove "${removing.name}"? Groups still referenced by rules cannot be removed.`)}
  confirmLabel={tr('删除', 'Remove')}
  busyLabel={tr('删除中…', 'Removing…')}
  danger
  busy={removeBusy}
  onconfirm={() => void remove()}
  oncancel={() => (removing = null)}
>
  {#if removeError !== ''}
    <div class="inline-error" role="alert"><Icon name="alert" /><span class="selectable">{removeError}</span></div>
  {/if}
</ConfirmDialog>

<style>
  .bar-row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    margin-bottom: 10px;
    font-size: 12.5px;
  }

  .gap {
    margin-bottom: 10px;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  /* 表头里只给读屏用的文字是绝对定位的：让它跟着滚动容器走，否则表格横向滚动时会把整页撑宽。 */
  .tbl-wrap {
    position: relative;
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
  }

  .table-card {
    padding: 4px 6px;
    overflow: hidden;
  }

  .name {
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
    font-weight: 500;
  }

  .ops {
    width: 72px;
    text-align: right;
  }

  /* .icon-btn 默认是块级的 grid：在单元格里并排要改成行内。 */
  .ops .icon-btn {
    display: inline-grid;
    vertical-align: middle;
    white-space: nowrap;
  }
</style>

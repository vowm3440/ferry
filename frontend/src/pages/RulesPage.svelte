<!--
  规则页：路由规则（表格、编辑、排序、测试、导入）、规则集与应用分组。
  每一块都按后端公布的能力显示；没有公布的不渲染，也不调用。
-->
<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { CAP_APP_GROUPS, CAP_RULES, CAP_RULE_SETS, type Rule } from '../lib/api';
  import CapGate from '../lib/components/CapGate.svelte';
  import ConfirmDialog from '../lib/components/ConfirmDialog.svelte';
  import EmptyState from '../lib/components/EmptyState.svelte';
  import Icon from '../lib/components/Icon.svelte';
  import { tr } from '../lib/i18n/index.svelte';
  import { MAX_RULES } from '../lib/rules-validate';
  import { blankRuleForm, formFromRule, type RuleForm } from '../lib/rules-view';
  import { caps } from '../lib/stores/caps.svelte';
  import { core } from '../lib/stores/core.svelte';
  import { ruleDraft } from '../lib/stores/rule-draft.svelte';
  import { rules } from '../lib/stores/rules.svelte';
  import { toasts } from '../lib/stores/toasts.svelte';
  import AppGroupsTab from './rules/AppGroupsTab.svelte';
  import ImportDialog from './rules/ImportDialog.svelte';
  import LazyPresetDialog from './rules/LazyPresetDialog.svelte';
  import RuleEditor from './rules/RuleEditor.svelte';
  import RuleSetsTab from './rules/RuleSetsTab.svelte';
  import RuleTester from './rules/RuleTester.svelte';
  import RulesTable from './rules/RulesTable.svelte';

  type TabId = 'rules' | 'sets' | 'groups';

  let tab = $state<TabId>('rules');
  /** 编辑器里的表单；null 表示没有打开。 */
  let editing = $state.raw<RuleForm | null>(null);
  let importing = $state(false);
  let presetOpen = $state(false);
  let highlightedId = $state('');
  let removing = $state.raw<Rule | null>(null);
  let removeBusy = $state(false);
  let removeError = $state('');
  let page = $state<HTMLElement | null>(null);

  const available = $derived(caps.has(CAP_RULES));
  const tabs = $derived<ReadonlyArray<{ id: TabId; label: string }>>([
    { id: 'rules', label: tr('路由规则', 'Routing Rules') },
    ...(caps.has(CAP_RULE_SETS) ? [{ id: 'sets' as const, label: tr('规则集', 'Rule Sets') }] : []),
    ...(caps.has(CAP_APP_GROUPS) ? [{ id: 'groups' as const, label: tr('应用分组', 'App Groups') }] : []),
  ]);
  /** 当前页签对应的能力不在了（例如能力列表重新读取之后）就退回路由规则。 */
  const current = $derived<TabId>(tabs.some((item) => item.id === tab) ? tab : 'rules');

  onMount(() => {
    // 连接页「据此创建规则」交过来的预填规则：尚未保存，id 为空。
    const draft = ruleDraft.take();
    if (draft !== null) editing = formFromRule({ ...draft, id: '' });
    // 连接页「命中溯源」点过来的规则 ID：高亮并滚动定位。
    const focusId = ruleDraft.takeHighlight();
    if (focusId !== '') {
      tab = 'rules';
      highlightedId = focusId;
      void tick().then(() => {
        page?.querySelector<HTMLElement>(`[data-rule-id="${CSS.escape(focusId)}"]`)?.scrollIntoView({ behavior: 'smooth', block: 'center' });
      });
      const timer = setTimeout(() => {
        if (highlightedId === focusId) highlightedId = '';
      }, 3600);
      return () => clearTimeout(timer);
    }
  });

  function create(): void {
    if (rules.busy) return;
    if (rules.list.length >= MAX_RULES) {
      toasts.info(tr(`规则最多 ${MAX_RULES} 条`, `Maximum of ${MAX_RULES} rules`));
      return;
    }
    tab = 'rules';
    editing = blankRuleForm();
  }

  function openImport(): void {
    if (rules.busy) return;
    tab = 'rules';
    importing = true;
  }

  function onTabKey(event: KeyboardEvent, index: number): void {
    const step = event.key === 'ArrowRight' ? 1 : event.key === 'ArrowLeft' ? -1 : 0;
    const next = tabs[(index + step + tabs.length) % tabs.length];
    if (step === 0 || next === undefined) return;
    event.preventDefault();
    tab = next.id;
    page?.querySelector<HTMLElement>(`[data-tab="${next.id}"]`)?.focus();
  }

  function askRemove(rule: Rule): void {
    removeError = '';
    removing = rule;
  }

  async function remove(): Promise<void> {
    const target = removing;
    if (target === null || removeBusy) return;
    const index = rules.list.findIndex((rule) => rule.id === target.id);
    removeBusy = true;
    const result = await rules.remove(target.id);
    removeBusy = false;
    if (!result.ok) {
      removeError = result.message;
      return;
    }
    removing = null;
    toasts.ok(tr(`已删除规则「${target.name}」`, `Removed rule "${target.name}"`));
    // 被删的那一行不在了：焦点交给顶上来的那一行，没有可编辑的行时交给「新建规则」。
    await tick();
    const next = rules.list[index]?.pinned === true ? rules.list[index - 1] : rules.list[index];
    const row = next === undefined ? null : page?.querySelector<HTMLElement>(`[data-rule-id="${CSS.escape(next.id)}"] [data-row-focus]`);
    (row ?? page?.querySelector<HTMLElement>('[data-rule-create]'))?.focus();
  }
</script>

<section class="page" aria-labelledby="rules-title" bind:this={page}>
  <div class="page-head">
    <h1 id="rules-title">{tr('规则', 'Rules')}</h1>
    <span class="sub num">{tr('自上而下匹配，首条命中生效', 'Evaluated top-down; first match wins')}</span>
    {#if available}
      <div class="actions">
        <button type="button" class="btn ghost" data-lazy-preset aria-disabled={rules.busy} onclick={() => { if (!rules.busy) { tab = 'rules'; presetOpen = true; } }}><Icon name="zap" />{tr('懒人预设', 'Lazy Preset')}</button>
        <button type="button" class="btn ghost" aria-disabled={rules.busy} onclick={openImport}><Icon name="import" />{tr('导入', 'Import')}</button>
        <button type="button" class="btn primary" data-rule-create aria-disabled={rules.busy} onclick={create}><Icon name="plus" />{tr('新建规则', 'New Rule')}</button>
      </div>
    {/if}
  </div>

  <CapGate cap={CAP_RULES} feature={tr('规则', 'Rules')} icon="rules">
    {#if tabs.length > 1}
      <div class="tabs" role="tablist" aria-label={tr('规则分类', 'Rule sections')}>
        {#each tabs as item, index (item.id)}
          <button
            type="button"
            role="tab"
            id={`rules-tab-${item.id}`}
            data-tab={item.id}
            class:on={current === item.id}
            aria-selected={current === item.id}
            aria-controls="rules-panel"
            tabindex={current === item.id ? 0 : -1}
            onclick={() => (tab = item.id)}
            onkeydown={(event) => onTabKey(event, index)}
          >
            {item.label}
          </button>
        {/each}
      </div>
    {/if}

    <div id="rules-panel" role={tabs.length > 1 ? 'tabpanel' : undefined} aria-labelledby={tabs.length > 1 ? `rules-tab-${current}` : undefined}>
      {#if current === 'sets'}
        <RuleSetsTab />
      {:else if current === 'groups'}
        <AppGroupsTab />
      {:else if !rules.loaded}
        <div class="card">
          {#if rules.error !== ''}
            <EmptyState icon="alert" tone="error" title={tr('无法读取规则', 'Failed to Load Rules')} description={rules.error}>
              <button type="button" class="btn" onclick={() => void rules.refresh()}>{tr('重试', 'Retry')}</button>
            </EmptyState>
          {:else}
            <EmptyState icon="rules" title={tr('正在读取规则…', 'Loading rules…')} description={tr('正在向后端请求规则列表。', 'Requesting rules from backend.')} />
          {/if}
        </div>
      {:else}
        {#if rules.error !== ''}
          <div class="inline-error gap" role="alert">
            <Icon name="alert" />
            <span class="grow">{tr(`刷新规则失败：${rules.error}`, `Failed to refresh rules: ${rules.error}`)}</span>
            <button type="button" class="btn sm" onclick={() => void rules.refresh()}>{tr('重试', 'Retry')}</button>
          </div>
        {/if}
        <RulesTable {highlightedId} onedit={(rule) => (editing = formFromRule(rule))} onremove={askRemove} />
        {#if core.running}
          <p class="muted apply-note">{tr('改动规则会立即应用：内核运行中会短暂重启，已有的连接会断开。', 'Rule changes apply immediately: running core will briefly restart and drop active connections.')}</p>
        {/if}
        <RuleTester />
      {/if}
    </div>

    {#if editing !== null}
      <RuleEditor initial={editing} onclose={() => (editing = null)} />
    {/if}
    {#if importing}
      <ImportDialog onclose={() => (importing = false)} />
    {/if}
    {#if presetOpen}
      <LazyPresetDialog onclose={() => (presetOpen = false)} />
    {/if}
    <ConfirmDialog
      open={removing !== null}
      title={tr('删除规则', 'Remove Rule')}
      message={removing === null ? '' : tr(`删除「${removing.name}」？删除后无法恢复。`, `Remove "${removing.name}"? This cannot be undone.`)}
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
  </CapGate>
</section>

<style>
  .tabs {
    display: flex;
    gap: 2px;
    margin-bottom: 12px;
    border-bottom: 1px solid var(--border);
  }

  .tabs button {
    position: relative;
    height: 34px;
    padding: 0 12px;
    border: 0;
    border-radius: 6px 6px 0 0;
    background: transparent;
    color: var(--text-2);
    cursor: pointer;
    white-space: nowrap;
  }

  .tabs button:hover {
    color: var(--text);
  }

  .tabs button.on {
    color: var(--text);
  }

  .tabs button.on::after {
    content: '';
    position: absolute;
    left: 12px;
    right: 12px;
    bottom: -1px;
    height: 2px;
    border-radius: 2px;
    background: var(--accent);
  }

  .gap {
    margin-bottom: 10px;
  }

  .grow {
    flex: 1;
    min-width: 0;
  }

  .apply-note {
    margin: 8px 2px 0;
    font-size: 12px;
  }
</style>

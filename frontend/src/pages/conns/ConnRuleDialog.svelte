<!--
  实时连接一键建规则与命中溯源对话框：
  - 查看该连接当前命中的规则并一键跳转定位；
  - 一键为该进程（进程名或完整路径）新建规则；
  - 一键为该域名（精确域名或泛域名）新建规则；
  - 一键把该进程加入已有的 @group:id 应用组（或新建应用组）。
-->
<script lang="ts">
  import { untrack } from 'svelte';
  import { CAP_APP_GROUPS, DEFAULT_TARGET_ID, type Connection, type Rule } from '../../lib/api';
  import Dialog from '../../lib/components/Dialog.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import Segmented from '../../lib/components/Segmented.svelte';
  import { appName, suggestedActionFor, targetText, wildcardDomainOf } from '../../lib/conns-view';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { actionOptions, ruleNumber } from '../../lib/rules-view';
  import { caps } from '../../lib/stores/caps.svelte';
  import { nav } from '../../lib/stores/nav.svelte';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import { ruleDraft } from '../../lib/stores/rule-draft.svelte';
  import { rules } from '../../lib/stores/rules.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';

  interface Props {
    conn: Connection;
    onclose: () => void;
    onjumprule: (ruleId: string) => void;
  }

  let { conn, onclose, onjumprule }: Props = $props();

  type MatchScope = 'process' | 'domain';

  const processBase = $derived(conn.process || (conn.processPath.split(/[\\/]/).findLast((p) => p !== '') ?? ''));
  const hasProcess = $derived(processBase !== '');
  const hasHost = $derived(conn.host !== '');
  const wildcard = $derived(wildcardDomainOf(conn.host));
  const matchedNo = $derived(conn.ruleId ? ruleNumber(rules.list, conn.ruleId) : 0);

  let scope = $state<MatchScope>(untrack(() => (conn.process || conn.processPath ? 'process' : 'domain')));
  let useFullPath = $state(false);
  let useWildcard = $state(untrack(() => wildcardDomainOf(conn.host) !== ''));
  let action = $state<string>(untrack(() => suggestedActionFor(conn)));
  let targetId = $state<string>(untrack(() => nodes.list.groups[0]?.id ?? DEFAULT_TARGET_ID));
  let newGroupName = $state('');
  let busy = $state(false);
  let error = $state('');

  const actOpts = $derived((void i18n.locale, actionOptions()));
  const targetOptions = $derived.by(() => {
    const groups = nodes.list.groups;
    const fallback = groups.find((group) => group.id === DEFAULT_TARGET_ID);
    return [
      { value: DEFAULT_TARGET_ID, label: fallback === undefined ? tr('默认代理', 'Default Proxy') : fallback.name },
      ...groups.filter((group) => group.id !== DEFAULT_TARGET_ID).map((group) => ({ value: group.id, label: group.name })),
    ];
  });

  function chosenProcess(): string {
    return useFullPath && conn.processPath !== '' ? conn.processPath : processBase;
  }

  function chosenHost(): string {
    return useWildcard && wildcard !== '' ? wildcard : conn.host;
  }

  function buildDraft(): Rule {
    const isProcess = scope === 'process' && hasProcess;
    const subject = isProcess ? chosenProcess() : chosenHost();
    const base: Rule = {
      id: '',
      name: isProcess ? tr(`应用：${processBase}`, `App: ${processBase}`) : tr(`域名：${subject}`, `Host: ${subject}`),
      enabled: true,
      apps: isProcess ? [subject] : [],
      hosts: !isProcess && subject !== '' ? [subject] : [],
      ports: [],
      network: 'any',
      action,
      resolveBefore: !isProcess,
    };
    return action === 'proxy' ? { ...base, targetId: targetId || DEFAULT_TARGET_ID } : base;
  }

  async function saveRuleDirectly(): Promise<void> {
    if (busy) return;
    const draft = buildDraft();
    if (draft.apps.length === 0 && draft.hosts.length === 0) {
      error = tr('缺少可用的进程或目标主机条件', 'Missing process or host condition');
      return;
    }
    busy = true;
    error = '';
    const res = await rules.save(draft);
    busy = false;
    if (!res.ok) {
      error = res.message;
      return;
    }
    toasts.ok(tr(`已新建规则「${draft.name}」并生效`, `Created rule "${draft.name}"`));
    onclose();
  }

  function openInEditor(): void {
    if (busy) return;
    ruleDraft.offer(buildDraft());
    onclose();
    nav.go('rules');
  }

  async function addToGroup(groupId: string, groupName: string): Promise<void> {
    if (busy || !hasProcess) return;
    const app = chosenProcess();
    busy = true;
    error = '';
    const res = await rules.addAppToGroup(groupId, app);
    busy = false;
    if (!res.ok) {
      error = res.message;
      return;
    }
    toasts.ok(tr(`已将「${app}」加入应用组「${groupName}」`, `Added "${app}" to app group "${groupName}"`));
    onclose();
  }

  async function createGroupWithApp(): Promise<void> {
    if (busy || !hasProcess) return;
    const name = newGroupName.trim();
    if (name === '') {
      error = tr('请填写新应用分组名称', 'Please enter a new app group name');
      return;
    }
    const app = chosenProcess();
    busy = true;
    error = '';
    const res = await rules.saveGroup({ id: '', name, apps: [app] });
    busy = false;
    if (!res.ok) {
      error = res.message;
      return;
    }
    toasts.ok(tr(`已新建应用组「${name}」并加入「${app}」`, `Created app group "${name}" with "${app}"`));
    onclose();
  }
</script>

<Dialog open title={tr('快捷分流与命中溯源', 'Quick Route & Attribution')} {busy} width={540} onclose={() => { if (!busy) onclose(); }}>
  <div class="body">
    <div class="meta-card">
      <div class="meta-row">
        <span class="muted">{tr('进程', 'Process')}</span>
        <span class="mono" title={conn.processPath || appName(conn)}>{appName(conn)}</span>
      </div>
      <div class="meta-row">
        <span class="muted">{tr('目标', 'Target')}</span>
        <span class="mono">{targetText(conn)}</span>
      </div>
      <div class="meta-row">
        <span class="muted">{tr('当前命中', 'Matched Rule')}</span>
        <span class="hit">
          {#if matchedNo > 0}
            <span class="num">#{matchedNo}</span>
          {/if}
          <b>{conn.ruleName || tr('未知规则', 'Unknown Rule')}</b>
          {#if conn.ruleId}
            <button type="button" class="btn sm ghost" onclick={() => { onclose(); onjumprule(conn.ruleId!); }}>
              {tr('去规则页查看', 'Locate Rule')}
            </button>
          {/if}
        </span>
      </div>
      {#if conn.ruleDetail}
        <div class="detail mono muted">{conn.ruleDetail}</div>
      {/if}
    </div>

    <div class="section">
      <span class="label">{tr('一键新建规则', 'One-Click New Rule')}</span>
      <div class="scope-tabs">
        <button type="button" class="btn sm" class:primary={scope === 'process'} disabled={!hasProcess} onclick={() => (scope = 'process')}>
          {tr(`按进程 (${processBase || '未知'})`, `By Process (${processBase || 'N/A'})`)}
        </button>
        <button type="button" class="btn sm" class:primary={scope === 'domain'} disabled={!hasHost} onclick={() => (scope = 'domain')}>
          {tr(`按目标 (${conn.host || '未知'})`, `By Host (${conn.host || 'N/A'})`)}
        </button>
      </div>

      {#if scope === 'process' && hasProcess && conn.processPath !== ''}
        <label class="check">
          <input type="checkbox" checked={useFullPath} onchange={(e) => (useFullPath = e.currentTarget.checked)} />
          <span>{tr('使用完整可执行文件路径精确匹配', 'Match full executable path')}</span>
        </label>
      {:else if scope === 'domain' && wildcard !== ''}
        <label class="check">
          <input type="checkbox" checked={useWildcard} onchange={(e) => (useWildcard = e.currentTarget.checked)} />
          <span>{tr(`匹配整个根域名（${wildcard}）`, `Match root wildcard (${wildcard})`)}</span>
        </label>
      {/if}

      <div class="row">
        <div class="field">
          <span class="label">{tr('新出口动作', 'Target Action')}</span>
          <Segmented options={actOpts} value={action} label={tr('动作', 'Action')} onchange={(next) => (action = next)} />
        </div>
        {#if action === 'proxy'}
          <label class="field grow">
            <span class="label">{tr('代理分组', 'Proxy Group')}</span>
            <select class="select" value={targetId} onchange={(e) => (targetId = e.currentTarget.value)}>
              {#each targetOptions as opt (opt.value)}
                <option value={opt.value}>{opt.label}</option>
              {/each}
            </select>
          </label>
        {/if}
      </div>

      <div class="act-row">
        <button type="button" class="btn primary" aria-disabled={busy} onclick={() => void saveRuleDirectly()}>
          <Icon name="zap" />{busy ? tr('应用中…', 'Applying…') : tr('立即创建并生效', 'Create & Apply Now')}
        </button>
        <button type="button" class="btn" aria-disabled={busy} onclick={openInEditor}>
          <Icon name="edit" />{tr('去编辑器微调', 'Edit in Rule Editor')}
        </button>
      </div>
    </div>

    {#if caps.has(CAP_APP_GROUPS) && hasProcess}
      <div class="section">
        <span class="label">{tr('或一键加入 @group:id 应用组', 'Or Add Process to @group:id App Group')}</span>
        {#if rules.groups.length > 0}
          <div class="group-chips">
            {#each rules.groups as group (group.id)}
              {@const exists = group.apps.includes(chosenProcess())}
              <button
                type="button"
                class="btn sm"
                disabled={exists}
                aria-disabled={busy}
                title={`@group:${group.id}`}
                onclick={() => void addToGroup(group.id, group.name)}
              >
                <Icon name={exists ? 'check' : 'plus'} />{group.name}
                <span class="muted">({group.apps.length})</span>
              </button>
            {/each}
          </div>
        {:else}
          <p class="muted small">{tr('还没有应用分组，可在下方直接创建。', 'No app groups yet; create one below.')}</p>
        {/if}
        <div class="new-group">
          <input
            class="control"
            placeholder={tr('新建应用组名称（如：办公软件）', 'New app group name (e.g. Office)')}
            value={newGroupName}
            oninput={(e) => (newGroupName = e.currentTarget.value)}
          />
          <button type="button" class="btn sm" aria-disabled={busy} onclick={() => void createGroupWithApp()}>
            <Icon name="plus" />{tr('新建并加入', 'Create & Add')}
          </button>
        </div>
      </div>
    {/if}

    {#if error !== ''}
      <div class="inline-error" role="alert"><Icon name="alert" /><span class="selectable">{error}</span></div>
    {/if}
  </div>

  {#snippet footer()}
    <button type="button" class="btn" aria-disabled={busy} onclick={() => { if (!busy) onclose(); }}>{tr('关闭', 'Close')}</button>
  {/snippet}
</Dialog>

<style>
  .body {
    display: grid;
    gap: 14px;
  }

  .meta-card {
    display: grid;
    gap: 6px;
    padding: 10px 12px;
    border-radius: 8px;
    background: var(--muted-soft);
    font-size: 12.5px;
  }

  .meta-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .hit {
    display: inline-flex;
    align-items: center;
    gap: 6px;
  }

  .detail {
    font-size: 11px;
    word-break: break-all;
    padding-top: 4px;
    border-top: 1px solid var(--border);
  }

  .section {
    display: grid;
    gap: 8px;
  }

  .scope-tabs,
  .act-row,
  .group-chips {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    align-items: center;
  }

  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    align-items: flex-end;
  }

  .grow {
    flex: 1 1 180px;
  }

  .select {
    width: 100%;
    height: 32px;
  }

  .check {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    cursor: pointer;
  }

  .check input {
    accent-color: var(--accent);
  }

  .new-group {
    display: flex;
    gap: 6px;
  }

  .new-group .control {
    flex: 1;
    min-width: 0;
  }

  .small {
    margin: 0;
    font-size: 12px;
  }
</style>

<!--
  规则编辑器：新建（id 为空）或修改一条规则。
  提交前用 rules-validate 的规则逐项检查并指出是哪一项；后端仍是最终裁决，它的错误原样显示。
  由父组件用 {#if} 挂载：每次打开都是一份新的表单。
-->
<script lang="ts">
  import { untrack } from 'svelte';
  import { APP_GROUP_PREFIX, CAP_APP_GROUPS, CAP_RULE_SETS, DEFAULT_TARGET_ID } from '../../lib/api';
  import Dialog from '../../lib/components/Dialog.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import Segmented from '../../lib/components/Segmented.svelte';
  import Switch from '../../lib/components/Switch.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { ruleSetRefs } from '../../lib/rules-validate';
  import { actionOptions, addLine, hasProblems, networkOptions, ruleFromForm, validateRuleForm, type RuleForm, type RuleFormProblems } from '../../lib/rules-view';
  import { caps } from '../../lib/stores/caps.svelte';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import { rules } from '../../lib/stores/rules.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';

  interface Props {
    /** 打开时的内容；id 为空表示新建。 */
    initial: RuleForm;
    onclose: () => void;
  }

  let { initial, onclose }: Props = $props();

  const NO_PROBLEMS: RuleFormProblems = { name: '', apps: '', hosts: '', ports: '', target: '' };
  const uid = $props.id();

  let form = $state.raw<RuleForm>(untrack(() => initial));
  let problems = $state.raw<RuleFormProblems>(NO_PROBLEMS);
  let error = $state('');
  let saving = $state(false);

  const creating = $derived(form.id === '');
  const netOpts = $derived((void i18n.locale, networkOptions()));
  const actOpts = $derived((void i18n.locale, actionOptions()));
  const setRefs = $derived((void i18n.locale, ruleSetRefs()));
  const targetIds = $derived([...nodes.list.groups.map((group) => group.id), ...nodes.list.nodes.map((node) => node.id)]);

  /** 目标下拉框：默认代理分组、其余分组，以及这条规则当前指向的（可能是节点或已不存在的）目标。 */
  const targetOptions = $derived.by(() => {
    const groups = nodes.list.groups;
    const fallback = groups.find((group) => group.id === DEFAULT_TARGET_ID);
    const options = [
      { value: DEFAULT_TARGET_ID, label: fallback === undefined ? tr('默认代理', 'Default Proxy') : fallback.name },
      ...groups.filter((group) => group.id !== DEFAULT_TARGET_ID).map((group) => ({ value: group.id, label: group.name })),
    ];
    if (form.targetId === '' || options.some((option) => option.value === form.targetId)) return options;
    const node = nodes.list.nodes.find((item) => item.id === form.targetId);
    return [
      ...options,
      {
        value: form.targetId,
        label: node === undefined ? tr(`${form.targetId}（不存在）`, `${form.targetId} (missing)`) : tr(`节点：${node.name}`, `Node: ${node.name}`),
      },
    ];
  });

  function patch(change: Partial<RuleForm>): void {
    form = { ...form, ...change };
  }

  function close(): void {
    if (!saving) onclose();
  }

  /** 「插入应用分组」下拉框：选完即复位到提示项。 */
  function insertGroup(select: HTMLSelectElement): void {
    if (select.value !== '') patch({ appsText: addLine(form.appsText, APP_GROUP_PREFIX + select.value) });
    select.value = '';
  }

  async function submit(event?: SubmitEvent): Promise<void> {
    event?.preventDefault();
    if (saving) return;
    // 列表没读到时不拿空列表当依据：只有确实读到了才在界面这一侧判断「不存在」。
    const knownGroups = caps.has(CAP_APP_GROUPS) && rules.groupsError === '' ? rules.groups : null;
    problems = validateRuleForm(form, knownGroups, nodes.loaded ? targetIds : null);
    error = '';
    if (hasProblems({ ...problems })) return;
    const rule = ruleFromForm(form);
    saving = true;
    const result = await rules.save(rule);
    saving = false;
    if (!result.ok) {
      error = result.message;
      return;
    }
    toasts.ok(creating ? tr(`已新建规则「${rule.name}」`, `Created rule "${rule.name}"`) : tr(`已保存规则「${rule.name}」`, `Saved rule "${rule.name}"`));
    onclose();
  }
</script>

<Dialog open title={creating ? tr('新建规则', 'New Rule') : tr('编辑规则', 'Edit Rule')} busy={saving} width={560} backdropClose={false} onclose={close}>
  <form id={`${uid}-form`} class="form" novalidate onsubmit={(event) => void submit(event)}>
    <div class="row">
      <label class="field grow">
        <span class="label">{tr('名称', 'Name')}</span>
        <input class="control" data-autofocus name="rule-name" autocomplete="off" aria-invalid={problems.name !== ''} value={form.name} oninput={(event) => patch({ name: event.currentTarget.value })} />
        {#if problems.name !== ''}<span class="hint err" role="alert">{problems.name}</span>{/if}
      </label>
      <div class="field">
        <span class="label">{tr('启用', 'Enabled')}</span>
        <span class="switch-box"><Switch checked={form.enabled} label={tr('启用这条规则', 'Enable this rule')} onchange={(next) => patch({ enabled: next })} /></span>
      </div>
    </div>

    <label class="field">
      <span class="label">{tr('应用程序', 'Applications')}</span>
      <textarea class="control mono" name="rule-apps" rows="3" spellcheck="false" placeholder={tr('每行一个：进程名（chrome.exe）或完整路径；留空表示任意', 'One per line: process name (chrome.exe) or full path; blank for any')} aria-invalid={problems.apps !== ''} value={form.appsText} oninput={(event) => patch({ appsText: event.currentTarget.value })}></textarea>
      {#if problems.apps !== ''}<span class="hint err" role="alert">{problems.apps}</span>{/if}
    </label>
    {#if caps.has(CAP_APP_GROUPS) && rules.groups.length > 0}
      <div class="shortcuts">
        <select class="select" aria-label={tr('插入应用分组', 'Insert app group')} onchange={(event) => insertGroup(event.currentTarget)}>
          <option value="">{tr('插入应用分组…', 'Insert app group…')}</option>
          {#each rules.groups as group (group.id)}
            <option value={group.id}>{tr(`${group.name}（${group.apps.length} 个应用）`, `${group.name} (${group.apps.length} apps)`)}</option>
          {/each}
        </select>
      </div>
    {/if}

    <label class="field">
      <span class="label">{tr('目标主机', 'Target Hosts')}</span>
      <textarea class="control mono" name="rule-hosts" rows="3" spellcheck="false" placeholder={tr('域名（可用 * ?）、IP、CIDR 或 IP 范围，用换行或逗号分隔；留空表示任意', 'Domains (* ? wildcards), IPs, CIDRs, or IP ranges separated by newline or comma; blank for any')} aria-invalid={problems.hosts !== ''} value={form.hostsText} oninput={(event) => patch({ hostsText: event.currentTarget.value })}></textarea>
      {#if problems.hosts !== ''}<span class="hint err" role="alert">{problems.hosts}</span>{/if}
    </label>
    {#if caps.has(CAP_RULE_SETS)}
      <div class="shortcuts" role="group" aria-label={tr('添加规则集', 'Add rule set')}>
        <span class="muted">{tr('规则集', 'Rule Sets')}</span>
        {#each setRefs as item (item.ref)}
          <button type="button" class="btn sm" title={item.ref} onclick={() => patch({ hostsText: addLine(form.hostsText, item.ref) })}>
            <Icon name="plus" />{item.label}
          </button>
        {/each}
      </div>
    {/if}

    <div class="row">
      <label class="field grow">
        <span class="label">{tr('端口', 'Ports')}</span>
        <input class="control mono" name="rule-ports" autocomplete="off" spellcheck="false" placeholder={tr('例如 443, 8000-9000；留空表示任意', 'e.g. 443, 8000-9000; blank for any')} aria-invalid={problems.ports !== ''} value={form.portsText} oninput={(event) => patch({ portsText: event.currentTarget.value })} />
        {#if problems.ports !== ''}<span class="hint err" role="alert">{problems.ports}</span>{/if}
      </label>
      <div class="field">
        <span class="label">{tr('协议', 'Protocol')}</span>
        <Segmented options={netOpts} value={form.network} label={tr('协议', 'Protocol')} onchange={(next) => patch({ network: next })} />
      </div>
    </div>

    <div class="row">
      <div class="field">
        <span class="label">{tr('动作', 'Action')}</span>
        <Segmented options={actOpts} value={form.action} label={tr('动作', 'Action')} onchange={(next) => patch({ action: next })} />
      </div>
      {#if form.action === 'proxy'}
        <label class="field grow">
          <span class="label">{tr('代理目标', 'Proxy Target')}</span>
          <select class="select target" name="rule-target" aria-invalid={problems.target !== ''} value={form.targetId === '' ? DEFAULT_TARGET_ID : form.targetId} onchange={(event) => patch({ targetId: event.currentTarget.value })}>
            {#each targetOptions as option (option.value)}
              <option value={option.value}>{option.label}</option>
            {/each}
          </select>
          {#if problems.target !== ''}<span class="hint err" role="alert">{problems.target}</span>{/if}
        </label>
      {/if}
    </div>

    <div class="resolve">
      <Switch checked={form.resolveBefore} label={tr('匹配 IP 条件前先解析域名', 'Resolve domains before matching IP rules')} onchange={(next) => patch({ resolveBefore: next })} />
      <span>
        {tr('匹配 IP 条件前先解析域名', 'Resolve domains before matching IP rules')}
        <span class="muted note">{tr('目标是域名、规则里有 IP 类条件时，先解析再比对。', 'Resolve domain targets before evaluating IP/CIDR conditions.')}</span>
      </span>
    </div>

    {#if error !== ''}
      <div class="inline-error" role="alert"><Icon name="alert" /><span class="selectable">{error}</span></div>
    {/if}
  </form>

  {#snippet footer()}
    <span class="muted foot-note">{tr('保存后立即生效；内核运行中会短暂重启。', 'Applies immediately; running core will briefly restart.')}</span>
    <button type="button" class="btn" aria-disabled={saving} onclick={close}>{tr('取消', 'Cancel')}</button>
    <button type="submit" class="btn primary" form={`${uid}-form`} aria-disabled={saving}>{saving ? tr('保存中…', 'Saving…') : tr('保存', 'Save')}</button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: grid;
    gap: 12px;
  }

  .row {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    align-items: flex-start;
  }

  .grow {
    flex: 1 1 200px;
  }

  .switch-box {
    display: flex;
    align-items: center;
    height: 32px;
  }

  textarea.control {
    min-height: 64px;
    font-size: 12.5px;
  }

  .shortcuts {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    margin-top: -6px;
    font-size: 12px;
  }

  .select {
    max-width: 100%;
  }

  .select.target {
    width: 100%;
    height: 32px;
  }

  .resolve {
    display: flex;
    align-items: flex-start;
    gap: 10px;
  }

  .note {
    display: block;
    font-size: 12px;
  }

  .foot-note {
    margin-right: auto;
    font-size: 12px;
  }
</style>

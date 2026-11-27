<!--
  应用分组编辑框：新建（id 为空）或修改。由父组件用 {#if} 挂载，每次打开都是一份新的表单。
-->
<script lang="ts">
  import { untrack } from 'svelte';
  import type { AppGroup } from '../../lib/api';
  import Dialog from '../../lib/components/Dialog.svelte';
  import Icon from '../../lib/components/Icon.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import { parseLines, validateAppGroupForm, type AppGroupProblems } from '../../lib/rules-view';
  import { rules } from '../../lib/stores/rules.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';

  interface Props {
    group: AppGroup;
    onclose: () => void;
  }

  let { group, onclose }: Props = $props();

  const uid = $props.id();
  const id = untrack(() => group.id);

  let name = $state(untrack(() => group.name));
  let appsText = $state(untrack(() => group.apps.join('\n')));
  let problems = $state.raw<AppGroupProblems>({ name: '', apps: '' });
  let error = $state('');
  let saving = $state(false);

  function close(): void {
    if (!saving) onclose();
  }

  async function submit(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (saving) return;
    problems = validateAppGroupForm(name, appsText);
    error = '';
    if (problems.name !== '' || problems.apps !== '') return;
    const next: AppGroup = { id, name: name.trim(), apps: parseLines(appsText) };
    saving = true;
    const result = await rules.saveGroup(next);
    saving = false;
    if (!result.ok) {
      error = result.message;
      return;
    }
    toasts.ok(id === '' ? tr(`已新建应用分组「${next.name}」`, `Created app group "${next.name}"`) : tr(`已保存应用分组「${next.name}」`, `Saved app group "${next.name}"`));
    onclose();
  }
</script>

<Dialog open title={id === '' ? tr('新建应用分组', 'New App Group') : tr('编辑应用分组', 'Edit App Group')} busy={saving} width={480} backdropClose={false} onclose={close}>
  <form id={`${uid}-form`} class="form" novalidate onsubmit={(event) => void submit(event)}>
    <label class="field">
      <span class="label">{tr('名称', 'Name')}</span>
      <input class="control" data-autofocus name="group-name" autocomplete="off" aria-invalid={problems.name !== ''} bind:value={name} />
      {#if problems.name !== ''}<span class="hint err" role="alert">{problems.name}</span>{/if}
    </label>
    <label class="field">
      <span class="label">{tr('应用', 'Applications')}</span>
      <textarea class="control mono" name="group-apps" rows="6" spellcheck="false" placeholder={tr('每行一个：进程名（chrome.exe）或完整路径', 'One per line: process name (chrome.exe) or full path')} aria-invalid={problems.apps !== ''} bind:value={appsText}></textarea>
      {#if problems.apps !== ''}
        <span class="hint err" role="alert">{problems.apps}</span>
      {:else}
        <span class="hint">{tr('最多 256 个；不能引用其它分组。', 'Up to 256 entries; cannot reference other groups.')}</span>
      {/if}
    </label>
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

  textarea.control {
    min-height: 120px;
    font-size: 12.5px;
  }

  .foot-note {
    margin-right: auto;
    font-size: 12px;
  }
</style>

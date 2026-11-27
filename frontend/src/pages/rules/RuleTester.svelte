<!--
  规则测试：填一个连接的应用、主机、端口与协议，看它会命中哪条规则。不改动任何东西。
  后端给出 stage=dns-required 时表示「是否命中取决于域名解析结果」，这里如实说明，不当成命中显示。
-->
<script lang="ts">
  import { type Rule } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import Segmented from '../../lib/components/Segmented.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { actionLabel, describeTestResult, validateTestInput, type TestOutcome } from '../../lib/rules-view';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import { rules } from '../../lib/stores/rules.svelte';

  type TestNetwork = 'tcp' | 'udp';

  const NETWORKS: ReadonlyArray<{ value: TestNetwork; label: string }> = [
    { value: 'tcp', label: 'TCP' },
    { value: 'udp', label: 'UDP' },
  ];

  let app = $state('');
  let host = $state('');
  let port = $state('443');
  let network = $state<TestNetwork>('tcp');
  let running = $state(false);
  let error = $state('');
  let outcome = $state.raw<TestOutcome | null>(null);
  /** 得出结果时的规则列表：之后列表变了，结果就作废。 */
  let testedAgainst = $state.raw<readonly Rule[] | null>(null);

  /** 出站 tag、节点 id、分组 id → 名称。 */
  const outboundNames = $derived(
    new Map<string, string>([
      ...nodes.list.nodes.flatMap((node): Array<[string, string]> => [[node.tag, node.name], [node.id, node.name]]),
      ...nodes.list.groups.map((group): [string, string] => [group.id, group.name]),
    ]),
  );

  // 规则改过之后，旧的测试结果不再可信。
  $effect(() => {
    if (testedAgainst !== null && rules.list !== testedAgainst) {
      testedAgainst = null;
      outcome = null;
    }
  });

  function reset(): void {
    outcome = null;
    error = '';
    testedAgainst = null;
  }

  async function run(event: SubmitEvent): Promise<void> {
    event.preventDefault();
    if (running) return;
    const target = host.trim();
    const problem = validateTestInput(app.trim(), target, port.trim());
    reset();
    if (problem !== '') {
      error = problem;
      return;
    }
    running = true;
    const list = rules.list;
    const result = await rules.test(app.trim(), target, Number(port.trim()), network);
    running = false;
    if (!result.ok) {
      error = result.message;
      return;
    }
    outcome = describeTestResult(result.value, list, outboundNames);
    testedAgainst = list;
  }
</script>

<form class="card tester" aria-label={tr('规则测试', 'Rule tester')} novalidate onsubmit={(event) => void run(event)}>
  <span class="title"><Icon name="flask" /><b>{tr('规则测试', 'Rule Tester')}</b></span>
  <label class="input app"><input name="test-app" autocomplete="off" spellcheck="false" placeholder={tr('应用（可留空）', 'App (optional)')} aria-label={tr('应用程序', 'Application')} bind:value={app} oninput={reset} /></label>
  <label class="input host"><input name="test-host" autocomplete="off" spellcheck="false" placeholder={tr('域名或 IP', 'Domain or IP')} aria-label={tr('目标主机', 'Target host')} bind:value={host} oninput={reset} /></label>
  <label class="input port"><input name="test-port" autocomplete="off" inputmode="numeric" placeholder={tr('端口', 'Port')} aria-label={tr('端口', 'Port')} bind:value={port} oninput={reset} /></label>
  <Segmented
    options={NETWORKS}
    value={network}
    label={tr('协议', 'Protocol')}
    onchange={(next) => {
      network = next;
      reset();
    }}
  />
  <button type="submit" class="btn" aria-disabled={running}>{running ? tr('测试中…', 'Testing…') : tr('测试', 'Test')}</button>

  <div class="result" role="status">
    {#if error !== ''}
      <span class="err selectable"><Icon name="alert" />{error}</span>
    {:else if outcome !== null}
      {@const label = outcome.number > 0 ? `#${outcome.number} ${outcome.name}` : outcome.name}
      {#if outcome.kind === 'hit'}
        <span>{tr('命中 ', 'Matched ')}<b>{label}</b></span>
        <span aria-hidden="true">→</span>
        <span class={`pill ${outcome.action}`}>{(void i18n.locale, actionLabel(outcome.action))}</span>
        {#if outcome.outbound !== ''}
          <span aria-hidden="true">→</span>
          <span>{outcome.outbound}</span>
        {/if}
      {:else}
        <span class="pill warn">{tr('需要解析域名', 'DNS Required')}</span>
        <span>{tr('无法确定：是否命中 ', 'Uncertain: matching ')}<b>{label}</b>{tr(' 取决于域名的解析结果，实际连接时才能判断。', ' depends on DNS resolution at connection time.')}</span>
      {/if}
    {:else}
      <span class="muted">{tr('填写后点「测试」，查看这个连接会命中哪条规则。', 'Enter connection details and click Test to see which rule matches.')}</span>
    {/if}
  </div>
</form>

<style>
  .tester {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    margin-top: 12px;
  }

  .title {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--text-3);
  }

  .title b {
    font-weight: 600;
    color: var(--text);
  }

  .input {
    min-width: 0;
  }

  .app {
    flex: 1 1 130px;
    max-width: 180px;
  }

  .host {
    flex: 2 1 160px;
    max-width: 240px;
  }

  .port {
    flex: 0 0 72px;
  }

  .result {
    display: flex;
    flex: 1 1 100%;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    min-width: 0;
    font-size: 12.5px;
    color: var(--text-2);
  }

  .result b {
    font-weight: 600;
    color: var(--text);
  }

  .err {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    color: var(--err);
  }

  .err :global(svg) {
    width: 14px;
    height: 14px;
    flex: none;
  }
</style>

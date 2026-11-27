<script lang="ts">
  import Icon from '../../lib/components/Icon.svelte';
  import { isRecord } from '../../lib/guard';
  import { tr } from '../../lib/i18n/index.svelte';
  import { core } from '../../lib/stores/core.svelte';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import { settings } from '../../lib/stores/settings.svelte';
  import { reloadAll } from './reload-all';

  /** 与 internal/spec.MaxBytes 一致。 */
  const MAX_BYTES = 1 << 20;

  /** 仅用于说明格式：一个指向本机 SOCKS 端口的节点，取自后端自己的测试用例。 */
  const exampleSpec = $derived(
    JSON.stringify(
      {
        nodes: [{ id: 'local', tag: tr('本机测试', 'Local Test'), outbound: { type: 'socks', server: '127.0.0.1', server_port: 1080 } }],
        groups: [],
        rules: [],
        dns: {
          mode: 'redirhost',
          remote: 'https://1.1.1.1/dns-query',
          direct: 'https://223.5.5.5/dns-query',
          strategy: 'prefer_ipv4',
        },
        inbound: {
          mixedPort: 7890,
          allowLan: false,
          tun: { enabled: false, stack: 'mixed', strictRoute: true, excludes: [], mtu: 1500 },
        },
        mode: 'rule',
        log: { level: 'info' },
      },
      null,
      2,
    ),
  );

  let text = $state('');
  let error = $state('');
  let notice = $state('');
  let input = $state<HTMLTextAreaElement | null>(null);

  const locked = $derived(core.running || core.transitioning);
  const busy = $derived(core.pending === 'configure');
  /** 内核运行中（上方有提示说明）或状态未知时不能导入。内容为空不算：点击后会直接说明缺什么。 */
  const unavailable = $derived(locked || core.state === null);

  /** 只做语法层面的预检，字段是否合法以后端的校验为准。返回空串表示可以提交。 */
  function precheck(payload: string): string {
    if (payload === '') return tr('请先粘贴配置内容', 'Please paste RunSpec JSON first');
    if (new TextEncoder().encode(payload).length > MAX_BYTES) return tr('配置过大：不能超过 1 MiB', 'Config too large: maximum 1 MiB');
    try {
      const parsed: unknown = JSON.parse(payload);
      return isRecord(parsed) ? '' : tr('配置必须是一个 JSON 对象', 'Config must be a JSON object');
    } catch {
      return tr('内容不是合法的 JSON，请检查括号、引号与逗号', 'Invalid JSON syntax; check brackets, quotes, and commas');
    }
  }

  function describeResult(): string {
    const failures = [
      nodes.error === '' ? '' : tr(`刷新节点列表失败：${nodes.error}`, `Failed to refresh nodes: ${nodes.error}`),
      settings.error === '' ? '' : tr(`重新读取设置失败：${settings.error}`, `Failed to reload settings: ${settings.error}`),
    ].filter((item) => item !== '');
    if (failures.length > 0) return tr(`配置已导入，但${failures.join('；')}`, `Config imported, but ${failures.join('; ')}`);
    return tr(
      `配置已导入：${nodes.list.nodes.length} 个节点、${nodes.list.groups.length} 个分组。内核尚未启动，可在概览页启动。`,
      `Config imported: ${nodes.list.nodes.length} nodes, ${nodes.list.groups.length} groups. Start core from Overview.`,
    );
  }

  async function submit(): Promise<void> {
    // 导入进行中按钮只是暂不响应（不禁用，以免丢掉键盘焦点）。
    if (busy) return;
    error = '';
    notice = '';
    const payload = text.trim();
    const problem = precheck(payload);
    if (problem !== '') {
      error = problem;
      return;
    }

    const result = await core.configure(payload);
    if (!result.ok) {
      error = result.message;
      return;
    }

    // 配置里可能有节点凭据：导入成功后立即清空，不在界面里保留。
    text = '';
    // 整份配置都换了（订阅被清空、规则可能变化）：让所有 store 重新读取。
    await reloadAll();
    notice = describeResult();
  }

  function fillExample(): void {
    text = exampleSpec;
    error = '';
    notice = '';
  }

  function clear(): void {
    text = '';
    error = '';
    // 「清空」随即变为不可用：把焦点交给输入框，方便接着粘贴。
    input?.focus();
  }
</script>

<details class="dev">
  <summary>
    <span class="chev"><Icon name="chev" size={14} /></span>
    <span class="grow">
      <span class="t">{tr('开发配置导入', 'Developer RunSpec Import')} <span class="pill">{tr('开发用途', 'Dev Only')}</span></span>
      <span class="d">{tr('导入受限的 RunSpec（JSON）用于联调内核；会替换「订阅」页已导入的节点', 'Import restricted RunSpec JSON for debugging; replaces nodes imported via Subscriptions')}</span>
    </span>
  </summary>

  <div class="body">
    <ul class="rules">
      <li>{tr('节点的 ', 'Node ')}<code>outbound</code>{tr(' 使用 sing-box 出站格式，只接受后端允许的协议与字段。', ' follows sing-box outbound format with allowlisted protocols and fields only.')}</li>
      <li><code>rules</code>{tr(' 必须为空数组，', ' must be empty and ')}<code>inbound.tun.enabled</code>{tr(' 必须为 ', ' must be ')}<code>false</code>.</li>
      <li>{tr('分组类型只支持 ', 'Group types allow ')}<code>select</code>{tr(' 与 ', ' and ')}<code>urltest</code>{tr('；DNS 只支持使用 IP 地址的服务器。', '; DNS servers must use IP literals.')}</li>
      <li>{tr('字段名区分大小写，不能包含未知字段；大小上限 1 MiB。', 'Field names are case-sensitive with no unknown fields; max size 1 MiB.')}</li>
      <li>{tr('配置可能包含节点凭据：内容只交给本机后端，界面不保存，导入成功后输入框会被清空。', 'May contain credentials: sent only to local backend, not saved in UI, and cleared after import.')}</li>
    </ul>

    <label class="lbl" for="dev-spec">RunSpec (JSON)</label>
    <textarea
      id="dev-spec"
      class="mono"
      rows="10"
      spellcheck="false"
      autocomplete="off"
      autocapitalize="off"
      placeholder={'{ "nodes": [ … ], "groups": [], "rules": [], … }'}
      bind:this={input}
      bind:value={text}
    ></textarea>

    {#if locked}
      <div class="inline-note"><Icon name="info" /><span>{tr('内核运行中：请先停止内核，再导入配置。', 'Core is running: stop core before importing RunSpec.')}</span></div>
    {/if}
    {#if error !== ''}
      <div class="inline-error" role="alert"><Icon name="alert" /><span>{error}</span></div>
    {/if}
    {#if notice !== ''}
      <div class="inline-note" role="status"><Icon name="check" /><span>{notice}</span></div>
    {/if}

    <div class="actions">
      <button type="button" class="btn sm ghost" onclick={fillExample}>{tr('填入示例', 'Insert Example')}</button>
      <span class="hint muted">{tr('示例指向本机 127.0.0.1:1080 的 SOCKS 代理，仅用于说明格式', 'Example points to local SOCKS 127.0.0.1:1080 to illustrate schema')}</span>
      <button type="button" class="btn sm ghost" disabled={text === ''} onclick={clear}>{tr('清空', 'Clear')}</button>
      <button
        type="button"
        class="btn sm primary"
        disabled={unavailable}
        aria-disabled={busy}
        onclick={() => void submit()}
      >
        {busy ? tr('导入中…', 'Importing…') : tr('导入', 'Import')}
      </button>
    </div>
  </div>
</details>

<style>
  summary {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    cursor: pointer;
    list-style: none;
    border-radius: var(--r-lg);
  }

  summary::-webkit-details-marker {
    display: none;
  }

  .chev {
    display: grid;
    place-items: center;
    color: var(--text-3);
    transform: rotate(-90deg);
    transition: transform var(--dur) var(--ease);
  }

  .dev[open] .chev {
    transform: none;
  }

  .grow {
    flex: 1;
    min-width: 0;
    display: grid;
  }

  .t {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    font-weight: 500;
  }

  .d {
    font-size: 12px;
    color: var(--text-3);
  }

  .body {
    display: grid;
    gap: 10px;
    padding: 4px 16px 16px;
    border-top: 1px solid var(--border);
  }

  .rules {
    margin: 10px 0 0;
    padding-left: 18px;
    display: grid;
    gap: 3px;
    font-size: 12px;
    color: var(--text-2);
  }

  code {
    font-family: var(--font-mono);
    font-size: 11.5px;
    padding: 0 4px;
    border-radius: 4px;
    background: var(--surface-3);
  }

  .lbl {
    font-size: 12px;
    color: var(--text-2);
  }

  textarea {
    width: 100%;
    min-height: 140px;
    resize: vertical;
    padding: 10px 12px;
    border-radius: var(--r);
    border: 1px solid var(--border);
    background: var(--surface-2);
    color: var(--text);
    line-height: 1.5;
    tab-size: 2;
  }

  textarea:focus {
    border-color: var(--accent);
    box-shadow: 0 0 0 3px var(--accent-soft);
    outline: none;
  }

  .actions {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 8px;
  }

  .hint {
    flex: 1;
    min-width: 0;
    font-size: 12px;
  }
</style>

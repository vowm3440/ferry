<!--
  局域网分享：临时在本机开一个只读地址，局域网里的其它设备凭链接取走全部节点。
  只在用户点「开启」时开启；链接带一次性令牌，只放在这个面板的局部状态里：
  只展示、可复制、可生成二维码，面板关掉就没了。
  面板关掉之后界面上再没有地方显示「分享还开着」，所以关闭时一并停止分享。
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { CAP_QR, copyText, messageOfError } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import { tr } from '../../lib/i18n/index.svelte';
  import { caps } from '../../lib/stores/caps.svelte';
  import { lanShare } from '../../lib/stores/lan-share.svelte';
  import { toasts } from '../../lib/stores/toasts.svelte';
  import { formatCountdown } from '../../lib/time';
  import QrBox from './QrBox.svelte';

  let showQr = $state(false);
  /** 当前分享的链接：只在这里。 */
  let link = $state('');

  const url = $derived(lanShare.active ? link : '');

  /** 面板是否还挂在页面上。 */
  let alive = true;

  /** 面板已经关掉：不让分享在没有任何提示的情况下继续开着。 */
  async function stopAfterClose(): Promise<void> {
    const result = await lanShare.stop();
    if (result.ok) toasts.info(tr('分享对话框已关闭，局域网分享已停止', 'Share dialog closed; LAN share stopped'));
    else toasts.error(tr(`局域网分享没有停止：${result.message}`, `Failed to stop LAN share: ${result.message}`));
  }

  // 打开面板时读一次状态：上一次没能停掉的分享还在的话，把链接取回来。
  onMount(() => {
    void lanShare.refresh().then((current) => {
      if (alive && link === '') link = current;
    });
    return () => {
      alive = false;
      if (lanShare.active && !lanShare.busy) void stopAfterClose();
    };
  });

  // 分享结束（停止、到期或已被读取）后丢掉链接、收起二维码。
  $effect(() => {
    if (lanShare.active) return;
    showQr = false;
    link = '';
  });

  async function start(): Promise<void> {
    if (lanShare.busy) return;
    const result = await lanShare.start();
    if (!alive) {
      // 还没开好对话框就关了：开成功了也立刻停掉。
      if (result.ok) await stopAfterClose();
      return;
    }
    if (result.ok) link = result.url;
    else toasts.error(result.message);
  }

  async function stop(): Promise<void> {
    if (lanShare.busy) return;
    const result = await lanShare.stop();
    if (result.ok) toasts.info(tr('已停止局域网分享', 'LAN share stopped'));
    else toasts.error(result.message);
  }

  async function copy(): Promise<void> {
    if (url === '') return;
    try {
      await copyText(url);
      toasts.ok(tr('已复制局域网分享链接', 'Copied LAN share link'));
    } catch (error: unknown) {
      toasts.error(messageOfError(error));
    }
  }
</script>

<section class="lan" aria-label={tr('局域网分享', 'LAN Share')}>
  <div class="head">
    <span class="ic"><Icon name="wifi" size={16} /></span>
    <div class="grow">
      <b>{tr('局域网分享', 'LAN Share')}</b>
      {#if lanShare.active}
        <span class="muted">
          {tr('剩余 ', 'Expires in ')}<span class="num" id="lan-countdown">{formatCountdown(lanShare.remainingMs)}</span>{tr('；被读取一次或关闭此对话框后停止', '; stops after one fetch or when closed')}
        </span>
      {:else}
        <span class="muted">{tr('把全部节点临时分享给同一局域网里的设备', 'Temporarily share all nodes with devices on your LAN')}</span>
      {/if}
    </div>
    {#if lanShare.active}
      <button type="button" class="btn sm" aria-disabled={lanShare.busy} onclick={() => void stop()}>
        {lanShare.busy ? tr('停止中…', 'Stopping…') : tr('停止', 'Stop')}
      </button>
    {:else}
      <button type="button" class="btn sm" aria-disabled={lanShare.busy} onclick={() => void start()}>
        {lanShare.busy ? tr('开启中…', 'Starting…') : tr('开启', 'Start')}
      </button>
    {/if}
  </div>

  {#if lanShare.active}
    <div class="link">
      <code class="mono selectable" id="lan-url">{url}</code>
      <button type="button" class="btn sm" onclick={() => void copy()}><Icon name="copy" />{tr('复制', 'Copy')}</button>
      {#if caps.has(CAP_QR)}
        <button type="button" class="btn sm" aria-pressed={showQr} onclick={() => (showQr = !showQr)}>
          <Icon name="qr" />{showQr ? tr('收起二维码', 'Hide QR') : tr('二维码', 'QR Code')}
        </button>
      {/if}
    </div>
    {#if showQr}
      <div class="qr"><QrBox payload={url} label={tr('局域网分享链接的二维码', 'QR code for LAN share link')} /></div>
    {/if}
  {:else}
    <p class="warn">
      <Icon name="alert" size={14} />
      <span>{tr('开启后 10 分钟内，局域网里任何拿到链接的人都能读取一次全部节点（含凭据）。只在可信的网络里使用；关闭此对话框会同时停止分享。', 'For 10 minutes, anyone on your LAN with the link can fetch all nodes (including credentials) once. Use only on trusted networks; closing this dialog stops sharing.')}</span>
    </p>
  {/if}
  {#if lanShare.error !== ''}
    <p class="warn err" role="alert"><Icon name="alert" size={14} /><span>{lanShare.error}</span></p>
  {/if}
</section>

<style>
  .lan {
    display: grid;
    gap: 8px;
    padding: 10px 12px;
    border-radius: var(--r);
    border: 1px solid var(--border);
    background: var(--surface-2);
    font-size: 12.5px;
  }

  .head {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  .ic {
    flex: none;
    display: grid;
    color: var(--text-3);
  }

  .grow {
    flex: 1;
    min-width: 0;
    display: grid;
  }

  .grow b {
    font-weight: 500;
  }

  .grow .muted {
    font-size: 12px;
  }

  .link {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
  }

  .link code {
    flex: 1;
    min-width: 160px;
    padding: 6px 8px;
    border-radius: 6px;
    border: 1px solid var(--border);
    background: var(--surface);
    font-size: 12px;
    overflow-wrap: anywhere;
  }

  .qr {
    display: grid;
    place-items: center;
  }

  .warn {
    display: flex;
    align-items: flex-start;
    gap: 6px;
    margin: 0;
    font-size: 12px;
    color: var(--warn);
  }

  .warn :global(svg.i) {
    flex: none;
    margin-top: 2px;
  }

  .warn.err {
    color: var(--err);
  }
</style>

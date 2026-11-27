<!--
  二维码：把一段文字交给后端生成 SVG，再以 <img> 显示在白底上（不把 SVG 当 HTML 插入）。
  文字可能含凭据或一次性令牌：只留在本组件的局部状态里，随组件销毁。
-->
<script lang="ts">
  import { messageOfError } from '../../lib/api';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { connection } from '../../lib/stores/connection.svelte';
  import { qrPayloadProblem, svgDataUrl } from '../../lib/subs-view';

  interface Props {
    /** 要编码的内容；空串表示还没有内容。 */
    payload: string;
    /** 图片的替代文字。 */
    label: string;
  }

  let { payload, label }: Props = $props();

  const SIZE = 320;

  let src = $state('');
  let problem = $state('');
  let loading = $state(false);

  $effect(() => {
    void i18n.locale;
    const text = payload;
    const backend = connection.backend;
    src = '';
    problem = '';
    loading = false;
    if (text === '' || backend === null) return;
    const tooLarge = qrPayloadProblem(text);
    if (tooLarge !== '') {
      problem = tooLarge;
      return;
    }

    let stale = false;
    loading = true;
    backend
      .subsQrSvg(text, SIZE)
      .then((svg) => {
        if (stale) return;
        const url = svgDataUrl(svg);
        if (url === null) problem = tr('后端返回的二维码无法显示', 'Invalid QR code SVG returned by backend');
        else src = url;
      })
      .catch((error: unknown) => {
        if (!stale) problem = messageOfError(error);
      })
      .finally(() => {
        if (!stale) loading = false;
      });
    return () => {
      stale = true;
    };
  });
</script>

<div class="qr" class:empty={src === ''}>
  {#if src !== ''}
    <img {src} alt={label} width="148" height="148" draggable="false" />
  {:else if problem !== ''}
    <span class="msg" role="status">{problem}</span>
  {:else}
    <span class="msg">{loading ? tr('正在生成二维码…', 'Generating QR code…') : '—'}</span>
  {/if}
</div>

<style>
  .qr {
    flex: none;
    display: grid;
    place-items: center;
    width: 172px;
    height: 172px;
    padding: 12px;
    border-radius: 10px;
    /* 二维码必须是白底黑码，两种主题下都一样。 */
    background: #fff;
  }

  .qr.empty {
    border: 1px dashed var(--border-strong);
    background: transparent;
  }

  img {
    display: block;
    width: 148px;
    height: 148px;
  }

  .msg {
    font-size: 12px;
    color: var(--text-3);
    text-align: center;
  }
</style>

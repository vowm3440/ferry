<script lang="ts">
  import { tick } from 'svelte';
  import { CAP_SHARE } from '../../lib/api';
  import Icon from '../../lib/components/Icon.svelte';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { latencyView, type MemberView } from '../../lib/nodes-view';
  import { caps } from '../../lib/stores/caps.svelte';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import { shareRequest } from '../../lib/stores/share-request.svelte';

  interface Props {
    member: MemberView;
    groupId: string;
    /** 所在分组支持手动选择。 */
    manual: boolean;
    /** 当前可以切换（内核运行中且没有其他操作在进行）。 */
    selectable: boolean;
    /** 切换结束后兑现（无论成功与否），用来在按钮消失时安置键盘焦点。 */
    onSelect: (member: MemberView) => Promise<void>;
    onFavorite: (member: MemberView, next: boolean) => void;
  }

  let { member, groupId, manual, selectable, onSelect, onFavorite }: Props = $props();

  const isNode = $derived(member.kind === 'node');
  const busy = $derived(nodes.isSwitching(groupId, member.id));
  /** 任意一次切换进行中时所有「使用」按钮都暂不响应；用 aria-disabled 才不会丢掉键盘焦点。 */
  const waiting = $derived(nodes.switching !== '');
  const favoriteLabel = $derived(
    member.favorite ? tr(`取消收藏 ${member.name}`, `Unfavorite ${member.name}`) : tr(`收藏 ${member.name}`, `Favorite ${member.name}`),
  );
  /** 右侧一栏是否有内容；没有内容时不渲染，窄屏下才不会多出一行空白。 */
  const hasTail = $derived(isNode || member.kind === 'unknown' || manual);
  const latency = $derived((void i18n.locale, latencyView(member.latencyMs, member.latencyStatus)));

  let favoriteButton = $state<HTMLButtonElement | null>(null);

  /**
   * 切换成功后本行的「使用」按钮会变成「当前」，键盘焦点所在的按钮随之消失；
   * 这时候把焦点交还给同一行的收藏按钮，键盘用户不必从页面开头重新找回位置。
   */
  async function use(): Promise<void> {
    if (waiting) return;
    await onSelect(member);
    await tick();
    if (document.activeElement === document.body) favoriteButton?.focus();
  }
</script>

<li class="row" class:selected={member.selected}>
  <span class="cc">{member.badge}</span>

  <div class="meta">
    <div class="name">
      <span class="selectable">{member.name}</span>
      {#if member.selected}
        <span class="pill proxy"><Icon name="check" size={11} />{tr('当前', 'Active')}</span>
      {/if}
    </div>
    <div class="desc">
      {member.protocol}{#if member.address !== ''}<span class="sep">·</span><span class="mono selectable"
          >{member.address}</span
        >{/if}
    </div>
  </div>

  {#if hasTail}
    <div class="tail">
      {#if isNode}
        <span class="lat {latency.level}" class:busy={latency.busy}>{latency.text}</span>
        {#if caps.has(CAP_SHARE)}
          <button
            type="button"
            class="icon-btn mini"
            aria-label={tr('分享节点 ' + member.name, 'Share node ' + member.name)}
            title={tr('分享', 'Share')}
            onclick={() => shareRequest.open({ kind: 'node', id: member.id, name: member.name })}
          >
            <Icon name="share" />
          </button>
        {/if}
        <button
          type="button"
          class="icon-btn mini star"
          class:on={member.favorite}
          bind:this={favoriteButton}
          aria-pressed={member.favorite}
          aria-label={favoriteLabel}
          title={member.favorite ? tr('取消收藏', 'Unfavorite') : tr('收藏', 'Favorite')}
          onclick={() => onFavorite(member, !member.favorite)}
        >
          <Icon name="star" filled={member.favorite} />
        </button>
      {/if}

      {#if manual && member.kind === 'group'}
        <span class="pill" title={tr('这是一个被引用为成员的分组，不能在这里手动指定', 'Referenced group member; select inside its own group')}>{tr('嵌套分组', 'Subgroup')}</span>
      {:else if member.kind === 'unknown'}
        <span class="pill" title={tr('分组里引用了不存在的成员 id', 'Group references a missing member ID')}>{tr('未识别', 'Missing')}</span>
      {/if}

      {#if manual && isNode && !member.selected}
        <button
          type="button"
          class="btn sm use"
          disabled={!selectable}
          aria-disabled={waiting}
          aria-label={tr(`使用节点 ${member.name}`, `Use node ${member.name}`)}
          onclick={() => void use()}
        >
          {busy ? tr('切换中…', 'Switching…') : tr('使用', 'Use')}
        </button>
      {/if}
    </div>
  {/if}
</li>

<style>
  .row {
    /* 协议徽标等宽，各行的名称才能对齐成一列。最长的徽标是 6 个字符。 */
    --badge-w: 52px;
    --row-gap: 10px;
    display: flex;
    align-items: center;
    gap: var(--row-gap);
    padding: 9px 12px;
    border-radius: var(--r);
    border: 1px solid transparent;
  }

  .row:hover {
    background: var(--surface-2);
  }

  .row.selected {
    background: var(--accent-soft);
    border-color: var(--accent-ring);
  }

  .cc {
    min-width: var(--badge-w);
  }

  .meta {
    flex: 1;
    min-width: 0;
  }

  .tail {
    flex: none;
    display: flex;
    align-items: center;
    gap: var(--row-gap);
  }

  .name {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 6px;
    font-weight: 600;
    overflow-wrap: anywhere;
  }

  .desc {
    font-size: 12px;
    color: var(--text-3);
    overflow-wrap: anywhere;
  }

  .sep {
    margin: 0 5px;
  }

  .lat {
    flex: none;
    min-width: 52px;
    text-align: right;
    white-space: nowrap;
  }

  .lat.busy {
    color: var(--text-3);
  }

  .mini {
    width: 26px;
    height: 26px;
    flex: none;
    color: var(--text-3);
  }

  .star.on {
    color: var(--warn);
  }

  .mini :global(svg.i) {
    width: 15px;
    height: 15px;
  }

  .use {
    flex: none;
    min-width: 58px;
  }

  /* 窄屏：第一行是徽标与名称，第二行是延迟、收藏与操作，并与名称左对齐。 */
  @container content (max-width: 560px) {
    .row {
      flex-wrap: wrap;
      row-gap: 4px;
    }

    .tail {
      flex-basis: 100%;
      padding-left: calc(var(--badge-w) + var(--row-gap));
    }

    .lat {
      min-width: 0;
      text-align: left;
    }

    .use {
      margin-left: auto;
    }
  }
</style>

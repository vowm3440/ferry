<!--
  活动连接表格。数据与速率由上层给出，这里只负责展示和行内操作。
  行内操作一直可见（不靠悬停），窄窗口横向滚动时贴在右侧，键盘可以直接到达。
-->
<script lang="ts">
  import Icon from '../../lib/components/Icon.svelte';
  import type { Connection } from '../../lib/api';
  import { i18n, tr } from '../../lib/i18n/index.svelte';
  import { nodes } from '../../lib/stores/nodes.svelte';
  import {
    actionKind,
    actionLabel,
    appHue,
    appInitial,
    appName,
    durationText,
    outboundText,
    rateText,
    ruleText,
    targetText,
    type RateMap,
  } from '../../lib/conns-view';

  interface Props {
    rows: readonly Connection[];
    rates: RateMap;
    /** 最近一次快照的时刻，用来算时长。 */
    now: number;
    /** 正在关闭的连接 id。 */
    closing: ReadonlySet<string>;
    /** 正在为它预填规则的连接 id；没有时为空串。 */
    drafting: string;
    /** 后端是否提供规则功能；没有时不显示「据此创建规则」。 */
    canCreateRule: boolean;
    onclose: (conn: Connection) => void;
    oncreaterule: (conn: Connection) => void;
  }

  let { rows, rates, now, closing, drafting, canCreateRule, onclose, oncreaterule }: Props = $props();

  /** 出站 tag、节点 id、分组 id → 名称：内核报的是内部 tag，界面上显示用户认得的名字。 */
  const outboundNames = $derived(
    new Map<string, string>([
      ...nodes.list.nodes.flatMap((node): Array<[string, string]> => [[node.tag, node.name], [node.id, node.name]]),
      ...nodes.list.groups.map((group): [string, string] => [group.id, group.name]),
    ]),
  );
</script>

<div class="tbl-wrap conn-wrap">
  <table class="tbl" aria-label={tr('活动连接', 'Active connections')}>
    <thead>
      <tr>
        <th scope="col">{tr('应用', 'App')}</th>
        <th scope="col">{tr('目标', 'Target')}</th>
        <th scope="col">{tr('规则', 'Rule')}</th>
        <th scope="col">{tr('出站', 'Outbound')}</th>
        <th scope="col" class="r" title={tr('下载速率', 'Download speed')} aria-label={tr('下载速率', 'Download speed')}>↓</th>
        <th scope="col" class="r" title={tr('上传速率', 'Upload speed')} aria-label={tr('上传速率', 'Upload speed')}>↑</th>
        <th scope="col">{tr('时长', 'Duration')}</th>
        <th scope="col" class="act" aria-label={tr('操作', 'Actions')}></th>
      </tr>
    </thead>
    <tbody>
      {#each rows as conn (conn.id)}
        {@const name = (void i18n.locale, appName(conn))}
        {@const target = targetText(conn)}
        {@const kind = actionKind(conn.action)}
        {@const outbound = outboundText(conn, outboundNames)}
        {@const rate = rates.get(conn.id)}
        {@const busy = closing.has(conn.id)}
        <tr>
          <td>
            <div class="app" title={conn.processPath || name}>
              <span class="avatar" style:--hue={appHue(conn)} aria-hidden="true">{appInitial(conn)}</span>
              <span class="clip name">{name}</span>
            </div>
          </td>
          <td><span class="clip host selectable" title={target}>{target}</span></td>
          <td><span class="clip rule" title={conn.ruleName}>{ruleText(conn)}</span></td>
          <td>
            <div class="chain">
              <span class="pill" class:proxy={kind === 'proxy'} class:direct={kind === 'direct'} class:block={kind === 'block'}>
                {(void i18n.locale, actionLabel(conn.action))}
              </span>
              {#if outbound !== ''}
                <Icon name="arrow" size={12} />
                <span class="clip node" title={outbound}>{outbound}</span>
              {/if}
            </div>
          </td>
          <td class="r num">{rateText(rate?.down)}</td>
          <td class="r num">{rateText(rate?.up)}</td>
          <td class="num muted">{durationText(conn.startAt, now)}</td>
          <td class="act">
            <div class="ops">
              {#if canCreateRule}
                <button
                  type="button"
                  class="icon-btn"
                  aria-disabled={drafting !== ''}
                  aria-label={tr(`据此创建规则：${name} → ${target}`, `Create rule from connection: ${name} -> ${target}`)}
                  title={tr('据此创建规则', 'Create Rule')}
                  onclick={() => oncreaterule(conn)}
                >
                  <Icon name="rules" />
                </button>
              {/if}
              <button
                type="button"
                class="icon-btn"
                aria-disabled={busy}
                aria-label={tr(`关闭连接：${name} → ${target}`, `Close connection: ${name} -> ${target}`)}
                title={tr('关闭连接', 'Close Connection')}
                onclick={() => onclose(conn)}
              >
                <Icon name="close" />
              </button>
            </div>
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  /* 窄窗口横向滚动时，系统默认的滚动条在深色主题下是一条白带，改成跟随主题的细滚动条。 */
  .conn-wrap {
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
  }

  .app {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .avatar {
    flex: none;
    width: 22px;
    height: 22px;
    display: grid;
    place-items: center;
    border-radius: 6px;
    background: hsl(var(--hue) 52% 46%);
    color: #fff;
    font-size: 11px;
    font-weight: 700;
  }

  .clip {
    display: block;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .name {
    max-width: 150px;
    font-weight: 500;
  }

  .host {
    max-width: 230px;
    font-family: var(--font-mono);
    font-size: 12px;
  }

  .rule {
    max-width: 150px;
  }

  .chain {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
    color: var(--text-2);
    white-space: nowrap;
  }

  .node {
    max-width: 170px;
  }

  th.act,
  td.act {
    position: sticky;
    right: 0;
    width: 1%;
    padding: 0 4px;
    background-color: var(--surface);
  }

  /* 贴边的单元格要用不透明底色盖住滚到它下面的内容，悬停色叠在底色之上。 */
  .tbl tbody tr:hover td.act {
    background-color: var(--surface);
    background-image: linear-gradient(var(--muted-soft), var(--muted-soft));
  }

  .ops {
    display: flex;
    justify-content: flex-end;
    gap: 2px;
  }

  .ops .icon-btn {
    width: 26px;
    height: 26px;
  }
</style>

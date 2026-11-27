/*
 * 节点与分组。数据只来自 Nodes.List() 与 nodes:latency 事件；延迟一律按后端给的
 * latencyMs（-1 = 没有结果）展示，不在前端编造。
 */

import { EVENT_LATENCY, messageOfError, parseLatencyMap, type Backend, type Group, type NodeList, type Unsubscribe } from '../api';
import { tr } from '../i18n/index.svelte';
import { mergeLatency } from '../nodes-view';
import { SUCCESS, failure, failureMessage, type ActionResult } from '../result';

const EMPTY_LIST: NodeList = { groups: [], nodes: [] };
/** Core.LatencyTest 一次最多接受的节点数。 */
const LATENCY_MAX_IDS = 1024;

function switchKey(groupId: string, nodeId: string): string {
  return `${groupId}\u0000${nodeId}`;
}

class NodesStore {
  list = $state.raw<NodeList>(EMPTY_LIST);
  loaded = $state(false);
  loading = $state(false);
  error = $state('');
  /** 正在切换的「分组 + 节点」，用于按钮的进行中状态。 */
  switching = $state('');
  /** 有测速请求尚未返回。 */
  testing = $state(false);
  savingGroup = $state(false);

  #backend: Backend | null = null;
  #offs: Unsubscribe[] = [];

  attach(backend: Backend): Promise<void> {
    this.detach();
    this.#backend = backend;
    this.#offs = [
      backend.on(EVENT_LATENCY, (payload) => {
        this.list = mergeLatency(this.list, parseLatencyMap(payload));
      }),
    ];
    return this.refresh();
  }

  detach(): void {
    for (const off of this.#offs) off();
    this.#offs = [];
    this.#backend = null;
    this.testing = false;
    this.list = EMPTY_LIST;
    this.loaded = false;
    this.loading = false;
    this.error = '';
    this.switching = '';
  }

  async refresh(): Promise<void> {
    const backend = this.#backend;
    if (backend === null) return;
    this.loading = true;
    try {
      const list = await backend.nodesList();
      if (this.#backend !== backend) return;
      this.list = list;
      this.error = '';
      this.loaded = true;
    } catch (error: unknown) {
      if (this.#backend === backend) this.error = messageOfError(error);
    } finally {
      if (this.#backend === backend) this.loading = false;
    }
  }

  isSwitching(groupId: string, nodeId: string): boolean {
    return this.switching === switchKey(groupId, nodeId);
  }

  /** 切换分组的选中节点。内核未运行时后端会拒绝，文案原样返回。 */
  async select(groupId: string, nodeId: string): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(tr('未连接到桌面后端', 'Not connected to desktop backend'));
    if (this.switching !== '') return failureMessage(tr('上一个切换还在进行中，请稍候', 'Node switch in progress; please wait'));

    this.switching = switchKey(groupId, nodeId);
    try {
      await backend.coreSelectNode(groupId, nodeId);
      await this.refresh();
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.switching = '';
    }
  }

  /**
   * 测速。进度由 nodes:latency 事件逐批写进列表；结束后重读一次，以后端保存的结果为准。
   * 内核未运行等情况由后端拒绝，文案原样返回。
   */
  async testLatency(ids: readonly string[]): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(tr('未连接到桌面后端', 'Not connected to desktop backend'));
    if (this.testing) return failureMessage(tr('上一次测速还在进行中，请稍候', 'Latency test in progress; please wait'));
    if (ids.length === 0) return failureMessage(tr('请选择需要测速的节点', 'Select at least one node to test'));

    this.testing = true;
    try {
      for (let start = 0; start < ids.length; start += LATENCY_MAX_IDS) {
        await backend.coreLatencyTest(ids.slice(start, start + LATENCY_MAX_IDS));
        if (this.#backend !== backend) return SUCCESS;
      }
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      if (this.#backend === backend) {
        this.testing = false;
        await this.refresh();
      }
    }
  }

  /** 收藏：先乐观更新，失败时以后端为准重新读取。 */
  async setFavorite(id: string, on: boolean): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(tr('未连接到桌面后端', 'Not connected to desktop backend'));

    this.list = {
      groups: this.list.groups,
      nodes: this.list.nodes.map((node) => (node.id === id ? { ...node, favorite: on } : node)),
    };
    try {
      await backend.nodesSetFavorite(id, on);
      return SUCCESS;
    } catch (error: unknown) {
      await this.refresh();
      return failure(error);
    }
  }

  async saveGroup(group: Group): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(tr('未连接到桌面后端', 'Not connected to desktop backend'));
    if (this.savingGroup) return failureMessage(tr('上一个分组操作还在进行中，请稍候', 'Group operation in progress; please wait'));
    this.savingGroup = true;
    try {
      await backend.nodesSaveGroup(group);
      await this.refresh();
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      if (this.#backend === backend) this.savingGroup = false;
    }
  }

  async removeGroup(id: string): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(tr('未连接到桌面后端', 'Not connected to desktop backend'));
    if (this.savingGroup) return failureMessage(tr('上一个分组操作还在进行中，请稍候', 'Group operation in progress; please wait'));
    this.savingGroup = true;
    try {
      await backend.nodesRemoveGroup(id);
      await this.refresh();
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      if (this.#backend === backend) this.savingGroup = false;
    }
  }
}

export const nodes = new NodesStore();

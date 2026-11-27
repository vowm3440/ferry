/* 演示后端：内核、节点与设置。行为与报错文案贴近真实后端（internal/app）。 */

import { asRecord, readArray, readNumber, readString } from '../guard';
import type { CoreApi, NodesApi, SettingsApi } from './backend';
import { delay, makeDemoNode, rejected, DEMO_GROUP_ID, type DemoWorld } from './demo-world';
import { EVENT_LATENCY, type Group, type Node } from './types';
import { LATENCY_OK, LATENCY_TESTING, LATENCY_TIMEOUT, type LatencyMap, type LatencyUpdate } from './types-inspect';

const MAX_SPEC_BYTES = 1024 * 1024;
const INVALID_SPEC = '配置无效，请检查节点、端口与支持的字段';
const LATENCY_BATCH = 32;
const LATENCY_MAX_IDS = 1024;
const LATENCY_DELAY_MS = 600;
/** 合成延迟的范围，以及大约每八个节点出现一次的「超时」。 */
const LATENCY_MIN_MS = 40;
const LATENCY_SPAN_MS = 380;
const TIMEOUT_ODDS = 0.125;

function nodesFromSpec(specJson: string): Node[] {
  let parsed: unknown;
  try {
    parsed = JSON.parse(specJson);
  } catch {
    throw rejected(INVALID_SPEC);
  }
  const nodes = readArray(asRecord(parsed), 'nodes')
    .map((item) => {
      const source = asRecord(item);
      const outbound = asRecord(source.outbound);
      const id = readString(source, 'id');
      return makeDemoNode(
        id,
        readString(source, 'tag', id),
        readString(outbound, 'type'),
        readString(outbound, 'server'),
        readNumber(outbound, 'server_port'),
      );
    })
    .filter((node) => node.id !== '' && node.type !== '');
  if (nodes.length === 0) throw rejected(INVALID_SPEC);
  return nodes;
}

function syntheticLatency(): LatencyUpdate {
  if (Math.random() < TIMEOUT_ODDS) return { latencyMs: -1, status: LATENCY_TIMEOUT };
  return { latencyMs: LATENCY_MIN_MS + Math.round(Math.random() * LATENCY_SPAN_MS), status: LATENCY_OK };
}

/** 写回节点并发出 nodes:latency，与真实后端「先标记测速中，再给结果」的两段式一致。 */
function applyLatency(world: DemoWorld, updates: LatencyMap): void {
  world.mapNodes((node) => {
    const update = updates[node.id];
    return update === undefined ? node : { ...node, latencyMs: update.latencyMs, latencyStatus: update.status };
  });
  world.emit(EVENT_LATENCY, updates);
}

async function testLatency(world: DemoWorld, ids: readonly string[]): Promise<Record<string, number>> {
  if (ids.length === 0 || ids.length > LATENCY_MAX_IDS) throw rejected('请选择需要测速的节点');
  const known = new Set(world.nodes.map((node) => node.id));
  if (ids.some((id) => !known.has(id))) throw rejected('测速节点不存在');
  if (!world.running) throw rejected('请先启动内核再测速');

  let result: Record<string, number> = {};
  for (let start = 0; start < ids.length; start += LATENCY_BATCH) {
    const batch = ids.slice(start, start + LATENCY_BATCH);
    applyLatency(
      world,
      Object.fromEntries(batch.map((id) => [id, { latencyMs: -1, status: LATENCY_TESTING }])),
    );
    await delay(LATENCY_DELAY_MS);
    const done: LatencyMap = Object.fromEntries(batch.map((id) => [id, syntheticLatency()]));
    applyLatency(world, done);
    result = { ...result, ...Object.fromEntries(batch.map((id) => [id, done[id]?.latencyMs ?? -1])) };
  }
  return result;
}

export function createDemoCore(world: DemoWorld): CoreApi & NodesApi & SettingsApi {
  return {
    async coreStatus() {
      return world.coreState();
    },

    async coreStart() {
      await world.start();
    },

    async coreStop() {
      await world.stop();
    },

    async coreRestart() {
      await world.stop();
      await world.start();
    },

    async coreSetMode(mode) {
      world.setMode(mode);
    },

    async coreSelectNode(groupId, nodeId) {
      if (!world.running) throw rejected('请先启动内核再切换节点');
      const known = groupId === DEMO_GROUP_ID && world.nodes.some((node) => node.id === nodeId);
      if (!known) throw rejected('节点不存在');
      world.select(nodeId);
    },

    async coreSnapshot() {
      return { ...world.traffic };
    },

    async coreConfigure(specJson) {
      if (new Blob([specJson]).size > MAX_SPEC_BYTES) throw rejected('配置过大');
      if (world.running) throw rejected('请先停止内核，再导入配置');
      world.replaceConfig(nodesFromSpec(specJson));
    },

    coreLatencyTest(ids) {
      return testLatency(world, ids);
    },

    async nodesList() {
      return world.nodeList();
    },

    async nodesSetFavorite(id, on) {
      if (!world.nodes.some((node) => node.id === id)) throw rejected('节点不存在');
      world.mapNodes((node) => (node.id === id ? { ...node, favorite: on } : node));
    },

    async nodesSaveGroup(group: Group) {
      const name = group.name.trim();
      if (name === '') throw rejected('分组名称无效');
      if (group.type !== 'select' && group.type !== 'urltest') throw rejected('分组类型仅支持手动选择或自动测速');
      if (group.members.length === 0) throw rejected('请至少选择一个节点或子分组');
      const id = group.id.trim() !== '' ? group.id.trim() : `demo-grp-${Date.now()}`;
      const saved: Group = {
        id,
        name,
        type: group.type,
        members: [...group.members],
        selected: group.members[0] ?? '',
        testUrl: group.type === 'urltest' ? group.testUrl.trim() : '',
        intervalMin: group.type === 'urltest' ? (group.intervalMin > 0 ? group.intervalMin : 5) : 0,
      };
      const existing = world.groups;
      const next = existing.some((g) => g.id === id) ? existing.map((g) => (g.id === id ? saved : g)) : [...existing, saved];
      world.setGroups(next);
      return saved;
    },

    async nodesRemoveGroup(id) {
      const existing = world.groups;
      if (!existing.some((g) => g.id === id)) {
        throw rejected(id === DEMO_GROUP_ID ? '默认代理分组不可删除' : '分组不存在');
      }
      if (existing.some((g) => g.id !== id && g.members.includes(id))) {
        throw rejected('其它分组或规则仍引用此分组，请先移除对应引用');
      }
      world.setGroups(existing.filter((g) => g.id !== id));
    },

    async settingsGet() {
      return world.settings;
    },

    async settingsSet(value) {
      if (!Number.isInteger(value.mixedPort) || value.mixedPort < 1 || value.mixedPort > 65535) {
        throw rejected('代理端口必须在 1 到 65535 之间');
      }
      const automation =
        value.autoStart || value.startMinimized || value.lightweight || value.sysProxyOnStart || value.ruleSetAutoUpdate;
      if (automation) throw rejected('这些自动化功能尚未开放');
      if (world.running) throw rejected('请先停止内核，再修改设置');
      world.setSettings(value);
    },
  };
}

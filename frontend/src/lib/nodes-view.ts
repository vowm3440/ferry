/*
 * 节点页的视图模型：把 Nodes.List() 的 {groups, nodes} 整理成按分组渲染的结构。
 * 全部是纯函数，不修改传入的数据。
 *
 * 约定（见 internal/app/nodes.go）：
 *   - group.members 是成员 id 的权威列表，成员可以是节点，也可以是另一个分组；
 *   - group.selected 在内核未启动时默认为第一个成员；
 *   - urltest 分组由内核自动选择，后端给的 selected 只是占位，不能当作真实选中项展示。
 */

import type { Group, LatencyMap, Node, NodeList } from './api';
import { LATENCY_IDLE, LATENCY_TESTING, LATENCY_TIMEOUT } from './api';
import { formatLatency, latencyLevel } from './format';
import { getLocale, t } from './i18n/locale';

export type NodeSort = 'default' | 'name' | 'latency';

export interface NodeFilter {
  readonly query: string;
  readonly favoritesOnly: boolean;
  readonly sort: NodeSort;
}

export type MemberKind = 'node' | 'group' | 'unknown';

export interface MemberView {
  readonly id: string;
  readonly name: string;
  readonly kind: MemberKind;
  /** 协议全称，例如 VLESS；分组成员为「分组」。 */
  readonly protocol: string;
  /** 徽标短名，例如 SS。 */
  readonly badge: string;
  /** server:port，仅节点有。 */
  readonly address: string;
  readonly latencyMs: number;
  /** idle / testing / ok / timeout；分组与未知成员恒为 idle。 */
  readonly latencyStatus: string;
  readonly favorite: boolean;
  readonly selected: boolean;
}

export type GroupKind = 'select' | 'urltest' | 'other' | 'orphan';

export interface GroupView {
  readonly id: string;
  readonly name: string;
  /** select 手动选择；urltest 自动选择；orphan 不属于任何分组的节点。 */
  readonly kind: GroupKind;
  /** 当前选中成员的名称，仅手动选择分组有值。 */
  readonly selectedName: string;
  /** 过滤前的成员总数。 */
  readonly total: number;
  readonly members: readonly MemberView[];
}

export const ORPHAN_GROUP_ID = '';

const PROTOCOL_LABELS: ReadonlyMap<string, string> = new Map([
  ['shadowsocks', 'Shadowsocks'],
  ['vless', 'VLESS'],
  ['vmess', 'VMess'],
  ['trojan', 'Trojan'],
  ['socks', 'SOCKS'],
  ['http', 'HTTP'],
  ['hysteria2', 'Hysteria2'],
  ['tuic', 'TUIC'],
  ['wireguard', 'WireGuard'],
]);

const PROTOCOL_BADGES: ReadonlyMap<string, string> = new Map([
  ['shadowsocks', 'SS'],
  ['vless', 'VLESS'],
  ['vmess', 'VMESS'],
  ['trojan', 'TROJAN'],
  ['socks', 'SOCKS'],
  ['http', 'HTTP'],
  ['hysteria2', 'HY2'],
  ['tuic', 'TUIC'],
  ['wireguard', 'WG'],
]);

const BADGE_MAX_LENGTH = 6;

export function protocolLabel(type: string): string {
  if (type === '') return t('未知协议', 'Unknown');
  return PROTOCOL_LABELS.get(type) ?? type;
}

export function protocolBadge(type: string): string {
  if (type === '') return '?';
  return PROTOCOL_BADGES.get(type) ?? type.slice(0, BADGE_MAX_LENGTH).toUpperCase();
}

export function nodeDisplayName(node: Node): string {
  if (node.name !== '') return node.name;
  return node.tag !== '' ? node.tag : node.id;
}

function formatAddress(server: string, port: number): string {
  if (server === '') return '';
  const host = server.includes(':') ? `[${server}]` : server;
  return port > 0 ? `${host}:${port}` : host;
}

function groupKind(type: string): GroupKind {
  if (type === 'select') return 'select';
  if (type === 'urltest') return 'urltest';
  return 'other';
}

function nodeMember(node: Node, selected: boolean): MemberView {
  return {
    id: node.id,
    name: nodeDisplayName(node),
    kind: 'node',
    protocol: protocolLabel(node.type),
    badge: protocolBadge(node.type),
    address: formatAddress(node.server, node.port),
    latencyMs: node.latencyMs,
    latencyStatus: node.latencyStatus,
    favorite: node.favorite,
    selected,
  };
}

function groupMember(group: Group, selected: boolean): MemberView {
  return {
    id: group.id,
    name: group.name !== '' ? group.name : group.id,
    kind: 'group',
    protocol: t('分组', 'Group'),
    badge: t('组', 'GRP'),
    address: '',
    latencyMs: -1,
    latencyStatus: LATENCY_IDLE,
    favorite: false,
    selected,
  };
}

function unknownMember(id: string, selected: boolean): MemberView {
  return {
    id,
    name: id,
    kind: 'unknown',
    protocol: t('未知成员', 'Unknown'),
    badge: '?',
    address: '',
    latencyMs: -1,
    latencyStatus: LATENCY_IDLE,
    favorite: false,
    selected,
  };
}

interface Lookup {
  readonly nodes: ReadonlyMap<string, Node>;
  readonly groups: ReadonlyMap<string, Group>;
}

function resolveMember(id: string, lookup: Lookup, selected: boolean): MemberView {
  const node = lookup.nodes.get(id);
  if (node !== undefined) return nodeMember(node, selected);
  const group = lookup.groups.get(id);
  if (group !== undefined) return groupMember(group, selected);
  return unknownMember(id, selected);
}

/** 延迟排序的档位：有数值的在前（测速中沿用上一次的数值），然后是超时，最后是没测过的。 */
function latencyRank(member: MemberView): number {
  if (member.latencyMs >= 0) return 0;
  return member.latencyStatus === LATENCY_TIMEOUT ? 1 : 2;
}

function byLatency(a: MemberView, b: MemberView): number {
  const rank = latencyRank(a) - latencyRank(b);
  if (rank !== 0) return rank;
  return latencyRank(a) === 0 ? a.latencyMs - b.latencyMs : 0;
}

function applyFilter(members: readonly MemberView[], filter: NodeFilter): MemberView[] {
  const query = filter.query.trim().toLowerCase();
  const visible = members.filter((member) => {
    if (filter.favoritesOnly && !member.favorite) return false;
    if (query === '') return true;
    return (
      member.name.toLowerCase().includes(query) ||
      member.protocol.toLowerCase().includes(query) ||
      member.address.toLowerCase().includes(query)
    );
  });
  if (filter.sort === 'name') return [...visible].sort((a, b) => a.name.localeCompare(b.name, getLocale()));
  if (filter.sort === 'latency') return [...visible].sort(byLatency);
  return visible;
}

function toGroupView(group: Group, lookup: Lookup, filter: NodeFilter): GroupView {
  const kind = groupKind(group.type);
  const selectedId = kind === 'select' ? (group.selected ?? '') : '';
  const members = group.members.map((id) => resolveMember(id, lookup, selectedId !== '' && id === selectedId));
  const selected = members.find((member) => member.selected);
  return {
    id: group.id,
    name: group.name !== '' ? group.name : group.id,
    kind,
    selectedName: selected?.name ?? '',
    total: members.length,
    members: applyFilter(members, filter),
  };
}

function buildLookup(list: NodeList): Lookup {
  return {
    nodes: new Map(list.nodes.map((node) => [node.id, node] as const)),
    groups: new Map(list.groups.map((group) => [group.id, group] as const)),
  };
}

/** 按后端给出的分组顺序生成视图；不属于任何分组的节点收在末尾的「未加入分组」里。 */
export function buildGroupViews(list: NodeList, filter: NodeFilter): GroupView[] {
  const lookup = buildLookup(list);
  const views = list.groups.map((group) => toGroupView(group, lookup, filter));

  const orphans = list.nodes.filter((node) => node.groupId === '');
  if (orphans.length === 0) return views;

  const orphanMembers = orphans.map((node) => nodeMember(node, false));
  return [
    ...views,
    {
      id: ORPHAN_GROUP_ID,
      name: t('未加入分组', 'Ungrouped'),
      kind: 'orphan',
      selectedName: '',
      total: orphanMembers.length,
      members: applyFilter(orphanMembers, filter),
    },
  ];
}

export interface CurrentSelection {
  readonly name: string;
  readonly badge: string;
  /** 「分组名 · 协议」一类的说明。 */
  readonly detail: string;
  /** true 表示出口由内核自动选择，name 是分组名而不是具体节点。 */
  readonly automatic: boolean;
  /** 后端给出的延迟；-1 表示尚未测速。 */
  readonly latencyMs: number;
}

/** 概览页使用：第一个分组当前（或启动后）使用的出口。没有分组或没有选中项时返回 null。 */
export function currentSelection(list: NodeList): CurrentSelection | null {
  const group = list.groups[0];
  if (group === undefined) return null;

  const groupName = group.name !== '' ? group.name : group.id;
  if (groupKind(group.type) !== 'select') {
    return { name: groupName, badge: 'AUTO', detail: t('出口节点由内核自动选择', 'Auto-selected by core'), automatic: true, latencyMs: -1 };
  }

  const selectedId = group.selected ?? '';
  if (selectedId === '') return null;
  const member = resolveMember(selectedId, buildLookup(list), true);
  return {
    name: member.name,
    badge: member.badge,
    detail: `${groupName} · ${member.protocol}`,
    automatic: false,
    latencyMs: member.latencyMs,
  };
}

/** 把 nodes:latency 的增量并进节点列表，返回新列表；没有任何节点受影响时返回原对象。 */
export function mergeLatency(list: NodeList, updates: LatencyMap): NodeList {
  let changed = false;
  const nodes = list.nodes.map((node) => {
    const update = Object.hasOwn(updates, node.id) ? updates[node.id] : undefined;
    if (update === undefined) return node;
    changed = true;
    // 测速中保留上一次的数值，列表按延迟排序时才不会来回跳。
    const latencyMs = update.status === LATENCY_TESTING ? node.latencyMs : update.latencyMs;
    return { ...node, latencyMs, latencyStatus: update.status };
  });
  return changed ? { ...list, nodes } : list;
}

export interface LatencyView {
  readonly text: string;
  /** .lat 的颜色类。 */
  readonly level: 'ok' | 'warn' | 'err' | 'none';
  readonly busy: boolean;
}

export function latencyView(latencyMs: number, status: string): LatencyView {
  if (status === LATENCY_TESTING) return { text: t('测速中…', 'Testing…'), level: 'none', busy: true };
  if (status === LATENCY_TIMEOUT) return { text: t('超时', 'Timeout'), level: 'err', busy: false };
  return { text: formatLatency(latencyMs), level: latencyLevel(latencyMs), busy: false };
}

/** 要测速的节点 id：不给分组就是全部节点；给分组则只取其中的节点成员（去重，不展开子分组）。 */
export function latencyTargets(list: NodeList, groupId?: string): string[] {
  if (groupId === undefined) return list.nodes.map((node) => node.id);
  if (groupId === ORPHAN_GROUP_ID) return list.nodes.filter((node) => node.groupId === '').map((node) => node.id);
  const group = list.groups.find((item) => item.id === groupId);
  if (group === undefined) return [];
  const known = new Set(list.nodes.map((node) => node.id));
  return [...new Set(group.members)].filter((id) => known.has(id));
}

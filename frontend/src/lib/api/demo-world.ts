/*
 * 演示后端的共享状态：内核、节点、设置、流量、连接与事件分发。
 *
 * 各功能区（demo-core、demo-subs、demo-rules、demo-inspect、demo-system）持有同一个
 * DemoWorld，只通过它读写共享状态；状态一律整体替换，不就地修改。
 * 数据全部是合成值，服务器地址使用保留域名 .invalid。
 */

import type { EventHandler, Unsubscribe } from './backend';
import { cloneSettings, createDefaultSettings } from './defaults';
import { BackendError } from './errors';
import {
  CORE_STATUS_RUNNING,
  CORE_STATUS_STOPPED,
  EVENT_CORE_STATE,
  EVENT_TRAFFIC,
  isProxyMode,
  type BackendEvent,
  type CoreState,
  type Group,
  type Node,
  type NodeList,
  type SettingsValue,
  type Traffic,
} from './types';
import type { Connection } from './types-inspect';

export const DEMO_GROUP_ID = 'proxy';
export const DEMO_GROUP_NAME = '代理';

const HISTORY_LENGTH = 60;
const TICK_MS = 1000;
const START_DELAY_MS = 700;
const STOP_DELAY_MS = 300;
const RECONFIGURE_DELAY_MS = 400;
const DEMO_VERSION = 'demo';
const MEBIBYTE = 1024 * 1024;

const DEMO_SEEDS = [
  { id: 'demo-a', type: 'vless', port: 443 },
  { id: 'demo-b', type: 'trojan', port: 443 },
  { id: 'demo-c', type: 'shadowsocks', port: 8388 },
  { id: 'demo-d', type: 'socks', port: 1080 },
] as const;

export type TickListener = () => void;
export type CoreListener = (running: boolean) => void;
export type ResetListener = () => void;

/** 与真实后端一样：业务上被拒绝的操作，文案原样展示。 */
export function rejected(message: string): BackendError {
  return new BackendError('rejected', message);
}

/** 演示模式里没有对应实现的操作（例如读写本机文件）。 */
export function unsupported(message = '演示模式不提供此功能'): BackendError {
  return new BackendError('unsupported', message);
}

export function delay(ms: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

export function makeDemoNode(id: string, name: string, type: string, server: string, port: number): Node {
  return {
    id,
    tag: id,
    name,
    groupId: DEMO_GROUP_ID,
    type,
    server,
    port,
    latencyMs: -1,
    latencyStatus: 'idle',
    selected: false,
    favorite: false,
  };
}

function seedNodes(): Node[] {
  return DEMO_SEEDS.map((seed, index) =>
    makeDemoNode(
      seed.id,
      `演示节点 ${String.fromCharCode(65 + index)}`,
      seed.type,
      `${seed.id}.example.invalid`,
      seed.port,
    ),
  );
}

function emptyTraffic(): Traffic {
  return {
    up: 0,
    down: 0,
    activeConns: 0,
    totalUp: 0,
    totalDown: 0,
    memoryBytes: 0,
    upHistory: new Array<number>(HISTORY_LENGTH).fill(0),
    downHistory: new Array<number>(HISTORY_LENGTH).fill(0),
  };
}

function nextRate(previous: number, ceiling: number): number {
  const drift = (Math.random() - 0.45) * ceiling * 0.35;
  const next = previous * 0.7 + drift + ceiling * 0.08;
  return Math.max(0, Math.min(ceiling, Math.round(next)));
}

function pushHistory(history: readonly number[], value: number): number[] {
  return [...history.slice(1 - HISTORY_LENGTH), value];
}

function subscribe<T>(listeners: Set<T>, listener: T): Unsubscribe {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export class DemoWorld {
  #running = false;
  #mode = 'rule';
  #startedAt = 0;
  #settings: SettingsValue = createDefaultSettings();
  #nodes: readonly Node[] = seedNodes();
  #groups: readonly Group[] = [];
  #selected: string = DEMO_SEEDS[0].id;
  #traffic: Traffic = emptyTraffic();
  #connections: readonly Connection[] = [];
  #timer: number | null = null;
  #listeners = 0;
  readonly #handlers = new Map<BackendEvent, Set<EventHandler>>();
  readonly #tickers = new Set<TickListener>();
  readonly #coreListeners = new Set<CoreListener>();
  readonly #resetListeners = new Set<ResetListener>();

  get running(): boolean {
    return this.#running;
  }

  get mode(): string {
    return this.#mode;
  }

  /** 节点原始数据（selected 字段未填充）；展示用 nodeList()。 */
  get nodes(): readonly Node[] {
    return this.#nodes;
  }

  get groups(): readonly Group[] {
    return this.#groups;
  }

  setGroups(groups: readonly Group[]): void {
    this.#groups = [...groups];
  }

  get selected(): string {
    return this.#selected;
  }

  get traffic(): Traffic {
    return this.#traffic;
  }

  /** 当前活动连接。由 demo-inspect 写入，规则区按 id 读取（「据此创建规则」）。 */
  get connections(): readonly Connection[] {
    return this.#connections;
  }

  /** 设置的副本，调用方可以放心修改。 */
  get settings(): SettingsValue {
    return cloneSettings(this.#settings);
  }

  coreState(): CoreState {
    return {
      status: this.#running ? CORE_STATUS_RUNNING : CORE_STATUS_STOPPED,
      mode: this.#mode,
      uptimeMs: this.#running ? Date.now() - this.#startedAt : 0,
      memoryBytes: this.#traffic.memoryBytes,
      version: DEMO_VERSION,
      coreVersion: DEMO_VERSION,
      serviceMode: false,
    };
  }

  nodeList(): NodeList {
    if (this.#nodes.length === 0 && this.#groups.length === 0) return { groups: [], nodes: [] };
    const groups: Group[] =
      this.#groups.length > 0
        ? this.#groups.map((g) => ({ ...g, selected: g.selected ?? g.members[0] ?? '' }))
        : [
            {
              id: DEMO_GROUP_ID,
              name: DEMO_GROUP_NAME,
              type: 'select',
              members: this.#nodes.map((node) => node.id),
              selected: this.#selected,
              testUrl: '',
              intervalMin: 0,
            },
          ];
    return {
      groups,
      nodes: this.#nodes.map((node) => {
        const owner = groups.find((g) => g.members.includes(node.id));
        return { ...node, groupId: owner?.id ?? '', selected: owner?.selected === node.id };
      }),
    };
  }

  async start(): Promise<void> {
    if (this.#running) return;
    if (this.#nodes.length === 0) throw rejected('请先在「订阅」页导入节点');
    await delay(START_DELAY_MS);
    this.#setRunning(true);
  }

  async stop(): Promise<void> {
    if (!this.#running) return;
    await delay(STOP_DELAY_MS);
    this.#setRunning(false);
  }

  /**
   * 模拟「配置变更后重建内核」（规则、应用分组、订阅更新、恢复备份）：
   * 运行中会短暂重启，运行时长归零、连接清空；已停止时只是提交，什么都不用做。
   */
  async reconfigure(): Promise<void> {
    if (!this.#running) return;
    await delay(RECONFIGURE_DELAY_MS);
    if (!this.#running) return;
    this.#startedAt = Date.now();
    this.#connections = [];
    this.emit(EVENT_CORE_STATE, this.coreState());
  }

  setMode(mode: string): void {
    if (!isProxyMode(mode)) throw rejected('代理模式无效');
    this.#mode = mode;
    this.emit(EVENT_CORE_STATE, this.coreState());
  }

  /** 替换全部节点。当前选中的节点还在时保留，否则选中第一个。 */
  setNodes(nodes: readonly Node[]): void {
    this.#nodes = [...nodes];
    if (!nodes.some((node) => node.id === this.#selected)) this.#selected = nodes[0]?.id ?? '';
  }

  /** 逐个节点生成新值（收藏、测速结果等）。 */
  mapNodes(update: (node: Node) => Node): void {
    this.#nodes = this.#nodes.map(update);
  }

  select(nodeId: string): void {
    this.#selected = nodeId;
  }

  setSettings(value: SettingsValue): void {
    this.#settings = cloneSettings(value);
  }

  setConnections(connections: readonly Connection[]): void {
    this.#connections = [...connections];
  }

  /** 开发入口导入整份配置：替换节点，并通知各功能区丢弃依附于旧配置的数据（例如订阅记录）。 */
  replaceConfig(nodes: readonly Node[]): void {
    for (const listener of [...this.#resetListeners]) listener();
    this.#nodes = [...nodes];
    this.#selected = nodes[0]?.id ?? '';
  }

  emit(event: BackendEvent, payload: unknown): void {
    for (const handler of [...(this.#handlers.get(event) ?? [])]) handler(payload);
  }

  /** 订阅后端事件。定时器只在有订阅者时运行。 */
  on(event: BackendEvent, handler: EventHandler): Unsubscribe {
    const handlers = this.#handlers.get(event) ?? new Set<EventHandler>();
    this.#handlers.set(event, handlers);
    handlers.add(handler);
    this.#listeners += 1;
    this.#syncTimer();

    let active = true;
    return () => {
      if (!active) return;
      active = false;
      handlers.delete(handler);
      this.#listeners -= 1;
      this.#syncTimer();
    };
  }

  /** 内核运行期间每秒调用一次，先于当秒的 traffic 事件。用来生成连接、日志等合成数据。 */
  onTick(listener: TickListener): Unsubscribe {
    return subscribe(this.#tickers, listener);
  }

  /** 内核启动或停止之后调用（先于 core:state 事件）。 */
  onCoreChange(listener: CoreListener): Unsubscribe {
    return subscribe(this.#coreListeners, listener);
  }

  /** 整份配置被替换之前调用。 */
  onReset(listener: ResetListener): Unsubscribe {
    return subscribe(this.#resetListeners, listener);
  }

  #setRunning(running: boolean): void {
    this.#running = running;
    this.#startedAt = running ? Date.now() : 0;
    this.#traffic = emptyTraffic();
    this.#connections = [];
    for (const listener of [...this.#coreListeners]) listener(running);
    this.emit(EVENT_CORE_STATE, this.coreState());
    this.emit(EVENT_TRAFFIC, this.#traffic);
  }

  #tick(): void {
    if (!this.#running) return;
    for (const listener of [...this.#tickers]) listener();
    const previous = this.#traffic;
    const up = nextRate(previous.up, 600 * 1024);
    const down = nextRate(previous.down, 6 * MEBIBYTE);
    const activeConns =
      this.#connections.length > 0 ? this.#connections.length : 8 + Math.round(Math.random() * 14);
    this.#traffic = {
      up,
      down,
      activeConns,
      totalUp: previous.totalUp + up,
      totalDown: previous.totalDown + down,
      memoryBytes: (34 + Math.round(Math.random() * 6)) * MEBIBYTE,
      upHistory: pushHistory(previous.upHistory, up),
      downHistory: pushHistory(previous.downHistory, down),
    };
    this.emit(EVENT_TRAFFIC, this.#traffic);
  }

  #syncTimer(): void {
    if (this.#listeners > 0 && this.#timer === null) {
      this.#timer = window.setInterval(() => this.#tick(), TICK_MS);
    } else if (this.#listeners === 0 && this.#timer !== null) {
      window.clearInterval(this.#timer);
      this.#timer = null;
    }
  }
}

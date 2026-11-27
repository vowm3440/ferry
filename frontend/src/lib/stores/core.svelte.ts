/*
 * 内核状态与流量。
 *
 * 后端每秒推送 traffic；core:state 只在 status/mode/error 变化时推送，
 * 所以运行时长在本地按最近一次 uptimeMs 续算，不靠轮询。
 */

import {
  EVENT_CORE_STATE,
  EVENT_TRAFFIC,
  isCoreBusy,
  isCoreRunning,
  isProxyMode,
  messageOfError,
  parseCoreState,
  parseTraffic,
  type Backend,
  type CoreState,
  type ProxyMode,
  type Traffic,
  type Unsubscribe,
} from '../api';
import { tr } from '../i18n/index.svelte';
import { SUCCESS, failureMessage, type ActionResult } from '../result';

export type CorePending = 'start' | 'stop' | 'restart' | 'mode' | 'configure';

const UPTIME_TICK_MS = 1000;

class CoreStore {
  state = $state.raw<CoreState | null>(null);
  traffic = $state.raw<Traffic | null>(null);
  pending = $state<CorePending | null>(null);
  /** 最近一次启停/模式操作失败的说明；下一次操作开始时清空。 */
  actionError = $state('');
  /** 读取内核状态失败的说明。 */
  loadError = $state('');
  uptimeMs = $state(0);

  running = $derived(this.state !== null && isCoreRunning(this.state.status));
  /** 启停过程中（本地发起的，或后端上报的过渡态）。 */
  transitioning = $derived(
    this.pending === 'start' ||
      this.pending === 'stop' ||
      this.pending === 'restart' ||
      (this.state !== null && isCoreBusy(this.state.status)),
  );
  mode = $derived<ProxyMode>(this.state !== null && isProxyMode(this.state.mode) ? this.state.mode : 'rule');
  memoryBytes = $derived(this.traffic?.memoryBytes || this.state?.memoryBytes || 0);

  #backend: Backend | null = null;
  #offs: Unsubscribe[] = [];
  #timer: number | null = null;
  #uptimeBase = 0;
  #uptimeAt = 0;

  readonly #onVisible = (): void => {
    if (document.visibilityState === 'visible') void this.refresh();
  };

  async attach(backend: Backend): Promise<void> {
    this.detach();
    this.#backend = backend;
    this.#offs = [
      backend.on(EVENT_CORE_STATE, (payload) => this.#applyState(parseCoreState(payload))),
      backend.on(EVENT_TRAFFIC, (payload) => {
        this.traffic = parseTraffic(payload);
      }),
    ];
    this.#timer = window.setInterval(() => this.#tick(), UPTIME_TICK_MS);
    document.addEventListener('visibilitychange', this.#onVisible);
    await this.refresh();
  }

  detach(): void {
    for (const off of this.#offs) off();
    this.#offs = [];
    if (this.#timer !== null) {
      window.clearInterval(this.#timer);
      this.#timer = null;
    }
    document.removeEventListener('visibilitychange', this.#onVisible);
    this.#backend = null;
    this.state = null;
    this.traffic = null;
    this.pending = null;
    this.actionError = '';
    this.loadError = '';
    this.uptimeMs = 0;
  }

  async refresh(): Promise<void> {
    const backend = this.#backend;
    if (backend === null) return;
    try {
      const state = await backend.coreStatus();
      if (this.#backend !== backend) return;
      this.#applyState(state);
      this.loadError = '';
    } catch (error: unknown) {
      if (this.#backend === backend) this.loadError = messageOfError(error);
      return;
    }
    try {
      const traffic = await backend.coreSnapshot();
      if (this.#backend === backend) this.traffic = traffic;
    } catch {
      // 快照只是辅助数据：读不到时显示「暂无流量数据」，下一次 traffic 事件会补上。
      if (this.#backend === backend) this.traffic = null;
    }
  }

  #applyState(state: CoreState): void {
    this.state = state;
    this.#uptimeBase = state.uptimeMs;
    this.#uptimeAt = Date.now();
    this.uptimeMs = isCoreRunning(state.status) ? state.uptimeMs : 0;
  }

  #tick(): void {
    if (!this.running) {
      if (this.uptimeMs !== 0) this.uptimeMs = 0;
      return;
    }
    this.uptimeMs = this.#uptimeBase + (Date.now() - this.#uptimeAt);
  }

  async #run(
    kind: CorePending,
    action: (backend: Backend) => Promise<void>,
    trackError: boolean,
  ): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(tr('未连接到桌面后端', 'Not connected to desktop backend'));
    if (this.pending !== null) return failureMessage(tr('上一个操作还在进行中，请稍候', 'Previous operation in progress; please wait'));

    this.pending = kind;
    if (trackError) this.actionError = '';
    try {
      await action(backend);
      await this.refresh();
      return SUCCESS;
    } catch (error: unknown) {
      const message = messageOfError(error);
      if (trackError) this.actionError = message;
      await this.refresh();
      return failureMessage(message);
    } finally {
      this.pending = null;
    }
  }

  start(): Promise<ActionResult> {
    return this.#run('start', (backend) => backend.coreStart(), true);
  }

  stop(): Promise<ActionResult> {
    return this.#run('stop', (backend) => backend.coreStop(), true);
  }

  restart(): Promise<ActionResult> {
    return this.#run('restart', (backend) => backend.coreRestart(), true);
  }

  toggle(): Promise<ActionResult> {
    return this.running ? this.stop() : this.start();
  }

  setMode(mode: ProxyMode): Promise<ActionResult> {
    return this.#run('mode', (backend) => backend.coreSetMode(mode), true);
  }

  /** 开发入口：导入受限 RunSpec。错误由调用方就地展示，不占用概览页的错误位。 */
  configure(specJson: string): Promise<ActionResult> {
    return this.#run('configure', (backend) => backend.coreConfigure(specJson), false);
  }

  dismissActionError(): void {
    this.actionError = '';
  }
}

export const core = new CoreStore();

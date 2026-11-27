/*
 * 内核日志。
 *
 * 只有日志页打开时才工作：页面挂载时 activate()，卸载时 deactivate()。
 * 页面放在 CapGate 里，所以 activate() 被调用就意味着后端公布了 logs 能力；这里仍然再确认一次。
 *
 * 两个来源按 seq 合并（见 logs-view.ts）：Logs.Snapshot 取回最近的一批，logs:entry 事件实时追加。
 * 后端暂停期间仍在收集，但那段日志不会再通过事件补发，所以恢复之后要再读一次快照把空档补上。
 */

import {
  CAP_LOGS,
  EVENT_LOGS,
  messageOfError,
  parseLogEntries,
  type Backend,
  type LogEntry,
  type Unsubscribe,
} from '../api';
import { tr } from '../i18n/index.svelte';
import { LOG_RING_MAX, lastSeq, mergeEntries } from '../logs-view';
import { SUCCESS, failure, failureMessage, type ActionResult } from '../result';
import { caps } from './caps.svelte';
import { settings } from './settings.svelte';

const NONE: readonly LogEntry[] = [];
const notConnected = (): string => tr('未连接到桌面后端', 'Not connected to desktop backend');
const busyMsg = (): string => tr('上一个操作还在进行中，请稍候', 'Previous operation in progress; please wait');

class LogsStore {
  /** 按 seq 升序，最多 LOG_RING_MAX 条。 */
  entries = $state.raw<readonly LogEntry[]>(NONE);
  loaded = $state(false);
  error = $state('');
  /** 用户暂停了刷新。 */
  paused = $state(false);
  pausing = $state(false);
  levelSaving = $state(false);
  exporting = $state(false);

  #backend: Backend | null = null;
  #offs: Unsubscribe[] = [];
  #wanted = false;
  #live = false;
  /** 「清空显示」时记下的位置：seq 不大于它的日志不再显示。只影响界面，后端的日志不受影响。 */
  #floor = 0;

  get attached(): boolean {
    return this.#backend !== null;
  }

  async attach(backend: Backend): Promise<void> {
    this.detach();
    this.#backend = backend;
    if (this.#wanted) this.#start();
  }

  detach(): void {
    this.#stop();
    this.#backend = null;
    // 换了后端，序号从头算起。
    this.#floor = 0;
  }

  /** 日志页挂载时调用。 */
  activate(): void {
    this.#wanted = true;
    this.#start();
  }

  /** 日志页卸载时调用。 */
  deactivate(): void {
    this.#wanted = false;
    this.#stop();
  }

  /** 重新读取。页面没有打开时什么都不做。 */
  async refresh(): Promise<void> {
    const backend = this.#backend;
    if (backend === null || !this.#live) return;
    await this.#load(backend);
  }

  /** 清空显示：只清界面，之后只显示新产生的日志。 */
  clear(): void {
    this.#floor = Math.max(this.#floor, lastSeq(this.entries));
    this.entries = NONE;
  }

  async setPaused(on: boolean): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null || !this.#live) return failureMessage(notConnected());
    if (this.pausing) return failureMessage(busyMsg());
    this.pausing = true;
    try {
      await backend.logsPause(on);
      if (this.#backend !== backend || !this.#live) return SUCCESS;
      this.paused = on;
      if (!on) await this.#load(backend);
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.pausing = false;
    }
  }

  /** 修改内核的日志级别。后端会把它写进设置，所以随后重新读取设置。 */
  async setLevel(level: string): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(notConnected());
    if (this.levelSaving) return failureMessage(busyMsg());
    this.levelSaving = true;
    try {
      await backend.logsSetLevel(level);
      if (this.#backend === backend) await settings.load();
      return SUCCESS;
    } catch (error: unknown) {
      // 失败时后端可能已经回退，也可能没有：以后端为准重新读取。
      if (this.#backend === backend) await settings.load();
      return failure(error);
    } finally {
      this.levelSaving = false;
    }
  }

  /** 把后端收集的日志明文写到 path。调用方负责事先向用户确认。 */
  async exportTo(path: string): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(notConnected());
    if (this.exporting) return failureMessage(busyMsg());
    this.exporting = true;
    try {
      await backend.logsExport(path);
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.exporting = false;
    }
  }

  #start(): void {
    const backend = this.#backend;
    if (this.#live || backend === null || !caps.has(CAP_LOGS)) return;
    this.#live = true;
    this.#offs = [
      backend.on(EVENT_LOGS, (payload) => {
        // 运行时会把只有一个元素的数组负载拆成单个对象，这里还原成数组再解析。
        if (!this.paused) this.#merge(parseLogEntries(Array.isArray(payload) ? payload : [payload]));
      }),
    ];
    void this.#open(backend);
  }

  async #open(backend: Backend): Promise<void> {
    try {
      // 暂停开关在后端是全局的，界面重新加载后可能还留着上一次的暂停。
      await backend.logsPause(false);
    } catch (error: unknown) {
      if (this.#backend === backend && this.#live) {
        this.error = messageOfError(error);
        this.loaded = true;
      }
      return;
    }
    if (this.#backend === backend && this.#live) await this.#load(backend);
  }

  async #load(backend: Backend): Promise<void> {
    try {
      const list = await backend.logsSnapshot(LOG_RING_MAX);
      if (this.#backend !== backend || !this.#live) return;
      this.#merge(list);
      this.loaded = true;
      this.error = '';
    } catch (error: unknown) {
      if (this.#backend !== backend || !this.#live) return;
      this.error = messageOfError(error);
      this.loaded = true;
    }
  }

  #merge(incoming: readonly LogEntry[]): void {
    this.entries = mergeEntries(this.entries, incoming, this.#floor);
  }

  #stop(): void {
    for (const off of this.#offs) off();
    this.#offs = [];
    const backend = this.#backend;
    if (this.#live && this.paused && backend !== null) {
      // 离开页面时恢复后端的推送。失败了也没有可以提示的地方，下次进入页面时会再取消一次暂停。
      void backend.logsPause(false).catch(() => undefined);
    }
    this.#live = false;
    this.entries = NONE;
    this.loaded = false;
    this.error = '';
    this.paused = false;
    this.pausing = false;
  }
}

export const logs = new LogsStore();

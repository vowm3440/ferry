/*
 * 活动连接。
 *
 * 只有连接页打开时才工作：页面挂载时 activate()，卸载时 deactivate()；
 * 其余时间不订阅事件、不调用后端，也不保留数据。
 * 页面放在 CapGate 里，所以 activate() 被调用就意味着后端公布了 connections 能力；这里仍然再确认一次。
 *
 * 后端每秒推送一次完整列表（connections:snapshot 的 items）。暂停是后端的全局开关且没有读取接口，
 * 所以由这里记住「用户是否暂停」：进入页面时先取消可能残留的暂停，离开时恢复。
 */

import {
  CAP_CONNECTIONS,
  EVENT_CONNECTIONS,
  messageOfError,
  parseConnectionsEvent,
  type Backend,
  type Connection,
  type Rule,
  type Unsubscribe,
} from '../api';
import { extendSample, ratesBetween, sampleOf, type ConnSample, type RateMap } from '../conns-view';
import { tr } from '../i18n/index.svelte';
import { SUCCESS, failure, failureMessage, type ActionResult } from '../result';
import { caps } from './caps.svelte';

export type RuleDraftResult =
  | { readonly ok: true; readonly rule: Rule }
  | { readonly ok: false; readonly message: string };

const NO_RATES: RateMap = new Map();
const NO_IDS: ReadonlySet<string> = new Set<string>();
const notConnected = (): string => tr('未连接到桌面后端', 'Not connected to desktop backend');
const busyMsg = (): string => tr('上一个操作还在进行中，请稍候', 'Previous operation in progress; please wait');
/** 两次快照离得太近（例如关闭连接后立刻重读）时不重新计算速率，避免把抖动放大成尖峰。 */
const MIN_SAMPLE_MS = 400;

class ConnsStore {
  list = $state.raw<readonly Connection[]>([]);
  /** 每条连接的速率；没有出现在里面的连接速率未知。 */
  rates = $state.raw<RateMap>(NO_RATES);
  /** 最近一次快照的时刻，用来算连接时长；暂停时随列表一起停住。 */
  now = $state(0);
  loaded = $state(false);
  error = $state('');
  /** 用户暂停了刷新。 */
  paused = $state(false);
  pausing = $state(false);
  closing = $state.raw<ReadonlySet<string>>(NO_IDS);
  closingAll = $state(false);

  #backend: Backend | null = null;
  #offs: Unsubscribe[] = [];
  #wanted = false;
  #live = false;
  #sample: ConnSample | null = null;
  #loadSeq = 0;

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
  }

  /** 连接页挂载时调用。 */
  activate(): void {
    this.#wanted = true;
    this.#start();
  }

  /** 连接页卸载时调用。 */
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

  async setPaused(on: boolean): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null || !this.#live) return failureMessage(notConnected());
    if (this.pausing) return failureMessage(busyMsg());
    this.pausing = true;
    try {
      await backend.connsPause(on);
      if (this.#backend !== backend || !this.#live) return SUCCESS;
      this.paused = on;
      if (!on) {
        // 暂停期间的累计值不能拿来算速率：从头取样，第一秒显示「—」。
        this.#resetRates();
        await this.#load(backend);
      }
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.pausing = false;
    }
  }

  async close(id: string): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null || !this.#live) return failureMessage(notConnected());
    if (this.closing.has(id) || this.closingAll) return failureMessage(busyMsg());
    this.closing = new Set([...this.closing, id]);
    try {
      await backend.connsClose(id);
      // 暂停时没有事件，主动重读一次，让列表反映这次关闭。
      if (this.#backend === backend && this.#live) await this.#load(backend);
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.closing = new Set([...this.closing].filter((item) => item !== id));
    }
  }

  async closeAll(): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null || !this.#live) return failureMessage(notConnected());
    if (this.closingAll) return failureMessage(busyMsg());
    this.closingAll = true;
    try {
      await backend.connsCloseAll();
      if (this.#backend === backend && this.#live) await this.#load(backend);
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.closingAll = false;
    }
  }

  /** 让后端按这条连接预填一条规则（尚未保存）。连接已经结束时后端会如实拒绝。 */
  async ruleFromConnection(id: string): Promise<RuleDraftResult> {
    const backend = this.#backend;
    if (backend === null) return { ok: false, message: notConnected() };
    try {
      return { ok: true, rule: await backend.rulesCreateFromConnection(id) };
    } catch (error: unknown) {
      return { ok: false, message: messageOfError(error) };
    }
  }

  #start(): void {
    const backend = this.#backend;
    if (this.#live || backend === null || !caps.has(CAP_CONNECTIONS)) return;
    this.#live = true;
    this.#offs = [
      backend.on(EVENT_CONNECTIONS, (payload) => {
        if (!this.paused) this.#apply(parseConnectionsEvent(payload));
      }),
    ];
    void this.#open(backend);
  }

  async #open(backend: Backend): Promise<void> {
    try {
      // 暂停开关在后端是全局的，界面重新加载后可能还留着上一次的暂停。
      await backend.connsPause(false);
    } catch (error: unknown) {
      if (this.#backend === backend && this.#live) this.error = messageOfError(error);
      return;
    }
    if (this.#backend === backend && this.#live) await this.#load(backend);
  }

  async #load(backend: Backend): Promise<void> {
    this.#loadSeq += 1;
    const seq = this.#loadSeq;
    try {
      const list = await backend.connsSnapshot();
      if (this.#backend !== backend || !this.#live || seq !== this.#loadSeq) return;
      this.#apply(list);
    } catch (error: unknown) {
      if (this.#backend !== backend || !this.#live || seq !== this.#loadSeq) return;
      this.error = messageOfError(error);
      this.loaded = true;
    }
  }

  #apply(list: readonly Connection[]): void {
    const at = Date.now();
    if (this.#sample !== null && at - this.#sample.at < MIN_SAMPLE_MS) {
      const ids = new Set(list.map((item) => item.id));
      this.rates = new Map([...this.rates].filter(([id]) => ids.has(id)));
      this.#sample = extendSample(this.#sample, list);
    } else {
      this.rates = ratesBetween(this.#sample, list, at);
      this.#sample = sampleOf(list, at);
    }
    this.list = list;
    this.now = at;
    this.loaded = true;
    this.error = '';
  }

  #resetRates(): void {
    this.#sample = null;
    this.rates = NO_RATES;
  }

  #stop(): void {
    for (const off of this.#offs) off();
    this.#offs = [];
    const backend = this.#backend;
    if (this.#live && this.paused && backend !== null) {
      // 离开页面时恢复后端的刷新。失败了也没有可以提示的地方，下次进入页面时会再取消一次暂停。
      void backend.connsPause(false).catch(() => undefined);
    }
    this.#live = false;
    this.#loadSeq += 1;
    this.#resetRates();
    this.list = [];
    this.now = 0;
    this.loaded = false;
    this.error = '';
    this.paused = false;
    this.pausing = false;
    this.closing = NO_IDS;
    this.closingAll = false;
  }
}

export const conns = new ConnsStore();

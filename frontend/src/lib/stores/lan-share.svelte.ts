/*
 * 局域网分享。开启后，局域网内任何拿到链接的设备都能在 10 分钟内读取一次全部节点，
 * 所以只在用户明确点击时开启；这里不会自动调用任何接口（attach 只记下后端）。
 * 链接里带一次性令牌：这个全局 store 只记「开着没有、何时到期」，不保存链接本身；
 * 链接由 start() / refresh() 交给调用它的面板，放在面板自己的局部状态里，面板关掉就没了。
 */

import { messageOfError, type Backend, type LanShare } from '../api';
import { tr } from '../i18n/index.svelte';
import { SUCCESS, failure, failureMessage, type ActionResult } from '../result';

/** 开启分享的结果：成功时带上链接，只交给调用者。 */
export type LanStartResult = { readonly ok: true; readonly url: string } | { readonly ok: false; readonly message: string };
import { lanRemainingMs } from '../subs-view';

const TICK_MS = 1000;
/** 开启期间每隔这么多秒向后端确认一次：链接被读取一次后后端会提前关闭分享。 */
const POLL_EVERY_TICKS = 5;
const msgOffline = (): string => tr('未连接到桌面后端', 'Not connected to desktop backend');
const msgBusy = (): string => tr('上一个操作还在进行中，请稍候', 'Previous operation in progress; please wait');

class LanShareStore {
  /** 分享的到期时间（毫秒时间戳）；0 表示没有开启。 */
  expiresAt = $state(0);
  busy = $state(false);
  /** 读取状态失败的说明。 */
  error = $state('');
  /** 倒计时用的当前时间，仅在分享开启期间每秒更新。 */
  now = $state(Date.now());

  #backend: Backend | null = null;
  #timer: ReturnType<typeof setInterval> | null = null;
  #ticks = 0;

  get active(): boolean {
    return this.remainingMs > 0;
  }

  get remainingMs(): number {
    return Math.max(0, this.expiresAt - this.now);
  }

  attach(backend: Backend): void {
    this.detach();
    this.#backend = backend;
  }

  detach(): void {
    this.#stopTimer();
    this.#backend = null;
    this.expiresAt = 0;
    this.busy = false;
    this.error = '';
  }

  /**
   * 读取当前状态，返回正在进行的分享的链接（没有开启或读取失败时为空串）。
   * 只应在界面确认后端公布了 share.lan 能力之后调用。
   */
  async refresh(): Promise<string> {
    const backend = this.#backend;
    if (backend === null) return '';
    try {
      const status = await backend.lanShareStatus();
      if (this.#backend !== backend) return '';
      this.error = '';
      return this.#apply(status);
    } catch (error: unknown) {
      if (this.#backend === backend) this.error = messageOfError(error);
      return '';
    }
  }

  async start(): Promise<LanStartResult> {
    const backend = this.#backend;
    if (backend === null) return { ok: false, message: msgOffline() };
    if (this.busy) return { ok: false, message: msgBusy() };

    this.busy = true;
    try {
      const status = await backend.lanShareStart();
      return { ok: true, url: this.#backend === backend ? this.#apply(status) : '' };
    } catch (error: unknown) {
      return { ok: false, message: messageOfError(error) };
    } finally {
      this.busy = false;
    }
  }

  async stop(): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(msgOffline());
    if (this.busy) return failureMessage(msgBusy());

    this.busy = true;
    try {
      await backend.lanShareStop();
      if (this.#backend === backend) this.#apply(null);
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.busy = false;
    }
  }

  /** 记下到期时间并返回链接；链接不进 store。 */
  #apply(status: LanShare | null): string {
    this.now = Date.now();
    const open = status !== null && lanRemainingMs(status, this.now) > 0;
    this.expiresAt = open ? status.expiresAt : 0;
    if (open) this.#startTimer();
    else this.#stopTimer();
    return open ? status.url : '';
  }

  #startTimer(): void {
    if (this.#timer !== null) return;
    this.#timer = setInterval(() => this.#tick(), TICK_MS);
  }

  #stopTimer(): void {
    if (this.#timer === null) return;
    clearInterval(this.#timer);
    this.#timer = null;
  }

  #tick(): void {
    this.now = Date.now();
    this.#ticks += 1;
    if (this.remainingMs > 0) {
      if (this.#ticks % POLL_EVERY_TICKS === 0) void this.refresh();
      return;
    }
    // 到期：再向后端确认一次。
    this.#apply(null);
    void this.refresh();
  }
}

export const lanShare = new LanShareStore();

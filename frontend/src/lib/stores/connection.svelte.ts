/*
 * 与桌面后端的连接状态。
 *
 *   checking  启动时检测中
 *   no-bridge 普通浏览器：没有桌面桥接，不发起任何调用
 *   failed    有桥接但首次调用失败（例如后端尚未注册服务），可重试
 *   ready     已连接（真实后端或用户显式进入的演示后端）
 */

import {
  connectDesktop,
  hasDesktopBridge,
  isDemoRequested,
  loadDemoBackend,
  messageOfError,
  type Backend,
} from '../api';

export type ConnectionPhase = 'checking' | 'no-bridge' | 'failed' | 'ready';

class ConnectionStore {
  phase = $state<ConnectionPhase>('checking');
  backend = $state.raw<Backend | null>(null);
  error = $state('');
  isDemo = $derived(this.backend?.kind === 'demo');

  #busy = false;

  /** 连接真实后端，并用 Core.Status 探测服务是否已注册。 */
  async connect(): Promise<void> {
    if (this.#busy) return;
    this.#busy = true;
    this.phase = 'checking';
    this.error = '';
    this.backend = null;

    try {
      if (!hasDesktopBridge()) {
        if (isDemoRequested()) {
          this.backend = await loadDemoBackend();
          this.phase = 'ready';
        } else {
          this.phase = 'no-bridge';
        }
        return;
      }
      const backend = await connectDesktop();
      await backend.coreStatus();
      this.backend = backend;
      this.phase = 'ready';
    } catch (error: unknown) {
      this.error = messageOfError(error);
      this.phase = 'failed';
    } finally {
      this.#busy = false;
    }
  }

  /** 用户显式进入演示模式。桌面壳内不可用。 */
  async enterDemo(): Promise<void> {
    if (this.#busy || hasDesktopBridge()) return;
    this.#busy = true;
    try {
      this.backend = await loadDemoBackend();
      this.error = '';
      this.phase = 'ready';
    } catch (error: unknown) {
      this.error = messageOfError(error);
      this.phase = 'failed';
    } finally {
      this.#busy = false;
    }
  }

  /** 退出演示模式，回到「未连接」。 */
  leaveDemo(): void {
    if (this.backend?.kind !== 'demo') return;
    this.backend = null;
    this.phase = 'no-bridge';
  }
}

export const connection = new ConnectionStore();

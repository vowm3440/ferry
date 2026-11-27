/*
 * 后端公布的能力（System.Capabilities）。本次运行内不变，挂接时读取一次。
 *
 * 界面用它决定哪些入口可用。读取失败或尚未读到时一律当作「没有这项能力」，
 * 不做猜测：宁可少显示一个入口，也不去调用后端没有公布的接口。
 */

import { messageOfError, type Backend } from '../api';

const NONE: ReadonlySet<string> = new Set<string>();

class CapsStore {
  /** 已读到能力列表（即使是空的）。读到之前不要把「没有」当成最终结论去提示用户。 */
  loaded = $state(false);
  /** 读取失败的说明。 */
  error = $state('');

  #names = $state.raw<ReadonlySet<string>>(NONE);
  #backend: Backend | null = null;

  async attach(backend: Backend): Promise<void> {
    this.detach();
    this.#backend = backend;
    await this.refresh();
  }

  detach(): void {
    this.#backend = null;
    this.#names = NONE;
    this.loaded = false;
    this.error = '';
  }

  /** 读取失败后的重试入口。 */
  async refresh(): Promise<void> {
    const backend = this.#backend;
    if (backend === null) return;
    try {
      const names = await backend.capabilities();
      if (this.#backend !== backend) return;
      this.#names = new Set(names);
      this.loaded = true;
      this.error = '';
    } catch (error: unknown) {
      if (this.#backend !== backend) return;
      this.#names = NONE;
      this.loaded = false;
      this.error = messageOfError(error);
    }
  }

  /** name 取 api 导出的 CAP_* 常量。 */
  has(name: string): boolean {
    return this.#names.has(name);
  }
}

export const caps = new CapsStore();

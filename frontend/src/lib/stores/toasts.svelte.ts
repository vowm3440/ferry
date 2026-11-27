/* 全局提示。错误提示停留更久，重复文案不叠加。 */

export type ToastKind = 'info' | 'ok' | 'error';

export interface Toast {
  readonly id: number;
  readonly kind: ToastKind;
  readonly message: string;
}

const DURATION_MS: Record<ToastKind, number> = {
  info: 4000,
  ok: 3200,
  error: 7000,
};

const MAX_VISIBLE = 4;

class ToastStore {
  items = $state.raw<Toast[]>([]);
  #nextId = 1;

  #push(kind: ToastKind, message: string): void {
    const text = message.trim();
    if (text === '') return;
    if (this.items.some((item) => item.kind === kind && item.message === text)) return;

    const id = this.#nextId;
    this.#nextId += 1;
    this.items = [...this.items, { id, kind, message: text }].slice(-MAX_VISIBLE);
    window.setTimeout(() => this.dismiss(id), DURATION_MS[kind]);
  }

  info(message: string): void {
    this.#push('info', message);
  }

  ok(message: string): void {
    this.#push('ok', message);
  }

  error(message: string): void {
    this.#push('error', message);
  }

  dismiss(id: number): void {
    this.items = this.items.filter((item) => item.id !== id);
  }
}

export const toasts = new ToastStore();

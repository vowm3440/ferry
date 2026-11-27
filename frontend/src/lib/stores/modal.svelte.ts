/*
 * 当前打开的对话框栈。
 *
 * 对话框（Dialog.svelte）打开时入栈、关闭时出栈。用途有两个：
 * - 全局快捷键（Ctrl+1…7 切页）在有对话框时不响应，避免把正在填写的内容连同页面一起丢掉；
 * - 叠在一起的对话框里，只有最上面那个响应 Esc 与 Tab。
 */

class ModalStore {
  #stack = $state.raw<readonly symbol[]>([]);

  /** 有对话框打开。 */
  get active(): boolean {
    return this.#stack.length > 0;
  }

  /** 入栈，返回这一层的标识。 */
  push(): symbol {
    const token = Symbol('modal');
    this.#stack = [...this.#stack, token];
    return token;
  }

  remove(token: symbol): void {
    this.#stack = this.#stack.filter((item) => item !== token);
  }

  isTop(token: symbol): boolean {
    return this.#stack[this.#stack.length - 1] === token;
  }
}

export const modal = new ModalStore();

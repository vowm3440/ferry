/*
 * @wailsio/runtime 的唯一入口，按需加载。
 *
 * 运行时在导入时会注册拖拽/右键监听并请求 /wails/custom.js，
 * 这些只在桌面壳内有意义。普通浏览器不加载它，也就不会产生无效请求。
 */

export type WailsRuntime = typeof import('@wailsio/runtime');

let pending: Promise<WailsRuntime> | null = null;

export function loadRuntime(): Promise<WailsRuntime> {
  if (pending === null) {
    pending = import('@wailsio/runtime');
  }
  return pending;
}

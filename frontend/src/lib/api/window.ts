/*
 * 窗口控制（自绘标题栏用）。API 全部来自 @wailsio/runtime 3.1.0 的 Window 与 Events：
 * Window.Minimise / ToggleMaximise / Close / IsMaximised，
 * Events.Types.Common.WindowMaximise 等事件名。
 *
 * 普通浏览器里没有窗口可控，返回 null，标题栏不渲染按钮。
 */

import { hasDesktopBridge } from './backend';
import { toBackendError } from './errors';
import { loadRuntime } from './runtime';

export interface WindowControls {
  minimise(): Promise<void>;
  toggleMaximise(): Promise<void>;
  close(): Promise<void>;
  isMaximised(): Promise<boolean>;
  /** 最大化/还原可能发生时回调；返回取消订阅函数。 */
  onStateChange(handler: () => void): () => void;
}

async function guarded<T>(action: () => Promise<T>): Promise<T> {
  try {
    return await action();
  } catch (error: unknown) {
    throw toBackendError(error);
  }
}

export async function loadWindowControls(): Promise<WindowControls | null> {
  if (!hasDesktopBridge()) return null;

  const { Window, Events } = await loadRuntime();
  const stateEvents = [
    Events.Types.Common.WindowMaximise,
    Events.Types.Common.WindowUnMaximise,
    Events.Types.Common.WindowRestore,
    Events.Types.Common.WindowFullscreen,
  ];

  return {
    minimise: () => guarded(() => Window.Minimise()),
    toggleMaximise: () => guarded(() => Window.ToggleMaximise()),
    close: () => guarded(() => Window.Close()),
    isMaximised: () => guarded(() => Window.IsMaximised()),
    onStateChange(handler: () => void): () => void {
      const offs = stateEvents.map((name) => Events.On(name, () => handler()));
      return () => {
        for (const off of offs) off();
      };
    },
  };
}

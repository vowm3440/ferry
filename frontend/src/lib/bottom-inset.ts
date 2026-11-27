/*
 * 贴在窗口底部的操作栏（例如设置页的保存栏）与右下角的全局提示会抢同一块位置。
 * 操作栏用这个 action 声明自己占用的高度，提示据此上移，不会盖住栏里的按钮。
 */

import type { Action } from 'svelte/action';

/** 写在根元素上的自定义属性；Toasts.svelte 读取它。 */
const PROPERTY = '--bottom-inset';
/** 提示与操作栏之间留出的间距。 */
const GAP_PX = 8;

export const reserveBottom: Action<HTMLElement> = (node) => {
  const root = document.documentElement;

  // 操作栏贴底显示，所以「窗口底边到操作栏顶边」的距离只随它自身的高度变化。
  const update = (): void => {
    const occupied = Math.ceil(window.innerHeight - node.getBoundingClientRect().top);
    root.style.setProperty(PROPERTY, `${Math.max(0, occupied) + GAP_PX}px`);
  };

  const observer = new ResizeObserver(update);
  observer.observe(node);
  update();

  return {
    destroy(): void {
      observer.disconnect();
      root.style.removeProperty(PROPERTY);
    },
  };
};

/*
 * 滚动定位：在一个滚动容器里找出「当前正在阅读」的分区，用于高亮设置页的目录。
 * 只读取布局，不改动滚动位置；滚动时用 requestAnimationFrame 合并计算。
 */

/** 参考线在滚动容器可视高度中的位置：越过这条线的最后一个分区视为当前分区。 */
const ANCHOR_RATIO = 0.25;
/** 判定「已滚到底」时允许的像素误差（缩放时滚动位置可能是小数）。 */
const BOTTOM_SLACK_PX = 2;

function activeSection(scroller: HTMLElement, ids: readonly string[]): string | null {
  const scrolled = scroller.scrollTop > 0;
  const atBottom = scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - BOTTOM_SLACK_PX;
  // 末尾的分区可能太短，永远到不了参考线：滚到底时直接算最后一个。
  if (scrolled && atBottom) return ids[ids.length - 1] ?? null;

  const line = scroller.getBoundingClientRect().top + scroller.clientHeight * ANCHOR_RATIO;
  const reached = ids.filter((id) => {
    const element = document.getElementById(id);
    return element !== null && element.getBoundingClientRect().top <= line;
  });
  return reached[reached.length - 1] ?? ids[0] ?? null;
}

/** 监听滚动与窗口尺寸变化，把当前分区的 id 交给回调。返回取消监听的函数。 */
export function watchSections(
  scroller: HTMLElement,
  ids: readonly string[],
  onActive: (id: string) => void,
): () => void {
  let frame = 0;

  const update = (): void => {
    frame = 0;
    const id = activeSection(scroller, ids);
    if (id !== null) onActive(id);
  };
  const schedule = (): void => {
    if (frame === 0) frame = window.requestAnimationFrame(update);
  };

  scroller.addEventListener('scroll', schedule, { passive: true });
  window.addEventListener('resize', schedule);
  schedule();

  return () => {
    scroller.removeEventListener('scroll', schedule);
    window.removeEventListener('resize', schedule);
    if (frame !== 0) window.cancelAnimationFrame(frame);
  };
}

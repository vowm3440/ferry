// 场景 C 的公共部分：伪造桌面桥接、模仿 Go 侧的事件推送、汇总控制台问题。
import { openPage } from './harness.mjs';

/** 只补上原生消息通道，让 hasDesktopBridge() 为真；不提供任何业务能力。 */
const BRIDGE_STUB = `window.chrome = window.chrome || {}; window.chrome.webview = { postMessage() {} };`;

export function dispatch(page, name, data) {
  return page.eval(`window._wails.dispatchWailsEvent({ name: ${JSON.stringify(name)}, data: ${JSON.stringify(data)}, sender: '' })`);
}

/** 模仿 cmd/ferry 的 emitSnapshots：状态变化时发 core:state，每次都发 traffic（停止时是全 0）。 */
export function createTicker(page, fake) {
  let previous = '';
  return async function tick({ wrapTraffic = false } = {}) {
    const state = fake.state();
    const key = `${state.status}|${state.mode}|${state.error ?? ''}`;
    if (key !== previous) {
      previous = key;
      await dispatch(page, 'core:state', state);
    }
    const traffic = fake.traffic();
    await dispatch(page, 'traffic', wrapTraffic ? [traffic] : traffic);
  };
}

export async function openBridgePage(browser, server, { width = 1180, height = 780 } = {}) {
  const page = await openPage(browser.port);
  await page.send('Page.addScriptToEvaluateOnNewDocument', { source: BRIDGE_STUB });
  await page.viewport(width, height);
  await page.goto(`${server.url}/`);
  return page;
}

/**
 * 控制台问题汇总。两类记录属于预期：
 *  - /wails/custom.js 的 404：桌面壳本来就这样应答（Wails v3.1.0 application.go 的中间件）；
 *  - 被后端拒绝的调用：Wails 用 HTTP 422 表示 Go 方法返回了 error，浏览器会各记一条网络错误。
 * 其余任何错误或警告都算问题。
 */
export function consoleReport(page, fake) {
  const rest = page.problems.filter((item) => !item.includes('/wails/custom.js'));
  const rejected = rest.filter((item) => item.startsWith('log.error: Failed to load resource') && item.includes('/wails/runtime'));
  const others = rest.filter((item) => !rejected.includes(item));
  return { others, rejected: rejected.length, expected: fake.s.failures };
}

export const activeMode = `document.querySelector('[role="radiogroup"][aria-label="代理模式"] button[aria-checked="true"]')?.innerText.trim()`;

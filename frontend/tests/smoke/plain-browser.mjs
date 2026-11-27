// 场景 A：普通浏览器（无桌面桥接）；场景 B：显式演示模式下走完七个页面与三种宽度。
import { CLIPPED, NO_OVERFLOW, createChecks, launchBrowser, openPage, sleep, startServer } from './harness.mjs';

const SIZES = [
  [1280, 800],
  [900, 640],
  [620, 700],
];
const PAGES = ['概览', '节点', '订阅', '规则', '连接', '日志', '设置'];

const { check, summary } = createChecks();

const FOCUS = `(() => {
  const el = document.activeElement;
  if (!el || el === document.body) return 'body';
  const style = getComputedStyle(el);
  const name = (el.getAttribute('aria-label') || el.innerText || el.getAttribute('title') || '').replace(/\\s+/g, ' ').trim().slice(0, 24);
  return el.tagName.toLowerCase() + '[' + name + '] outline=' + style.outlineStyle + '/' + style.outlineWidth + ' visible=' + el.matches(':focus-visible');
})()`;

async function scenarioPlain(browser, server) {
  console.log('\n== A 普通浏览器 ==');
  const page = await openPage(browser.port);
  await page.viewport(1280, 800);
  await page.goto(`${server.url}/`);

  check('A1 显示「未连接桌面后端」', await page.waitFor(`document.body.innerText.includes('未连接桌面后端')`));
  console.log('      正文：', (await page.text('main')).slice(0, 220));
  check('A2 没有向 /wails 发起任何请求', !server.requests.some((item) => item.includes('/wails')), server.requests.join(', '));
  check('A3 1280 宽无横向溢出', await page.eval(NO_OVERFLOW));
  await page.shot('a-nobridge-1280');

  await page.clickText('重新检测');
  check('A4 重新检测后仍诚实显示未连接', await page.waitFor(`document.body.innerText.includes('未连接桌面后端')`));
  check('A5 重新检测也没有请求 /wails', !server.requests.some((item) => item.includes('/wails')));

  await page.key('3', { ctrl: true, code: 'Digit3', vk: 51 });
  const active = await page.eval(`document.querySelector('.nav button[aria-current="page"]')?.innerText.replace(/\\s+/g, ' ').trim()`);
  check('A6 未连接时 Ctrl+3 仍能切换导航且不报错', String(active).startsWith('订阅'), String(active));
  check('A7 未连接时页面区不显示业务内容', await page.eval(`document.body.innerText.includes('未连接桌面后端')`));

  await page.viewport(620, 700);
  await sleep(250);
  check('A8 620 宽无横向溢出', await page.eval(NO_OVERFLOW));
  const clipped = await page.eval(CLIPPED);
  check('A9 620 宽没有被裁掉的元素', clipped.length === 0, clipped.join(' | '));
  await page.shot('a-nobridge-620');

  await page.viewport(1280, 800);
  await page.clickText('进入演示模式');
  check('A10 点击后进入演示模式并有明显标识', await page.waitFor(`document.body.innerText.includes('演示模式')`));
  console.log('      横幅：', (await page.text('.demo-banner, [class*="demo"]')).slice(0, 160));

  const policy = await page.eval(`document.querySelector('meta[http-equiv="Content-Security-Policy"]')?.content ?? ''`);
  check('A12 构建产物带内容安全策略：脚本只来自应用自身、不许内联', policy.includes("default-src 'none'") && policy.includes("script-src 'self'") && !/script-src[^;]*unsafe/.test(policy), policy);
  check('A11 全程没有控制台错误或警告', page.problems.length === 0, page.problems.join('\n      '));
  // 放在「没有控制台错误」之后：这一步会故意触发一条策略违规报告。
  const injected = await page.eval(`(() => { const el = document.createElement('script'); el.textContent = 'window.__injected = true'; document.head.append(el); return window.__injected === true; })()`);
  check('A13 注入的内联脚本不会执行', injected === false);
  page.close();
}

async function scenarioDemo(browser, server) {
  console.log('\n== B 演示模式 ==');
  const page = await openPage(browser.port);
  await page.viewport(1280, 800);
  await page.goto(`${server.url}/#demo`);
  check('B1 #demo 直接进入演示模式', await page.waitFor(`document.body.innerText.includes('演示模式')`));

  for (const [width, height] of SIZES) {
    await page.viewport(width, height);
    await sleep(250);
    for (const [index, label] of PAGES.entries()) {
      const digit = String(index + 1);
      await page.key(digit, { ctrl: true, code: `Digit${digit}`, vk: 48 + index + 1 });
      const reached = await page.waitFor(`document.querySelector('main h1')?.innerText.trim() === ${JSON.stringify(label)}`, 2500);
      const overflow = await page.eval(NO_OVERFLOW);
      const clipped = await page.eval(CLIPPED);
      await sleep(250);
      await page.shot(`b-${width}-${index + 1}`);
      check(`B ${width}px Ctrl+${digit} → ${label}`, reached && overflow && clipped.length === 0, `到达=${reached} 无溢出=${overflow} 被裁=${clipped.join(' | ')}`);
    }
  }

  await page.viewport(1280, 800);
  await page.key('1', { ctrl: true, code: 'Digit1', vk: 49 });
  await sleep(200);
  console.log('\n   Tab 顺序（概览页，前 16 个焦点）：');
  for (let step = 0; step < 16; step += 1) {
    await page.key('Tab', { code: 'Tab', vk: 9 });
    console.log(`     ${String(step + 1).padStart(2)}. ${await page.eval(FOCUS)}`);
  }

  await page.click('.foot-row .icon-btn');
  await sleep(300);
  console.log('\n   切换主题后：', await page.eval(`JSON.stringify({ attr: document.documentElement.getAttribute('data-theme'), scheme: getComputedStyle(document.documentElement).colorScheme })`));
  await page.shot('b-theme-1');
  await page.key('7', { ctrl: true, code: 'Digit7', vk: 55 });
  await sleep(300);
  await page.shot('b-theme-7');

  check('B 全程没有控制台错误或警告', page.problems.length === 0, page.problems.join('\n      '));
  page.close();
}

const server = await startServer();
const browser = await launchBrowser();
let passed = false;
try {
  await scenarioPlain(browser, server);
  await scenarioDemo(browser, server);
  passed = summary();
} catch (error) {
  console.error('冒烟脚本自身出错：', error);
} finally {
  await browser.close();
  await server.close();
}
process.exit(passed ? 0 : 1);

/*
 * 冒烟检查的公共工具：零依赖的静态服务器 + 无头浏览器 + 很薄的 CDP 客户端。
 * 每次运行都自己启动、自己退出，不留常驻进程；需要本机装有 Chrome 或 Edge。
 *
 * 用法：先 pnpm build，再 pnpm test:smoke —— 检查的是 dist 里的构建产物，后端是假的
 * （见 fake-backend.mjs），所以它不能代替「真实 WebView2 + Go 后端」的联调。
 *   FERRY_SMOKE_BROWSER  指定浏览器可执行文件；默认依次查找 Chrome、Edge 的常见安装位置。
 *   FERRY_SMOKE_SHOTS    指定目录后把各场景的截图写到那里；默认不保存截图。
 */
import { spawn } from 'node:child_process';
import { existsSync } from 'node:fs';
import { mkdtemp, readFile, rm, writeFile, mkdir } from 'node:fs/promises';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { extname, join, normalize, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

/** 默认检查 frontend/dist；FERRY_SMOKE_DIST 可以指到别处的构建产物（并行开发时各自构建到临时目录）。 */
const DIST = resolve(process.env.FERRY_SMOKE_DIST ?? fileURLToPath(new URL('../../dist', import.meta.url)));
const SHOTS_DIR = process.env.FERRY_SMOKE_SHOTS ?? '';
const BROWSER_CANDIDATES = [
  'C:/Program Files/Google/Chrome/Application/chrome.exe',
  'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe',
  'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe',
  'C:/Program Files/Microsoft/Edge/Application/msedge.exe',
];

function findBrowser() {
  const override = process.env.FERRY_SMOKE_BROWSER ?? '';
  if (override !== '') {
    if (!existsSync(override)) throw new Error(`FERRY_SMOKE_BROWSER 指向的文件不存在：${override}`);
    return override;
  }
  const found = BROWSER_CANDIDATES.find((path) => existsSync(path));
  if (found === undefined) {
    throw new Error('找不到 Chrome 或 Edge，请用环境变量 FERRY_SMOKE_BROWSER 指定浏览器可执行文件');
  }
  return found;
}

const MIME = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
  '.json': 'application/json',
};

export const sleep = (ms) => new Promise((done) => setTimeout(done, ms));

/** 页面内表达式：没有横向滚动条。 */
export const NO_OVERFLOW = `document.documentElement.scrollWidth <= window.innerWidth`;
/** 页面内表达式：水平方向被视口裁掉的可见元素（可横向滚动容器里的内容除外）。 */
export const CLIPPED = `(() => {
  const scrollsX = (el) => {
    for (let node = el.parentElement; node; node = node.parentElement) {
      const style = getComputedStyle(node);
      if ((style.overflowX === 'auto' || style.overflowX === 'scroll') && node.scrollWidth > node.clientWidth) return true;
    }
    return false;
  };
  return [...document.querySelectorAll('body *')]
    .filter((el) => {
      const rect = el.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) return false;
      return (rect.right > window.innerWidth + 0.5 || rect.left < -0.5) && !scrollsX(el);
    })
    .slice(0, 6)
    .map((el) => el.tagName.toLowerCase() + '.' + String(el.className).replace(/\\s+/g, '.') + ' right=' + Math.round(el.getBoundingClientRect().right));
})()`;

/** 静态服务器。extra(req, res) 返回 true 表示已处理（用来挂假的 /wails/runtime）。 */
export async function startServer(extra = null) {
  if (!existsSync(join(DIST, 'index.html'))) {
    throw new Error(`找不到 ${join(DIST, 'index.html')}，请先运行 pnpm build`);
  }
  const requests = [];
  const server = createServer(async (req, res) => {
    requests.push(`${req.method} ${req.url}`);
    try {
      if (extra !== null && (await extra(req, res))) return;
      const path = decodeURIComponent(new URL(req.url, 'http://x').pathname);
      const file = normalize(join(DIST, path === '/' ? 'index.html' : path));
      if (!file.startsWith(DIST)) {
        res.writeHead(403).end();
        return;
      }
      const body = await readFile(file);
      res.writeHead(200, { 'Content-Type': MIME[extname(file)] ?? 'application/octet-stream' });
      res.end(req.method === 'HEAD' ? undefined : body);
    } catch {
      res.writeHead(404, { 'Content-Type': 'text/plain' }).end('not found');
    }
  });
  await new Promise((done) => server.listen(0, '127.0.0.1', done));
  const { port } = server.address();
  return {
    url: `http://127.0.0.1:${port}`,
    requests,
    close: () => new Promise((done) => server.close(done)),
  };
}

export async function launchBrowser() {
  const executable = findBrowser();
  const profile = await mkdtemp(join(tmpdir(), 'ferry-smoke-profile-'));
  const child = spawn(
    executable,
    [
      '--headless=new',
      '--remote-debugging-port=0',
      `--user-data-dir=${profile}`,
      '--no-first-run',
      '--no-default-browser-check',
      '--disable-extensions',
      '--disable-background-networking',
      '--disable-sync',
      '--disable-component-update',
      '--force-device-scale-factor=1',
      '--lang=zh-CN',
      '--window-size=1280,800',
      'about:blank',
    ],
    { stdio: 'ignore' },
  );

  let port = 0;
  for (let attempt = 0; attempt < 100 && port === 0; attempt += 1) {
    await sleep(100);
    try {
      const text = await readFile(join(profile, 'DevToolsActivePort'), 'utf8');
      port = Number.parseInt(text.split('\n')[0], 10) || 0;
    } catch {
      // 浏览器还没写出端口文件，继续等。
    }
  }
  if (port === 0) {
    child.kill();
    throw new Error('浏览器没有给出调试端口');
  }

  return {
    port,
    async close() {
      child.kill();
      await sleep(400);
      await rm(profile, { recursive: true, force: true, maxRetries: 5, retryDelay: 200 }).catch(() => {});
    },
  };
}

/** 打开一个标签页并返回一个很薄的 CDP 会话。 */
export async function openPage(port) {
  const response = await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: 'PUT' });
  const target = await response.json();
  return connectPage(target.webSocketDebuggerUrl);
}

/** 连接一个已经打开的页面（真实 WebView2 联调用）：等它出现在调试端口的页面列表里。 */
export async function attachPage(port, timeoutMs = 20000) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    try {
      const targets = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json();
      const target = targets.find((item) => item.type === 'page' && item.webSocketDebuggerUrl);
      if (target !== undefined) return connectPage(target.webSocketDebuggerUrl);
    } catch {
      // 调试端口还没打开，继续等。
    }
    await sleep(200);
  }
  throw new Error('没有等到可以连接的页面');
}

async function connectPage(debuggerUrl) {
  const socket = new WebSocket(debuggerUrl);
  await new Promise((done, fail) => {
    socket.addEventListener('open', done, { once: true });
    socket.addEventListener('error', fail, { once: true });
  });

  let nextId = 0;
  const waiting = new Map();
  const listeners = new Map();
  const problems = [];

  socket.addEventListener('message', (event) => {
    const message = JSON.parse(event.data);
    if (message.id !== undefined) {
      const entry = waiting.get(message.id);
      waiting.delete(message.id);
      if (entry === undefined) return;
      if (message.error) entry.fail(new Error(`${entry.method}: ${message.error.message}`));
      else entry.done(message.result);
      return;
    }
    for (const listener of listeners.get(message.method) ?? []) listener(message.params);
  });

  const send = (method, params = {}) =>
    new Promise((done, fail) => {
      nextId += 1;
      waiting.set(nextId, { done, fail, method });
      socket.send(JSON.stringify({ id: nextId, method, params }));
    });
  const on = (method, listener) => listeners.set(method, [...(listeners.get(method) ?? []), listener]);

  on('Runtime.exceptionThrown', ({ exceptionDetails }) => {
    problems.push(`exception: ${exceptionDetails.exception?.description ?? exceptionDetails.text}`);
  });
  on('Runtime.consoleAPICalled', ({ type, args }) => {
    if (type !== 'error' && type !== 'warning') return;
    problems.push(`console.${type}: ${args.map((arg) => arg.value ?? arg.description ?? '').join(' ')}`);
  });
  on('Log.entryAdded', ({ entry }) => {
    if (entry.level !== 'error' && entry.level !== 'warning') return;
    problems.push(`log.${entry.level}: ${entry.text} ${entry.url ?? ''}`);
  });

  await send('Page.enable');
  await send('Runtime.enable');
  await send('Log.enable');

  const page = {
    send,
    on,
    problems,
    async viewport(width, height) {
      await send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1, mobile: false });
    },
    async goto(url) {
      const loaded = new Promise((done) => on('Page.loadEventFired', done));
      await send('Page.navigate', { url });
      await loaded;
      await sleep(350);
    },
    async eval(expression) {
      const { result, exceptionDetails } = await send('Runtime.evaluate', {
        expression,
        awaitPromise: true,
        returnByValue: true,
      });
      if (exceptionDetails) throw new Error(exceptionDetails.exception?.description ?? exceptionDetails.text);
      return result.value;
    },
    /** 页面可见文本，压成一行便于断言与打印。 */
    async text(selector = 'body') {
      return page.eval(
        `(document.querySelector(${JSON.stringify(selector)})?.innerText ?? '').replace(/\\s+/g, ' ').trim()`,
      );
    },
    async waitFor(expression, timeoutMs = 4000) {
      const deadline = Date.now() + timeoutMs;
      while (Date.now() < deadline) {
        if (await page.eval(expression)) return true;
        await sleep(80);
      }
      return false;
    },
    async click(selector) {
      const found = await page.eval(
        `(() => { const el = document.querySelector(${JSON.stringify(selector)}); if (!el) return false; el.click(); return true; })()`,
      );
      if (!found) throw new Error(`找不到元素：${selector}`);
      await sleep(200);
    },
    /** 按文字点击按钮（精确匹配去空白后的文本）。 */
    async clickText(label, scope = 'button') {
      const found = await page.eval(
        `(() => { const el = [...document.querySelectorAll(${JSON.stringify(scope)})].find((item) => item.innerText.replace(/\\s+/g, '') === ${JSON.stringify(label.replace(/\s+/g, ''))}); if (!el) return false; el.click(); return true; })()`,
      );
      if (!found) throw new Error(`找不到按钮：${label}`);
      await sleep(250);
    },
    async key(key, { ctrl = false, shift = false, code = '', vk = 0 } = {}) {
      const modifiers = (ctrl ? 2 : 0) | (shift ? 8 : 0);
      const base = { modifiers, key, code, windowsVirtualKeyCode: vk, nativeVirtualKeyCode: vk };
      await send('Input.dispatchKeyEvent', { type: 'rawKeyDown', ...base });
      await send('Input.dispatchKeyEvent', { type: 'keyUp', ...base });
      await sleep(150);
    },
    /** 只有设置了 FERRY_SMOKE_SHOTS 才保存截图；返回文件路径，未保存时为空串。 */
    async shot(name) {
      if (SHOTS_DIR === '') return '';
      const dir = resolve(SHOTS_DIR);
      await mkdir(dir, { recursive: true });
      const { data } = await send('Page.captureScreenshot', { format: 'png' });
      const file = join(dir, `${name}.png`);
      await writeFile(file, Buffer.from(data, 'base64'));
      return file;
    },
    close: () => socket.close(),
  };
  return page;
}

/** 简单的断言收集器：不中断流程，最后统一汇报。 */
export function createChecks() {
  const results = [];
  return {
    results,
    check(name, passed, detail = '') {
      results.push({ name, passed: Boolean(passed), detail });
      console.log(`${passed ? 'PASS' : 'FAIL'}  ${name}${passed || detail === '' ? '' : `\n      ${detail}`}`);
    },
    summary() {
      const failed = results.filter((item) => !item.passed);
      console.log(`\n${results.length - failed.length}/${results.length} 项通过`);
      return failed.length === 0;
    },
  };
}

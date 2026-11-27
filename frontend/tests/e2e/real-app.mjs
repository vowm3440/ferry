/*
 * 真实联调：启动带调试开关的 Ferry-devtools.exe（真实 WebView2 + Go 后端 + ferry-core.exe；
 * 用 scripts/build.ps1 -DevTools 生成，正式的 Ferry.exe 不理会 FERRY_DEVTOOLS_PORT），
 * 像用户一样操作界面，并用 curl 经混合入站发真实请求，证明流量确实走了节点。
 *
 * 需要一个可用节点，所以不在 pnpm test 里自动运行。节点凭据只从仓库外的文件读取：
 *   FERRY_E2E_RUNSPEC  受限 RunSpec 的 JSON 文件路径（放在仓库之外，不要入库）
 *   FERRY_E2E_LINKS    分享链接文本文件（每行一条）；给了它就在「订阅」页粘贴导入，
 *                      此时 RunSpec 里的 nodes 应为空数组，只用来设定入站端口
 *   FERRY_E2E_APP      可执行文件路径，默认 ../../../bin/Ferry-devtools.exe
 * 两者至少要有一个。混合入站端口会被替换成一个空闲端口，不会与本机其它代理客户端冲突，
 * 也不改系统代理。
 */
import { execFile, spawn } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import net from 'node:net';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';
import { attachPage, createChecks, sleep } from '../smoke/harness.mjs';

const run = promisify(execFile);
const APP = process.env.FERRY_E2E_APP ?? fileURLToPath(new URL('../../../bin/Ferry-devtools.exe', import.meta.url));
const SPEC_FILE = process.env.FERRY_E2E_RUNSPEC ?? '';
const LINK_FILE = process.env.FERRY_E2E_LINKS ?? '';
const TRACE_URL = 'https://www.cloudflare.com/cdn-cgi/trace';
const MODES = '[role="radiogroup"][aria-label="代理模式"] button';
const { check, summary } = createChecks();

async function freePort() {
  const probe = net.createServer();
  await new Promise((done) => probe.listen(0, '127.0.0.1', done));
  const { port } = probe.address();
  await new Promise((done) => probe.close(done));
  return port;
}

function portIsFree(port) {
  return new Promise((done) => {
    const probe = net.createServer();
    probe.once('error', () => done(false));
    probe.listen(port, '127.0.0.1', () => probe.close(() => done(true)));
  });
}

/** 出口 IP 与地区；proxyPort 为 0 表示不经代理。失败时返回 error，不抛出。 */
async function trace(proxyPort) {
  const route = proxyPort > 0 ? ['-x', `http://127.0.0.1:${proxyPort}`] : ['--noproxy', '*'];
  try {
    const { stdout } = await run('curl.exe', ['-s', '--max-time', '25', ...route, TRACE_URL]);
    const field = (name) => stdout.match(new RegExp(`^${name}=(.*)$`, 'm'))?.[1]?.trim() ?? '';
    return { ip: field('ip'), loc: field('loc'), error: '' };
  } catch (error) {
    return { ip: '', loc: '', error: error instanceof Error ? error.message.split('\n')[0] : String(error) };
  }
}

/** base64（含 URL 安全变体）解出的文本；不是 base64 时返回空串。 */
function decodeBase64(text) {
  if (!/^[A-Za-z0-9+/_=-]+$/.test(text)) return '';
  try {
    return Buffer.from(text, 'base64').toString('utf8');
  } catch {
    return '';
  }
}

/**
 * 界面里不该出现的凭据：出站里的 uuid / password，以及分享链接的用户信息——
 * 包括 ss:// 那种 base64 的 method:password 解出来的密码。
 * 只排除短于 4 个字符的串：它们在页面里碰巧出现的概率太高，查了也没有意义。
 */
function secretsOf(nodes, links) {
  const fromNodes = nodes.flatMap((node) => [node?.outbound?.uuid, node?.outbound?.password]);
  const fromLinks = links.split('\n').flatMap((line) => {
    const userinfo = /^[a-z0-9]+:\/\/([^@/]+)@/i.exec(line.trim())?.[1];
    if (userinfo === undefined) return [];
    let plain = userinfo;
    try {
      plain = decodeURIComponent(userinfo);
    } catch {
      plain = userinfo;
    }
    const decoded = decodeBase64(plain);
    return [userinfo, plain, ...plain.split(':'), ...(decoded.includes(':') ? decoded.split(':').slice(1) : [])];
  });
  return [...new Set([...fromNodes, ...fromLinks].filter((value) => typeof value === 'string' && value.length >= 4))];
}

async function coreProcessCount() {
  const { stdout } = await run('tasklist', ['/FI', 'IMAGENAME eq ferry-core.exe', '/NH']);
  return stdout.split('\n').filter((line) => line.toLowerCase().startsWith('ferry-core.exe')).length;
}

const goPage = (page, index) => page.key(String(index), { ctrl: true, code: `Digit${index}`, vk: 48 + index });
const bodyHas = (page, text, timeoutMs = 4000) =>
  page.waitFor(`document.body.innerText.includes(${JSON.stringify(text)})`, timeoutMs);
const tiles = (page) =>
  page.eval(`Object.fromEntries([...document.querySelectorAll('.stats .stat')].map((el) => [el.querySelector('.k').innerText.trim(), el.querySelector('.v').innerText.trim()]))`);

async function setMode(page, label) {
  await page.clickText(label, MODES);
  return page.waitFor(
    `[...document.querySelectorAll(${JSON.stringify(MODES)})].find((el) => el.innerText.trim() === ${JSON.stringify(label)})?.getAttribute('aria-checked') === 'true'`,
    6000,
  );
}

async function importSpec(page, specText) {
  await goPage(page, 7);
  await page.eval(`(() => { const el = document.querySelector('details.dev'); if (el && !el.open) el.querySelector('summary').click(); })()`);
  await page.eval(`(() => { const el = document.querySelector('#dev-spec'); el.focus(); el.select(); })()`);
  await page.send('Input.insertText', { text: specText });
  await sleep(200);
  await page.clickText('导入', 'details.dev button');
  const imported = await page.waitFor(`document.querySelector('details.dev').innerText.includes('配置已导入')`, 8000);
  const detail = imported ? '' : await page.text('details.dev [role="alert"]');
  check('开发配置导入成功（真实 Core.Configure）', imported, detail);
  check('导入后输入框已清空，不保留凭据', (await page.eval(`document.querySelector('#dev-spec').value`)) === '');
}

/** 节点页的当前选中项——导入后用它证明节点确实进了节点列表。 */
async function currentNodes(page) {
  await goPage(page, 2);
  return page.eval(
    `[...document.querySelectorAll('li.row .name .selectable')].map((el) => el.innerText.trim()).join('|')`,
  );
}

/** 在「订阅」页粘贴真实分享链接，走真实的 convert.ParseText + app.Subs。 */
async function importLinks(page, text) {
  await goPage(page, 3);
  await page.eval(`(() => { const el = document.querySelector('#sub-text'); if (!el) return false; el.focus(); el.select(); return true; })()`);
  await page.send('Input.insertText', { text });
  await sleep(200);
  await page.clickText('导入', '.import button');
  const status = await page.waitFor(`(document.querySelector('.import [role="status"]')?.innerText ?? '').includes('已导入')`, 10000);
  check('订阅页导入成功（真实 convert.ParseText + Subs.AddFromText）', status, await page.text('.import'));
  const notice = status ? await page.text('.import [role="status"]') : '';
  const skipped = await page.text('.import .skipped').catch(() => '');
  console.log(`      ${notice.split('\n')[0]}${skipped === '' ? '' : `；跳过：${skipped.replace(/\n/g, ' / ')}`}`);
  check('导入后输入框已清空，不保留链接', (await page.eval(`document.querySelector('#sub-text').value`)) === '');
  check('订阅列表出现该订阅', (await page.text('.list .row')).includes('手动导入'), await page.text('.list'));
  return { skipped };
}

async function startAndVerify(page, mixedPort, direct) {
  await goPage(page, 1);
  await page.click('button[aria-label="启动内核"]');
  const running = await bodyHas(page, '内核运行中', 20000);
  check('界面显示内核运行中', running, running ? '' : await page.text('.hero'));
  check('代理模式切到「全局」', await setMode(page, '全局'));
  const viaNode = await trace(mixedPort);
  check('全局模式：请求经节点发出', viaNode.ip !== '' && viaNode.ip !== direct.ip, JSON.stringify(viaNode));
  // 不打印 IP：节点出口与本机公网地址都不该留在终端记录里。
  console.log(`      出口地区 ${viaNode.loc || '未知'}，本机直连地区 ${direct.loc || '未知'}`);

  await sleep(2500);
  const stats = await tiles(page);
  check('流量统计来自真实内核', !['—', '0 B', ''].includes(stats['本次下载'] ?? ''), JSON.stringify(stats));

  check('代理模式切到「直连」', await setMode(page, '直连'));
  const viaDirect = await trace(mixedPort);
  check('直连模式：同一入站改为本机出口', viaDirect.ip === direct.ip, JSON.stringify(viaDirect));
  check('代理模式切回「规则」', await setMode(page, '规则'));
}

async function main() {
  if (SPEC_FILE === '' && LINK_FILE === '') throw new Error('请用 FERRY_E2E_RUNSPEC 或 FERRY_E2E_LINKS 指定仓库外的节点信息');
  const mixedPort = await freePort();
  const debugPort = await freePort();
  const source = SPEC_FILE === '' ? null : JSON.parse(await readFile(SPEC_FILE, 'utf8'));
  const links = LINK_FILE === '' ? '' : (await readFile(LINK_FILE, 'utf8')).trim();
  const spec =
    source === null
      ? {
          nodes: [],
          groups: [],
          rules: [],
          dns: { mode: 'redirhost', remote: 'https://1.1.1.1/dns-query', direct: 'https://223.5.5.5/dns-query', strategy: 'prefer_ipv4' },
          inbound: { mixedPort, allowLan: false, tun: { enabled: false, stack: 'mixed', strictRoute: true, excludes: [], mtu: 1500 } },
          mode: 'rule',
          log: { level: 'info' },
        }
      : { ...source, inbound: { ...source.inbound, mixedPort } };
  const secrets = secretsOf(spec.nodes, links);
  const coresBefore = await coreProcessCount();
  const direct = await trace(0);
  check('本机可以直连测试站点', direct.ip !== '', direct.error);

  const app = spawn(APP, [], {
    stdio: 'ignore',
    env: { ...process.env, FERRY_DEVTOOLS_PORT: String(debugPort) },
  });
  const exited = new Promise((done) => app.once('exit', done));
  try {
    const page = await attachPage(debugPort);
    const ready = await page.waitFor(`document.querySelector('button[aria-label="启动内核"]')?.disabled === false`, 15000);
    check('真实后端已连接，首屏为未启动', ready && (await bodyHas(page, '内核未启动')), await page.text('.hero'));
    const expectedTag = source === null ? '' : spec.nodes[0]?.tag ?? '';
    if (expectedTag !== '') check('未配置时没有节点', !(await page.eval(`document.body.innerText.includes(${JSON.stringify(expectedTag)})`)));

    if (LINK_FILE !== '') {
      // 先用不含节点的开发配置把入站换到空闲端口；文本导入只替换节点，入站设置保留。
      await importSpec(page, JSON.stringify({ ...spec, nodes: [], groups: [] }));
      const { skipped } = await importLinks(page, links);
      const nodes = await currentNodes(page);
      check('节点页出现导入的节点', nodes !== '', nodes);
      check('订阅导入的节点没有被跳过', skipped === '', skipped);
      console.log(`      节点数：${nodes.split('|').length}`);
    } else {
      await importSpec(page, JSON.stringify(spec));
      await goPage(page, 2);
      check('节点页显示导入的节点', await bodyHas(page, expectedTag));
    }
    await startAndVerify(page, mixedPort, direct);

    await page.click('button[aria-label="停止内核"]');
    check('界面显示内核已停止', await bodyHas(page, '内核未启动', 15000));
    check('停止后入站端口已释放', await portIsFree(mixedPort));
    check('停止后不再显示流量数字', (await tiles(page))['本次下载'] === '—');

    await page.click('button[aria-label="启动内核"]');
    check('可以再次启动', await bodyHas(page, '内核运行中', 20000));
    const leaked = await page.eval(`${JSON.stringify(secrets)}.some((secret) => document.documentElement.innerHTML.includes(secret))`);
    check('界面任何位置都不出现节点凭据', !leaked);
    // Wails 3.1.0 的运行时会探测可选的 /wails/custom.js，桌面模式下框架固定回 404（上游设计），只忽略这一条。
    const problems = page.problems.filter((item) => !item.includes('/wails/custom.js'));
    check('运行期间没有前端报错', problems.length === 0, problems.join('\n      '));

    // 关闭窗口 = 收进托盘：进程与内核都还在，代理照常工作（退出只能从托盘菜单点「退出 Ferry」）。
    await page.click('button[aria-label="关闭"]').catch(() => {});
    const exitedOnClose = await Promise.race([exited.then(() => true), sleep(4000).then(() => false)]);
    check('点击关闭后收进托盘，进程没有退出', !exitedOnClose);
    check('收进托盘后内核仍在运行', (await coreProcessCount()) === coresBefore + 1);
    const viaTray = await trace(mixedPort);
    check('收进托盘后入站仍然可用', viaTray.ip !== '', viaTray.error);

    // 托盘菜单无法由脚本点击：这里直接结束主进程，验证内核不会变成孤儿进程。
    app.kill();
    const closed = await Promise.race([exited.then(() => true), sleep(10000).then(() => false)]);
    check('主进程已结束', closed);
    await sleep(2500);
    check('主进程结束后内核子进程随之退出', (await coreProcessCount()) === coresBefore);
    check('主进程结束后入站端口已释放', await portIsFree(mixedPort));
  } finally {
    if (app.exitCode === null) app.kill();
  }
  if (!summary()) process.exitCode = 1;
}

await main();

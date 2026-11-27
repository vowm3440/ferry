/*
 * 真实联调（新功能）：在真实的 Ferry-devtools.exe 上走一遍规则、规则测试、测速、连接、日志与分享。
 * 与 real-app.mjs 一样，节点只从仓库外的文件读取：
 *   FERRY_E2E_LINKS   分享链接文本文件（必填）
 *   FERRY_E2E_APP     可执行文件路径，默认 ../../../bin/Ferry-devtools.exe
 *   FERRY_CONFIG_DIR  请指向一个空的临时目录，避免动到真实配置
 * 不碰系统代理、TUN、服务，也不弹出本机文件对话框（备份、诊断、日志导出不在这里测）。
 */
import { execFile, spawn } from 'node:child_process';
import { readFile } from 'node:fs/promises';
import net from 'node:net';
import { fileURLToPath } from 'node:url';
import { attachPage, createChecks, sleep } from '../smoke/harness.mjs';

const APP = process.env.FERRY_E2E_APP ?? fileURLToPath(new URL('../../../bin/Ferry-devtools.exe', import.meta.url));
const LINK_FILE = process.env.FERRY_E2E_LINKS ?? '';
const DIALOG = '[role="dialog"]';
const RULE_NAME = 'e2e 直连规则';
const RULE_HOST = 'e2e.example.invalid';
const { check, summary } = createChecks();
const q = (value) => JSON.stringify(value);

async function freePort() {
  const probe = net.createServer();
  await new Promise((done) => probe.listen(0, '127.0.0.1', done));
  const { port } = probe.address();
  await new Promise((done) => probe.close(done));
  return port;
}

const goPage = (page, index) => page.key(String(index), { ctrl: true, code: `Digit${index}`, vk: 48 + index });
const bodyHas = (page, text, timeoutMs = 5000) => page.waitFor(`document.body.innerText.includes(${q(text)})`, timeoutMs);

async function type(page, selector, text) {
  await page.eval(`(() => { const el = document.querySelector(${q(selector)}); el.focus(); el.select?.(); })()`);
  await page.send('Input.insertText', { text });
  await sleep(150);
}

async function prepare(page, mixedPort, links) {
  const spec = {
    nodes: [],
    groups: [],
    rules: [],
    dns: { mode: 'redirhost', remote: 'https://1.1.1.1/dns-query', direct: 'https://223.5.5.5/dns-query', strategy: 'prefer_ipv4' },
    inbound: { mixedPort, allowLan: false, tun: { enabled: false, stack: 'mixed', strictRoute: true, excludes: [], mtu: 1500 } },
    mode: 'rule',
    log: { level: 'info' },
  };
  await goPage(page, 7);
  await page.eval(`(() => { const el = document.querySelector('details.dev'); if (el && !el.open) el.querySelector('summary').click(); })()`);
  await type(page, '#dev-spec', JSON.stringify(spec));
  await page.clickText('导入', 'details.dev button');
  check('入站端口换成空闲端口', await page.waitFor(`document.querySelector('details.dev').innerText.includes('配置已导入')`, 8000));
  await goPage(page, 3);
  await type(page, '#sub-text', links);
  await page.clickText('导入', '.import button');
  check('节点导入成功', await page.waitFor(`(document.querySelector('.import [role="status"]')?.innerText ?? '').includes('已导入')`, 10000));
}

async function rules(page) {
  await goPage(page, 4);
  check('规则页列出两条固定规则', await page.waitFor(`document.querySelectorAll('tr[data-rule-id]').length === 2`, 6000));
  await page.clickText('新建规则', '.page-head button');
  await page.waitFor(`document.querySelector(${q(`${DIALOG} input[name="rule-name"]`)}) !== null`);
  await type(page, `${DIALOG} input[name="rule-name"]`, RULE_NAME);
  await type(page, `${DIALOG} textarea[name="rule-hosts"]`, RULE_HOST);
  await page.clickText('直连', `${DIALOG} [role="radiogroup"][aria-label="动作"] button`);
  await page.clickText('保存', `${DIALOG} .foot button`);
  const saved = await page.waitFor(`[...document.querySelectorAll('tr[data-rule-id]')].some((row) => row.innerText.includes(${q(RULE_NAME)}))`, 15000);
  check('新建规则经真实 Rules.Save 写入并出现在表格里', saved, saved ? '' : await page.text(DIALOG).catch(() => ''));

  await type(page, 'form[aria-label="规则测试"] input[name="test-host"]', RULE_HOST);
  await page.clickText('测试', 'form[aria-label="规则测试"] button');
  const hit = await page.waitFor(`(document.querySelector('form[aria-label="规则测试"] [role="status"]')?.innerText ?? '').includes(${q(RULE_NAME)})`, 10000);
  check('规则测试（真实 Rules.Test）命中刚建的规则', hit, await page.text('form[aria-label="规则测试"]'));
  await page.shot('e2e-rules');
}

async function running(page, mixedPort) {
  await goPage(page, 1);
  await page.click('button[aria-label="启动内核"]');
  check('内核启动（带着新规则）', await bodyHas(page, '内核运行中', 20000), await page.text('.hero').catch(() => ''));
  await page.shot('e2e-overview');

  await goPage(page, 2);
  await page.clickText('全部测速', 'button');
  const measured = await page.waitFor(`/\\d+\\s*ms/.test(document.querySelector('main')?.innerText ?? document.body.innerText)`, 25000);
  check('测速（真实 Core.LatencyTest）给出毫秒数', measured);
  await page.shot('e2e-nodes');

  // 一条限速的下载，保证连接页打开时它还活着。
  const curl = execFile('curl.exe', ['-s', '-o', 'NUL', '--limit-rate', '2k', '--max-time', '15', '-x', `http://127.0.0.1:${mixedPort}`, 'https://www.cloudflare.com/'], () => {});
  await goPage(page, 5);
  const rows = await page.waitFor(`document.querySelectorAll('table[aria-label="活动连接"] tbody tr').length > 0`, 10000);
  check('连接页显示真实连接', rows);
  if (rows) check('连接行带目标主机', (await page.text('table[aria-label="活动连接"] tbody')).includes('cloudflare'));
  await page.shot('e2e-conns');
  curl.kill();

  await goPage(page, 6);
  check('日志页收到真实内核日志', await page.waitFor(`(document.querySelector('[aria-label="日志内容"]')?.innerText ?? '').trim().length > 0`, 10000));
  await page.shot('e2e-logs');
}

async function share(page) {
  await goPage(page, 3);
  await page.click('button[aria-label^="分享订阅"]');
  const exported = await page.waitFor(`(document.querySelector('#share-text')?.value ?? '').length > 20`, 10000);
  check('分享对话框拿到真实导出内容（Subs.Export）', exported, await page.text(DIALOG).catch(() => ''));
  check('二维码由后端生成并以 <img> 显示', await page.waitFor(`document.querySelector('.share .qr img')?.src.startsWith('data:image/') === true`, 8000));
  check('局域网分享没有自动开启', (await page.eval(`document.querySelector('#lan-url') === null`)) === true);
  // 不截图：对话框里是真实节点凭据。
  await page.click(`${DIALOG} .head button[aria-label="关闭"]`);
  check('关闭后导出内容不再留在页面上', await page.waitFor(`document.querySelector('#share-text') === null`, 4000));
}

async function main() {
  if (LINK_FILE === '') throw new Error('请用 FERRY_E2E_LINKS 指定仓库外的分享链接文件');
  const links = (await readFile(LINK_FILE, 'utf8')).trim();
  const mixedPort = await freePort();
  const debugPort = await freePort();
  const app = spawn(APP, [], { stdio: 'ignore', env: { ...process.env, FERRY_DEVTOOLS_PORT: String(debugPort) } });
  try {
    const page = await attachPage(debugPort);
    check('真实后端已连接', await page.waitFor(`document.querySelector('button[aria-label="启动内核"]')?.disabled === false`, 15000));
    await prepare(page, mixedPort, links);
    await rules(page);
    await running(page, mixedPort);
    await share(page);
    await goPage(page, 1);
    check('全程之后内核仍在运行', await bodyHas(page, '内核运行中'));
    await page.click('button[aria-label="停止内核"]');
    check('内核可以停止', await bodyHas(page, '内核未启动', 15000));
    const problems = page.problems.filter((item) => !item.includes('/wails/custom.js'));
    check('运行期间没有前端报错（含内容安全策略违规）', problems.length === 0, problems.join('\n      '));
  } finally {
    app.kill();
  }
  if (!summary()) process.exitCode = 1;
}

await main();

// 场景 I：连接页与日志页（走假的桌面桥接）。
//   FERRY_SMOKE_DIST=<构建产物> [FERRY_SMOKE_SHOTS=<截图目录>] node tests/smoke/flows-inspect.mjs
import { CLIPPED, NO_OVERFLOW, createChecks, launchBrowser, sleep, startServer } from './harness.mjs';
import { createFakeBackend } from './fake-backend.mjs';
import { consoleReport, createTicker, dispatch, openBridgePage } from './bridge-kit.mjs';
import { appendLogs, connectionsEvent, fakeInspect, makeConnection } from './fake-inspect.mjs';

const { check, summary } = createChecks();
const NODES = [
  { id: 'a', tag: '节点甲', outbound: { type: 'socks', server: '127.0.0.1', server_port: 1080 } },
  { id: 'b', tag: '节点乙', outbound: { type: 'trojan', server: '127.0.0.2', server_port: 443 } },
];
const SIZES = [
  [1180, 780],
  [900, 640],
  [620, 700],
];
const MIB = 1024 * 1024;
const ROWS = `document.querySelectorAll('table[aria-label="活动连接"] tbody tr').length`;
const LOG_ROWS = `document.querySelectorAll('.log .row').length`;
const LOG_TEXT = `(document.querySelector('.log')?.innerText ?? '')`;
const H1 = `document.querySelector('main h1')?.innerText.trim()`;
const DIALOG = `(document.querySelector('.overlay .panel')?.innerText ?? '')`;
const FILTER_SEG = '[role="radiogroup"][aria-label="按出站方式筛选"] button';
const LEVEL_SEG = '[role="radiogroup"][aria-label="最低显示级别"] button';
const LEVEL_SELECT = 'select[aria-label="内核日志级别"]';

const goPage = (page, index) => page.key(String(index), { ctrl: true, code: `Digit${index}`, vk: 48 + index });
const toastHas = (page, message) => page.waitFor(`(document.querySelector('.toasts')?.innerText ?? '').includes(${JSON.stringify(message)})`);
const bodyHas = (page, text) => page.waitFor(`document.body.innerText.includes(${JSON.stringify(text)})`);
const setInput = (page, selector, value) =>
  page.eval(`(() => { const el = document.querySelector(${JSON.stringify(selector)}); el.value = ${JSON.stringify(value)}; el.dispatchEvent(new Event('input', { bubbles: true })); })()`);
const setSelect = (page, selector, value) =>
  page.eval(`(() => { const el = document.querySelector(${JSON.stringify(selector)}); el.value = ${JSON.stringify(value)}; el.dispatchEvent(new Event('change', { bubbles: true })); })()`);
const buttonState = (page, label) =>
  page.eval(`(() => { const el = [...document.querySelectorAll('.page-head button')].find((b) => b.innerText.replace(/\\s+/g, '') === ${JSON.stringify(label)}); return el ? (el.disabled ? 'disabled' : 'enabled') : 'missing'; })()`);

function helpers(fake) {
  return {
    lastCall: (method) => fake.s.log.findLast((item) => item.method === method),
    countCalls: (method) => fake.s.log.filter((item) => item.method === method).length,
    reject: (method, message) => fake.s.faults.set(method, { status: 422, body: { kind: 'RuntimeError', message } }),
  };
}

async function pushConns(page, fake) {
  const payload = connectionsEvent(fake.s);
  if (payload !== null) await dispatch(page, 'connections:snapshot', payload);
  await sleep(150);
}

async function pushLogs(page, fake, items) {
  const payload = appendLogs(fake.s, items);
  if (payload !== null) await dispatch(page, 'logs:entry', payload);
  await sleep(150);
}

async function layout(page, label, name) {
  for (const [width, height] of SIZES) {
    await page.viewport(width, height);
    await sleep(300);
    const clipped = await page.eval(CLIPPED);
    check(`${label} ${width}px 无溢出、无裁切`, (await page.eval(NO_OVERFLOW)) && clipped.length === 0, clipped.join(' | '));
    await page.shot(`${name}-${width}`);
  }
  await page.click('.foot-row .icon-btn');
  await sleep(300);
  await page.shot(`${name}-theme2-620`);
  await page.viewport(SIZES[0][0], SIZES[0][1]);
  await sleep(300);
  await page.shot(`${name}-theme2-1180`);
  await page.click('.foot-row .icon-btn');
  await sleep(200);
}

// 开始时间取同一个时刻再错开：列表按开始时间倒序，同毫秒内才按 id；
// 各自调用 Date.now() 时一旦跨毫秒，首行就会换人，断言随之忽好忽坏。
function seedConnections() {
  const now = Date.now();
  const at = (secondsAgo) => ({ startAt: now - secondsAgo * 1000 });
  return [
    makeConnection({ id: 'c1', up: 1000, down: 5000, ...at(65) }),
    makeConnection({ id: 'c2', process: 'Code.exe', processPath: 'D:\\Apps\\Code\\Code.exe', host: 'github.example.invalid', ruleName: '开发工具', outboundTag: '节点乙', startAt: Date.now() - 3_725_000 }),
    makeConnection({ id: 'c3', process: 'sync.exe', processPath: 'C:\\Sync\\sync.exe', host: '192.168.1.20', port: 445, ruleName: '本机 / 局域网', action: 'direct', outboundTag: 'direct', ...at(66) }),
    makeConnection({ id: 'c4', process: '', processPath: '', host: 'ads.example.invalid', port: 80, ruleName: '', action: 'block', outboundTag: 'block', ...at(67) }),
  ];
}

async function connsSection(page, fake, tick) {
  const { lastCall, countCalls, reject } = helpers(fake);
  console.log('\n== I1 连接页：按需加载与内核未运行 ==');
  const early = fake.s.calls.filter((name) => name.startsWith('Connections.') || name.startsWith('Logs.'));
  check('I1 没有打开页面时不调用连接或日志接口', early.length === 0, early.join(', '));
  await goPage(page, 5);
  check('I1 标题是「连接」', await page.waitFor(`${H1} === '连接'`));
  check('I1 内核未运行时显示空状态而不是 0', (await bodyHas(page, '内核未运行')) && (await page.eval(ROWS)) === 0);
  check('I1 进入页面先取消可能残留的暂停', JSON.stringify(lastCall('Connections.Pause')?.args) === '[false]');
  check('I1 未运行时「暂停」「关闭全部」不可用', (await buttonState(page, '暂停')) === 'disabled' && (await buttonState(page, '关闭全部')) === 'disabled');
  check('I1 后端没有规则功能时不显示「据此创建规则」', (await page.eval(`document.querySelector('button[title="据此创建规则"]') === null`)) === true);

  console.log('\n== I2 活动连接、速率与筛选 ==');
  fake.s.conns = seedConnections();
  await page.clickText('启动内核', '.page .empty button');
  await tick();
  check('I2 启动后列出全部连接', await page.waitFor(`${ROWS} === 4`), String(await page.eval(ROWS)));
  const first = await page.text('main');
  check('I2 显示应用、目标、规则与出站', ['4 个活动连接', 'chrome.exe', 'www.example.invalid:443', '默认规则', '节点甲', '开发工具', '192.168.1.20:445', '直连', '拦截', '未知应用'].every((item) => first.includes(item)), first.slice(0, 300));
  check('I2 第一次快照没有速率，显示「—」而不是 0', (await page.eval(`[...document.querySelectorAll('table[aria-label="活动连接"] td.r')].every((cell) => cell.innerText.trim() === '—')`)) === true);
  check('I2 完整路径放在 title 里', (await page.eval(`document.querySelector('table[aria-label="活动连接"] .app')?.getAttribute('title') ?? ''`)).includes('\\'));
  check('I2 超过一小时的时长带小时', first.includes('1:02:'), first.slice(0, 300));

  await sleep(1100);
  fake.s.conns = fake.s.conns.map((conn) => (conn.id === 'c1' ? { ...conn, up: conn.up + 64 * 1024, down: conn.down + 3 * MIB } : conn));
  await pushConns(page, fake);
  const c1 = await page.eval(`[...document.querySelectorAll('table[aria-label="活动连接"] tbody tr')].find((row) => row.innerText.includes('chrome.exe'))?.innerText.replace(/\\s+/g, ' ')`);
  check('I2 第二次快照按累计字节差算出速率', /MB\/s/.test(String(c1)) && /KB\/s/.test(String(c1)), String(c1));
  const sum = await page.text('.summary');
  check('I2 汇总行显示合计速率', /MB\/s/.test(sum) && sum.includes('4 个活动连接'), sum);
  await page.shot('i-conns-running');

  await setInput(page, 'input[aria-label="过滤连接"]', 'GITHUB');
  check('I2 文字筛选不分大小写', await page.waitFor(`${ROWS} === 1`), String(await page.eval(ROWS)));
  await setInput(page, 'input[aria-label="过滤连接"]', '');
  await page.clickText('直连', FILTER_SEG);
  check('I2 按出站方式筛选', (await page.waitFor(`${ROWS} === 1`)) && (await page.text('table[aria-label="活动连接"] tbody')).includes('sync.exe'));
  await setInput(page, 'input[aria-label="过滤连接"]', 'chrome');
  check('I2 没有匹配时如实说明', await bodyHas(page, '没有匹配的连接'));
  await page.clickText('清除筛选', '.page .empty button');
  check('I2 清除筛选后恢复全部', await page.waitFor(`${ROWS} === 4`));

  console.log('\n== I3 暂停、关闭与上限 ==');
  await page.clickText('暂停', '.page-head button');
  check('I3 暂停走 Connections.Pause(true)', JSON.stringify(lastCall('Connections.Pause')?.args) === '[true]');
  check('I3 按钮变成「继续」并标明已暂停', (await buttonState(page, '继续')) === 'enabled' && (await bodyHas(page, '已暂停刷新')));
  await dispatch(page, 'connections:snapshot', { items: [], added: [], updated: [], removed: [] });
  await sleep(200);
  check('I3 暂停期间即使收到事件也不改列表', (await page.eval(ROWS)) === 4);

  const closeBefore = countCalls('Connections.Snapshot');
  await page.click('button[aria-label^="关闭连接：chrome.exe"]');
  check('I3 关闭走 Connections.Close(id)', JSON.stringify(lastCall('Connections.Close')?.args) === '["c1"]', JSON.stringify(lastCall('Connections.Close')?.args));
  check('I3 暂停时关闭后主动重读，行随之消失', (await page.waitFor(`${ROWS} === 3`)) && countCalls('Connections.Snapshot') > closeBefore);
  reject('Connections.Close', '请选择连接');
  await page.click('button[aria-label^="关闭连接：Code.exe"]');
  check('I3 关闭失败时提示后端原文', (await toastHas(page, '请选择连接')) && (await page.eval(ROWS)) === 3);

  await page.clickText('继续', '.page-head button');
  check('I3 继续走 Connections.Pause(false)', JSON.stringify(lastCall('Connections.Pause')?.args) === '[false]');
  check('I3 继续后速率重新取样', await page.waitFor(`[...document.querySelectorAll('table[aria-label="活动连接"] td.r')].every((cell) => cell.innerText.trim() === '—')`));

  await page.clickText('关闭全部', '.page-head button');
  check('I3 关闭全部先确认', (await page.waitFor(`${DIALOG}.includes('3 个活动连接')`)) && countCalls('Connections.CloseAll') === 0, await page.eval(DIALOG));
  await page.clickText('取消', '.overlay button');
  check('I3 取消后什么都不做', (await page.waitFor(`document.querySelector('.overlay') === null`)) && countCalls('Connections.CloseAll') === 0);
  await page.clickText('关闭全部', '.page-head button');
  await page.waitFor(`document.querySelector('.overlay .panel') !== null`);
  await sleep(250);
  await page.clickText('关闭全部', '.overlay button');
  check('I3 确认后走 Connections.CloseAll()', (await toastHas(page, '已关闭全部连接')) && countCalls('Connections.CloseAll') === 1);
  check('I3 没有连接时显示空状态', await bodyHas(page, '当前没有活动连接'));

  fake.s.conns = Array.from({ length: 320 }, (_, index) => makeConnection({ id: `m${index}`, host: `h${index}.example.invalid`, startAt: Date.now() - index * 1000 }));
  await pushConns(page, fake);
  check('I3 最多渲染 300 行并如实说明', (await page.waitFor(`${ROWS} === 300`)) && (await bodyHas(page, '仅显示前 300 条，另有 20 条未显示')), String(await page.eval(ROWS)));
  fake.s.conns = seedConnections();
  await pushConns(page, fake);
  await layout(page, 'I3 连接页', 'i-conns');

  await page.clickText('暂停', '.page-head button');
  await goPage(page, 1);
  await sleep(300);
  check('I3 暂停着离开页面时恢复后端推送', JSON.stringify(lastCall('Connections.Pause')?.args) === '[false]' && fake.s.connsPaused === false);
  const snapshots = countCalls('Connections.Snapshot');
  await pushConns(page, fake);
  check('I3 离开页面后不再读取连接', countCalls('Connections.Snapshot') === snapshots);
}

async function logsSection(page, fake) {
  const { lastCall, countCalls, reject } = helpers(fake);
  console.log('\n== I4 日志：快照、事件与筛选 ==');
  appendLogs(fake.s, [
    ['debug', 'dns: exchanged www.example.invalid A'],
    ['info', 'inbound/mixed[mixed-in]: inbound connection to www.example.invalid:443'],
    ['warn', 'outbound/proxy: connection upload closed: i/o timeout'],
    ['error', 'outbound/proxy: dial tcp: connection refused'],
  ]);
  await goPage(page, 6);
  check('I4 标题是「日志」', await page.waitFor(`${H1} === '日志'`));
  check('I4 进入页面读取最近 2000 条', (await page.waitFor(`${LOG_ROWS} === 4`)) && JSON.stringify(lastCall('Logs.Snapshot')?.args) === '[2000]', JSON.stringify(lastCall('Logs.Snapshot')?.args));
  const rows = await page.eval(LOG_TEXT);
  check('I4 显示级别与原文', ['DEBUG', 'INFO', 'WARN', 'ERROR', 'connection refused'].every((item) => rows.includes(item)), rows.slice(0, 200));
  check('I4 日志行是 90px 56px 1fr 的网格且可选中', (await page.eval(`(() => { const row = document.querySelector('.log .row'); const cols = getComputedStyle(row).gridTemplateColumns.split(' '); return cols[0] === '90px' && cols[1] === '56px' && getComputedStyle(row).userSelect === 'text'; })()`)) === true);

  await pushLogs(page, fake, [['info', 'router: found process path']]);
  check('I4 事件追加到末尾', await page.waitFor(`${LOG_ROWS} === 5`));
  await dispatch(page, 'logs:entry', fake.s.logEntries.slice(-3));
  await sleep(200);
  check('I4 重复的序号不会重复显示', (await page.eval(LOG_ROWS)) === 5);

  await page.clickText('Warn', LEVEL_SEG);
  check('I4 最低级别在本地筛选', (await page.waitFor(`${LOG_ROWS} === 2`)) && (await bodyHas(page, '匹配 2 / 5 条')));
  await page.clickText('Debug', LEVEL_SEG);
  await setInput(page, 'input[aria-label="搜索日志"]', 'CONNECTION');
  check('I4 搜索不分大小写', await page.waitFor(`${LOG_ROWS} === 3`), String(await page.eval(LOG_ROWS)));
  await setInput(page, 'input[aria-label="搜索日志"]', '没有这样的日志');
  check('I4 没有匹配时如实说明', await bodyHas(page, '没有匹配的日志'));
  await setInput(page, 'input[aria-label="搜索日志"]', '');
  check('I4 筛选没有再次请求后端', countCalls('Logs.Snapshot') === 1);

  console.log('\n== I5 暂停、清空与自动滚动 ==');
  await page.clickText('暂停', '.page-head button');
  check('I5 暂停走 Logs.Pause(true)', JSON.stringify(lastCall('Logs.Pause')?.args) === '[true]' && (await bodyHas(page, '已暂停刷新')));
  await pushLogs(page, fake, [['info', 'paused-gap-1'], ['info', 'paused-gap-2']]);
  check('I5 暂停期间不出现新日志', (await page.eval(LOG_ROWS)) === 5);
  await page.clickText('继续', '.page-head button');
  check('I5 继续后重读快照补上空档', (await page.waitFor(`${LOG_ROWS} === 7`)) && countCalls('Logs.Snapshot') === 2 && (await page.eval(LOG_TEXT)).includes('paused-gap-2'));

  await page.clickText('清空显示', '.page-head button');
  check('I5 清空显示只清界面', (await bodyHas(page, '暂无日志')) && fake.s.logEntries.length === 7 && fake.s.log.every((item) => !/Clear/.test(item.method)));
  await page.clickText('暂停', '.page-head button');
  await page.clickText('继续', '.page-head button');
  check('I5 清空之后重读快照也不把旧日志带回来', countCalls('Logs.Snapshot') === 3 && (await page.eval(LOG_ROWS)) === 0);
  await pushLogs(page, fake, Array.from({ length: 80 }, (_, index) => ['info', `line-${index}`]));
  check('I5 清空之后只显示新日志', await page.waitFor(`${LOG_ROWS} === 80`), String(await page.eval(LOG_ROWS)));
  const AT_BOTTOM = `(() => { const el = document.querySelector('.log'); return el.scrollHeight > el.clientHeight && el.scrollHeight - el.scrollTop - el.clientHeight <= 24; })()`;
  check('I5 停在底部时跟随新日志', await page.waitFor(AT_BOTTOM));
  await page.eval(`(() => { const el = document.querySelector('.log'); el.scrollTop = 0; el.dispatchEvent(new Event('scroll')); })()`);
  await sleep(150);
  await pushLogs(page, fake, [['info', 'while-reading']]);
  check('I5 往上翻看时不被新日志拉回底部', (await page.eval(`document.querySelector('.log').scrollTop`)) === 0 && (await bodyHas(page, '回到最新')));
  await page.clickText('回到最新', '.wrap button');
  check('I5 「回到最新」回到底部', await page.waitFor(AT_BOTTOM));
  await page.shot('i-logs');

  console.log('\n== I6 内核日志级别与导出 ==');
  check('I6 下拉框显示设置里的级别', (await page.eval(`document.querySelector(${JSON.stringify(LEVEL_SELECT)}).value`)) === 'info');
  const gets = countCalls('Settings.Get');
  await setSelect(page, LEVEL_SELECT, 'debug');
  check('I6 修改走 Logs.SetLevel(level)', (await page.waitFor(`document.querySelector(${JSON.stringify(LEVEL_SELECT)}).disabled === false`)) && JSON.stringify(lastCall('Logs.SetLevel')?.args) === '["debug"]');
  check('I6 修改后重新读取设置', countCalls('Settings.Get') > gets && (await page.eval(`document.querySelector(${JSON.stringify(LEVEL_SELECT)}).value`)) === 'debug');
  reject('Logs.SetLevel', '内核正在启动，请稍后重试');
  await setSelect(page, LEVEL_SELECT, 'error');
  check('I6 修改失败时原样显示后端的说明', await bodyHas(page, '内核正在启动，请稍后重试'));
  check('I6 失败后下拉框回到实际的级别', await page.waitFor(`document.querySelector(${JSON.stringify(LEVEL_SELECT)}).value === 'debug'`));
  await page.clickText('知道了', '.inline-error button');

  await page.clickText('导出', '.page-head button');
  const confirmText = await page.eval(DIALOG);
  check('I6 导出前先说明是明文导出', confirmText.includes('明文') && confirmText.includes('域名') && confirmText.includes('IP'), confirmText);
  check('I6 确认之前不弹保存框、不导出', fake.s.dialogs.length === 0 && countCalls('Logs.Export') === 0);
  await sleep(250);
  await page.clickText('选择位置并导出', '.overlay button');
  check('I6 在保存框里取消就什么都不写', fake.s.dialogs.length === 1 && fake.s.dialogs[0].method === 5 && countCalls('Logs.Export') === 0);
  check('I6 默认文件名带时间戳', /ferry-log-\d{8}-\d{6}\.log/.test(JSON.stringify(fake.s.dialogs[0].args)), JSON.stringify(fake.s.dialogs[0].args));
  fake.s.dialogReplies.push('C:\\Temp\\ferry.log');
  await page.clickText('选择位置并导出', '.overlay button');
  check('I6 选好位置后走 Logs.Export(path)', (await toastHas(page, '日志已导出')) && JSON.stringify(lastCall('Logs.Export')?.args) === '["C:\\\\Temp\\\\ferry.log"]', JSON.stringify(lastCall('Logs.Export')?.args));
  check('I6 导出成功后确认框关闭', await page.waitFor(`document.querySelector('.overlay') === null`));
  await page.clickText('导出', '.page-head button');
  await sleep(250);
  fake.s.dialogReplies.push('C:\\Temp\\ferry.log');
  await page.clickText('选择位置并导出', '.overlay button');
  check('I6 目标已存在时原样显示后端的说明', await page.waitFor(`${DIALOG}.includes('无法创建日志文件，请选择尚不存在的文件')`), await page.eval(DIALOG));
  check('I6 没有覆盖已有文件', fake.s.exports.length === 1);
  await page.shot('i-logs-export-error');
  await page.clickText('取消', '.overlay button');
  await layout(page, 'I6 日志页', 'i-logs');

  fake.s.status = 'stopped';
  await dispatch(page, 'core:state', fake.state());
  await sleep(400);
  check('I6 内核停止后日志仍可查看', (await page.eval(LOG_ROWS)) > 0, String(await page.eval(LOG_ROWS)));
}

/** 后端公布了规则功能时：「据此创建规则」→ 预填 → 跳到规则页。规则区的其它接口不在这里验证。 */
async function ruleSection(browser, server) {
  console.log('\n== I7 据此创建规则 ==');
  const draft = () => ({ id: '', name: 'chrome.exe → www.example.invalid', enabled: true, apps: ['chrome.exe'], hosts: ['www.example.invalid'], ports: [], network: 'any', action: 'proxy', targetId: 'proxy', resolveBefore: true });
  const fakeRules = ({ s, Rejected }) => ({
    'Rules.CreateFromConnection': (id) => {
      if (!s.conns.some((conn) => conn.id === id)) throw new Rejected('连接已结束或不存在');
      return draft();
    },
  });
  const fake = createFakeBackend({ capabilities: ['subs.text', 'connections', 'logs', 'rules'], extra: [fakeInspect, fakeRules] });
  const { lastCall } = helpers(fake);
  fake.s.nodes = NODES;
  fake.s.status = 'running';
  fake.s.startedAt = Date.now();
  fake.s.conns = seedConnections();
  server.use(fake.handle);
  const page = await openBridgePage(browser, server);
  await page.waitFor(`document.querySelector('main h1') !== null`);
  await goPage(page, 5);
  check('I7 有规则功能时每行都有「据此创建规则」', await page.waitFor(`document.querySelectorAll('button[title="据此创建规则"]').length === 4`));
  fake.s.conns = fake.s.conns.filter((conn) => conn.id !== 'c2');
  await page.click('button[aria-label^="据此创建规则：Code.exe"]');
  check('I7 连接已结束时提示后端原文并留在连接页', (await toastHas(page, '连接已结束或不存在')) && (await page.eval(H1)) === '连接');
  await page.click('button[aria-label^="据此创建规则：chrome.exe"]');
  check('I7 走 Rules.CreateFromConnection(id)', JSON.stringify(lastCall('Rules.CreateFromConnection')?.args) === '["c1"]', JSON.stringify(lastCall('Rules.CreateFromConnection')?.args));
  check('I7 预填后跳到规则页', await page.waitFor(`${H1} === '规则'`), String(await page.eval(H1)));
  await page.shot('i-rule-draft');
  page.close();
}

let handler = null;
const server = await startServer((req, res) => (handler === null ? false : handler(req, res)));
server.use = (next) => {
  handler = next;
};
const browser = await launchBrowser();
let passed = false;
try {
  const fake = createFakeBackend({ capabilities: ['subs.text', 'connections', 'logs'], extra: [fakeInspect] });
  fake.s.nodes = NODES;
  server.use(fake.handle);
  const page = await openBridgePage(browser, server);
  const tick = createTicker(page, fake);
  await page.waitFor(`document.body.innerText.includes('内核未启动')`);
  await tick();

  await connsSection(page, fake, tick);
  await logsSection(page, fake);

  const allowed = ['Core.', 'Nodes.', 'Settings.', 'Subs.', 'System.Capabilities', 'Connections.', 'Logs.'];
  const foreign = [...new Set(fake.s.log.map((item) => item.method))].filter((name) => !allowed.some((prefix) => name.startsWith(prefix)));
  check('I 没有规则等能力时不调用对应接口', foreign.length === 0, foreign.join(', '));
  const report = consoleReport(page, fake);
  check('I 没有控制台错误或警告（后端拒绝的调用除外）', report.others.length === 0, report.others.join('\n      '));
  check('I 失败应答次数与控制台记录一致', report.rejected === report.expected, `记录 ${report.rejected} 条，实际返回 4xx/5xx ${report.expected} 次`);
  page.close();

  await ruleSection(browser, server);
  passed = summary();
} catch (error) {
  console.error('冒烟脚本自身出错：', error);
} finally {
  await browser.close();
  await server.close();
}
process.exit(passed ? 0 : 1);

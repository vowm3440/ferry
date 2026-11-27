// 场景 S：订阅页、分享对话框与节点测速（走假的桌面桥接）。
//   FERRY_SMOKE_DIST=<构建产物> [FERRY_SMOKE_SHOTS=<截图目录>] node tests/smoke/flows-subs.mjs
import { CLIPPED, NO_OVERFLOW, createChecks, launchBrowser, sleep, startServer } from './harness.mjs';
import { createFakeBackend } from './fake-backend.mjs';
import { consoleReport, createTicker, dispatch, openBridgePage } from './bridge-kit.mjs';
import { fakeSubs } from './fake-subs.mjs';

const { check, summary } = createChecks();
const CAPS = ['subs.text', 'subs.url', 'share', 'share.lan', 'qr', 'core.latency', 'settings.persist'];
const SIZES = [
  [1280, 800],
  [900, 640],
  [620, 700],
];
const SUB_URL = 'https://sub.example.invalid/api?token=synthetic-token';
const FILE_PATH = 'C:\\smoke\\nodes.txt';
const H1 = `document.querySelector('main h1')?.innerText.trim()`;
const CARDS = `document.querySelectorAll('.list > li.row').length`;
const SHARE_TEXT = `(document.querySelector('#share-text')?.value ?? null)`;
const FORMAT_SEG = '[role="radiogroup"][aria-label="导出格式"] button';
const SORT_SEG = '[role="radiogroup"][aria-label="排序方式"] button';
const LAN = 'section[aria-label="局域网分享"]';
const ROW_NAMES = `[...document.querySelectorAll('.rows li.row .name .selectable')].map((item) => item.innerText.trim())`;
/** 浏览器自带的文件选择框没法在无头模式里操作：改成直接交出 window.__nextFile（没有就当作取消）。 */
const FILE_INPUT_STUB = `(() => { const original = HTMLInputElement.prototype.click; HTMLInputElement.prototype.click = function () { if (this.type !== 'file') return original.call(this); const next = window.__nextFile; window.__nextFile = undefined; if (!next) { this.dispatchEvent(new Event('cancel')); return; } const data = new DataTransfer(); data.items.add(new File([next.text], next.name, { type: next.type })); this.files = data.files; this.dispatchEvent(new Event('change')); }; })()`;

const q = JSON.stringify;
const goPage = (page, index) => page.key(String(index), { ctrl: true, code: `Digit${index}`, vk: 48 + index });
const toastHas = (page, message) => page.waitFor(`(document.querySelector('.toasts')?.innerText ?? '').includes(${q(message)})`);
const has = (page, selector, text) => page.waitFor(`(document.querySelector(${q(selector)})?.innerText ?? '').includes(${q(text)})`);
const gone = (page, selector) => page.waitFor(`document.querySelector(${q(selector)}) === null`);
const setInput = (page, selector, value) =>
  page.eval(`(() => { const el = document.querySelector(${q(selector)}); el.value = ${q(value)}; el.dispatchEvent(new Event('input', { bubbles: true })); })()`);
const tile = (page, title) =>
  page.eval(`(() => { const el = [...document.querySelectorAll('.tiles button.imp')].find((item) => item.querySelector('b')?.innerText.trim() === ${q(title)}); if (!el) return false; el.click(); return true; })()`);
const label = (name) => `button[aria-label=${q(name)}]`;
const latOf = (page, name) =>
  page.eval(`[...document.querySelectorAll('.rows li.row')].find((row) => row.querySelector('.name .selectable')?.innerText.trim() === ${q(name)})?.querySelector('.lat')?.innerText.trim()`);

async function layout(page, name, title) {
  for (const [width, height] of SIZES) {
    await page.viewport(width, height);
    await sleep(350);
    const clipped = await page.eval(CLIPPED);
    check(`${title} ${width}px 无溢出、无裁切`, (await page.eval(NO_OVERFLOW)) && clipped.length === 0, clipped.join(' | '));
    await page.shot(`${name}-${width}`);
  }
  await page.click('.foot-row .icon-btn');
  await sleep(300);
  await page.shot(`${name}-theme2-620`);
  await page.viewport(SIZES[0][0], SIZES[0][1]);
  await sleep(300);
  await page.shot(`${name}-theme2-1280`);
  await page.click('.foot-row .icon-btn');
  await sleep(200);
}

async function importSection(page, fake, { lastCall, countCalls }) {
  console.log('\n== S1 订阅页与导入方式 ==');
  await goPage(page, 3);
  check('S1 标题是「订阅」', await page.waitFor(`${H1} === '订阅'`));
  check('S1 副标题列出支持的格式', await has(page, '.page-head', '支持 Clash / sing-box / Base64 / 分享链接 / SIP008'));
  const tiles = await page.eval(`[...document.querySelectorAll('.tiles button.imp b')].map((item) => item.innerText.trim()).join(',')`);
  check('S1 四种导入方式', tiles === '订阅链接,粘贴内容,二维码图片,本地文件', tiles);
  check('S1 没有订阅时显示空状态', await has(page, 'main', '还没有订阅'));
  const early = fake.s.calls.filter((name) => name.startsWith('Subs.LanShare') || name === 'Core.LatencyTest');
  check('S1 进入页面不调用分享与测速接口', early.length === 0, early.join(', '));

  console.log('\n== S2 订阅链接 ==');
  await tile(page, '订阅链接');
  check('S2 打开订阅链接对话框', await page.waitFor(`document.querySelector('#sub-url') !== null`));
  await page.clickText('添加', '[role="dialog"] .foot button');
  check('S2 没填地址时不提交', (await has(page, '[role="dialog"] [role="alert"]', '请填写订阅地址')) && countCalls('Subs.AddFromURL') === 0);
  await setInput(page, '#sub-url', 'ftp://sub.example.invalid/a');
  await page.clickText('添加', '[role="dialog"] .foot button');
  check('S2 非 HTTP(S) 地址在提交前拦下', (await has(page, '[role="dialog"] [role="alert"]', '订阅地址须为有效的 HTTP 或 HTTPS 地址')) && countCalls('Subs.AddFromURL') === 0);
  await setInput(page, '#sub-url', 'http://plain.example.invalid/sub');
  check('S2 HTTP 地址给出明文警告', await has(page, '[role="dialog"] .inline-note.warn', '此订阅使用未加密的 HTTP'));
  await setInput(page, '#sub-url', 'https://down.example.invalid/sub');
  await page.clickText('添加', '[role="dialog"] .foot button');
  check('S2 下载失败时显示后端原文', await has(page, '[role="dialog"] [role="alert"]', '订阅下载失败，请检查网络或地址'));
  check('S2 失败后保留已填的地址', (await page.eval(`document.querySelector('#sub-url')?.value`)) === 'https://down.example.invalid/sub');
  await setInput(page, '#sub-url', SUB_URL);
  await setInput(page, '#sub-url-name', '机场甲');
  await page.clickText('添加', '[role="dialog"] .foot button');
  check('S2 导入成功后对话框关闭', await gone(page, '#sub-url'));
  check('S2 走 Subs.AddFromURL(名称, 地址, 自动更新)', q(lastCall('Subs.AddFromURL')?.args) === q(['机场甲', SUB_URL, true]), q(lastCall('Subs.AddFromURL')?.args));
  check('S2 页面说明导入结果', await has(page, '.inline-note.outcome', '已导入「机场甲」：3 个节点'));
  const card = await page.text('.list > li.row');
  check('S2 卡片显示打码地址、流量与自动更新', ['机场甲', 'https://sub.example.invalid/****', '3 个节点', '已用', '每 24 小时'].every((item) => card.includes(item)), card);
  check('S2 页面上没有订阅地址明文', !(await page.text('body')).includes('synthetic-token'));
  check('S2 有总量时画出用量条', await page.eval(`document.querySelector('.list .bar i') !== null`));
  await tile(page, '订阅链接');
  await page.waitFor(`document.querySelector('#sub-url') !== null`);
  check('S2 再次打开时输入框已清空', (await page.eval(`document.querySelector('#sub-url').value + '|' + document.querySelector('#sub-url-name').value`)) === '|');
  await page.clickText('取消', '[role="dialog"] .foot button');
  await gone(page, '#sub-url');

  console.log('\n== S3 二维码图片与本地文件 ==');
  await page.eval(`window.__nextFile = { name: 'qr.png', type: 'image/png', text: 'https://sub.example.invalid/x' }`);
  await tile(page, '二维码图片');
  check('S3 二维码里是订阅地址时显示后端的拒绝原因', await has(page, '.inline-error.outcome', '二维码包含订阅地址，请在 URL 导入入口确认来源后导入'));
  const qrText = 'socks5://qr1.example.invalid:1080#qr-node';
  await page.eval(`window.__nextFile = { name: 'qr.png', type: 'image/png', text: ${q(qrText)} }`);
  await tile(page, '二维码图片');
  check('S3 二维码导入成功', await has(page, '.inline-note.outcome', '已导入「二维码导入」：1 个节点'));
  check('S3 图片以 Base64 交给 Subs.ImportFromQR', lastCall('Subs.ImportFromQR')?.args[0] === Buffer.from(qrText).toString('base64'));
  const qrCalls = countCalls('Subs.ImportFromQR');
  await tile(page, '二维码图片');
  await sleep(300);
  check('S3 取消选图不调用后端', countCalls('Subs.ImportFromQR') === qrCalls);

  fake.s.files.set(FILE_PATH, 'socks5://file1.example.invalid:1080#file-node');
  fake.s.dialogReplies.push(FILE_PATH);
  await tile(page, '本地文件');
  check('S3 本地文件只把路径交给后端', (await has(page, '.inline-note.outcome', '已导入「nodes.txt」：1 个节点')) && q(lastCall('Subs.AddFromFile')?.args) === q([FILE_PATH]));
  fake.s.dialogReplies.push('C:\\smoke\\missing.txt');
  await tile(page, '本地文件');
  check('S3 文件读不到时显示后端原文', await has(page, '.inline-error.outcome', '无法读取节点文件'));
  const fileCalls = countCalls('Subs.AddFromFile');
  await tile(page, '本地文件');
  await sleep(300);
  check('S3 取消选择文件不调用后端', countCalls('Subs.AddFromFile') === fileCalls && (await page.eval(CARDS)) === 3);
}

async function manageSection(page, fake, { lastCall, countCalls }) {
  console.log('\n== S4 更新、进度与设置 ==');
  const id = fake.s.subs[0].view.id;
  await page.click(label('更新订阅 机场甲'));
  check('S4 更新走 Subs.Update(id)', (await toastHas(page, '已更新订阅「机场甲」')) && q(lastCall('Subs.Update')?.args) === q([id]));
  fake.s.updateFailures.add(id);
  await page.click(label('更新订阅 机场甲'));
  check('S4 更新失败提示后端原文', await toastHas(page, '已保留上次可用配置'));
  const failed = await page.text('.list > li.row');
  check('S4 卡片标出更新失败与原因', failed.includes('更新失败') && failed.includes('订阅包含不支持或无效的节点'), failed);
  fake.s.updateFailures.add(id);
  await page.clickText('全部更新', '.page-head button');
  check('S4 全部更新失败时列出是哪个订阅', await has(page, '.inline-error.outcome', '机场甲：订阅包含不支持或无效的节点'));
  await page.clickText('全部更新', '.page-head button');
  check('S4 全部更新成功', (await toastHas(page, '全部订阅已更新')) && !(await page.text('.list > li.row')).includes('更新失败'));
  check('S4 手动导入的订阅没有更新按钮', await page.eval(`document.querySelector(${q(label('更新订阅 二维码导入'))}) === null`));

  const lists = countCalls('Subs.List');
  await dispatch(page, 'subs:progress', { id, phase: 'fetch' });
  check('S4 后台更新的进度：下载', await has(page, '.list .progress[role="status"]', '正在下载'));
  await dispatch(page, 'subs:progress', { id, phase: 'parse' });
  check('S4 后台更新的进度：解析', await has(page, '.list .progress[role="status"]', '正在解析'));
  await page.shot('s-subs-progress');
  await dispatch(page, 'subs:progress', { id, phase: 'done' });
  check('S4 完成后进度消失并重新读取列表', (await gone(page, '.list .progress')) && countCalls('Subs.List') > lists);

  await page.click(label('设置订阅 机场甲'));
  check('S4 设置对话框带出当前名称', await page.waitFor(`document.querySelector('#sub-edit-name')?.value === '机场甲'`));
  await setInput(page, '#sub-edit-name', '机场乙');
  await setInput(page, '#sub-edit-hours', '0');
  await page.clickText('保存', '[role="dialog"] .foot button');
  check('S4 间隔越界时不提交', (await has(page, '[role="dialog"] [role="alert"]', '1 到 720 小时')) && countCalls('Subs.SetAutoUpdate') === 0);
  await setInput(page, '#sub-edit-hours', '12');
  await page.clickText('保存', '[role="dialog"] .foot button');
  check('S4 保存后对话框关闭', await gone(page, '#sub-edit-name'));
  check('S4 走 Subs.Rename 与 Subs.SetAutoUpdate', q(lastCall('Subs.Rename')?.args) === q([id, '机场乙']) && q(lastCall('Subs.SetAutoUpdate')?.args) === q([id, true, 12]));
  const renamed = await page.text('.list > li.row');
  check('S4 卡片显示新名称与新间隔', renamed.includes('机场乙') && renamed.includes('每 12 小时'), renamed);
  await page.click(label('设置订阅 二维码导入'));
  check('S4 手动导入的订阅说明不能自动更新', await has(page, '[role="dialog"]', '不能自动更新'));
  await page.clickText('取消', '[role="dialog"] .foot button');
  await gone(page, '#sub-edit-name');
  await layout(page, 's-subs', 'S4 订阅页');
}

async function shareSection(page, fake, { lastCall, countCalls, reject }) {
  console.log('\n== S5 分享对话框 ==');
  const id = fake.s.subs[0].view.id;
  await page.click(label('分享订阅 机场乙'));
  check('S5 打开分享对话框', await has(page, '[role="dialog"] .head', '分享「机场乙」'));
  check('S5 默认导出 Clash', (await page.waitFor(`${SHARE_TEXT}?.startsWith('proxies:')`)) && q(lastCall('Subs.Export')?.args) === q([id, 'clash']));
  check('S5 打开时才读取局域网分享状态', countCalls('Subs.LanShareStatus') === 1, String(countCalls('Subs.LanShareStatus')));
  check('S5 二维码用 <img> 显示后端给的 SVG', await page.waitFor(`(() => { const img = document.querySelector('.share .out .qr img'); return img !== null && img.src.startsWith('data:image/svg+xml;charset=utf-8,') && img.naturalWidth > 0; })()`));
  await page.clickText('链接', FORMAT_SEG);
  check('S5 链接格式', await page.waitFor(`${SHARE_TEXT}?.includes('socks5://u1-1.example.invalid:1081')`));
  await page.clickText('Base64', FORMAT_SEG);
  check('S5 Base64 格式', await page.waitFor(`/^[A-Za-z0-9+/=]+$/.test(${SHARE_TEXT} ?? '')`));
  reject('Subs.QRSvg', '内容超过二维码容量，请使用文件分享');
  await page.clickText('sing-box', FORMAT_SEG);
  check('S5 sing-box 格式', await page.waitFor(`${SHARE_TEXT}?.includes('"outbounds"')`));
  check('S5 放不进二维码时如实说明', await has(page, '.share .out .qr', '内容超过二维码容量'));
  check('S5 预览框只读', (await page.eval(`document.querySelector('#share-text').readOnly`)) === true);
  check('S5 没点复制之前不写剪贴板', fake.s.clipboard === '');
  await page.clickText('复制', '.share .side button');
  check('S5 复制写入剪贴板', (await toastHas(page, '已复制到剪贴板')) && fake.s.clipboard === (await page.eval(SHARE_TEXT)));

  check('S5 局域网分享先给出警告，不自动开启', (await has(page, LAN, '局域网里任何拿到链接的人都能读取一次全部节点')) && countCalls('Subs.LanShareStart') === 0);
  await page.clickText('开启', `${LAN} button`);
  check('S5 开启后显示链接', await has(page, '#lan-url', 'http://192.0.2.10:18080/share/'));
  const countdown = await page.text('#lan-countdown');
  check('S5 显示十分钟倒计时', /^(10:00|09:5\d)$/.test(countdown), countdown);
  await page.clickText('复制', `${LAN} button`);
  check('S5 复制局域网链接', (await toastHas(page, '已复制局域网分享链接')) && fake.s.clipboard === fake.s.lanShare.url);
  await page.clickText('二维码', `${LAN} button`);
  check('S5 局域网链接的二维码', await page.waitFor(`document.querySelector(${q(`${LAN} .qr img`)}) !== null`));
  await sleep(350);
  for (const [width, height] of SIZES) {
    await page.viewport(width, height);
    await sleep(350);
    const clipped = await page.eval(CLIPPED);
    check(`S5 分享对话框 ${width}px 无溢出、无裁切`, (await page.eval(NO_OVERFLOW)) && clipped.length === 0, clipped.join(' | '));
    await page.shot(`s-share-${width}`);
  }
  await page.viewport(SIZES[0][0], SIZES[0][1]);
  // 模拟「被读取一次」：后端那边已经结束，界面靠轮询发现。
  fake.s.lanShare = { url: '', expiresAt: 0, active: false };
  check('S5 分享被读取或到期后自动收起链接', await page.waitFor(`document.querySelector('#lan-url') === null`, 9000));
  await page.clickText('开启', `${LAN} button`);
  await page.waitFor(`document.querySelector('#lan-url') !== null`);
  await page.clickText('停止', `${LAN} button`);
  check('S5 停止分享', (await toastHas(page, '已停止局域网分享')) && (await gone(page, '#lan-url')) && fake.s.lanShare.active === false);

  const exports = countCalls('Subs.Export');
  await page.clickText('开启', `${LAN} button`);
  await page.waitFor(`document.querySelector('#lan-url') !== null`);
  const stops = countCalls('Subs.LanShareStop');
  await page.click('[role="dialog"] .head button[aria-label="关闭"]');
  check('S5 关闭后预览内容不再留在页面上', await gone(page, '#share-text'));
  check('S5 分享开着时关闭对话框会停止分享并告知', (await toastHas(page, '分享对话框已关闭，局域网分享已停止')) && countCalls('Subs.LanShareStop') === stops + 1 && fake.s.lanShare.active === false);
  const polls = countCalls('Subs.LanShareStatus');
  await sleep(6500);
  check('S5 对话框关闭后不再轮询分享状态（不再取回带令牌的链接）', countCalls('Subs.LanShareStatus') === polls, String(countCalls('Subs.LanShareStatus') - polls));
  await page.click(label('分享订阅 机场乙'));
  check('S5 再次打开时重新向后端导出', (await page.waitFor(`${SHARE_TEXT}?.startsWith('proxies:')`)) && countCalls('Subs.Export') === exports + 1);
  await page.click('.foot-row .icon-btn');
  await sleep(350);
  await page.shot('s-share-theme2');
  await page.click('.foot-row .icon-btn');
  await page.click('[role="dialog"] .head button[aria-label="关闭"]');
  await gone(page, '#share-text');

  console.log('\n== S6 删除订阅 ==');
  const removes = countCalls('Subs.Remove');
  await page.click(label('删除订阅 nodes.txt'));
  check('S6 删除前先确认', (await has(page, '[role="dialog"]', '删除「nodes.txt」')) && countCalls('Subs.Remove') === removes);
  await page.clickText('取消', '[role="dialog"] .foot button');
  await sleep(300);
  check('S6 取消后订阅还在', (await page.eval(CARDS)) === 3 && countCalls('Subs.Remove') === removes);
  await page.click(label('删除订阅 nodes.txt'));
  await page.clickText('删除', '[role="dialog"] .foot button');
  check('S6 确认后删除', (await toastHas(page, '已删除订阅「nodes.txt」')) && (await page.waitFor(`${CARDS} === 2`)));
}

async function nodesSection(page, fake, tick, { lastCall, countCalls, reject }) {
  console.log('\n== S7 节点页与测速 ==');
  await goPage(page, 2);
  check('S7 标题是「节点」', await page.waitFor(`${H1} === '节点'`));
  const sorts = await page.eval(`[...document.querySelectorAll(${q(SORT_SEG)})].map((item) => item.innerText.trim()).join(',')`);
  check('S7 排序是分段选择：默认 / 延迟 / 名称', sorts === '默认,延迟,名称', sorts);
  check('S7 未测速时如实显示', (await latOf(page, '合成节点 1')) === '未测速');
  await page.clickText('全部测速', '.page-head button');
  check('S7 内核未运行时不测速并说明原因', (await toastHas(page, '请先启动内核再测速')) && countCalls('Core.LatencyTest') === 0);

  await page.clickText('启动内核', '.inline-note button');
  await tick();
  await page.waitFor(`document.querySelector('.inline-note .grow')?.innerText.includes('内核未运行') !== true`);
  await dispatch(page, 'nodes:latency', { 'u1-1': { latencyMs: -1, status: 'testing' } });
  check('S7 测速事件：进行中', (await latOf(page, '合成节点 1')) === '测速中…', await latOf(page, '合成节点 1'));
  await dispatch(page, 'nodes:latency', { 'u1-1': { latencyMs: 123, status: 'ok' } });
  check('S7 测速事件：结果', (await latOf(page, '合成节点 1')) === '123 ms', await latOf(page, '合成节点 1'));

  fake.s.latencyPlan = new Map([['u1-1', 300], ['u1-2', -1], ['u1-3', 45]]);
  await page.clickText('全部测速', '.page-head button');
  await page.waitFor(`[...document.querySelectorAll('.page-head button')].some((item) => item.innerText.trim() === '全部测速')`);
  const ids = lastCall('Core.LatencyTest')?.args[0] ?? [];
  check('S7 全部测速把所有节点交给 Core.LatencyTest', q([...ids].sort()) === q(fake.s.nodes.map((node) => node.id).sort()), q(ids));
  const lats = [await latOf(page, '合成节点 1'), await latOf(page, '合成节点 2'), await latOf(page, '合成节点 3')];
  check('S7 结果在刷新后仍然保留，超时如实显示', q(lats) === q(['300 ms', '超时', '45 ms']), q(lats));
  await page.clickText('延迟', SORT_SEG);
  const sorted = await page.eval(ROW_NAMES);
  check('S7 按延迟排序：快的在前，超时在后', sorted[0] === '合成节点 3' && sorted.at(-1) === '合成节点 2', q(sorted));
  await page.clickText('名称', SORT_SEG);
  const named = await page.eval(ROW_NAMES);
  check('S7 按名称排序', q(named) === q(['合成节点 1', '合成节点 2', '合成节点 3', 'qr-node']), q(named));
  await page.clickText('默认', SORT_SEG);

  const before = countCalls('Core.LatencyTest');
  await page.click(label('测速分组 代理'));
  check('S7 分组测速只测这个分组的节点', countCalls('Core.LatencyTest') === before + 1 && lastCall('Core.LatencyTest').args[0].length === fake.s.nodes.length);
  await sleep(300);
  reject('Core.LatencyTest', '当前内核不支持此功能');
  await page.clickText('全部测速', '.page-head button');
  check('S7 测速失败提示后端原文', await toastHas(page, '当前内核不支持此功能'));

  await setInput(page, 'input[aria-label="搜索节点"]', 'qr-node');
  check('S7 搜索仍然可用', await page.waitFor(`${ROW_NAMES}.join(',') === 'qr-node'`));
  await setInput(page, 'input[aria-label="搜索节点"]', '');
  await page.click(label('分享节点 合成节点 1'));
  check('S7 节点可以单独分享', (await has(page, '[role="dialog"] .head', '分享「合成节点 1」')) && (await page.waitFor(`${SHARE_TEXT} === 'socks5://u1-1.example.invalid:1081#' + encodeURIComponent('合成节点 1')`)));
  check('S7 走 Subs.ExportNode(节点, 链接)', q(lastCall('Subs.ExportNode')?.args) === q(['u1-1', 'uri']));
  await page.shot('s-share-node');
  await page.click('[role="dialog"] .head button[aria-label="关闭"]');
  await gone(page, '#share-text');
  await layout(page, 's-nodes', 'S7 节点页');
}

const server = await startServer((req, res) => fake.handle(req, res));
const fake = createFakeBackend({ capabilities: CAPS, extra: [fakeSubs] });
const browser = await launchBrowser();
let passed = false;
try {
  const page = await openBridgePage(browser, server, { width: SIZES[0][0], height: SIZES[0][1] });
  const tick = createTicker(page, fake);
  await page.waitFor(`document.body.innerText.includes('内核未启动')`);
  await tick();
  await page.eval(FILE_INPUT_STUB);
  const tools = {
    lastCall: (method) => fake.s.log.findLast((item) => item.method === method),
    countCalls: (method) => fake.s.log.filter((item) => item.method === method).length,
    reject: (method, message) => fake.s.faults.set(method, { status: 422, body: { kind: 'RuntimeError', message } }),
  };

  await importSection(page, fake, tools);
  await manageSection(page, fake, tools);
  await shareSection(page, fake, tools);
  await nodesSection(page, fake, tick, tools);

  const banned = ['Subs.CaptureScreenQR', 'Subs.ShareDeepLink', 'System.InstallService', 'System.SetTun', 'Rules.', 'Connections.', 'Logs.'];
  const foreign = [...new Set(fake.s.log.map((item) => item.method))].filter((name) => banned.some((prefix) => name.startsWith(prefix)));
  check('S 不调用没有实现或没有能力的接口', foreign.length === 0, foreign.join(', '));
  const report = consoleReport(page, fake);
  check('S 没有控制台错误或警告（后端拒绝的调用除外）', report.others.length === 0, report.others.join('\n      '));
  check('S 失败应答次数与控制台记录一致', report.rejected === report.expected, `记录 ${report.rejected} 条，实际返回 4xx/5xx ${report.expected} 次`);
  page.close();
  passed = summary();
} catch (error) {
  console.error('冒烟脚本自身出错：', error);
} finally {
  await browser.close();
  await server.close();
}
process.exit(passed ? 0 : 1);

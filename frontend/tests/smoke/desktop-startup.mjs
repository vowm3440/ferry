// 场景 C（第一部分）：伪桌面桥接 + 假 /wails/runtime 后端，走前端真实适配层
// （lib/api/wails.ts + @wailsio/runtime 3.1.0），检查 P0 的调用、事件与错误文案。
import { createChecks, launchBrowser, sleep, startServer } from './harness.mjs';
import { createFakeBackend } from './fake-backend.mjs';
import { activeMode, consoleReport, createTicker, openBridgePage } from './bridge-kit.mjs';

const { check, summary } = createChecks();
const fake = createFakeBackend();
const MIB = 1024 * 1024;
const totalText = (bytes) => `${(bytes / MIB).toFixed(bytes < 10 * MIB ? 2 : 1)}MB`;

const server = await startServer(fake.handle);
const browser = await launchBrowser();
let passed = false;
try {
  const page = await openBridgePage(browser, server);
  const tick = createTicker(page, fake);

  console.log('\n== C 假桌面桥接（一） ==');
  check('C1 概览显示「内核未启动」', await page.waitFor(`document.body.innerText.includes('内核未启动')`));
  check('C1 不带演示模式标识', await page.eval(`!document.querySelector('.demo-banner') && !document.body.innerText.includes('未连接桌面后端')`));
  check('C1 标题栏出现窗口按钮', await page.eval(`document.querySelector('.titlebar .caption .close') !== null`));
  console.log('      初始调用：', fake.s.calls.join(', '));
  check(
    'C1 读取了内核状态、节点与设置',
    ['Core.Status', 'Nodes.List', 'Settings.Get'].every((name) => fake.s.calls.includes(name)),
    fake.s.calls.join(', '),
  );

  await tick();
  await tick();
  await sleep(200);
  const stopped = await page.text('main');
  check('C1 停止时的全 0 快照不会显示成真实数据', stopped.includes('内核未运行，暂无流量') && !stopped.includes('0 B/s'), stopped.slice(0, 200));
  await page.shot('c-stopped');

  await page.click('button[aria-label="启动内核"]');
  check('C2 没有节点时给出后端原文', await page.waitFor(`document.querySelector('.hero .inline-error') !== null`));
  const startError = await page.text('.hero .inline-error');
  check('C2 错误文案与后端一致', startError === '请先在「订阅」页导入节点', JSON.stringify(startError));
  check('C2 失败后仍显示未启动', await page.eval(`document.body.innerText.includes('内核未启动')`));
  await page.click('button[aria-label="关闭提示"]');
  check('C2 提示可关闭', await page.eval(`document.querySelector('.hero .inline-error') === null`));

  await page.key('7', { ctrl: true, code: 'Digit7', vk: 55 });
  check('C3 打开设置页', await page.waitFor(`document.getElementById('dev-spec') !== null`));
  await page.click('details.dev summary');
  await page.clickText('填入示例');
  await page.clickText('导入');
  check('C3 导入后给出结果说明', await page.waitFor(`document.querySelector('details.dev [role="status"]') !== null`));
  const notice = await page.text('details.dev [role="status"]');
  check('C3 结果说明与后端数据一致', notice === '配置已导入：1 个节点、1 个分组。内核尚未启动，可在概览页启动。', notice);
  check('C3 导入成功后清空输入框', (await page.eval(`document.getElementById('dev-spec').value`)) === '');
  const configure = fake.s.log.find((item) => item.method === 'Core.Configure');
  const spec = JSON.parse(configure?.args?.[0] ?? 'null');
  check('C3 提交的是 RunSpec 文本', spec?.nodes?.[0]?.tag === '本机测试', JSON.stringify(spec?.nodes?.[0] ?? null));
  await page.shot('c-settings-import');

  await page.key('2', { ctrl: true, code: 'Digit2', vk: 50 });
  await sleep(300);
  const nodesText = await page.text('main');
  check('C4 节点页显示导入的节点', ['本机测试', 'SOCKS', '127.0.0.1:1080'].every((item) => nodesText.includes(item)), nodesText.slice(0, 160));
  check('C4 未测速时如实显示', nodesText.includes('未测速') && nodesText.includes('内核未运行'));
  await page.shot('c-nodes');

  await page.key('1', { ctrl: true, code: 'Digit1', vk: 49 });
  await page.click('button[aria-label="启动内核"]');
  check('C5 启动成功', await page.waitFor(`document.body.innerText.includes('内核运行中')`));
  check('C5 按钮变为停止内核', await page.eval(`document.querySelector('button[aria-label="停止内核"]') !== null`));
  const started = fake.s.log.findLast((item) => item.method === 'Core.Start')?.args ?? null;
  check('C5 启动没有多余参数', Array.isArray(started) && started.length === 0, JSON.stringify(started));

  await tick();
  await tick();
  await tick();
  await sleep(250);
  const ticks = fake.s.ticks;
  const stats = await page.text('.stats');
  check(
    `C5 本次下载按事件更新（第 ${ticks} 次快照）`,
    stats.includes(totalText(1_572_864 * ticks)),
    `${stats} 期望包含 ${totalText(1_572_864 * ticks)}`,
  );

  await tick({ wrapTraffic: true });
  await sleep(250);
  check('C5 兼容数组包裹的事件数据', fake.s.ticks > ticks);
  console.log('      运行中正文：', (await page.text('main')).slice(0, 240));

  fake.s.mode = 'global';
  await tick();
  check('C5 后端事件推动模式变化', (await page.eval(activeMode)) === '全局', await page.eval(activeMode));
  await page.clickText('规则', '[role="radiogroup"][aria-label="代理模式"] button');
  const setMode = fake.s.log.findLast((item) => item.method === 'Core.SetMode');
  check('C5 点击模式走 Core.SetMode', setMode?.args?.[0] === 'rule' && fake.s.mode === 'rule', JSON.stringify(setMode?.args ?? null));
  check('C5 模式切换后界面跟随后端', await page.waitFor(`${activeMode} === '规则'`));
  await page.shot('c-running');

  const report = consoleReport(page, fake);
  check('C 没有控制台错误或警告（后端拒绝的调用除外）', report.others.length === 0, report.others.join('\n      '));
  check('C 后端拒绝次数与控制台记录一致', report.rejected === report.expected, `记录 ${report.rejected} 条，实际返回 4xx ${report.expected} 次`);
  console.log('      其他对象调用：', fake.s.calls.filter((item) => item.startsWith('object')).join(', ') || '（无）');
  console.log('      方法调用顺序：', fake.s.calls.filter((item) => !item.startsWith('object')).join(' → '));
  page.close();
  passed = summary();
} catch (error) {
  console.error('冒烟脚本自身出错：', error);
} finally {
  await browser.close();
  await server.close();
}
process.exit(passed ? 0 : 1);

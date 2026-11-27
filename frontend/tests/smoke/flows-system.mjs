// 场景 S（系统）：概览页的系统代理卡片与订阅用量、设置页的保存说明 / 未开放项 / 备份与恢复 / 诊断。
// 用法与其它冒烟脚本相同：FERRY_SMOKE_DIST 指向构建产物，FERRY_SMOKE_SHOTS 指向截图目录。
import { CLIPPED, NO_OVERFLOW, createChecks, launchBrowser, sleep, startServer } from './harness.mjs';
import { createFakeBackend } from './fake-backend.mjs';
import { fakeSystem } from './fake-system.mjs';
import { consoleReport, createTicker, openBridgePage } from './bridge-kit.mjs';

const { check, summary } = createChecks();
const SIZES = [[1280, 800], [900, 640], [620, 700]];
const GIB = 1024 ** 3;
const BACKUP_PATH = 'C:\\Users\\me\\Documents\\ferry.ferrybak';
const DIAG_PATH = 'C:\\Users\\me\\Documents\\ferry-diag.json';
const FORBIDDEN = ['System.InstallService', 'System.UninstallService', 'System.RepairService', 'System.SetTun'];
const STALE = /计划于|P[0-5] 阶段|尚未开放/;

const q = (value) => JSON.stringify(value);
const TILE = `document.querySelector('[data-tile="sysproxy"]')`;
const tileText = (page) => page.eval(`${TILE}?.innerText.replace(/\\s+/g, ' ').trim()`);
const tileSwitch = (page) => page.eval(`(() => { const el = ${TILE}?.querySelector('[role="switch"]'); return el ? { checked: el.getAttribute('aria-checked'), disabled: el.disabled } : null; })()`);
const rowError = (page, action) => page.eval(`document.querySelector('[data-action=${q(action)}]')?.closest('.srow')?.querySelector('.e')?.innerText.trim() ?? ''`);
const dialogText = (page) => page.eval(`document.querySelector('[role="dialog"]')?.innerText ?? ''`);
const goPage = (page, index) => page.key(String(index), { ctrl: true, code: `Digit${index}`, vk: 48 + index });
const toastHas = (page, message) => page.waitFor(`(document.querySelector('.toasts')?.innerText ?? '').includes(${q(message)})`);

function subView(id, name, patch) {
  return { id, name, urlMasked: 'https://sub.example/…', nodeCount: 1, groupCount: 0, usedBytes: 0, totalBytes: 0, expiresAt: 0, lastUpdatedAt: Date.now(), intervalHours: 24, autoUpdate: true, userAgent: '', status: 'ok', ...patch };
}

function seed(fake) {
  fake.s.nodes = [
    { id: 'a', tag: '节点甲', outbound: { type: 'socks', server: '127.0.0.1', server_port: 1080 } },
    { id: 'b', tag: '节点乙', outbound: { type: 'trojan', server: '127.0.0.2', server_port: 443 } },
  ];
  fake.s.subs = [
    { view: subView('s-1', '机场甲', { usedBytes: 42 * GIB, totalBytes: 100 * GIB, expiresAt: Date.now() + 30 * 86_400_000 }), nodeIds: ['a'] },
    { view: subView('s-2', '手动导入', { urlMasked: '', autoUpdate: false, intervalHours: 0 }), nodeIds: ['b'] },
  ];
}

/** anchor：截图前滚动到的元素 id（设置页的分区）。 */
async function layout(page, name, title, anchor = '') {
  const settle = async () => {
    if (anchor !== '') await page.eval(`document.getElementById(${q(anchor)})?.scrollIntoView()`);
    await sleep(350);
  };
  for (const [width, height] of SIZES) {
    await page.viewport(width, height);
    await settle();
    const clipped = await page.eval(CLIPPED);
    check(`${title} ${width}px 无溢出、无裁切`, (await page.eval(NO_OVERFLOW)) && clipped.length === 0, clipped.join(' | '));
    await page.shot(`${name}-${width}`);
  }
  await page.click('.foot-row .icon-btn');
  await settle();
  await page.shot(`${name}-theme2-620`);
  await page.viewport(SIZES[0][0], SIZES[0][1]);
  await settle();
  await page.shot(`${name}-theme2-1280`);
  await page.click('.foot-row .icon-btn');
  await sleep(200);
}

async function overviewSection(page, fake, tick) {
  const count = (method) => fake.s.log.filter((item) => item.method === method).length;
  const last = (method) => fake.s.log.findLast((item) => item.method === method);

  console.log('\n== S1 概览（内核未启动） ==');
  check('S1 标题是「概览」', (await page.text('h1')) === '概览');
  check('S1 能力读到后读取了系统代理状态', await page.waitFor(`${TILE}?.innerText.includes('启动内核后可开启')`), await tileText(page));
  check('S1 SysProxyStatus 已调用', count('System.SysProxyStatus') >= 1);
  const stopped = await tileSwitch(page);
  check('S1 内核未启动时开关不可用且为关', stopped?.disabled === true && stopped?.checked === 'false', q(stopped));
  const tun = await page.eval(`(() => { const el = document.querySelector('[data-tile="tun"]'); return { text: el?.innerText ?? '', controls: el?.querySelectorAll('button, [role="switch"]').length }; })()`);
  check('S1 TUN 卡片写明未开放，没有任何开关', tun.text.includes('未开放') && tun.controls === 0, q(tun));
  const main = await page.text('main');
  check('S1 概览没有过期的阶段文案', !STALE.test(main) && !main.includes('应用流量'), main.match(STALE)?.[0] ?? '');
  const usage = await page.eval(`[...document.querySelectorAll('[data-sub]')].map((row) => ({ id: row.dataset.sub, text: row.innerText.replace(/\\s+/g, ' '), bar: row.querySelector('.bar') !== null, width: row.querySelector('.bar > i')?.style.width ?? '' }))`);
  check('S1 有总量的订阅显示用量、百分比和进度条', usage[0]?.bar === true && usage[0].width === '42%' && usage[0].text.includes('42.0 GB / 100 GB') && usage[0].text.includes('还有'), q(usage[0]));
  check('S1 没有用量信息的订阅显示「—」、不画进度条', usage[1]?.bar === false && usage[1].text.includes('—') && !usage[1].text.includes('%'), q(usage[1]));

  console.log('\n== S2 启动内核后切换系统代理 ==');
  await page.click('button.power');
  await tick();
  check('S2 内核启动后开关可用', await page.waitFor(`${TILE}?.querySelector('[role="switch"]')?.disabled === false`), await tileText(page));
  check('S2 未开启时显示本机地址', /127.0.0.1:7890 · 未开启/.test(await tileText(page)), await tileText(page));
  await page.click('[data-tile="sysproxy"] [role="switch"]');
  check('S2 开启走 SetSysProxy(true)', q(last('System.SetSysProxy')?.args) === '[true]', q(last('System.SetSysProxy')?.args));
  check('S2 卡片变为开启状态', await page.waitFor(`${TILE}?.classList.contains('on') && ${TILE}.querySelector('[role="switch"]').getAttribute('aria-checked') === 'true'`), await tileText(page));
  check('S2 提示已开启', await toastHas(page, '系统代理已开启'));
  await page.shot('s-overview-proxy-on');

  fake.s.faults.set('System.SetSysProxy', { status: 422, body: { kind: 'RuntimeError', message: '无法写入系统代理设置' } });
  await page.click('[data-tile="sysproxy"] [role="switch"]');
  check('S2 切换失败时提示后端原文', await toastHas(page, '无法写入系统代理设置'));
  check('S2 失败后以后端的实际状态为准（仍为开启）', (await tileSwitch(page))?.checked === 'true', q(await tileSwitch(page)));
  check('S2 系统代理的错误不占用内核错误的位置', (await page.eval(`document.querySelectorAll('.hero .inline-error').length`)) === 0);

  await page.click('button.power');
  await tick();
  check('S2 停止内核后重新读取，系统代理显示为关闭', await page.waitFor(`${TILE}?.innerText.includes('启动内核后可开启') && !${TILE}.classList.contains('on')`), await tileText(page));
  await layout(page, 's-overview', 'S2 概览');
}

async function settingsSection(page, fake) {
  const count = (method) => fake.s.log.filter((item) => item.method === method).length;
  const last = (method) => fake.s.log.findLast((item) => item.method === method);

  console.log('\n== S3 设置：说明与未开放项 ==');
  await goPage(page, 7);
  await page.waitFor(`document.querySelector('[data-action="backup-export"]') !== null`);
  check('S3 标题是「设置」', (await page.text('h1')) === '设置');
  const note = await page.text('[data-note="storage"]');
  check('S3 有持久化能力时说明加密保存在本机', note.includes('加密保存在本机'), note);
  const main = await page.text('main');
  check('S3 设置页没有过期的阶段文案', !STALE.test(main), main.match(STALE)?.[0] ?? '');
  check('S3 写明语言、在线更新、TUN 与服务的真实状态', ['支持简体中文与 English', '未配置生产更新地址与签名公钥', '不能安装服务，也不能启用 TUN'].every((item) => main.includes(item)));
  await page.click('[aria-label="语言"] [role="radio"]:nth-child(2)');
  await sleep(120);
  check('S3 切换为 English 后界面标题即时变为英文', (await page.text('h1')) === 'Settings', await page.text('h1'));
  await page.click('[aria-label="Language"] [role="radio"]:nth-child(1)');
  await sleep(120);
  check('S3 切回简体中文后界面标题恢复中文', (await page.text('h1')) === '设置', await page.text('h1'));
  const locked = await page.eval(`['开机自启', '启动时最小化', '轻量模式', '启动时设置系统代理', '规则集自动更新'].map((name) => { const el = document.querySelector('[role="switch"][aria-label="' + name + '（未开放）"]'); return el ? el.disabled && el.getAttribute('aria-checked') === 'false' : null; })`);
  check('S3 五个未开放的自动化开关都不可操作', locked.every((item) => item === true), q(locked));

  console.log('\n== S4 备份、恢复与诊断 ==');
  await page.click('[data-action="backup-export"]');
  await sleep(300);
  check('S4 取消文件选择框时不调用 ExportBackup', count('System.ExportBackup') === 0 && fake.s.dialogs.length === 1 && fake.s.dialogs[0].method === 5);
  check('S4 默认文件名带时间戳', /ferry-backup-\d{8}-\d{6}\.ferrybak/.test(q(fake.s.dialogs[0].args)), q(fake.s.dialogs[0].args).slice(0, 200));

  fake.s.dialogReplies.push(BACKUP_PATH);
  await page.click('[data-action="backup-export"]');
  check('S4 导出走 ExportBackup(path)', (await toastHas(page, '备份已导出')) && q(last('System.ExportBackup')?.args) === q([BACKUP_PATH]), q(last('System.ExportBackup')?.args));
  fake.s.dialogReplies.push(BACKUP_PATH);
  await page.click('[data-action="backup-export"]');
  await page.waitFor(`document.querySelector('[data-action="backup-export"]').closest('.srow').querySelector('.e') !== null`);
  check('S4 目标已存在时显示后端原文', (await rowError(page, 'backup-export')) === '无法创建备份，请选择同一磁盘上尚不存在的文件', await rowError(page, 'backup-export'));

  // 备份之后改动后端状态，恢复应当把它们换回去。
  fake.s.nodes = fake.s.nodes.slice(0, 1);
  fake.s.settings = { ...fake.s.settings, mixedPort: 9000 };
  fake.s.dialogReplies.push(BACKUP_PATH);
  await page.click('[data-action="backup-restore"]');
  await page.waitFor(`document.querySelector('[role="dialog"]') !== null`);
  await sleep(350);
  const confirm = await dialogText(page);
  check('S4 恢复前确认：说明会替换的内容并显示所选文件', ['全部节点与分组', '全部订阅', '全部规则', '无法撤销', BACKUP_PATH].every((item) => confirm.includes(item)), confirm);
  check('S4 危险操作的默认焦点在「取消」', (await page.eval(`document.activeElement?.innerText.trim()`)) === '取消');
  await page.shot('s-restore-confirm');
  await page.clickText('取消', '[role="dialog"] button');
  check('S4 取消确认时不调用 RestoreBackup', count('System.RestoreBackup') === 0);

  fake.s.dialogReplies.push(BACKUP_PATH);
  await page.click('[data-action="backup-restore"]');
  await page.waitFor(`document.querySelector('[role="dialog"]') !== null`);
  const before = fake.s.log.length;
  await page.clickText('替换并恢复', '[role="dialog"] button');
  check('S4 恢复走 RestoreBackup(path) 并提示完成', (await toastHas(page, '已从备份恢复')) && q(last('System.RestoreBackup')?.args) === q([BACKUP_PATH]));
  const after = fake.s.log.slice(before).map((item) => item.method);
  const reread = ['Core.Status', 'Settings.Get', 'Nodes.List', 'Subs.List', 'System.SysProxyStatus'].filter((name) => !after.includes(name));
  check('S4 恢复后重新读取内核、设置、节点、订阅与系统代理', reread.length === 0, `未读取：${reread.join(', ')}`);
  check('S4 界面显示恢复后的设置', await page.waitFor(`document.querySelector('input[aria-label="混合端口"]')?.value === '7890'`));

  fake.s.dialogReplies.push('C:\\Users\\me\\missing.ferrybak');
  await page.click('[data-action="backup-restore"]');
  await page.waitFor(`document.querySelector('[role="dialog"]') !== null`);
  await page.clickText('替换并恢复', '[role="dialog"] button');
  await page.waitFor(`document.querySelector('[role="dialog"]') === null`);
  check('S4 恢复失败时显示后端原文', (await rowError(page, 'backup-restore')) === '备份为空或不存在', await rowError(page, 'backup-restore'));

  await page.click('[data-action="diagnostics-export"]');
  await page.waitFor(`document.querySelector('[role="dialog"]') !== null`);
  await sleep(350);
  const about = await dialogText(page);
  check('S4 导出诊断前说明内容', ['未加密的 JSON', '数量', '不包含节点、服务器地址、订阅链接、密码、日志和文件路径'].every((item) => about.includes(item)), about);
  await page.shot('s-diagnostics-confirm');
  fake.s.dialogReplies.push(DIAG_PATH);
  await page.clickText('选择保存位置', '[role="dialog"] button');
  check('S4 诊断走 ExportDiagnostics(path)', (await toastHas(page, '诊断包已导出')) && q(last('System.ExportDiagnostics')?.args) === q([DIAG_PATH]));
  check('S4 诊断文件名带时间戳', /ferry-diagnostics-\d{8}-\d{6}\.json/.test(q(fake.s.dialogs.at(-1).args)));
  await layout(page, 's-settings-backup', 'S4 设置（备份与诊断）', 'set-subs');

  console.log('\n== S5 订阅默认值可以修改并保存 ==');
  await page.eval(`(() => { const el = document.querySelector('input[aria-label="订阅 User-Agent"]'); el.focus(); el.select(); })()`);
  await page.send('Input.insertText', { text: 'ferry-smoke/1.0' });
  await page.eval(`(() => { const el = document.querySelector('select[aria-label="订阅自动更新间隔"]'); el.value = '12'; el.dispatchEvent(new Event('change', { bubbles: true })); })()`);
  await page.waitFor(`document.querySelector('.savebar') !== null`);
  await page.clickText('保存', '.savebar button');
  check('S5 保存成功', await toastHas(page, '设置已保存'));
  const saved = last('Settings.Set')?.args?.[0];
  check('S5 提交了新的 User-Agent 与间隔', saved?.subscriptionUA === 'ferry-smoke/1.0' && saved?.subscriptionUpdateHours === 12, q(saved));
  check('S5 未开放的自动化开关没有以 true 提交', [saved?.autoStart, saved?.startMinimized, saved?.lightweight, saved?.sysProxyOnStart, saved?.ruleSetAutoUpdate].every((item) => item === false));
  await page.viewport(1280, 800);
  await page.eval(`document.getElementById('set-general')?.scrollIntoView()`);
  await sleep(300);
  await page.shot('s-settings-top');
}

/** 后端只有最基础能力时：不读取系统代理，不提供备份与诊断，并如实说明。 */
async function bareSection(browser) {
  console.log('\n== S6 后端没有这些能力 ==');
  const fake = createFakeBackend({ extra: [fakeSystem] });
  seed(fake);
  const server = await startServer(fake.handle);
  try {
    const page = await openBridgePage(browser, server, { width: 1280, height: 800 });
    await page.waitFor(`${TILE}?.innerText.includes('当前版本不能自动设置')`);
    const state = await tileSwitch(page);
    check('S6 没有 system.proxy 时开关不可用，并给出手动地址', state?.disabled === true && (await tileText(page)).includes('127.0.0.1:7890'), await tileText(page));
    await goPage(page, 7);
    await page.waitFor(`document.querySelector('[data-action="backup-export"]') !== null`);
    const buttons = await page.eval(`['backup-export', 'backup-restore', 'diagnostics-export'].map((name) => { const el = document.querySelector('[data-action="' + name + '"]'); return el.disabled && el.closest('.srow').innerText.includes('当前后端不提供这项功能'); })`);
    check('S6 备份与诊断按钮禁用并说明原因', buttons.every((item) => item === true), q(buttons));
    check('S6 没有持久化能力时说明只在本次运行有效', (await page.text('[data-note="storage"]')).includes('只在本次运行期间有效'));
    check('S6 没有调用未公布的系统接口', !fake.s.log.some((item) => item.method.startsWith('System.') && item.method !== 'System.Capabilities'));
    const report = consoleReport(page, fake);
    check('S6 没有控制台错误或警告', report.others.length === 0 && report.rejected === report.expected, report.others.join('\n      '));
    await page.shot('s-settings-bare');
    page.close();
  } finally {
    await server.close();
  }
}

const fake = createFakeBackend({ capabilities: ['subs.text', 'system.proxy', 'backup', 'diagnostics', 'settings.persist'], extra: [fakeSystem] });
seed(fake);
const server = await startServer(fake.handle);
const browser = await launchBrowser();
let passed = false;
try {
  const page = await openBridgePage(browser, server, { width: 1280, height: 800 });
  const tick = createTicker(page, fake);
  await page.waitFor(`document.body.innerText.includes('内核未启动')`);
  await tick();

  await overviewSection(page, fake, tick);
  await settingsSection(page, fake);

  const forbidden = fake.s.log.map((item) => item.method).filter((name) => FORBIDDEN.includes(name));
  check('S 全程没有调用服务与 TUN 接口', forbidden.length === 0, forbidden.join(', '));
  const unknown = [...new Set(fake.s.log.map((item) => item.method))].filter((name) => !fake.methodNames().includes(name));
  check('S 全程只调用假后端实现了的接口', unknown.length === 0, unknown.join(', '));
  const report = consoleReport(page, fake);
  check('S 没有控制台错误或警告（后端拒绝的调用除外）', report.others.length === 0, report.others.join('\n      '));
  check('S 失败应答次数与控制台记录一致', report.rejected === report.expected, `记录 ${report.rejected} 条，实际返回 4xx/5xx ${report.expected} 次`);
  page.close();

  await bareSection(browser);
  passed = summary();
} catch (error) {
  console.error('冒烟脚本自身出错：', error);
} finally {
  await browser.close();
  await server.close();
}
process.exit(passed ? 0 : 1);

// 场景 C（第二部分）：节点收藏与切换、设置锁定与保存、导入校验、键盘、
// 接口缺失/参数不符、内核异常、窗口按钮、最小窗口尺寸、启动探测失败与重试。
import { CLIPPED, NO_OVERFLOW, createChecks, launchBrowser, sleep, startServer } from './harness.mjs';
import { createFakeBackend } from './fake-backend.mjs';
import { activeMode, consoleReport, createTicker, openBridgePage } from './bridge-kit.mjs';

const { check, summary } = createChecks();
const fake = createFakeBackend();
fake.s.nodes = [
  { id: 'a', tag: '节点甲', outbound: { type: 'socks', server: '127.0.0.1', server_port: 1080 } },
  { id: 'b', tag: '节点乙', outbound: { type: 'trojan', server: '127.0.0.2', server_port: 443 } },
];

/** 与 internal/app 的公开方法一一对应；测试要证明前端没有调用别的接口。 */
const IMPLEMENTED = [
  'Core.Status',
  'Core.Configure',
  'Core.Start',
  'Core.Stop',
  'Core.Restart',
  'Core.SetMode',
  'Core.SelectNode',
  'Core.Snapshot',
  'Nodes.List',
  'Nodes.SetFavorite',
  'Settings.Get',
  'Settings.Set',
  'Subs.List',
  'Subs.AddFromText',
  'Subs.Remove',
  'System.Capabilities',
];
const MODES = '[role="radiogroup"][aria-label="代理模式"] button';
const CURRENT_NODE = `[...document.querySelectorAll('li.row.selected')].map((row) => row.querySelector('.name .selectable')?.innerText.trim()).join(',')`;
const HERO_ERRORS = `[...document.querySelectorAll('.hero .inline-error')].map((item) => item.innerText.trim())`;
const SAVE_DISABLED = `[...document.querySelectorAll('.savebar button')].find((item) => item.innerText.trim() === '保存')?.disabled`;
/** 「通知数:与保存栏重叠的通知数」；保存栏出现时通知应当整体上移。 */
const TOAST_OVERLAP = `(() => {
  const bar = document.querySelector('.savebar')?.getBoundingClientRect();
  if (!bar) return 'no-bar';
  const all = [...document.querySelectorAll('.toasts .toast')].map((el) => el.getBoundingClientRect());
  const hit = all.filter((r) => r.bottom > bar.top && r.top < bar.bottom && r.right > bar.left && r.left < bar.right);
  return all.length + ':' + hit.length;
})()`;
/** 当前焦点的简短描述，用来断言键盘焦点没有被禁用按钮带跑。 */
const ACTIVE = `(() => { const el = document.activeElement; return el === null || el === document.body ? 'BODY' : el.tagName + ':' + (el.getAttribute('aria-label') ?? el.innerText.trim()).slice(0, 10); })()`;
/** 按文字聚焦 savebar 里的按钮（模拟键盘用户先 Tab 到按钮再回车）。 */
const focusSavebar = (page, label) =>
  page.eval(
    `(() => { const el = [...document.querySelectorAll('.savebar button')].find((b) => b.innerText.trim() === ${JSON.stringify(label)}); if (!el) return false; el.focus(); return document.activeElement === el; })()`,
  );

const lastCall = (method) => fake.s.log.findLast((item) => item.method === method);
const countCalls = (method) => fake.s.log.filter((item) => item.method === method).length;
const reject = (method, kind, message) => fake.s.faults.set(method, { status: 422, body: { kind, message } });
const goPage = (page, index) => page.key(String(index), { ctrl: true, code: `Digit${index}`, vk: 48 + index });
const toastHas = (page, message) => page.waitFor(`(document.querySelector('.toasts')?.innerText ?? '').includes(${JSON.stringify(message)})`);
const heroHasError = (page, message) => page.waitFor(`${HERO_ERRORS}.includes(${JSON.stringify(message)})`);
/** 像用户一样点开折叠区（已展开时不再点，避免又收起来）。 */
const openDev = (page) =>
  page.eval(`(() => { const el = document.querySelector('details.dev'); if (el && !el.open) el.querySelector('summary').click(); return el?.open === true; })()`);

/** 聚焦输入框、全选后一次性输入（走 CDP 的真实输入事件，Svelte 的 oninput 会收到）。 */
async function typeInto(page, selector, text) {
  const found = await page.eval(
    `(() => { const el = document.querySelector(${JSON.stringify(selector)}); if (!el) return false; el.focus(); el.select(); return true; })()`,
  );
  if (!found) throw new Error(`找不到输入框：${selector}`);
  await page.send('Input.insertText', { text });
  await sleep(150);
}

/** 键盘激活（Enter）：Chrome 需要带 text 的 keyDown 才会触发按钮的默认行为。 */
async function pressEnter(page) {
  const base = { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13 };
  await page.send('Input.dispatchKeyEvent', { type: 'keyDown', ...base, text: '\r' });
  await page.send('Input.dispatchKeyEvent', { type: 'keyUp', ...base });
  await sleep(300);
}

async function nodesSection(page, tick) {
  console.log('\n== C6 节点页（内核未启动） ==');
  await goPage(page, 2);
  await page.waitFor(`document.querySelector('button[aria-label="使用节点 节点乙"]') !== null`);
  const listed = await page.text('main');
  check(
    'C6 列出节点与分组统计',
    ['节点甲', '节点乙', 'TROJAN', '127.0.0.2:443', '共 2 个节点 · 1 个分组'].every((item) => listed.includes(item)),
    listed.slice(0, 220),
  );
  check('C6 默认选中第一个成员', (await page.eval(CURRENT_NODE)) === '节点甲', await page.eval(CURRENT_NODE));
  check('C6 未启动时不能切换节点', (await page.eval(`document.querySelector('button[aria-label="使用节点 节点乙"]')?.disabled`)) === true);

  await page.click('button[aria-label="收藏 节点乙"]');
  const favorite = lastCall('Nodes.SetFavorite')?.args ?? null;
  check('C6 收藏走 Nodes.SetFavorite(id, on)', JSON.stringify(favorite) === '["b",true]', JSON.stringify(favorite));
  check(
    'C6 收藏状态立即更新',
    await page.waitFor(`document.querySelector('button[aria-label="取消收藏 节点乙"]')?.getAttribute('aria-pressed') === 'true'`),
  );

  reject('Nodes.SetFavorite', 'RuntimeError', '节点不存在');
  await page.click('button[aria-label="收藏 节点甲"]');
  check('C6 收藏失败时提示后端原文', await toastHas(page, '节点不存在'));
  check(
    'C6 收藏失败后状态回滚',
    await page.waitFor(`document.querySelector('button[aria-label="收藏 节点甲"]')?.getAttribute('aria-pressed') === 'false'`),
  );

  console.log('\n== C7 启动内核并切换节点 ==');
  await page.clickText('启动内核', '.inline-note button');
  check('C7 节点页可以直接启动内核', await page.waitFor(`document.querySelector('button[aria-label="使用节点 节点乙"]')?.disabled === false`));
  await tick();
  check('C7 启动后未运行提示消失', await page.eval(`!document.body.innerText.includes('启动后才能切换出口节点')`));

  await page.click('button[aria-label="使用节点 节点乙"]');
  const select = lastCall('Core.SelectNode')?.args ?? null;
  check('C7 切换走 Core.SelectNode(分组, 节点)', JSON.stringify(select) === '["proxy","b"]', JSON.stringify(select));
  check('C7 「当前」标记移到新节点', await page.waitFor(`${CURRENT_NODE} === '节点乙'`), await page.eval(CURRENT_NODE));
  check('C7 切换成功提示', await toastHas(page, '已切换到「节点乙」'));

  reject('Core.SelectNode', 'RuntimeError', '请先启动内核再切换节点');
  await page.click('button[aria-label="使用节点 节点甲"]');
  check('C7 切换失败时提示后端原文', await toastHas(page, '请先启动内核再切换节点'));
  check('C7 切换失败后当前节点不变', (await page.eval(CURRENT_NODE)) === '节点乙', await page.eval(CURRENT_NODE));
  await page.shot('c-nodes-running');
}

async function settingsSection(page, tick) {
  console.log('\n== C8 设置在内核运行中锁定 ==');
  await goPage(page, 7);
  await page.waitFor(`document.querySelector('button[role="switch"][aria-label="允许局域网连接"]') !== null`);
  await page.click('button[role="switch"][aria-label="允许局域网连接"]');
  const lockedBar = await page.text('.savebar');
  check('C8 修改设置时说明要先停止内核', lockedBar.includes('内核运行中：需要先停止内核，才能修改设置。'), lockedBar);
  check('C8 运行中保存被禁用', (await page.eval(SAVE_DISABLED)) === true);
  check('C8 运行中没有提交设置', countCalls('Settings.Set') === 0);
  await openDev(page);
  check('C8 导入区说明锁定原因', (await page.text('details.dev')).includes('内核运行中：请先停止内核，再导入配置。'));
  await page.shot('c-settings-locked');

  await page.clickText('停止内核', '.savebar button');
  check('C8 保存栏可直接停止内核', await page.waitFor(`(document.querySelector('.savebar')?.innerText ?? '').includes('有未保存的修改。')`));
  check('C8 停止走 Core.Stop', countCalls('Core.Stop') === 1);
  check('C8 停止后可以保存', (await page.eval(SAVE_DISABLED)) === false);
  await tick();

  console.log('\n== C9 设置的校验、拒绝与保存 ==');
  await typeInto(page, 'input[aria-label="混合端口"]', '70000');
  const invalidRow = await page.text('#set-network');
  check('C9 端口非法时就地提示', invalidRow.includes('请输入 1 到 65535 之间的整数'), invalidRow.slice(0, 160));
  check('C9 端口非法时不能保存', (await page.text('.savebar')).includes('端口无效，修正后才能保存。') && (await page.eval(SAVE_DISABLED)) === true);
  check('C9 端口非法时没有提交', countCalls('Settings.Set') === 0);

  await typeInto(page, 'input[aria-label="混合端口"]', '7891');
  await typeInto(page, 'input[aria-label="远程 DNS 服务器"]', 'https://dns.google/dns-query');
  await page.clickText('保存', '.savebar button');
  check('C9 被拒绝时出现保存栏错误', await page.waitFor(`document.querySelector('.savebar [role="alert"]') !== null`));
  const rejected = await page.text('.savebar [role="alert"]');
  check('C9 后端拒绝文案原样展示', rejected === '设置无效，请检查 DNS、端口和入站设置', rejected);
  check('C9 被拒绝后保留用户的修改', (await page.eval(`document.querySelector('input[aria-label="远程 DNS 服务器"]').value`)) === 'https://dns.google/dns-query');
  const overlap = await page.eval(TOAST_OVERLAP);
  check('C9 通知不遮挡保存栏里的按钮', /^[1-9]\d*:0$/.test(overlap), `通知数:重叠数 = ${overlap}`);
  await page.shot('c-settings-rejected');

  await page.clickText('还原', '.savebar button');
  check(
    'C9 还原回后端取值',
    await page.eval(
      `document.querySelector('.savebar') === null &&
       document.querySelector('input[aria-label="混合端口"]').value === '7890' &&
       document.querySelector('input[aria-label="远程 DNS 服务器"]').value === 'https://1.1.1.1/dns-query' &&
       document.querySelector('button[role="switch"][aria-label="允许局域网连接"]').getAttribute('aria-checked') === 'false'`,
    ),
  );

  await typeInto(page, 'input[aria-label="混合端口"]', '7891');
  await page.clickText('保存', '.savebar button');
  check('C9 保存成功给出提示', await toastHas(page, '设置已保存'));
  const saved = lastCall('Settings.Set')?.args?.[0] ?? null;
  check(
    'C9 提交的是完整设置对象',
    saved?.mixedPort === 7891 &&
      saved?.allowLan === false &&
      saved?.theme === 'system' &&
      saved?.dns?.remote === 'https://1.1.1.1/dns-query' &&
      saved?.clashApiMode === 'random',
    JSON.stringify(saved),
  );
  check('C9 保存后保存栏消失', await page.waitFor(`document.querySelector('.savebar') === null`));

  await goPage(page, 1);
  check('C9 概览提示使用保存后的端口', await page.waitFor(`document.querySelector('.hero')?.innerText.includes('127.0.0.1:7891')`));
}

async function importSection(page) {
  console.log('\n== C10 开发配置导入的本地预检 ==');
  await goPage(page, 7);
  await page.waitFor(`document.getElementById('dev-spec') !== null`);
  check('C10 展开导入区', await openDev(page));
  const legal = await page.text('.legal');
  check(
    'C10 关于区显示 GPL 法律声明（版权、无担保、许可证、内核署名、无隶属关系）',
    ['版权所有', '不提供任何担保', 'GNU 通用公共许可证第 3 版', 'nekohasekai', '没有隶属', 'LICENSE'].every((item) => legal.includes(item)),
    legal,
  );
  const before = countCalls('Core.Configure');

  await typeInto(page, '#dev-spec', '{"nodes": [');
  await page.clickText('导入', 'details.dev button');
  const syntax = await page.text('details.dev [role="alert"]');
  check('C10 语法错误在本地拦截', syntax === '内容不是合法的 JSON，请检查括号、引号与逗号', syntax);
  check('C10 语法错误没有提交后端', countCalls('Core.Configure') === before);

  await typeInto(page, '#dev-spec', '[1, 2]');
  await page.clickText('导入', 'details.dev button');
  check('C10 非对象 JSON 在本地拦截', (await page.text('details.dev [role="alert"]')) === '配置必须是一个 JSON 对象');
  check('C10 非对象 JSON 没有提交后端', countCalls('Core.Configure') === before);

  await typeInto(page, '#dev-spec', '{"unknown":1}');
  await page.clickText('导入', 'details.dev button');
  check(
    'C10 后端拒绝时显示原文',
    await page.waitFor(`document.querySelector('details.dev [role="alert"]')?.innerText.trim() === '配置无效，请检查节点、端口与支持的字段'`),
    await page.text('details.dev [role="alert"]'),
  );
  check('C10 失败后保留输入内容', (await page.eval(`document.getElementById('dev-spec').value`)) === '{"unknown":1}');

  await page.clickText('清空', 'details.dev button');
  check(
    'C10 清空按钮同时清掉内容与错误',
    await page.eval(`document.getElementById('dev-spec').value === '' && document.querySelector('details.dev [role="alert"]') === null`),
  );
  check('C10 清空后焦点交给输入框', (await page.eval(`document.activeElement?.id`)) === 'dev-spec', await page.eval(ACTIVE));

  const submitted = countCalls('Core.Configure');
  await page.clickText('导入', 'details.dev button');
  check('C10 内容为空时说明缺什么', (await page.text('details.dev [role="alert"]')) === '请先粘贴配置内容', await page.text('details.dev'));
  check('C10 内容为空时没有提交后端', countCalls('Core.Configure') === submitted);
}

/** 放在最后执行：第一次文本导入会替换预置的节点甲/乙。 */
async function subsSection(page) {
  console.log('\n== C17 订阅页：粘贴导入与删除 ==');
  // 上一段结束时内核在运行；导入要求先停止。
  fake.s.status = 'stopped';
  await page.goto(`${server.url}/`);
  await page.waitFor(`document.body.innerText.includes('内核未启动')`);
  await goPage(page, 3);
  await page.waitFor(`document.getElementById('sub-text') !== null`);
  check('C17 没有订阅时显示空状态', await page.waitFor(`document.body.innerText.includes('还没有订阅')`));
  const before = countCalls('Subs.AddFromText');

  await page.clickText('导入', '.import button');
  check('C17 内容为空时说明缺什么', (await page.text('.import [role="alert"]')) === '请先粘贴节点链接');
  check('C17 内容为空时没有提交后端', countCalls('Subs.AddFromText') === before);

  await typeInto(page, '#sub-text', 'vmess://abc');
  await page.clickText('导入', '.import button');
  check(
    'C17 全部被跳过时显示后端给的原因',
    await page.waitFor(`document.querySelector('.import [role="alert"]')?.innerText.trim() === '没有可导入的节点：第 1 行暂不支持 vmess 协议'`),
    await page.text('.import'),
  );
  check('C17 失败后保留输入内容', (await page.eval(`document.getElementById('sub-text').value`)) === 'vmess://abc');

  await typeInto(page, '#sub-name', '测试订阅');
  await typeInto(page, '#sub-text', 'socks5://127.0.0.9:1080#%E8%8A%82%E7%82%B9%E4%B8%99\nvmess://abc\nsocks5://127.0.0.9:1081');
  await page.clickText('导入', '.import button');
  check(
    'C17 导入成功后说明结果',
    await page.waitFor(`(document.querySelector('.import [role="status"]')?.innerText ?? '').includes('已导入 2 个节点，跳过 1 行')`),
    await page.text('.import'),
  );
  const status = await page.text('.import [role="status"]');
  check('C17 如实说明只在本次运行内有效', status.includes('本次运行内有效'), status);
  const skipped = await page.text('.import .skipped');
  check('C17 列出被跳过的行与原因', skipped === '第 2 行：暂不支持 vmess 协议', skipped);
  check(
    'C17 导入后清空输入（链接含凭据）',
    await page.eval(`document.getElementById('sub-text').value === '' && document.getElementById('sub-name').value === ''`),
  );
  check('C17 名称原样交给后端', lastCall('Subs.AddFromText')?.args[0] === '测试订阅');
  const row = await page.text('.list .row');
  check('C17 订阅列表显示名称与节点数', row.includes('测试订阅') && row.includes('2 个节点'), row);
  check('C17 没有横向溢出', await page.eval(NO_OVERFLOW));

  await goPage(page, 2);
  check('C17 节点页出现新导入的节点', (await page.waitFor(`document.querySelector('button[aria-label="使用节点 127.0.0.9:1081"]') !== null`)) && (await page.text('main')).includes('节点丙'));
  check('C17 文本导入替换了原有的开发配置节点', await page.eval(`document.querySelector('button[aria-label="使用节点 节点甲"]') === null`));

  await goPage(page, 3);
  reject('Subs.Remove', 'RuntimeError', '请先停止内核，再删除订阅');
  await page.clickText('删除', '.list button');
  await page.clickText('删除', '[role="dialog"] .foot button');
  check('C17 删除被拒绝时提示原文', await toastHas(page, '请先停止内核，再删除订阅'));
  check('C17 删除失败后订阅仍在', (await page.eval(`document.querySelectorAll('.list .row').length`)) === 1);
  await page.clickText('删除', '.list button');
  await page.clickText('删除', '[role="dialog"] .foot button');
  check('C17 删除成功后回到空状态', await page.waitFor(`document.body.innerText.includes('还没有订阅')`));
  await goPage(page, 2);
  check('C17 删除订阅后节点一并移除', await page.waitFor(`document.body.innerText.includes('尚未导入节点')`));
}

async function focusSection(page, tick) {
  console.log('\n== C17 操作前后键盘焦点的去向 ==');
  const power = async (expected) => {
    await goPage(page, 1);
    await page.eval(`document.querySelector('button.power').focus()`);
    await pressEnter(page);
    await page.waitFor(`document.body.innerText.includes(${JSON.stringify(expected)})`);
    await tick();
  };

  // 节点页：切换成功后「使用」按钮会消失，失败时保留。
  await power('内核运行中');
  await goPage(page, 2);
  await page.waitFor(`document.querySelector('button[aria-label="使用节点 节点乙"]')?.disabled === false`);
  await page.eval(`document.querySelector('button[aria-label="使用节点 节点乙"]').focus()`);
  await pressEnter(page);
  check('C17 回车可以切换节点', await page.waitFor(`${CURRENT_NODE} === '节点乙'`), await page.eval(CURRENT_NODE));
  check('C17 「使用」消失后焦点交给同一行的收藏按钮', await page.waitFor(`${ACTIVE} === 'BUTTON:取消收藏 节点乙'`), await page.eval(ACTIVE));

  const selects = countCalls('Core.SelectNode');
  reject('Core.SelectNode', 'RuntimeError', '请先启动内核再切换节点');
  await page.eval(`document.querySelector('button[aria-label="使用节点 节点甲"]').focus()`);
  await pressEnter(page);
  await page.waitFor(`document.querySelector('button[aria-label="使用节点 节点甲"]')?.innerText.trim() === '使用'`);
  check(
    'C17 切换失败后焦点留在原按钮',
    countCalls('Core.SelectNode') === selects + 1 && (await page.eval(ACTIVE)) === 'BUTTON:使用节点 节点甲',
    await page.eval(ACTIVE),
  );

  // 设置页：被拒绝时焦点留在「保存」；保存栏消失时回到所在分区的标题。
  await power('内核未启动');
  await goPage(page, 7);
  await page.waitFor(`document.querySelector('input[aria-label="远程 DNS 服务器"]') !== null`);
  await typeInto(page, 'input[aria-label="远程 DNS 服务器"]', 'https://dns.google/dns-query');
  await focusSavebar(page, '保存');
  await pressEnter(page);
  check(
    'C17 保存被拒绝后焦点留在保存按钮',
    (await page.waitFor(`document.querySelector('.savebar [role="alert"]') !== null`)) && (await page.eval(ACTIVE)) === 'BUTTON:保存',
    await page.eval(ACTIVE),
  );
  await focusSavebar(page, '还原');
  await pressEnter(page);
  check('C17 还原后焦点回到所在分区标题', await page.waitFor(`document.querySelector('.savebar') === null && document.activeElement?.tagName === 'H2'`), await page.eval(ACTIVE));
  await typeInto(page, 'input[aria-label="混合端口"]', '7892');
  await focusSavebar(page, '保存');
  await pressEnter(page);
  check('C17 保存成功后焦点回到所在分区标题', await page.waitFor(`document.querySelector('.savebar') === null && document.activeElement?.tagName === 'H2'`), await page.eval(ACTIVE));

  // 运行中修改设置：在保存栏里停止内核后，下一步就是保存。
  await power('内核运行中');
  await goPage(page, 7);
  await page.waitFor(`document.querySelector('button[role="switch"][aria-label="允许局域网连接"]') !== null`);
  await page.click('button[role="switch"][aria-label="允许局域网连接"]');
  await focusSavebar(page, '停止内核');
  await pressEnter(page);
  check('C17 停止内核后焦点交给保存按钮', await page.waitFor(`${ACTIVE} === 'BUTTON:保存'`), await page.eval(ACTIVE));
  await tick();
  await page.clickText('还原', '.savebar button');
}

async function keyboardSection(page) {
  console.log('\n== C11 键盘操作 ==');
  await goPage(page, 1);
  await page.waitFor(`document.querySelector('${MODES}[aria-checked="true"]') !== null`);
  await page.eval(`document.querySelector('${MODES}[aria-checked="true"]').focus()`);
  await page.key('ArrowRight', { code: 'ArrowRight', vk: 39 });
  check('C11 方向键切换代理模式', await page.waitFor(`${activeMode} === '全局'`), await page.eval(activeMode));
  check('C11 方向键走 Core.SetMode', lastCall('Core.SetMode')?.args?.[0] === 'global', JSON.stringify(lastCall('Core.SetMode')?.args ?? null));
  check('C11 焦点跟随到新选项', (await page.eval(`document.activeElement?.innerText.trim()`)) === '全局');

  await page.eval(`document.querySelector('button.power').focus()`);
  await pressEnter(page);
  check('C11 回车可以启动内核', await page.waitFor(`document.querySelector('button[aria-label="停止内核"]') !== null`));
  check('C11 启动后焦点仍留在电源键', (await page.eval(ACTIVE)) === 'BUTTON:停止内核', await page.eval(ACTIVE));

  await page.eval(`document.querySelector('button[aria-label="重启内核"]').focus()`);
  await pressEnter(page);
  check('C11 重启走 Core.Restart', countCalls('Core.Restart') === 1, `调用 ${countCalls('Core.Restart')} 次`);
  check('C11 重启后仍在运行', await page.waitFor(`document.querySelector('button[aria-label="停止内核"]') !== null && document.body.innerText.includes('内核运行中')`));
  check('C11 重启后焦点仍留在重启按钮', (await page.eval(ACTIVE)) === 'BUTTON:重启内核', await page.eval(ACTIVE));

  await page.eval(`document.querySelector('button.power').focus()`);
  await pressEnter(page);
  check('C11 回车可以停止内核', await page.waitFor(`document.body.innerText.includes('内核未启动')`));
  check('C11 停止后焦点仍留在电源键', (await page.eval(ACTIVE)) === 'BUTTON:启动内核', await page.eval(ACTIVE));
}

async function failureSection(page, tick) {
  console.log('\n== C12 接口缺失 / 参数不符 / 传输失败 ==');
  await goPage(page, 1);
  reject('Core.SetMode', 'ReferenceError', "unknown bound method name 'ferry/internal/app.Core.SetMode'");
  await page.clickText('直连', MODES);
  check('C12 接口不存在时说明原因', await heroHasError(page, '当前后端版本未实现该接口'), JSON.stringify(await page.eval(HERO_ERRORS)));

  reject('Core.SetMode', 'TypeError', 'ferry/internal/app.Core.SetMode expects 2 arguments, got 1');
  await page.clickText('直连', MODES);
  check('C12 参数不符时说明原因', await heroHasError(page, '接口参数与后端不匹配，请更新前端版本'));

  fake.s.faults.set('Core.SetMode', { status: 500, body: 'unexpected failure in transport', type: 'text/plain' });
  await page.clickText('直连', MODES);
  check('C12 传输层错误标明来源并附原文', await heroHasError(page, '与桌面后端通信失败：unexpected failure in transport'));
  check('C12 失败后模式保持不变', (await page.eval(activeMode)) === '全局', await page.eval(activeMode));
  await page.shot('c-transport-error');

  console.log('\n== C13 内核上报异常 ==');
  const CORE_ERROR = '内核连接已断开，请重新启动';
  const CLOSE_BUTTONS = `document.querySelectorAll('.hero .inline-error button[aria-label="关闭提示"]').length`;
  fake.s.status = 'error';
  fake.s.error = CORE_ERROR;
  await tick();
  const heroText = await page.text('.hero');
  check('C13 显示后端上报的原因', heroText.includes('内核异常') && heroText.includes(CORE_ERROR), heroText.slice(0, 200));
  const both = await page.eval(HERO_ERRORS);
  check(
    'C13 内核错误与上一次操作的失败各自显示',
    both.length === 2 && both[0] === CORE_ERROR && both[1].startsWith('与桌面后端通信失败'),
    JSON.stringify(both),
  );
  check('C13 只有操作失败的提示可以关闭', (await page.eval(CLOSE_BUTTONS)) === 1);
  check('C13 侧栏如实反映状态', (await page.text('.core-chip')).includes('内核异常'), await page.text('.core-chip'));
  const tiles = await page.text('.stats');
  check('C13 异常时不把全 0 快照显示成流量', !tiles.includes('0 B/s'), tiles.slice(0, 160));
  await page.shot('c-core-error');

  await page.click('button[aria-label="关闭提示"]');
  const remaining = await page.eval(HERO_ERRORS);
  check('C13 关闭后内核错误仍然保留', remaining.length === 1 && remaining[0] === CORE_ERROR, JSON.stringify(remaining));

  // 启动失败时，后端返回的说明与内核上报的错误是同一句话：只应显示一次。
  reject('Core.Start', 'RuntimeError', CORE_ERROR);
  await page.click('button[aria-label="启动内核"]');
  await sleep(400);
  const deduped = await page.eval(HERO_ERRORS);
  check('C13 相同的错误不重复显示', deduped.length === 1 && deduped[0] === CORE_ERROR, JSON.stringify(deduped));
  check('C13 重复的那条不留下无效的关闭按钮', (await page.eval(CLOSE_BUTTONS)) === 0);

  await page.click('button[aria-label="启动内核"]');
  check('C13 异常之后可以重新启动', await page.waitFor(`document.body.innerText.includes('内核运行中')`));
  check('C13 恢复后错误提示消失', (await page.eval(HERO_ERRORS)).length === 0, JSON.stringify(await page.eval(HERO_ERRORS)));
}

async function chromeSection(page) {
  console.log('\n== C14 窗口按钮 ==');
  await page.click('.titlebar .caption .max');
  check(
    'C14 最大化调用窗口接口并更新标签',
    fake.s.calls.includes('object6.method39') && (await page.waitFor(`document.querySelector('.titlebar .caption .max')?.getAttribute('aria-label') === '还原'`)),
  );
  await page.click('.titlebar .caption .max');
  check('C14 再次点击还原', await page.waitFor(`document.querySelector('.titlebar .caption .max')?.getAttribute('aria-label') === '最大化'`));
  await page.click('.titlebar .caption .min');
  check('C14 最小化调用窗口接口', fake.s.calls.includes('object6.method17'));
  await page.click('.titlebar .caption .close');
  check('C14 关闭调用窗口接口', fake.s.calls.includes('object6.method2'));
}

async function minSizeSection(page) {
  console.log('\n== C15 最小窗口尺寸 900×620 ==');
  await page.viewport(900, 620);
  await sleep(350);
  for (const [index, name] of [
    [1, '概览'],
    [2, '节点'],
    [7, '设置'],
  ]) {
    await goPage(page, index);
    await page.waitFor(`document.querySelector('main h1')?.innerText.trim() === ${JSON.stringify(name)}`);
    await sleep(300);
    const overflow = await page.eval(NO_OVERFLOW);
    const clipped = await page.eval(CLIPPED);
    check(`C15 ${name} 在最小尺寸下不溢出`, overflow && clipped.length === 0, clipped.join(' | '));
    await page.shot(`c-min-${index}`);
  }
  await page.viewport(1180, 780);
  await sleep(300);
}

async function probeSection(page) {
  console.log('\n== C16 启动探测失败与重试 ==');
  fake.s.faults.set('Core.Status', { status: 500, body: 'service unavailable', type: 'text/plain' });
  await page.goto(`${server.url}/`);
  check('C16 探测失败时说明无法连接', await page.waitFor(`document.body.innerText.includes('无法连接桌面后端')`));
  const failed = await page.text('main');
  check('C16 说明失败来源与原文', failed.includes('与桌面后端通信失败：service unavailable'), failed.slice(0, 200));
  check('C16 失败时不显示业务内容', !failed.includes('内核运行中') && !failed.includes('内核未启动'));
  await page.shot('c-probe-failed');

  // 键盘重试：再失败一次时，新的「重试」按钮应当重新拿到焦点。
  fake.s.faults.set('Core.Status', { status: 500, body: 'service unavailable', type: 'text/plain' });
  await page.eval(`[...document.querySelectorAll('button')].find((item) => item.innerText.trim() === '重试').focus()`);
  await pressEnter(page);
  check(
    'C16 再次失败后焦点回到重试按钮',
    await page.waitFor(`document.body.innerText.includes('无法连接桌面后端') && document.activeElement?.innerText.trim() === '重试'`),
    await page.eval(ACTIVE),
  );
  await pressEnter(page);
  check('C16 重试后恢复', await page.waitFor(`document.body.innerText.includes('内核运行中')`));
}

const server = await startServer(fake.handle);
const browser = await launchBrowser();
let passed = false;
try {
  const page = await openBridgePage(browser, server);
  const tick = createTicker(page, fake);
  await page.waitFor(`document.body.innerText.includes('内核未启动')`);
  await tick();

  await nodesSection(page, tick);
  await settingsSection(page, tick);
  await importSection(page);
  await keyboardSection(page);
  await focusSection(page, tick);
  await failureSection(page, tick);
  await chromeSection(page);
  await minSizeSection(page);
  await probeSection(page);
  await subsSection(page);

  const foreign = [...new Set(fake.s.log.map((item) => item.method))].filter((name) => !IMPLEMENTED.includes(name));
  check('C 全程只调用已实现的接口', foreign.length === 0, foreign.join(', '));
  const report = consoleReport(page, fake);
  check('C 没有控制台错误或警告（后端拒绝的调用除外）', report.others.length === 0, report.others.join('\n      '));
  check('C 失败应答次数与控制台记录一致', report.rejected === report.expected, `记录 ${report.rejected} 条，实际返回 4xx/5xx ${report.expected} 次`);
  console.log('      窗口对象调用：', fake.s.calls.filter((item) => item.startsWith('object')).join(', ') || '（无）');
  console.log('      方法调用：', fake.s.calls.filter((item) => !item.startsWith('object')).join(' → '));
  page.close();
  passed = summary();
} catch (error) {
  console.error('冒烟脚本自身出错：', error);
} finally {
  await browser.close();
  await server.close();
}
process.exit(passed ? 0 : 1);

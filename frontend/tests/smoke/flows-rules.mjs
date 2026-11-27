// 场景 R：规则页——路由规则、规则测试、导入、应用分组与规则集（走假的桌面桥接）。
//   FERRY_SMOKE_DIST=<构建产物> [FERRY_SMOKE_SHOTS=<截图目录>] node tests/smoke/flows-rules.mjs
import { CLIPPED, NO_OVERFLOW, createChecks, launchBrowser, sleep, startServer } from './harness.mjs';
import { createFakeBackend } from './fake-backend.mjs';
import { consoleReport, createTicker, dispatch, openBridgePage } from './bridge-kit.mjs';
import { fakeRules } from './fake-rules.mjs';

const { check, summary } = createChecks();
const CAPS = ['subs.text', 'rules', 'rules.appgroups', 'rulesets'];
const SIZES = [
  [1280, 800],
  [900, 640],
  [620, 700],
];
const STARTING = '内核正在启动，请稍后重试';
const H1 = `document.querySelector('main h1')?.innerText.trim()`;
const ROW_IDS = `[...document.querySelectorAll('table[aria-label="路由规则"] tr[data-rule-id]')].map((row) => row.dataset.ruleId)`;
const DIALOG = '[role="dialog"]';
const FOOT = `${DIALOG} .foot button`;
const TESTER = 'form[aria-label="规则测试"]';
const RESULT = `${TESTER} [role="status"]`;
const FOCUS_LABEL = `document.activeElement?.getAttribute('aria-label') ?? ''`;
/** 浏览器自带的文件选择框没法在无头模式里操作：改成直接交出 window.__nextFile（没有就当作取消）。 */
const FILE_INPUT_STUB = `(() => { const original = HTMLInputElement.prototype.click; HTMLInputElement.prototype.click = function () { if (this.type !== 'file') return original.call(this); const next = window.__nextFile; window.__nextFile = undefined; if (!next) { this.dispatchEvent(new Event('cancel')); return; } const data = new DataTransfer(); data.items.add(new File([next.text], next.name, { type: next.type })); this.files = data.files; this.dispatchEvent(new Event('change')); }; })()`;

const q = JSON.stringify;
/** 页面内表达式：把整页撑宽的元素（排查溢出时用）。 */
const WIDE = `[...document.querySelectorAll('body *')].filter((el) => el.getBoundingClientRect().right > window.innerWidth + 0.5).slice(0, 6).map((el) => el.tagName.toLowerCase() + '.' + el.className).join(' | ')`;
const fake = createFakeBackend({ capabilities: CAPS, extra: [fakeRules] });
fake.s.nodes = [
  { id: 'a', tag: '节点甲', outbound: { type: 'socks', server: '192.0.2.10', server_port: 1080 } },
  { id: 'b', tag: '节点乙', outbound: { type: 'socks', server: '192.0.2.11', server_port: 1080 } },
];

const lastCall = (method) => fake.s.log.findLast((item) => item.method === method);
const countCalls = (method) => fake.s.log.filter((item) => item.method === method).length;
const reject = (method, message) => fake.s.faults.set(method, { status: 422, body: { kind: 'RuntimeError', message } });
const same = (left, right) => q(left) === q(right);
const goPage = (page, index) => page.key(String(index), { ctrl: true, code: `Digit${index}`, vk: 48 + index });
const toastHas = (page, message) => page.waitFor(`(document.querySelector('.toasts')?.innerText ?? '').includes(${q(message)})`);
const has = (page, selector, text) => page.waitFor(`(document.querySelector(${q(selector)})?.innerText ?? '').includes(${q(text)})`);
const gone = (page, selector) => page.waitFor(`document.querySelector(${q(selector)}) === null`);
const label = (name) => `button[aria-label=${q(name)}]`;
const rowOf = (name) => `[...document.querySelectorAll('tr[data-rule-id]')].find((row) => row.querySelector('.name')?.innerText.includes(${q(name)}))`;
const idOf = (name) => fake.s.rules.find((rule) => rule.name === name)?.id ?? '';

async function typeInto(page, selector, text) {
  const found = await page.eval(`(() => { const el = document.querySelector(${q(selector)}); if (!el) return false; el.focus(); el.select(); return true; })()`);
  if (!found) throw new Error(`找不到输入框：${selector}`);
  // 先清空再输入：空文本的 insertText 不会替换选区。
  if (text === '') await page.key('Delete', { code: 'Delete', vk: 46 });
  else await page.send('Input.insertText', { text });
  await sleep(150);
}

/** 真正的键盘激活：带 text 的 keyDown 才会产生 keypress，按钮才会响应回车。 */
async function pressEnter(page) {
  const base = { key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, nativeVirtualKeyCode: 13 };
  await page.send('Input.dispatchKeyEvent', { type: 'keyDown', text: '\r', ...base });
  await page.send('Input.dispatchKeyEvent', { type: 'keyUp', ...base });
  await sleep(250);
}

async function layout(page, name, title) {
  for (const [width, height] of SIZES) {
    await page.viewport(width, height);
    await sleep(350);
    const clipped = await page.eval(CLIPPED);
    check(`${title} ${width}px 无溢出、无裁切`, (await page.eval(NO_OVERFLOW)) && clipped.length === 0, clipped.join(' | ') || (await page.eval(WIDE)));
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

/** 打开编辑器、填表并保存；fields 里没有的项保持默认。 */
async function createRule(page, { name, apps, hosts, ports, action }) {
  await page.click('[data-rule-create]');
  await page.waitFor(`document.querySelector(${q(`${DIALOG} input[name="rule-name"]`)}) !== null`);
  await typeInto(page, `${DIALOG} input[name="rule-name"]`, name);
  if (apps !== undefined) await typeInto(page, `${DIALOG} textarea[name="rule-apps"]`, apps);
  if (hosts !== undefined) await typeInto(page, `${DIALOG} textarea[name="rule-hosts"]`, hosts);
  if (ports !== undefined) await typeInto(page, `${DIALOG} input[name="rule-ports"]`, ports);
  if (action !== undefined) await page.clickText(action, `${DIALOG} [role="radiogroup"][aria-label="动作"] button`);
  await page.clickText('保存', FOOT);
  await gone(page, DIALOG);
}

async function tableSection(page) {
  console.log('\n== R1 规则页与固定规则 ==');
  await goPage(page, 4);
  check('R1 标题是「规则」', await page.waitFor(`${H1} === '规则'`));
  check('R1 读到首尾两条固定规则', await page.waitFor(`${q(q(['builtin-lan', 'builtin-default']))} === JSON.stringify(${ROW_IDS})`));
  const pinned = await page.eval(`[...document.querySelectorAll('tr[data-rule-id]')].every((row) => row.classList.contains('pinned') && row.querySelector('.lock') !== null && row.querySelector('button') === null)`);
  check('R1 固定规则带锁、没有任何操作按钮', pinned);
  check('R1 三个页签都在，没有 DNS 与场景模板', same(await page.eval(`[...document.querySelectorAll('[role="tablist"] [role="tab"]')].map((tab) => tab.innerText.trim())`), ['路由规则', '规则集', '应用分组']));
  check('R1 副标题说明匹配顺序', await has(page, '.page-head', '自上而下匹配，首条命中生效'));

  console.log('\n== R2 新建规则 ==');
  await page.click('[data-rule-create]');
  await page.waitFor(`document.querySelector(${q(DIALOG)}) !== null`);
  check('R2 打开时焦点在名称输入框', await page.waitFor(`document.activeElement?.getAttribute('name') === 'rule-name'`));
  await page.clickText('保存', FOOT);
  check('R2 没填名称时不提交', (await has(page, `${DIALOG} [role="alert"]`, '请填写规则名称')) && countCalls('Rules.Save') === 0);
  await typeInto(page, `${DIALOG} input[name="rule-name"]`, '直连测试');
  await typeInto(page, `${DIALOG} textarea[name="rule-hosts"]`, '*.example.invalid');
  await typeInto(page, `${DIALOG} input[name="rule-ports"]`, '70000');
  await page.clickText('保存', FOOT);
  check('R2 端口越界在提交前拦下，文案与后端一致', (await has(page, DIALOG, '规则端口必须为 1 到 65535 或递增的端口范围')) && countCalls('Rules.Save') === 0);
  await page.shot('rules-editor-error');
  await typeInto(page, `${DIALOG} input[name="rule-ports"]`, '443');
  await page.clickText('直连', `${DIALOG} [role="radiogroup"][aria-label="动作"] button`);
  check('R2 选了直连后不再显示代理目标', await gone(page, `${DIALOG} select[name="rule-target"]`));
  await page.clickText('保存', FOOT);
  check('R2 保存成功后提示并关闭', (await toastHas(page, '已新建规则「直连测试」')) && (await gone(page, DIALOG)));
  const created = lastCall('Rules.Save')?.args[0] ?? {};
  check('R2 新建时 id 为空、条件如实提交', created.id === '' && created.action === 'direct' && created.targetId === undefined && same(created.hosts, ['*.example.invalid']) && same(created.ports, ['443']) && created.enabled === true, q(created));
  await createRule(page, { name: '代理测试', apps: 'chrome.exe' });
  const proxied = lastCall('Rules.Save')?.args[0] ?? {};
  check('R2 代理规则默认指向默认代理组', proxied.action === 'proxy' && proxied.targetId === 'proxy' && same(proxied.apps, ['chrome.exe']), q(proxied));
  await page.click('[data-rule-create]');
  await typeInto(page, `${DIALOG} input[name="rule-name"]`, 'IP 规则');
  await page.clickText('中国大陆 IP', `${DIALOG} [role="group"][aria-label="添加规则集"] button`);
  await page.clickText('直连', `${DIALOG} [role="radiogroup"][aria-label="动作"] button`);
  await page.clickText('保存', FOOT);
  await gone(page, DIALOG);
  const byIp = lastCall('Rules.Save')?.args[0] ?? {};
  check('R2 规则集快捷按钮写入 geoip:cn，默认先解析域名', same(byIp.hosts, ['geoip:cn']) && byIp.resolveBefore === true, q(byIp));
  const order = ['builtin-lan', idOf('直连测试'), idOf('代理测试'), idOf('IP 规则'), 'builtin-default'];
  check('R2 新规则排在默认规则之前', await page.waitFor(`${q(q(order))} === JSON.stringify(${ROW_IDS})`));
  check('R2 规则集条件用专门的标签样式', await page.eval(`${rowOf('IP 规则')}?.querySelector('.chip.set')?.innerText.trim() === 'geoip:cn'`));

  console.log('\n== R3 编辑与后端拒绝 ==');
  await page.click(label('编辑「直连测试」'));
  check('R3 编辑器带出原有内容', await page.waitFor(`document.querySelector(${q(`${DIALOG} input[name="rule-name"]`)})?.value === '直连测试'`));
  await typeInto(page, `${DIALOG} input[name="rule-name"]`, '直连改名');
  reject('Rules.Save', STARTING);
  await page.clickText('保存', FOOT);
  check('R3 后端拒绝时原样显示说明，编辑器不关闭', (await has(page, `${DIALOG} .inline-error`, STARTING)) && (await page.eval(`document.querySelector(${q(DIALOG)}) !== null`)));
  await page.clickText('保存', FOOT);
  check('R3 再次保存成功', (await toastHas(page, '已保存规则「直连改名」')) && (await gone(page, DIALOG)));
  check('R3 修改时带上原来的 id', lastCall('Rules.Save')?.args[0]?.id === idOf('直连改名') && idOf('直连改名') !== '');

  console.log('\n== R4 启停与排序 ==');
  const first = idOf('直连改名');
  await page.click(label('启用「直连改名」'));
  check('R4 停用规则调用 SetEnabled', await page.waitFor(`${rowOf('直连改名')}?.classList.contains('off') === true`), q(lastCall('Rules.SetEnabled')?.args));
  check('R4 SetEnabled 的参数正确', same(lastCall('Rules.SetEnabled')?.args, [first, false]));
  check('R4 停用的行降到 45% 不透明度', await page.waitFor(`getComputedStyle(${rowOf('直连改名')}.querySelector('td.name')).opacity === '0.45'`));
  await page.click(label('上移「直连改名」'));
  await sleep(200);
  check('R4 已在最前的规则不能上移（按钮仍可聚焦）', countCalls('Rules.Reorder') === 0 && (await page.eval(`document.querySelector(${q(label('上移「直连改名」'))}).getAttribute('aria-disabled') === 'true' && !document.querySelector(${q(label('上移「直连改名」'))}).disabled`)));
  await page.eval(`document.querySelector(${q(label('下移「直连改名」'))}).focus()`);
  await pressEnter(page);
  const moved = ['builtin-lan', idOf('代理测试'), first, idOf('IP 规则'), 'builtin-default'];
  check('R4 用键盘下移：Reorder 收到完整的新顺序', await page.waitFor(`${q(q(moved))} === JSON.stringify(${ROW_IDS})`), q(lastCall('Rules.Reorder')?.args));
  check('R4 Reorder 的参数正确', same(lastCall('Rules.Reorder')?.args, [moved]));
  check('R4 移动后焦点仍在这条规则的按钮上', (await page.eval(FOCUS_LABEL)).includes('直连改名'), await page.eval(FOCUS_LABEL));
  reject('Rules.Reorder', STARTING);
  await page.click(label('上移「直连改名」'));
  check('R4 内核启动中时排序被拒绝，说明原样显示', await toastHas(page, STARTING));
  check('R4 被拒绝后顺序不变', same(await page.eval(ROW_IDS), moved));

  console.log('\n== R5 rules:applied 事件 ==');
  const before = countCalls('Rules.List');
  fake.s.rules = fake.s.rules.map((rule) => (rule.name === 'IP 规则' ? { ...rule, name: '大陆 IP 直连' } : rule));
  await dispatch(page, 'rules:applied', { version: 99 });
  check('R5 收到事件后重新读取规则', (await page.waitFor(`${rowOf('大陆 IP 直连')} !== undefined`)) && countCalls('Rules.List') > before);
}

async function testerSection(page) {
  console.log('\n== R6 规则测试 ==');
  const app = `${TESTER} input[name="test-app"]`;
  const host = `${TESTER} input[name="test-host"]`;
  const run = () => page.clickText('测试', `${TESTER} button[type="submit"]`);
  await run();
  check('R6 没填主机时不调用后端', (await has(page, RESULT, '请填写目标主机')) && countCalls('Rules.Test') === 0);
  await typeInto(page, host, 'a.example.invalid');
  await run();
  check('R6 需要解析域名的结果如实标成「无法确定」', (await has(page, RESULT, '需要解析域名')) && (await has(page, RESULT, '无法确定')));
  const uncertain = await page.eval(`(() => { const el = document.querySelector(${q(RESULT)}); return !el.innerText.trim().startsWith('命中') && el.querySelector('.pill.direct, .pill.proxy, .pill.block') === null; })()`);
  check('R6 dns-required 不显示成命中，也不给出动作', uncertain, await page.text(RESULT));
  check('R6 Test 的参数：端口是数字，协议是 tcp', same(lastCall('Rules.Test')?.args, ['', 'a.example.invalid', 443, 'tcp']), q(lastCall('Rules.Test')?.args));
  await page.shot('rules-tester-uncertain');
  await typeInto(page, app, 'chrome.exe');
  check('R6 改了输入后旧结果作废', await has(page, RESULT, '填写后点「测试」'));
  await run();
  check('R6 命中代理规则：序号、名称、动作与出站', (await has(page, RESULT, '#2 代理测试')) && (await page.eval(`document.querySelector(${q(`${RESULT} .pill.proxy`)}) !== null`)), await page.text(RESULT));
  await typeInto(page, app, '');
  await typeInto(page, host, '10.1.2.3');
  await page.clickText('UDP', `${TESTER} [role="radiogroup"] button`);
  await run();
  check('R6 局域网地址命中固定规则并直连', (await has(page, RESULT, '#1 本机 / 局域网')) && (await page.eval(`document.querySelector(${q(`${RESULT} .pill.direct`)}) !== null`)), await page.text(RESULT));
  check('R6 协议只会是 tcp 或 udp', same(lastCall('Rules.Test')?.args, ['', '10.1.2.3', 443, 'udp']), q(lastCall('Rules.Test')?.args));
  await page.shot('rules-tester-hit');
  // 规则在别处改过（这里用 rules:applied 事件模拟）：屏幕上的「命中 #1」不再可信。
  const lists = countCalls('Rules.List');
  await dispatch(page, 'rules:applied', { version: 100 });
  check('R6 规则变了之后旧的测试结果作废', (await has(page, RESULT, '填写后点「测试」')) && countCalls('Rules.List') > lists, await page.text(RESULT));
  reject('Rules.Test', '规则测试的应用、主机、端口或网络类型无效');
  await run();
  check('R6 后端拒绝时原样显示说明', await has(page, RESULT, '规则测试的应用、主机、端口或网络类型无效'));
}

async function importSection(page) {
  console.log('\n== R7 导入 ==');
  const content = `${DIALOG} textarea[name="import-content"]`;
  const summaryHas = (text) => page.waitFor(`(document.querySelector('[data-import-summary]')?.innerText ?? '').replace(/\\s+/g, '').includes(${q(text.replace(/\s+/g, ''))})`);
  const saves = countCalls('Rules.Save');
  const batches = countCalls('Rules.SaveMany');
  await page.clickText('导入', '.page-head button');
  await page.waitFor(`document.querySelector(${q(content)}) !== null`);
  await page.clickText('解析并预览', FOOT);
  check('R7 没有内容时不调用后端', (await has(page, `${DIALOG} [role="alert"]`, '请粘贴规则内容，或选择一个文件')) && countCalls('Rules.Import') === 0);
  await typeInto(page, content, '这不是规则');
  await page.clickText('解析并预览', FOOT);
  check('R7 解析失败时原样显示后端说明', await has(page, `${DIALOG} [role="alert"]`, '规则文档无法解析'));
  const text = ['DOMAIN-SUFFIX,cn.example.invalid,DIRECT', 'PROCESS-NAME,ga|me.exe,PROXY', 'GEOIP,CN,DIRECT', 'DST-PORT,8443,REJECT'].join('\n');
  await page.eval(`window.__nextFile = ${q({ name: 'rules.yaml', text, type: 'text/yaml' })}`);
  await page.clickText('选择文件…', `${DIALOG} button`);
  check('R7 选择文件后内容进入输入框', (await has(page, `${DIALOG} .file`, '已读取：rules.yaml')) && (await page.eval(`document.querySelector(${q(content)}).value.includes('DST-PORT,8443')`)));
  await page.clickText('解析并预览', FOOT);
  check('R7 预览列出 3 条规则与 1 条提示', (await summaryHas('解析出 3 条规则，尚未写入')) && (await has(page, '[aria-label="导入提示"]', '第 3 条规则已跳过：条件或目标无法完整转换')));
  check('R7 Import 收到格式与内容', same(lastCall('Rules.Import')?.args, ['clash', text]));
  check('R7 确认之前没有写入任何规则', countCalls('Rules.Save') === saves && countCalls('Rules.SaveMany') === batches && fake.s.rules.every((rule) => !rule.name.startsWith('导入规则')));
  await page.viewport(620, 700);
  await sleep(300);
  const clipped = await page.eval(CLIPPED);
  check('R7 预览对话框 620px 无溢出、无裁切', (await page.eval(NO_OVERFLOW)) && clipped.length === 0, clipped.join(' | ') || (await page.eval(WIDE)));
  await page.shot('rules-import-preview-620');
  await page.viewport(1180, 780);
  await page.clickText('上一步', FOOT);
  check('R7 回到上一步时内容已清空（可能含凭据，不保留）', await page.waitFor(`document.querySelector(${q(content)})?.value === ''`));
  await typeInto(page, content, text);
  await page.clickText('解析并预览', FOOT);
  await summaryHas('解析出 3 条规则');
  await page.clickText('导入 3 条规则', FOOT);
  check('R7 有一条被后端拒绝时整批不写入并如实报告', await has(page, `${DIALOG} [role="alert"]`, '没有导入任何规则：规则应用条件无效'));
  check('R7 整批只调用一次 SaveMany，不逐条保存', countCalls('Rules.SaveMany') === batches + 1 && countCalls('Rules.Save') === saves && fake.s.rules.filter((rule) => rule.name.startsWith('导入规则')).length === 0);
  check('R7 被拒绝后预览原样保留', (await summaryHas('解析出 3 条规则，尚未写入')) && same(await page.eval(`[...document.querySelectorAll(${q(FOOT)})].map((item) => item.innerText.trim())`), ['上一步', '取消', '导入 3 条规则']));
  await page.shot('rules-import-rejected');
  await page.eval(`document.querySelectorAll('${DIALOG} button[aria-label^="不导入"]')[1].click()`);
  check('R7 拿掉一条后焦点留在对话框里（落在补上来的那一行）', await page.waitFor(`document.activeElement?.matches('${DIALOG} button[data-drop]') === true && document.activeElement.getAttribute('aria-label') === '不导入「导入规则 4」'`), await page.eval(`document.activeElement?.outerHTML.slice(0, 120)`));
  check('R7 拿掉被拒绝的那条后提示清空、条数更新', (await summaryHas('已拿掉 1 条，将导入 2 条')) && (await gone(page, `${DIALOG} [role="alert"]`)));
  await page.clickText('导入 2 条规则', FOOT);
  check('R7 其余 2 条一次导入成功', (await toastHas(page, '已导入 2 条规则')) && (await gone(page, DIALOG)) && countCalls('Rules.SaveMany') === batches + 2 && fake.s.rules.filter((rule) => rule.name.startsWith('导入规则')).length === 2);
  check('R7 已导入的规则出现在表格里', await page.waitFor(`${rowOf('导入规则 1')} !== undefined`));

  await page.clickText('导入', '.page-head button');
  await page.clickText('Proxifier (PPX)', `${DIALOG} [role="radiogroup"][aria-label="来源格式"] button`);
  await typeInto(page, content, 'DOMAIN,b.example.invalid,REJECT\nIP-CIDR,198.51.100.0/24,DIRECT');
  await page.clickText('解析并预览', FOOT);
  await summaryHas('解析出 2 条规则');
  await page.clickText('导入 2 条规则', FOOT);
  check('R7 全部成功时提示条数并关闭', (await toastHas(page, '已导入 2 条规则')) && (await gone(page, DIALOG)));
  check('R7 来源格式如实传给后端', lastCall('Rules.Import')?.args[0] === 'ppx' && countCalls('Rules.SaveMany') === batches + 3 && countCalls('Rules.Save') === saves);
}

async function groupsSection(page) {
  console.log('\n== R8 应用分组 ==');
  const cells = (name) => page.eval(`[...([...document.querySelectorAll('tr[data-group-id]')].find((row) => row.querySelector('.name')?.innerText.trim() === ${q(name)})?.querySelectorAll('td.num') ?? [])].map((cell) => cell.innerText.trim())`);
  await page.click('[data-tab="groups"]');
  check('R8 没有分组时给出空状态', await has(page, '#rules-panel', '还没有应用分组'));
  await page.click('[data-group-create]');
  await page.clickText('保存', FOOT);
  check('R8 没填名称时不提交', (await has(page, `${DIALOG} [role="alert"]`, '请填写分组名称')) && countCalls('AppGroups.Save') === 0);
  await typeInto(page, `${DIALOG} input[name="group-name"]`, '浏览器');
  await typeInto(page, `${DIALOG} textarea[name="group-apps"]`, 'chrome.exe\nmsedge.exe');
  await page.clickText('保存', FOOT);
  check('R8 新建分组成功', (await toastHas(page, '已新建应用分组「浏览器」')) && same(lastCall('AppGroups.Save')?.args, [{ id: '', name: '浏览器', apps: ['chrome.exe', 'msedge.exe'] }]), q(lastCall('AppGroups.Save')?.args));
  const groupId = fake.s.appGroups[0]?.id ?? '';
  check('R8 分组列出应用数与引用数', same(await cells('浏览器'), ['2', '0']));

  await page.click('[data-tab="rules"]');
  await page.click(label('编辑「代理测试」'));
  await page.waitFor(`document.querySelector(${q(`${DIALOG} select[aria-label="插入应用分组"]`)}) !== null`);
  await page.eval(`(() => { const el = document.querySelector(${q(`${DIALOG} select[aria-label="插入应用分组"]`)}); el.value = ${q(groupId)}; el.dispatchEvent(new Event('change', { bubbles: true })); })()`);
  await page.clickText('保存', FOOT);
  await gone(page, DIALOG);
  check('R8 规则里以 @group: 引用分组', same(lastCall('Rules.Save')?.args[0]?.apps, ['chrome.exe', `@group:${groupId}`]), q(lastCall('Rules.Save')?.args[0]?.apps));
  check('R8 表格里显示分组名称而不是标识', await page.waitFor(`${rowOf('代理测试')}?.innerText.includes('分组：浏览器')`));

  await page.click('[data-tab="groups"]');
  await page.waitFor(`document.querySelector('tr[data-group-id]') !== null`);
  check('R8 引用数随规则更新', same(await cells('浏览器'), ['2', '1']), q(await cells('浏览器')));
  await page.click(label('删除分组「浏览器」'));
  await page.clickText('删除', FOOT);
  check('R8 仍被引用的分组删不掉，说明原样显示', (await has(page, `${DIALOG} [role="alert"]`, '规则仍在引用此应用分组，请先移除对应引用')) && fake.s.appGroups.length === 1);
  await page.clickText('取消', FOOT);
  await gone(page, DIALOG);
  await page.click(label('编辑分组「浏览器」'));
  await typeInto(page, `${DIALOG} input[name="group-name"]`, '浏览器组');
  await page.clickText('保存', FOOT);
  check('R8 修改分组带上原来的 id', (await toastHas(page, '已保存应用分组「浏览器组」')) && lastCall('AppGroups.Save')?.args[0]?.id === groupId);
  await page.click('[data-group-create]');
  await typeInto(page, `${DIALOG} input[name="group-name"]`, '临时');
  await typeInto(page, `${DIALOG} textarea[name="group-apps"]`, 'a.exe');
  await page.clickText('保存', FOOT);
  await gone(page, DIALOG);
  await page.click(label('删除分组「临时」'));
  await page.clickText('删除', FOOT);
  check('R8 删除没被引用的分组', (await toastHas(page, '已删除应用分组「临时」')) && fake.s.appGroups.length === 1);
  check('R8 删除后焦点交给「新建分组」', await page.waitFor(`document.activeElement?.hasAttribute('data-group-create') === true`));
  await page.shot('rules-groups');
}

async function setsSection(page) {
  console.log('\n== R9 规则集 ==');
  const pills = `[...document.querySelectorAll('tr[data-set-id] .pill')].map((item) => item.innerText.trim())`;
  const update = (text) => page.clickText(text, '#rules-panel button');
  await page.click('[data-tab="sets"]');
  check('R9 列出目录里的 4 个规则集，都是未下载', await page.waitFor(`JSON.stringify(${pills}) === ${q(q(['未下载', '未下载', '未下载', '未下载']))}`), q(await page.eval(pills)));
  check('R9 打开页签只读状态、不触发下载', same(lastCall('RuleSets.List')?.args, [false]) && countCalls('RuleSets.Update') === 0);
  check('R9 说明更新何时生效', await has(page, '#rules-panel .inline-note', '重新应用规则'));
  await update('更新所选');
  check('R9 没有勾选时不调用后端', (await toastHas(page, '请先勾选要更新的规则集')) && countCalls('RuleSets.Update') === 0);
  await page.click('tr[data-set-id="geosite-cn"] input');
  await page.click('tr[data-set-id="geoip-cn"] input');
  await update('更新所选');
  check('R9 更新所选：只提交勾选的标识', (await toastHas(page, '已更新 2 个规则集，重新应用规则或重启内核后生效')) && same(lastCall('RuleSets.Update')?.args, [['geosite-cn', 'geoip-cn']]), q(lastCall('RuleSets.Update')?.args));
  check('R9 更新后重新读取状态与大小', (await page.waitFor(`JSON.stringify(${pills}) === ${q(q(['已就绪', '未下载', '已就绪', '未下载']))}`)) && same(lastCall('RuleSets.List')?.args, [true]));
  check('R9 没有数据的大小与时间显示为 —', await page.eval(`[...document.querySelectorAll('tr[data-set-id="geoip-private"] td.num')].every((cell) => cell.innerText.trim() === '—') && [...document.querySelectorAll('tr[data-set-id="geosite-cn"] td.num')].every((cell) => cell.innerText.trim() !== '—')`));
  reject('RuleSets.Update', '规则集不在支持的目录内');
  await update('全部更新');
  check('R9 后端拒绝时原样提示', await toastHas(page, '规则集不在支持的目录内'));
  await update('全部更新');
  check('R9 全部更新一次提交 4 个', await page.waitFor(`JSON.stringify(${pills}) === ${q(q(['已就绪', '已就绪', '已就绪', '已就绪']))}`) && lastCall('RuleSets.Update')?.args[0]?.length === 4);
  await layout(page, 'rules-sets', 'R9 规则集');
}

async function removeSection(page, tick) {
  console.log('\n== R10 删除与布局 ==');
  await page.click('[data-tab="rules"]');
  const id = idOf('直连改名');
  await page.eval(`document.querySelector(${q(label('删除「直连改名」'))}).focus()`);
  await pressEnter(page);
  check('R10 删除前先确认', await has(page, DIALOG, '删除「直连改名」？删除后无法恢复。'));
  reject('Rules.Remove', STARTING);
  await page.clickText('删除', FOOT);
  check('R10 内核启动中时删除被拒绝，说明原样显示', (await has(page, `${DIALOG} [role="alert"]`, STARTING)) && idOf('直连改名') === id);
  await page.clickText('删除', FOOT);
  check('R10 删除成功', (await toastHas(page, '已删除规则「直连改名」')) && same(lastCall('Rules.Remove')?.args, [id]) && (await page.waitFor(`${rowOf('直连改名')} === undefined`)));
  check('R10 焦点交给顶上来的那一行', await page.waitFor(`${FOCUS_LABEL} === '编辑「大陆 IP 直连」'`), await page.eval(FOCUS_LABEL));
  fake.s.status = 'running';
  await tick();
  check('R10 内核运行中时说明改动会重启内核', await has(page, '.apply-note', '内核运行中会短暂重启'));
  fake.s.status = 'stopped';
  await tick();
  await layout(page, 'rules', 'R10 规则页');
  await page.click(label('编辑「代理测试」'));
  await page.viewport(620, 700);
  await sleep(300);
  const clipped = await page.eval(CLIPPED);
  check('R10 编辑器 620px 无溢出、无裁切', (await page.eval(NO_OVERFLOW)) && clipped.length === 0, clipped.join(' | ') || (await page.eval(WIDE)));
  await page.shot('rules-editor-620');
  await page.clickText('取消', FOOT);
  await page.viewport(1180, 780);
}

/** 另起一个只公布部分能力的后端，确认没有公布的部分既不渲染也不调用。 */
async function capsRun(browser, capabilities, title, body) {
  const other = createFakeBackend({ capabilities, extra: [fakeRules] });
  const server = await startServer(other.handle);
  try {
    const page = await openBridgePage(browser, server);
    await page.waitFor(`document.body.innerText.includes('内核未启动')`);
    await goPage(page, 4);
    await page.waitFor(`${H1} === '规则'`);
    await sleep(500);
    await body(page, [...new Set(other.s.log.map((item) => item.method))]);
    const report = consoleReport(page, other);
    check(`${title} 没有控制台错误或警告`, report.others.length === 0 && report.rejected === report.expected, report.others.join('\n      '));
    page.close();
  } finally {
    await server.close();
  }
}

async function capsSection(browser) {
  console.log('\n== R11 能力开关 ==');
  await capsRun(browser, ['subs.text', 'rules'], 'R11 只有 rules', async (page, called) => {
    check('R11 只有 rules：没有页签，规则表照常', (await page.eval(`document.querySelector('[role="tablist"]') === null`)) && (await page.waitFor(`${ROW_IDS}.length === 2`)));
    check('R11 只有 rules：不调用分组与规则集接口', called.includes('Rules.List') && !called.some((name) => name.startsWith('AppGroups.') || name.startsWith('RuleSets.')), called.join(', '));
    await page.click('[data-rule-create]');
    await page.waitFor(`document.querySelector(${q(DIALOG)}) !== null`);
    check('R11 只有 rules：编辑器里没有规则集快捷按钮', await page.eval(`document.querySelector(${q(`${DIALOG} [aria-label="添加规则集"]`)}) === null`));
  });
  await capsRun(browser, ['subs.text'], 'R11 没有 rules', async (page, called) => {
    check('R11 没有 rules：如实说明后端未提供', (await has(page, 'main', '后端没有提供这项功能')) && (await page.eval(`document.querySelector('[data-rule-create]') === null && document.querySelector('table') === null`)));
    check('R11 没有 rules：不调用任何规则接口', !called.some((name) => /^(Rules|AppGroups|RuleSets)\./.test(name)), called.join(', '));
  });
}

const server = await startServer(fake.handle);
const browser = await launchBrowser();
let passed = false;
try {
  const page = await openBridgePage(browser, server);
  const tick = createTicker(page, fake);
  await page.waitFor(`document.body.innerText.includes('内核未启动')`);
  await tick();
  await page.eval(FILE_INPUT_STUB);

  await tableSection(page);
  await testerSection(page);
  await importSection(page);
  await groupsSection(page);
  await setsSection(page);
  await removeSection(page, tick);

  const known = fake.methodNames();
  const banned = ['System.InstallService', 'System.UninstallService', 'System.RepairService', 'System.SetTun'];
  const foreign = [...new Set(fake.s.log.map((item) => item.method))].filter((name) => !known.includes(name) || banned.includes(name));
  check('R 全程只调用已实现的接口', foreign.length === 0, foreign.join(', '));
  const report = consoleReport(page, fake);
  check('R 没有控制台错误或警告（后端拒绝的调用除外）', report.others.length === 0, report.others.join('\n      '));
  check('R 失败应答次数与控制台记录一致', report.rejected === report.expected, `记录 ${report.rejected} 条，实际返回 4xx/5xx ${report.expected} 次`);
  page.close();

  await capsSection(browser);
  passed = summary();
} catch (error) {
  console.error('冒烟脚本自身出错：', error);
} finally {
  await browser.close();
  await server.close();
}
process.exit(passed ? 0 : 1);

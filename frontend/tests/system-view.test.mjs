import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  BACKUP_KINDS,
  DIAGNOSTICS_KINDS,
  backupFilename,
  diagnosticsFilename,
  persistenceNote,
  sysProxyTile,
} from '../src/lib/system-view.ts';

const AT = new Date(2026, 10, 15, 9, 5, 7).getTime();
const BASE = { capsLoaded: true, capable: true, status: null, error: '', running: false, busy: false, mixedPort: 7890 };

test('backupFilename / diagnosticsFilename：带本地时间戳，精确到秒', () => {
  assert.equal(backupFilename(AT), 'ferry-backup-20261115-090507.ferrybak');
  assert.equal(diagnosticsFilename(AT), 'ferry-diagnostics-20261115-090507.json');
});

test('文件类型过滤项与默认文件名的扩展名一致', () => {
  assert.equal(BACKUP_KINDS[0].pattern, '*.ferrybak');
  assert.equal(DIAGNOSTICS_KINDS[0].pattern, '*.json');
  assert.ok(BACKUP_KINDS.some((kind) => kind.pattern === '*.*'));
});

test('sysProxyTile：能力还没读到时不下结论', () => {
  const tile = sysProxyTile({ ...BASE, capsLoaded: false, capable: false });
  assert.deepEqual(tile, { on: false, canToggle: false, address: '', detail: '正在读取…' });
});

test('sysProxyTile：后端没有这项能力时如实说明，并给出可手动设置的地址', () => {
  const tile = sysProxyTile({ ...BASE, capable: false });
  assert.equal(tile.canToggle, false);
  assert.equal(tile.on, false);
  assert.equal(tile.address, '127.0.0.1:7890');
  assert.equal(tile.detail, '当前版本不能自动设置，可在系统里手动填写');
});

test('sysProxyTile：读取失败时显示后端的说明，不允许切换', () => {
  const tile = sysProxyTile({ ...BASE, error: '无法读取系统代理设置' });
  assert.deepEqual(tile, { on: false, canToggle: false, address: '', detail: '无法读取系统代理设置' });
});

test('sysProxyTile：状态尚未读到', () => {
  assert.equal(sysProxyTile(BASE).detail, '正在读取…');
  assert.equal(sysProxyTile(BASE).canToggle, false);
});

test('sysProxyTile：内核未启动时不能开启', () => {
  const tile = sysProxyTile({ ...BASE, status: { enabled: false, address: '', bypass: [] } });
  assert.deepEqual(tile, { on: false, canToggle: false, address: '127.0.0.1:7890', detail: '启动内核后可开启' });
});

test('sysProxyTile：内核运行中、未开启', () => {
  const tile = sysProxyTile({ ...BASE, running: true, status: { enabled: false, address: '10.0.0.1:8080', bypass: [] } });
  // 未开启时系统里登记的地址不是 Ferry 设置的，不拿来显示。
  assert.deepEqual(tile, { on: false, canToggle: true, address: '127.0.0.1:7890', detail: '未开启' });
});

test('sysProxyTile：已开启时显示系统里实际登记的地址', () => {
  const tile = sysProxyTile({ ...BASE, running: true, status: { enabled: true, address: '127.0.0.1:7891', bypass: [] } });
  assert.deepEqual(tile, { on: true, canToggle: true, address: '127.0.0.1:7891', detail: '已开启' });
});

test('sysProxyTile：已开启但内核不在运行时仍然允许关闭', () => {
  const tile = sysProxyTile({ ...BASE, status: { enabled: true, address: '', bypass: [] } });
  assert.equal(tile.on, true);
  assert.equal(tile.canToggle, true);
  assert.equal(tile.address, '127.0.0.1:7890');
});

test('sysProxyTile：切换进行中不允许再点；端口未知时不编造地址', () => {
  const busy = sysProxyTile({ ...BASE, running: true, busy: true, status: { enabled: false, address: '', bypass: [] } });
  assert.equal(busy.canToggle, false);
  const noPort = sysProxyTile({ ...BASE, running: true, mixedPort: 0, status: { enabled: false, address: '', bypass: [] } });
  assert.equal(noPort.address, '');
});

test('persistenceNote：按后端是否公布 settings.persist 说明保存方式', () => {
  assert.equal(persistenceNote({ capsLoaded: false, persistent: false, demo: false }), '');
  assert.equal(persistenceNote({ capsLoaded: true, persistent: true, demo: false }), '');
  assert.match(persistenceNote({ capsLoaded: true, persistent: false, demo: false }), /只在本次运行期间有效/);
  assert.match(persistenceNote({ capsLoaded: true, persistent: false, demo: true }), /演示模式/);
  // 演示模式优先：即使能力列表里有持久化，也不说「保存在本机」。
  assert.match(persistenceNote({ capsLoaded: true, persistent: true, demo: true }), /演示模式/);
});

test('fileActionBlock：说明备份 / 诊断按钮为什么不可用；可用时返回空串', async () => {
  const { fileActionBlock } = await import('../src/lib/system-view.ts');
  assert.equal(fileActionBlock({ capsLoaded: true, capable: true, dialogs: true }), '');
  assert.equal(fileActionBlock({ capsLoaded: false, capable: false, dialogs: true }), '正在读取后端支持的功能…');
  assert.equal(fileActionBlock({ capsLoaded: true, capable: false, dialogs: true }), '当前后端不提供这项功能');
  // 没有文件选择框（浏览器演示）优先说明：即使能力列表里有，也选不了保存位置。
  assert.equal(fileActionBlock({ capsLoaded: true, capable: true, dialogs: false }), '需要桌面版的文件选择框，浏览器演示中不可用');
  assert.equal(fileActionBlock({ capsLoaded: false, capable: false, dialogs: false }), '需要桌面版的文件选择框，浏览器演示中不可用');
});

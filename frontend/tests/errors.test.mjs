import assert from 'node:assert/strict';
import { test } from 'node:test';
import { BackendError, isUnavailable, messageOfError, toBackendError } from '../src/lib/api/errors.ts';

const TRANSPORT_PREFIX = '与桌面后端通信失败：';
const DETAIL_MAX_LENGTH = 160;

/** @wailsio/runtime 抛出的异常靠 name 区分，这里按同样的方式构造。 */
function named(name, message) {
  return Object.assign(new Error(message), { name });
}

function expectFailure(thrown, kind, message) {
  const error = toBackendError(thrown);
  assert.equal(error.kind, kind);
  assert.equal(error.message, message);
}

test('后端方法返回的 error 原样展示', () => {
  expectFailure(named('RuntimeError', '请先在「订阅」页导入节点'), 'rejected', '请先在「订阅」页导入节点');
});

test('后端文案里出现网络或参数字样时仍然原样展示', () => {
  expectFailure(named('RuntimeError', '订阅下载失败：Failed to fetch'), 'rejected', '订阅下载失败：Failed to fetch');
  expectFailure(named('RuntimeError', 'unknown method'), 'rejected', 'unknown method');
});

test('网络失败归为未连接，先于「参数不符」判断', () => {
  const samples = [
    'Failed to fetch',
    'NetworkError when attempting to fetch resource.',
    'Load failed',
    'fetch failed',
    'connect ECONNREFUSED 127.0.0.1:34115',
  ];
  for (const raw of samples) {
    expectFailure(new TypeError(raw), 'unavailable', '未连接到桌面后端');
    assert.equal(isUnavailable(new TypeError(raw)), true, raw);
  }
});

test('方法不存在归为后端版本不支持', () => {
  const samples = [
    new ReferenceError("unknown bound method name 'ferry/internal/app.Core.LatencyTest'"),
    new ReferenceError('anything'),
    new Error('unknown bound method name x'),
    new Error('unknown method'),
  ];
  for (const thrown of samples) expectFailure(thrown, 'unsupported', '当前后端版本未实现该接口');
});

test('参数不符提示更新前端', () => {
  const samples = [
    new TypeError('json: cannot unmarshal string into Go value of type int'),
    new Error('Core.SetMode expects 1 arguments, got 0'),
  ];
  for (const thrown of samples) expectFailure(thrown, 'argument', '接口参数与后端不匹配，请更新前端版本');
});

test('其它失败标明来源并附上原文', () => {
  expectFailure(new Error('service unavailable'), 'unknown', `${TRANSPORT_PREFIX}service unavailable`);
  assert.equal(messageOfError('boom'), `${TRANSPORT_PREFIX}boom`);
  assert.equal(messageOfError({ message: 'from message' }), `${TRANSPORT_PREFIX}from message`);
  assert.equal(messageOfError({ error: 'from error' }), `${TRANSPORT_PREFIX}from error`);
  assert.equal(isUnavailable(new Error('service unavailable')), false);
});

test('原文压成一行并截断', () => {
  const message = messageOfError(new Error(`第一行\n\t第二行 ${'x'.repeat(300)}`));
  assert.equal(message.startsWith(`${TRANSPORT_PREFIX}第一行 第二行 xxx`), true, message);
  assert.equal(message.endsWith('…'), true);
  assert.equal(message.length, TRANSPORT_PREFIX.length + DETAIL_MAX_LENGTH + 1);
});

test('没有可用文案时给出通用提示', () => {
  for (const thrown of [undefined, null, '', '   ', {}, 42, new Error('')]) {
    expectFailure(thrown, 'unknown', '操作失败，请稍后重试');
  }
});

test('已经归一化的错误原样返回', () => {
  const original = new BackendError('rejected', '节点不存在');
  assert.equal(toBackendError(original), original);
  assert.equal(original.name, 'BackendError');
  assert.equal(original instanceof Error, true);
  assert.equal(isUnavailable(original), false);
});

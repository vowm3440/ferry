import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  asRecord,
  isRecord,
  readArray,
  readBool,
  readNumber,
  readNumberArray,
  readString,
  readStringArray,
} from '../src/lib/guard.ts';

test('isRecord：只接受普通对象', () => {
  assert.equal(isRecord({}), true);
  assert.equal(isRecord({ a: 1 }), true);
  for (const value of [null, undefined, [], 'text', 1, true]) {
    assert.equal(isRecord(value), false, String(value));
  }
});

test('asRecord：不是对象时回退为空对象', () => {
  const source = { a: 1 };
  assert.equal(asRecord(source), source);
  assert.deepEqual(asRecord(null), {});
  assert.deepEqual(asRecord([1, 2]), {});
  assert.deepEqual(asRecord('text'), {});
});

test('readString：类型不符时使用回退值', () => {
  const source = { name: '节点甲', port: 1080, empty: '' };
  assert.equal(readString(source, 'name'), '节点甲');
  assert.equal(readString(source, 'empty', '默认'), '');
  assert.equal(readString(source, 'port'), '');
  assert.equal(readString(source, 'missing'), '');
  assert.equal(readString(source, 'missing', '默认'), '默认');
});

test('readNumber：只接受有限数值', () => {
  const source = { ok: 12.5, zero: 0, nan: Number.NaN, inf: Number.POSITIVE_INFINITY, text: '12' };
  assert.equal(readNumber(source, 'ok'), 12.5);
  assert.equal(readNumber(source, 'zero', 7), 0);
  assert.equal(readNumber(source, 'nan'), 0);
  assert.equal(readNumber(source, 'inf', -1), -1);
  assert.equal(readNumber(source, 'text', -1), -1);
  assert.equal(readNumber(source, 'missing'), 0);
});

test('readBool：只接受布尔值', () => {
  const source = { on: true, off: false, text: 'true', one: 1 };
  assert.equal(readBool(source, 'on'), true);
  assert.equal(readBool(source, 'off', true), false);
  assert.equal(readBool(source, 'text'), false);
  assert.equal(readBool(source, 'one', true), true);
  assert.equal(readBool(source, 'missing'), false);
});

test('readStringArray / readNumberArray：丢弃类型不符的元素', () => {
  const source = { tags: ['a', 1, null, 'b'], points: [1, '2', Number.NaN, 3], text: 'abc' };
  assert.deepEqual(readStringArray(source, 'tags'), ['a', 'b']);
  assert.deepEqual(readNumberArray(source, 'points'), [1, 3]);
  assert.deepEqual(readStringArray(source, 'text'), []);
  assert.deepEqual(readNumberArray(source, 'missing'), []);
});

test('readArray：不是数组时回退为空数组', () => {
  const items = [1, 'a', null];
  assert.equal(readArray({ items }, 'items'), items);
  assert.deepEqual(readArray({ items: 'abc' }, 'items'), []);
  assert.deepEqual(readArray({}, 'items'), []);
});

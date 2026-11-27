import assert from 'node:assert/strict';
import { test } from 'node:test';
import { bytesToBase64, checkFileSize, utf8Length } from '../src/lib/file-pick.ts';

test('bytesToBase64：与标准 Base64 一致，含补位', () => {
  assert.equal(bytesToBase64(new Uint8Array([])), '');
  assert.equal(bytesToBase64(new Uint8Array([102])), 'Zg==');
  assert.equal(bytesToBase64(new Uint8Array([102, 111])), 'Zm8=');
  assert.equal(bytesToBase64(new Uint8Array([102, 111, 111])), 'Zm9v');
  assert.equal(bytesToBase64(new Uint8Array([0, 255, 128, 7])), 'AP+ABw==');
});

test('bytesToBase64：大块数据分段编码，结果不变', () => {
  const bytes = new Uint8Array(200_000);
  for (let index = 0; index < bytes.length; index += 1) bytes[index] = (index * 31 + 7) % 256;
  assert.equal(bytesToBase64(bytes), Buffer.from(bytes).toString('base64'));
});

test('checkFileSize：空文件与超限文件给出说明，其余放行', () => {
  assert.equal(checkFileSize(0, 1024), '文件是空的');
  assert.equal(checkFileSize(1, 1024), '');
  assert.equal(checkFileSize(1024, 1024), '');
  assert.equal(checkFileSize(1025, 1024), '文件过大：不能超过 1 KiB');
  assert.equal(checkFileSize(2 * 1024 * 1024, 1024 * 1024), '文件过大：不能超过 1 MiB');
  assert.equal(checkFileSize(9 * 1024 * 1024, 8 * 1024 * 1024), '文件过大：不能超过 8 MiB');
  assert.equal(checkFileSize(Number.NaN, 1024), '无法读取文件大小');
});

test('utf8Length：按 UTF-8 字节数计，不是字符数', () => {
  assert.equal(utf8Length(''), 0);
  assert.equal(utf8Length('abc'), 3);
  assert.equal(utf8Length('渡'), 3);
  assert.equal(utf8Length('a渡😀'), 1 + 3 + 4);
});

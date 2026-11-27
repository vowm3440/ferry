/*
 * 单元测试直接加载 src 下的 TypeScript 源码（Node 自带的类型擦除），不经过打包，也不引入测试框架。
 * 源码按打包器的习惯省略扩展名（'./guard'、'./api'），Node 不认识这种写法，
 * 这里只补上这一步解析：依次尝试 <路径>.ts 与 <路径>/index.ts。
 *
 * 通过 `node --import ./tests/resolve-ts.mjs --test …` 加载，见 package.json 的 test 脚本。
 * 需要 Node 22.18 或更新的版本（自带类型擦除与 module.registerHooks）。
 */

import { existsSync } from 'node:fs';
import { registerHooks } from 'node:module';
import { fileURLToPath } from 'node:url';

const SUFFIXES = ['.ts', '/index.ts'];

registerHooks({
  resolve(specifier, context, nextResolve) {
    const parent = context.parentURL;
    if (specifier.startsWith('.') && typeof parent === 'string' && parent.startsWith('file:')) {
      const base = new URL(specifier, parent).href;
      const match = SUFFIXES.map((suffix) => base + suffix).find((url) => existsSync(fileURLToPath(url)));
      if (match !== undefined) return nextResolve(match, context);
    }
    return nextResolve(specifier, context);
  },
});

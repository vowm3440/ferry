/*
 * 演示后端：只在用户显式进入「演示模式」时加载，数据全部是合成值。
 * 不访问网络，不读写本机文件与系统设置，不代表任何真实节点或流量；界面会一直显示演示标记。
 * 行为与报错文案贴近真实后端，方便在没有桌面壳时检查交互。
 *
 * 共享状态在 demo-world.ts，各功能区按 Go 侧的服务分文件实现。
 */

import type { Backend } from './backend';
import { createDemoCore } from './demo-core';
import { createDemoInspect } from './demo-inspect';
import { createDemoRules } from './demo-rules';
import { createDemoSubs } from './demo-subs';
import { createDemoSystem } from './demo-system';
import { DemoWorld } from './demo-world';

export function createDemoBackend(): Backend {
  const world = new DemoWorld();
  return {
    kind: 'demo',
    ...createDemoCore(world),
    ...createDemoSubs(world),
    ...createDemoRules(world),
    ...createDemoInspect(world),
    ...createDemoSystem(world),
    on: (event, handler) => world.on(event, handler),
  };
}

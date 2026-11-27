/*
 * 演示后端：能力、系统代理、备份与诊断。
 * 系统代理只是内存里的一个开关，不会改动本机的任何设置。
 */

import type { SystemApi } from './backend';
import { rejected, unsupported, type DemoWorld } from './demo-world';
import {
  CAP_APP_GROUPS,
  CAP_CONNECTIONS,
  CAP_LATENCY,
  CAP_LOGS,
  CAP_NODE_GROUPS,
  CAP_RULES,
  CAP_RULE_SETS,
  CAP_SHARE,
  CAP_SUBS_UA,
  CAP_SUBS_URL,
  CAP_SYS_PROXY,
} from './types-inspect';

/**
 * 只公布演示后端真的模拟了的能力。
 * 不公布：settings.persist（数据关掉页面就没有了）、qr 与 share.lan（没有模拟二维码和局域网分享）、
 * backup 与 diagnostics（演示模式不读写本机文件）。
 */
const DEMO_CAPABILITIES: readonly string[] = [
  'subs.text',
  CAP_SUBS_URL,
  CAP_SUBS_UA,
  CAP_NODE_GROUPS,
  CAP_SHARE,
  CAP_RULES,
  CAP_APP_GROUPS,
  CAP_RULE_SETS,
  CAP_SYS_PROXY,
  CAP_LATENCY,
  CAP_CONNECTIONS,
  CAP_LOGS,
];

const BYPASS: readonly string[] = ['<local>', 'localhost', '127.*', '[::1]'];

export function createDemoSystem(world: DemoWorld): SystemApi {
  let sysProxy = false;

  // 与真实后端一致：停止内核会一并关闭系统代理。
  world.onCoreChange((running) => {
    if (!running) sysProxy = false;
  });

  return {
    async capabilities() {
      return [...DEMO_CAPABILITIES];
    },

    async sysProxyStatus() {
      return {
        enabled: sysProxy,
        address: sysProxy ? `127.0.0.1:${world.settings.mixedPort}` : '',
        bypass: [...BYPASS],
      };
    },

    async setSysProxy(on) {
      if (on && !world.running) throw rejected('请先启动内核，再启用系统代理');
      sysProxy = on;
    },

    async serviceStatus() {
      return { installed: false, running: false, startType: 'disabled', version: '' };
    },

    async installService() {
      throw unsupported();
    },

    async uninstallService() {
      throw unsupported();
    },

    async repairService() {
      throw unsupported();
    },

    async tunStatus() {
      return { enabled: false, running: false, stack: 'mixed', strictRoute: true, excludes: [] };
    },

    async setTun() {
      throw unsupported();
    },

    async exportBackup() {
      throw unsupported();
    },

    async restoreBackup() {
      throw unsupported();
    },

    async exportDiagnostics() {
      throw unsupported();
    },

    async testInbound() {
      return world.running ? 1 : 0;
    },

    async openConfigDir() {
      throw unsupported();
    },

    async openLogDir() {
      throw unsupported();
    },

    async parseDeepLink() {
      throw unsupported();
    },

    async deepLinkStatus() {
      return false;
    },

    async setDeepLinkRegistered() {
      throw unsupported();
    },

    async windowSetLightweight() {
      throw unsupported();
    },

    async updateGetState() {
      return { phase: 'unconfigured', version: '', progress: 0 };
    },

    async checkUpdate() {
      throw rejected('演示模式不支持在线更新检查');
    },

    async applyUpdate() {
      throw unsupported();
    },

    async installUpdate() {
      throw unsupported();
    },
  };
}

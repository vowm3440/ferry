/*
 * 前端侧的设置默认值，仅在「演示模式」中使用。真实后端始终以
 * Settings.Get() 的返回值为准，这里不参与判断后端是否已配置。
 */

import type { SettingsValue } from './types';

/** 深拷贝设置。对 Svelte 的 $state 代理同样有效，返回普通对象。 */
export function cloneSettings(value: SettingsValue): SettingsValue {
  return {
    ...value,
    tun: { ...value.tun, excludes: [...value.tun.excludes] },
    dns: { ...value.dns },
  };
}

export function createDefaultSettings(): SettingsValue {
  return {
    autoStart: false,
    startMinimized: false,
    lightweight: false,
    language: 'zh-CN',
    theme: 'system',
    mixedPort: 7890,
    allowLan: false,
    sysProxyOnStart: false,
    tun: {
      stack: 'mixed',
      strictRoute: true,
      excludes: [],
      mtu: 9000,
      fakeIp: true,
    },
    dns: {
      remote: 'https://1.1.1.1/dns-query',
      direct: 'https://223.5.5.5/dns-query',
      strategy: 'prefer_ipv4',
    },
    ruleSetAutoUpdate: false,
    subscriptionUA: 'clash.meta',
    subscriptionUpdateHours: 24,
    logLevel: 'info',
    updateChannel: 'stable',
    clashApiMode: 'random',
  };
}

/** 与 internal/convert.MaxBytes 一致：粘贴导入的大小上限。 */
export const MAX_IMPORT_BYTES = 1 << 20;

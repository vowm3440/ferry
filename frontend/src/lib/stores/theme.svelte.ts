/*
 * 主题。保存在前端本地（localStorage['ferry.theme']），与 index.html 里的
 * 首屏脚本共用同一个键，避免启动时闪烁。
 *
 * 不依赖 Settings.Set：P0 后端在内核运行时会拒绝一切设置修改，
 * 而切换主题不应受此限制。保存设置时会把当前选择一并写回后端。
 */

import { tr } from '../i18n/index.svelte';

export type ThemeChoice = 'system' | 'dark' | 'light';
export type ResolvedTheme = 'dark' | 'light';

const STORAGE_KEY = 'ferry.theme';
const DARK_QUERY = '(prefers-color-scheme: dark)';

export const THEME_OPTIONS: ReadonlyArray<{ value: ThemeChoice; label: string }> = [
  { value: 'system', label: '跟随系统' },
  { value: 'light', label: '浅色' },
  { value: 'dark', label: '深色' },
];

export function themeOptions(): ReadonlyArray<{ value: ThemeChoice; label: string }> {
  return [
    { value: 'system', label: tr('跟随系统', 'System') },
    { value: 'light', label: tr('浅色', 'Light') },
    { value: 'dark', label: tr('深色', 'Dark') },
  ];
}

export function isThemeChoice(value: string): value is ThemeChoice {
  return value === 'system' || value === 'dark' || value === 'light';
}

function readStored(): ThemeChoice {
  try {
    const stored = window.localStorage.getItem(STORAGE_KEY) ?? '';
    return isThemeChoice(stored) ? stored : 'system';
  } catch {
    return 'system';
  }
}

function writeStored(choice: ThemeChoice): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, choice);
  } catch {
    // 本地存储不可用（例如被策略禁用）时只影响下次启动的初始主题。
  }
}

function readSystem(): ResolvedTheme {
  return window.matchMedia(DARK_QUERY).matches ? 'dark' : 'light';
}

class ThemeStore {
  choice = $state<ThemeChoice>(readStored());
  system = $state<ResolvedTheme>(readSystem());
  resolved = $derived<ResolvedTheme>(this.choice === 'system' ? this.system : this.choice);

  constructor() {
    window.matchMedia(DARK_QUERY).addEventListener('change', (event) => {
      this.system = event.matches ? 'dark' : 'light';
      this.#apply();
    });
    this.#apply();
  }

  #apply(): void {
    document.documentElement.dataset.theme = this.resolved;
  }

  set(choice: ThemeChoice): void {
    this.choice = choice;
    writeStored(choice);
    this.#apply();
  }

  /** 侧栏的快捷切换：在深色与浅色之间来回。 */
  toggle(): void {
    this.set(this.resolved === 'dark' ? 'light' : 'dark');
  }
}

export const theme = new ThemeStore();

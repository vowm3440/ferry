export type Locale = 'zh-CN' | 'en-US';

export interface LocaleOption {
  readonly value: Locale;
  readonly label: string;
}

export const LOCALE_OPTIONS: readonly LocaleOption[] = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'en-US', label: 'English' },
];

let currentLocale: Locale = 'zh-CN';

export function isLocale(value: string): value is Locale {
  return value === 'zh-CN' || value === 'en-US';
}

export function getLocale(): Locale {
  return currentLocale;
}

export function setCurrentLocale(locale: Locale): void {
  currentLocale = locale;
}

export function translateText(locale: Locale, zh: string, en: string): string {
  return locale === 'en-US' ? en : zh;
}

/** 供不引入 Svelte runes 的纯 TS 模块按当前语言返回中英文文案。 */
export function t(zh: string, en: string): string {
  return translateText(currentLocale, zh, en);
}

/*
 * 轻量中英文国际化（zh-CN / en-US）。
 * 默认简体中文，切换后立即生效并保存在本机，同时随 Settings.language 与后端同步。
 */

import {
  getLocale,
  isLocale,
  LOCALE_OPTIONS,
  setCurrentLocale,
  t,
  translateText,
  type Locale,
  type LocaleOption,
} from './locale';

export { getLocale, isLocale, LOCALE_OPTIONS, setCurrentLocale, t, translateText, type Locale, type LocaleOption };

const STORAGE_KEY = 'ferry.language';

function readStoredLocale(): Locale {
  if (typeof window === 'undefined') return 'zh-CN';
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (raw !== null && isLocale(raw)) return raw;
  } catch {
    // 忽略受限存储环境。
  }
  return 'zh-CN';
}

function writeStoredLocale(locale: Locale): void {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(STORAGE_KEY, locale);
  } catch {
    // 忽略受限存储环境。
  }
}

function applyDocumentLang(locale: Locale): void {
  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale;
  }
}

class I18nStore {
  locale = $state<Locale>(readStoredLocale());

  constructor() {
    setCurrentLocale(this.locale);
    applyDocumentLang(this.locale);
  }

  get isEnglish(): boolean {
    return this.locale === 'en-US';
  }

  set(next: Locale): void {
    if (this.locale === next) return;
    this.locale = next;
    setCurrentLocale(next);
    writeStoredLocale(next);
    applyDocumentLang(next);
  }

  syncFromBackend(language: string): void {
    if (isLocale(language) && language !== this.locale) {
      this.locale = language;
      setCurrentLocale(language);
      writeStoredLocale(language);
      applyDocumentLang(language);
    }
  }

  /** 按当前语言返回简体中文或英文文案。 */
  tr(zh: string, en: string): string {
    return translateText(this.locale, zh, en);
  }
}

export const i18n = new I18nStore();

export function tr(zh: string, en: string): string {
  return i18n.tr(zh, en);
}

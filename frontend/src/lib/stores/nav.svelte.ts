/* 七个页面的元数据与当前页面。 */

import type { IconName } from '../icons';
import { tr } from '../i18n/index.svelte';

export type PageId = 'overview' | 'nodes' | 'subs' | 'rules' | 'conns' | 'logs' | 'settings';

export interface PageMeta {
  readonly id: PageId;
  readonly label: string;
  readonly labelEn: string;
  readonly icon: IconName;
}

export const PAGES: readonly PageMeta[] = [
  { id: 'overview', label: '概览', labelEn: 'Overview', icon: 'overview' },
  { id: 'nodes', label: '节点', labelEn: 'Proxies', icon: 'nodes' },
  { id: 'subs', label: '订阅', labelEn: 'Subscriptions', icon: 'subs' },
  { id: 'rules', label: '规则', labelEn: 'Rules', icon: 'rules' },
  { id: 'conns', label: '连接', labelEn: 'Connections', icon: 'conns' },
  { id: 'logs', label: '日志', labelEn: 'Logs', icon: 'logs' },
  { id: 'settings', label: '设置', labelEn: 'Settings', icon: 'settings' },
];

const FALLBACK_PAGE: PageMeta = { id: 'overview', label: '概览', labelEn: 'Overview', icon: 'overview' };

export function pageTitle(page: PageMeta): string {
  return tr(page.label, page.labelEn);
}

class NavStore {
  current = $state<PageId>('overview');

  meta = $derived<PageMeta>(PAGES.find((page) => page.id === this.current) ?? FALLBACK_PAGE);

  go(id: PageId): void {
    this.current = id;
  }

  /** 供 Ctrl+1…7 使用：按侧栏顺序跳转，越界时忽略。 */
  goIndex(index: number): void {
    const page = PAGES[index];
    if (page !== undefined) this.current = page.id;
  }
}

export const nav = new NavStore();

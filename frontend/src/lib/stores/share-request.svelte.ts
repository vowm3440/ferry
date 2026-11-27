/*
 * 「分享」的接力。
 *
 * 订阅页、节点页都能发起分享，但分享对话框（components/ShareDialog.svelte）全局只有一个，
 * 挂在 App 里：它读这里的 target 决定是否打开、分享的是什么。
 * 这里只放「分享谁」，不放导出的内容——导出结果带着节点凭据，只留在对话框自己的局部状态里。
 */

export type ShareKind = 'sub' | 'node';

export interface ShareTarget {
  readonly kind: ShareKind;
  /** 订阅 id 或节点 id。 */
  readonly id: string;
  /** 显示在标题里的名称。 */
  readonly name: string;
}

class ShareRequestStore {
  target = $state.raw<ShareTarget | null>(null);

  open(target: ShareTarget): void {
    this.target = target;
  }

  close(): void {
    this.target = null;
  }
}

export const shareRequest = new ShareRequestStore();

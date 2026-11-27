/*
 * 深链导入确认状态。监听后端 deeplink:received 事件，
 * 始终先展示来源域名与安全提醒，绝不在用户确认前导入。
 */

import { EVENT_DEEPLINK, type Backend, type DeepLinkPreview, type Unsubscribe } from '../api';
import { isRecord } from '../guard';

function parsePreview(raw: unknown): DeepLinkPreview | null {
  if (!isRecord(raw)) return null;
  const scheme = typeof raw['scheme'] === 'string' ? raw['scheme'] : '';
  const kind = raw['kind'] === 'url' || raw['kind'] === 'text' ? raw['kind'] : null;
  const payload = typeof raw['payload'] === 'string' ? raw['payload'] : '';
  if (scheme === '' || kind === null || payload === '') return null;
  return {
    scheme,
    kind,
    name: typeof raw['name'] === 'string' ? raw['name'] : '',
    host: typeof raw['host'] === 'string' ? raw['host'] : '',
    urlMasked: typeof raw['urlMasked'] === 'string' && raw['urlMasked'] !== '' ? raw['urlMasked'] : undefined,
    payload,
    warning: typeof raw['warning'] === 'string' && raw['warning'] !== '' ? raw['warning'] : undefined,
  };
}

class DeepLinkStore {
  pending = $state.raw<DeepLinkPreview | null>(null);
  #off: Unsubscribe | null = null;

  attach(backend: Backend): void {
    this.detach();
    this.#off = backend.on(EVENT_DEEPLINK, (payload) => {
      const preview = parsePreview(payload);
      if (preview !== null) this.pending = preview;
    });
  }

  detach(): void {
    this.#off?.();
    this.#off = null;
    this.pending = null;
  }

  open(preview: DeepLinkPreview): void {
    this.pending = preview;
  }

  dismiss(): void {
    this.pending = null;
  }
}

export const deepLink = new DeepLinkStore();

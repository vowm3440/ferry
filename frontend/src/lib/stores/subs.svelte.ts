/*
 * 订阅。列表、导入与更新结果都以后端为准。
 * 粘贴的文本、订阅地址与二维码内容可能含凭据：只转交给后端一次，绝不保存在这里。
 */

import {
  EVENT_SUBS_PROGRESS,
  SUB_PHASE_DONE,
  SUB_PHASE_ERROR,
  messageOfError,
  parseSubsProgress,
  type Backend,
  type Subscription,
  type Unsubscribe,
} from '../api';
import { tr } from '../i18n/index.svelte';
import { SUCCESS, failure, failureMessage, type ActionResult } from '../result';
import { lanShare } from './lan-share.svelte';
import { nodes } from './nodes.svelte';

export type ImportOutcome = { readonly ok: true; readonly subscription: Subscription } | { readonly ok: false; readonly message: string };

const msgOffline = (): string => tr('未连接到桌面后端', 'Not connected to desktop backend');
const msgImportBusy = (): string => tr('上一个导入还在进行中，请稍候', 'Import in progress; please wait');
const msgBusy = (): string => tr('上一个操作还在进行中，请稍候', 'Previous operation in progress; please wait');

class SubsStore {
  list = $state.raw<Subscription[]>([]);
  loaded = $state(false);
  loading = $state(false);
  /** 读取失败的说明。 */
  error = $state('');
  importing = $state(false);
  /** 正在删除的订阅 id。 */
  removing = $state('');
  /** 正在改名或修改自动更新的订阅 id。 */
  saving = $state('');
  /** 由界面发起、尚未返回的更新。 */
  updating = $state.raw<readonly string[]>([]);
  updatingAll = $state(false);
  /** subs:progress 报告的阶段：订阅 id → fetch / parse / apply。结束后即移除。 */
  progress = $state.raw<Readonly<Record<string, string>>>({});

  #backend: Backend | null = null;
  #offs: Unsubscribe[] = [];

  attach(backend: Backend): Promise<void> {
    this.detach();
    this.#backend = backend;
    // 局域网分享只接上后端，不发起任何调用：要等用户打开分享对话框。
    lanShare.attach(backend);
    this.#offs = [backend.on(EVENT_SUBS_PROGRESS, (payload) => this.#onProgress(payload))];
    return this.refresh();
  }

  detach(): void {
    for (const off of this.#offs) off();
    this.#offs = [];
    this.#backend = null;
    lanShare.detach();
    this.list = [];
    this.loaded = false;
    this.loading = false;
    this.error = '';
    this.importing = false;
    this.removing = '';
    this.saving = '';
    this.updating = [];
    this.updatingAll = false;
    this.progress = {};
  }

  isUpdating(id: string): boolean {
    return this.updatingAll || this.updating.includes(id) || Object.hasOwn(this.progress, id);
  }

  async refresh(): Promise<void> {
    const backend = this.#backend;
    if (backend === null) return;
    this.loading = true;
    try {
      const list = await backend.subsList();
      if (this.#backend !== backend) return;
      this.list = list;
      this.error = '';
      this.loaded = true;
    } catch (error: unknown) {
      if (this.#backend === backend) this.error = messageOfError(error);
    } finally {
      if (this.#backend === backend) this.loading = false;
    }
  }

  /** 导入粘贴的分享链接。 */
  import(name: string, text: string): Promise<ImportOutcome> {
    return this.#import((backend) => backend.subsAddFromText(name, text));
  }

  /** 按地址添加订阅。地址只交给后端，之后界面只能拿到打码后的 urlMasked。 */
  importUrl(name: string, url: string, autoUpdate: boolean): Promise<ImportOutcome> {
    return this.#import((backend) => backend.subsAddFromURL(name, url, autoUpdate));
  }

  /** 导入本机文件：path 来自系统文件选择框，由后端读取。 */
  importFile(path: string): Promise<ImportOutcome> {
    return this.#import((backend) => backend.subsAddFromFile(path));
  }

  /** 导入二维码图片（Base64）。后端解出的若是订阅地址会拒绝，并提示改用地址导入。 */
  importQr(encoded: string): Promise<ImportOutcome> {
    return this.#import(async (backend) => {
      const [first] = await backend.subsImportFromQR(encoded);
      if (first === undefined) throw new Error(tr('二维码里没有可导入的节点', 'No importable nodes found in QR code'));
      return first;
    });
  }

  /** 截取当前屏幕识别二维码并导入；调用前界面必须先确认。 */
  captureScreenQr(): Promise<ImportOutcome> {
    return this.#import(async (backend) => {
      const [first] = await backend.subsCaptureScreenQR();
      if (first === undefined) throw new Error(tr('屏幕上未找到可导入的二维码节点', 'No QR code with importable nodes found on screen'));
      return first;
    });
  }

  rename(id: string, name: string): Promise<ActionResult> {
    return this.#save(id, (backend) => backend.subsRename(id, name));
  }

  setAutoUpdate(id: string, on: boolean, hours: number): Promise<ActionResult> {
    return this.#save(id, (backend) => backend.subsSetAutoUpdate(id, on, hours));
  }

  setUserAgent(id: string, ua: string): Promise<ActionResult> {
    return this.#save(id, (backend) => backend.subsSetUserAgent(id, ua));
  }

  /** 立即更新一个订阅。内核运行中也可以，后端会在应用后重启内核。 */
  async update(id: string): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(msgOffline());
    if (this.isUpdating(id)) return failureMessage(msgBusy());

    this.updating = [...this.updating, id];
    try {
      await backend.subsUpdate(id);
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      if (this.#backend === backend) {
        this.updating = this.updating.filter((item) => item !== id);
        this.#clearProgress(id);
        await this.#reload();
      }
    }
  }

  /** 更新全部订阅；有失败时后端把各自的原因逐行合在一起返回。 */
  async updateAll(): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(msgOffline());
    if (this.updatingAll || this.updating.length > 0) return failureMessage(msgBusy());

    this.updatingAll = true;
    try {
      await backend.subsUpdateAll();
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      if (this.#backend === backend) {
        this.updatingAll = false;
        this.progress = {};
        await this.#reload();
      }
    }
  }

  async remove(id: string): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(msgOffline());
    if (this.removing !== '') return failureMessage(msgBusy());

    this.removing = id;
    try {
      await backend.subsRemove(id);
      await this.#reload();
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.removing = '';
    }
  }

  async #import(run: (backend: Backend) => Promise<Subscription>): Promise<ImportOutcome> {
    const backend = this.#backend;
    if (backend === null) return { ok: false, message: msgOffline() };
    if (this.importing) return { ok: false, message: msgImportBusy() };

    this.importing = true;
    try {
      const subscription = await run(backend);
      await this.#reload();
      return { ok: true, subscription };
    } catch (error: unknown) {
      return { ok: false, message: messageOfError(error) };
    } finally {
      this.importing = false;
    }
  }

  async #save(id: string, run: (backend: Backend) => Promise<void>): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(msgOffline());
    if (this.saving !== '') return failureMessage(msgBusy());

    this.saving = id;
    try {
      await run(backend);
      await this.refresh();
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.saving = '';
    }
  }

  /** 订阅变了，节点也跟着变：两边一起重读。 */
  async #reload(): Promise<void> {
    await Promise.all([this.refresh(), nodes.refresh()]);
  }

  #clearProgress(id: string): void {
    if (!Object.hasOwn(this.progress, id)) return;
    this.progress = Object.fromEntries(Object.entries(this.progress).filter(([key]) => key !== id));
  }

  #onProgress(payload: unknown): void {
    const event = parseSubsProgress(payload);
    if (event.id === '') return;
    const finished = event.phase === SUB_PHASE_DONE || event.phase === SUB_PHASE_ERROR;
    if (!finished) {
      this.progress = { ...this.progress, [event.id]: event.phase };
      return;
    }
    this.#clearProgress(event.id);
    // 界面发起的更新在调用返回时统一重读；这里只处理后台定时更新。
    if (!this.updatingAll && !this.updating.includes(event.id)) void this.#reload();
  }
}

export const subs = new SubsStore();

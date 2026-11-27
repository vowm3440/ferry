/*
 * 设置。取值以 Settings.Get() 为准；界面编辑的是页面内的草稿，
 * 保存时整体提交给 Settings.Set()，后端拒绝时原样展示它给的说明。
 *
 * 设置是否保存到磁盘由后端决定（能力 settings.persist），界面按能力如实说明。
 */

import { cloneSettings, messageOfError, type Backend, type SettingsValue } from '../api';
import { i18n, tr } from '../i18n/index.svelte';
import { SUCCESS, failure, failureMessage, type ActionResult } from '../result';

class SettingsStore {
  value = $state.raw<SettingsValue | null>(null);
  loading = $state(false);
  saving = $state(false);
  /** 读取失败的说明。 */
  error = $state('');

  #backend: Backend | null = null;

  attach(backend: Backend): Promise<void> {
    this.#backend = backend;
    return this.load();
  }

  detach(): void {
    this.#backend = null;
    this.value = null;
    this.loading = false;
    this.saving = false;
    this.error = '';
  }

  async load(): Promise<void> {
    const backend = this.#backend;
    if (backend === null) return;
    this.loading = true;
    try {
      const value = await backend.settingsGet();
      if (this.#backend !== backend) return;
      this.value = value;
      i18n.syncFromBackend(value.language);
      this.error = '';
    } catch (error: unknown) {
      if (this.#backend === backend) this.error = messageOfError(error);
    } finally {
      if (this.#backend === backend) this.loading = false;
    }
  }

  async save(next: SettingsValue): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(tr('未连接到桌面后端', 'Not connected to desktop backend'));
    if (this.saving) return failureMessage(tr('正在保存，请稍候', 'Saving; please wait'));

    this.saving = true;
    try {
      await backend.settingsSet(cloneSettings(next));
      await this.load();
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      this.saving = false;
    }
  }
}

export const settings = new SettingsStore();

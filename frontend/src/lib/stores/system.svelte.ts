/*
 * 系统代理、系统服务、TUN、备份与诊断。
 *
 * 每一项状态只在后端公布对应能力时才调用，不猜测、不越权调用未公布接口。
 */

import { untrack } from 'svelte';
import {
  CAP_DEEPLINK_REGISTER,
  CAP_SERVICE,
  CAP_SYS_PROXY,
  CAP_TUN,
  messageOfError,
  type Backend,
  type ServiceStatus,
  type SysProxyStatus,
  type TunStatus,
} from '../api';
import { tr } from '../i18n/index.svelte';
import { failure, failureMessage, SUCCESS, type ActionResult } from '../result';
import { caps } from './caps.svelte';
import { core } from './core.svelte';

/** 正在进行的文件操作；同一时间只做一件。 */
export type SystemTask = '' | 'backup' | 'restore' | 'diagnostics';
export type ServiceAction = '' | 'install' | 'uninstall' | 'repair';

const notAttached = (): string => tr('未连接到桌面后端', 'Not connected to desktop backend');
const busyMsg = (): string => tr('上一个操作还在进行中，请稍候', 'Previous operation in progress; please wait');

class SystemStore {
  /** 系统代理状态；没有这项能力或尚未读到时为 null。 */
  proxy = $state.raw<SysProxyStatus | null>(null);
  proxyError = $state('');
  proxyBusy = $state(false);
  service = $state.raw<ServiceStatus | null>(null);
  serviceAction = $state<ServiceAction>('');
  tun = $state.raw<TunStatus | null>(null);
  tunBusy = $state(false);
  deepLinkRegistered = $state(false);
  deepLinkBusy = $state(false);
  task = $state<SystemTask>('');

  #backend: Backend | null = null;
  #stopWatch: (() => void) | null = null;
  #reads = 0;

  get attached(): boolean {
    return this.#backend !== null;
  }

  async attach(backend: Backend): Promise<void> {
    this.detach();
    this.#backend = backend;
    this.#stopWatch = $effect.root(() => {
      $effect(() => {
        const hasProxy = caps.has(CAP_SYS_PROXY);
        const hasService = caps.has(CAP_SERVICE);
        const hasTun = caps.has(CAP_TUN);
        const hasDeepLinkReg = caps.has(CAP_DEEPLINK_REGISTER);
        void core.state?.status;
        if (hasProxy || hasService || hasTun || hasDeepLinkReg) untrack(() => void this.refresh());
      });
    });
  }

  async refresh(): Promise<void> {
    const backend = this.#backend;
    if (backend === null) return;
    this.#reads += 1;
    const read = this.#reads;
    const tasks: Promise<void>[] = [];
    if (caps.has(CAP_SYS_PROXY)) {
      tasks.push(
        backend
          .sysProxyStatus()
          .then((status) => {
            if (this.#backend === backend && read === this.#reads) {
              this.proxy = status;
              this.proxyError = '';
            }
          })
          .catch((error: unknown) => {
            if (this.#backend === backend && read === this.#reads) {
              this.proxy = null;
              this.proxyError = messageOfError(error);
            }
          }),
      );
    }
    if (caps.has(CAP_SERVICE)) {
      tasks.push(
        backend
          .serviceStatus()
          .then((status) => {
            if (this.#backend === backend && read === this.#reads) this.service = status;
          })
          .catch(() => {
            if (this.#backend === backend && read === this.#reads) this.service = null;
          }),
      );
    }
    if (caps.has(CAP_TUN)) {
      tasks.push(
        backend
          .tunStatus()
          .then((status) => {
            if (this.#backend === backend && read === this.#reads) this.tun = status;
          })
          .catch(() => {
            if (this.#backend === backend && read === this.#reads) this.tun = null;
          }),
      );
    }
    if (caps.has(CAP_DEEPLINK_REGISTER)) {
      tasks.push(
        backend
          .deepLinkStatus()
          .then((registered) => {
            if (this.#backend === backend && read === this.#reads) this.deepLinkRegistered = registered;
          })
          .catch(() => {
            if (this.#backend === backend && read === this.#reads) this.deepLinkRegistered = false;
          }),
      );
    }
    await Promise.all(tasks);
  }

  async setSysProxy(on: boolean): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(notAttached());
    if (this.proxyBusy) return failureMessage(busyMsg());
    this.proxyBusy = true;
    let result = SUCCESS;
    try {
      await backend.setSysProxy(on);
    } catch (error: unknown) {
      result = failure(error);
    }
    if (this.#backend !== backend) return result;
    await this.refresh();
    if (this.#backend === backend) this.proxyBusy = false;
    return result;
  }

  async runServiceAction(action: Exclude<ServiceAction, ''>): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(notAttached());
    if (this.serviceAction !== '') return failureMessage(busyMsg());
    this.serviceAction = action;
    let result = SUCCESS;
    try {
      if (action === 'install') await backend.installService();
      else if (action === 'uninstall') await backend.uninstallService();
      else await backend.repairService();
    } catch (error: unknown) {
      result = failure(error);
    }
    if (this.#backend !== backend) return result;
    await Promise.all([this.refresh(), core.refresh()]);
    if (this.#backend === backend) this.serviceAction = '';
    return result;
  }

  async setTun(on: boolean): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(notAttached());
    if (this.tunBusy) return failureMessage(busyMsg());
    this.tunBusy = true;
    let result = SUCCESS;
    try {
      await backend.setTun(on);
    } catch (error: unknown) {
      result = failure(error);
    }
    if (this.#backend !== backend) return result;
    await Promise.all([this.refresh(), core.refresh()]);
    if (this.#backend === backend) this.tunBusy = false;
    return result;
  }

  async setDeepLinkRegistered(on: boolean): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(notAttached());
    if (this.deepLinkBusy) return failureMessage(busyMsg());
    this.deepLinkBusy = true;
    let result = SUCCESS;
    try {
      await backend.setDeepLinkRegistered(on);
    } catch (error: unknown) {
      result = failure(error);
    }
    if (this.#backend !== backend) return result;
    await this.refresh();
    if (this.#backend === backend) this.deepLinkBusy = false;
    return result;
  }

  exportBackup(path: string): Promise<ActionResult> {
    return this.#run('backup', (backend) => backend.exportBackup(path));
  }

  restoreBackup(path: string): Promise<ActionResult> {
    return this.#run('restore', (backend) => backend.restoreBackup(path));
  }

  exportDiagnostics(path: string): Promise<ActionResult> {
    return this.#run('diagnostics', (backend) => backend.exportDiagnostics(path));
  }

  detach(): void {
    this.#stopWatch?.();
    this.#stopWatch = null;
    this.#backend = null;
    this.#reads += 1;
    this.proxy = null;
    this.proxyError = '';
    this.proxyBusy = false;
    this.service = null;
    this.serviceAction = '';
    this.tun = null;
    this.tunBusy = false;
    this.deepLinkRegistered = false;
    this.deepLinkBusy = false;
    this.task = '';
  }

  async #run(task: Exclude<SystemTask, ''>, action: (backend: Backend) => Promise<void>): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(notAttached());
    if (this.task !== '') return failureMessage(busyMsg());
    this.task = task;
    try {
      await action(backend);
      return SUCCESS;
    } catch (error: unknown) {
      return failure(error);
    } finally {
      if (this.#backend === backend) this.task = '';
    }
  }
}

export const system = new SystemStore();

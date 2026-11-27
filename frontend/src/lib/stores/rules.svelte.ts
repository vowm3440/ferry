/*
 * 规则、应用分组与规则集。
 *
 * 数据只来自后端；每次改动成功后后端会推送 rules:applied，这里据此重新读取。
 * 改动会让运行中的内核重建，所以同一时间只允许一个改动（pending）。
 * 后端没有公布对应能力时不读取、也不调用（见 caps）。
 */

import { untrack } from 'svelte';
import {
  CAP_APP_GROUPS,
  CAP_RULES,
  CAP_RULE_SETS,
  EVENT_RULES_APPLIED,
  messageOfError,
  type AppGroup,
  type Backend,
  type ImportPreview,
  type Rule,
  type RuleSetInfo,
  type RuleTestResult,
  type Unsubscribe,
} from '../api';
import { tr } from '../i18n/index.svelte';
import { SUCCESS, failureMessage, type ActionResult } from '../result';
import { moveRule } from '../rules-view';
import { caps } from './caps.svelte';

/** 进行中的改动；空串表示空闲。 */
export type RulesPending = '' | 'save' | 'remove' | 'toggle' | 'move' | 'import' | 'group' | 'sets';

/** 带返回值的读取结果。 */
export type Outcome<T> = { readonly ok: true; readonly value: T } | { readonly ok: false; readonly message: string };

const notConnected = (): string => tr('未连接到桌面后端', 'Not connected to desktop backend');
const busyMsg = (): string => tr('上一个操作还在进行中，请稍候', 'Previous operation in progress; please wait');

class RulesStore {
  list = $state.raw<readonly Rule[]>([]);
  groups = $state.raw<readonly AppGroup[]>([]);
  sets = $state.raw<readonly RuleSetInfo[]>([]);
  loaded = $state(false);
  loading = $state(false);
  /** 读取规则失败的说明。 */
  error = $state('');
  groupsError = $state('');
  setsLoaded = $state(false);
  setsLoading = $state(false);
  setsError = $state('');
  pending = $state<RulesPending>('');
  /** 进行中的改动针对的规则、分组 id（用于行内的进行中状态）。 */
  pendingId = $state('');
  busy = $derived(this.pending !== '');

  #backend: Backend | null = null;
  #offs: Unsubscribe[] = [];
  #seq = 0;

  get attached(): boolean {
    return this.#backend !== null;
  }

  async attach(backend: Backend): Promise<void> {
    this.detach();
    this.#backend = backend;
    this.#offs = [
      backend.on(EVENT_RULES_APPLIED, () => {
        // 自己发起的改动结束时会重新读取，不必读两遍。
        if (this.pending === '') void this.refresh();
      }),
      // 能力列表与本仓库同时挂接，读到之后（或重试成功之后）再读取规则。
      $effect.root(() => {
        $effect(() => {
          if (caps.has(CAP_RULES)) untrack(() => void this.refresh());
        });
      }),
    ];
  }

  detach(): void {
    for (const off of this.#offs) off();
    this.#offs = [];
    this.#backend = null;
    this.#seq += 1;
    this.list = [];
    this.groups = [];
    this.sets = [];
    this.loaded = false;
    this.loading = false;
    this.error = '';
    this.groupsError = '';
    this.setsLoaded = false;
    this.setsLoading = false;
    this.setsError = '';
    this.pending = '';
    this.pendingId = '';
  }

  /** 重新读取规则与应用分组；规则集只在读过之后才跟着刷新。 */
  async refresh(): Promise<void> {
    const backend = this.#backend;
    if (backend === null || !caps.has(CAP_RULES)) return;
    const seq = (this.#seq += 1);
    const current = (): boolean => this.#backend === backend && this.#seq === seq;
    this.loading = true;
    try {
      const list = await backend.rulesList();
      if (!current()) return;
      this.list = list;
      this.error = '';
      this.loaded = true;
    } catch (error: unknown) {
      if (current()) this.error = messageOfError(error);
    }
    if (current() && caps.has(CAP_APP_GROUPS)) {
      try {
        const groups = await backend.appGroupsList();
        if (!current()) return;
        this.groups = groups;
        this.groupsError = '';
      } catch (error: unknown) {
        if (current()) this.groupsError = messageOfError(error);
      }
    }
    if (current()) this.loading = false;
  }

  /** 读取规则集状态。只读本地缓存的元数据，不会触发下载。 */
  async loadSets(refresh: boolean): Promise<void> {
    const backend = this.#backend;
    if (backend === null || !caps.has(CAP_RULE_SETS)) return;
    this.setsLoading = true;
    try {
      const sets = await backend.ruleSetsList(refresh);
      if (this.#backend !== backend) return;
      this.sets = sets;
      this.setsError = '';
      this.setsLoaded = true;
    } catch (error: unknown) {
      if (this.#backend === backend) this.setsError = messageOfError(error);
    } finally {
      if (this.#backend === backend) this.setsLoading = false;
    }
  }

  async #mutate(kind: RulesPending, id: string, action: (backend: Backend) => Promise<void>): Promise<ActionResult> {
    const backend = this.#backend;
    if (backend === null) return failureMessage(notConnected());
    if (this.pending !== '') return failureMessage(busyMsg());
    this.pending = kind;
    this.pendingId = id;
    let result: ActionResult = SUCCESS;
    try {
      await action(backend);
    } catch (error: unknown) {
      result = failureMessage(messageOfError(error));
    }
    if (this.#backend !== backend) return result;
    // 失败后也重新读取：列表可能已经被别处改过（例如「规则不存在」）。
    await (kind === 'sets' ? this.loadSets(true) : this.refresh());
    if (this.#backend === backend) {
      this.pending = '';
      this.pendingId = '';
    }
    return result;
  }

  /** 新建（id 为空）或修改一条规则。 */
  save(rule: Rule): Promise<ActionResult> {
    return this.#mutate('save', rule.id, async (backend) => {
      await backend.rulesSave(rule);
    });
  }

  remove(id: string): Promise<ActionResult> {
    return this.#mutate('remove', id, (backend) => backend.rulesRemove(id));
  }

  setEnabled(id: string, on: boolean): Promise<ActionResult> {
    return this.#mutate('toggle', id, (backend) => backend.rulesSetEnabled(id, on));
  }

  /** 上移（-1）或下移（1）一位。 */
  async move(id: string, delta: -1 | 1): Promise<ActionResult> {
    const ids = moveRule(this.list, id, delta);
    if (ids === null) return failureMessage(tr('这条规则不能再移动了', 'This rule cannot be moved further'));
    return this.#mutate('move', id, (backend) => backend.rulesReorder(ids));
  }

  /**
   * 把导入预览里的规则一次写入：后端整批校验、只应用一次（运行中的内核只重启一次），
   * 有一条不通过就一条都不写。预览里的临时 id 不会被沿用。
   */
  saveImported(items: readonly Rule[]): Promise<ActionResult> {
    return this.#mutate('import', '', (backend) => backend.rulesSaveMany(items));
  }

  /** 解析规则文档，只生成预览，不写入任何东西。 */
  async preview(kind: string, content: string): Promise<Outcome<ImportPreview>> {
    const backend = this.#backend;
    if (backend === null) return { ok: false, message: notConnected() };
    try {
      return { ok: true, value: await backend.rulesImport(kind, content) };
    } catch (error: unknown) {
      return { ok: false, message: messageOfError(error) };
    }
  }

  /** 查看一个连接会命中哪条规则。不改动任何东西。 */
  async test(application: string, host: string, port: number, network: string): Promise<Outcome<RuleTestResult>> {
    const backend = this.#backend;
    if (backend === null) return { ok: false, message: notConnected() };
    try {
      return { ok: true, value: await backend.rulesTest(application, host, port, network) };
    } catch (error: unknown) {
      return { ok: false, message: messageOfError(error) };
    }
  }

  /** 新建（id 为空）或修改一个应用分组。 */
  saveGroup(group: AppGroup): Promise<ActionResult> {
    return this.#mutate('group', group.id, async (backend) => {
      await backend.appGroupsSave(group);
    });
  }

  removeGroup(id: string): Promise<ActionResult> {
    return this.#mutate('group', id, (backend) => backend.appGroupsRemove(id));
  }

  /** 下载并更新规则集（1 到 4 个）。更新后的内容在重新应用规则或重启内核后生效。 */
  updateSets(ids: readonly string[]): Promise<ActionResult> {
    return this.#mutate('sets', '', (backend) => backend.ruleSetsUpdate([...ids]));
  }
}

export const rules = new RulesStore();

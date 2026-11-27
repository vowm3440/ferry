/* 动作结果：成功，或带一条可直接展示给用户的中文说明。 */

import { messageOfError } from './api';

export type ActionResult = { readonly ok: true } | { readonly ok: false; readonly message: string };

export const SUCCESS: ActionResult = { ok: true };

export function failure(error: unknown): ActionResult {
  return { ok: false, message: messageOfError(error) };
}

export function failureMessage(message: string): ActionResult {
  return { ok: false, message };
}

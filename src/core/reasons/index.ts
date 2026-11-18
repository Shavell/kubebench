import { i18n, registerTranslations } from '@/i18n'
import { localeBundles } from '@/i18n/bundles'
import type { ReasonParams, ReasonRef } from '@/data/types'
import { localizeReason, type ReasonMessage, type ReasonTranslator } from './localize'

export { isReasonMessage, type ReasonMessage } from './localize'

/**
 * Translations of the backend's reason codes (design §3.3): every
 * explanation KubeBench gives itself (an error, why a plugin or a
 * capability is unavailable, why a live-update topic is polled, why the
 * update check failed) comes with a stable code and parameters, e.g.
 * `{"reason": "cluster.read_only", "params": {"cluster": "prod"}}`. The
 * `reasons` namespace holds one string per code (keys are the codes);
 * a backend test checks that every code the server defines has a string
 * in every language, and that English equals the server's own text.
 */
export const REASONS_NS = 'reasons'

const reasonLocales = localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' }))

registerTranslations(REASONS_NS, reasonLocales)

const translator = (lng?: string): ReasonTranslator => ({
  has: (code) => i18n.exists(code, { ns: REASONS_NS, lng }),
  // `replace` passes the values for interpolation only, so a parameter
  // named like an i18next option (count, context, …) stays a value.
  translate: (code, params) => i18n.t(code, { ns: REASONS_NS, lng, replace: params }),
})

/**
 * The reason in the UI language; a code the UI does not know (from a newer
 * server or an external plugin) shows the server's English fallback.
 */
export function reasonText(
  code: string | null | undefined,
  params: ReasonParams | null | undefined,
  fallback: string,
): string {
  return localizeReason(translator(), code, params, fallback)
}

/** The server's English text of a reason, e.g. as a mock's error message. */
export function englishReason(code: string, params?: ReasonParams): string {
  return localizeReason(translator('en'), code, params, code)
}

/**
 * A response's advice in the UI language: `warnings` (the server's English
 * texts) translated by `warningReasons`, index by index.
 */
export function warningTexts(warnings: readonly string[] | undefined, reasons?: readonly ReasonRef[]): string[] {
  return (warnings ?? []).map((text, i) => reasonText(reasons?.[i]?.reason, reasons?.[i]?.params, text))
}

/** The server's warnings for `reasons` in English, e.g. as a mock's response. */
export function englishWarnings(reasons: ReasonRef[]): { warnings: string[]; warningReasons: ReasonRef[] } {
  return { warnings: reasons.map((r) => englishReason(r.reason, r.params)), warningReasons: reasons }
}

/** A ReasonMessage (e.g. why the server ended a stream) in the UI language. */
export function reasonMessageText(message: ReasonMessage): string {
  return reasonText(message.reason, message.params, message.error)
}

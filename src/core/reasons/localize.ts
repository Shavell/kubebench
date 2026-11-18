/**
 * Picking the text of a server reason (design §3.3), kept free of i18next
 * so it can be unit-tested: the UI string of a known code with the
 * server's parameters, else the server's own English message.
 */

export interface ReasonTranslator {
  /** Whether the UI has a string for the code. */
  has: (code: string) => boolean
  /** The UI string of a known code with params filled in. */
  translate: (code: string, params: Record<string, string>) => string
}

/**
 * An explanation on its own (the server's reason.Message): the English text
 * with the reason code and parameters, if it has them. Streams end with one,
 * and it can be the parameter of another reason (`{{cause}}`).
 */
export interface ReasonMessage {
  error: string
  reason?: string
  params?: Record<string, unknown>
}

/** Whether value is a ReasonMessage. */
export function isReasonMessage(value: unknown): value is ReasonMessage {
  return typeof value === 'object' && value !== null && typeof (value as { error?: unknown }).error === 'string'
}

/**
 * Parameter values as text: numbers and strings as they are, a nested
 * ReasonMessage translated with `translator` when given (else its English
 * text), anything else as JSON.
 */
export function reasonParamText(
  params: Record<string, unknown> | null | undefined,
  translator?: ReasonTranslator,
): Record<string, string> {
  const out: Record<string, string> = {}
  for (const [name, value] of Object.entries(params ?? {})) {
    if (typeof value === 'string') out[name] = value
    else if (typeof value === 'number') out[name] = String(value)
    else if (isReasonMessage(value)) {
      out[name] = translator ? localizeReason(translator, value.reason, value.params, value.error) : value.error
    } else out[name] = JSON.stringify(value)
  }
  return out
}

/**
 * The text of a reason: its translation when the code is known, else the
 * fallback (the server's English message). Codes the UI does not know yet,
 * from a newer server or a plugin, therefore still read well.
 */
export function localizeReason(
  translator: ReasonTranslator,
  code: string | null | undefined,
  params: Record<string, unknown> | null | undefined,
  fallback: string,
): string {
  if (!code || !translator.has(code)) return fallback
  return translator.translate(code, reasonParamText(params, translator))
}

import { PLATFORM_NS } from '@/core/locales'
import { i18n } from '@/i18n'

/** One row of a KeyValueEditor (labels, annotations, env vars, Helm --set values, …). */
export interface KeyValuePair {
  key: string
  value: string
}

/** Rows for a map such as `metadata.labels`, sorted by key. */
export function pairsFromRecord(record: Record<string, string> | null | undefined): KeyValuePair[] {
  return Object.entries(record ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => ({ key, value }))
}

/** The map the rows describe. Rows without a key are left out. */
export function recordFromPairs(pairs: readonly KeyValuePair[]): Record<string, string> {
  const record: Record<string, string> = {}
  for (const { key, value } of pairs) {
    const name = key.trim()
    if (name) record[name] = value
  }
  return record
}

/**
 * The translated problem of each row, or null when the row is fine: a value
 * without a key, a key used twice, or whatever `validateKey` reports.
 * Callers block submitting while any entry is non-null.
 */
export function keyValueErrors(
  pairs: readonly KeyValuePair[],
  validateKey?: (key: string) => string | null,
): (string | null)[] {
  const counts = new Map<string, number>()
  for (const { key } of pairs) {
    const name = key.trim()
    if (name) counts.set(name, (counts.get(name) ?? 0) + 1)
  }
  return pairs.map(({ key, value }) => {
    const name = key.trim()
    if (!name) return value ? i18n.t('ui.keyValue.keyRequired', { ns: PLATFORM_NS }) : null
    if ((counts.get(name) ?? 0) > 1) return i18n.t('ui.keyValue.duplicate', { ns: PLATFORM_NS })
    return validateKey?.(name) ?? null
  })
}

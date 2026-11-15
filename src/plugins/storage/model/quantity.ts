/**
 * Kubernetes resource quantities ("10Gi", "500M", "1.5e9") as far as
 * storage sizes need them: parsing to bytes for comparisons and the form's
 * number + unit split.
 */

const QUANTITY = /^([+-]?(?:\d+\.?\d*|\.\d+))(Ki|Mi|Gi|Ti|Pi|Ei|m|k|M|G|T|P|E|[eE][+-]?\d+)?$/

const MULTIPLIERS: Record<string, number> = {
  '': 1,
  m: 1e-3,
  k: 1e3,
  M: 1e6,
  G: 1e9,
  T: 1e12,
  P: 1e15,
  E: 1e18,
  Ki: 2 ** 10,
  Mi: 2 ** 20,
  Gi: 2 ** 30,
  Ti: 2 ** 40,
  Pi: 2 ** 50,
  Ei: 2 ** 60,
}

/** Bytes a quantity stands for (rounded up like the API server), or null when it is not one. */
export function quantityBytes(text: string): number | null {
  const match = QUANTITY.exec(text.trim())
  if (!match) return null
  const [, number, suffix = ''] = match
  const factor = /^[eE]/.test(suffix) ? 10 ** Number(suffix.slice(1)) : MULTIPLIERS[suffix]
  return Math.ceil(Number(number) * factor)
}

/** Units offered next to a size field. */
export const SIZE_UNITS = ['Mi', 'Gi', 'Ti'] as const
export type SizeUnit = (typeof SIZE_UNITS)[number]

/** "10Gi" from the form's amount and unit; '' while the amount is empty. */
export function sizeText(amount: string, unit: SizeUnit): string {
  const trimmed = amount.trim()
  return trimmed ? `${trimmed}${unit}` : ''
}

/**
 * The form's amount and unit for a quantity. A quantity without a form
 * unit ("10G") is rounded: 'up' to whole Gi where the size must be at
 * least the quantity (a clone's or restore's source), 'down' to whole Mi
 * (Gi when exact) where it must be at most the quantity (binding to a
 * volume of that capacity) or must not exceed it unasked (an expansion's
 * starting value).
 */
export function splitSize(text: string, rounding: 'up' | 'down' = 'up'): { amount: string; unit: SizeUnit } {
  const match = /^(\d+(?:\.\d+)?)(Mi|Gi|Ti)$/.exec(text)
  if (match) return { amount: match[1], unit: match[2] as SizeUnit }
  const bytes = quantityBytes(text)
  if (bytes === null) return { amount: '', unit: 'Gi' }
  if (rounding === 'up') return { amount: String(Math.ceil(bytes / 2 ** 30)), unit: 'Gi' }
  const mi = Math.floor(bytes / 2 ** 20)
  return mi % 1024 === 0 ? { amount: String(mi / 1024), unit: 'Gi' } : { amount: String(mi), unit: 'Mi' }
}

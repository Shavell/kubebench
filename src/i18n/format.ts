import { browserLanguages, i18n, matchLanguage } from './index'
import { regionalLocale } from './languageMatch'

/** Locale-aware formatting helpers bound to the active UI language. */

/**
 * The locale numbers and dates are formatted in: the browser's regional
 * variant of the UI language when it lists one (UI "de" with a de-AT browser
 * → "de-AT"), else the UI language.
 */
export function formatLocale(): string {
  return regionalLocale(i18n.language, browserLanguages(), matchLanguage)
}

export function formatNumber(value: number, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(formatLocale(), options).format(value)
}

export function formatPercent(fraction: number, maximumFractionDigits = 0): string {
  return new Intl.NumberFormat(formatLocale(), {
    style: 'percent',
    maximumFractionDigits,
  }).format(fraction)
}

export function formatDateTime(value: string | number | Date, options?: Intl.DateTimeFormatOptions): string {
  const date = value instanceof Date ? value : new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  return new Intl.DateTimeFormat(
    formatLocale(),
    options ?? { dateStyle: 'medium', timeStyle: 'short' },
  ).format(date)
}

const BYTE_UNITS = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB']

/** 1536 → "1.5 KiB" (binary units, locale-aware digits). */
export function formatBytes(bytes: number, fractionDigits = 1): string {
  if (!Number.isFinite(bytes)) return '—'
  let value = Math.abs(bytes)
  let unit = 0
  while (value >= 1024 && unit < BYTE_UNITS.length - 1) {
    value /= 1024
    unit++
  }
  const sign = bytes < 0 ? '-' : ''
  return `${sign}${formatNumber(value, { maximumFractionDigits: unit === 0 ? 0 : fractionDigits })} ${BYTE_UNITS[unit]}`
}

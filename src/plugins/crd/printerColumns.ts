import type { CrdDefinition, CustomResource, PrinterColumn } from '../../data/types.ts'

/**
 * The printer columns of a CRD's instance list, in the CRD's order: before the
 * CRD is known, the keys the rows have. Like `kubectl get`, columns of
 * priority > 0 only with `wide` (-o wide).
 */
export function printerColumns(
  crd: CrdDefinition | undefined,
  rows: CustomResource[],
  wide: boolean,
): { shown: PrinterColumn[]; hasWide: boolean } {
  const all: PrinterColumn[] =
    crd?.printerColumns ??
    [...new Set(rows.flatMap((r) => Object.keys(r.extra)))].map((name) => ({ name, type: 'string', priority: 0 }))
  return {
    shown: all.filter((c) => wide || c.priority === 0),
    hasWide: all.some((c) => c.priority > 0),
  }
}

/**
 * Sort key of a printer column value: numbers numerically (missing ones
 * first), dates (shown as ages, e.g. "3h") not at all, the rest as text.
 */
export function printerSortValue(col: PrinterColumn): ((value: string | undefined) => string | number) | undefined {
  if (col.type === 'date') return undefined
  if (col.type === 'integer' || col.type === 'number') {
    return (value) => {
      const n = Number(value)
      return value === undefined || Number.isNaN(n) ? Number.NEGATIVE_INFINITY : n
    }
  }
  return (value) => value ?? ''
}

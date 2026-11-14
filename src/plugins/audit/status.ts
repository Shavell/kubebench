import type { BadgeTone } from '@/shared/Badge'

/** Badge tone of an HTTP status in the audit log: refusals stand out from failures. */
export function statusTone(status: number): BadgeTone {
  if (status >= 500) return 'error'
  if (status >= 400) return 'warning'
  return 'success'
}

/** Codicon for an audit row in compact lists. */
export function statusIcon(status: number): string {
  if (status >= 500) return 'error'
  if (status >= 400) return 'circle-slash'
  return 'pass'
}

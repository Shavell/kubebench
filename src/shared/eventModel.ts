import type { KubeObject } from '../data/types.ts'

/**
 * Cluster-wide Kubernetes events (core/v1 Event objects from the generic
 * list) reduced to what the Events view and the overview's recent warnings
 * show. Pure functions, unit tested in eventModel.test.ts.
 */
export interface EventRow {
  /** Stable row key: namespace/name of the Event object. */
  key: string
  name: string
  namespace: string
  /** "Normal" or "Warning" (Kubernetes values, never translated). */
  type: string
  reason: string
  message: string
  /** Occurrences (`count`, or the event series' count; at least 1). */
  count: number
  /** The object the event is about, e.g. "Pod/web-1". */
  object: string
  /** Minutes since the event was last seen (Infinity when unknown). */
  lastSeenMinutes: number
  source: KubeObject
}

export interface EventGroup {
  reason: string
  /** "Warning" when any event of the group is a warning. */
  type: string
  events: EventRow[]
  /** Sum of the events' counts. */
  occurrences: number
  lastSeenMinutes: number
}

type Obj = Record<string, unknown>

const field = (o: unknown, key: string): unknown =>
  o !== null && typeof o === 'object' ? (o as Obj)[key] : undefined
const text = (v: unknown): string => (typeof v === 'string' ? v : '')

/** "5s" | "26m" | "23h" | "92d" | "2y1mo" → minutes (mock objects carry such ages). */
export function ageToMinutes(age: string): number {
  let total = 0
  let matched = false
  for (const [, num, unit] of age.matchAll(/(\d+)(mo|[smhdy])/g)) {
    const factors: Record<string, number> = { s: 1 / 60, m: 1, h: 60, d: 1440, mo: 43200, y: 525600 }
    total += Number(num) * (factors[unit] ?? 0)
    matched = true
  }
  return matched ? total : Infinity
}

/** Minutes → the compact age kubectl prints ("45s", "12m", "5h", "3d"). */
export function formatAge(minutes: number): string {
  if (!Number.isFinite(minutes)) return '—'
  if (minutes < 1) return `${Math.max(0, Math.floor(minutes * 60))}s`
  if (minutes < 60) return `${Math.floor(minutes)}m`
  if (minutes < 48 * 60) return `${Math.floor(minutes / 60)}h`
  return `${Math.floor(minutes / 1440)}d`
}

/**
 * When the event was last seen: `lastTimestamp`, else the events.k8s.io
 * series or `eventTime`, else the object's creation; mock events carry a
 * synthetic `age` instead.
 */
function lastSeenMinutes(o: KubeObject, now: number): number {
  if (typeof o.age === 'string') return ageToMinutes(o.age)
  const stamp =
    text(o.lastTimestamp) ||
    text(field(o.series, 'lastObservedTime')) ||
    text(o.eventTime) ||
    text(o.firstTimestamp) ||
    text(o.metadata?.creationTimestamp)
  const at = stamp ? Date.parse(stamp) : NaN
  return Number.isNaN(at) ? Infinity : Math.max(0, (now - at) / 60000)
}

export function toEventRow(o: KubeObject, now: number): EventRow {
  const involved = o.involvedObject
  const kind = text(field(involved, 'kind'))
  const objectName = text(field(involved, 'name'))
  const count = Number(o.count ?? field(o.series, 'count') ?? 1)
  const name = o.metadata?.name ?? ''
  const namespace = o.metadata?.namespace ?? ''
  return {
    key: `${namespace}/${name}`,
    name,
    namespace,
    type: text(o.type) || 'Normal',
    reason: text(o.reason) || '—',
    message: text(o.message),
    count: Number.isFinite(count) && count > 0 ? count : 1,
    object: kind && objectName ? `${kind}/${objectName}` : objectName,
    lastSeenMinutes: lastSeenMinutes(o, now),
    source: o,
  }
}

export const isWarning = (row: EventRow) => row.type === 'Warning'

/** Most recent first. */
export function byLastSeen(a: EventRow, b: EventRow): number {
  return a.lastSeenMinutes - b.lastSeenMinutes || a.key.localeCompare(b.key)
}

/**
 * Events grouped by reason: groups with warnings first, then by
 * occurrences and reason; events inside a group most recent first.
 */
export function groupByReason(rows: EventRow[]): EventGroup[] {
  const groups = new Map<string, EventRow[]>()
  for (const row of rows) {
    const list = groups.get(row.reason)
    if (list) list.push(row)
    else groups.set(row.reason, [row])
  }
  return [...groups.entries()]
    .map(([reason, events]): EventGroup => {
      const sorted = [...events].sort(byLastSeen)
      return {
        reason,
        type: sorted.some(isWarning) ? 'Warning' : 'Normal',
        events: sorted,
        occurrences: sorted.reduce((sum, e) => sum + e.count, 0),
        lastSeenMinutes: sorted[0].lastSeenMinutes,
      }
    })
    .sort(
      (a, b) =>
        Number(b.type === 'Warning') - Number(a.type === 'Warning') ||
        b.occurrences - a.occurrences ||
        a.reason.localeCompare(b.reason),
    )
}

/** The `limit` most recently seen warnings. */
export function recentWarnings(rows: EventRow[], limit: number): EventRow[] {
  return rows.filter(isWarning).sort(byLastSeen).slice(0, limit)
}

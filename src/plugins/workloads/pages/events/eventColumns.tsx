import { Badge } from '@/shared/Badge'
import { formatAge, type EventRow } from '@/shared/eventModel'
import type { ColumnSpec } from '../../resourceCatalog'

// Event types (Normal/Warning) are Kubernetes values and stay untranslated.
const typeColumn: ColumnSpec<EventRow> = {
  key: 'type',
  render: (e) => <Badge tone={e.type === 'Warning' ? 'warning' : 'neutral'}>{e.type}</Badge>,
  sortValue: (e) => e.type,
}

const tail: ColumnSpec<EventRow>[] = [
  { key: 'object', render: (e) => e.object || '—', sortValue: (e) => e.object },
  { key: 'namespace', render: (e) => e.namespace || '—', sortValue: (e) => e.namespace },
  {
    key: 'message',
    render: (e) => <span title={e.message}>{e.message}</span>,
    width: '40%',
  },
  { key: 'count', render: (e) => String(e.count), sortValue: (e) => e.count },
  { key: 'lastSeen', render: (e) => formatAge(e.lastSeenMinutes), sortValue: (e) => e.lastSeenMinutes },
]

/** Columns of the flat Events list; headers are `columns.<key>`. */
export const eventColumns: ColumnSpec<EventRow>[] = [
  typeColumn,
  { key: 'reason', render: (e) => e.reason, sortValue: (e) => e.reason },
  ...tail,
]

/** Columns inside a reason group, where the reason is the group's heading. */
export const groupedEventColumns: ColumnSpec<EventRow>[] = [typeColumn, ...tail]

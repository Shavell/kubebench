import { useProviderQuery } from '@/data/DataContext'
import type { K8sEvent } from '@/data/types'
import { Icon } from './Icon'
import './EventsTable.css'

interface EventsTableProps {
  kind: string
  namespace: string | null
  name: string
}

/** Kubernetes events for one object, newest first — reused by every detail page. */
export function EventsTable({ kind, namespace, name }: EventsTableProps) {
  const { data, loading } = useProviderQuery(
    (p) => p.cluster.listEvents(kind, namespace, name),
    [kind, namespace, name],
  )

  if (loading) return <div className="kb-events__empty">Loading events…</div>
  const events = data ?? []
  if (events.length === 0) return <div className="kb-events__empty">No events</div>

  return (
    <table className="kb-table kb-events">
      <thead>
        <tr>
          <th>Type</th>
          <th>Reason</th>
          <th>Message</th>
          <th>Count</th>
          <th>Age</th>
          <th>Source</th>
        </tr>
      </thead>
      <tbody>
        {events.map((event: K8sEvent, i) => (
          <tr key={i}>
            <td>
              <span className={event.type === 'Warning' ? 'kb-events__warning' : 'kb-events__normal'}>
                <Icon name={event.type === 'Warning' ? 'warning' : 'info'} size={13} /> {event.type}
              </span>
            </td>
            <td>{event.reason}</td>
            <td className="kb-events__message" title={event.message}>
              {event.message}
            </td>
            <td>{event.count > 1 ? `×${event.count}` : '1'}</td>
            <td>{event.age}</td>
            <td>{event.source}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
  const { data, loading, error } = useProviderQuery(
    (p) => p.cluster.listEvents(kind, namespace, name),
    [kind, namespace, name],
  )

  if (loading) return <div className="kb-events__empty">{t('events.loading')}</div>
  // A failed request must not pass for "no events".
  if (error) return <div className="kb-events__empty">{t('events.loadFailed', { error })}</div>
  const events = data ?? []
  if (events.length === 0) return <div className="kb-events__empty">{t('events.empty')}</div>

  return (
    <table className="kb-table kb-events">
      <thead>
        <tr>
          <th>{t('common.type')}</th>
          <th>{t('common.reason')}</th>
          <th>{t('common.message')}</th>
          <th>{t('common.count')}</th>
          <th>{t('common.age')}</th>
          <th>{t('common.source')}</th>
        </tr>
      </thead>
      <tbody>
        {events.map((event: K8sEvent, i) => (
          <tr key={i}>
            <td>
              {/* Event types (Normal/Warning) are Kubernetes values and stay untranslated. */}
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

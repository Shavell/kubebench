import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { AuditEntry } from '@/data/types'
import { formatDateTime } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Page } from '@/shared/Page'
import { Table, type Column } from '@/shared/Table'
import { Unavailable } from '@/shared/Unavailable'
import { AuditFiltersBar } from './AuditFiltersBar'
import { AUDIT_NS } from './namespace'
import { statusTone } from './status'
import { useAuditLog, type AuditFilters } from './useAuditLog'
import { useDebouncedValue } from './useDebouncedValue'
import './audit.css'

const NO_FILTERS: AuditFilters = { user: '', cluster: '', action: '' }

/**
 * The audit log (admin): who changed what, on which cluster, and whether
 * it was allowed (design §3.4). Newest first; "Load more" pages back.
 */
export function AuditPage() {
  const { t } = useTranslation(AUDIT_NS)
  const [filters, setFilters] = useState(NO_FILTERS)
  // Typing into the action filter queries once the user pauses.
  const query = useDebouncedValue(filters, 300)
  const log = useAuditLog(query)
  const filtered = Boolean(query.user || query.cluster || query.action)

  const columns: Column<AuditEntry>[] = [
    {
      key: 'time',
      header: t('columns.time'),
      render: (e) => (
        <time dateTime={e.ts} className="audit__time">
          {formatDateTime(e.ts, { dateStyle: 'short', timeStyle: 'medium' })}
        </time>
      ),
    },
    { key: 'user', header: t('columns.user'), render: (e) => e.user },
    {
      key: 'cluster',
      header: t('columns.cluster'),
      render: (e) => e.cluster || <span className="audit__muted">{t('table.noCluster')}</span>,
    },
    {
      key: 'action',
      header: t('columns.action'),
      render: (e) => (
        <code className="audit__code audit__code--action" title={e.action}>
          {e.action}
        </code>
      ),
    },
    {
      key: 'target',
      header: t('columns.target'),
      render: (e) => (
        <code className="audit__code audit__code--target" title={e.target}>
          {e.target}
        </code>
      ),
    },
    {
      key: 'status',
      header: t('columns.status'),
      render: (e) => <Badge tone={statusTone(e.status)}>{String(e.status)}</Badge>,
    },
    {
      key: 'detail',
      header: t('columns.detail'),
      render: (e) => (
        <span className="audit__detail" title={e.detail}>
          {e.detail}
        </span>
      ),
    },
  ]

  const renderBody = () => {
    if (log.items.length === 0) {
      if (log.error) return <Unavailable error={log.error} onRetry={log.reload} />
      if (log.loading) return <div className="audit__loading">{t('table.loading')}</div>
      return (
        <EmptyState
          icon="history"
          title={filtered ? t('table.noMatches') : t('table.empty')}
          hint={filtered ? t('table.noMatchesHint') : t('table.emptyHint')}
        />
      )
    }
    return (
      <>
        <Table columns={columns} rows={log.items} rowKey={(e) => String(e.id)} />
        <div className="audit__footer">
          <span className="audit__muted" aria-live="polite">
            {t('table.shown', { count: log.items.length })}
          </span>
          {log.nextBefore !== null && (
            <Button variant="secondary" icon="fold-down" busy={log.loading} onClick={log.loadMore}>
              {t('table.loadMore')}
            </Button>
          )}
        </div>
        <ErrorNotice error={log.error} />
      </>
    )
  }

  return (
    <Page
      title={t('page.title')}
      actions={
        <Button variant="secondary" icon="refresh" disabled={log.loading} onClick={log.reload}>
          {t('actions.refresh')}
        </Button>
      }
    >
      <AuditFiltersBar filters={filters} onChange={setFilters} />
      {renderBody()}
    </Page>
  )
}

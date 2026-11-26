import { useMemo, useState } from 'react'
import { usePluginQuery } from '@/data/pluginData'
import { formatDateTime } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { securityData } from '../data'
import type { AcceptedRisk } from '../data/types'
import { UnacceptDialog, type RiskTarget } from './AcceptDialogs'
import { ResourceLink, WriteButton } from './parts'
import { useSecurityTexts } from './texts'
import '@/shared/tabPage.css'
import '../security.css'

/**
 * Every accepted risk of the cluster, including expired ones and those
 * whose object a scan no longer finds, so stale entries can be removed.
 */
export function AcceptedPage() {
  const texts = useSecurityTexts()
  const { t } = texts
  const query = usePluginQuery(() => securityData().accepted(), [])
  const [removing, setRemoving] = useState<RiskTarget | null>(null)

  const columns = useMemo<Column<AcceptedRisk>[]>(
    () => [
      {
        key: 'check',
        header: t('columns.check'),
        sortValue: (r) => texts.title(r.checkId, r.checkId),
        render: (r) => texts.title(r.checkId, r.checkId),
      },
      { key: 'resource', header: t('columns.resource'), render: (r) => <ResourceLink resource={r.resource} /> },
      { key: 'reason', header: t('columns.reason'), render: (r) => <span className="security-wrap">{r.reason}</span> },
      { key: 'by', header: t('columns.acceptedBy'), sortValue: (r) => r.by, render: (r) => r.by },
      { key: 'at', header: t('columns.acceptedAt'), sortValue: (r) => r.at, render: (r) => formatDateTime(r.at) },
      {
        key: 'expires',
        header: t('columns.expires'),
        sortValue: (r) => r.expiresAt ?? '~',
        render: (r) =>
          r.expired ? (
            <Badge tone="warning">{t('accepted.expired')}</Badge>
          ) : r.expiresAt ? (
            formatDateTime(r.expiresAt)
          ) : (
            <span className="security-muted">{t('accepted.never')}</span>
          ),
      },
      {
        key: 'actions',
        header: '',
        // Shrinks to the button, whose label length depends on the language.
        width: '1px',
        render: (r) => (
          <WriteButton
            label={t('actions.unaccept')}
            onClick={() => setRemoving({ key: r.key, title: texts.title(r.checkId, r.checkId), resource: r.resource })}
          />
        ),
      },
    ],
    [t, texts],
  )

  return (
    <div className="page">
      <Toolbar title={t('accepted.title')} />
      {query.loading && !query.data ? (
        <div className="page-loading">{t('accepted.loading')}</div>
      ) : query.error ? (
        <Unavailable code={query.errorCode} reasonCode={query.errorReason} reason={query.error} title={t('accepted.unavailable')} />
      ) : !query.data?.length ? (
        <EmptyState icon="pass" title={t('accepted.empty')} hint={t('accepted.emptyHint')} />
      ) : (
        <div className="page__scroll">
          <Table columns={columns} rows={query.data} rowKey={(r) => r.key} />
        </div>
      )}
      {removing && <UnacceptDialog target={removing} onClose={() => setRemoving(null)} />}
    </div>
  )
}

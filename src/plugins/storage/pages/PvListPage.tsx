import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { usePluginQuery } from '@/data/pluginData'
import { Badge } from '@/shared/Badge'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { storageData } from '../data'
import type { PvSummary } from '../data/types'
import { NS } from '../i18n'
import { openPvcWizard, openPvWizard, openResource } from '../tabs'
import { WriteButton } from './CreateButton'
import '@/shared/tabPage.css'
import '../storage.css'

/** PersistentVolumes with their source and claim; Available ones can be claimed directly. */
export function PvListPage() {
  const { t } = useTranslation(NS)
  const query = usePluginQuery(() => storageData().listPvs(), [])

  const columns = useMemo<Column<PvSummary>[]>(
    () => [
      { key: 'name', header: t('columns.name'), sortValue: (v) => v.name, render: (v) => v.name },
      {
        key: 'status',
        header: t('columns.status'),
        sortValue: (v) => v.phase,
        render: (v) => (
          <span title={v.reason}>
            <Badge tone={v.phase === 'Available' ? 'info' : undefined}>{v.phase}</Badge>
          </span>
        ),
      },
      { key: 'capacity', header: t('columns.capacity'), sortValue: (v) => v.capacityBytes, render: (v) => v.capacity || '—' },
      { key: 'modes', header: t('columns.accessModes'), render: (v) => v.accessModes.join(', ') },
      { key: 'reclaim', header: t('columns.reclaimPolicy'), render: (v) => v.reclaimPolicy },
      { key: 'class', header: t('columns.storageClass'), render: (v) => v.storageClass || '—' },
      {
        key: 'source',
        header: t('columns.source'),
        render: (v) => (
          <>
            {t(`sources.${v.source}`)}
            {v.sourceDetail && <span className="storage-mono storage-muted"> {v.sourceDetail}</span>}
          </>
        ),
      },
      { key: 'claim', header: t('columns.claim'), render: (v) => (v.claim ? `${v.claim.namespace}/${v.claim.name}` : '—') },
      { key: 'nodes', header: t('columns.nodes'), render: (v) => v.nodes.join(', ') || '—' },
      { key: 'age', header: t('columns.age'), render: (v) => v.age },
      {
        key: 'actions',
        header: '',
        render: (v) =>
          v.phase === 'Available' && !v.claim ? (
            <div className="storage-row-actions">
              <WriteButton
                label={t('actions.claim')}
                icon="link"
                onClick={() => openPvcWizard({ volumeName: v.name, storageClassName: v.storageClass })}
              />
            </div>
          ) : null,
      },
    ],
    [t],
  )

  const body = () => {
    if (query.loading && !query.data) return <div className="page-loading">{t('pvs.loading')}</div>
    if (query.error) return <Unavailable code={query.errorCode} reason={query.error} title={t('pvs.unavailable')} />
    const rows = query.data ?? []
    if (rows.length === 0) return <EmptyState icon="save-all" title={t('pvs.empty')} />
    return (
      <div className="page__scroll">
        <Table columns={columns} rows={rows} rowKey={(v) => v.name} onRowClick={(v) => openResource('PersistentVolume', undefined, v.name)} />
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={t('pvs.title')}>
        <WriteButton label={t('actions.newPv')} icon="add" variant="primary" onClick={openPvWizard} />
      </Toolbar>
      {body()}
    </div>
  )
}

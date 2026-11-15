import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { usePluginQuery } from '@/data/pluginData'
import { Badge } from '@/shared/Badge'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { storageData } from '../data'
import type { VolumeSnapshot } from '../data/types'
import { NS } from '../i18n'
import { openPvcWizard } from '../tabs'
import { WriteButton } from './CreateButton'
import '@/shared/tabPage.css'
import '../storage.css'

/**
 * VolumeSnapshots of the selected namespace; each can be restored into a
 * new claim. Unavailable (with the reason) when the snapshot CRDs are missing.
 */
export function SnapshotListPage() {
  const { t } = useTranslation(NS)
  const namespace = useContextStore((s) => s.namespace)
  const ns = namespace === ALL_NAMESPACES ? undefined : namespace
  const query = usePluginQuery(() => storageData().listSnapshots(ns), [ns], {
    topics: ['volumesnapshots.snapshot.storage.k8s.io'],
    namespace: ns,
  })

  const columns = useMemo<Column<VolumeSnapshot>[]>(
    () => [
      { key: 'name', header: t('columns.name'), sortValue: (s) => s.name, render: (s) => s.name },
      { key: 'ns', header: t('columns.namespace'), sortValue: (s) => s.namespace, render: (s) => s.namespace },
      {
        key: 'ready',
        header: t('columns.status'),
        render: (s) =>
          s.error ? (
            <span title={s.error}>
              <Badge tone="error">{t('snapshots.failed')}</Badge>
            </span>
          ) : (
            <Badge tone={s.readyToUse ? 'success' : 'warning'}>{s.readyToUse ? t('snapshots.ready') : t('snapshots.notReady')}</Badge>
          ),
      },
      { key: 'source', header: t('columns.sourcePvc'), render: (s) => s.sourcePVC || s.sourceContent || '—' },
      { key: 'class', header: t('columns.snapshotClass'), render: (s) => s.snapshotClass || '—' },
      { key: 'size', header: t('columns.restoreSize'), sortValue: (s) => s.restoreSizeBytes, render: (s) => s.restoreSize || '—' },
      { key: 'age', header: t('columns.age'), render: (s) => s.age },
      {
        key: 'actions',
        header: '',
        render: (s) => (
          <div className="storage-row-actions">
            <WriteButton
              label={t('actions.restore')}
              icon="history"
              onClick={() => openPvcWizard({ namespace: s.namespace, dataSource: { kind: 'VolumeSnapshot', name: s.name } })}
            />
          </div>
        ),
      },
    ],
    [t],
  )

  const body = () => {
    if (query.loading && !query.data) return <div className="page-loading">{t('snapshots.loading')}</div>
    if (query.error) return <Unavailable code={query.errorCode} reason={query.error} title={t('snapshots.unavailable')} />
    const rows = query.data ?? []
    if (rows.length === 0) return <EmptyState icon="history" title={t('snapshots.empty')} />
    return (
      <div className="page__scroll">
        <Table columns={columns} rows={rows} rowKey={(s) => `${s.namespace}/${s.name}`} />
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={ns ? t('snapshots.titleIn', { namespace: ns }) : t('snapshots.title')} />
      {body()}
    </div>
  )
}

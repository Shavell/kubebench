import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { usePluginQuery } from '@/data/pluginData'
import { Badge } from '@/shared/Badge'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { storageData } from '../data'
import type { PvcSummary } from '../data/types'
import { NS } from '../i18n'
import { openPvcWizard, openResource } from '../tabs'
import { WriteButton } from './CreateButton'
import { ExpandPvcDialog } from './ExpandPvcDialog'
import '@/shared/tabPage.css'
import '../storage.css'

/** PersistentVolumeClaims of the selected namespace, with Expand and Clone on every row. */
export function PvcListPage() {
  const { t } = useTranslation(NS)
  const namespace = useContextStore((s) => s.namespace)
  const ns = namespace === ALL_NAMESPACES ? undefined : namespace
  const query = usePluginQuery(() => storageData().listPvcs(ns), [ns], {
    topics: ['persistentvolumeclaims'],
    namespace: ns,
  })
  const [expanding, setExpanding] = useState<PvcSummary | null>(null)

  const columns = useMemo<Column<PvcSummary>[]>(
    () => [
      { key: 'name', header: t('columns.name'), sortValue: (p) => p.name, render: (p) => p.name },
      { key: 'ns', header: t('columns.namespace'), sortValue: (p) => p.namespace, render: (p) => p.namespace },
      {
        key: 'status',
        header: t('columns.status'),
        sortValue: (p) => p.phase,
        render: (p) => (
          <>
            <Badge>{p.phase}</Badge>
            {p.resizing && <span className="storage-muted"> {t('pvcs.resizing')}</span>}
          </>
        ),
      },
      { key: 'class', header: t('columns.storageClass'), render: (p) => p.storageClass || '—' },
      { key: 'requested', header: t('columns.requested'), sortValue: (p) => p.requestedBytes, render: (p) => p.requested || '—' },
      { key: 'capacity', header: t('columns.capacity'), sortValue: (p) => p.capacityBytes, render: (p) => p.capacity || '—' },
      { key: 'modes', header: t('columns.accessModes'), render: (p) => p.accessModes.join(', ') },
      { key: 'volume', header: t('columns.volume'), render: (p) => <span className="storage-mono">{p.volumeName || '—'}</span> },
      { key: 'age', header: t('columns.age'), render: (p) => p.age },
      {
        key: 'actions',
        header: '',
        render: (p) => (
          <div className="storage-row-actions">
            <WriteButton
              label={t('actions.expand')}
              icon="arrow-both"
              disabledReason={p.expandBlocker ? t(`expand.blocked.${p.expandBlocker}`) : null}
              onClick={() => setExpanding(p)}
            />
            <WriteButton
              label={t('actions.clone')}
              icon="copy"
              onClick={() =>
                openPvcWizard({
                  namespace: p.namespace,
                  storageClassName: p.storageClass,
                  dataSource: { kind: 'PersistentVolumeClaim', name: p.name },
                })
              }
            />
          </div>
        ),
      },
    ],
    [t],
  )

  const body = () => {
    if (query.loading && !query.data) return <div className="page-loading">{t('pvcs.loading')}</div>
    if (query.error) return <Unavailable code={query.errorCode} reasonCode={query.errorReason} reason={query.error} title={t('pvcs.unavailable')} />
    const rows = query.data ?? []
    if (rows.length === 0) return <EmptyState icon="save" title={t('pvcs.empty')} hint={ns ? t('pvcs.emptyNamespaceHint', { namespace: ns }) : undefined} />
    return (
      <div className="page__scroll">
        <Table
          columns={columns}
          rows={rows}
          rowKey={(p) => `${p.namespace}/${p.name}`}
          onRowClick={(p) => openResource('PersistentVolumeClaim', p.namespace, p.name)}
        />
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={ns ? t('pvcs.titleIn', { namespace: ns }) : t('pvcs.title')}>
        <WriteButton label={t('actions.newPvc')} icon="add" variant="primary" onClick={() => openPvcWizard({ namespace: ns })} />
      </Toolbar>
      {body()}
      {expanding && <ExpandPvcDialog pvc={expanding} onClose={() => setExpanding(null)} />}
    </div>
  )
}

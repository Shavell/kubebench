import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useProviderQuery } from '@/data/DataContext'
import type { CrdDefinition } from '@/data/types'
import { CORE_NS } from '@/i18n'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { NS } from '../i18n'
import { openInstancesTab } from '../tabs'
import '@/shared/tabPage.css'

export function CrdListPage() {
  const { t } = useTranslation(NS)
  const { data, loading, error } = useProviderQuery((p) => p.crd.listCrds(), [], { topics: ['customresourcedefinitions.apiextensions.k8s.io'] })

  const columns = useMemo<Column<CrdDefinition>[]>(
    () => [
      { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (c) => c.name, sortValue: (c) => c.name },
      { key: 'group', header: t('definitions.columns.group'), render: (c) => c.group, sortValue: (c) => c.group },
      { key: 'kind', header: t('definitions.columns.kind'), render: (c) => c.kind },
      { key: 'versions', header: t('definitions.columns.versions'), render: (c) => c.versions.join(', ') },
      { key: 'scope', header: t('definitions.columns.scope'), render: (c) => c.scope },
      {
        key: 'short',
        header: t('definitions.columns.shortNames'),
        render: (c) => c.shortNames.join(', ') || '—',
      },
      { key: 'age', header: t('common.age', { ns: CORE_NS }), render: (c) => c.age },
    ],
    [t],
  )

  if (loading) return <div className="page-loading">{t('definitions.loading')}</div>

  return (
    <div className="page">
      <Toolbar title={t('definitions.title')} />
      {error ? (
        <EmptyState icon="warning" title={t('definitions.unavailable')} hint={error} />
      ) : (
        <div className="page__scroll">
          <Table
            columns={columns}
            rows={data ?? []}
            rowKey={(c) => c.name}
            onRowClick={(c) => openInstancesTab(c.name, c.kind)}
          />
        </div>
      )}
    </div>
  )
}

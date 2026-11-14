import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useProviderQuery } from '@/data/DataContext'
import type { HelmRelease } from '@/data/types'
import { CORE_NS } from '@/i18n'
import { Badge } from '@/shared/Badge'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { NS } from '../i18n'
import { openReleaseTab } from '../tabs'
import '@/shared/tabPage.css'

export function ReleaseListPage() {
  const { t } = useTranslation(NS)
  const { data, loading, error } = useProviderQuery((p) => p.helm.listReleases(), [])

  const columns = useMemo<Column<HelmRelease>[]>(
    () => [
      { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (r) => r.name, sortValue: (r) => r.name },
      {
        key: 'ns',
        header: t('common.namespace', { ns: CORE_NS }),
        render: (r) => r.namespace,
        sortValue: (r) => r.namespace,
      },
      { key: 'chart', header: t('releases.columns.chart'), render: (r) => `${r.chart}-${r.chartVersion}` },
      { key: 'app', header: t('releases.columns.appVersion'), render: (r) => r.appVersion },
      {
        key: 'rev',
        header: t('releases.columns.revision'),
        render: (r) => String(r.revision),
        sortValue: (r) => r.revision,
      },
      {
        key: 'status',
        header: t('common.status', { ns: CORE_NS }),
        render: (r) => <Badge>{r.status}</Badge>,
        sortValue: (r) => r.status,
      },
      { key: 'updated', header: t('releases.columns.updated'), render: (r) => r.updated, sortValue: (r) => r.updated },
    ],
    [t],
  )

  if (loading) return <div className="page-loading">{t('releases.loading')}</div>

  return (
    <div className="page">
      <Toolbar title={t('releases.title')} />
      {error ? (
        <EmptyState icon="warning" title={t('releases.unavailable')} hint={error} />
      ) : (
        <div className="page__scroll">
          <Table
            columns={columns}
            rows={data ?? []}
            rowKey={(r) => `${r.namespace}/${r.name}`}
            onRowClick={(r) => openReleaseTab(r.name)}
          />
        </div>
      )}
    </div>
  )
}

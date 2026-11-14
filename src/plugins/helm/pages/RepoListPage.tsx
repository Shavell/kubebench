import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useProviderQuery } from '@/data/DataContext'
import type { HelmRepo } from '@/data/types'
import { CORE_NS } from '@/i18n'
import { formatNumber } from '@/i18n/format'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { NS } from '../i18n'
import '@/shared/tabPage.css'

export function RepoListPage() {
  const { t } = useTranslation(NS)
  const { data, loading, error } = useProviderQuery((p) => p.helm.listRepositories(), [])

  const columns = useMemo<Column<HelmRepo>[]>(
    () => [
      { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (r) => r.name, sortValue: (r) => r.name },
      { key: 'url', header: t('repos.columns.url'), render: (r) => r.url },
      {
        key: 'charts',
        header: t('repos.columns.charts'),
        render: (r) => formatNumber(r.chartCount),
        sortValue: (r) => r.chartCount,
      },
    ],
    [t],
  )

  if (loading) return <div className="page-loading">{t('repos.loading')}</div>

  return (
    <div className="page">
      <Toolbar title={t('repos.title')} />
      {error ? (
        <EmptyState icon="warning" title={t('repos.unavailable')} hint={error} />
      ) : (
        <div className="page__scroll">
          <Table columns={columns} rows={data ?? []} rowKey={(r) => r.name} />
        </div>
      )}
    </div>
  )
}

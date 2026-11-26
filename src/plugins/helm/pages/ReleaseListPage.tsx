import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { refreshData } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { CORE_NS } from '@/i18n'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { helmData, type HelmRelease } from '../data'
import { NS } from '../i18n'
import { openInstallTab, openReleaseTab, openReposTab } from '../tabs'
import { DateTime } from '../DateTime'
import '@/shared/tabPage.css'

export function ReleaseListPage() {
  const { t } = useTranslation(NS)
  const namespace = useContextStore((s) => s.namespace)
  const scope = namespace === ALL_NAMESPACES ? undefined : namespace
  const write = usePermission('write')
  const [search, setSearch] = useState('')
  const query = usePluginQuery(() => helmData().listReleases(scope), [scope], {
    topics: ['secrets'],
    namespace: scope,
  })

  const columns = useMemo<Column<HelmRelease>[]>(
    () => [
      { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (r) => r.name, sortValue: (r) => r.name },
      {
        key: 'ns',
        header: t('common.namespace', { ns: CORE_NS }),
        render: (r) => r.namespace,
        sortValue: (r) => r.namespace,
      },
      {
        key: 'chart',
        header: t('releases.columns.chart'),
        render: (r) => `${r.chart}-${r.chartVersion}`,
        sortValue: (r) => r.chart,
      },
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
      {
        key: 'updated',
        header: t('releases.columns.updated'),
        render: (r) => <DateTime value={r.updated} />,
        sortValue: (r) => r.updated,
      },
    ],
    [t],
  )

  const all = query.data ?? []
  const needle = search.trim().toLowerCase()
  const rows = needle
    ? all.filter((r) => [r.name, r.namespace, r.chart].some((field) => field.toLowerCase().includes(needle)))
    : all

  return (
    <div className="page">
      <Toolbar title={t('releases.title')}>
        <ListFilterBar
          search={search}
          onSearch={setSearch}
          summary={query.data ? t('releases.summary', { shown: rows.length, total: all.length }) : undefined}
        />
        <Button
          variant="secondary"
          icon="refresh"
          onClick={refreshData}
          title={t('common.refresh', { ns: CORE_NS })}
          aria-label={t('common.refresh', { ns: CORE_NS })}
        />
        <Button variant="secondary" icon="repo" onClick={openReposTab}>
          {t('releases.repositories')}
        </Button>
        <Button icon="cloud-download" disabled={!write.allowed} title={write.reason ?? undefined} onClick={openInstallTab}>
          {t('releases.install')}
        </Button>
      </Toolbar>
      {query.error ? (
        <Unavailable code={query.errorCode} reasonCode={query.errorReason} reason={query.error} title={t('releases.unavailable')} onRetry={refreshData} />
      ) : query.loading && !query.data ? (
        <div className="page-loading">{t('releases.loading')}</div>
      ) : all.length === 0 ? (
        <EmptyState
          icon="package"
          title={scope ? t('releases.emptyIn', { namespace: scope }) : t('releases.empty')}
          hint={t('releases.emptyHint')}
        />
      ) : (
        <div className="page__scroll">
          <Table
            columns={columns}
            rows={rows}
            rowKey={(r) => `${r.namespace}/${r.name}`}
            onRowClick={(r) => openReleaseTab(r.namespace, r.name)}
          />
        </div>
      )}
    </div>
  )
}

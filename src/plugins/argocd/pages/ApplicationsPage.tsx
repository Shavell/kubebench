import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { refreshData } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { CORE_NS } from '@/i18n'
import { formatDateTime } from '@/i18n/format'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { Page, PageSection } from '@/shared/Page'
import { Table, type Column } from '@/shared/Table'
import { Unavailable } from '@/shared/Unavailable'
import { StateBadge } from '../components/StatusBadges'
import { argoCd } from '../data'
import type { ArgoApplicationSummary } from '../data/types'
import { NS } from '../i18n'
import { openApplicationTab } from '../tabs'
import { destinationText, sourceText } from '../text'
import '@/shared/tabPage.css'
import '../argocd.css'

/** Applications grouped by project, with their sync, health and operation state. */
export function ApplicationsPage() {
  const { t } = useTranslation(NS)
  const [search, setSearch] = useState('')
  const query = usePluginQuery(() => argoCd().listApplications(), [], { topics: ['applications.argoproj.io'] })

  const columns = useMemo<Column<ArgoApplicationSummary>[]>(
    () => [
      { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (a) => a.name, sortValue: (a) => a.name },
      { key: 'sync', header: t('columns.sync'), render: (a) => <StateBadge value={a.sync} />, sortValue: (a) => a.sync },
      { key: 'health', header: t('columns.health'), render: (a) => <StateBadge value={a.health} />, sortValue: (a) => a.health },
      {
        key: 'operation',
        header: t('columns.operation'),
        render: (a) => <StateBadge value={a.operationPhase} />,
        sortValue: (a) => a.operationPhase,
      },
      { key: 'source', header: t('columns.source'), render: (a) => <span className="argocd-mono argocd-source" title={sourceText(a)}>{sourceText(a)}</span> },
      { key: 'destination', header: t('columns.destination'), render: (a) => destinationText(a.destination) },
      {
        key: 'auto',
        header: t('columns.autoSync'),
        render: (a) => (a.autoSync ? t('common.yes', { ns: CORE_NS }) : t('common.no', { ns: CORE_NS })),
      },
      {
        key: 'synced',
        header: t('columns.lastSynced'),
        render: (a) => (a.lastSyncedAt ? formatDateTime(a.lastSyncedAt) : '—'),
        sortValue: (a) => a.lastSyncedAt,
      },
    ],
    [t],
  )

  const groups = useMemo(() => {
    const needle = search.trim().toLowerCase()
    const byProject = new Map<string, ArgoApplicationSummary[]>()
    for (const app of query.data ?? []) {
      if (needle && !`${app.name} ${app.namespace} ${app.repo} ${app.path} ${app.chart}`.toLowerCase().includes(needle)) continue
      byProject.set(app.project, [...(byProject.get(app.project) ?? []), app])
    }
    return [...byProject.entries()].sort(([a], [b]) => a.localeCompare(b))
  }, [query.data, search])

  const total = query.data?.length ?? 0
  const shown = groups.reduce((n, [, apps]) => n + apps.length, 0)

  const body = () => {
    if (query.error) {
      return <Unavailable code={query.errorCode} reason={query.error} title={t('applications.unavailable')} onRetry={refreshData} />
    }
    if (!query.data) return <div className="page-loading">{t('applications.loading')}</div>
    if (total === 0) return <EmptyState icon="layers" title={t('applications.empty')} hint={t('applications.emptyHint')} />
    if (shown === 0) return <EmptyState icon="search" title={t('applications.noMatch')} />
    return groups.map(([project, apps]) => (
      <PageSection key={project} title={t('applications.project', { project, count: apps.length })}>
        <Table
          columns={columns}
          rows={apps}
          rowKey={(a) => `${a.namespace}/${a.name}`}
          onRowClick={(a) => openApplicationTab(a.namespace, a.name)}
        />
      </PageSection>
    ))
  }

  return (
    <Page
      title={t('applications.title')}
      className="argocd-wide"
      actions={
        <>
          <ListFilterBar
            search={search}
            onSearch={setSearch}
            summary={query.data ? t('applications.summary', { shown, total }) : undefined}
          />
          <Button variant="secondary" icon="refresh" onClick={refreshData} title={t('actions.reload')}>
            {t('actions.reload')}
          </Button>
        </>
      }
    >
      {body()}
    </Page>
  )
}

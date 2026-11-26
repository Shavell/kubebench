import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { refreshData } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { CORE_NS } from '@/i18n'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { Page } from '@/shared/Page'
import { Table, type Column } from '@/shared/Table'
import { Unavailable } from '@/shared/Unavailable'
import { argoCd } from '../data'
import type { ArgoProject } from '../data/types'
import { NS } from '../i18n'
import { destinationText } from '../text'
import '@/shared/tabPage.css'
import '../argocd.css'

const list = (items: string[]) => (items.length ? items.join(', ') : '—')

/** AppProjects: what their applications may deploy, from where and to where. */
export function ProjectsPage() {
  const { t } = useTranslation(NS)
  const projects = usePluginQuery(() => argoCd().listProjects(), [], { topics: ['appprojects.argoproj.io'] })
  const apps = usePluginQuery(() => argoCd().listApplications(), [], { topics: ['applications.argoproj.io'] })

  const appCounts = useMemo(() => {
    const counts = new Map<string, number>()
    for (const app of apps.data ?? []) counts.set(app.project, (counts.get(app.project) ?? 0) + 1)
    return counts
  }, [apps.data])

  const columns = useMemo<Column<ArgoProject>[]>(
    () => [
      { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (p) => p.name, sortValue: (p) => p.name },
      { key: 'namespace', header: t('common.namespace', { ns: CORE_NS }), render: (p) => p.namespace },
      { key: 'description', header: t('common.description', { ns: CORE_NS }), render: (p) => p.description || '—' },
      {
        key: 'apps',
        header: t('projects.columns.applications'),
        render: (p) => (apps.data ? String(appCounts.get(p.name) ?? 0) : '…'),
        sortValue: (p) => appCounts.get(p.name) ?? 0,
      },
      { key: 'repos', header: t('projects.columns.sourceRepos'), render: (p) => <span className="argocd-mono">{list(p.sourceRepos)}</span> },
      { key: 'destinations', header: t('projects.columns.destinations'), render: (p) => list(p.destinations.map(destinationText)) },
      { key: 'cluster', header: t('projects.columns.clusterResources'), render: (p) => <span className="argocd-mono">{list(p.clusterResources)}</span> },
      { key: 'roles', header: t('projects.columns.roles'), render: (p) => list(p.roles) },
      { key: 'windows', header: t('projects.columns.syncWindows'), render: (p) => String(p.syncWindows) },
    ],
    [t, apps.data, appCounts],
  )

  const body = () => {
    if (projects.error) {
      return <Unavailable code={projects.errorCode} reasonCode={projects.errorReason} reason={projects.error} title={t('projects.unavailable')} onRetry={refreshData} />
    }
    if (!projects.data) return <div className="page-loading">{t('projects.loading')}</div>
    if (projects.data.length === 0) return <EmptyState icon="project" title={t('projects.empty')} />
    return <Table columns={columns} rows={projects.data} rowKey={(p) => `${p.namespace}/${p.name}`} />
  }

  return (
    <Page
      title={t('projects.title')}
      className="argocd-wide"
      actions={
        <Button variant="secondary" icon="refresh" onClick={refreshData}>
          {t('actions.reload')}
        </Button>
      }
    >
      {body()}
    </Page>
  )
}

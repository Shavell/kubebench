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
import { StateBadge } from '../components/StatusBadges'
import { argoCd } from '../data'
import type { ArgoApplicationSet } from '../data/types'
import { NS } from '../i18n'
import { openApplicationTab } from '../tabs'
import '@/shared/tabPage.css'
import '../argocd.css'

/** The first condition that reports a problem, else the newest one. */
function headline(set: ArgoApplicationSet) {
  return set.conditions.find((c) => /Error/.test(c.type) && c.status !== 'False') ?? set.conditions[0]
}

/** ApplicationSets with their generators and the applications they generated. */
export function ApplicationSetsPage() {
  const { t } = useTranslation(NS)
  const query = usePluginQuery(() => argoCd().listApplicationSets(), [], {
    topics: ['applicationsets.argoproj.io'],
  })

  const columns = useMemo<Column<ArgoApplicationSet>[]>(
    () => [
      { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (s) => s.name, sortValue: (s) => s.name },
      { key: 'namespace', header: t('common.namespace', { ns: CORE_NS }), render: (s) => s.namespace },
      { key: 'generators', header: t('applicationSets.columns.generators'), render: (s) => s.generators.join(', ') || '—' },
      { key: 'template', header: t('applicationSets.columns.template'), render: (s) => <span className="argocd-mono">{s.templateName || '—'}</span> },
      { key: 'project', header: t('applicationSets.columns.project'), render: (s) => s.project || '—' },
      {
        key: 'apps',
        header: t('applicationSets.columns.applications'),
        render: (s) =>
          s.applications.length === 0 ? (
            '—'
          ) : (
            <span className="argocd-appset-apps">
              {s.applications.map((a) => (
                <button
                  key={a.name}
                  type="button"
                  className="argocd-link"
                  title={a.message || undefined}
                  onClick={(e) => {
                    e.stopPropagation()
                    openApplicationTab(s.namespace, a.name)
                  }}
                >
                  {a.name} <StateBadge value={a.health} />
                </button>
              ))}
            </span>
          ),
        sortValue: (s) => s.applications.length,
      },
      {
        key: 'condition',
        header: t('applicationSets.columns.condition'),
        render: (s) => {
          const c = headline(s)
          return c ? <span className={/Error/.test(c.type) && c.status !== 'False' ? 'argocd-error' : undefined}>{c.message || c.type}</span> : '—'
        },
      },
    ],
    [t],
  )

  const body = () => {
    if (query.error) {
      return <Unavailable code={query.errorCode} reasonCode={query.errorReason} reason={query.error} title={t('applicationSets.unavailable')} onRetry={refreshData} />
    }
    if (!query.data) return <div className="page-loading">{t('applicationSets.loading')}</div>
    if (query.data.length === 0) return <EmptyState icon="files" title={t('applicationSets.empty')} />
    return <Table columns={columns} rows={query.data} rowKey={(s) => `${s.namespace}/${s.name}`} />
  }

  return (
    <Page
      title={t('applicationSets.title')}
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

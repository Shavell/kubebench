import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useProviderQuery } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { EmptyState } from '@/shared/EmptyState'
import { Checkbox, Select } from '@/shared/form'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { accessData } from '../data'
import { NS } from '../i18n'
import { BindingGraphView } from './BindingGraphView'
import '@/shared/tabPage.css'
import '../access.css'

/** Subjects → bindings → roles, for one namespace or the whole cluster. */
export function GraphPage() {
  const { t } = useTranslation(NS)
  const globalNamespace = useContextStore((s) => s.namespace)
  const [namespace, setNamespace] = useState(globalNamespace === ALL_NAMESPACES ? '' : globalNamespace)
  const [includeSystem, setIncludeSystem] = useState(false)
  const namespaces = useProviderQuery((p) => p.cluster.listNamespaces(), [])
  const query = usePluginQuery(() => accessData().graph(namespace, includeSystem), [namespace, includeSystem])
  const graph = query.data

  const body = () => {
    if (query.loading && !graph) return <div className="page-loading">{t('graph.loading')}</div>
    if (!graph) return <Unavailable code={query.errorCode} reason={query.error} title={t('graph.unavailable')} />
    if (!graph.bindings.length) return <EmptyState icon="type-hierarchy" title={t('graph.empty')} hint={t('graph.emptyHint')} />
    return (
      <div className="page__scroll access-page">
        <p className="access-muted">
          {namespace ? t('graph.introNamespace', { namespace }) : t('graph.introCluster')}
          {graph.truncated && ` ${t('graph.truncated', { count: graph.bindings.length })}`}
        </p>
        <div className="access-graph-wrap">
          <BindingGraphView graph={graph} />
        </div>
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={t('graph.title')}>
        <Select
          aria-label={t('graph.namespace')}
          value={namespace}
          onChange={setNamespace}
          options={[
            { value: '', label: t('graph.allNamespaces') },
            ...(namespaces.data ?? []).map((n) => ({ value: n.name, label: n.name })),
          ]}
        />
        <Checkbox className="access-toolbar-check" checked={includeSystem} onChange={setIncludeSystem} label={t('graph.includeSystem')} />
      </Toolbar>
      {body()}
    </div>
  )
}

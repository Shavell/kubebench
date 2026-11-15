import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import type { TabDescriptor } from '@/core/plugin/types'
import { refreshData, useProviderQuery } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { Checkbox, Select } from '@/shared/form'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { LIST_NAMES, useTopologyData, type ListName } from '../data/useTopologyData'
import { buildGraph } from '../model/build'
import { adjacency, relatedIds, subgraph } from '../model/focus'
import { layoutGraph } from '../model/layout'
import { nodeId, type TopologyKind } from '../model/types'
import { NS } from '../i18n'
import { openTopology, type TopologyFocus } from '../tabs'
import { TopologyCanvas } from './TopologyCanvas'
import { useTopologyOptions } from './options'
import '@/shared/tabPage.css'
import '../topology.css'

/** The list that holds objects of a focusable kind. */
const LIST_OF: Partial<Record<TopologyKind, ListName>> = {
  Deployment: 'deployments',
  StatefulSet: 'statefulSets',
  DaemonSet: 'daemonSets',
  Service: 'services',
  Ingress: 'ingresses',
}

/**
 * The topology tab (props `{namespace, focus}`): the namespace's graph, or
 * the part related to the focused object, with display options. Picking
 * another namespace, or leaving the focus, opens that namespace's tab.
 * It fills in as the lists load.
 */
export function TopologyPage({ tab }: { tab: TabDescriptor }) {
  // Keyed: the editor area reuses one renderer instance for every tab of a kind.
  return <TopologyView key={tab.id} tab={tab} />
}

function TopologyView({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const namespace = String(tab.props?.namespace ?? 'default')
  const focus = (tab.props?.focus as TopologyFocus | null) ?? null
  const { options, setOption } = useTopologyOptions()
  const namespaces = useProviderQuery((p) => p.cluster.listNamespaces(), [])
  const data = useTopologyData(namespace)

  const full = useMemo(() => buildGraph(namespace, data.snapshot, options), [namespace, data.snapshot, options])
  const focusId = focus ? nodeId(focus.kind, namespace, focus.name) : null
  const focusFound = focusId !== null && full.nodes.some((n) => n.id === focusId && !n.missing)
  const graph = useMemo(
    () => (focusId && focusFound ? subgraph(full, relatedIds(adjacency(full.edges), focusId)) : full),
    [full, focusId, focusFound],
  )
  const layout = useMemo(() => layoutGraph(graph), [graph])

  const showNamespace = () => openTopology(namespace)
  const nsOptions = useMemo(() => {
    const names = new Set((namespaces.data ?? []).map((n) => n.name))
    names.add(namespace)
    return [...names].sort().map((name) => ({ value: name, label: name }))
  }, [namespaces.data, namespace])

  const body = () => {
    if (data.errors.length === LIST_NAMES.length) {
      const first = data.errors[0]
      return <Unavailable code={first.code} reason={first.error} title={t('page.unavailable')} onRetry={refreshData} />
    }
    if (data.pending === LIST_NAMES.length) return <div className="page-loading">{t('page.loading', { namespace })}</div>
    if (focus && !focusFound) {
      const list = LIST_OF[focus.kind]!
      const failed = data.errors.find((e) => e.list === list)
      if (failed) {
        return (
          <div className="topo-empty">
            <Unavailable
              code={failed.code}
              reason={failed.error}
              title={t('focus.unavailable', { kind: focus.kind, name: focus.name })}
              onRetry={refreshData}
            />
            <Button variant="secondary" icon="type-hierarchy" onClick={showNamespace}>
              {t('focus.clear')}
            </Button>
          </div>
        )
      }
      // Until its list arrives, the whole namespace would only flash by.
      if (!data.snapshot[list]) return <div className="page-loading">{t('page.loading', { namespace })}</div>
      return (
        <div className="topo-empty">
          <EmptyState icon="search" title={t('focus.missing', { kind: focus.kind, name: focus.name })} hint={t('focus.missingHint')} />
          <Button variant="secondary" icon="type-hierarchy" onClick={showNamespace}>
            {t('focus.clear')}
          </Button>
        </div>
      )
    }
    if (graph.nodes.length === 0 && data.pending === 0) {
      return <EmptyState icon="type-hierarchy" title={t('page.empty', { namespace })} hint={t('page.emptyHint')} />
    }
    return (
      <TopologyCanvas
        namespace={namespace}
        graph={graph}
        layout={layout}
        focusId={focusFound ? focusId : null}
      />
    )
  }

  return (
    <div className="page topo-page">
      <Toolbar title={t('page.title')}>
        <Select
          className="topo-page__namespace"
          aria-label={t('page.namespace')}
          title={t('page.namespace')}
          value={namespace}
          options={nsOptions}
          onChange={(next) => openTopology(next)}
        />
        <Checkbox
          checked={options.showConfig}
          onChange={(value) => setOption('showConfig', value)}
          label={t('page.showConfig')}
        />
        <Checkbox
          checked={options.showInactiveReplicaSets}
          onChange={(value) => setOption('showInactiveReplicaSets', value)}
          label={t('page.showInactive')}
        />
        <Button variant="secondary" icon="refresh" onClick={refreshData}>
          {t('page.refresh')}
        </Button>
      </Toolbar>
      {(focus || data.pending > 0 || data.errors.length > 0) && (
        <div className="topo-status">
          {focus && (
            <span className="topo-status__focus">
              {t('focus.label', { kind: focus.kind, name: focus.name })}
              <Button variant="secondary" icon="type-hierarchy" onClick={showNamespace}>
                {t('focus.clear')}
              </Button>
            </span>
          )}
          {data.pending > 0 && data.pending < LIST_NAMES.length && (
            <span className="topo-status__pending">{t('page.pending', { pending: data.pending })}</span>
          )}
          {data.errors.length > 0 && data.errors.length < LIST_NAMES.length && (
            <span className="topo-status__errors" role="alert">
              {data.errors.map((e) => (
                <span key={e.list}>{t('page.listFailed', { list: t(`lists.${e.list}`), error: e.error })}</span>
              ))}
            </span>
          )}
        </div>
      )}
      <div className="topo-page__body">{body()}</div>
    </div>
  )
}

import { ResourceActionButtons } from '@/core/plugin/ResourceActionButtons'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery, type QueryState } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { Badge } from '@/shared/Badge'
import { EmptyState } from '@/shared/EmptyState'
import { EventsTable } from '@/shared/EventsTable'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { ResourceDialogs } from '../editing/ResourceDialogs'
import { ResourceEditButtons } from '../editing/ResourceEditButtons'
import { useWorkloadsTranslation } from '../i18n'
import { DetailRow, DetailSection } from './detail/DetailBits'
import { PodDetail } from './detail/PodDetail'
import { NodeDetail } from './detail/NodeDetail'
import { DeploymentDetail } from './detail/DeploymentDetail'
import '@/shared/tabPage.css'

/** A kind's detail page, with the dialogs its toolbar and menus open. */
export function ResourceDetailPage({ tab }: { tab: TabDescriptor }) {
  return (
    <>
      <ResourceDetail tab={tab} />
      <ResourceDialogs />
    </>
  )
}

function ResourceDetail({ tab }: { tab: TabDescriptor }) {
  const { t } = useWorkloadsTranslation()
  const kind = tab.props?.kind as string
  const namespace = (tab.props?.namespace as string | null) ?? null
  const name = tab.props?.name as string
  const closeTab = useLayoutStore((s) => s.closeTab)
  const closeThisTab = () => closeTab(tab.id)

  const pods = useProviderQuery((p) => p.cluster.listPods(), [], { topics: ['pods'] })
  const nodes = useProviderQuery((p) => p.cluster.listNodes(), [], { topics: ['nodes'] })
  const deployments = useProviderQuery((p) => p.cluster.listDeployments(), [], { topics: ['deployments'] })
  const services = useProviderQuery((p) => p.cluster.listServices(), [], { topics: ['services'] })

  /** Shown until the object turns up: why its list failed, else a loading line. */
  const pending = (query: QueryState<unknown>) =>
    query.error ? (
      <Unavailable code={query.errorCode} reason={query.error} title={t('detail.loadFailed', { name })} />
    ) : (
      <div className="page-loading">{t('detail.loading', { name })}</div>
    )

  if (kind === 'Pod') {
    const pod = pods.data?.find((p) => p.namespace === namespace && p.name === name)
    if (!pod) {
      if (pods.error || pods.loading || !pods.data) return pending(pods)
      return <EmptyState icon="trash" title={t('detail.podGone', { name })} hint={t('detail.podGoneHint')} />
    }
    return <PodDetail pod={pod} tabId={tab.id} />
  }

  if (kind === 'Node') {
    const node = nodes.data?.find((n) => n.name === name)
    if (!node) return pending(nodes)
    return <NodeDetail node={node} onDeleted={closeThisTab} />
  }

  if (kind === 'Deployment') {
    const d = deployments.data?.find((x) => x.namespace === namespace && x.name === name)
    if (!d) return pending(deployments)
    return <DeploymentDetail deployment={d} onDeleted={closeThisTab} />
  }

  if (kind === 'Service') {
    const s = services.data?.find((x) => x.namespace === namespace && x.name === name)
    if (!s) return pending(services)
    const resource = { kind: 'Service', apiVersion: 'v1', namespace: s.namespace, name: s.name }
    return (
      <div className="page">
        <Toolbar title={`${s.namespace} / ${s.name}`}>
          <ResourceEditButtons resource={resource} onDeleted={closeThisTab} />
          <ResourceActionButtons resource={resource} />
        </Toolbar>
        <div className="detail">
          <div className="detail__grid">
            <DetailRow label={t('columns.type')} value={s.type} />
            <DetailRow label={t('columns.clusterIP')} value={s.clusterIP} />
            <DetailRow label={t('columns.externalIP')} value={s.externalIP ?? '—'} />
            <DetailRow label={t('columns.ports')} value={s.ports} />
            <DetailRow label={t('columns.age')} value={s.age} />
            <DetailRow label={t('columns.status')} value={<Badge tone="success">{t('detail.serviceActive')}</Badge>} />
          </div>
          <DetailSection title={t('common.events')} />
          <EventsTable kind="Service" namespace={s.namespace} name={s.name} />
        </div>
      </div>
    )
  }

  return <div className="page-loading">{t('detail.unknownKind', { kind })}</div>
}

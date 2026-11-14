import { ResourceActionButtons } from '@/core/plugin/ResourceActionButtons'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery, useProviders, type QueryState } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { Badge } from '@/shared/Badge'
import { EmptyState } from '@/shared/EmptyState'
import { EventsTable } from '@/shared/EventsTable'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { useWorkloadsTranslation } from '../i18n'
import { DetailRow, DetailSection } from './detail/DetailBits'
import { PodDetail } from './detail/PodDetail'
import { NodeDetail } from './detail/NodeDetail'
import { DeploymentDetail } from './detail/DeploymentDetail'
import '@/shared/tabPage.css'

export function ResourceDetailPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useWorkloadsTranslation()
  const kind = tab.props?.kind as string
  const namespace = (tab.props?.namespace as string | null) ?? null
  const name = tab.props?.name as string
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)

  const pods = useProviderQuery((p) => p.cluster.listPods(), [])
  const nodes = useProviderQuery((p) => p.cluster.listNodes(), [])
  const deployments = useProviderQuery((p) => p.cluster.listDeployments(), [])
  const services = useProviderQuery((p) => p.cluster.listServices(), [])

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
    return <NodeDetail node={node} />
  }

  if (kind === 'Deployment') {
    const d = deployments.data?.find((x) => x.namespace === namespace && x.name === name)
    if (!d) return pending(deployments)
    return <DeploymentDetail deployment={d} />
  }

  if (kind === 'Service') {
    const s = services.data?.find((x) => x.namespace === namespace && x.name === name)
    if (!s) return pending(services)
    const viewYaml = async () => {
      let text: string
      try {
        text = await providers.cluster.getResourceYaml('Service', s.namespace, s.name)
      } catch (err) {
        notify.error(err)
        return
      }
      openTab({
        id: `yaml:service:${s.namespace}/${s.name}`,
        title: `${s.name}.yaml`,
        icon: 'file-code',
        kind: 'yaml',
        props: { text, readOnly: true },
      })
    }
    return (
      <div className="page">
        <Toolbar title={`${s.namespace} / ${s.name}`}>
          <button className="kb-button kb-button--secondary" onClick={viewYaml}>
            <Icon name="file-code" size={14} /> {t('actions.yaml')}
          </button>
          <ResourceActionButtons resource={{ kind: 'Service', apiVersion: 'v1', namespace: s.namespace, name: s.name }} />
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

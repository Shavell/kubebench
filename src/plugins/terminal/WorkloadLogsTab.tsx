import { useTranslation } from 'react-i18next'
import type { TabDescriptor } from '@/core/plugin/types'
import { ApiError } from '@/data/apiClient'
import { useProviderQuery } from '@/data/DataContext'
import type { DataProviders } from '@/data/providers'
import { i18n } from '@/i18n'
import { Unavailable } from '@/shared/Unavailable'
import { NS } from './i18n'
import { LogsView } from './logview/LogsView'
import { TabPage } from './TabPage'
import { selectorString, workloadKey, type WorkloadRef } from './workloadLogs'

const text = (value: unknown) => (typeof value === 'string' ? value : '')

function workloadOf(value: unknown): WorkloadRef | null {
  if (!value || typeof value !== 'object') return null
  const { kind, name } = value as Record<string, unknown>
  return typeof kind === 'string' && typeof name === 'string' ? { kind, name } : null
}

/**
 * Editor tab of kind "workload-logs": the logs of every pod a label
 * selector matches (`{namespace, selector, title}`), or of a workload's
 * pods (`{namespace, title, workload: {kind, name}}`), whose selector is
 * read from its spec.
 */
export function WorkloadLogsTab({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const props = tab.props ?? {}
  const namespace = text(props.namespace)
  const title = text(props.title) || tab.title
  const given = text(props.selector)
  const workload = given ? null : workloadOf(props.workload)
  const resolved = useProviderQuery(
    (p) => (workload ? workloadSelector(p, namespace, workload) : Promise.resolve(given || null)),
    [namespace, given, workload?.kind, workload?.name],
  )

  if (!namespace || (!given && !workload)) return <Unavailable code="bad_request" reason={t('workloadLogs.badTarget')} />
  if (resolved.error) {
    return <Unavailable code={resolved.errorCode} reasonCode={resolved.errorReason} reason={resolved.error} title={t('workloadLogs.loadFailed', { title })} />
  }
  if (resolved.data === null && resolved.loading) return <div className="page-loading">{t('workloadLogs.resolving')}</div>
  if (!resolved.data) return <Unavailable icon="search-stop" title={t('workloadLogs.noSelector', { title })} />
  return (
    <TabPage>
      <LogsView key={`${namespace}/${resolved.data}`} target={{ kind: 'selector', namespace, selector: resolved.data, title }} />
    </TabPage>
  )
}

/** The label selector of a workload's pods, from its spec. */
async function workloadSelector(providers: DataProviders, namespace: string, workload: WorkloadRef): Promise<string | null> {
  const t = (key: string, values: Record<string, string>) => i18n.t(key, { ns: NS, ...values })
  const key = workloadKey(workload.kind)
  if (key === undefined) {
    throw new ApiError(400, 'bad_request', t('workloadLogs.unsupportedKind', { kind: workload.kind }))
  }
  if (key === null) {
    // Deployments are a rich kind: the list maps spec.selector.matchLabels to `labels`.
    const deployment = (await providers.cluster.listDeployments(namespace)).find((d) => d.name === workload.name)
    if (!deployment) {
      throw new ApiError(404, 'not_found', t('workloadLogs.notFound', { kind: workload.kind, name: workload.name }))
    }
    return selectorString({ matchLabels: deployment.labels })
  }
  const object = await providers.cluster.getResource(key, namespace, workload.name)
  const spec = object.spec as { selector?: Parameters<typeof selectorString>[0] } | undefined
  return selectorString(spec?.selector)
}

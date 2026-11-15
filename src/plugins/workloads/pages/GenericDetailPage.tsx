import { ResourceActionButtons } from '@/core/plugin/ResourceActionButtons'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import type { KubeObject } from '@/data/types'
import { EventsTable } from '@/shared/EventsTable'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { ResourceDialogs } from '../editing/ResourceDialogs'
import { ResourceEditButtons } from '../editing/ResourceEditButtons'
import { useWorkloadsTranslation } from '../i18n'
import { catalogByKey } from '../resourceCatalog'
import { DetailRow, DetailSection } from './detail/DetailBits'
import '@/shared/tabPage.css'

/** List columns that the detail grid shows elsewhere or that are too long for it. */
const NOT_IN_SUMMARY = new Set(['name', 'namespace', 'age', 'message'])

/** Uniform detail for any generic resource: metadata + labels/annotations
 *  + spec/status summary + events + YAML. */
export function GenericDetailPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useWorkloadsTranslation()
  const key = tab.props?.resourceKey as string
  const namespace = (tab.props?.namespace as string | null) ?? null
  const name = tab.props?.name as string
  const descriptor = catalogByKey.get(key)
  const closeTab = useLayoutStore((s) => s.closeTab)

  const { data, loading, error, errorCode } = useProviderQuery(
    (p) => p.cluster.getResource(key, namespace, name),
    [key, namespace, name],
  )

  if (!descriptor) return <div className="page-loading">{t('list.unknownResource', { key })}</div>
  if (error) return <Unavailable code={errorCode} reason={error} title={t('detail.loadFailed', { name })} />
  if (loading || !data) return <div className="page-loading">{t('detail.loading', { name })}</div>

  const obj = data as KubeObject
  const labels = obj.metadata?.labels ?? {}
  const annotations = obj.metadata?.annotations ?? {}

  const resource = { kind: descriptor.kind, apiVersion: obj.apiVersion, namespace: namespace ?? undefined, name }

  // Reuse the list columns (minus Name/Namespace/Age/Message) as a quick summary.
  const summaryCols = descriptor.columns.filter((c) => !NOT_IN_SUMMARY.has(c.key))

  return (
    <div className="page">
      <Toolbar title={namespace ? `${namespace} / ${name}` : name}>
        <ResourceEditButtons resource={resource} onDeleted={() => closeTab(tab.id)} />
        <ResourceActionButtons resource={resource} />
      </Toolbar>
      <ResourceDialogs />
      <div className="detail">
        <div className="detail__grid">
          <DetailRow label={t('common.kind')} value={descriptor.kind} />
          {namespace && <DetailRow label={t('common.namespace')} value={namespace} />}
          {summaryCols.map((c) => (
            <DetailRow key={c.key} label={t(`columns.${c.key}`)} value={c.render(obj)} />
          ))}
        </div>

        {Object.keys(labels).length > 0 && (
          <>
            <DetailSection title={t('common.labels')} />
            {Object.entries(labels).map(([k, v]) => (
              <span key={k} className="detail__chip">
                {k}={v}
              </span>
            ))}
          </>
        )}

        {Object.keys(annotations).length > 0 && (
          <>
            <DetailSection title={t('common.annotations')} />
            <div className="detail__grid detail__grid--wide">
              {Object.entries(annotations).map(([k, v]) => (
                <DetailRow key={k} label={k} value={<code className="detail__code">{String(v)}</code>} />
              ))}
            </div>
          </>
        )}

        {descriptor.namespaced && (
          <>
            <DetailSection title={t('common.events')} />
            <EventsTable kind={descriptor.kind} namespace={namespace} name={name} />
          </>
        )}
      </div>
    </div>
  )
}

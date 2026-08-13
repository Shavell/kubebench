import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import type { KubeObject } from '@/data/types'
import { EventsTable } from '@/shared/EventsTable'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import { catalogByKey } from '../resourceCatalog'
import { DetailRow, DetailSection } from './detail/DetailBits'
import './pages.css'

/** Uniform detail for any generic resource: metadata + labels/annotations
 *  + spec/status summary + events + YAML. */
export function GenericDetailPage({ tab }: { tab: TabDescriptor }) {
  const key = tab.props?.resourceKey as string
  const namespace = (tab.props?.namespace as string | null) ?? null
  const name = tab.props?.name as string
  const descriptor = catalogByKey.get(key)
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)

  const { data, loading } = useProviderQuery(
    (p) => p.cluster.getResource(key, namespace, name),
    [key, namespace, name],
  )

  if (!descriptor) return <div className="page-loading">Unknown resource {key}</div>
  if (loading || !data) return <div className="page-loading">Loading {name}…</div>

  const obj = data as KubeObject
  const labels = obj.metadata?.labels ?? {}
  const annotations = obj.metadata?.annotations ?? {}

  const viewYaml = async () => {
    const text = await providers.cluster.getResourceYaml(descriptor.kind, namespace, name)
    openTab({
      id: `yaml:${key}:${namespace ?? '-'}/${name}`,
      title: `${name}.yaml`,
      icon: 'file-code',
      kind: 'yaml',
      props: { text, readOnly: true },
    })
  }

  // Reuse the list columns (minus Name/Namespace/Age) as a quick summary.
  const summaryCols = descriptor.columns.filter(
    (c) => !['Name', 'Namespace', 'Age', 'Message'].includes(c.header),
  )

  return (
    <div className="page">
      <Toolbar title={namespace ? `${namespace} / ${name}` : name}>
        <button className="kb-button kb-button--secondary" onClick={viewYaml}>
          <Icon name="file-code" size={14} /> YAML
        </button>
      </Toolbar>
      <div className="detail">
        <div className="detail__grid">
          <DetailRow label="Kind" value={descriptor.kind} />
          {namespace && <DetailRow label="Namespace" value={namespace} />}
          {summaryCols.map((c) => (
            <DetailRow key={c.header} label={c.header} value={c.render(obj)} />
          ))}
        </div>

        {Object.keys(labels).length > 0 && (
          <>
            <DetailSection title="Labels" />
            {Object.entries(labels).map(([k, v]) => (
              <span key={k} className="detail__chip">
                {k}={v}
              </span>
            ))}
          </>
        )}

        {Object.keys(annotations).length > 0 && (
          <>
            <DetailSection title="Annotations" />
            <div className="detail__grid detail__grid--wide">
              {Object.entries(annotations).map(([k, v]) => (
                <DetailRow key={k} label={k} value={<code className="detail__code">{String(v)}</code>} />
              ))}
            </div>
          </>
        )}

        {descriptor.namespaced && (
          <>
            <DetailSection title="Events" />
            <EventsTable kind={descriptor.kind} namespace={namespace} name={name} />
          </>
        )}
      </div>
    </div>
  )
}

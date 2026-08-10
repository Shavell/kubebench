import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { Badge } from '@/shared/Badge'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { EmptyState } from '@/shared/EmptyState'
import type { CustomResource } from '@/data/types'
import '@/plugins/workloads/pages/pages.css'

export function CustomResourceListPage({ tab }: { tab: TabDescriptor }) {
  const crdName = tab.props?.crdName as string
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)

  const { data: crds } = useProviderQuery((p) => p.crd.listCrds(), [])
  const { data, loading } = useProviderQuery((p) => p.crd.listCustomResources(crdName), [crdName])

  const crd = crds?.find((c) => c.name === crdName)
  if (loading) return <div className="page-loading">Loading {crdName}…</div>

  const rows = data ?? []
  const extraKeys = [...new Set(rows.flatMap((r) => Object.keys(r.extra)))]

  const columns: Column<CustomResource>[] = [
    { key: 'name', header: 'Name', render: (r) => r.name, sortValue: (r) => r.name },
    { key: 'ns', header: 'Namespace', render: (r) => r.namespace ?? '—', sortValue: (r) => r.namespace ?? '' },
    { key: 'status', header: 'Status', render: (r) => <Badge>{r.status}</Badge> },
    ...extraKeys.map(
      (key): Column<CustomResource> => ({
        key,
        header: key,
        render: (r) => r.extra[key] ?? '—',
      }),
    ),
    { key: 'age', header: 'Age', render: (r) => r.age },
  ]

  const viewYaml = async (r: CustomResource) => {
    const text = await providers.cluster.getResourceYaml(crd?.kind ?? 'CustomResource', r.namespace ?? null, r.name)
    openTab({
      id: `yaml:cr:${crdName}:${r.namespace ?? '-'}/${r.name}`,
      title: `${r.name}.yaml`,
      icon: 'file-code',
      kind: 'yaml',
      props: { text, readOnly: true },
    })
  }

  return (
    <div className="page">
      <Toolbar title={`${crd?.kind ?? crdName} · ${crdName}`} />
      {rows.length === 0 ? (
        <EmptyState icon="inbox" title="No custom resources" />
      ) : (
        <div className="page__scroll">
          <Table
            columns={columns}
            rows={rows}
            rowKey={(r) => `${r.namespace ?? '-'}/${r.name}`}
            onRowClick={viewYaml}
          />
        </div>
      )}
    </div>
  )
}

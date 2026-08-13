import { useState } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useProviderQuery } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import type { KubeObject } from '@/data/types'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { EmptyState } from '@/shared/EmptyState'
import { catalogByKey, objName, objNamespace } from '../resourceCatalog'
import './pages.css'

export function GenericListPage({ tab }: { tab: TabDescriptor }) {
  const key = tab.props?.resourceKey as string
  const descriptor = catalogByKey.get(key)
  const globalNamespace = useContextStore((s) => s.namespace)
  const [search, setSearch] = useState('')
  const [localNs, setLocalNs] = useState('')

  const nsFilter = descriptor?.namespaced
    ? localNs || (globalNamespace === ALL_NAMESPACES ? undefined : globalNamespace)
    : undefined

  const { data: namespaces } = useProviderQuery((p) => p.cluster.listNamespaces(), [])
  const query = useProviderQuery(
    (p) => (descriptor ? p.cluster.listResource(key, nsFilter) : Promise.resolve([])),
    [key, nsFilter],
  )

  if (!descriptor) return <div className="page-loading">Unknown resource {key}</div>
  if (query.loading && !query.data) return <div className="page-loading">Loading {descriptor.label}…</div>

  const all = query.data ?? []
  const rows = search
    ? all.filter((o) => objName(o).toLowerCase().includes(search.toLowerCase()))
    : all

  const columns: Column<KubeObject>[] = descriptor.columns.map((c) => ({
    key: c.header,
    header: c.header,
    width: c.width,
    render: c.render,
    sortValue: c.sortValue,
  }))

  const openDetail = (o: KubeObject) =>
    useLayoutStore.getState().openTab({
      id: `res:${key}:${objNamespace(o) || '-'}/${objName(o)}`,
      title: objName(o),
      icon: descriptor.icon,
      kind: 'generic-detail',
      props: { resourceKey: key, namespace: objNamespace(o) || null, name: objName(o) },
    })

  const scopedTitle = descriptor.namespaced && nsFilter ? `${descriptor.label} · ${nsFilter}` : descriptor.label

  return (
    <div className="page">
      <Toolbar title={scopedTitle}>
        <ListFilterBar
          search={search}
          onSearch={setSearch}
          filtersActive={Boolean(localNs)}
          summary={`${rows.length} of ${all.length}`}
        >
          {descriptor.namespaced && (
            <select title="Namespace filter (this tab)" value={localNs} onChange={(e) => setLocalNs(e.target.value)}>
              <option value="">
                {globalNamespace === ALL_NAMESPACES ? 'ns: all' : `ns: global (${globalNamespace})`}
              </option>
              {(namespaces ?? []).map((ns) => (
                <option key={ns.name} value={ns.name}>
                  {ns.name}
                </option>
              ))}
            </select>
          )}
        </ListFilterBar>
      </Toolbar>
      {rows.length === 0 ? (
        <EmptyState icon="inbox" title={`No ${descriptor.label}`} hint={search || nsFilter ? 'Try clearing the filters' : undefined} />
      ) : (
        <div className="page__scroll">
          <Table
            columns={columns}
            rows={rows}
            rowKey={(o) => `${objNamespace(o)}/${objName(o)}`}
            onRowClick={openDetail}
          />
        </div>
      )}
    </div>
  )
}

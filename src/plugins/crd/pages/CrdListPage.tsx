import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import type { CrdDefinition } from '@/data/types'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'

const columns: Column<CrdDefinition>[] = [
  { key: 'name', header: 'Name', render: (c) => c.name, sortValue: (c) => c.name },
  { key: 'group', header: 'Group', render: (c) => c.group, sortValue: (c) => c.group },
  { key: 'kind', header: 'Kind', render: (c) => c.kind },
  { key: 'versions', header: 'Versions', render: (c) => c.versions.join(', ') },
  { key: 'scope', header: 'Scope', render: (c) => c.scope },
  { key: 'short', header: 'Short Names', render: (c) => c.shortNames.join(', ') || '—' },
  { key: 'age', header: 'Age', render: (c) => c.age },
]

export function CrdListPage() {
  const { data, loading } = useProviderQuery((p) => p.crd.listCrds(), [])
  const openTab = useLayoutStore((s) => s.openTab)

  if (loading) return <div className="page-loading">Loading CRDs…</div>

  return (
    <div className="page">
      <Toolbar title="Custom Resource Definitions" />
      <div className="page__scroll">
        <Table
          columns={columns}
          rows={data ?? []}
          rowKey={(c) => c.name}
          onRowClick={(c) =>
            openTab({
              id: `crd:${c.name}`,
              title: c.kind,
              icon: 'symbol-class',
              kind: 'crd-instances',
              props: { crdName: c.name },
            })
          }
        />
      </div>
    </div>
  )
}

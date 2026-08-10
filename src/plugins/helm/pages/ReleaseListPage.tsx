import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import type { HelmRelease } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'

const columns: Column<HelmRelease>[] = [
  { key: 'name', header: 'Name', render: (r) => r.name, sortValue: (r) => r.name },
  { key: 'ns', header: 'Namespace', render: (r) => r.namespace, sortValue: (r) => r.namespace },
  { key: 'chart', header: 'Chart', render: (r) => `${r.chart}-${r.chartVersion}` },
  { key: 'app', header: 'App Version', render: (r) => r.appVersion },
  { key: 'rev', header: 'Revision', render: (r) => String(r.revision), sortValue: (r) => r.revision },
  { key: 'status', header: 'Status', render: (r) => <Badge>{r.status}</Badge>, sortValue: (r) => r.status },
  { key: 'updated', header: 'Updated', render: (r) => r.updated, sortValue: (r) => r.updated },
]

export function ReleaseListPage() {
  const { data, loading } = useProviderQuery((p) => p.helm.listReleases(), [])
  const openTab = useLayoutStore((s) => s.openTab)

  if (loading) return <div className="page-loading">Loading releases…</div>

  return (
    <div className="page">
      <Toolbar title="Helm Releases" />
      <div className="page__scroll">
        <Table
          columns={columns}
          rows={data ?? []}
          rowKey={(r) => `${r.namespace}/${r.name}`}
          onRowClick={(r) =>
            openTab({
              id: `release:${r.name}`,
              title: r.name,
              icon: 'package',
              kind: 'helm-release-detail',
              props: { name: r.name },
            })
          }
        />
      </div>
    </div>
  )
}

import { useProviderQuery } from '@/data/DataContext'
import type { HelmRepo } from '@/data/types'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'

const columns: Column<HelmRepo>[] = [
  { key: 'name', header: 'Name', render: (r) => r.name, sortValue: (r) => r.name },
  { key: 'url', header: 'URL', render: (r) => r.url },
  { key: 'charts', header: 'Charts', render: (r) => String(r.chartCount), sortValue: (r) => r.chartCount },
]

export function RepoListPage() {
  const { data, loading } = useProviderQuery((p) => p.helm.listRepositories(), [])

  if (loading) return <div className="page-loading">Loading repositories…</div>

  return (
    <div className="page">
      <Toolbar title="Helm Repositories" />
      <div className="page__scroll">
        <Table columns={columns} rows={data ?? []} rowKey={(r) => r.name} />
      </div>
    </div>
  )
}

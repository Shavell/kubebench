import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'

export function MetricsSidebar() {
  const openTab = useLayoutStore((s) => s.openTab)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data: nodes } = useProviderQuery((p) => p.cluster.listNodes(), [])
  const { data: sources } = useProviderQuery((p) => p.metrics.listSources(), [])

  const tree: TreeNode[] = [
    { id: 'metrics-overview', label: 'Cluster Overview', icon: 'dashboard' },
    {
      id: 'metrics-nodes',
      label: 'Nodes',
      icon: 'server',
      children: (nodes ?? []).map((n) => ({
        id: `metrics-node:${n.name}`,
        label: n.name,
        icon: 'pulse',
      })),
    },
    {
      id: 'metrics-sources',
      label: 'Sources',
      icon: 'plug',
      children: (sources ?? []).map((s) => ({
        id: `src-${s.id}`,
        label: s.detail && s.available ? `${s.name} · ${s.detail}` : s.name,
        icon: s.available ? 'check' : 'circle-slash',
      })),
    },
  ]

  return (
    <Tree
      nodes={tree}
      selectedId={activeTabId}
      onSelect={(node) => {
        if (node.id === 'metrics-overview') {
          openTab({
            id: 'metrics-overview',
            title: 'Cluster Overview',
            icon: 'dashboard',
            kind: 'metrics-overview',
          })
        } else if (node.id.startsWith('metrics-node:')) {
          const name = node.id.slice('metrics-node:'.length)
          openTab({
            id: node.id,
            title: `${name} · metrics`,
            icon: 'pulse',
            kind: 'metrics-node',
            props: { node: name },
          })
        }
      }}
    />
  )
}

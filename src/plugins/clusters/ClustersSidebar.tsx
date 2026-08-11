import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'
import { Icon } from '@/shared/Icon'

export function ClustersSidebar() {
  const openTab = useLayoutStore((s) => s.openTab)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data: clusters } = useProviderQuery((p) => p.inventory.listClusters(), [])

  const nodes: TreeNode[] = [
    {
      id: 'clusters-inventory',
      label: 'Kubernetes Clusters',
      icon: 'server-environment',
      count: clusters?.length,
      children: (clusters ?? []).map((c) => ({
        id: `cluster:${c.name}`,
        label: c.name,
        icon: c.error ? 'warning' : 'vm',
        suffix: c.active ? (
          <Icon name="pass-filled" size={13} style={{ color: 'var(--status-running)' }} />
        ) : undefined,
      })),
    },
  ]

  return (
    <Tree
      nodes={nodes}
      selectedId={activeTabId}
      onSelect={() =>
        openTab({
          id: 'clusters-inventory',
          title: 'Cluster Inventory',
          icon: 'server-environment',
          kind: 'clusters-inventory',
        })
      }
    />
  )
}

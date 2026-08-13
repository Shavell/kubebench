import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'
import { RESOURCE_CATALOG, type ResourceGroup } from './resourceCatalog'

// Rich types keep their bespoke pages; everything else is generic.
const RICH: Record<string, { icon: string; label: string }> = {
  Nodes: { icon: 'server', label: 'Nodes' },
  Namespaces: { icon: 'folder', label: 'Namespaces' },
  Pods: { icon: 'package', label: 'Pods' },
  Deployments: { icon: 'rocket', label: 'Deployments' },
  Services: { icon: 'plug', label: 'Services' },
}

const GROUP_ORDER: ResourceGroup[] = [
  'Cluster',
  'Workloads',
  'Config',
  'Network',
  'Storage',
  'Access Control',
]
const GROUP_ICON: Record<ResourceGroup, string> = {
  Cluster: 'vm',
  Workloads: 'layers',
  Config: 'settings-gear',
  Network: 'globe',
  Storage: 'database',
  'Access Control': 'shield',
}

function openRichList(resource: string, icon: string) {
  useLayoutStore.getState().openTab({
    id: `list:${resource}`,
    title: resource,
    icon,
    kind: 'workloads-list',
    props: { resource },
  })
}

function openGenericList(key: string, label: string, icon: string) {
  useLayoutStore.getState().openTab({
    id: `reslist:${key}`,
    title: label,
    icon,
    kind: 'generic-list',
    props: { resourceKey: key },
  })
}

export function WorkloadsSidebar() {
  const activeTabId = useLayoutStore((s) => s.activeTabId)

  // Counts for the rich types (nice to keep the familiar badges).
  const { data: pods } = useProviderQuery((p) => p.cluster.listPods(), [])
  const { data: nodes } = useProviderQuery((p) => p.cluster.listNodes(), [])
  const { data: deployments } = useProviderQuery((p) => p.cluster.listDeployments(), [])
  const { data: services } = useProviderQuery((p) => p.cluster.listServices(), [])
  const { data: namespaces } = useProviderQuery((p) => p.cluster.listNamespaces(), [])

  // Build each group's children: rich entries first, then catalog entries.
  const richByGroup: Record<string, TreeNode[]> = {
    Cluster: [
      { id: 'list:Nodes', label: 'Nodes', icon: RICH.Nodes.icon, count: nodes?.length },
      { id: 'list:Namespaces', label: 'Namespaces', icon: RICH.Namespaces.icon, count: namespaces?.length },
    ],
    Workloads: [
      { id: 'list:Pods', label: 'Pods', icon: RICH.Pods.icon, count: pods?.length },
      { id: 'list:Deployments', label: 'Deployments', icon: RICH.Deployments.icon, count: deployments?.length },
    ],
    Network: [{ id: 'list:Services', label: 'Services', icon: RICH.Services.icon, count: services?.length }],
  }

  const nodes_: TreeNode[] = GROUP_ORDER.map((group) => {
    const generic = RESOURCE_CATALOG.filter((d) => d.group === group).map((d) => ({
      id: `reslist:${d.key}`,
      label: d.label,
      icon: d.icon,
    }))
    return {
      id: `group:${group}`,
      label: group,
      icon: GROUP_ICON[group],
      children: [...(richByGroup[group] ?? []), ...generic],
    }
  })

  return (
    <Tree
      nodes={nodes_}
      selectedId={activeTabId}
      defaultExpanded={['group:Cluster', 'group:Workloads', 'group:Network']}
      onSelect={(node) => {
        if (node.id.startsWith('list:')) {
          const resource = node.id.slice('list:'.length)
          openRichList(resource, RICH[resource]?.icon ?? 'symbol-field')
        } else if (node.id.startsWith('reslist:')) {
          const key = node.id.slice('reslist:'.length)
          const d = RESOURCE_CATALOG.find((x) => x.key === key)!
          openGenericList(key, d.label, d.icon)
        }
      }}
    />
  )
}

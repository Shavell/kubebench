import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'

const RESOURCE_ICONS: Record<string, string> = {
  Nodes: 'server',
  Namespaces: 'folder',
  Pods: 'package',
  Deployments: 'rocket',
  Services: 'plug',
}

export function WorkloadsSidebar() {
  const openTab = useLayoutStore((s) => s.openTab)
  const activeTabId = useLayoutStore((s) => s.activeTabId)

  const { data: pods } = useProviderQuery((p) => p.cluster.listPods(), [])
  const { data: nodes } = useProviderQuery((p) => p.cluster.listNodes(), [])
  const { data: deployments } = useProviderQuery((p) => p.cluster.listDeployments(), [])
  const { data: services } = useProviderQuery((p) => p.cluster.listServices(), [])
  const { data: namespaces } = useProviderQuery((p) => p.cluster.listNamespaces(), [])

  const nodesTree: TreeNode[] = [
    {
      id: 'cluster',
      label: 'Cluster',
      icon: 'vm',
      children: [
        { id: 'list:Nodes', label: 'Nodes', icon: RESOURCE_ICONS.Nodes, count: nodes?.length },
        {
          id: 'list:Namespaces',
          label: 'Namespaces',
          icon: RESOURCE_ICONS.Namespaces,
          count: namespaces?.length,
        },
      ],
    },
    {
      id: 'workloads',
      label: 'Workloads',
      icon: 'layers',
      children: [
        { id: 'list:Pods', label: 'Pods', icon: RESOURCE_ICONS.Pods, count: pods?.length },
        {
          id: 'list:Deployments',
          label: 'Deployments',
          icon: RESOURCE_ICONS.Deployments,
          count: deployments?.length,
        },
      ],
    },
    {
      id: 'network',
      label: 'Network',
      icon: 'globe',
      children: [
        {
          id: 'list:Services',
          label: 'Services',
          icon: RESOURCE_ICONS.Services,
          count: services?.length,
        },
      ],
    },
  ]

  return (
    <Tree
      nodes={nodesTree}
      selectedId={activeTabId}
      onSelect={(node) => {
        if (!node.id.startsWith('list:')) return
        const resource = node.id.slice('list:'.length)
        openTab({
          id: node.id,
          title: resource,
          icon: RESOURCE_ICONS[resource],
          kind: 'workloads-list',
          props: { resource },
        })
      }}
    />
  )
}

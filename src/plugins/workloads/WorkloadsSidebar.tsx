import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'
import { useWorkloadsTranslation, WORKLOADS_NS } from './i18n'
import { RESOURCE_CATALOG, resourceLabel, richLabel, type ResourceGroup } from './resourceCatalog'

// Rich types keep their bespoke pages; everything else is generic.
// Keys are the `resource` prop of workloads-list tabs.
const RICH_ICONS: Record<string, string> = {
  Nodes: 'server',
  Namespaces: 'folder',
  Pods: 'package',
  Deployments: 'rocket',
  Services: 'plug',
}

const GROUP_ORDER: ResourceGroup[] = ['cluster', 'workloads', 'config', 'network', 'storage', 'accessControl']
const GROUP_ICON: Record<ResourceGroup, string> = {
  cluster: 'vm',
  workloads: 'layers',
  config: 'settings-gear',
  network: 'globe',
  storage: 'database',
  accessControl: 'shield',
}

function openRichList(resource: string, icon: string) {
  useLayoutStore.getState().openTab({
    id: `list:${resource}`,
    title: richLabel(resource),
    titleKey: `resources.${resource.toLowerCase()}`,
    titleNs: WORKLOADS_NS,
    icon,
    kind: 'workloads-list',
    props: { resource },
  })
}

function openGenericList(key: string, icon: string) {
  useLayoutStore.getState().openTab({
    id: `reslist:${key}`,
    title: resourceLabel(key),
    titleKey: `resources.${key}`,
    titleNs: WORKLOADS_NS,
    icon,
    kind: 'generic-list',
    props: { resourceKey: key },
  })
}

export function WorkloadsSidebar() {
  const { t } = useWorkloadsTranslation()
  const activeTabId = useLayoutStore((s) => s.activeTabId)

  // Counts for the rich types (nice to keep the familiar badges).
  const { data: pods } = useProviderQuery((p) => p.cluster.listPods(), [])
  const { data: nodes } = useProviderQuery((p) => p.cluster.listNodes(), [])
  const { data: deployments } = useProviderQuery((p) => p.cluster.listDeployments(), [])
  const { data: services } = useProviderQuery((p) => p.cluster.listServices(), [])
  const { data: namespaces } = useProviderQuery((p) => p.cluster.listNamespaces(), [])

  const rich = (resource: string, count: number | undefined): TreeNode => ({
    id: `list:${resource}`,
    label: richLabel(resource),
    icon: RICH_ICONS[resource],
    count,
  })

  // Build each group's children: rich entries first, then catalog entries.
  const richByGroup: Partial<Record<ResourceGroup, TreeNode[]>> = {
    cluster: [rich('Nodes', nodes?.length), rich('Namespaces', namespaces?.length)],
    workloads: [rich('Pods', pods?.length), rich('Deployments', deployments?.length)],
    network: [rich('Services', services?.length)],
  }

  const tree: TreeNode[] = GROUP_ORDER.map((group) => {
    const generic = RESOURCE_CATALOG.filter((d) => d.group === group).map((d) => ({
      id: `reslist:${d.key}`,
      label: resourceLabel(d.key),
      icon: d.icon,
    }))
    return {
      id: `group:${group}`,
      label: t(`groups.${group}`),
      icon: GROUP_ICON[group],
      children: [...(richByGroup[group] ?? []), ...generic],
    }
  })

  return (
    <Tree
      nodes={tree}
      selectedId={activeTabId}
      defaultExpanded={['group:cluster', 'group:workloads', 'group:network']}
      onSelect={(node) => {
        if (node.id.startsWith('list:')) {
          const resource = node.id.slice('list:'.length)
          openRichList(resource, RICH_ICONS[resource] ?? 'symbol-field')
        } else if (node.id.startsWith('reslist:')) {
          const key = node.id.slice('reslist:'.length)
          const d = RESOURCE_CATALOG.find((x) => x.key === key)!
          openGenericList(key, d.icon)
        }
      }}
    />
  )
}

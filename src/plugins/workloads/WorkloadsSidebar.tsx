import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import type { DataProviders } from '@/data/providers'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { useWorkloadsTranslation, WORKLOADS_NS } from './i18n'
import { isServed, RESOURCE_CATALOG, resourceLabel, richLabel, type ResourceGroup } from './resourceCatalog'

// Rich types keep their bespoke pages; everything else is generic.
// Keys are the `resource` prop of workloads-list tabs.
const RICH_ICONS: Record<string, string> = {
  Nodes: 'server',
  Namespaces: 'folder',
  Pods: 'package',
  Deployments: 'rocket',
  Services: 'plug',
}

const GROUP_ORDER: ResourceGroup[] = ['cluster', 'workloads', 'config', 'network', 'storage', 'accessControl', 'openshift']
const GROUP_ICON: Record<ResourceGroup, string> = {
  cluster: 'vm',
  workloads: 'layers',
  config: 'settings-gear',
  network: 'globe',
  storage: 'database',
  accessControl: 'shield',
  openshift: 'server-environment',
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

  // Counts for the rich types (nice to keep the familiar badges): counted
  // by the server (GET /api/count), not by listing every object.
  const count = (topic: string) => (p: DataProviders) => p.cluster.countResource(topic)
  const podsQuery = useProviderQuery(count('pods'), [], { topics: ['pods'] })
  const nodesQuery = useProviderQuery(count('nodes'), [], { topics: ['nodes'] })
  const deploymentsQuery = useProviderQuery(count('deployments'), [], { topics: ['deployments'] })
  const servicesQuery = useProviderQuery(count('services'), [], { topics: ['services'] })
  const namespacesQuery = useProviderQuery(count('namespaces'), [], { topics: ['namespaces'] })
  const queries = [podsQuery, nodesQuery, deploymentsQuery, servicesQuery, namespacesQuery]

  const rich = (resource: string, count: number | undefined): TreeNode => ({
    id: `list:${resource}`,
    label: richLabel(resource),
    icon: RICH_ICONS[resource],
    count,
  })

  // Build each group's children: rich entries first, then catalog entries.
  const richByGroup: Partial<Record<ResourceGroup, TreeNode[]>> = {
    cluster: [rich('Nodes', nodesQuery.data ?? undefined), rich('Namespaces', namespacesQuery.data ?? undefined)],
    workloads: [rich('Pods', podsQuery.data ?? undefined), rich('Deployments', deploymentsQuery.data ?? undefined)],
    network: [rich('Services', servicesQuery.data ?? undefined)],
  }

  // Optional types (OpenShift's) only where the cluster serves them; a
  // group left without entries is not shown.
  const tree: TreeNode[] = GROUP_ORDER.flatMap((group) => {
    const generic = RESOURCE_CATALOG.filter((d) => d.group === group && isServed(d)).map((d) => ({
      id: `reslist:${d.key}`,
      label: resourceLabel(d.key),
      icon: d.icon,
    }))
    const children = [...(richByGroup[group] ?? []), ...generic]
    return children.length ? [{ id: `group:${group}`, label: t(`groups.${group}`), icon: GROUP_ICON[group], children }] : []
  })

  return (
    <>
      {/* Above the long tree, where a failed count query stays visible. */}
      <SidebarErrors errors={queries} />
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
    </>
  )
}

import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { NS } from './i18n'
import { openTopology, topologyTabId } from './tabs'

/** The cluster's namespaces; selecting one opens its topology. */
export function TopologySidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const namespaces = useProviderQuery((p) => p.cluster.listNamespaces(), [], { topics: ['namespaces'] })

  const nodes: TreeNode[] = [
    {
      id: 'topology-namespaces',
      label: t('sidebar.namespaces'),
      icon: 'symbol-namespace',
      count: namespaces.data?.length,
      children: (namespaces.data ?? []).map((n) => ({
        id: topologyTabId(n.name),
        label: n.name,
        icon: 'type-hierarchy',
      })),
    },
  ]

  return (
    <>
      <Tree
        nodes={nodes}
        selectedId={activeTabId}
        defaultExpanded={['topology-namespaces']}
        onSelect={(node) => {
          const name = (namespaces.data ?? []).find((n) => topologyTabId(n.name) === node.id)?.name
          if (name) openTopology(name)
        }}
      />
      <SidebarErrors errors={[namespaces.error]} />
    </>
  )
}

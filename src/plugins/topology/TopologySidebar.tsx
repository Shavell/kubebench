import { useTranslation } from 'react-i18next'
import { useNamespaceNames } from '@/core/namespaceNames'
import { useLayoutStore } from '@/core/store/layoutStore'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { NS } from './i18n'
import { openTopology, topologyTabId } from './tabs'

/** The cluster's namespaces; selecting one opens its topology. */
export function TopologySidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const namespaces = useNamespaceNames()

  const nodes: TreeNode[] = [
    {
      id: 'topology-namespaces',
      label: t('sidebar.namespaces'),
      icon: 'symbol-namespace',
      count: namespaces.names.length,
      children: namespaces.names.map((name) => ({
        id: topologyTabId(name),
        label: name,
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
          const name = namespaces.names.find((n) => topologyTabId(n) === node.id)
          if (name) openTopology(name)
        }}
      />
      <SidebarErrors errors={[namespaces.error]} />
    </>
  )
}

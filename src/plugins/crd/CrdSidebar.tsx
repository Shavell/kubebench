import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'

export function CrdSidebar() {
  const openTab = useLayoutStore((s) => s.openTab)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data: crds } = useProviderQuery((p) => p.crd.listCrds(), [])

  const groups = new Map<string, TreeNode[]>()
  for (const crd of crds ?? []) {
    const children = groups.get(crd.group) ?? []
    children.push({
      id: `crd:${crd.name}`,
      label: crd.kind,
      icon: 'symbol-class',
    })
    groups.set(crd.group, children)
  }

  const nodes: TreeNode[] = [
    { id: 'crd-all', label: 'All Definitions', icon: 'list-tree', count: crds?.length },
    ...[...groups.entries()].map(([group, children]) => ({
      id: `group:${group}`,
      label: group,
      icon: 'symbol-namespace',
      children,
    })),
  ]

  return (
    <Tree
      nodes={nodes}
      selectedId={activeTabId}
      onSelect={(node) => {
        if (node.id === 'crd-all') {
          openTab({
            id: 'crd-all',
            title: 'Custom Resource Definitions',
            icon: 'list-tree',
            kind: 'crd-list',
          })
        } else if (node.id.startsWith('crd:')) {
          const crdName = node.id.slice('crd:'.length)
          openTab({
            id: node.id,
            title: node.label,
            icon: 'symbol-class',
            kind: 'crd-instances',
            props: { crdName },
          })
        }
      }}
    />
  )
}

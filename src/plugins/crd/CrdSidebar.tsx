import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'
import { NS } from './i18n'
import {
  DEFINITIONS_TAB_ID,
  INSTANCES_TAB_PREFIX,
  openDefinitionsTab,
  openInstancesTab,
} from './tabs'

export function CrdSidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data: crds } = useProviderQuery((p) => p.crd.listCrds(), [])

  // Tree node ids equal the ids of the tabs they open, so the open tab is highlighted.
  const groups = new Map<string, TreeNode[]>()
  for (const crd of crds ?? []) {
    const children = groups.get(crd.group) ?? []
    children.push({
      id: `${INSTANCES_TAB_PREFIX}${crd.name}`,
      label: crd.kind,
      icon: 'symbol-class',
    })
    groups.set(crd.group, children)
  }

  const nodes: TreeNode[] = [
    {
      id: DEFINITIONS_TAB_ID,
      label: t('sidebar.allDefinitions'),
      icon: 'list-tree',
      count: crds?.length,
    },
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
        if (node.id === DEFINITIONS_TAB_ID) {
          openDefinitionsTab()
        } else if (node.id.startsWith(INSTANCES_TAB_PREFIX)) {
          openInstancesTab(node.id.slice(INSTANCES_TAB_PREFIX.length), node.label)
        }
      }}
    />
  )
}

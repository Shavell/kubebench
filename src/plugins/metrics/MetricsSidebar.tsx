import { useTranslation } from 'react-i18next'
import { reasonMessageText } from '@/core/reasons'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { NS } from './i18n'
import { NODE_TAB_PREFIX, openNodeMetricsTab, openOverviewTab, OVERVIEW_TAB_ID } from './tabs'

export function MetricsSidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data: nodes, error: nodesError } = useProviderQuery((p) => p.cluster.listNodes(), [])
  const { data: sources, error: sourcesError } = useProviderQuery((p) => p.metrics.listSources(), [])

  // Tree node ids equal the ids of the tabs they open, so the open tab is highlighted.
  const tree: TreeNode[] = [
    { id: OVERVIEW_TAB_ID, label: t('overview.title'), icon: 'dashboard' },
    {
      id: 'metrics-nodes',
      label: t('sidebar.nodes'),
      icon: 'server',
      children: (nodes ?? []).map((n) => ({
        id: `${NODE_TAB_PREFIX}${n.name}`,
        label: n.name,
        icon: 'pulse',
      })),
    },
    {
      id: 'metrics-sources',
      label: t('sidebar.sources'),
      icon: 'plug',
      children: (sources ?? []).map((s) => ({
        id: `src-${s.id}`,
        label: s.detail && s.available ? `${s.name} · ${s.detail}` : s.name,
        description: !s.available && s.unavailable ? reasonMessageText(s.unavailable) : undefined,
        icon: s.available ? 'check' : 'circle-slash',
      })),
    },
  ]

  return (
    <>
      <Tree
        nodes={tree}
        selectedId={activeTabId}
        onSelect={(node) => {
          if (node.id === OVERVIEW_TAB_ID) {
            openOverviewTab()
          } else if (node.id.startsWith(NODE_TAB_PREFIX)) {
            openNodeMetricsTab(node.id.slice(NODE_TAB_PREFIX.length))
          }
        }}
      />
      <SidebarErrors errors={[nodesError, sourcesError]} />
    </>
  )
}

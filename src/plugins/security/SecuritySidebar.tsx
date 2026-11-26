import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { usePluginQuery } from '@/data/pluginData'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { securityData } from './data'
import { NS } from './i18n'
import { openSecurityTab, tabId, type SecurityTab } from './tabs'

const PAGES: { tab: SecurityTab; icon: string }[] = [
  { tab: 'overview', icon: 'shield' },
  { tab: 'findings', icon: 'warning' },
  { tab: 'checks', icon: 'checklist' },
  { tab: 'accepted', icon: 'pass' },
]

export function SecuritySidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const report = usePluginQuery(() => securityData().report(), [])
  const accepted = usePluginQuery(() => securityData().accepted(), [])
  const counts = report.data?.counts

  const count: Record<SecurityTab, number | undefined> = {
    overview: undefined,
    findings: counts ? counts.total - counts.accepted : undefined,
    checks: undefined,
    accepted: accepted.data?.length,
  }
  const nodes: TreeNode[] = PAGES.map(({ tab, icon }) => ({
    id: tabId(tab),
    label: t(`sidebar.${tab}`),
    icon,
    count: count[tab],
    description: tab === 'overview' && report.data?.score.overall != null ? t('sidebar.score', { score: report.data.score.overall }) : undefined,
  }))

  return (
    <>
      <Tree
        nodes={nodes}
        selectedId={activeTabId}
        onSelect={(node) => {
          const page = PAGES.find((p) => tabId(p.tab) === node.id)
          if (page) openSecurityTab(page.tab)
        }}
      />
      <SidebarErrors errors={[report, accepted]} />
    </>
  )
}

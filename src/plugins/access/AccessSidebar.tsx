import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { usePluginQuery } from '@/data/pluginData'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { accessData } from './data'
import { RBAC_SCOPE } from './data/topics'
import { NS } from './i18n'
import { listTabIcon, listTabId, openListTab, type ListTab } from './tabs'

const LISTS: ListTab[] = ['whoCan', 'subjects', 'risky', 'graph']

export function AccessSidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const subjects = usePluginQuery(() => accessData().listSubjects(), [], RBAC_SCOPE)
  const risky = usePluginQuery(() => accessData().listRiskyRoles(), [], RBAC_SCOPE)

  const counts: Partial<Record<ListTab, number>> = {
    subjects: subjects.data?.length,
    risky: risky.data?.filter((r) => r.severity === 'critical').length,
  }
  const nodes: TreeNode[] = LISTS.map((id) => ({
    id: listTabId(id),
    label: t(`sidebar.${id}`),
    icon: listTabIcon(id),
    count: counts[id],
  }))

  return (
    <>
      <Tree
        nodes={nodes}
        selectedId={activeTabId}
        onSelect={(node) => {
          const target = LISTS.find((l) => listTabId(l) === node.id)
          if (target) openListTab(target)
        }}
      />
      <SidebarErrors errors={[subjects, risky]} />
    </>
  )
}

import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Icon } from '@/shared/Icon'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { EnvironmentDot } from './ClusterBadges'
import { CLUSTERS_NS } from './namespace'
import { INVENTORY_TAB_KIND, openInventoryTab } from './tabs'

/** The clusters the user can see, with environment and active marker; any row opens the inventory. */
export function ClustersSidebar() {
  const { t } = useTranslation(CLUSTERS_NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data: clusters, error } = useProviderQuery((p) => p.inventory.listClusters(), [])

  const nodes: TreeNode[] = [
    {
      id: INVENTORY_TAB_KIND,
      label: t('sidebar.clusters'),
      icon: 'server-environment',
      count: clusters?.length,
      children: (clusters ?? []).map((c) => ({
        id: `cluster:${c.name}`,
        label: c.name,
        icon: c.error ? 'warning' : c.readOnly ? 'lock' : 'vm',
        suffix: (
          <>
            <EnvironmentDot environment={c.environment} />
            {c.active && <Icon name="pass-filled" size={13} className="clusters__active" />}
          </>
        ),
      })),
    },
  ]

  return (
    <>
      <Tree nodes={nodes} selectedId={activeTabId} onSelect={openInventoryTab} />
      <SidebarErrors errors={[error]} />
    </>
  )
}

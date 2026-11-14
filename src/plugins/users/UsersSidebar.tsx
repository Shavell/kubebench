import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'
import { USERS_NS } from './namespace'
import { openUsersTab } from './tabs'

/** The user accounts; any row opens the Users page. */
export function UsersSidebar() {
  const { t } = useTranslation(USERS_NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data: users } = useProviderQuery((p) => p.users.listUsers(), [])

  const nodes: TreeNode[] = [
    {
      id: 'users',
      label: t('sidebar.users'),
      icon: 'organization',
      count: users?.length,
      children: (users ?? []).map((u) => ({
        id: `user:${u.id}`,
        label: u.username,
        icon: u.role === 'admin' ? 'shield' : 'account',
      })),
    },
  ]
  return <Tree nodes={nodes} selectedId={activeTabId} onSelect={openUsersTab} />
}

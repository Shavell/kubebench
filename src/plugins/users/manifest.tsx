import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { UsersPage } from './UsersPage'

/** Admin-only user management; registered at bootstrap when role === admin. */
export const usersPlugin: PluginManifest = {
  id: 'users',
  name: 'Users',
  activityBar: { icon: 'organization', tooltip: 'Users', order: 5 },
  sidebar: {
    id: 'users-explorer',
    title: 'Administration',
    component: UsersSidebar,
  },
  tabRenderers: [{ kind: 'users', component: UsersPage }],
  commands: [
    {
      id: 'users.open',
      title: 'Users: Manage Users',
      icon: 'organization',
      run: openUsersTab,
    },
  ],
}

function openUsersTab(): void {
  useLayoutStore.getState().openTab({ id: 'users', title: 'Users', icon: 'organization', kind: 'users' })
}

function UsersSidebar() {
  return (
    <div style={{ padding: '4px 0' }}>
      <button
        className="kb-tree__row"
        style={{ width: '100%', background: 'transparent', border: 'none', color: 'inherit', cursor: 'pointer' }}
        onClick={openUsersTab}
      >
        <span className="kb-tree__twistie" />
        <span className="kb-tree__label" style={{ paddingLeft: 8 }}>
          Users &amp; Access
        </span>
      </button>
    </div>
  )
}

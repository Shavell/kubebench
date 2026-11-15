import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { USERS_NS } from './namespace'
import { openUsersTab, USERS_TAB_KIND } from './tabs'
import { UsersSidebar } from './UsersSidebar'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const UsersPage = lazyComponent(() => import('./UsersPage').then((m) => m.UsersPage))

/**
 * Admin-only user management: registered at boot only for global admins
 * (main.tsx → adminPlugins). Display strings are keys in the "users"
 * namespace.
 */
export const usersPlugin: PluginManifest = {
  id: USERS_NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'core',
  core: true,
  locales: { en, tr },
  activityBar: { icon: 'organization', tooltip: 'manifest.tooltip', order: 5 },
  sidebar: { id: 'users-explorer', title: 'manifest.sidebarTitle', component: UsersSidebar },
  tabRenderers: [{ kind: USERS_TAB_KIND, component: UsersPage }],
  commands: [{ id: 'users.open', title: 'commands.open', icon: 'organization', run: openUsersTab }],
}

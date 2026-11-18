import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { AuthSettingsSection } from './AuthSettingsSection'
import { AUTH_NS } from './namespace'
import { AUTH_TAB_KIND, openAuthenticationTab } from './tabs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const AuthenticationPage = lazyComponent(() => import('./AuthenticationPage').then((m) => m.AuthenticationPage))

/**
 * Settings > Authentication (design §3.6): the sign-in methods, the group
 * mappings that grant external users their roles and the "who would I be"
 * preview. Admin-only, registered at boot for global admins (adminPlugins).
 */
export const authenticationPlugin: PluginManifest = {
  id: AUTH_NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'core',
  core: true,
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  tabRenderers: [{ kind: AUTH_TAB_KIND, component: AuthenticationPage }],
  settingsSections: [{ id: 'authentication', title: 'settings.title', component: AuthSettingsSection }],
  commands: [{ id: 'authentication.open', title: 'commands.open', icon: 'shield', run: openAuthenticationTab }],
}

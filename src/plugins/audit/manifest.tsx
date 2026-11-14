import type { PluginManifest } from '@/core/plugin/types'
import { AuditPage } from './AuditPage'
import { AuditSidebar } from './AuditSidebar'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { AUDIT_NS } from './namespace'
import { AUDIT_TAB_KIND, openAuditTab } from './tabs'

/**
 * The audit log (design §3.4). Admin-only: registered at boot only for
 * global admins (main.tsx → adminPlugins). Display strings are keys in
 * the "audit" namespace.
 */
export const auditPlugin: PluginManifest = {
  id: AUDIT_NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'core',
  core: true,
  locales: { en, tr },
  activityBar: { icon: 'history', tooltip: 'manifest.tooltip', order: 6 },
  sidebar: { id: 'audit-recent', title: 'manifest.sidebarTitle', component: AuditSidebar },
  tabRenderers: [{ kind: AUDIT_TAB_KIND, component: AuditPage }],
  commands: [{ id: 'audit.open', title: 'commands.open', icon: 'history', run: openAuditTab }],
}

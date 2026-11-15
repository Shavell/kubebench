import type { PluginManifest } from '@/core/plugin/types'
import { ArgoSidebar } from './ArgoSidebar'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { ApplicationDetailPage } from './pages/ApplicationDetailPage'
import { ApplicationSetsPage } from './pages/ApplicationSetsPage'
import { ApplicationsPage } from './pages/ApplicationsPage'
import { ProjectsPage } from './pages/ProjectsPage'
import {
  APPLICATION_SETS_TAB_ID,
  APPLICATIONS_TAB_ID,
  openApplicationSetsTab,
  openApplicationsTab,
  openProjectsTab,
  PROJECTS_TAB_ID,
} from './tabs'

/**
 * Argo CD (design §7.5). Needs the `argocd` backend plugin: on clusters
 * without the Argo CD CRDs the plugin is hidden and the Extensions view
 * shows why; pages opened before that show the reason as Unavailable.
 * Display strings are i18n keys in the plugin's namespace.
 */
export const argocdPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'gitops',
  backendPlugin: NS,
  locales: { en, tr },
  activityBar: { icon: 'git-merge', tooltip: 'manifest.tooltip' },
  sidebar: { id: 'argocd-explorer', title: 'manifest.sidebarTitle', component: ArgoSidebar },
  tabRenderers: [
    { kind: APPLICATIONS_TAB_ID, component: ApplicationsPage },
    { kind: 'argocd-application', component: ApplicationDetailPage },
    { kind: PROJECTS_TAB_ID, component: ProjectsPage },
    { kind: APPLICATION_SETS_TAB_ID, component: ApplicationSetsPage },
  ],
  commands: [
    { id: 'argocd.openApplications', title: 'commands.openApplications', icon: 'layers', run: openApplicationsTab },
    { id: 'argocd.openProjects', title: 'commands.openProjects', icon: 'project', run: openProjectsTab },
    { id: 'argocd.openApplicationSets', title: 'commands.openApplicationSets', icon: 'files', run: openApplicationSetsTab },
  ],
}

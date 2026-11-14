import type { PluginManifest } from '@/core/plugin/types'
import { HelmSidebar } from './HelmSidebar'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { ReleaseListPage } from './pages/ReleaseListPage'
import { ReleaseDetailPage } from './pages/ReleaseDetailPage'
import { RepoListPage } from './pages/RepoListPage'
import { openReleasesTab, openReposTab } from './tabs'

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const helmPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'workloads',
  locales: { en, tr },
  activityBar: { icon: 'package', tooltip: 'manifest.tooltip', order: 3 },
  sidebar: { id: 'helm-explorer', title: 'manifest.sidebarTitle', component: HelmSidebar },
  tabRenderers: [
    { kind: 'helm-releases', component: ReleaseListPage },
    { kind: 'helm-release-detail', component: ReleaseDetailPage },
    { kind: 'helm-repos', component: RepoListPage },
  ],
  commands: [
    {
      id: 'helm.openReleases',
      title: 'commands.openReleases',
      icon: 'package',
      run: openReleasesTab,
    },
    {
      id: 'helm.openRepos',
      title: 'commands.openRepos',
      icon: 'repo',
      run: openReposTab,
    },
  ],
}

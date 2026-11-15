import type { PluginManifest } from '@/core/plugin/types'
import { HelmSidebar } from './HelmSidebar'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { InstallPage } from './pages/install/InstallPage'
import { ReleaseDetailPage } from './pages/ReleaseDetailPage'
import { ReleaseListPage } from './pages/ReleaseListPage'
import { RepoListPage } from './pages/RepoListPage'
import { UpgradePage } from './pages/UpgradePage'
import { openInstallTab, openReleasesTab, openReposTab } from './tabs'

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const helmPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  version: '0.5.0',
  category: 'workloads',
  backendPlugin: 'helm',
  locales: { en, tr },
  activityBar: { icon: 'package', tooltip: 'manifest.tooltip', order: 3 },
  sidebar: { id: 'helm-explorer', title: 'manifest.sidebarTitle', component: HelmSidebar },
  tabRenderers: [
    { kind: 'helm-releases', component: ReleaseListPage },
    { kind: 'helm-release-detail', component: ReleaseDetailPage },
    { kind: 'helm-repos', component: RepoListPage },
    { kind: 'helm-install', component: InstallPage },
    { kind: 'helm-upgrade', component: UpgradePage },
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
    {
      id: 'helm.install',
      title: 'commands.install',
      icon: 'cloud-download',
      run: openInstallTab,
    },
  ],
}

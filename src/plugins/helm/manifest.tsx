import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { HelmSidebar } from './HelmSidebar'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { openInstallTab, openReleasesTab, openReposTab } from './tabs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const InstallPage = lazyComponent(() => import('./pages/install/InstallPage').then((m) => m.InstallPage))
const ReleaseDetailPage = lazyComponent(() => import('./pages/ReleaseDetailPage').then((m) => m.ReleaseDetailPage))
const ReleaseListPage = lazyComponent(() => import('./pages/ReleaseListPage').then((m) => m.ReleaseListPage))
const RepoListPage = lazyComponent(() => import('./pages/RepoListPage').then((m) => m.RepoListPage))
const UpgradePage = lazyComponent(() => import('./pages/UpgradePage').then((m) => m.UpgradePage))

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

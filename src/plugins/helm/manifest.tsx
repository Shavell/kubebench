import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { HelmSidebar } from './HelmSidebar'
import { ReleaseListPage } from './pages/ReleaseListPage'
import { ReleaseDetailPage } from './pages/ReleaseDetailPage'
import { RepoListPage } from './pages/RepoListPage'

export const helmPlugin: PluginManifest = {
  id: 'helm',
  name: 'Helm',
  activityBar: { icon: 'package', tooltip: 'Helm', order: 3 },
  sidebar: { id: 'helm-explorer', title: 'Helm', component: HelmSidebar },
  tabRenderers: [
    { kind: 'helm-releases', component: ReleaseListPage },
    { kind: 'helm-release-detail', component: ReleaseDetailPage },
    { kind: 'helm-repos', component: RepoListPage },
  ],
  commands: [
    {
      id: 'helm.openReleases',
      title: 'Helm: Open Releases',
      icon: 'package',
      run: () =>
        useLayoutStore.getState().openTab({
          id: 'helm-releases',
          title: 'Helm Releases',
          icon: 'package',
          kind: 'helm-releases',
        }),
    },
    {
      id: 'helm.openRepos',
      title: 'Helm: Open Repositories',
      icon: 'repo',
      run: () =>
        useLayoutStore.getState().openTab({
          id: 'helm-repos',
          title: 'Helm Repositories',
          icon: 'repo',
          kind: 'helm-repos',
        }),
    },
  ],
}

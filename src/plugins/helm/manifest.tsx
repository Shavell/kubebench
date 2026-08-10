import type { PluginManifest } from '@/core/plugin/types'
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
}

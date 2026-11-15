import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { usePluginQuery } from '@/data/pluginData'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { helmData } from './data'
import { NS } from './i18n'
import {
  INSTALL_TAB_ID,
  openInstallTab,
  openReleasesTab,
  openReleaseTab,
  openReposTab,
  RELEASES_TAB_ID,
  releaseTabId,
  REPOS_TAB_ID,
} from './tabs'

export function HelmSidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const releasesQuery = usePluginQuery(() => helmData().listReleases(), [], { topics: ['secrets'] })
  const reposQuery = usePluginQuery(() => helmData().listRepos(), [])
  const releases = releasesQuery.data ?? []

  // Tree node ids equal the ids of the tabs they open, so the open tab is highlighted.
  const releaseById = new Map(releases.map((r) => [releaseTabId(r.namespace, r.name), r]))
  const nodes: TreeNode[] = [
    {
      id: RELEASES_TAB_ID,
      label: t('sidebar.releases'),
      icon: 'package',
      count: releasesQuery.data?.length,
      children: releases.map((r) => ({
        id: releaseTabId(r.namespace, r.name),
        label: r.name,
        description: r.namespace,
        icon: r.status === 'failed' ? 'error' : r.status === 'deployed' ? 'circle-filled' : 'circle-outline',
      })),
    },
    { id: REPOS_TAB_ID, label: t('sidebar.repos'), icon: 'repo', count: reposQuery.data?.length },
    { id: INSTALL_TAB_ID, label: t('sidebar.install'), icon: 'cloud-download' },
  ]

  return (
    <>
      <Tree
        nodes={nodes}
        selectedId={activeTabId}
        onSelect={(node) => {
          const release = releaseById.get(node.id)
          if (release) openReleaseTab(release.namespace, release.name)
          else if (node.id === RELEASES_TAB_ID) openReleasesTab()
          else if (node.id === REPOS_TAB_ID) openReposTab()
          else if (node.id === INSTALL_TAB_ID) openInstallTab()
        }}
      />
      <SidebarErrors errors={[releasesQuery.error, reposQuery.error]} />
    </>
  )
}

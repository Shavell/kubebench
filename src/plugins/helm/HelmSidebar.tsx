import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { NS } from './i18n'
import { openReleasesTab, openReleaseTab, openReposTab, releaseTabId } from './tabs'

export function HelmSidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const releasesQuery = useProviderQuery((p) => p.helm.listReleases(), [])
  const reposQuery = useProviderQuery((p) => p.helm.listRepositories(), [])
  const releases = releasesQuery.data ?? []

  // Tree node ids equal the ids of the tabs they open, so the open tab is highlighted.
  const releaseById = new Map(releases.map((r) => [releaseTabId(r.namespace, r.name), r]))
  const nodes: TreeNode[] = [
    {
      id: 'releases',
      label: t('sidebar.releases'),
      icon: 'package',
      count: releasesQuery.data?.length,
      children: releases.map((r) => ({
        id: releaseTabId(r.namespace, r.name),
        label: r.name,
        description: r.namespace,
        icon: r.status === 'failed' ? 'error' : 'circle-filled',
      })),
    },
    {
      id: 'repos',
      label: t('sidebar.repos'),
      icon: 'repo',
      count: reposQuery.data?.length,
    },
  ]

  return (
    <>
      <Tree
        nodes={nodes}
        selectedId={activeTabId}
        onSelect={(node) => {
          const release = releaseById.get(node.id)
          if (release) openReleaseTab(release.namespace, release.name)
          else if (node.id === 'releases') openReleasesTab()
          else if (node.id === 'repos') openReposTab()
        }}
      />
      <SidebarErrors errors={[releasesQuery.error, reposQuery.error]} />
    </>
  )
}

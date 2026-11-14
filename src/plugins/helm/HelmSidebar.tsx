import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'
import { NS } from './i18n'
import { openReleasesTab, openReleaseTab, openReposTab, RELEASE_TAB_PREFIX } from './tabs'

export function HelmSidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data: releases } = useProviderQuery((p) => p.helm.listReleases(), [])
  const { data: repos } = useProviderQuery((p) => p.helm.listRepositories(), [])

  const nodes: TreeNode[] = [
    {
      id: 'releases',
      label: t('sidebar.releases'),
      icon: 'package',
      count: releases?.length,
      children: (releases ?? []).map((r) => ({
        id: `${RELEASE_TAB_PREFIX}${r.name}`,
        label: r.name,
        icon: r.status === 'failed' ? 'error' : 'circle-filled',
      })),
    },
    {
      id: 'repos',
      label: t('sidebar.repos'),
      icon: 'repo',
      count: repos?.length,
    },
  ]

  return (
    <Tree
      nodes={nodes}
      selectedId={activeTabId}
      onSelect={(node) => {
        if (node.id === 'releases') {
          openReleasesTab()
        } else if (node.id === 'repos') {
          openReposTab()
        } else if (node.id.startsWith(RELEASE_TAB_PREFIX)) {
          openReleaseTab(node.id.slice(RELEASE_TAB_PREFIX.length))
        }
      }}
    />
  )
}

import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { Tree, type TreeNode } from '@/shared/Tree'

export function HelmSidebar() {
  const openTab = useLayoutStore((s) => s.openTab)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data: releases } = useProviderQuery((p) => p.helm.listReleases(), [])
  const { data: repos } = useProviderQuery((p) => p.helm.listRepositories(), [])

  const nodes: TreeNode[] = [
    {
      id: 'releases',
      label: 'Releases',
      icon: 'package',
      count: releases?.length,
      children: (releases ?? []).map((r) => ({
        id: `release:${r.name}`,
        label: r.name,
        icon: r.status === 'failed' ? 'error' : 'circle-filled',
      })),
    },
    {
      id: 'repos',
      label: 'Repositories',
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
          openTab({ id: 'helm-releases', title: 'Helm Releases', icon: 'package', kind: 'helm-releases' })
        } else if (node.id === 'repos') {
          openTab({ id: 'helm-repos', title: 'Helm Repositories', icon: 'repo', kind: 'helm-repos' })
        } else if (node.id.startsWith('release:')) {
          const name = node.id.slice('release:'.length)
          openTab({
            id: node.id,
            title: name,
            icon: 'package',
            kind: 'helm-release-detail',
            props: { name },
          })
        }
      }}
    />
  )
}

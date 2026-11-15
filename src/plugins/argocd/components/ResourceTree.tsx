import { useTranslation } from 'react-i18next'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { Tree, type TreeNode } from '@/shared/Tree'
import type { ArgoResource } from '../data/types'
import { NS } from '../i18n'
import { openManagedResource, openManagedYaml } from '../tabs'
import { StateBadge } from './StatusBadges'

const resourceId = (r: ArgoResource) => `res|${r.group}|${r.kind}|${r.namespace}|${r.name}`

/**
 * The application's managed resources as a tree: application → namespace
 * → kind → resource, each with its sync and health state. Selecting a
 * resource opens it in KubeBench (detail page, else YAML).
 */
export function ResourceTree({ appName, resources }: { appName: string; resources: ArgoResource[] }) {
  const { t } = useTranslation(NS)
  if (resources.length === 0) {
    return <EmptyState icon="list-tree" title={t('detail.resources.empty')} hint={t('detail.resources.emptyHint')} />
  }

  const byId = new Map(resources.map((r) => [resourceId(r), r]))
  const namespaces = new Map<string, Map<string, ArgoResource[]>>()
  for (const r of resources) {
    const kinds = namespaces.get(r.namespace) ?? new Map<string, ArgoResource[]>()
    kinds.set(r.kind, [...(kinds.get(r.kind) ?? []), r])
    namespaces.set(r.namespace, kinds)
  }

  const leaf = (r: ArgoResource): TreeNode => ({
    id: resourceId(r),
    label: r.name,
    icon: r.hook ? 'debug-step-into' : 'symbol-field',
    description: r.requiresPruning ? t('detail.resources.requiresPruning') : undefined,
    suffix: (
      <span className="argocd-tree-badges">
        <StateBadge value={r.status} />
        <StateBadge value={r.health} />
        <Button
          variant="secondary"
          icon="file-code"
          className="argocd-tree-yaml"
          title={t('detail.resources.yaml')}
          aria-label={t('detail.resources.yamlOf', { name: r.name })}
          onClick={(e) => {
            e.stopPropagation()
            openManagedYaml(r)
          }}
        />
      </span>
    ),
  })

  const nsNodes: TreeNode[] = [...namespaces.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([namespace, kinds]) => ({
      id: `ns|${namespace}`,
      label: namespace || t('detail.resources.clusterScoped'),
      icon: namespace ? 'symbol-namespace' : 'globe',
      count: [...kinds.values()].reduce((n, list) => n + list.length, 0),
      children: [...kinds.entries()]
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([kind, list]) => ({
          id: `kind|${namespace}|${kind}`,
          label: kind,
          icon: 'symbol-class',
          count: list.length,
          children: list.map(leaf),
        })),
    }))

  const root: TreeNode = { id: 'app', label: appName, icon: 'layers', count: resources.length, children: nsNodes }
  const expanded = ['app', ...nsNodes.flatMap((n) => [n.id, ...(n.children ?? []).map((k) => k.id)])]

  return (
    <div className="argocd-tree">
      <Tree
        nodes={[root]}
        defaultExpanded={expanded}
        onSelect={(node) => {
          const r = byId.get(node.id)
          if (r) openManagedResource(r)
        }}
      />
    </div>
  )
}

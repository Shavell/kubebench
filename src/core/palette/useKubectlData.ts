import { orderNamespaces } from '@/core/store/namespaceOrder'
import { useNamespacePrefs } from '@/core/store/namespacePrefsStore'
import { useProviderQuery } from '@/data/DataContext'
import type { ResourceKindContribution } from '@/core/plugin/types'
import type { ClusterInfo, KubectlNeeds, PodInfo } from './kubectlRows'

export interface KubectlData {
  namespaces: string[] | null
  pods: PodInfo[] | null
  objects: { namespace?: string; name: string }[] | null
  clusters: ClusterInfo[] | null
  loadError: string | null
}

const none = () => Promise.resolve(null)

/**
 * Loads what a palette command needs (namespaces, Pods, a kind's objects,
 * clusters) and nothing else; `needs` is null while the palette shows no
 * command. Namespaces come favourites first, without hidden ones.
 */
export function useKubectlData(
  needs: KubectlNeeds<ResourceKindContribution> | null,
  labels: { bookmark: string; broken: string },
): KubectlData {
  const prefs = useNamespacePrefs()
  const podScope = needs?.pods ? needs.pods.namespace : null
  const objects = needs?.objects ?? null
  const objectKey = objects ? `${objects.kind.kind}/${objects.namespace}` : null

  const namespaces = useProviderQuery(
    (p) => (needs?.namespaces ? p.cluster.listNamespaces() : none()),
    [needs?.namespaces ?? false],
  )
  const pods = useProviderQuery(
    (p) => (podScope === null ? none() : p.cluster.listPods(podScope || undefined)),
    [podScope],
  )
  const objectList = useProviderQuery(
    (p) => (objects ? objects.kind.listObjects(p, objects.namespace || undefined) : none()),
    [objectKey],
  )
  const clusters = useProviderQuery((p) => (needs?.clusters ? p.inventory.listClusters() : none()), [needs?.clusters ?? false])

  const ordered = namespaces.data
    ? (({ pinned, others }) => [...pinned, ...others])(orderNamespaces(namespaces.data.map((n) => n.name), prefs))
    : null

  return {
    namespaces: ordered,
    pods: pods.data?.map((p) => ({ namespace: p.namespace, name: p.name, containers: p.containers.map((c) => c.name) })) ?? null,
    objects: objectList.data,
    clusters:
      clusters.data?.map((c) => ({
        name: c.name,
        active: c.active,
        disabledReason: c.error ? labels.broken : c.source === 'address' ? labels.bookmark : undefined,
      })) ?? null,
    loadError: namespaces.error ?? pods.error ?? objectList.error ?? clusters.error,
  }
}

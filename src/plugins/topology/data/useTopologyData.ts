import { useMemo } from 'react'
import { useProviderQuery, type QueryState } from '@/data/DataContext'
import type { DataProviders } from '@/data/providers'
import type { KubeObject } from '@/data/types'
import type { TopologySnapshot } from '../model/types'

/**
 * Loads the lists a namespace's topology is built from, each on its own,
 * so the graph fills in as they arrive and one list that cannot be read
 * (e.g. Secrets without RBAC) leaves the others standing. Pods,
 * Deployments and Services come from the read-only Kubernetes passthrough,
 * which keeps owner references, selectors and volumes; the other kinds
 * from the generic resource lists. Re-runs on refreshData().
 */

export type ListName = keyof TopologySnapshot

/** All lists, in the order their errors are reported. */
export const LIST_NAMES: ListName[] = [
  'pods',
  'deployments',
  'replicaSets',
  'statefulSets',
  'daemonSets',
  'services',
  'ingresses',
  'pvcs',
  'pvs',
  'configMaps',
  'secrets',
]

export interface ListError {
  list: ListName
  error: string
  code: string | null
}

export interface TopologyData {
  snapshot: TopologySnapshot
  /** Lists still loading. */
  pending: number
  errors: ListError[]
}

interface ItemList {
  items?: KubeObject[]
}

const passthrough = (p: DataProviders, path: string) =>
  p.cluster.k8sGet<ItemList>(path).then((list) => list.items ?? [])

function useList(load: (p: DataProviders) => Promise<KubeObject[]>, namespace: string): QueryState<KubeObject[]> {
  return useProviderQuery(load, [namespace])
}

export function useTopologyData(namespace: string): TopologyData {
  const ns = encodeURIComponent(namespace)
  const queries: Record<ListName, QueryState<KubeObject[]>> = {
    pods: useList((p) => passthrough(p, `/api/v1/namespaces/${ns}/pods`), namespace),
    deployments: useList((p) => passthrough(p, `/apis/apps/v1/namespaces/${ns}/deployments`), namespace),
    services: useList((p) => passthrough(p, `/api/v1/namespaces/${ns}/services`), namespace),
    replicaSets: useList((p) => p.cluster.listResource('replicasets', namespace), namespace),
    statefulSets: useList((p) => p.cluster.listResource('statefulsets', namespace), namespace),
    daemonSets: useList((p) => p.cluster.listResource('daemonsets', namespace), namespace),
    ingresses: useList((p) => p.cluster.listResource('ingresses', namespace), namespace),
    pvcs: useList((p) => p.cluster.listResource('persistentvolumeclaims', namespace), namespace),
    pvs: useList((p) => p.cluster.listResource('persistentvolumes'), namespace),
    configMaps: useList((p) => p.cluster.listResource('configmaps', namespace), namespace),
    secrets: useList((p) => p.cluster.listResource('secrets', namespace), namespace),
  }

  // A refresh keeps each list's previous data, so the snapshot (and the
  // graph built from it) only changes when a list's content arrives.
  const lists = LIST_NAMES.map((name) => queries[name].data)
  const snapshot = useMemo(
    () => Object.fromEntries(LIST_NAMES.map((name, i) => [name, lists[i] ?? undefined])) as TopologySnapshot,
    // eslint-disable-next-line react-hooks/exhaustive-deps
    lists,
  )
  const errors = LIST_NAMES.flatMap((list) => {
    const q = queries[list]
    return q.error ? [{ list, error: q.error, code: q.errorCode }] : []
  })
  const pending = LIST_NAMES.filter((name) => queries[name].loading && !queries[name].data).length
  return { snapshot, pending, errors }
}

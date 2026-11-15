import { useProviderQuery } from '@/data/DataContext'

/**
 * The container names of a pod, for container pickers; null asks for
 * none. `current` is always among them, also when the pod list does not
 * show it (init and ephemeral debug containers, a pod that is gone).
 */
export function usePodContainers(namespace: string, pod: string | null, current: string): string[] {
  const { data } = useProviderQuery(
    (p) => (pod ? p.cluster.listPods(namespace) : Promise.resolve([])),
    [namespace, pod],
  )
  const names = data?.find((p) => p.name === pod)?.containers.map((c) => c.name) ?? []
  return names.includes(current) ? names : [current, ...names]
}

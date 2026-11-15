import type { ResourceActionContribution, ResourceRef } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { NS } from './i18n'

/** The kinds whose pods "Logs (all pods)" follows, with their generic catalog keys. */
const WORKLOAD_KEYS: Record<string, string | null> = {
  Deployment: null, // a rich kind: its selector comes from the deployments list
  StatefulSet: 'statefulsets',
  DaemonSet: 'daemonsets',
  ReplicaSet: 'replicasets',
  Job: 'jobs',
}

export const WORKLOAD_LOG_KINDS = Object.keys(WORKLOAD_KEYS)

/** The workload a `workload-logs` tab derives its selector from, when opened without one. */
export interface WorkloadRef {
  kind: string
  name: string
}

/** The generic catalog key of a workload kind; undefined for kinds it does not cover. */
export function workloadKey(kind: string): string | null | undefined {
  return WORKLOAD_KEYS[kind]
}

/**
 * Opens the multi-pod log tab (kind `workload-logs`) of a workload. The tab
 * takes `{namespace, selector, title}`; this opener passes the workload
 * instead of the selector, which the tab reads from the workload's spec.
 */
export function openWorkloadLogs(ref: ResourceRef): void {
  const namespace = ref.namespace ?? ''
  const title = `${ref.kind} ${ref.name}`
  useLayoutStore.getState().openTab({
    id: `workload-logs:${ref.kind}:${namespace}/${ref.name}`,
    title,
    titleKey: 'workloadLogs.tabTitle',
    titleNs: NS,
    titleValues: { name: ref.name },
    icon: 'output',
    kind: 'workload-logs',
    props: { namespace, title, workload: { kind: ref.kind, name: ref.name } satisfies WorkloadRef },
  })
}

/** The "Logs (all pods)" row and toolbar action (design §6.4 resource actions). */
export const workloadLogsAction: ResourceActionContribution = {
  id: 'workload-logs',
  title: 'workloadLogs.action',
  icon: 'output',
  kinds: WORKLOAD_LOG_KINDS,
  permission: 'read',
  run: openWorkloadLogs,
}

interface LabelSelector {
  matchLabels?: Record<string, string>
  matchExpressions?: { key: string; operator: string; values?: string[] }[]
}

/**
 * A Kubernetes label selector in the string syntax of `?selector=`
 * (`app=web,tier in (a,b),!legacy`), or null when it selects nothing.
 */
export function selectorString(selector: LabelSelector | undefined): string | null {
  const parts = Object.entries(selector?.matchLabels ?? {}).map(([key, value]) => `${key}=${value}`)
  for (const { key, operator, values = [] } of selector?.matchExpressions ?? []) {
    switch (operator) {
      case 'In':
        parts.push(`${key} in (${values.join(',')})`)
        break
      case 'NotIn':
        parts.push(`${key} notin (${values.join(',')})`)
        break
      case 'Exists':
        parts.push(key)
        break
      case 'DoesNotExist':
        parts.push(`!${key}`)
        break
    }
  }
  return parts.length ? parts.join(',') : null
}

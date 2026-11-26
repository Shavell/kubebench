import { apiDelete, apiGet, apiPost, apiPut } from '../apiClient'
import type { WorkloadOpsProvider } from '../providers'
import type {
  CronJobRun,
  DebugContainer,
  DrainResult,
  NodeTaint,
  PodDeleteResult,
  RolloutHistory,
} from '../types'

/** Workload and node operations of the Go backend (design §7.3). */
export function createApiWorkloadOps(): WorkloadOpsProvider {
  return {
    cordonNode: async (name, unschedulable) => {
      await apiPost('/nodes/cordon', undefined, { name, unschedulable })
    },
    drainNode: (name, o) =>
      apiPost<DrainResult>('/nodes/drain', undefined, {
        name,
        ignoreDaemonSets: o.ignoreDaemonSets,
        deleteEmptyDirData: o.deleteEmptyDirData,
        force: o.force,
        gracePeriod: o.gracePeriod,
        timeoutSeconds: o.timeoutSeconds,
      }),
    setNodeTaints: async (name, taints) =>
      (await apiPut<{ taints: NodeTaint[] }>('/nodes/taints', { taints }, { name })).taints,
    deploymentHistory: (ns, name) => apiGet<RolloutHistory>('/deployments/history', { ns, name }),
    rollbackDeployment: async (ns, name, revision) => {
      await apiPost('/deployments/rollback', undefined, { ns, name, revision })
    },
    pauseDeployment: async (ns, name, paused) => {
      await apiPost('/deployments/pause', undefined, { ns, name, paused })
    },
    restartWorkload: async (kind, ns, name) => {
      await apiPost('/workloads/restart', undefined, { kind, ns, name })
    },
    scaleWorkload: async (kind, ns, name, replicas, apiVersion) => {
      await apiPost('/workloads/scale', undefined, { apiVersion, kind, ns, name, replicas })
    },
    forceDeletePod: async (ns, name) => {
      await apiDelete('/pods', { ns, name, force: true })
    },
    deletePods: async (items, force) =>
      (await apiPost<{ results: PodDeleteResult[] }>('/pods/bulk-delete', { items, force })).results,
    addDebugContainer: (ns, pod, { image, target }) =>
      apiPost<DebugContainer>('/pods/debug', undefined, { ns, pod, image, target }),
    triggerCronJob: (ns, name) => apiPost<CronJobRun>('/cronjobs/trigger', undefined, { ns, name }),
    suspendCronJob: async (ns, name, suspend) => {
      await apiPost('/cronjobs/suspend', undefined, { ns, name, suspend })
    },
    createNamespace: async (name, labels) => {
      await apiPost('/namespaces', { name, labels })
    },
    deleteNamespace: async (name, force) => {
      // Only a deliberate override is sent; the server refuses protected namespaces otherwise.
      await apiDelete('/namespaces', { name, force: force || undefined })
    },
  }
}

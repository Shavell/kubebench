import type {
  ArgoApplicationDetail,
  ArgoApplicationSet,
  ArgoApplicationSummary,
  ArgoProject,
  ArgoSyncRequest,
} from './types'

/** The Argo CD plugin's data capability (mock and live implement it). */
export interface ArgoCdProvider {
  listApplications(): Promise<ArgoApplicationSummary[]>
  getApplication(namespace: string, name: string): Promise<ArgoApplicationDetail>
  /** Requests a sync; rejects with `conflict` (409) while an operation runs. */
  syncApplication(namespace: string, name: string, request: ArgoSyncRequest): Promise<ArgoApplicationDetail>
  /** Asks Argo CD to compare again; `hard` also regenerates the manifests. */
  refreshApplication(namespace: string, name: string, hard: boolean): Promise<ArgoApplicationDetail>
  /** Syncs back to a history entry; rejects with `conflict` while automated sync is on. */
  rollbackApplication(namespace: string, name: string, historyId: number): Promise<ArgoApplicationDetail>
  /** Deletes the application; `cascade` also deletes the resources it manages. */
  deleteApplication(namespace: string, name: string, cascade: boolean): Promise<void>
  listProjects(): Promise<ArgoProject[]>
  listApplicationSets(): Promise<ArgoApplicationSet[]>
}

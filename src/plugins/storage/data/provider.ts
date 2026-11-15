import type {
  CreateResult,
  CsiDriverSummary,
  PvcRequest,
  PvcSummary,
  PvRequest,
  PvSummary,
  StorageClassRequest,
  StorageOverview,
  VolumeSnapshot,
} from './types'

/** Options of the create calls: `dryRun` validates on the server without storing. */
export interface CreateOptions {
  dryRun?: boolean
}

/**
 * The storage plugin's data capability (design §7.6). The live
 * implementation calls /api/plugins/storage; the mock one changes the mock
 * cluster's storage objects, so created resources show up everywhere.
 */
export interface StorageProvider {
  overview(): Promise<StorageOverview>
  listPvcs(namespace?: string): Promise<PvcSummary[]>
  listPvs(): Promise<PvSummary[]>
  listCsiDrivers(): Promise<CsiDriverSummary[]>
  /** Rejects with `capability_unavailable` while the snapshot CRDs are missing. */
  listSnapshots(namespace?: string): Promise<VolumeSnapshot[]>
  createPvc(request: PvcRequest, options?: CreateOptions): Promise<CreateResult>
  createPv(request: PvRequest, options?: CreateOptions): Promise<CreateResult>
  createStorageClass(request: StorageClassRequest, options?: CreateOptions): Promise<CreateResult>
  /** Raises the claim's request; only for Bound claims whose class allows expansion. */
  resizePvc(namespace: string, name: string, size: string): Promise<PvcSummary>
}

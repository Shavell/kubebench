/**
 * Shapes of the storage plugin's API (backend/internal/plugins/storage,
 * design §7.6). Byte counts are numbers; quantities keep the text the
 * cluster stores ("10Gi").
 */

export interface StorageClassSummary {
  name: string
  provisioner: string
  reclaimPolicy: string
  volumeBindingMode: string
  allowVolumeExpansion: boolean
  /** Marked default (annotation); `StorageOverview.defaultClass` is the one that applies. */
  isDefault: boolean
  parameters: Record<string, string>
  mountOptions: string[]
  pvcs: number
  pvs: number
  requestedBytes: number
  age: string
}

export interface StorageOverview {
  classes: StorageClassSummary[]
  /** The class claims without one get: the newest default; '' when none. */
  defaultClass: string
  pvcs: { total: number; bound: number; pending: number; lost: number }
  pvs: { total: number; available: number; bound: number; released: number; failed: number; pending: number }
  capacity: { requestedBytes: number; provisionedBytes: number; availableBytes: number }
  csiDrivers: number
  snapshotsAvailable: boolean
}

/** Why a claim cannot be expanded; '' when it can. */
export type ExpandBlocker = '' | 'not_bound' | 'no_class' | 'class_missing' | 'not_allowed'

export interface DataSourceRef {
  kind: 'PersistentVolumeClaim' | 'VolumeSnapshot'
  name: string
}

export interface PvcSummary {
  namespace: string
  name: string
  phase: string
  storageClass: string
  requested: string
  requestedBytes: number
  /** '' until the claim is bound. */
  capacity: string
  capacityBytes: number
  volumeName: string
  accessModes: string[]
  volumeMode: string
  expandBlocker: ExpandBlocker
  resizing: boolean
  dataSource?: DataSourceRef
  labels: Record<string, string>
  age: string
}

export type PvSourceKind = 'hostPath' | 'nfs' | 'local' | 'csi'

export interface PvSummary {
  name: string
  phase: string
  capacity: string
  capacityBytes: number
  accessModes: string[]
  reclaimPolicy: string
  storageClass: string
  volumeMode: string
  source: PvSourceKind | 'other'
  /** The path, server:path or CSI driver. */
  sourceDetail: string
  claim?: { namespace: string; name: string }
  /** Hostnames a required node affinity pins the volume to. */
  nodes: string[]
  reason?: string
  age: string
}

export interface CsiDriverSummary {
  name: string
  attachRequired: boolean
  podInfoOnMount: boolean
  storageCapacity: boolean
  fsGroupPolicy: string
  lifecycleModes: string[]
  age: string
}

export interface VolumeSnapshot {
  namespace: string
  name: string
  sourcePVC: string
  sourceContent: string
  snapshotClass: string
  readyToUse: boolean
  restoreSize: string
  restoreSizeBytes: number
  error: string
  age: string
}

// --- create requests (form-shaped; the server builds the objects) ---------

export interface PvcRequest {
  namespace: string
  name: string
  /** Omitted: the cluster default applies; '': no class (static binding). */
  storageClassName?: string
  accessModes: string[]
  size: string
  volumeMode?: string
  /** Binds to this Available PersistentVolume. */
  volumeName?: string
  dataSource?: DataSourceRef
  labels?: Record<string, string>
}

export interface PvRequest {
  name: string
  capacity: string
  accessModes: string[]
  reclaimPolicy?: string
  storageClassName?: string
  volumeMode?: string
  mountOptions?: string[]
  labels?: Record<string, string>
  hostPath?: { path: string; type?: string }
  nfs?: { server: string; path: string; readOnly?: boolean }
  local?: { path: string; fsType?: string }
  csi?: {
    driver: string
    volumeHandle: string
    fsType?: string
    readOnly?: boolean
    volumeAttributes?: Record<string, string>
  }
  /** Required node affinity on kubernetes.io/hostname. */
  nodes?: string[]
}

export interface StorageClassRequest {
  name: string
  provisioner: string
  parameters?: Record<string, string>
  reclaimPolicy?: string
  volumeBindingMode?: string
  allowVolumeExpansion?: boolean
  isDefault?: boolean
  mountOptions?: string[]
  labels?: Record<string, string>
}

export interface CreateResult {
  apiVersion: string
  kind: string
  namespace?: string
  name: string
  dryRun: boolean
  warnings?: string[]
}

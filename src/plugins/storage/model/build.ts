import { dump } from 'js-yaml'
import type { PvcRequest, PvRequest, StorageClassRequest } from '../data/types'

/**
 * The objects the server builds from create requests
 * (backend/internal/plugins/storage: pvcSpec.object, pvSpec.object,
 * storageClassSpec.object), for the wizards' YAML preview and the mock.
 * Keep the two in step: the preview must show what Create stores.
 */

export type Manifest = Record<string, unknown>

export const SNAPSHOT_GROUP = 'snapshot.storage.k8s.io'
export const DEFAULT_CLASS_ANNOTATION = 'storageclass.kubernetes.io/is-default-class'
export const HOSTNAME_LABEL = 'kubernetes.io/hostname'

const ALIASES: Record<string, string> = {
  RWO: 'ReadWriteOnce',
  ROX: 'ReadOnlyMany',
  RWX: 'ReadWriteMany',
  RWOP: 'ReadWriteOncePod',
}

const accessModes = (modes: readonly string[]) => modes.map((m) => ALIASES[m] ?? m)

/** Drops undefined values and empty maps/lists, as Go's omitempty does. */
function compact(value: Record<string, unknown>): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  for (const [key, v] of Object.entries(value)) {
    if (v === undefined || v === null) continue
    if (Array.isArray(v) && v.length === 0) continue
    if (typeof v === 'object' && !Array.isArray(v) && Object.keys(v).length === 0) continue
    out[key] = v
  }
  return out
}

function metadata(name: string, namespace?: string, labels?: Record<string, string>, annotations?: Record<string, string>) {
  return compact({ name, namespace, labels, annotations })
}

export function pvcObject(req: PvcRequest): Manifest {
  const dataSource = req.dataSource
    ? compact({
        apiGroup: req.dataSource.kind === 'VolumeSnapshot' ? SNAPSHOT_GROUP : undefined,
        kind: req.dataSource.kind,
        name: req.dataSource.name,
      })
    : undefined
  return {
    apiVersion: 'v1',
    kind: 'PersistentVolumeClaim',
    metadata: metadata(req.name, req.namespace, req.labels),
    spec: compact({
      accessModes: accessModes(req.accessModes),
      resources: { requests: { storage: req.size } },
      volumeName: req.volumeName || undefined,
      // '' is meaningful (no class), so it is kept.
      storageClassName: req.storageClassName,
      volumeMode: req.volumeMode || undefined,
      dataSource,
    }),
  }
}

export function pvObject(req: PvRequest): Manifest {
  const source = req.hostPath
    ? { hostPath: compact({ path: req.hostPath.path, type: req.hostPath.type || undefined }) }
    : req.nfs
      ? { nfs: compact({ server: req.nfs.server, path: req.nfs.path, readOnly: req.nfs.readOnly || undefined }) }
      : req.local
        ? { local: compact({ path: req.local.path, fsType: req.local.fsType || undefined }) }
        : req.csi
          ? {
              csi: compact({
                driver: req.csi.driver,
                volumeHandle: req.csi.volumeHandle,
                readOnly: req.csi.readOnly || undefined,
                fsType: req.csi.fsType || undefined,
                volumeAttributes: req.csi.volumeAttributes,
              }),
            }
          : {}
  const nodes = req.nodes ?? []
  const nodeAffinity = nodes.length
    ? {
        required: {
          nodeSelectorTerms: [{ matchExpressions: [{ key: HOSTNAME_LABEL, operator: 'In', values: nodes }] }],
        },
      }
    : undefined
  return {
    apiVersion: 'v1',
    kind: 'PersistentVolume',
    metadata: metadata(req.name, undefined, req.labels),
    spec: compact({
      capacity: { storage: req.capacity },
      ...source,
      accessModes: accessModes(req.accessModes),
      persistentVolumeReclaimPolicy: req.reclaimPolicy || 'Retain',
      storageClassName: req.storageClassName || undefined,
      mountOptions: req.mountOptions,
      volumeMode: req.volumeMode || undefined,
      nodeAffinity,
    }),
  }
}

export function storageClassObject(req: StorageClassRequest): Manifest {
  return compact({
    apiVersion: 'storage.k8s.io/v1',
    kind: 'StorageClass',
    metadata: metadata(req.name, undefined, req.labels, req.isDefault ? { [DEFAULT_CLASS_ANNOTATION]: 'true' } : undefined),
    provisioner: req.provisioner,
    parameters: req.parameters,
    reclaimPolicy: req.reclaimPolicy || 'Delete',
    mountOptions: req.mountOptions,
    allowVolumeExpansion: req.allowVolumeExpansion || undefined,
    volumeBindingMode: req.volumeBindingMode || 'Immediate',
  })
}

/** YAML of a built object for the preview. */
export function toYaml(object: Manifest): string {
  return dump(object, { noRefs: true, lineWidth: 120 })
}

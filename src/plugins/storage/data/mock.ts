import { ApiError } from '@/data/apiClient'
import { delay, delayReject } from '@/data/mock/delay'
import { namespaces, nodes } from '@/data/mock/mockCluster'
import { genericMock } from '@/data/mock/mockGeneric'
import { mulberry32 } from '@/data/mock/rng'
import type { KubeObject } from '@/data/types'
import { i18n } from '@/i18n'
import { DEFAULT_CLASS_ANNOTATION, pvcObject, pvObject, storageClassObject, type Manifest } from '../model/build'
import { quantityBytes } from '../model/quantity'
import { firstProblem, pvcProblems, pvProblems, storageClassProblems, type Problems } from '../model/rules'
import { NS } from '../i18n'
import type { StorageProvider } from './provider'
import type {
  CreateResult,
  CsiDriverSummary,
  ExpandBlocker,
  PvcSummary,
  PvSummary,
  StorageClassSummary,
  StorageOverview,
  VolumeSnapshot,
} from './types'

/**
 * Mock storage (design §7.6) over the mock cluster's PVC, PV and
 * StorageClass objects (genericMock), so what the wizards create shows up
 * in the generic resource browser and its detail pages too. CSI drivers
 * and VolumeSnapshots, which the generic browser does not list, live here.
 * Requests follow the server's rules and messages; nothing reads the clock.
 */

const CSI_DRIVERS: CsiDriverSummary[] = [
  {
    name: 'hostpath.csi.k8s.io',
    attachRequired: false,
    podInfoOnMount: true,
    storageCapacity: true,
    fsGroupPolicy: 'File',
    lifecycleModes: ['Persistent', 'Ephemeral'],
    age: '30d',
  },
  {
    name: 'nfs.csi.k8s.io',
    attachRequired: false,
    podInfoOnMount: false,
    storageCapacity: false,
    fsGroupPolicy: 'File',
    lifecycleModes: ['Persistent'],
    age: '20d',
  },
]

const SNAPSHOTS: VolumeSnapshot[] = [
  {
    namespace: 'shop',
    name: 'shop-uploads-nightly',
    sourcePVC: 'shop-uploads',
    sourceContent: '',
    snapshotClass: 'csi-hostpath-snapclass',
    readyToUse: true,
    restoreSize: '5Gi',
    restoreSizeBytes: 5 * 2 ** 30,
    error: '',
    age: '9h',
  },
  {
    namespace: 'shop',
    name: 'shop-uploads-pre-migration',
    sourcePVC: 'shop-uploads',
    sourceContent: '',
    snapshotClass: 'csi-hostpath-snapclass',
    readyToUse: false,
    restoreSize: '',
    restoreSizeBytes: 0,
    error: '',
    age: '2m',
  },
]

// --- object access --------------------------------------------------------------

type Obj = KubeObject & Record<string, unknown>

const list = (key: string) => (genericMock[key] ??= []) as Obj[]
const pvcs = () => list('persistentvolumeclaims')
const pvs = () => list('persistentvolumes')
const classes = () => list('storageclasses')

function record(value: unknown): Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value) ? (value as Record<string, unknown>) : {}
}
const spec = (o: Obj) => record(o.spec)
const status = (o: Obj) => record(o.status)
const str = (value: unknown) => (typeof value === 'string' ? value : '')
const strings = (value: unknown) => (Array.isArray(value) ? value.filter((v): v is string => typeof v === 'string') : [])
const nameOf = (o: Obj) => o.metadata?.name ?? ''
const namespaceOf = (o: Obj) => o.metadata?.namespace ?? ''
const bytes = (q: string) => quantityBytes(q) ?? 0

const requestOf = (o: Obj) => str(record(record(spec(o).resources).requests).storage) || str(record(status(o).capacity).storage)
const pvCapacity = (o: Obj) => str(record(spec(o).capacity).storage)
const findPvc = (ns: string, name: string) => pvcs().find((o) => namespaceOf(o) === ns && nameOf(o) === name)
const findPv = (name: string) => pvs().find((o) => nameOf(o) === name)
const findClass = (name: string) => classes().find((o) => nameOf(o) === name)
const isDefault = (o: Obj) => o.metadata?.annotations?.[DEFAULT_CLASS_ANNOTATION] === 'true'

// --- responses -------------------------------------------------------------------

const enT = () => i18n.getFixedT('en', NS)
const badRequest = (field: string, message: string) => new ApiError(400, 'bad_request', `${field}: ${message}`)

/** The first problem as the server words it (in English, like every API message). */
function refuse(problems: Problems): ApiError | null {
  const first = firstProblem(problems)
  if (!first) return null
  const [field, problem] = first
  return badRequest(field, enT()(`validation.${problem.key}`, problem.values ?? {}))
}

function respond<T>(run: () => T): Promise<T> {
  try {
    return delay(run())
  } catch (err) {
    return delayReject(err instanceof Error ? err : new Error(String(err)))
  }
}

// --- summaries -------------------------------------------------------------------

function expandBlocker(o: Obj): ExpandBlocker {
  if (str(status(o).phase) !== 'Bound') return 'not_bound'
  const className = str(spec(o).storageClassName)
  if (!className) return 'no_class'
  const sc = findClass(className)
  if (!sc) return 'class_missing'
  return sc.allowVolumeExpansion === true ? '' : 'not_allowed'
}

function pvcSummary(o: Obj): PvcSummary {
  const s = spec(o)
  const requested = requestOf(o)
  const capacity = str(record(status(o).capacity).storage)
  const ds = record(s.dataSource)
  return {
    namespace: namespaceOf(o),
    name: nameOf(o),
    phase: str(status(o).phase) || 'Pending',
    storageClass: str(s.storageClassName),
    requested,
    requestedBytes: bytes(requested),
    capacity,
    capacityBytes: bytes(capacity),
    volumeName: str(s.volumeName),
    accessModes: strings(s.accessModes),
    volumeMode: str(s.volumeMode) || 'Filesystem',
    expandBlocker: expandBlocker(o),
    resizing: false,
    dataSource: ds.kind ? { kind: ds.kind as 'PersistentVolumeClaim' | 'VolumeSnapshot', name: str(ds.name) } : undefined,
    labels: o.metadata?.labels ?? {},
    age: o.age ?? '',
  }
}

function pvSource(s: Record<string, unknown>): Pick<PvSummary, 'source' | 'sourceDetail'> {
  if (s.hostPath) return { source: 'hostPath', sourceDetail: str(record(s.hostPath).path) }
  if (s.nfs) return { source: 'nfs', sourceDetail: `${str(record(s.nfs).server)}:${str(record(s.nfs).path)}` }
  if (s.local) return { source: 'local', sourceDetail: str(record(s.local).path) }
  if (s.csi) return { source: 'csi', sourceDetail: str(record(s.csi).driver) }
  return { source: 'other', sourceDetail: '' }
}

function affinityNodes(s: Record<string, unknown>): string[] {
  const terms = record(record(s.nodeAffinity).required).nodeSelectorTerms
  if (!Array.isArray(terms)) return []
  return terms.flatMap((term) => {
    const exprs = record(term).matchExpressions
    return Array.isArray(exprs) ? exprs.filter((e) => record(e).key === 'kubernetes.io/hostname').flatMap((e) => strings(record(e).values)) : []
  })
}

function pvSummary(o: Obj): PvSummary {
  const s = spec(o)
  const claim = record(s.claimRef)
  const capacity = pvCapacity(o)
  return {
    name: nameOf(o),
    phase: str(status(o).phase) || 'Pending',
    capacity,
    capacityBytes: bytes(capacity),
    accessModes: strings(s.accessModes),
    reclaimPolicy: str(s.persistentVolumeReclaimPolicy) || 'Retain',
    storageClass: str(s.storageClassName),
    volumeMode: str(s.volumeMode) || 'Filesystem',
    ...pvSource(s),
    claim: claim.name ? { namespace: str(claim.namespace), name: str(claim.name) } : undefined,
    nodes: affinityNodes(s),
    age: o.age ?? '',
  }
}

function classSummary(o: Obj): StorageClassSummary {
  const name = nameOf(o)
  const claims = pvcs().filter((p) => str(spec(p).storageClassName) === name)
  return {
    name,
    provisioner: str(o.provisioner),
    reclaimPolicy: str(o.reclaimPolicy) || 'Delete',
    volumeBindingMode: str(o.volumeBindingMode) || 'Immediate',
    allowVolumeExpansion: o.allowVolumeExpansion === true,
    isDefault: isDefault(o),
    parameters: (record(o.parameters) as Record<string, string>) ?? {},
    mountOptions: strings(o.mountOptions),
    pvcs: claims.length,
    pvs: pvs().filter((p) => str(spec(p).storageClassName) === name).length,
    requestedBytes: claims.reduce((sum, p) => sum + bytes(requestOf(p)), 0),
    age: o.age ?? '',
  }
}

function overview(): StorageOverview {
  const claims = pvcs().map(pvcSummary)
  const volumes = pvs().map(pvSummary)
  const count = <T extends { phase: string }>(items: T[], phase: string) => items.filter((i) => i.phase === phase).length
  // Mock objects carry no creation time: the last default in the list counts as the newest.
  const defaults = classes().filter(isDefault)
  return {
    classes: classes().map(classSummary),
    defaultClass: defaults.length ? nameOf(defaults[defaults.length - 1]) : '',
    pvcs: {
      total: claims.length,
      bound: count(claims, 'Bound'),
      pending: count(claims, 'Pending'),
      lost: count(claims, 'Lost'),
    },
    pvs: {
      total: volumes.length,
      available: count(volumes, 'Available'),
      bound: count(volumes, 'Bound'),
      released: count(volumes, 'Released'),
      failed: count(volumes, 'Failed'),
      pending: count(volumes, 'Pending'),
    },
    capacity: {
      requestedBytes: claims.reduce((sum, c) => sum + c.requestedBytes, 0),
      provisionedBytes: volumes.reduce((sum, v) => sum + v.capacityBytes, 0),
      availableBytes: volumes.filter((v) => v.phase === 'Available').reduce((sum, v) => sum + v.capacityBytes, 0),
    },
    csiDrivers: CSI_DRIVERS.length,
    snapshotsAvailable: true,
  }
}

// --- mutations -------------------------------------------------------------------

/** Seeded ids for dynamically provisioned volumes ("pvc-3f9a1c"). */
const volumeIds = mulberry32(0x5707)
const nextVolumeName = () => `pvc-${Math.floor(volumeIds() * 0xffffff).toString(16).padStart(6, '0')}`

const conflict = (message: string) => new ApiError(409, 'kubernetes', message)
const alreadyExists = (resource: string, name: string) => conflict(`${resource} "${name}" already exists`)

/** Stores a built object the way the mock cluster keeps them (synthetic age, no clock). */
function store(key: string, object: Manifest, extra: Record<string, unknown>): void {
  list(key).push({ ...(object as Obj), ...extra, age: '0s' })
}

function createResult(object: Manifest, dryRun: boolean, warnings?: string[]): CreateResult {
  const meta = record(object.metadata)
  return {
    apiVersion: str(object.apiVersion),
    kind: str(object.kind),
    namespace: str(meta.namespace) || undefined,
    name: str(meta.name),
    dryRun,
    warnings,
  }
}

export function createMockStorage(): StorageProvider {
  return {
    overview: () => respond(overview),
    listPvcs: (namespace) =>
      respond(() => pvcs().filter((o) => !namespace || namespaceOf(o) === namespace).map(pvcSummary)),
    listPvs: () => respond(() => pvs().map(pvSummary)),
    listCsiDrivers: () => respond(() => CSI_DRIVERS),
    listSnapshots: (namespace) => respond(() => SNAPSHOTS.filter((s) => !namespace || s.namespace === namespace)),

    createPvc: (req, options) =>
      respond(() => {
        const problem = refuse(pvcProblems(req))
        if (problem) throw problem
        if (!findNamespace(req.namespace)) throw new ApiError(404, 'kubernetes', `namespaces "${req.namespace}" not found`)
        const size = bytes(req.size)
        const warnings: string[] = []
        if (req.volumeName) {
          const pv = findPv(req.volumeName)
          if (!pv) throw badRequest('volumeName', `no PersistentVolume "${req.volumeName}"`)
          const phase = str(status(pv).phase) || 'Pending'
          if (phase !== 'Available') throw badRequest('volumeName', `PersistentVolume "${req.volumeName}" is ${phase}, not Available`)
          const pvClass = str(spec(pv).storageClassName)
          if (req.storageClassName !== pvClass) throw badRequest('storageClassName', `must be the volume's class (${pvClass || '""'})`)
          if (bytes(pvCapacity(pv)) < size) throw badRequest('size', `PersistentVolume "${req.volumeName}" holds only ${pvCapacity(pv)}`)
          const offered = strings(spec(pv).accessModes)
          const missing = req.accessModes.find((m) => !offered.includes(m))
          if (missing) throw badRequest('accessModes', `PersistentVolume "${req.volumeName}" does not offer ${missing}`)
        } else if (req.dataSource?.kind === 'PersistentVolumeClaim') {
          const source = findPvc(req.namespace, req.dataSource.name)
          if (!source) throw badRequest('dataSource.name', `no PersistentVolumeClaim "${req.dataSource.name}" in namespace "${req.namespace}"`)
          if (size < bytes(requestOf(source))) throw badRequest('size', `a clone must be at least as large as its source (${requestOf(source)})`)
        } else if (req.dataSource?.kind === 'VolumeSnapshot') {
          const name = req.dataSource.name
          const snap = SNAPSHOTS.find((s) => s.namespace === req.namespace && s.name === name)
          if (!snap) throw badRequest('dataSource.name', `no VolumeSnapshot "${name}" in namespace "${req.namespace}"`)
          if (snap.restoreSize && size < snap.restoreSizeBytes) {
            throw badRequest('size', `a restored claim must be at least as large as the snapshot (${snap.restoreSize})`)
          }
          if (!snap.readyToUse) warnings.push('the snapshot is not ready to use yet; the claim stays Pending until it is')
        }
        if (findPvc(req.namespace, req.name)) throw alreadyExists('persistentvolumeclaims', req.name)

        const object = pvcObject({ ...req, storageClassName: req.storageClassName ?? defaultClassName() })
        if (!options?.dryRun) provisionClaim(object, req.volumeName)
        return createResult(object, Boolean(options?.dryRun), warnings.length ? warnings : undefined)
      }),

    createPv: (req, options) =>
      respond(() => {
        const problem = refuse(pvProblems(req))
        if (problem) throw problem
        const unknown = (req.nodes ?? []).find((n) => !nodes.some((node) => node.name === n))
        if (unknown) throw badRequest('nodes', `no node "${unknown}"`)
        if (findPv(req.name)) throw alreadyExists('persistentvolumes', req.name)
        const object = pvObject(req)
        if (!options?.dryRun) store('persistentvolumes', object, { status: { phase: 'Available' } })
        return createResult(object, Boolean(options?.dryRun))
      }),

    createStorageClass: (req, options) =>
      respond(() => {
        const problem = refuse(storageClassProblems(req))
        if (problem) throw problem
        if (findClass(req.name)) throw alreadyExists('storageclasses.storage.k8s.io', req.name)
        const defaults = classes().filter(isDefault).map(nameOf)
        const warnings =
          req.isDefault && defaults.length
            ? [`already marked as default: ${defaults.join(', ')}. Claims without a class get the newest default`]
            : undefined
        const object = storageClassObject(req)
        if (!options?.dryRun) store('storageclasses', object, {})
        return createResult(object, Boolean(options?.dryRun), warnings)
      }),

    resizePvc: (namespace, name, size) =>
      respond(() => {
        const pvc = findPvc(namespace, name)
        if (!pvc) throw new ApiError(404, 'kubernetes', `persistentvolumeclaims "${name}" not found`)
        const sizeError = refuse(pvcProblems({ namespace, name, accessModes: ['ReadWriteOnce'], size }))
        if (sizeError) throw sizeError
        const current = requestOf(pvc)
        if (bytes(size) <= bytes(current)) {
          throw badRequest('size', `must be larger than the current request (${current}): volumes cannot shrink`)
        }
        const blocker = expandBlocker(pvc)
        if (blocker) throw new ApiError(409, 'conflict', enT()(`expand.blocked.${blocker}`))
        const s = spec(pvc)
        pvc.spec = { ...s, resources: { requests: { storage: size } } }
        pvc.status = { ...status(pvc), capacity: { storage: size } }
        const pv = findPv(str(s.volumeName))
        if (pv) pv.spec = { ...spec(pv), capacity: { storage: size } }
        return pvcSummary(pvc)
      }),
  }
}

const findNamespace = (name: string) => namespaces.some((n) => n.name === name)

function defaultClassName(): string | undefined {
  const defaults = classes().filter(isDefault)
  return defaults.length ? nameOf(defaults[defaults.length - 1]) : undefined
}

/**
 * Stores a new claim and binds it the way the cluster would right away: to
 * the requested volume, or to a freshly provisioned one when its class has
 * a dynamic provisioner. Claims without a class stay Pending.
 */
function provisionClaim(object: Manifest, volumeName: string | undefined): void {
  const meta = record(object.metadata)
  const s = record(object.spec)
  const className = str(s.storageClassName)
  const sc = className ? findClass(className) : undefined
  const dynamic = sc && str(sc.provisioner) !== 'kubernetes.io/no-provisioner'
  const size = str(record(record(s.resources).requests).storage)
  const claimRef = { namespace: str(meta.namespace), name: str(meta.name) }

  let bound = volumeName ? findPv(volumeName) : undefined
  if (!bound && dynamic && !volumeName) {
    const name = nextVolumeName()
    store('persistentvolumes', pvObject({
      name,
      capacity: size,
      accessModes: strings(s.accessModes),
      reclaimPolicy: str(sc.reclaimPolicy) || 'Delete',
      storageClassName: className,
      csi: { driver: str(sc.provisioner), volumeHandle: name },
    }), { status: { phase: 'Bound' } })
    bound = findPv(name)
  }
  if (bound) {
    bound.spec = { ...spec(bound), claimRef }
    bound.status = { phase: 'Bound' }
  }
  const boundSpec = bound ? { ...s, volumeName: nameOf(bound) } : s
  const capacity = bound ? { storage: pvCapacity(bound) } : undefined
  store('persistentvolumeclaims', { ...object, spec: boundSpec }, {
    status: bound ? { phase: 'Bound', capacity } : { phase: 'Pending' },
  })
}

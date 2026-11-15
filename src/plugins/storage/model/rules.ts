import type { PvcRequest, PvRequest, StorageClassRequest } from '../data/types'
import { quantityBytes } from './quantity'

/**
 * Client-side copies of the server's request rules
 * (backend/internal/plugins/storage/validate.go), so the wizards flag
 * problems as they are typed and the mock refuses what the API refuses.
 * A problem is an i18n key under `validation.` in the plugin namespace.
 */
export interface Problem {
  key: string
  values?: Record<string, string | number>
}

/** Problems by request field ("name", "size", "csi.driver", …). */
export type Problems = Record<string, Problem>

const DNS_LABEL = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/
const DNS_SUBDOMAIN = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/
const QUALIFIED_NAME = /^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$/
const LABEL_VALUE = /^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$/
const IPV4 = /^(25[0-5]|2[0-4]\d|1?\d?\d)(\.(25[0-5]|2[0-4]\d|1?\d?\d)){3}$/

export const ACCESS_MODES = ['ReadWriteOnce', 'ReadOnlyMany', 'ReadWriteMany', 'ReadWriteOncePod'] as const
export const VOLUME_MODES = ['Filesystem', 'Block'] as const
export const HOST_PATH_TYPES = [
  '',
  'DirectoryOrCreate',
  'Directory',
  'FileOrCreate',
  'File',
  'Socket',
  'CharDevice',
  'BlockDevice',
] as const

/** Object names (PV, PVC, StorageClass): DNS-1123 subdomains. */
export function nameProblem(name: string): Problem | null {
  if (!name) return { key: 'required' }
  if (name.length > 253 || !DNS_SUBDOMAIN.test(name)) return { key: 'dnsSubdomain' }
  return null
}

export function namespaceProblem(name: string): Problem | null {
  if (!name) return { key: 'required' }
  if (name.length > 63 || !DNS_LABEL.test(name)) return { key: 'dnsLabel' }
  return null
}

export function sizeProblem(text: string): Problem | null {
  if (!text) return { key: 'required' }
  const bytes = quantityBytes(text)
  if (bytes === null) return { key: 'quantity' }
  if (bytes <= 0) return { key: 'positive' }
  return null
}

export function accessModesProblem(modes: readonly string[]): Problem | null {
  if (modes.length === 0) return { key: 'accessModesRequired' }
  if (modes.some((m) => !(ACCESS_MODES as readonly string[]).includes(m))) return { key: 'accessModeUnknown' }
  if (new Set(modes).size !== modes.length) return { key: 'accessModeTwice' }
  if (modes.length > 1 && modes.includes('ReadWriteOncePod')) return { key: 'rwopAlone' }
  return null
}

/** Label keys: an optional DNS subdomain prefix and "/", then a name of at most 63 characters. */
export function labelKeyProblem(key: string): Problem | null {
  const parts = key.split('/')
  if (parts.length > 2) return { key: 'labelKey' }
  const [prefix, name] = parts.length === 2 ? parts : [null, parts[0]]
  if (prefix !== null && (prefix.length > 253 || !DNS_SUBDOMAIN.test(prefix))) return { key: 'labelKey' }
  if (name.length > 63 || !QUALIFIED_NAME.test(name)) return { key: 'labelKey' }
  return null
}

export function labelsProblem(labels: Record<string, string> | undefined): Problem | null {
  for (const [key, value] of Object.entries(labels ?? {})) {
    const problem = labelKeyProblem(key)
    if (problem) return { ...problem, values: { key } }
    if (value.length > 63 || !LABEL_VALUE.test(value)) return { key: 'labelValue', values: { key } }
  }
  return null
}

/** Provisioners and CSI drivers: qualified names, compared in lower case. */
export function driverProblem(name: string): Problem | null {
  if (!name) return { key: 'required' }
  return labelKeyProblem(name.toLowerCase()) ? { key: 'driver' } : null
}

export function pathProblem(path: string): Problem | null {
  if (!path) return { key: 'required' }
  const clean = path === '/' || (path.startsWith('/') && !path.endsWith('/') && !/\/\.{0,2}(\/|$)|\/\//.test(path))
  return clean ? null : { key: 'absolutePath' }
}

function serverProblem(server: string): Problem | null {
  if (!server) return { key: 'required' }
  if (IPV4.test(server) || server.includes(':')) return null
  return DNS_SUBDOMAIN.test(server.toLowerCase()) ? null : { key: 'nfsServer' }
}

function fsTypeProblem(fs: string | undefined): Problem | null {
  return fs && !DNS_LABEL.test(fs) ? { key: 'fsType' } : null
}

function mountOptionsProblem(opts: readonly string[] | undefined): Problem | null {
  return (opts ?? []).some((o) => !o || /\s/.test(o)) ? { key: 'mountOption' } : null
}

/** Collects the non-null problems by field. */
function collect(entries: [string, Problem | null][]): Problems {
  const out: Problems = {}
  for (const [field, problem] of entries) if (problem && !out[field]) out[field] = problem
  return out
}

export function pvcProblems(req: PvcRequest): Problems {
  return collect([
    ['namespace', namespaceProblem(req.namespace)],
    ['name', nameProblem(req.name)],
    ['storageClassName', req.storageClassName ? nameProblem(req.storageClassName) : null],
    ['accessModes', accessModesProblem(req.accessModes)],
    ['size', sizeProblem(req.size)],
    ['volumeName', req.volumeName ? nameProblem(req.volumeName) : null],
    ['dataSource', req.volumeName && req.dataSource ? { key: 'bindAndClone' } : null],
    ['dataSource.name', req.dataSource ? nameProblem(req.dataSource.name) : null],
    ['labels', labelsProblem(req.labels)],
  ])
}

export function pvProblems(req: PvRequest): Problems {
  const sources = [req.hostPath, req.nfs, req.local, req.csi].filter(Boolean).length
  const block = req.volumeMode === 'Block'
  const nodes = req.nodes ?? []
  return collect([
    ['name', nameProblem(req.name)],
    ['capacity', sizeProblem(req.capacity)],
    ['accessModes', accessModesProblem(req.accessModes)],
    ['storageClassName', req.storageClassName ? nameProblem(req.storageClassName) : null],
    ['mountOptions', mountOptionsProblem(req.mountOptions)],
    ['labels', labelsProblem(req.labels)],
    ['source', sources === 1 ? null : { key: 'oneSource' }],
    ['hostPath.path', req.hostPath ? pathProblem(req.hostPath.path) : null],
    ['nfs.server', req.nfs ? serverProblem(req.nfs.server) : null],
    ['nfs.path', req.nfs ? pathProblem(req.nfs.path) : null],
    ['local.path', req.local ? pathProblem(req.local.path) : null],
    ['local.fsType', req.local ? fsTypeProblem(req.local.fsType) : null],
    ['csi.driver', req.csi ? driverProblem(req.csi.driver) : null],
    ['csi.volumeHandle', req.csi && !req.csi.volumeHandle.trim() ? { key: 'required' } : null],
    ['csi.fsType', req.csi ? fsTypeProblem(req.csi.fsType) : null],
    ['volumeMode', block && (req.hostPath || req.nfs) ? { key: 'noBlock' } : null],
    ['nodes', req.local && nodes.length === 0 ? { key: 'localNodes' } : req.nfs && nodes.length > 0 ? { key: 'nfsNodes' } : null],
  ])
}

export function storageClassProblems(req: StorageClassRequest): Problems {
  return collect([
    ['name', nameProblem(req.name)],
    ['provisioner', driverProblem(req.provisioner)],
    ['parameters', Object.keys(req.parameters ?? {}).some((k) => !k) ? { key: 'parameterKey' } : null],
    ['mountOptions', mountOptionsProblem(req.mountOptions)],
    ['labels', labelsProblem(req.labels)],
  ])
}

/** The first problem, for a request refused as a whole. */
export function firstProblem(problems: Problems): [string, Problem] | null {
  const entry = Object.entries(problems)[0]
  return entry ?? null
}

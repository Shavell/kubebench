/**
 * Plain-object helpers for the mock's resource editing (design §7.2): the
 * same neat, redaction and merge rules the Go backend applies, so mock
 * mode behaves like a live cluster.
 */

export type Manifest = Record<string, unknown>

export const isObject = (value: unknown): value is Manifest =>
  typeof value === 'object' && value !== null && !Array.isArray(value)

/** `manifest.metadata`, created when missing. */
export function metadataOf(manifest: Manifest): Manifest {
  if (!isObject(manifest.metadata)) manifest.metadata = {}
  return manifest.metadata as Manifest
}

/** A string map (labels, annotations, data) of the manifest, or {}. */
export function stringMap(value: unknown): Record<string, string> {
  if (!isObject(value)) return {}
  return Object.fromEntries(Object.entries(value).filter((e): e is [string, string] => typeof e[1] === 'string'))
}

export const LAST_APPLIED = 'kubectl.kubernetes.io/last-applied-configuration'

const NEAT_METADATA = ['managedFields', 'uid', 'resourceVersion', 'generation', 'creationTimestamp', 'selfLink']
const NEAT_ANNOTATIONS = [LAST_APPLIED, 'deployment.kubernetes.io/revision']

/** Like the backend's kube.Neat: a copy without what the API server and tools maintain. */
export function neat(manifest: Manifest): Manifest {
  const out = structuredClone(manifest)
  delete out.status
  if (isObject(out.metadata)) {
    const meta = out.metadata
    for (const field of NEAT_METADATA) delete meta[field]
    if (isObject(meta.annotations)) {
      for (const key of NEAT_ANNOTATIONS) delete meta.annotations[key]
      if (Object.keys(meta.annotations).length === 0) delete meta.annotations
    }
  }
  dropNullTimestamps(out)
  return out
}

function dropNullTimestamps(value: unknown): void {
  if (Array.isArray(value)) {
    value.forEach(dropNullTimestamps)
  } else if (isObject(value)) {
    for (const [key, child] of Object.entries(value)) {
      if (key === 'metadata' && isObject(child) && child.creationTimestamp === null) delete child.creationTimestamp
      dropNullTimestamps(child)
    }
  }
}

/** The placeholders redacted reads show instead of Secret values ("<12 bytes, redacted>"). */
export function isRedacted(value: string): boolean {
  return value.startsWith('<') && value.endsWith('redacted>')
}

export function deepEqual(a: unknown, b: unknown): boolean {
  if (a === b) return true
  if (Array.isArray(a) && Array.isArray(b)) return a.length === b.length && a.every((v, i) => deepEqual(v, b[i]))
  if (isObject(a) && isObject(b)) {
    const keys = Object.keys(a)
    return keys.length === Object.keys(b).length && keys.every((k) => k in b && deepEqual(a[k], b[k]))
  }
  return false
}

/**
 * The object after applying `manifest` to `base`, roughly as server-side
 * apply merges: maps merge key by key, lists and values the manifest names
 * replace the old ones, fields it does not name stay.
 */
export function mergeManifest(base: Manifest, manifest: Manifest): Manifest {
  const out = structuredClone(base)
  for (const [key, value] of Object.entries(manifest)) {
    out[key] = isObject(value) && isObject(out[key]) ? mergeManifest(out[key] as Manifest, value) : structuredClone(value)
  }
  return out
}

/** The value at a dotted path ("spec.replicas"), or undefined. */
export function valueAt(manifest: Manifest, path: string): unknown {
  let current: unknown = manifest
  for (const part of path.split('.')) {
    if (!isObject(current)) return undefined
    current = current[part]
  }
  return current
}

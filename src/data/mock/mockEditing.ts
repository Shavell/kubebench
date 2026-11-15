import { dump, load } from 'js-yaml'
import { ApiError } from '../apiClient'
import type { ClusterProvider } from '../providers'
import type { ApplyOptions, ApplyResult, KubeObject, MetadataPatch, ObjectRef } from '../types'
import { delay, delayReject } from './delay'
import { namespaces } from './mockCluster'
import { findKind, type MockKind } from './mockKinds'
import { objectKey, readObject, removeObject, revealedValue, writeObject } from './mockObjects'
import {
  deepEqual,
  isObject,
  isRedacted,
  LAST_APPLIED,
  mergeManifest,
  metadataOf,
  neat,
  stringMap,
  valueAt,
  type Manifest,
} from './manifest'

/**
 * Mock resource editing (design §7.2) with the live API's rules: the same
 * validation and error codes, namespace defaulting, dry runs that change
 * nothing, and a server-side apply conflict to try "Force apply" on — the
 * HPA's controller owns shop/shop-frontend's spec.replicas, as it does on
 * a real cluster.
 */

/** Fields other field managers own, by objectKey, then field path → manager. */
const fieldOwners = new Map<string, Record<string, string>>([
  ['apps/v1/Deployment/shop/shop-frontend', { 'spec.replicas': 'kube-controller-manager' }],
])

const badRequest = (message: string) => new ApiError(400, 'bad_request', message)

/** The kind, scope-checked namespace and name of ref; throws the API's errors. */
function resolve(ref: ObjectRef): { kind: MockKind; namespace: string } {
  const kind = findKind(ref.apiVersion, ref.kind)
  if (!kind) throw new ApiError(404, 'not_found', `no matches for kind "${ref.kind}" in version "${ref.apiVersion}"`)
  const namespace = kind.namespaced ? (ref.namespace ?? '') : ''
  if (kind.namespaced && !namespace) throw badRequest(`${ref.kind} is a namespaced kind: name its namespace`)
  return { kind, namespace }
}

function notFound(kind: MockKind, name: string): ApiError {
  return new ApiError(404, 'kubernetes', `${kind.key ?? `${kind.kind.toLowerCase()}s`} "${name}" not found`)
}

/** Runs a mock request: thrown ApiErrors become rejections after the simulated latency. */
function respond<T>(run: () => T): Promise<T> {
  try {
    return delay(run())
  } catch (err) {
    return delayReject(err instanceof Error ? err : new Error(String(err)))
  }
}

// --- manifests ----------------------------------------------------------------

/** Like the backend's kube.ParseManifests: documents, lists flattened, refusals as ApiErrors. */
function parseManifests(text: string): Manifest[] {
  const objects: Manifest[] = []
  const documents = text.split(/^---(?:[ \t].*)?$/m).filter((doc) => doc.trim() !== '')
  documents.forEach((doc, index) => {
    const refuse = (problem: string, code = 'bad_request') =>
      new ApiError(400, code, `document ${index + 1}: ${problem}`)
    let content: unknown
    try {
      content = load(doc)
    } catch (err) {
      // js-yaml appends a source excerpt on further lines; the API answers one line.
      const reason = (err instanceof Error ? err.message : String(err)).split('\n')[0]
      throw refuse(`not valid YAML or JSON: ${reason}`)
    }
    if (content === null || content === undefined) return
    if (!isObject(content)) throw refuse('not an object')
    if (Object.keys(content).length === 0) return
    const items = Array.isArray(content.items) ? content.items : [content]
    items.forEach((item, i) => {
      const where = Array.isArray(content.items) ? `items[${i}]: ` : ''
      if (!isObject(item)) throw refuse(`${where}not an object`)
      const problem = manifestProblem(item)
      if (problem) throw refuse(where + problem.text, problem.code)
      objects.push(item)
    })
  })
  return objects
}

function manifestProblem(obj: Manifest): { text: string; code?: string } | null {
  const meta = isObject(obj.metadata) ? obj.metadata : {}
  if (typeof obj.apiVersion !== 'string' || !obj.apiVersion) return { text: 'apiVersion is required' }
  if (typeof obj.kind !== 'string' || !obj.kind) return { text: 'kind is required' }
  if (typeof meta.name !== 'string' || !meta.name) {
    return { text: `${obj.kind}: metadata.name is required (server-side apply cannot generate names)` }
  }
  if (obj.apiVersion !== 'v1' || obj.kind !== 'Secret') return null
  for (const field of ['data', 'stringData']) {
    const key = Object.keys(stringMap(obj[field]))
      .sort()
      .find((k) => isRedacted(stringMap(obj[field])[k]))
    if (key) {
      return {
        code: 'redacted_secret',
        text: `Secret ${meta.name}: ${field}.${key} holds a redaction placeholder, not its value; reveal the Secret and use the real values`,
      }
    }
  }
  if (isRedacted(stringMap(meta.annotations)[LAST_APPLIED] ?? '')) {
    return {
      code: 'redacted_secret',
      text: `Secret ${meta.name}: the ${LAST_APPLIED} annotation holds a redaction placeholder; remove the annotation`,
    }
  }
  return null
}

// --- apply ----------------------------------------------------------------------

function conflictsOf(key: string, apiVersion: string, manifest: Manifest, existing: Manifest): string[] {
  return Object.entries(fieldOwners.get(key) ?? {})
    .filter(([path]) => {
      const wanted = valueAt(manifest, path)
      return wanted !== undefined && !deepEqual(wanted, valueAt(existing, path))
    })
    .map(([path, manager]) => `conflict with "${manager}" using ${apiVersion}: .${path}`)
}

/** A forced apply takes over the fields the manifest names. */
function takeOwnership(key: string, manifest: Manifest) {
  const owners = fieldOwners.get(key)
  if (!owners) return
  for (const path of Object.keys(owners)) {
    if (valueAt(manifest, path) !== undefined) delete owners[path]
  }
}

function applyObject(manifest: Manifest, options: ApplyOptions): ApplyResult {
  const apiVersion = String(manifest.apiVersion)
  const kindName = String(manifest.kind)
  const meta = metadataOf(manifest)
  const name = String(meta.name)
  let namespace = typeof meta.namespace === 'string' ? meta.namespace : ''
  const result: ApplyResult = { apiVersion, kind: kindName, name }

  const kind = findKind(apiVersion, kindName)
  if (!kind) {
    return {
      ...result,
      namespace: namespace || undefined,
      error: `no matches for kind "${kindName}" in version "${apiVersion}"`,
      code: 'not_found',
    }
  }
  if (!kind.namespaced && namespace) {
    result.warnings = [`${kindName} is cluster-scoped: metadata.namespace "${namespace}" was ignored`]
    namespace = ''
  } else if (kind.namespaced) {
    namespace ||= options.namespace || 'default'
    result.namespace = namespace
    if (!namespaces.some((ns) => ns.name === namespace)) {
      return { ...result, error: `namespaces "${namespace}" not found`, code: 'kubernetes' }
    }
  }

  const desired = structuredClone(manifest)
  delete desired.status // written by controllers, not by an apply
  const desiredMeta = metadataOf(desired)
  delete desiredMeta.managedFields
  if (namespace) desiredMeta.namespace = namespace
  else delete desiredMeta.namespace

  const key = objectKey(kind, namespace, name)
  const existing = readObject(kind, namespace, name)
  const conflicts = existing && !options.force ? conflictsOf(key, apiVersion, desired, existing) : []
  if (conflicts.length > 0) {
    const plural = conflicts.length === 1 ? 'conflict' : 'conflicts'
    return { ...result, error: `Apply failed with ${conflicts.length} ${plural}: ${conflicts.join('; ')}`, code: 'conflict' }
  }
  const next = existing ? mergeManifest(existing, desired) : desired
  const action = !existing ? 'created' : deepEqual(neat(existing), neat(next)) ? 'unchanged' : 'configured'
  if (!options.dryRun) {
    writeObject(kind, namespace, name, next)
    if (options.force) takeOwnership(key, desired)
  }
  return { ...result, action }
}

// --- metadata -------------------------------------------------------------------

function patched(current: unknown, changes: Record<string, string | null> | undefined): Record<string, string> {
  const out = stringMap(current)
  for (const [key, value] of Object.entries(changes ?? {})) {
    if (value === null) delete out[key]
    else out[key] = value
  }
  return out
}

function patchObject(ref: ObjectRef, patch: MetadataPatch): void {
  const { kind, namespace } = resolve(ref)
  const values = [...Object.values(patch.labels ?? {}), ...Object.values(patch.annotations ?? {})]
  if (values.length === 0) throw badRequest('nothing to change: name labels or annotations')
  const manifest = readObject(kind, namespace, ref.name)
  if (!manifest) throw notFound(kind, ref.name)
  if (kind.kind === 'Secret' && values.some((v) => typeof v === 'string' && isRedacted(v))) {
    throw new ApiError(400, 'redacted_secret', "a value is a redaction placeholder; it would replace the real value of the Secret's annotation")
  }
  const meta = metadataOf(manifest)
  for (const [field, changes] of [['labels', patch.labels], ['annotations', patch.annotations]] as const) {
    const next = patched(meta[field], changes)
    if (Object.keys(next).length > 0) meta[field] = next
    else delete meta[field]
  }
  writeObject(kind, namespace, ref.name, manifest)
}

// --- secrets --------------------------------------------------------------------

function toBase64(text: string): string {
  let binary = ''
  for (const byte of new TextEncoder().encode(text)) binary += String.fromCharCode(byte)
  return btoa(binary)
}

function revealObject(namespace: string, name: string): KubeObject {
  const kind = findKind('v1', 'Secret')!
  const secret = readObject(kind, namespace, name)
  if (!secret) throw notFound(kind, name)
  const data = Object.fromEntries(
    Object.keys(stringMap(secret.data)).map((key) => [key, toBase64(revealedValue(namespace, name, key))]),
  )
  return { ...secret, data } as KubeObject
}

// --- provider -------------------------------------------------------------------

export const mockEditing: Pick<
  ClusterProvider,
  'getYaml' | 'applyYaml' | 'deleteResource' | 'patchMetadata' | 'revealSecret'
> = {
  getYaml: (ref, options = {}) =>
    respond(() => {
      const { kind, namespace } = resolve(ref)
      const manifest = readObject(kind, namespace, ref.name)
      if (!manifest) throw notFound(kind, ref.name)
      return dump(options.neat ? neat(manifest) : manifest, { lineWidth: -1, noRefs: true })
    }),
  applyYaml: (manifests, options = {}) =>
    respond(() => {
      const objects = parseManifests(manifests)
      if (objects.length === 0) throw badRequest('the request contains no objects')
      return objects.map((obj) => applyObject(obj, options))
    }),
  deleteResource: (ref, options = {}) =>
    respond(() => {
      const { kind, namespace } = resolve(ref)
      if (kind.apiVersion === 'v1' && kind.kind === 'Namespace') {
        throw badRequest('Namespaces are deleted with DELETE /api/namespaces, which protects the system namespaces')
      }
      if (!removeObject(kind, namespace, ref.name, options.propagation ?? 'Background')) throw notFound(kind, ref.name)
    }),
  patchMetadata: (ref, patch) => respond(() => patchObject(ref, patch)),
  revealSecret: (namespace, name) => respond(() => revealObject(namespace, name)),
}

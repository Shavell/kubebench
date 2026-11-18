import { dump, load, loadAll } from 'js-yaml'
import { englishWarnings } from '@/core/reasons'
import { ApiError } from '../apiClient'
import { reasonError } from '../reasonError'
import type { ClusterProvider } from '../providers'
import type { ApplyOptions, ApplyResult, KubeObject, MetadataPatch, ObjectRef } from '../types'
import { credentialFields, credentialPlaceholder } from './credentials'
import { delay, delayReject } from './delay'
import { namespaces } from './mockCluster'
import { findKind, type MockKind } from './mockKinds'
import { objectKey, readObject, removeObject, revealedValue, writeObject } from './mockObjects'
import {
  applyMergePatch,
  deepEqual,
  isObject,
  isRedacted,
  LAST_APPLIED,
  mergeManifest,
  mergePatch,
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

/** The kind, scope-checked namespace and name of ref; throws the API's errors. */
function resolve(ref: ObjectRef): { kind: MockKind; namespace: string } {
  const kind = findKind(ref.apiVersion, ref.kind)
  if (!kind) throw new ApiError(404, 'not_found', `no matches for kind "${ref.kind}" in version "${ref.apiVersion}"`)
  const namespace = kind.namespaced ? (ref.namespace ?? '') : ''
  if (kind.namespaced && !namespace) throw reasonError(400, 'bad_request', 'resource.namespaced_kind', { kind: ref.kind })
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
    // The backend names the document, and the item of a list: "2, items[3]".
    const refuse = (problem: ManifestProblem, item?: number) =>
      reasonError(400, problem.code ?? 'bad_request', problem.reason, {
        ...problem.params,
        document: item === undefined ? String(index + 1) : `${index + 1}, items[${item}]`,
      })
    let content: unknown
    try {
      content = load(doc)
    } catch (err) {
      // js-yaml appends a source excerpt on further lines; the API answers one line.
      const detail = (err instanceof Error ? err.message : String(err)).split('\n')[0]
      throw refuse({ reason: 'manifest.invalid_yaml', params: { detail } })
    }
    if (content === null || content === undefined) return
    if (!isObject(content)) throw refuse({ reason: 'manifest.not_object' })
    if (Object.keys(content).length === 0) return
    const items = Array.isArray(content.items) ? content.items : [content]
    items.forEach((item, i) => {
      const at = Array.isArray(content.items) ? i : undefined
      if (!isObject(item)) throw refuse({ reason: 'manifest.not_object' }, at)
      const problem = manifestProblem(item)
      if (problem) throw refuse(problem, at)
      objects.push(item)
    })
  })
  return objects
}

/** Why the backend refuses a manifest: a reason code (§3.3) and the §3.3 error code. */
interface ManifestProblem {
  reason: string
  params?: Record<string, string>
  code?: string
}

function manifestProblem(obj: Manifest): ManifestProblem | null {
  const meta = isObject(obj.metadata) ? obj.metadata : {}
  if (typeof obj.apiVersion !== 'string' || !obj.apiVersion) return { reason: 'manifest.api_version_required' }
  if (typeof obj.kind !== 'string' || !obj.kind) return { reason: 'manifest.kind_required' }
  if (typeof meta.name !== 'string' || !meta.name) return { reason: 'manifest.name_required', params: { kind: obj.kind } }
  const credentials = credentialFields(obj.apiVersion, obj.kind)
  if (credentials) {
    const field = credentialPlaceholder(credentials, obj)
    if (field) return { code: 'redacted_secret', reason: 'manifest.credential_placeholder', params: { kind: obj.kind, name: meta.name, field } }
    if (isRedacted(stringMap(meta.annotations)[LAST_APPLIED] ?? '')) {
      return { code: 'redacted_secret', reason: 'manifest.credential_annotation_placeholder', params: { kind: obj.kind, name: meta.name, annotation: LAST_APPLIED } }
    }
    return null
  }
  if (obj.apiVersion !== 'v1' || obj.kind !== 'Secret') return null
  for (const field of ['data', 'stringData']) {
    const key = Object.keys(stringMap(obj[field]))
      .sort()
      .find((k) => isRedacted(stringMap(obj[field])[k]))
    if (key) {
      return { code: 'redacted_secret', reason: 'manifest.secret_placeholder', params: { name: meta.name, field: `${field}.${key}` } }
    }
  }
  if (isRedacted(stringMap(meta.annotations)[LAST_APPLIED] ?? '')) {
    return { code: 'redacted_secret', reason: 'manifest.secret_annotation_placeholder', params: { name: meta.name, annotation: LAST_APPLIED } }
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
    Object.assign(result, englishWarnings([{ reason: 'apply.namespace_ignored', params: { kind: kindName, namespace } }]))
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

// --- edit -----------------------------------------------------------------------

/** The backend's reason codes for the original and the edited manifest of an edit. */
const EDIT_REASONS = {
  original: {
    invalidYaml: 'edit.original_invalid_yaml',
    several: 'edit.original_several_documents',
    notObject: 'edit.original_not_object',
    empty: 'edit.original_empty',
    other: 'edit.original_other_object',
  },
  edited: {
    invalidYaml: 'edit.invalid_yaml',
    several: 'edit.several_documents',
    notObject: 'edit.not_object',
    empty: 'edit.empty',
    other: 'edit.other_object',
  },
}

/** One edited object (YAML or JSON), checked to be ref; refusals as the API's 400s. */
function parseEdited(text: string, which: keyof typeof EDIT_REASONS, ref: ObjectRef, namespace: string): Manifest {
  const reasons = EDIT_REASONS[which]
  const refuse = (reason: string, params?: Record<string, string>) => reasonError(400, 'bad_request', reason, params)
  let documents: unknown[]
  try {
    documents = loadAll(text).filter((doc) => doc !== null && doc !== undefined)
  } catch (err) {
    throw refuse(reasons.invalidYaml, { detail: (err instanceof Error ? err.message : String(err)).split('\n')[0] })
  }
  if (documents.length === 0) throw refuse(reasons.empty)
  if (documents.length > 1) throw refuse(reasons.several)
  const obj = documents[0]
  if (!isObject(obj)) throw refuse(reasons.notObject)
  const meta = metadataOf(obj)
  const ns = typeof meta.namespace === 'string' ? meta.namespace : ''
  if (obj.apiVersion !== ref.apiVersion || obj.kind !== ref.kind || meta.name !== ref.name || (namespace && ns && ns !== namespace)) {
    const name = (n: string, value: unknown) => (n ? `${n}/${String(value)}` : String(value))
    throw refuse(reasons.other, { kind: String(obj.kind), name: name(ns, meta.name), edited: name(namespace, ref.name) })
  }
  return obj
}

/** Like the backend's checkPatchValues: no placeholder may become a Secret value or a credential. */
function checkPatchValues(ref: ObjectRef, patch: Manifest): void {
  const credentials = credentialFields(ref.apiVersion, ref.kind)
  if (credentials) {
    const field = credentialPlaceholder(credentials, patch)
    if (field) throw reasonError(400, 'redacted_secret', 'edit.credential_placeholder', { kind: ref.kind, name: ref.name, field })
    if (isRedacted(stringMap(metadataOf(patch).annotations)[LAST_APPLIED] ?? '')) {
      throw reasonError(400, 'redacted_secret', 'edit.credential_annotation_placeholder', { kind: ref.kind, name: ref.name, annotation: LAST_APPLIED })
    }
    return
  }
  if (ref.apiVersion !== 'v1' || ref.kind !== 'Secret') return
  for (const field of ['data', 'stringData']) {
    const values = stringMap(patch[field])
    const key = Object.keys(values)
      .sort()
      .find((k) => isRedacted(values[k]))
    if (key) throw reasonError(400, 'redacted_secret', 'edit.secret_placeholder', { name: ref.name, field: `${field}.${key}` })
  }
  if (isRedacted(stringMap(metadataOf(patch).annotations)[LAST_APPLIED] ?? '')) {
    throw reasonError(400, 'redacted_secret', 'edit.secret_annotation_placeholder', { name: ref.name, annotation: LAST_APPLIED })
  }
}

function editObject(ref: ObjectRef, original: string, edited: string, dryRun: boolean): ApplyResult {
  const { kind, namespace } = resolve(ref)
  const from = parseEdited(original, 'original', ref, namespace)
  const to = parseEdited(edited, 'edited', ref, namespace)
  const patch = mergePatch(from, to)
  if (isObject(patch.metadata)) {
    delete patch.metadata.namespace
    if (Object.keys(patch.metadata).length === 0) delete patch.metadata
  }
  checkPatchValues(ref, patch)
  const result: ApplyResult = { apiVersion: ref.apiVersion, kind: ref.kind, namespace: namespace || undefined, name: ref.name }
  const existing = readObject(kind, namespace, ref.name)
  if (!existing) return { ...result, error: notFound(kind, ref.name).message, code: 'kubernetes' }
  if (Object.keys(patch).length === 0) return { ...result, action: 'unchanged' }
  const next = applyMergePatch(existing, patch)
  const action = deepEqual(neat(existing), neat(next)) ? 'unchanged' : 'configured'
  if (!dryRun) writeObject(kind, namespace, ref.name, next)
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
  if (values.length === 0) throw reasonError(400, 'bad_request', 'editing.nothing_to_change')
  const manifest = readObject(kind, namespace, ref.name)
  if (!manifest) throw notFound(kind, ref.name)
  const holdsCredentials = kind.kind === 'Secret' || credentialFields(kind.apiVersion, kind.kind) !== undefined
  if (holdsCredentials && values.some((v) => typeof v === 'string' && isRedacted(v))) {
    throw reasonError(400, 'redacted_secret', 'editing.annotation_placeholder')
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
  'getYaml' | 'applyYaml' | 'editYaml' | 'deleteResource' | 'patchMetadata' | 'revealSecret'
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
      if (objects.length === 0) throw reasonError(400, 'bad_request', 'editing.no_objects')
      return objects.map((obj) => applyObject(obj, options))
    }),
  editYaml: (ref, original, edited, options = {}) =>
    respond(() => [editObject(ref, original, edited, Boolean(options.dryRun))]),
  deleteResource: (ref, options = {}) =>
    respond(() => {
      const { kind, namespace } = resolve(ref)
      if (kind.apiVersion === 'v1' && kind.kind === 'Namespace') {
        throw reasonError(400, 'bad_request', 'editing.namespace_delete')
      }
      if (!removeObject(kind, namespace, ref.name, options.propagation ?? 'Background')) throw notFound(kind, ref.name)
    }),
  patchMetadata: (ref, patch) => respond(() => patchObject(ref, patch)),
  revealSecret: (namespace, name) => respond(() => revealObject(namespace, name)),
}

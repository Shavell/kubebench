import { load } from 'js-yaml'
import { tw } from '../i18n'

/** kubectl's client-side apply state: tool data, not edited as an annotation. */
export const LAST_APPLIED = 'kubectl.kubernetes.io/last-applied-configuration'

const NAME = /^[A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?$/
const DNS_SUBDOMAIN = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$/

/**
 * Why `key` is not a valid label or annotation key (a Kubernetes
 * qualified name: an optional DNS subdomain prefix and "/", then a name of
 * at most 63 characters), translated; null when it is valid.
 */
export function keyProblem(key: string): string | null {
  const parts = key.split('/')
  if (parts.length > 2) return tw('editing.metadata.keyInvalid')
  const [prefix, name] = parts.length === 2 ? parts : [null, parts[0]]
  if (prefix !== null && (prefix.length > 253 || !DNS_SUBDOMAIN.test(prefix))) return tw('editing.metadata.prefixInvalid')
  if (name.length > 63 || !NAME.test(name)) return tw('editing.metadata.nameInvalid')
  return null
}

/** Label values: empty, or at most 63 characters of the name alphabet. */
export const validLabelValue = (value: string) => value === '' || (value.length <= 63 && NAME.test(value))

export interface ObjectMetadata {
  labels: Record<string, string>
  annotations: Record<string, string>
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value)

function strings(value: unknown): Record<string, string> {
  if (!isRecord(value)) return {}
  return Object.fromEntries(Object.entries(value).map(([k, v]) => [k, typeof v === 'string' ? v : String(v)]))
}

/** The labels and annotations of an object's YAML (GET /api/yaml). */
export function metadataOf(yamlText: string): ObjectMetadata {
  const obj = load(yamlText)
  const meta = isRecord(obj) && isRecord(obj.metadata) ? obj.metadata : {}
  return { labels: strings(meta.labels), annotations: strings(meta.annotations) }
}

/** The patch turning `before` into `after`: changed and new keys, null for removed ones. */
export function changes(before: Record<string, string>, after: Record<string, string>): Record<string, string | null> {
  const patch: Record<string, string | null> = {}
  for (const key of Object.keys(before)) if (!(key in after)) patch[key] = null
  for (const [key, value] of Object.entries(after)) if (before[key] !== value) patch[key] = value
  return patch
}

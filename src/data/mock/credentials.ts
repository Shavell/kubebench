/**
 * The kinds other than Secret that hold credentials in their own fields,
 * as the Go backend's kube/credentials.go knows them: reads show these
 * fields redacted, and no write may store a redaction placeholder there.
 */
import { isObject, isRedacted } from './manifest.ts'

/** Paths of the credential fields; '*' stands for every list item. */
const CREDENTIAL_FIELDS: Record<string, string[][]> = {
  'route.openshift.io/Route': [['spec', 'tls', 'key']],
  'build.openshift.io/BuildConfig': ['github', 'generic', 'gitlab', 'bitbucket'].map((m) => ['spec', 'triggers', '*', m, 'secret']),
  'build.openshift.io/Build': ['githubWebHook', 'genericWebHook', 'gitlabWebHook', 'bitbucketWebHook'].map((m) => [
    'spec',
    'triggeredBy',
    '*',
    m,
    'secret',
  ]),
}

const groupOf = (apiVersion: string) => (apiVersion.includes('/') ? apiVersion.slice(0, apiVersion.indexOf('/')) : '')

/** The credential fields of a kind, or undefined for a kind without any. */
export function credentialFields(apiVersion: unknown, kind: unknown): string[][] | undefined {
  if (typeof apiVersion !== 'string' || typeof kind !== 'string') return undefined
  return CREDENTIAL_FIELDS[`${groupOf(apiVersion)}/${kind}`]
}

/** The first credential field of obj holding a placeholder ("spec.triggers[1].github.secret"), or null. */
export function credentialPlaceholder(fields: string[][], obj: unknown): string | null {
  for (const path of fields) {
    const found = placeholderAt(obj, path, '')
    if (found) return found
  }
  return null
}

function placeholderAt(node: unknown, path: string[], name: string): string | null {
  const [head, ...rest] = path
  if (head === '*') {
    if (!Array.isArray(node)) return null
    for (const [i, item] of node.entries()) {
      const found = placeholderAt(item, rest, `${name}[${i}]`)
      if (found) return found
    }
    return null
  }
  if (!isObject(node) || !(head in node)) return null
  const child = name ? `${name}.${head}` : head
  if (rest.length === 0) return typeof node[head] === 'string' && isRedacted(node[head]) ? child : null
  return placeholderAt(node[head], rest, child)
}

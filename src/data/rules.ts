/**
 * Validation rules of the backend that the UI and the mock mirror for
 * immediate feedback. The server stays authoritative; keep these in sync
 * with backend/internal/api (auth.go, users.go, clusters.go) and
 * backend/internal/plugin.
 */

/** Minimum password length in characters (`validatePassword`). */
export const MIN_PASSWORD_LENGTH = 8

/** 1-64 letters, digits, '.', '_', '@' or '-', starting with a letter or digit. */
export const USERNAME_PATTERN = /^[A-Za-z0-9][A-Za-z0-9._@-]{0,63}$/

/** Cluster names are 1-63 characters. */
export const MAX_CLUSTER_NAME_LENGTH = 63

/** Plugin ids (design §4.6). */
export const PLUGIN_ID_PATTERN = /^[a-z0-9][a-z0-9-]{0,39}$/

/** Password length as the server counts it: in characters (code points), not UTF-16 units. */
export function passwordLength(password: string): number {
  return [...password].length
}

/** A namespace name (DNS-1123 label, at most 63 characters). */
export function isNamespaceName(name: string): boolean {
  return name.length <= 63 && /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(name)
}

/** At most this many allowed namespaces per cluster (design §7.16, `kube.MaxProbeNamespaces`). */
export const MAX_ALLOWED_NAMESPACES = 20

/**
 * A cluster's allowed namespaces as the server stores them: blanks
 * dropped, duplicates removed. Returns the first invalid name, if any.
 */
export function normalizeAllowedNamespaces(names: readonly string[]): { names: string[]; invalid?: string } {
  const out: string[] = []
  for (const raw of names) {
    const name = raw.trim()
    if (!name || out.includes(name)) continue
    if (!isNamespaceName(name)) return { names: out, invalid: name }
    out.push(name)
  }
  return { names: out }
}

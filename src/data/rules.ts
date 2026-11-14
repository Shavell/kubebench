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

import { PLATFORM_NS } from '@/core/locales'
import { errorMessage, isApiError, type ApiErrorCode } from '@/data/apiClient'
import { i18n } from '@/i18n'
import { isKubernetesForbidden } from './forbidden'

/**
 * Friendly wording for ApiError codes (design §3.3). Each entry names the
 * i18n keys `ui.errors.<key>.title` / `.hint` in the platform namespace and
 * an icon. Unknown codes use the `generic` entry.
 */
const CODES: Record<string, { key: string; icon: string }> = {
  capability_unavailable: { key: 'capabilityUnavailable', icon: 'circle-slash' },
  plugin_unavailable: { key: 'pluginUnavailable', icon: 'circle-slash' },
  plugin_disabled: { key: 'pluginDisabled', icon: 'extensions' },
  forbidden_role: { key: 'forbiddenRole', icon: 'lock' },
  cluster_read_only: { key: 'clusterReadOnly', icon: 'lock' },
  admin_required: { key: 'adminRequired', icon: 'lock' },
  forbidden: { key: 'forbidden', icon: 'lock' },
  no_cluster: { key: 'noCluster', icon: 'server-environment' },
  not_found: { key: 'notFound', icon: 'search-stop' },
  unauthenticated: { key: 'sessionExpired', icon: 'account' },
  session_expired: { key: 'sessionExpired', icon: 'account' },
  password_change_required: { key: 'passwordChangeRequired', icon: 'key' },
  rate_limited: { key: 'rateLimited', icon: 'watch' },
  conflict: { key: 'conflict', icon: 'warning' },
  bad_request: { key: 'badRequest', icon: 'warning' },
  redacted_secret: { key: 'redactedSecret', icon: 'eye-closed' },
  kubernetes: { key: 'kubernetes', icon: 'error' },
  network: { key: 'network', icon: 'debug-disconnect' },
}

const GENERIC = { key: 'generic', icon: 'error' }

/** A Kubernetes 403 (see ./forbidden): "no permission", whatever the code. */
const K8S_FORBIDDEN = { key: 'kubernetesForbidden', icon: 'lock' }

export interface ErrorTextKeys {
  /** i18n key (platform namespace) of the one-line explanation. */
  title: string
  /** i18n key (platform namespace) of what the user can do about it. */
  hint: string
  icon: string
}

/**
 * i18n keys and icon for an ApiError code (and its reason code, which can
 * be more specific); null/unknown codes get the generic wording.
 */
export function errorTextKeys(code: ApiErrorCode | null | undefined, reason?: string | null): ErrorTextKeys {
  const entry = isKubernetesForbidden(reason) ? K8S_FORBIDDEN : (code && CODES[code]) || GENERIC
  return { title: `ui.errors.${entry.key}.title`, hint: `ui.errors.${entry.key}.hint`, icon: entry.icon }
}

/** The ApiError code of a thrown value, if it is an ApiError. */
export function errorCodeOf(err: unknown): ApiErrorCode | null {
  return isApiError(err) ? err.code : null
}

/** The reason code of a thrown value, if it is an ApiError the server explained. */
export function errorReasonOf(err: unknown): string | null {
  return (isApiError(err) && err.reason) || null
}

/**
 * A translated headline for a thrown value plus the raw message as detail:
 * known ApiError codes get their friendly title, anything else shows its
 * own message.
 */
export function describeError(err: unknown): { message: string; detail?: string } {
  const code = errorCodeOf(err)
  const raw = errorMessage(err)
  if (!code || !CODES[code]) return { message: raw }
  const message = i18n.t(errorTextKeys(code, errorReasonOf(err)).title, { ns: PLATFORM_NS })
  return raw && raw !== message ? { message, detail: raw } : { message }
}

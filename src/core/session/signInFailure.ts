import type { SignInFailure } from '@/data/types'

/** Query parameter the server returns a refused redirect sign-in's failure class in (design §3.6). */
const SIGN_IN_ERROR_PARAM = 'signInError'

/** Failure classes of external sign-ins (the server's identity.Class* values). */
const SIGN_IN_FAILURES: readonly SignInFailure[] = [
  'invalid_credentials',
  'account_disabled',
  'domain_not_allowed',
  'no_matching_group',
  'username_conflict',
  'invalid_username',
  'invalid_request',
  'provider_error',
  'rate_limited',
]

/**
 * Takes the failure class of a refused redirect sign-in (OIDC) from the
 * URL and removes it, so a reload does not show it again. Null when the
 * URL carries none.
 */
export function takeSignInFailure(): SignInFailure | 'unknown' | null {
  const url = new URL(window.location.href)
  const failure = url.searchParams.get(SIGN_IN_ERROR_PARAM)
  if (failure === null) return null
  url.searchParams.delete(SIGN_IN_ERROR_PARAM)
  window.history.replaceState(window.history.state, '', url.pathname + url.search + url.hash)
  return SIGN_IN_FAILURES.find((f) => f === failure) ?? 'unknown'
}

import type { SessionProvider } from '@/data/providers'
import { notify } from '@/shared/notify'

/**
 * Ends the session and reloads into the sign-in screen, or, after an OIDC
 * sign-in, goes on to end the identity provider's session, which returns
 * the browser to KubeBench.
 */
export async function signOut(session: SessionProvider): Promise<void> {
  let result: { logoutUrl?: string }
  try {
    result = await session.logout()
  } catch (err) {
    notify.error(err)
    return
  }
  if (result.logoutUrl) window.location.assign(result.logoutUrl)
  else window.location.reload()
}

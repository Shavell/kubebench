import type { SessionProvider } from '@/data/providers'
import { notify } from '@/shared/notify'

/** Ends the session and reloads into the sign-in screen. */
export async function signOut(session: SessionProvider): Promise<void> {
  try {
    await session.logout()
  } catch (err) {
    notify.error(err)
    return
  }
  window.location.reload()
}

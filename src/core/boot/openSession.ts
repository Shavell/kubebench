import { isApiError } from '@/data/apiClient'
import { createApiProviders } from '@/data/api'
import { fetchServerInfo } from '@/data/api/info'
import { dataModeFromInfo, setDataMode } from '@/data/mode'
import type { DataProviders } from '@/data/providers'
import type { MockPolicy } from './mockPolicy'

/** What boot found out about the session (design §6.3). */
export type BootSession =
  /** A usable session; `dataMode` is set. */
  | { kind: 'ready'; providers: DataProviders }
  /** The backend wants a sign-in (401). */
  | { kind: 'signIn' }
  /** Signed in, but every cluster request is refused until the password is changed. */
  | { kind: 'changePassword'; providers: DataProviders }
  /** The backend is unreachable or failing, and mock data is not allowed. */
  | { kind: 'failed'; error: unknown }

async function openMockSession(): Promise<BootSession> {
  // The demo dataset is a chunk of its own: live sessions never download it.
  const { createMockProviders } = await import('@/data/mock')
  const providers = createMockProviders()
  setDataMode(dataModeFromInfo('mock', await providers.session.info()))
  return { kind: 'ready', providers }
}

/**
 * Probes GET /api/info and picks the data source. Live providers are
 * gated by the capabilities the server reports, so a missing capability
 * fails with its reason instead of ever falling back to mock data.
 */
export async function openSession(policy: MockPolicy): Promise<BootSession> {
  if (policy.forced) return openMockSession()
  try {
    const info = await fetchServerInfo()
    setDataMode(dataModeFromInfo('live', info))
    const providers = createApiProviders(info.capabilities)
    return info.mustChangePassword ? { kind: 'changePassword', providers } : { kind: 'ready', providers }
  } catch (error) {
    if (isApiError(error) && error.status === 401) return { kind: 'signIn' }
    if (policy.fallback) return openMockSession()
    return { kind: 'failed', error }
  }
}

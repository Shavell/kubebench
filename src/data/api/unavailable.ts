import { PLATFORM_NS } from '@/core/locales'
import { i18n } from '@/i18n'
import { ApiError } from '../apiClient'

/** Capabilities reported by /api/info that gate whole providers. */
export type Capability = 'crds' | 'helm' | 'exec'

/**
 * The error a live provider rejects with when the active cluster lacks a
 * capability. Live mode never substitutes mock data (design §0); pages
 * show this reason instead.
 */
export function capabilityUnavailable(capability: Capability): ApiError {
  return new ApiError(
    503,
    'capability_unavailable',
    i18n.t(`errors.capability.${capability}`, { ns: PLATFORM_NS }),
  )
}

/** A rejected promise carrying `capabilityUnavailable(capability)`. */
export function rejectUnavailable<T>(capability: Capability): Promise<T> {
  return Promise.reject(capabilityUnavailable(capability))
}

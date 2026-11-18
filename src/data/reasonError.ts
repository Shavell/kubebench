import { englishReason } from '@/core/reasons'
import type { ReasonParams } from './types'
import { ApiError, type ApiErrorCode } from './apiClient'

/**
 * An ApiError explained the way the server explains its own errors (design
 * §3.3): the reason code and parameters, with the server's English text as
 * the message. Mock providers use it so that demo mode shows the same
 * translated reasons as live mode.
 */
export function reasonError(status: number, code: ApiErrorCode, reason: string, params?: ReasonParams): ApiError {
  return new ApiError(status, code, englishReason(reason, params), { reason, params })
}

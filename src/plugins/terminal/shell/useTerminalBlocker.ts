import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { usePermission } from '@/core/permissions'
import type { ApiErrorCode } from '@/data/apiClient'
import { useDataMode } from '@/data/mode'

export interface TerminalBlocker {
  /** Selects the explanation of <Unavailable>. */
  code: ApiErrorCode
  reason: string
}

/**
 * Why no terminal can be opened on the active cluster, or null: shells
 * need the exec permission and, in live mode, the cluster's exec
 * capability (whether KubeBench's credentials may exec at all).
 */
export function useTerminalBlocker(): TerminalBlocker | null {
  const { t } = useTranslation(PLATFORM_NS)
  const exec = usePermission('exec')
  const mode = useDataMode()
  if (!exec.allowed) {
    return { code: exec.code, reason: exec.reason }
  }
  if (mode.mode === 'live' && mode.capabilities && !mode.capabilities.exec) {
    return { code: 'capability_unavailable', reason: t('errors.capability.exec') }
  }
  return null
}

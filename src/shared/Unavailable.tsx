import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { errorMessage, type ApiErrorCode } from '@/data/apiClient'
import { errorCodeOf, errorTextKeys } from './apiErrorText'
import { Button } from './Button'
import { Icon } from './Icon'
import './Unavailable.css'

export interface UnavailableProps {
  /**
   * ApiError code that selects the explanation, e.g. `QueryState.errorCode`
   * (`capability_unavailable`, `plugin_unavailable`, `forbidden_role`, …).
   */
  code?: ApiErrorCode | null
  /** A thrown value; supplies the code and reason when those props are not given. */
  error?: unknown
  /** The specific reason, usually the server's message (e.g. "Argo CD CRDs are not installed"). */
  reason?: string | null
  /** Overrides the title derived from the code. */
  title?: string
  /** Overrides the hint derived from the code. */
  hint?: string
  /** Overrides the icon derived from the code. */
  icon?: string
  /** Shows a Retry button. */
  onRetry?: () => void
  /** Smaller, top-aligned variant for panels and cards instead of full pages. */
  compact?: boolean
}

/**
 * Empty-state variant explaining why something cannot be shown: a missing
 * capability or plugin, a role or read-only restriction, an unreachable
 * server. Live mode shows this instead of ever substituting mock data.
 */
export function Unavailable({
  code,
  error,
  reason,
  title,
  hint,
  icon,
  onRetry,
  compact = false,
}: UnavailableProps) {
  const { t } = useTranslation(PLATFORM_NS)
  const keys = errorTextKeys(code ?? errorCodeOf(error))
  const detail = reason ?? (error === undefined ? null : errorMessage(error))

  return (
    <div className={clsx('kb-unavailable', compact && 'kb-unavailable--compact')} role="status">
      <Icon name={icon ?? keys.icon} size={compact ? 24 : 40} className="kb-unavailable__icon" />
      <div className="kb-unavailable__title">{title ?? t(keys.title)}</div>
      <div className="kb-unavailable__hint">{hint ?? t(keys.hint)}</div>
      {detail && <div className="kb-unavailable__reason">{detail}</div>}
      {onRetry && (
        <Button variant="secondary" icon="refresh" onClick={onRetry}>
          {t('ui.unavailable.retry')}
        </Button>
      )}
    </div>
  )
}

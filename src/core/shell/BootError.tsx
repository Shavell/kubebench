import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { errorMessage, isApiError } from '@/data/apiClient'
import { Button } from '@/shared/Button'
import { Icon } from '@/shared/Icon'
import './fullscreen.css'

interface BootErrorProps {
  error: unknown
  /** Tries to boot again; the boot sequence replaces this screen with its outcome. */
  onRetry: () => Promise<void>
}

/**
 * A production build whose backend is unreachable or failing shows this
 * instead of the workbench (design §6.3) — it never falls back to mock data.
 */
export function BootError({ error, onRetry }: BootErrorProps) {
  const { t } = useTranslation(PLATFORM_NS)
  const [retrying, setRetrying] = useState(false)
  const unreachable = isApiError(error) && error.status === 0
  const status = isApiError(error) && error.status > 0 ? error.status : null

  const retry = () => {
    setRetrying(true)
    void onRetry()
  }

  return (
    <main className="kb-fullscreen">
      <div className="kb-fullscreen__card" role="alert">
        <div className="kb-fullscreen__brand">
          <Icon name={unreachable ? 'debug-disconnect' : 'error'} size={36} />
          <h1 className="kb-fullscreen__title">{t('boot.failedTitle')}</h1>
          <p className="kb-fullscreen__text">
            {unreachable ? t('boot.unreachableHint') : t('boot.failedHint')}
          </p>
        </div>
        <div className="kb-fullscreen__detail">
          {status ? t('boot.status', { status, message: errorMessage(error) }) : errorMessage(error)}
        </div>
        <div className="kb-fullscreen__actions">
          <Button icon="refresh" busy={retrying} onClick={retry}>
            {t('boot.retry')}
          </Button>
        </div>
      </div>
    </main>
  )
}

import clsx from 'clsx'
import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { Icon } from './Icon'
import { useToastStore, type Toast, type ToastKind } from './notify'
import './Toaster.css'

const ICONS: Record<ToastKind, string> = { success: 'pass', error: 'error', info: 'info' }

/**
 * Renders the toasts raised with `notify.*`, bottom right above the status
 * bar. Mount it once, in the shell.
 */
export function Toaster() {
  const { t } = useTranslation(PLATFORM_NS)
  const toasts = useToastStore((s) => s.toasts)

  // One live region that exists before any toast does, so screen readers
  // announce every toast added to it.
  return createPortal(
    <section className="kb-toaster" aria-label={t('ui.notify.region')} aria-live="polite">
      {toasts.map((toast) => (
        <ToastItem key={toast.id} toast={toast} />
      ))}
    </section>,
    document.body,
  )
}

function ToastItem({ toast }: { toast: Toast }) {
  const { t } = useTranslation(PLATFORM_NS)
  const dismiss = useToastStore((s) => s.dismiss)
  const [paused, setPaused] = useState(false)

  // The countdown restarts after the pointer or focus leaves the toast.
  useEffect(() => {
    if (paused || toast.timeout <= 0) return
    const timer = setTimeout(() => dismiss(toast.id), toast.timeout)
    return () => clearTimeout(timer)
  }, [paused, toast.id, toast.timeout, dismiss])

  return (
    <div
      className={clsx('kb-toast', `kb-toast--${toast.kind}`)}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
    >
      <Icon name={ICONS[toast.kind]} size={16} className="kb-toast__icon" />
      <div className="kb-toast__text">
        <div className="kb-toast__message">{toast.message}</div>
        {toast.detail && <div className="kb-toast__detail">{toast.detail}</div>}
      </div>
      {toast.action && (
        <button
          type="button"
          className="kb-toast__action"
          onClick={() => {
            toast.action?.run()
            dismiss(toast.id)
          }}
        >
          {toast.action.label}
        </button>
      )}
      <button
        type="button"
        className="kb-toast__close"
        aria-label={t('ui.notify.close')}
        title={t('ui.notify.close')}
        onClick={() => dismiss(toast.id)}
      >
        <Icon name="close" size={14} />
      </button>
    </div>
  )
}

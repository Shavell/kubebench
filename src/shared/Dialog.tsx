import clsx from 'clsx'
import {
  useEffect,
  useId,
  useRef,
  type FormEvent,
  type KeyboardEvent,
  type MouseEvent,
  type ReactNode,
  type SyntheticEvent,
} from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { initialFocusTarget, trapTabKey } from './dialogFocus'
import { Icon } from './Icon'
import './Dialog.css'

export type DialogSize = 'small' | 'medium' | 'large'

export interface DialogProps {
  /** Whether the dialog is shown. A closed dialog renders nothing, so its content state resets. */
  open: boolean
  /** Heading; also the dialog's accessible name. */
  title: ReactNode
  /** Asks the owner to close (Esc, the close button, a backdrop click). Not called while `busy`. */
  onClose: () => void
  children?: ReactNode
  /** Text under the title, announced as the dialog's description. */
  description?: ReactNode
  /** Action buttons, right-aligned; put the primary action last. */
  footer?: ReactNode
  /** Codicon shown before the title. */
  icon?: string
  /** Width preset: small 420px, medium 560px (default), large 820px. */
  size?: DialogSize
  /**
   * Turns body and footer into a form: Enter in a field or a
   * `type="submit"` button calls this (never while busy). Validation stays
   * with the caller.
   */
  onSubmit?: () => void
  /** An operation is running: the dialog cannot be dismissed. */
  busy?: boolean
  /** Close on a click on the dimmed backdrop. Off by default so typed input is not lost. */
  closeOnBackdropClick?: boolean
  /** `alertdialog` for confirmations that interrupt the user. */
  role?: 'dialog' | 'alertdialog'
  className?: string
}

/**
 * Modal dialog rendered in a portal over the workbench. Focus moves to the
 * first field (or a `data-autofocus` element) on open, Tab stays inside,
 * Esc closes, and focus returns to the opener on close.
 */
export function Dialog({ open, ...props }: DialogProps) {
  if (!open) return null
  return createPortal(<DialogPanel {...props} />, document.body)
}

// React bubbles events from a portal to its owner's tree; keep clicks and
// keys typed in a dialog away from the rows or pages that opened it.
const stopPropagation = (event: SyntheticEvent) => event.stopPropagation()

function DialogPanel({
  title,
  onClose,
  children,
  description,
  footer,
  icon,
  size = 'medium',
  onSubmit,
  busy = false,
  closeOnBackdropClick = false,
  role = 'dialog',
  className,
}: Omit<DialogProps, 'open'>) {
  const { t } = useTranslation(PLATFORM_NS)
  const panelRef = useRef<HTMLDivElement>(null)
  const pressedOnBackdrop = useRef(false)
  const titleId = useId()
  const descriptionId = useId()

  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null
    if (panelRef.current) initialFocusTarget(panelRef.current).focus()
    return () => {
      if (opener?.isConnected) opener.focus()
    }
  }, [])

  const requestClose = () => {
    if (!busy) onClose()
  }

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    event.stopPropagation()
    if (event.defaultPrevented || event.nativeEvent.isComposing) return
    if (event.key === 'Escape') {
      event.preventDefault()
      requestClose()
    } else if (event.key === 'Tab' && panelRef.current) {
      trapTabKey(event, panelRef.current)
    }
  }

  const onBackdropMouseDown = (event: MouseEvent<HTMLDivElement>) => {
    event.stopPropagation()
    pressedOnBackdrop.current = event.target === event.currentTarget
    // Keep focus inside the dialog instead of moving it to <body>.
    if (pressedOnBackdrop.current) event.preventDefault()
  }

  const onBackdropClick = (event: MouseEvent<HTMLDivElement>) => {
    event.stopPropagation()
    // Only a click that also started on the backdrop counts: a text selection
    // dragged out of a field must not close the dialog.
    if (closeOnBackdropClick && pressedOnBackdrop.current && event.target === event.currentTarget) {
      requestClose()
    }
    pressedOnBackdrop.current = false
  }

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    event.stopPropagation()
    if (!busy) onSubmit?.()
  }

  const content = (
    <>
      {description && (
        <div id={descriptionId} className="kb-dialog__description">
          {description}
        </div>
      )}
      <div className="kb-dialog__body">{children}</div>
      {footer && <div className="kb-dialog__footer">{footer}</div>}
    </>
  )

  return (
    <div
      className="kb-dialog-overlay"
      onMouseDown={onBackdropMouseDown}
      onClick={onBackdropClick}
      onMouseUp={stopPropagation}
      onDoubleClick={stopPropagation}
      onContextMenu={stopPropagation}
      onKeyDown={onKeyDown}
    >
      <div
        ref={panelRef}
        role={role}
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={description ? descriptionId : undefined}
        aria-busy={busy || undefined}
        tabIndex={-1}
        className={clsx('kb-dialog', `kb-dialog--${size}`, className)}
      >
        <div className="kb-dialog__header">
          {icon && <Icon name={icon} size={16} className="kb-dialog__icon" />}
          <h2 id={titleId} className="kb-dialog__title">
            {title}
          </h2>
          <button
            type="button"
            className="kb-dialog__close"
            aria-label={t('ui.dialog.close')}
            title={t('ui.dialog.close')}
            disabled={busy}
            onClick={requestClose}
          >
            <Icon name="close" size={16} />
          </button>
        </div>
        {onSubmit ? (
          <form className="kb-dialog__content" noValidate onSubmit={submit}>
            {content}
          </form>
        ) : (
          <div className="kb-dialog__content">{content}</div>
        )}
      </div>
    </div>
  )
}

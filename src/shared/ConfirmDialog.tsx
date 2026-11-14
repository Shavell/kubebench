import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { dataMode } from '@/data/mode'
import { describeError } from './apiErrorText'
import { Button } from './Button'
import { Dialog } from './Dialog'
import { Field, TextInput } from './form'
import { Icon } from './Icon'
import './ConfirmDialog.css'

export interface ConfirmDialogProps {
  open: boolean
  title: ReactNode
  /** What will happen; also the dialog's accessible description. */
  message: ReactNode
  /** Label of the confirm button (default "Confirm"); better name the action, e.g. "Delete". */
  confirmLabel?: string
  cancelLabel?: string
  /** Destructive action: red confirm button and a warning icon. */
  danger?: boolean
  /**
   * Text the user must type before confirming, usually the resource name.
   * Asked for automatically when the active cluster's environment is
   * production; `requirePhrase` asks for it on every cluster (irreversible
   * actions at scale, such as deleting a namespace).
   */
  confirmPhrase?: string
  requirePhrase?: boolean
  /**
   * Performs the action. While a returned promise is pending the dialog shows
   * progress and cannot be dismissed; when it resolves the dialog closes,
   * when it rejects the error is shown and the dialog stays open.
   */
  onConfirm: () => void | Promise<unknown>
  /** Closes the dialog: Cancel, Esc, or after a successful confirm. */
  onClose: () => void
  /** Options shown under the message, e.g. a "Force delete" checkbox. */
  children?: ReactNode
}

/** Asks before running an action (design §6.6). */
export function ConfirmDialog({ open, ...props }: ConfirmDialogProps) {
  // Mounted only while open, so typed text and errors reset every time.
  return open ? <ConfirmDialogContent {...props} /> : null
}

function ConfirmDialogContent({
  title,
  message,
  confirmLabel,
  cancelLabel,
  danger = false,
  confirmPhrase,
  requirePhrase = false,
  onConfirm,
  onClose,
  children,
}: Omit<ConfirmDialogProps, 'open'>) {
  const { t } = useTranslation(PLATFORM_NS)
  const [typed, setTyped] = useState('')
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<{ message: string; detail?: string } | null>(null)

  // The environment comes from the session (/api/info), never from plugins.
  const production = dataMode.cluster?.environment === 'production'
  const askPhrase = Boolean(confirmPhrase) && (requirePhrase || production)
  const canConfirm = !busy && (!askPhrase || typed === confirmPhrase)

  const confirm = async () => {
    if (!canConfirm) return
    setBusy(true)
    setFailure(null)
    try {
      await onConfirm()
    } catch (err) {
      setFailure(describeError(err))
      setBusy(false)
      return
    }
    setBusy(false)
    onClose()
  }

  return (
    <Dialog
      open
      role="alertdialog"
      size="small"
      icon={danger ? 'warning' : undefined}
      title={title}
      description={message}
      busy={busy}
      onClose={onClose}
      onSubmit={confirm}
      footer={
        <>
          {/* Without a phrase to type, the safe choice gets the initial focus. */}
          <Button variant="secondary" disabled={busy} onClick={onClose} data-autofocus={askPhrase ? undefined : ''}>
            {cancelLabel ?? t('ui.confirm.cancel')}
          </Button>
          <Button type="submit" variant={danger ? 'danger' : 'primary'} busy={busy} disabled={!canConfirm}>
            {confirmLabel ?? t('ui.confirm.confirm')}
          </Button>
        </>
      }
    >
      {production && (
        <div className="kb-confirm__production">
          <Icon name="warning" size={14} />
          <span>{t('ui.confirm.production')}</span>
        </div>
      )}
      {children}
      {askPhrase && (
        <Field label={t('ui.confirm.typePrompt')} help={<code className="kb-confirm__phrase">{confirmPhrase}</code>}>
          <TextInput
            value={typed}
            onChange={setTyped}
            monospace
            autoComplete="off"
            disabled={busy}
            data-autofocus=""
          />
        </Field>
      )}
      {failure && (
        <div className="kb-confirm__error" role="alert">
          <Icon name="error" size={14} />
          <div>
            <div>{failure.message}</div>
            {failure.detail && <div className="kb-confirm__error-detail">{failure.detail}</div>}
          </div>
        </div>
      )}
    </Dialog>
  )
}

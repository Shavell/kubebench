import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { useEditorTranslation } from './i18n'

/** Asks before unapplied edits are thrown away (cancel editing, replace the text). */
export function DiscardDialog({
  open,
  title,
  message,
  confirmLabel,
  onConfirm,
  onClose,
}: {
  open: boolean
  title: string
  message: string
  confirmLabel: string
  onConfirm: () => void
  onClose: () => void
}) {
  const { t } = useEditorTranslation()
  return (
    <Dialog
      open={open}
      role="alertdialog"
      size="small"
      icon="warning"
      title={title}
      description={message}
      onClose={onClose}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} data-autofocus="">
            {t('common.cancel')}
          </Button>
          <Button
            variant="danger"
            onClick={() => {
              onClose()
              onConfirm()
            }}
          >
            {confirmLabel}
          </Button>
        </>
      }
    />
  )
}

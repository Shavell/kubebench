import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { usePasswordChange } from '@/core/session/usePasswordChange'
import { useDataMode } from '@/data/mode'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { notify } from '@/shared/notify'
import { PasswordFields } from './PasswordFields'

/** "Change password" from the user menu; the user stays signed in here. */
export function ChangePasswordDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation(PLATFORM_NS)
  const { user } = useDataMode()
  const form = usePasswordChange(() => {
    notify.success(t('password.changed'))
    onClose()
  })

  return (
    <Dialog
      open
      size="small"
      icon="key"
      title={t('password.dialogTitle')}
      description={t('password.dialogDescription')}
      busy={form.busy}
      onClose={onClose}
      onSubmit={() => void form.submit()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={form.busy}>
            {t('password.cancel')}
          </Button>
          <Button type="submit" busy={form.busy}>
            {t('password.submit')}
          </Button>
        </>
      }
    >
      <PasswordFields form={form} username={user} />
    </Dialog>
  )
}

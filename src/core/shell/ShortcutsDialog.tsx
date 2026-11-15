import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { ShortcutList } from '@/core/shortcuts/ShortcutList'
import { Dialog } from '@/shared/Dialog'

/** The keyboard shortcuts cheat sheet (`?`). */
export function ShortcutsDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation(PLATFORM_NS)
  return (
    <Dialog
      open
      size="large"
      icon="keyboard"
      title={t('shortcuts.title')}
      description={t('shortcuts.description')}
      closeOnBackdropClick
      onClose={onClose}
    >
      <ShortcutList />
    </Dialog>
  )
}

import { useTranslation } from 'react-i18next'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ChangelogSections } from './ChangelogSections'
import { UPDATES_NS } from './namespace'
import type { WhatsNewRequest } from './updatesStore'
import { openChangelogTab } from './whatsNew'

/**
 * "What's new in vX", shown once after an upgrade. Closing it (either way)
 * records the version as seen; the full changelog opens in a tab.
 */
export function WhatsNewDialog({ request, onClose }: { request: WhatsNewRequest; onClose: () => void }) {
  const { t } = useTranslation(UPDATES_NS)
  return (
    <Dialog
      open
      size="large"
      icon="sparkle"
      title={t('whatsNew.title', { version: request.version })}
      description={
        request.from
          ? t('whatsNew.introSince', { from: request.from, version: request.version })
          : t('whatsNew.intro', { version: request.version })
      }
      onClose={onClose}
      footer={
        <>
          <Button
            variant="secondary"
            icon="history"
            onClick={() => {
              openChangelogTab()
              onClose()
            }}
          >
            {t('whatsNew.fullChangelog')}
          </Button>
          <Button onClick={onClose} data-autofocus="">
            {t('whatsNew.close')}
          </Button>
        </>
      }
    >
      <ChangelogSections releases={request.releases} />
    </Dialog>
  )
}

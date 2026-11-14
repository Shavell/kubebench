import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { appVersion, useDataMode } from '@/data/mode'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import './AboutDialog.css'

/** Help: About — version and commit of the server and of this UI build. */
export function AboutDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation(PLATFORM_NS)
  const mode = useDataMode()
  const version = appVersion()
  // The embedded UI normally matches the server; a mismatch (stale browser cache,
  // a dev server against another backend) is worth seeing.
  const uiDiffers = __APP_VERSION__ !== version

  return (
    <Dialog
      open
      size="small"
      icon="info"
      title={t('about.title')}
      onClose={onClose}
      footer={
        <Button onClick={onClose} data-autofocus="">
          {t('about.close')}
        </Button>
      }
    >
      <dl className="kb-about">
        <dt>{t('about.version')}</dt>
        <dd>{version}</dd>
        <dt>{t('about.commit')}</dt>
        <dd>{mode.commit ?? t('about.notAvailable')}</dd>
        {uiDiffers && (
          <>
            <dt>{t('about.uiVersion')}</dt>
            <dd>{__APP_VERSION__}</dd>
          </>
        )}
        <dt>{t('about.dataSource')}</dt>
        <dd>{mode.mode === 'live' ? t('about.live') : t('about.mock')}</dd>
        <dt>{t('about.cluster')}</dt>
        <dd>{mode.cluster?.name ?? t('about.noCluster')}</dd>
        {mode.server && (
          <>
            <dt>{t('about.server')}</dt>
            <dd>{mode.server}</dd>
          </>
        )}
      </dl>
    </Dialog>
  )
}

import { useTranslation } from 'react-i18next'
import type { UpdateStatus } from '@/data/types'
import { formatDateTime } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { copyText } from '@/shared/fileActions'
import { Icon } from '@/shared/Icon'
import { Markdown } from '@/shared/markdown/Markdown'
import { safeHref } from '@/shared/markdown/markdown'
import { UPDATES_NS } from './namespace'
import { UPGRADE_GUIDE_URL, UPGRADE_METHODS } from './upgradeMethods'
import './updates.css'

/** "Update available": the release, its notes and the upgrade steps per install method. */
export function UpdateAvailableDialog({ status, onClose }: { status: UpdateStatus; onClose: () => void }) {
  const { t } = useTranslation(UPDATES_NS)
  const releasePage = safeHref(status.url)
  const known = status.current !== 'unknown'

  return (
    <Dialog
      open
      size="large"
      icon="arrow-circle-up"
      title={t('available.title')}
      onClose={onClose}
      footer={
        <Button onClick={onClose} data-autofocus="">
          {t('available.close')}
        </Button>
      }
    >
      <div className="kb-update">
        <p className="kb-update__summary">
          {known
            ? t('available.summary', { latest: status.latest, current: status.current })
            : t('available.summaryUnknown', { latest: status.latest })}
        </p>
        <div className="kb-update__meta">
          {status.prerelease && <Badge tone="warning">{t('available.prerelease')}</Badge>}
          {status.publishedAt && <span>{t('available.published', { date: formatDateTime(status.publishedAt) })}</span>}
          {releasePage && (
            <a href={releasePage} target="_blank" rel="noopener noreferrer">
              <Icon name="link-external" size={12} /> {t('available.releasePage')}
            </a>
          )}
        </div>

        <h3 className="kb-update__heading">{t('available.notes')}</h3>
        <div className="kb-update__notes">
          {status.notes.trim() ? (
            <Markdown source={status.notes} />
          ) : (
            <p className="kb-update__muted">{t('available.noNotes')}</p>
          )}
        </div>

        <h3 className="kb-update__heading">{t('available.howTo')}</h3>
        <p className="kb-update__muted">{t('available.backup')}</p>
        <dl className="kb-update__methods">
          {UPGRADE_METHODS.map((method) => (
            <div key={method.id} className="kb-update__method">
              <dt>{t(`methods.${method.id}.title`)}</dt>
              <dd>
                <span className="kb-update__muted">{t(`methods.${method.id}.hint`)}</span>
                {method.commands(status.latest).map((command) => (
                  <div key={command} className="kb-update__command">
                    <code>{command}</code>
                    <button
                      type="button"
                      className="kb-update__copy"
                      title={t('available.copy')}
                      aria-label={t('available.copy')}
                      onClick={() => void copyText(command)}
                    >
                      <Icon name="copy" size={13} />
                    </button>
                  </div>
                ))}
              </dd>
            </div>
          ))}
        </dl>
        <a href={UPGRADE_GUIDE_URL} target="_blank" rel="noopener noreferrer">
          <Icon name="link-external" size={12} /> {t('available.guide')}
        </a>
      </div>
    </Dialog>
  )
}

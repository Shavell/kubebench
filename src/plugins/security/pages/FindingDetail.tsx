import { useTranslation } from 'react-i18next'
import { formatDateTime } from '@/i18n/format'
import { Button } from '@/shared/Button'
import { Icon } from '@/shared/Icon'
import type { Finding } from '../data/types'
import { NS } from '../i18n'
import { openFindings, openResource } from '../tabs'
import { SeverityBadge, WriteButton } from './parts'
import { resourceLabel, useSecurityTexts } from './texts'

/** The selected finding: evidence, remediation, frameworks and its accepted risk, with actions. */
export function FindingDetail({
  finding,
  onAccept,
  onUnaccept,
  onClose,
}: {
  finding: Finding
  onAccept: () => void
  onUnaccept: () => void
  onClose: () => void
}) {
  const { t } = useTranslation(NS)
  const texts = useSecurityTexts()
  const a = finding.accepted
  return (
    <aside className="security-detail" aria-label={t('detail.label')}>
      <div className="security-detail__head">
        <SeverityBadge severity={finding.severity} />
        <h3>{texts.title(finding.checkId, finding.title)}</h3>
        <button type="button" className="security-icon-button" title={t('common.close')} onClick={onClose}>
          <Icon name="close" size={14} />
        </button>
      </div>
      <p>{texts.message(finding)}</p>

      <h4 className="security-section-title">{t('detail.resource')}</h4>
      <p className="security-mono">{resourceLabel(finding.resource)}</p>

      {finding.evidence && finding.evidence.length > 0 && (
        <>
          <h4 className="security-section-title">{t('detail.evidence')}</h4>
          <ul className="security-evidence">
            {finding.evidence.map((e) => (
              <li key={e} className="security-mono">
                {e}
              </li>
            ))}
          </ul>
        </>
      )}

      <h4 className="security-section-title">{t('detail.remediation')}</h4>
      <p>{texts.remediation(finding.checkId, finding.remediation)}</p>

      <h4 className="security-section-title">{t('detail.frameworks')}</h4>
      <div className="security-chips">
        {finding.frameworks.map((fw) => (
          <span key={fw} className="security-chip">
            {fw}
          </span>
        ))}
      </div>

      {a && (
        <>
          <h4 className="security-section-title">{t('detail.accepted')}</h4>
          <p>
            {a.expiresAt
              ? t('detail.acceptedUntil', { by: a.by, at: formatDateTime(a.at), until: formatDateTime(a.expiresAt) })
              : t('detail.acceptedBy', { by: a.by, at: formatDateTime(a.at) })}
          </p>
          <blockquote className="security-reason">{a.reason}</blockquote>
        </>
      )}

      <div className="security-detail__actions">
        <Button variant="secondary" icon="go-to-file" onClick={() => openResource(finding.resource)}>
          {t('actions.openResource')}
        </Button>
        <Button variant="secondary" icon="filter" onClick={() => openFindings({ checkId: finding.checkId, status: 'all' })}>
          {t('actions.sameCheck')}
        </Button>
        {a ? (
          <WriteButton label={t('actions.unaccept')} icon="circle-slash" onClick={onUnaccept} />
        ) : (
          <WriteButton label={t('actions.accept')} icon="pass" onClick={onAccept} />
        )}
      </div>
    </aside>
  )
}

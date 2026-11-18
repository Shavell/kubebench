import { useTranslation } from 'react-i18next'
import { reasonText } from '@/core/reasons'
import { Icon } from '@/shared/Icon'
import type { SecurityReport } from '../data/types'
import { NS } from '../i18n'
import { useSecurityTexts } from './texts'

/** What a scan could not do: skipped checks (with the reason) and partial failures. */
export function ReportNotices({ report }: { report: SecurityReport }) {
  const { t } = useTranslation(NS)
  const texts = useSecurityTexts()
  if (report.skipped.length === 0 && report.warnings.length === 0) return null
  return (
    <section className="security-notices">
      {report.skipped.length > 0 && (
        <>
          <h2 className="security-section-title">{t('notices.skipped', { count: report.skipped.length })}</h2>
          <ul>
            {report.skipped.map((s) => (
              <li key={s.checkId}>
                <Icon name="circle-slash" size={13} /> <strong>{texts.title(s.checkId, s.checkId)}</strong>
                <span className="security-muted"> — {reasonText(s.reasonCode, s.reasonParams, s.reason)}</span>
              </li>
            ))}
          </ul>
        </>
      )}
      {report.warnings.length > 0 && (
        <>
          <h2 className="security-section-title">{t('notices.warnings')}</h2>
          <ul>
            {report.warnings.map((w, i) => (
              <li key={w}>
                <Icon name="warning" size={13} />{' '}
                <span className="security-muted">
                  {reasonText(report.warningReasons?.[i]?.reason, report.warningReasons?.[i]?.params, w)}
                </span>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  )
}

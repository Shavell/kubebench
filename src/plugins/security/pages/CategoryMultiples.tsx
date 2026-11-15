import { useTranslation } from 'react-i18next'
import { formatNumber } from '@/i18n/format'
import { SEVERITIES, type CategoryScore } from '../data/types'
import { NS } from '../i18n'
import { scoreColor } from '../model/severity'
import { openFindings } from '../tabs'
import { useSecurityTexts } from './texts'

/** One series color for every multiple: open findings by severity. */
const BAR_COLOR = 'var(--vscode-charts-orange)'

/**
 * Per-category small multiples: the category's score as a bar, and its
 * open findings by severity as a single-series column chart on a shared
 * scale, so the panels compare at a glance. A panel opens its findings.
 */
export function CategoryMultiples({ categories }: { categories: CategoryScore[] }) {
  const { t } = useTranslation(NS)
  const texts = useSecurityTexts()
  const max = Math.max(1, ...categories.flatMap((c) => SEVERITIES.map((s) => c.bySeverity[s])))

  return (
    <div className="security-multiples">
      {categories.map((c) => (
        <button
          key={c.category}
          type="button"
          className="security-multiple"
          onClick={() => openFindings({ category: c.category })}
          title={t('overview.openCategory', { category: texts.category(c.category) })}
        >
          <div className="security-multiple__head">
            <span className="security-multiple__name">{texts.category(c.category)}</span>
            <span className="security-multiple__score" style={{ color: scoreColor(c.score) }}>
              {c.score === null ? t('overview.noScore') : formatNumber(c.score)}
            </span>
          </div>
          <div className="security-scorebar" aria-hidden>
            <div className="security-scorebar__fill" style={{ width: `${c.score ?? 0}%`, background: scoreColor(c.score) }} />
          </div>
          <div
            className="security-columns"
            role="img"
            aria-label={t('overview.columnsLabel', {
              category: texts.category(c.category),
              critical: c.bySeverity.critical,
              high: c.bySeverity.high,
              medium: c.bySeverity.medium,
              low: c.bySeverity.low,
            })}
          >
            {SEVERITIES.map((s) => (
              <div key={s} className="security-columns__col" title={`${texts.severity(s)}: ${c.bySeverity[s]}`}>
                <span className="security-columns__value">{c.bySeverity[s] || ''}</span>
                <div
                  className="security-columns__bar"
                  style={{ height: `${(c.bySeverity[s] / max) * 100}%`, background: BAR_COLOR }}
                />
                <span className="security-columns__label">{t(`severityShort.${s}`)}</span>
              </div>
            ))}
          </div>
          <div className="security-multiple__foot">
            {c.checks === 0
              ? t('overview.categorySkipped')
              : t('overview.categoryDetail', { failed: c.failed, accepted: c.accepted, evaluated: c.evaluated })}
          </div>
        </button>
      ))}
    </div>
  )
}

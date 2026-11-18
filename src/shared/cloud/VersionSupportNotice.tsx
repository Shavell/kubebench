import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import type { ClusterPlatform } from '@/data/types'
import { Icon } from '@/shared/Icon'
import { displayVersion, formatEndDate, platformName } from './platformText'
import './cloud.css'

/**
 * Warns that the cluster's Kubernetes version is past (error) or near
 * (warning, under 90 days) the end of its provider's standard support
 * (design §7.12). Renders nothing while the version is supported or its
 * support window is unknown.
 */
export function VersionSupportNotice({ platform }: { platform: ClusterPlatform | null | undefined }) {
  const { t } = useTranslation(PLATFORM_NS)
  if (!platform?.detected) return null
  const { status, endDate, basis } = platform.support
  if (status !== 'end-of-life' && status !== 'ending-soon') return null
  const { phase, eusEnd } = platform.support
  const values = {
    version: displayVersion(platform),
    provider: platformName(t, platform),
    basis: basis ? t(`cloud.basis.${basis}`) : '',
    date: endDate ? formatEndDate(endDate) : '',
    eusDate: eusEnd ? formatEndDate(eusEnd) : '',
  }
  // OpenShift names its own version, and past maintenance an even minor
  // may still be in Extended Update Support (design §7.15): a warning then.
  const inEus = phase === 'eus'
  const base = status === 'end-of-life' ? (inEus ? 'inEus' : endDate ? 'endOfLife' : 'endOfLifeOld') : 'endingSoon'
  const key = platform.productVersion ? `openshift.${base}` : base
  const error = status === 'end-of-life' && !inEus
  return (
    <div
      className={clsx('kb-support-notice', error ? 'kb-support-notice--error' : 'kb-support-notice--warning')}
      role={error ? 'alert' : 'status'}
    >
      <Icon name={error ? 'error' : 'warning'} size={16} />
      <div>
        <div className="kb-support-notice__title">{t(`cloud.notice.${key}.title`, values)}</div>
        <div className="kb-support-notice__hint">{t(`cloud.notice.${key}.hint`, values)}</div>
      </div>
    </div>
  )
}

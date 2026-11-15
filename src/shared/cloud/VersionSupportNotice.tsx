import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import type { ClusterPlatform } from '@/data/types'
import { Icon } from '@/shared/Icon'
import { formatEndDate, platformName } from './platformText'
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
  const values = {
    version: platform.version,
    provider: platformName(t, platform),
    basis: basis ? t(`cloud.basis.${basis}`) : '',
    date: endDate ? formatEndDate(endDate) : '',
  }
  const key = status === 'end-of-life' ? (endDate ? 'endOfLife' : 'endOfLifeOld') : 'endingSoon'
  return (
    <div
      className={clsx('kb-support-notice', status === 'end-of-life' ? 'kb-support-notice--error' : 'kb-support-notice--warning')}
      role={status === 'end-of-life' ? 'alert' : 'status'}
    >
      <Icon name={status === 'end-of-life' ? 'error' : 'warning'} size={16} />
      <div>
        <div className="kb-support-notice__title">{t(`cloud.notice.${key}.title`, values)}</div>
        <div className="kb-support-notice__hint">{t(`cloud.notice.${key}.hint`, values)}</div>
      </div>
    </div>
  )
}

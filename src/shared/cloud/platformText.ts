import type { TFunction } from 'i18next'
import type { ClusterPlatform, VersionSupport } from '@/data/types'
import { formatDateTime } from '@/i18n/format'

/**
 * Texts about a cluster's platform (design §7.12), from the platform
 * namespace's `cloud` group. `t` must be bound to PLATFORM_NS.
 */

/**
 * The short badge label: the provider ("EKS", "k3s"), or the cloud ("AWS")
 * for a generic cluster on one; null for plain Kubernetes, which needs no
 * badge.
 */
export function platformLabel(t: TFunction, platform: ClusterPlatform): string | null {
  if (platform.provider !== 'kubernetes') return t(`cloud.providers.${platform.provider}`)
  if (platform.cloud) return t(`cloud.clouds.${platform.cloud}`)
  return null
}

/** The full provider name, e.g. "Amazon EKS", or "Kubernetes on AWS". */
export function platformName(t: TFunction, platform: ClusterPlatform): string {
  if (platform.provider === 'kubernetes' && platform.cloud) {
    return t('cloud.onCloud', { cloud: t(`cloud.clouds.${platform.cloud}`) })
  }
  return t(`cloud.names.${platform.provider}`)
}

/** An end date (YYYY-MM-DD) in the UI language, as a calendar day. */
export function formatEndDate(endDate: string): string {
  return formatDateTime(`${endDate}T00:00:00Z`, { dateStyle: 'medium', timeZone: 'UTC' })
}

/** "Supported until 28 Feb 2027 (upstream Kubernetes)" and the like. */
export function supportText(t: TFunction, support: VersionSupport): string {
  const basis = support.basis ? t(`cloud.basis.${support.basis}`) : ''
  if (support.status === 'unknown' || (!support.endDate && support.status !== 'end-of-life')) {
    return t('cloud.support.unknown')
  }
  if (!support.endDate) return t('cloud.support.endedLongAgo', { basis })
  return t(`cloud.support.${support.status}`, { date: formatEndDate(support.endDate), basis })
}

/** The multi-line tooltip of a platform badge. */
export function platformTooltip(t: TFunction, platform: ClusterPlatform): string {
  if (!platform.detected) {
    return [platformName(t, platform), t('cloud.notDetected')].join('\n')
  }
  return [
    platformName(t, platform),
    platform.version ? t('cloud.version', { version: platform.version }) : '',
    supportText(t, platform.support),
  ]
    .filter(Boolean)
    .join('\n')
}

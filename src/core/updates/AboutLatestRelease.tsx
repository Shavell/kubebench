import type { TFunction } from 'i18next'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useProviders } from '@/data/DataContext'
import type { UpdateStatus } from '@/data/types'
import { UPDATES_NS } from './namespace'
import { useUpdatesStore } from './updatesStore'

/**
 * The "Latest release" row of the About dialog, for every user (design
 * §7.11): term and description for the host's <dl>.
 */
export function AboutLatestRelease() {
  const { t } = useTranslation(UPDATES_NS)
  const { updates } = useProviders()
  const status = useUpdatesStore((s) => s.status)

  useEffect(() => {
    void useUpdatesStore.getState().load(updates)
  }, [updates])

  if (!status) return null
  return (
    <>
      <dt>{t('about.latest')}</dt>
      <dd>{describe(status, t)}</dd>
    </>
  )
}

function describe(status: UpdateStatus, t: TFunction<typeof UPDATES_NS>): string {
  if (status.mode === 'off') return t('about.off')
  if (!status.latest) return status.checkedAt ? t('about.none') : t('about.unknown')
  if (status.available) return t('about.available', { version: status.latest })
  if (status.current === 'unknown') return `v${status.latest}`
  return `v${status.latest} (${t('about.upToDate')})`
}

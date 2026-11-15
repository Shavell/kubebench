import { useTranslation } from 'react-i18next'
import { appVersion, useDataMode } from '@/data/mode'
import { EmptyState } from '@/shared/EmptyState'
import { Page } from '@/shared/Page'
import { changelog } from './changelog'
import { ChangelogSections } from './ChangelogSections'
import { UPDATES_NS } from './namespace'

/** The `changelog` tab: every release of the bundled CHANGELOG.md, newest first. */
export function ChangelogTab() {
  const { t } = useTranslation(UPDATES_NS)
  // appVersion() reads the session facts; follow them.
  useDataMode()
  return (
    <Page title={t('changelog.title')}>
      {changelog.length ? (
        <div className="kb-changelog-page">
          <ChangelogSections releases={changelog} running={appVersion()} />
        </div>
      ) : (
        <EmptyState icon="history" title={t('changelog.empty')} />
      )}
    </Page>
  )
}

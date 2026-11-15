import { useTranslation } from 'react-i18next'
import { Badge } from '@/shared/Badge'
import type { ChangelogRelease } from './changelog'
import { Markdown } from './Markdown'
import { UPDATES_NS } from './namespace'

/** Changelog sections, each a version heading over its Markdown body. */
export function ChangelogSections({ releases, running }: { releases: ChangelogRelease[]; running?: string }) {
  const { t } = useTranslation(UPDATES_NS)
  return (
    <div className="kb-changelog">
      {releases.map((release) => (
        <section key={release.version} className="kb-changelog__release">
          <h3 className="kb-changelog__version">
            <span>{t('changelog.version', { version: release.version })}</span>
            {release.date && <span className="kb-changelog__date">{release.date}</span>}
            {release.version === running && <Badge tone="info">{t('changelog.running')}</Badge>}
          </h3>
          <Markdown source={release.body} headingOffset={1} />
        </section>
      ))}
    </div>
  )
}

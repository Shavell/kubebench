import changelogSource from '../../../CHANGELOG.md?raw'
import { compareVersions, parseVersion, releaseVersion, type Version } from './semver'

/** One `## [x.y.z] - date` section of a Keep a Changelog file. */
export interface ChangelogRelease {
  version: string
  /** As written after the version: a date, or e.g. "Unreleased". */
  date: string
  /** The section's Markdown (### Added, ### Fixed, … and their lists). */
  body: string
}

const RELEASE_HEADING = /^##\s+\[?([^\]\s]+)\]?(?:\s+-\s+(.+?))?\s*$/
/** Link reference definitions (`[0.5.0]: https://…`) at the end of the file. */
const LINK_DEFINITION = /^\[[^\]]+\]:\s+\S/

/**
 * The released sections of a Keep a Changelog document, newest first as
 * written. `[Unreleased]` and anything before the first section (title,
 * preamble) are dropped.
 */
export function parseChangelog(source: string): ChangelogRelease[] {
  const releases: ChangelogRelease[] = []
  let current: { version: string; date: string; lines: string[] } | null = null
  const finish = () => {
    if (current && current.version.toLowerCase() !== 'unreleased') {
      releases.push({ version: current.version, date: current.date, body: current.lines.join('\n').trim() })
    }
  }
  for (const line of source.replace(/\r\n?/g, '\n').split('\n')) {
    const heading = RELEASE_HEADING.exec(line)
    if (heading) {
      finish()
      current = { version: heading[1].replace(/^v/, ''), date: heading[2] ?? '', lines: [] }
    } else if (current && !LINK_DEFINITION.test(line)) {
      current.lines.push(line)
    }
  }
  finish()
  return releases
}

/** KubeBench's own changelog, bundled at build time. */
export const changelog: ChangelogRelease[] = parseChangelog(changelogSource)

/**
 * What a user should see after an upgrade: the sections newer than the
 * version they last saw, up to the running one (only the running one's for
 * a user who never saw any). Null when there is nothing to show: not a
 * release build, already seen, or a downgrade.
 */
export function whatsNewFor(
  releases: ChangelogRelease[],
  appVersion: string,
  lastSeen: string,
): ChangelogRelease[] | null {
  const current = releaseVersion(appVersion)
  if (!current) return null
  const seen = lastSeen ? parseVersion(lastSeen) : null
  if (seen && compareVersions(current, seen) <= 0) return null
  const inRange = (v: Version) =>
    compareVersions(v, current) <= 0 && (seen ? compareVersions(v, seen) > 0 : compareVersions(v, current) === 0)
  return releases.filter((r) => {
    const v = parseVersion(r.version)
    return v !== null && inRange(v)
  })
}

/**
 * The SemVer subset KubeBench versions use (design §7.11): `X.Y.Z` with an
 * optional prerelease part and build metadata, an optional `v` prefix.
 */
export interface Version {
  major: number
  minor: number
  patch: number
  prerelease: string[]
}

const SEMVER =
  /^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/

export function parseVersion(text: string): Version | null {
  const m = SEMVER.exec(text.trim())
  if (!m) return null
  return { major: Number(m[1]), minor: Number(m[2]), patch: Number(m[3]), prerelease: m[4] ? m[4].split('.') : [] }
}

/** First prerelease identifier of a release candidate build; see the backend's ParseBuildVersion. */
const CANDIDATE = /^(alpha|beta|rc|pre|preview)\d*$/

/**
 * The version of a release build, or null for builds that are none: `dev`,
 * git-describe and untagged commits, dirty trees, snapshots. Only release
 * builds show What's new.
 */
export function releaseVersion(text: string): Version | null {
  const v = parseVersion(text)
  if (!v || v.prerelease.length === 0) return v
  const [first, ...rest] = v.prerelease
  if (!CANDIDATE.test(first) || rest.some((id) => id.includes('-'))) return null
  return v
}

function compareIdentifiers(a: string, b: string): number {
  const numA = /^\d+$/.test(a)
  const numB = /^\d+$/.test(b)
  if (numA && numB) return Number(a) - Number(b)
  if (numA !== numB) return numA ? -1 : 1 // numeric identifiers sort first
  return a < b ? -1 : a > b ? 1 : 0
}

/** SemVer precedence (§11): negative when a < b, 0 when equal, positive when a > b. */
export function compareVersions(a: Version, b: Version): number {
  const core = a.major - b.major || a.minor - b.minor || a.patch - b.patch
  if (core) return core
  // A release ranks above its prereleases.
  if (!a.prerelease.length || !b.prerelease.length) return b.prerelease.length - a.prerelease.length
  for (let i = 0; i < Math.min(a.prerelease.length, b.prerelease.length); i++) {
    const c = compareIdentifiers(a.prerelease[i], b.prerelease[i])
    if (c) return c
  }
  return a.prerelease.length - b.prerelease.length
}

export function formatVersion(v: Version): string {
  const core = `${v.major}.${v.minor}.${v.patch}`
  return v.prerelease.length ? `${core}-${v.prerelease.join('.')}` : core
}

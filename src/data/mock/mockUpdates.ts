import { reasonError } from '../reasonError'
import type { UpdatesProvider } from '../providers'
import type { UpdateMode, UpdateStatus } from '../types'
import { delay, delayReject } from './delay'

/** The bundle's version one patch up: the demo always has an update to show. */
function nextPatch(version: string): string {
  const m = /^(\d+)\.(\d+)\.(\d+)/.exec(version)
  return m ? `${m[1]}.${m[2]}.${Number(m[3]) + 1}` : '0.0.1'
}

const LATEST = nextPatch(__APP_VERSION__)

/**
 * Release notes as GitHub would return them (Markdown source). The raw HTML
 * line shows that the UI renders notes as text, never as markup.
 */
const NOTES = `## What's changed

### Fixed

- Log streams reconnect after the API server restarts.
- The **Helm** values editor keeps comments when it saves.
- \`kubebench healthcheck\` honours \`KUBEBENCH_LISTEN_ADDR\`.

### Security

- Dependencies updated. Details in the [security advisory](https://github.com/Shavell/kubebench/security/advisories).

<img src=x onerror="alert('notes are never HTML')">
`

const RELEASE_PAGE = `https://github.com/Shavell/kubebench/releases/tag/v${LATEST}`

/**
 * Demo update check: a fixed release one patch above the app version,
 * published 2026-11-01. The demo user has already seen the current
 * version's What's new, so demo boots show no dialog.
 */
export function createMockUpdates(): UpdatesProvider {
  let mode: UpdateMode = 'stable'
  let lastSeen = __APP_VERSION__

  const status = (): UpdateStatus => {
    const base: UpdateStatus = {
      mode,
      defaultMode: 'stable',
      current: __APP_VERSION__,
      latest: '',
      available: false,
      prerelease: false,
      publishedAt: null,
      url: 'https://github.com/Shavell/kubebench/releases',
      notes: '',
      checkedAt: null,
      error: '',
    }
    if (mode === 'off') return base
    return {
      ...base,
      latest: LATEST,
      available: true,
      publishedAt: '2026-11-01T09:30:00Z',
      url: RELEASE_PAGE,
      notes: NOTES,
      checkedAt: '2026-11-15T08:00:00Z',
    }
  }

  return {
    status: () => delay(status()),
    setMode: (next) => {
      mode = next
      return delay(status())
    },
    checkNow: () =>
      mode === 'off'
        ? delayReject(reasonError(409, 'conflict', 'updates.off'))
        : delay(status()),
    lastSeenVersion: () => delay(lastSeen),
    markSeen: (version) => {
      lastSeen = version
      return delay(undefined)
    },
  }
}

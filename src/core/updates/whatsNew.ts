import type { UpdatesProvider } from '@/data/providers'
import { appVersion } from '@/data/mode'
import { useLayoutStore } from '@/core/store/layoutStore'
import { i18n } from '@/i18n'
import { changelog, whatsNewFor } from './changelog'
import { UPDATES_NS } from './namespace'
import { useUpdatesStore } from './updatesStore'

/** Tab kind of the full changelog. */
export const CHANGELOG_TAB_KIND = 'changelog'

/** Opens (or focuses) the tab with the whole bundled changelog. */
export function openChangelogTab(): void {
  useLayoutStore.getState().openTab({
    id: CHANGELOG_TAB_KIND,
    title: i18n.t('changelog.tab', { ns: UPDATES_NS }),
    titleKey: 'changelog.tab',
    titleNs: UPDATES_NS,
    icon: 'sparkle',
    kind: CHANGELOG_TAB_KIND,
  })
}

let checked = false

/**
 * Shows "What's new in vX" once after an upgrade (design §7.11): when the
 * running release is newer than the version the user last saw. With no
 * changelog section to show, the version is recorded silently. Runs once
 * per page load.
 */
export async function showWhatsNewAfterUpgrade(provider: UpdatesProvider): Promise<void> {
  if (checked) return
  checked = true
  const version = appVersion()
  let seen: string
  try {
    seen = await provider.lastSeenVersion()
  } catch {
    return // the next page load asks again
  }
  const releases = whatsNewFor(changelog, version, seen)
  if (releases === null) return
  if (releases.length === 0) {
    await provider.markSeen(version).catch(() => undefined)
    return
  }
  useUpdatesStore.getState().showWhatsNew({ version, from: seen, releases })
}

import { useLayoutStore } from '@/core/store/layoutStore'
import { NS, translate } from './i18n'

/**
 * Editor tabs of the Helm plugin, opened from the command palette, the
 * sidebar and the releases list. Tab descriptors carry plain strings, so
 * titles are translated when the tab opens.
 */

export const RELEASES_TAB_ID = 'helm-releases'
export const REPOS_TAB_ID = 'helm-repos'
export const RELEASE_TAB_PREFIX = 'release:'

export function openReleasesTab(): void {
  useLayoutStore.getState().openTab({
    id: RELEASES_TAB_ID,
    title: translate('releases.title'),
    titleKey: 'releases.title',
    titleNs: NS,
    icon: 'package',
    kind: 'helm-releases',
  })
}

export function openReposTab(): void {
  useLayoutStore.getState().openTab({
    id: REPOS_TAB_ID,
    title: translate('repos.title'),
    titleKey: 'repos.title',
    titleNs: NS,
    icon: 'repo',
    kind: 'helm-repos',
  })
}

/**
 * Id of a release's detail tab. Release names are unique per namespace
 * only, so the namespace is part of every release's identity.
 */
export function releaseTabId(namespace: string, name: string): string {
  return `${RELEASE_TAB_PREFIX}${namespace}/${name}`
}

/** Detail tab of one release (`helm-release-detail`, props `{namespace, name}`). */
export function openReleaseTab(namespace: string, name: string): void {
  useLayoutStore.getState().openTab({
    id: releaseTabId(namespace, name),
    title: `${namespace}/${name}`,
    icon: 'package',
    kind: 'helm-release-detail',
    props: { namespace, name },
  })
}

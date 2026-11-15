import { useLayoutStore } from '@/core/store/layoutStore'
import { NS, translate } from './i18n'

/**
 * Editor tabs of the Helm plugin, opened from the command palette, the
 * sidebar and the pages. Titles are i18n keys, so open tabs follow
 * language changes.
 */

export const RELEASES_TAB_ID = 'helm-releases'
export const REPOS_TAB_ID = 'helm-repos'
export const INSTALL_TAB_ID = 'helm-install'

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

/** The install wizard (`helm-install`); one at a time, so an open wizard keeps its state. */
export function openInstallTab(): void {
  useLayoutStore.getState().openTab({
    id: INSTALL_TAB_ID,
    title: translate('install.title'),
    titleKey: 'install.title',
    titleNs: NS,
    icon: 'cloud-download',
    kind: 'helm-install',
  })
}

/**
 * Id of a release's detail tab. Release names are unique per namespace
 * only, so the namespace is part of every release's identity.
 */
export function releaseTabId(namespace: string, name: string): string {
  return `helm-release:${namespace}/${name}`
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

export function upgradeTabId(namespace: string, name: string): string {
  return `helm-upgrade:${namespace}/${name}`
}

/** Upgrade of one release (`helm-upgrade`, props `{namespace, name}`). */
export function openUpgradeTab(namespace: string, name: string): void {
  useLayoutStore.getState().openTab({
    id: upgradeTabId(namespace, name),
    title: translate('upgrade.tabTitle', { name }),
    titleKey: 'upgrade.tabTitle',
    titleNs: NS,
    titleValues: { name },
    icon: 'arrow-circle-up',
    kind: 'helm-upgrade',
    props: { namespace, name },
  })
}

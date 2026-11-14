import manifest from '../plugin.json'

/** The plugin id: also its i18n namespace and the prefix of its tab ids. */
export const PLUGIN_ID = manifest.id
export const PLUGIN_VERSION = manifest.version

/** Tab kind of the namespace list (tab kinds are global, so they are prefixed). */
export const NAMESPACES_TAB = `${PLUGIN_ID}.namespaces`

/** Opens (or focuses) the namespace list. */
export function openNamespacesTab(): void {
  const { layout, i18n } = window.KubeBench
  layout.openTab({
    id: NAMESPACES_TAB,
    title: i18n.t('tab.title', { ns: PLUGIN_ID }),
    titleKey: 'tab.title',
    titleNs: PLUGIN_ID,
    icon: 'globe',
    kind: NAMESPACES_TAB,
  })
}

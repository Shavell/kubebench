import { useLayoutStore } from '@/core/store/layoutStore'

/** Tab kind of the plugin detail page. */
export const EXTENSION_TAB_KIND = 'extension'

export const extensionTabId = (pluginId: string) => `extension:${pluginId}`

/**
 * Opens the detail page of a plugin, titled by its translated `name`. A
 * registered plugin also passes its manifest `nameKey` (a key in its own
 * namespace), so the title follows language changes.
 */
export function openExtensionTab(pluginId: string, name: string, nameKey?: string): void {
  useLayoutStore.getState().openTab({
    id: extensionTabId(pluginId),
    title: name,
    titleKey: nameKey,
    titleNs: nameKey && pluginId,
    icon: 'extensions',
    kind: EXTENSION_TAB_KIND,
    props: { pluginId },
  })
}

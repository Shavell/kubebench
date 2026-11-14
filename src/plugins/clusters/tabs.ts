import { useLayoutStore } from '@/core/store/layoutStore'
import { i18n } from '@/i18n'
import { CLUSTERS_NS } from './namespace'

/** Tab kind of the inventory page; the status bar's "Manage clusters" opens it too. */
export const INVENTORY_TAB_KIND = 'clusters-inventory'

export function openInventoryTab(): void {
  useLayoutStore.getState().openTab({
    id: INVENTORY_TAB_KIND,
    title: i18n.t('tab.title', { ns: CLUSTERS_NS }),
    titleKey: 'tab.title',
    titleNs: CLUSTERS_NS,
    icon: 'server-environment',
    kind: INVENTORY_TAB_KIND,
  })
}

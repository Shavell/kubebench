import { useLayoutStore } from '@/core/store/layoutStore'
import { NS, translate } from './i18n'

/**
 * Editor tabs of the CRD plugin, opened from the command palette, the
 * sidebar and the definitions list. Tab descriptors carry plain strings,
 * so titles are translated when the tab opens.
 */

export const DEFINITIONS_TAB_ID = 'crd-all'
export const INSTANCES_TAB_PREFIX = 'crd:'

export function openDefinitionsTab(): void {
  useLayoutStore.getState().openTab({
    id: DEFINITIONS_TAB_ID,
    title: translate('definitions.title'),
    titleKey: 'definitions.title',
    titleNs: NS,
    icon: 'list-tree',
    kind: 'crd-list',
  })
}

/** Instances of one CRD (`crd-instances` tab kind, props `{crdName}`); titled by its kind. */
export function openInstancesTab(crdName: string, kind: string): void {
  useLayoutStore.getState().openTab({
    id: `${INSTANCES_TAB_PREFIX}${crdName}`,
    title: kind,
    icon: 'symbol-class',
    kind: 'crd-instances',
    props: { crdName },
  })
}

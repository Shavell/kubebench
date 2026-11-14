import { useLayoutStore } from '@/core/store/layoutStore'
import { NS, translate } from './i18n'

/**
 * Editor tabs of the metrics plugin, opened from the command palette and
 * the sidebar. Tab descriptors carry plain strings, so titles are
 * translated when the tab opens.
 */

export const OVERVIEW_TAB_ID = 'metrics-overview'
export const NODE_TAB_PREFIX = 'metrics-node:'

export function openOverviewTab(): void {
  useLayoutStore.getState().openTab({
    id: OVERVIEW_TAB_ID,
    title: translate('overview.title'),
    titleKey: 'overview.title',
    titleNs: NS,
    icon: 'dashboard',
    kind: 'metrics-overview',
  })
}

export function openNodeMetricsTab(node: string): void {
  useLayoutStore.getState().openTab({
    id: `${NODE_TAB_PREFIX}${node}`,
    title: translate('tabs.node', { node }),
    titleKey: 'tabs.node',
    titleNs: NS,
    titleValues: { node },
    icon: 'pulse',
    kind: 'metrics-node',
    props: { node },
  })
}

import { useSideGroupStore } from '@/core/store/sideGroupStore'
import type { TabDescriptor } from './types'

/**
 * Opens `tab` in the side editor group to the right of the editor area
 * (design §7.14), or brings it to the front when a tab with its id is
 * open there. The tab's `kind` is rendered by the registered tab renderer,
 * as in the editor area; the user closes it from the group's tab strip.
 */
export function openInSideGroup(tab: TabDescriptor): void {
  useSideGroupStore.getState().open(tab)
}

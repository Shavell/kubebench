import type { TFunction } from 'i18next'
import type { TabDescriptor } from '@/core/plugin/types'

/**
 * The tab's title in the current language: tabs opened with a `titleKey`
 * are translated on every render; others keep the title they were opened with.
 */
export function tabTitle(tab: TabDescriptor, t: TFunction): string {
  if (!tab.titleKey) return tab.title
  return t(tab.titleKey, { ...tab.titleValues, ns: tab.titleNs, defaultValue: tab.title })
}

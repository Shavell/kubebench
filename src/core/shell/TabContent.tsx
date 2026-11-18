import { registry } from '@/core/plugin/registry'
import type { TabDescriptor } from '@/core/plugin/types'

/**
 * The page of a tab, rendered by the component its plugin registered for
 * the tab's kind (nothing while no active plugin renders that kind).
 */
export function TabContent({ tab }: { tab: TabDescriptor }) {
  const Renderer = registry.tabRenderer(tab.kind)
  // The renderer is looked up, not created: plugins register module-level
  // components, so the same kind always yields the same component and its
  // state survives re-renders. The rule cannot see through the registry.
  // oxlint-disable-next-line react/static-components
  return Renderer ? <Renderer tab={tab} /> : null
}

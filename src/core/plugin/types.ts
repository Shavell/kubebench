import type { ComponentType } from 'react'

/**
 * A tab in the editor area. Tabs are plain serializable descriptors;
 * the component that renders a tab is resolved through the plugin
 * registry by `kind`, so any plugin can open tabs rendered by another
 * plugin without importing it.
 */
export interface TabDescriptor {
  /** Unique id, e.g. "pod:default/nginx-7d4b" or "yaml:pod:default/nginx-7d4b". */
  id: string
  title: string
  /** Codicon name without the "codicon-" prefix, e.g. "file-code". */
  icon?: string
  /** Maps to a registered TabRendererContribution. */
  kind: string
  /** Serializable payload for the renderer (resource refs, yaml text...). */
  props?: Record<string, unknown>
}

export interface SidebarViewContribution {
  id: string
  /** Shown uppercase in the sidebar header, like VSCode view titles. */
  title: string
  component: ComponentType
}

export interface BottomPanelTabContribution {
  id: string
  title: string
  component: ComponentType
  order?: number
}

export interface StatusBarItemContribution {
  id: string
  alignment: 'left' | 'right'
  /** Higher priority renders closer to the edge. */
  priority?: number
  component: ComponentType
}

export interface TabRendererContribution {
  kind: string
  component: ComponentType<{ tab: TabDescriptor }>
}

export interface CommandContribution {
  id: string
  /** Shown in the palette, e.g. "Helm: Open Releases". */
  title: string
  /** Codicon name shown next to the title. */
  icon?: string
  run: () => void
}

/**
 * Everything a module contributes to the shell. The shell only knows
 * this shape — never plugin internals.
 */
export interface PluginManifest {
  id: string
  name: string
  /** Omit to contribute no activity bar entry (e.g. the terminal plugin). */
  activityBar?: { icon: string; tooltip: string; order?: number }
  sidebar?: SidebarViewContribution
  tabRenderers?: TabRendererContribution[]
  bottomPanelTabs?: BottomPanelTabContribution[]
  statusBarItems?: StatusBarItemContribution[]
  commands?: CommandContribution[]
}

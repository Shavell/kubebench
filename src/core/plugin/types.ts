import type { ComponentType } from 'react'
import type { DataProviders } from '@/data/providers'

/**
 * A tab in the editor area. Tabs are plain serializable descriptors;
 * the component that renders a tab is resolved through the plugin
 * registry by `kind`, so any plugin can open tabs rendered by another
 * plugin without importing it.
 */
export interface TabDescriptor {
  /** Unique id, e.g. "pod:default/nginx-7d4b" or "yaml:pod:default/nginx-7d4b". */
  id: string
  /** Title as translated when the tab was opened; shown when `titleKey` is not set. */
  title: string
  /**
   * i18n key of the title in namespace `titleNs`, for tabs whose title is
   * a UI string ("Settings", "Pods", "{{name}} · logs"), so it follows
   * language changes. `titleValues` fills its interpolation variables.
   */
  titleKey?: string
  titleNs?: string
  titleValues?: Record<string, string | number>
  /** Codicon name without the "codicon-" prefix, e.g. "file-code". */
  icon?: string
  /** Maps to a registered TabRendererContribution. */
  kind: string
  /** Serializable payload for the renderer (resource refs, yaml text...). */
  props?: Record<string, unknown>
  /**
   * Unsaved changes (set with layoutStore.setTabDirty): the tab strip marks
   * the tab, and closing it asks before the changes are discarded.
   */
  dirty?: boolean
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
  /**
   * Request kinds this tab shows when another plugin calls `showInPanel`
   * (core/plugin/panelRequests.ts), e.g. ['pod-logs'].
   */
  kinds?: string[]
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
  /**
   * Global keyboard shortcut, e.g. "Ctrl+," or "Alt+N" (Ctrl is Cmd on
   * macOS); shown in the palette and the shortcuts list. Not bound inside
   * terminals, which need the keys themselves.
   */
  keybinding?: string
  run: () => void
}

/** Grouping shown in the Extensions view (design §6.4). */
export type PluginCategory =
  | 'core'
  | 'workloads'
  | 'gitops'
  | 'storage'
  | 'security'
  | 'observability'
  | 'tools'

/** A Kubernetes object a resource action is invoked on. */
export interface ResourceRef {
  kind: string
  apiVersion?: string
  namespace?: string
  name: string
}

/**
 * An action a plugin adds to resource row context menus and detail page
 * toolbars of other views (workloads, CRDs), e.g. "Show topology".
 */
export interface ResourceActionContribution {
  id: string
  /** i18n key in the contributing plugin's namespace. */
  title: string
  icon?: string
  /** Kinds the action applies to, e.g. ['Deployment', 'Service'], or ['*'] for all. */
  kinds: string[]
  /**
   * Permission needed on the active cluster (default 'read'). Hosts render
   * the action disabled, with the reason, when it is not held.
   */
  permission?: 'read' | 'write' | 'exec'
  run: (ref: ResourceRef) => void
  /**
   * Rendered inside the detail-page button after the title (row menus
   * show no badge), e.g. the number of notes on the object. A component,
   * so it can load what it shows; render nothing when there is nothing to say.
   */
  badge?: ComponentType<{ resource: ResourceRef }>
}

/** An object of a resource kind, as kubectl-style palette commands name it. */
export interface ResourceKindObject {
  namespace?: string
  name: string
}

/**
 * A resource kind a plugin can list and show, addressed by kubectl names in
 * the command palette: `get <kind> [-n ns]` opens the list, `describe
 * <kind>/<name>` (or `get <kind> <name>`) the object's details.
 */
export interface ResourceKindContribution {
  /** Kubernetes kind, e.g. 'Pod'. */
  kind: string
  /** Lowercase kubectl names: plural first, then singular and short names ('pods', 'pod', 'po'). */
  names: string[]
  namespaced: boolean
  icon?: string
  /**
   * Opens the list; `namespace` scopes a namespaced list, `null` lists all
   * namespaces whatever the global filter (palette `-A`), and without it
   * the global filter applies.
   */
  openList: (namespace?: string | null) => void
  openDetail: (object: ResourceKindObject) => void
  /** Objects for name completion, in `namespace` or in all namespaces. */
  listObjects: (providers: DataProviders, namespace?: string) => Promise<ResourceKindObject[]>
}

/** A section a plugin adds to the Settings page. */
export interface SettingsSectionContribution {
  id: string
  /** i18n key in the contributing plugin's namespace. */
  title: string
  component: ComponentType
}

/**
 * Everything a module contributes to the shell. The shell only knows
 * this shape — never plugin internals.
 *
 * i18n: display strings in a manifest (name, description,
 * activityBar.tooltip, sidebar.title, command/bottom-panel titles,
 * resource action and settings section titles) are i18n keys in the
 * plugin's own namespace (= plugin id). The shell resolves them with
 * translateManifestText()/useManifestText() from '@/i18n'; a key that is
 * not found renders literally, so plain strings keep working.
 */
export interface PluginManifest {
  id: string
  name: string
  /** Shown in the Extensions view; built-in plugins default to the app version. */
  version?: string
  /** i18n key describing the plugin, shown in the Extensions view. */
  description?: string
  category?: PluginCategory
  /**
   * Part of the workbench itself (workloads, clusters, settings, terminal,
   * editor, extensions, users, audit): always enabled, cannot be disabled.
   */
  core?: boolean
  /**
   * Id of the backend plugin this UI needs. While that plugin is disabled
   * or reports itself unavailable on the active cluster, this plugin's
   * contributions are hidden and the Extensions view shows the reason.
   */
  backendPlugin?: string
  /**
   * Translation bundles for this plugin's namespace, keyed by language
   * code: `{ en: enJson, tr: trJson }`. Registered automatically when
   * the plugin is registered.
   */
  locales?: Record<string, object>
  /**
   * Omit to contribute no activity bar entry (e.g. the terminal plugin).
   * `order` sorts the entries: the built-in views use 0–6 and Extensions 90;
   * without it the entry goes after the built-in views (50).
   */
  activityBar?: { icon: string; tooltip: string; order?: number }
  sidebar?: SidebarViewContribution
  tabRenderers?: TabRendererContribution[]
  bottomPanelTabs?: BottomPanelTabContribution[]
  statusBarItems?: StatusBarItemContribution[]
  commands?: CommandContribution[]
  resourceActions?: ResourceActionContribution[]
  settingsSections?: SettingsSectionContribution[]
  /** Kinds for the palette's kubectl-style `get` and `describe` commands. */
  resourceKinds?: ResourceKindContribution[]
}

/** Where a registered plugin came from. */
export type PluginSource = 'builtin' | 'external'

/**
 * Contributions returned by the registry carry the id of the plugin that
 * contributed them, so hosts can translate titles in its namespace.
 */
export type Contributed<T> = T & { pluginId: string }

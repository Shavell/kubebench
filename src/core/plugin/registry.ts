import type { ComponentType } from 'react'
import type {
  BottomPanelTabContribution,
  CommandContribution,
  Contributed,
  PluginManifest,
  PluginSource,
  ResourceActionContribution,
  SettingsSectionContribution,
  SidebarViewContribution,
  StatusBarItemContribution,
  TabDescriptor,
} from './types'
import { registerTranslations } from '@/i18n'
import {
  resolvePluginStatus,
  usePluginStatusStore,
  type ResolvedPluginStatus,
} from './pluginStatusStore'

interface Registration {
  manifest: PluginManifest
  source: PluginSource
}

/**
 * All plugins known to the shell. Contribution getters only return
 * contributions of *active* plugins (enabled and available, see
 * pluginStatusStore); `all()` lists everything for the Extensions view.
 * Listeners are told about registrations and status changes so the shell
 * can re-render.
 */
class PluginRegistry {
  private plugins = new Map<string, Registration>()
  private listeners = new Set<() => void>()
  private revision = 0

  constructor() {
    usePluginStatusStore.subscribe(() => this.emit())
  }

  register(manifest: PluginManifest, source: PluginSource = 'builtin'): void {
    if (this.plugins.has(manifest.id)) {
      throw new Error(`Plugin "${manifest.id}" is already registered`)
    }
    if (manifest.locales) registerTranslations(manifest.id, manifest.locales)
    this.plugins.set(manifest.id, { manifest, source })
    this.emit()
  }

  /** Every registered plugin, including disabled and unavailable ones. */
  all(): PluginManifest[] {
    return [...this.plugins.values()].map((r) => r.manifest)
  }

  get(id: string): PluginManifest | undefined {
    return this.plugins.get(id)?.manifest
  }

  sourceOf(id: string): PluginSource | undefined {
    return this.plugins.get(id)?.source
  }

  status(id: string): ResolvedPluginStatus {
    const registration = this.plugins.get(id)
    return resolvePluginStatus(id, registration?.manifest, registration?.source ?? 'external')
  }

  isActive(id: string): boolean {
    const { enabled, available } = this.status(id)
    return this.plugins.has(id) && enabled && available
  }

  /** Enabled and available plugins: the ones whose contributions are shown. */
  active(): PluginManifest[] {
    return this.all().filter((p) => this.isActive(p.id))
  }

  activityBarEntries(): PluginManifest[] {
    return this.active()
      .filter((p) => p.activityBar)
      .sort((a, b) => (a.activityBar!.order ?? 0) - (b.activityBar!.order ?? 0))
  }

  sidebarFor(pluginId: string): SidebarViewContribution | undefined {
    return this.isActive(pluginId) ? this.get(pluginId)?.sidebar : undefined
  }

  tabRenderer(kind: string): ComponentType<{ tab: TabDescriptor }> | undefined {
    for (const plugin of this.active()) {
      const renderer = plugin.tabRenderers?.find((r) => r.kind === kind)
      if (renderer) return renderer.component
    }
    return undefined
  }

  bottomPanelTabs(): Contributed<BottomPanelTabContribution>[] {
    return this.contributions((p) => p.bottomPanelTabs).sort((a, b) => (a.order ?? 0) - (b.order ?? 0))
  }

  commands(): Contributed<CommandContribution>[] {
    return this.contributions((p) => p.commands)
  }

  statusBarItems(alignment: 'left' | 'right'): Contributed<StatusBarItemContribution>[] {
    return this.contributions((p) => p.statusBarItems)
      .filter((item) => item.alignment === alignment)
      .sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0))
  }

  /** Actions other plugins contribute for a resource kind (row menus, detail toolbars). */
  resourceActionsFor(kind: string): Contributed<ResourceActionContribution>[] {
    return this.contributions((p) => p.resourceActions).filter(
      (action) => action.kinds.includes('*') || action.kinds.includes(kind),
    )
  }

  settingsSections(): Contributed<SettingsSectionContribution>[] {
    return this.contributions((p) => p.settingsSections)
  }

  subscribe(listener: () => void): () => void {
    this.listeners.add(listener)
    return () => this.listeners.delete(listener)
  }

  /** Bumped on every registration or plugin status change. */
  getRevision(): number {
    return this.revision
  }

  private contributions<T>(pick: (p: PluginManifest) => T[] | undefined): Contributed<T>[] {
    return this.active().flatMap((p) => (pick(p) ?? []).map((c) => ({ ...c, pluginId: p.id })))
  }

  private emit(): void {
    this.revision++
    this.listeners.forEach((listener) => listener())
  }
}

export const registry = new PluginRegistry()

import type { ComponentType } from 'react'
import type {
  BottomPanelTabContribution,
  PluginManifest,
  SidebarViewContribution,
  StatusBarItemContribution,
  TabDescriptor,
} from './types'

class PluginRegistry {
  private plugins = new Map<string, PluginManifest>()

  register(manifest: PluginManifest): void {
    if (this.plugins.has(manifest.id)) {
      throw new Error(`Plugin "${manifest.id}" is already registered`)
    }
    this.plugins.set(manifest.id, manifest)
  }

  all(): PluginManifest[] {
    return [...this.plugins.values()]
  }

  get(id: string): PluginManifest | undefined {
    return this.plugins.get(id)
  }

  activityBarEntries(): PluginManifest[] {
    return this.all()
      .filter((p) => p.activityBar)
      .sort((a, b) => (a.activityBar!.order ?? 0) - (b.activityBar!.order ?? 0))
  }

  sidebarFor(pluginId: string): SidebarViewContribution | undefined {
    return this.plugins.get(pluginId)?.sidebar
  }

  tabRenderer(kind: string): ComponentType<{ tab: TabDescriptor }> | undefined {
    for (const plugin of this.plugins.values()) {
      const renderer = plugin.tabRenderers?.find((r) => r.kind === kind)
      if (renderer) return renderer.component
    }
    return undefined
  }

  bottomPanelTabs(): BottomPanelTabContribution[] {
    return this.all()
      .flatMap((p) => p.bottomPanelTabs ?? [])
      .sort((a, b) => (a.order ?? 0) - (b.order ?? 0))
  }

  statusBarItems(alignment: 'left' | 'right'): StatusBarItemContribution[] {
    return this.all()
      .flatMap((p) => p.statusBarItems ?? [])
      .filter((item) => item.alignment === alignment)
      .sort((a, b) => (b.priority ?? 0) - (a.priority ?? 0))
  }
}

export const registry = new PluginRegistry()

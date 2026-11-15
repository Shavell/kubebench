import type { PluginManifest, ResourceRef } from '@/core/plugin/types'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { TopologyPage } from './components/TopologyPage'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import type { TopologyKind } from './model/types'
import { FOCUSABLE_KINDS, ICON, openTopology, TAB_KIND } from './tabs'
import { TopologySidebar } from './TopologySidebar'

/** The namespace list views filter on, else `default`. */
function currentNamespace(): string {
  const namespace = useContextStore.getState().namespace
  return namespace === ALL_NAMESPACES ? 'default' : namespace
}

function showTopology(ref: ResourceRef): void {
  openTopology(ref.namespace || currentNamespace(), { kind: ref.kind as TopologyKind, name: ref.name })
}

/**
 * Topology (design §7.9): a frontend-only plugin that draws a namespace's
 * Ingresses, Services, workloads, Pods, volumes and config as a graph from
 * the core providers. Display strings below are i18n keys in its namespace.
 */
export const topologyPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'workloads',
  locales: { en, tr },
  activityBar: { icon: ICON, tooltip: 'manifest.tooltip' },
  sidebar: { id: 'topology-namespaces', title: 'manifest.sidebarTitle', component: TopologySidebar },
  tabRenderers: [{ kind: TAB_KIND, component: TopologyPage }],
  commands: [{ id: 'topology.open', title: 'commands.open', icon: ICON, run: () => openTopology(currentNamespace()) }],
  resourceActions: [
    { id: 'topology.show', title: 'resourceActions.show', icon: ICON, kinds: FOCUSABLE_KINDS, run: showTopology },
  ],
}

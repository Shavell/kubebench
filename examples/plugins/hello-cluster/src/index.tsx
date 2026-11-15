/**
 * Hello Cluster — an example KubeBench external plugin (docs/design-v0.5.md
 * §6.8). Built as an IIFE (see vite.config.ts); when KubeBench loads the
 * script, it registers an activity bar entry, a side bar view, a tab that
 * lists namespaces and a command.
 */
import type { PluginManifest } from '@kubebench/sdk'
import de from './locales/de.json'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { NamespacesTab } from './NamespacesTab'
import { NAMESPACES_TAB, openNamespacesTab, PLUGIN_ID, PLUGIN_VERSION } from './plugin'
import { Sidebar } from './Sidebar'

// Display strings are keys in the plugin's namespace (its id), from `locales`.
const manifest: PluginManifest = {
  id: PLUGIN_ID,
  name: 'manifest.name',
  version: PLUGIN_VERSION,
  description: 'manifest.description',
  category: 'tools',
  locales: { en, tr, de },
  activityBar: { icon: 'globe', tooltip: 'manifest.tooltip', order: 50 },
  sidebar: { id: `${PLUGIN_ID}.sidebar`, title: 'manifest.name', component: Sidebar },
  tabRenderers: [{ kind: NAMESPACES_TAB, component: NamespacesTab }],
  commands: [{ id: `${PLUGIN_ID}.showNamespaces`, title: 'commands.showNamespaces', icon: 'globe', run: openNamespacesTab }],
}

window.KubeBench.registerPlugin(manifest)

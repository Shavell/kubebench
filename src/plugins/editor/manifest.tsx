import { lazyComponent } from '@/core/plugin/lazyComponent'
import { openNewResourceTab } from '@/core/plugin/editorTabs'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { EDITOR_NS } from './i18n'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const NewResourceTab = lazyComponent(() => import('./NewResourceTab').then((m) => m.NewResourceTab))
const YamlEditorTab = lazyComponent(() => import('./YamlEditorTab').then((m) => m.YamlEditorTab))

/**
 * The resource editor (design §7.2): the "yaml" tab kind (view, edit,
 * diff, dry run, server-side apply) and the "new-resource" tab kind
 * (templates and imported files). Other plugins open them through
 * core/plugin/editorTabs. Part of the workbench, so it cannot be disabled.
 */
export const editorPlugin: PluginManifest = {
  id: EDITOR_NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'core',
  core: true,
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  tabRenderers: [
    { kind: 'yaml', component: YamlEditorTab },
    { kind: 'new-resource', component: NewResourceTab },
  ],
  commands: [{ id: 'editor.newResource', title: 'commands.newResource', icon: 'new-file', run: () => openNewResourceTab() }],
}

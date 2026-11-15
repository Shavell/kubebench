import { openNewResourceTab } from '@/core/plugin/editorTabs'
import type { PluginManifest } from '@/core/plugin/types'
import { EDITOR_NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { NewResourceTab } from './NewResourceTab'
import { YamlEditorTab } from './YamlEditorTab'

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
  locales: { en, tr },
  tabRenderers: [
    { kind: 'yaml', component: YamlEditorTab },
    { kind: 'new-resource', component: NewResourceTab },
  ],
  commands: [{ id: 'editor.newResource', title: 'commands.newResource', icon: 'new-file', run: () => openNewResourceTab() }],
}

import type { PluginManifest } from '@/core/plugin/types'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { TerminalPanel } from './panels/TerminalPanel'
import { LogsPanel } from './panels/LogsPanel'
import { ProblemsPanel } from './panels/ProblemsPanel'
import { PodLogsTab } from './PodLogsTab'
import { PodExecTab } from './PodExecTab'

/**
 * Contributes the bottom panel tabs and the pod logs and shell tabs (the
 * "yaml" tab is the editor plugin's). Has no activity bar entry of its own.
 * Part of the workbench itself, so it cannot be disabled. Display strings
 * are i18n keys in the plugin's namespace (locales/*.json).
 */
export const terminalPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'core',
  core: true,
  locales: { en, tr },
  bottomPanelTabs: [
    { id: 'problems', title: 'panels.problems', component: ProblemsPanel, order: 1 },
    // Other plugins open these with showInPanel('pod-logs' | 'pod-exec', {namespace, pod, container}).
    { id: 'logs', title: 'panels.logs', component: LogsPanel, order: 2, kinds: ['pod-logs'] },
    { id: 'terminal', title: 'panels.terminal', component: TerminalPanel, order: 3, kinds: ['pod-exec'] },
  ],
  tabRenderers: [
    { kind: 'pod-logs', component: PodLogsTab },
    { kind: 'pod-exec', component: PodExecTab },
  ],
}

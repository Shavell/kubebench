import type { PluginManifest } from '@/core/plugin/types'
import { TerminalPanel } from './panels/TerminalPanel'
import { LogsPanel } from './panels/LogsPanel'
import { ProblemsPanel } from './panels/ProblemsPanel'
import { YamlEditorTab } from './YamlEditorTab'
import { PodLogsTab } from './PodLogsTab'
import { PodExecTab } from './PodExecTab'

/**
 * Contributes the bottom panel tabs and the shared "yaml" editor tab
 * renderer. Has no activity bar entry of its own.
 */
export const terminalPlugin: PluginManifest = {
  id: 'terminal',
  name: 'Terminal & Editors',
  bottomPanelTabs: [
    { id: 'problems', title: 'Problems', component: ProblemsPanel, order: 1 },
    { id: 'logs', title: 'Logs', component: LogsPanel, order: 2 },
    { id: 'terminal', title: 'Terminal', component: TerminalPanel, order: 3 },
  ],
  tabRenderers: [
    { kind: 'yaml', component: YamlEditorTab },
    { kind: 'pod-logs', component: PodLogsTab },
    { kind: 'pod-exec', component: PodExecTab },
  ],
}

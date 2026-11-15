import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { TerminalPanel } from './panels/TerminalPanel'
import { LogsPanel } from './panels/LogsPanel'
import { ProblemsPanel } from './panels/ProblemsPanel'
import { workloadLogsAction } from './workloadLogs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const PodLogsTab = lazyComponent(() => import('./PodLogsTab').then((m) => m.PodLogsTab))
const PodExecTab = lazyComponent(() => import('./PodExecTab').then((m) => m.PodExecTab))
const WorkloadLogsTab = lazyComponent(() => import('./WorkloadLogsTab').then((m) => m.WorkloadLogsTab))

/**
 * Contributes the bottom panel tabs (problems, live logs, terminal
 * sessions), the log and shell tabs and the "Logs (all pods)" action of
 * workloads (the "yaml" tab is the editor plugin's). Has no activity bar
 * entry of its own. Part of the workbench itself, so it cannot be
 * disabled. Display strings are i18n keys in the plugin's namespace
 * (locales/*.json).
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
    // {namespace, selector, title}: the logs of every pod a label selector matches.
    { kind: 'workload-logs', component: WorkloadLogsTab },
  ],
  resourceActions: [workloadLogsAction],
}

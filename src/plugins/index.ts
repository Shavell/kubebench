import type { PluginManifest } from '@/core/plugin/types'
import { workloadsPlugin } from './workloads/manifest'
import { crdPlugin } from './crd/manifest'
import { helmPlugin } from './helm/manifest'
import { metricsPlugin } from './metrics/manifest'
import { terminalPlugin } from './terminal/manifest'

/**
 * The single place modules are wired in. Comment one out and its
 * activity bar icon, sidebar, tabs, panels, and status bar items all
 * disappear — nothing else references plugin internals.
 */
export const allPlugins: PluginManifest[] = [
  workloadsPlugin,
  crdPlugin,
  helmPlugin,
  metricsPlugin,
  terminalPlugin,
]

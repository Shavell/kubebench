import type { PluginManifest } from '@/core/plugin/types'
import { clustersPlugin } from './clusters/manifest'
import { workloadsPlugin } from './workloads/manifest'
import { crdPlugin } from './crd/manifest'
import { helmPlugin } from './helm/manifest'
import { metricsPlugin } from './metrics/manifest'
import { terminalPlugin } from './terminal/manifest'
import { settingsPlugin } from './settings/manifest'
export { usersPlugin } from './users/manifest'

/**
 * The single place modules are wired in. Comment one out and its
 * activity bar icon, sidebar, tabs, panels, and status bar items all
 * disappear — nothing else references plugin internals.
 */
export const allPlugins: PluginManifest[] = [
  clustersPlugin,
  workloadsPlugin,
  crdPlugin,
  helmPlugin,
  metricsPlugin,
  terminalPlugin,
  settingsPlugin,
]

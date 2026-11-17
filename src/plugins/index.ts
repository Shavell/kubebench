import type { PluginManifest } from '@/core/plugin/types'
import { auditPlugin } from './audit/manifest'
import { clustersPlugin } from './clusters/manifest'
import { crdPlugin } from './crd/manifest'
import { extensionsPlugin } from './extensions/manifest'
import { helmPlugin } from './helm/manifest'
import { metricsPlugin } from './metrics/manifest'
import { settingsPlugin } from './settings/manifest'
import { terminalPlugin } from './terminal/manifest'
import { usersPlugin } from './users/manifest'
import { workloadsPlugin } from './workloads/manifest'
import { editorPlugin } from './editor/manifest'
import { argocdPlugin } from './argocd/manifest'
import { storagePlugin } from './storage/manifest'
import { accessPlugin } from './access/manifest'
import { securityPlugin } from './security/manifest'
import { topologyPlugin } from './topology/manifest'
import { notesPlugin } from './notes/manifest'
import { serviceBrowserPlugin } from './service-browser/manifest'

/**
 * The single place built-in modules are wired in. Comment one out and its
 * activity bar icon, sidebar, tabs, panels and status bar items all
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
  extensionsPlugin,
  editorPlugin,
  argocdPlugin,
  storagePlugin,
  accessPlugin,
  securityPlugin,
  topologyPlugin,
  notesPlugin,
  serviceBrowserPlugin,
]

/** Registered at boot only when the signed-in user is a global admin. */
export const adminPlugins: PluginManifest[] = [usersPlugin, auditPlugin]

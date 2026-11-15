import { PLATFORM_NS } from '@/core/locales'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { ResourceRef } from './types'

/**
 * Openers for the resource editor's tab kinds (design §7.2). Like
 * showInPanel, they let any plugin open the editor without importing it:
 *
 * - `yaml` with props `{ ref: {apiVersion, kind, namespace, name} }` loads
 *   a cluster object and offers Neat, Copy, Download and Edit → Review diff
 *   → Dry run → Apply; with `{ text, readOnly? }` (no ref) it shows that
 *   text, as in v0.4.
 * - `new-resource` with props `{ namespace?, kind? }` creates objects from a
 *   template (the one for `kind`, if any) or imported files.
 */

/** A cluster object for the `yaml` tab: a ResourceRef whose apiVersion is known. */
export type YamlTabRef = ResourceRef & { apiVersion: string }

/** One YAML tab per object, whoever opens it. */
export function yamlTabId(ref: YamlTabRef): string {
  return `yaml:${ref.apiVersion}/${ref.kind}:${ref.namespace || '-'}/${ref.name}`
}

export function openYamlTab(ref: YamlTabRef): void {
  useLayoutStore.getState().openTab({
    id: yamlTabId(ref),
    title: `${ref.name}.yaml`,
    icon: 'file-code',
    kind: 'yaml',
    props: { ref: { apiVersion: ref.apiVersion, kind: ref.kind, namespace: ref.namespace || null, name: ref.name } },
  })
}

/** Opens the New resource tab, or brings the open one to the front. */
export function openNewResourceTab(options: { namespace?: string; kind?: string } = {}): void {
  useLayoutStore.getState().openTab({
    id: 'new-resource',
    title: 'New resource',
    titleKey: 'editor.newResourceTab',
    titleNs: PLATFORM_NS,
    icon: 'new-file',
    kind: 'new-resource',
    props: options,
  })
}

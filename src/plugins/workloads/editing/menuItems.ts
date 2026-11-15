import { can } from '@/core/permissions'
import { openYamlTab } from '@/core/plugin/editorTabs'
import type { ResourceRef } from '@/core/plugin/types'
import type { ContextMenuItem } from '@/shared/contextMenu'
import { tw } from '../i18n'
import { deletable, isSecret, openResourceDialog, targetOf } from './dialogStore'

/** "View YAML": the editor's YAML tab, where the object can also be edited and applied. */
export function yamlMenuItem(ref: ResourceRef): ContextMenuItem[] {
  const target = targetOf(ref)
  if (!target) return []
  return [{ label: tw('actions.viewYaml'), icon: 'file-code', onClick: () => openYamlTab({ ...target, namespace: target.namespace ?? undefined }) }]
}

/**
 * Row menu items of resource editing (design §7.2) the user may run:
 * labels and annotations, Secret reveal, and the generic delete unless the
 * page has its own (`withDelete: false`, e.g. Pods).
 */
export function editingMenuItems(ref: ResourceRef, { withDelete = true } = {}): ContextMenuItem[] {
  const target = targetOf(ref)
  if (!target) return []
  const items: ContextMenuItem[] = []
  if (can('write')) {
    items.push({ label: tw('editing.menu.metadata'), icon: 'tag', onClick: () => openResourceDialog({ type: 'metadata', target }) })
  }
  if (isSecret(target) && can('exec')) {
    items.push({ label: tw('editing.menu.reveal'), icon: 'eye', onClick: () => openResourceDialog({ type: 'reveal', target }) })
  }
  if (withDelete && deletable(target) && can('write')) {
    items.push({ label: tw('editing.menu.delete'), icon: 'trash', danger: true, onClick: () => openResourceDialog({ type: 'delete', target }) })
  }
  return items
}

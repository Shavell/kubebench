import { deniedReason } from '@/core/permissions'
import { translateManifestText } from '@/i18n'
import type { ContextMenuItem } from '@/shared/contextMenu'
import { registry } from './registry'
import type { ResourceRef } from './types'

/**
 * Context menu entries for the actions other plugins contribute to
 * `ref.kind` (design §6.4), for row menus of resource lists. An action
 * whose permission is not held stays listed, disabled, with the reason as
 * tooltip. Pair with ResourceActionButtons on detail pages.
 */
export function resourceActionItems(ref: ResourceRef): ContextMenuItem[] {
  return registry.resourceActionsFor(ref.kind).map((action) => {
    const reason = deniedReason(action.permission ?? 'read')
    return {
      label: translateManifestText(action.pluginId, action.title),
      icon: action.icon,
      disabled: reason !== null,
      title: reason ?? undefined,
      onClick: () => action.run(ref),
    }
  })
}

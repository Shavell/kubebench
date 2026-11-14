import { deniedReason, usePermissions } from '@/core/permissions'
import { useManifestText } from '@/i18n'
import { Button } from '@/shared/Button'
import { registry } from './registry'
import type { ResourceRef } from './types'
import { useRegistryRevision } from './useRegistry'

/**
 * Detail-page toolbar buttons for the actions other plugins contribute to
 * `resource.kind` (design §6.4; resourceActionItems is the row-menu
 * counterpart). Follows plugins being enabled or disabled; an action whose
 * permission is not held is disabled with the reason as tooltip.
 */
export function ResourceActionButtons({ resource }: { resource: ResourceRef }) {
  useRegistryRevision()
  const perms = usePermissions()
  const text = useManifestText()

  return registry.resourceActionsFor(resource.kind).map((action) => {
    const reason = deniedReason(action.permission ?? 'read', perms)
    return (
      <Button
        key={`${action.pluginId}/${action.id}`}
        variant="secondary"
        icon={action.icon}
        disabled={reason !== null}
        title={reason ?? undefined}
        onClick={() => action.run(resource)}
      >
        {text(action.pluginId, action.title)}
      </Button>
    )
  })
}

import { usePermission } from '@/core/permissions'
import { Button } from '@/shared/Button'

/**
 * A toolbar or row button for a mutation: disabled, with the reason as
 * tooltip, while the write permission is not held.
 */
export function WriteButton({
  label,
  icon,
  onClick,
  variant = 'secondary',
  disabledReason,
}: {
  label: string
  icon?: string
  onClick: () => void
  variant?: 'primary' | 'secondary'
  /** Another reason the action is not possible right now (shown when the permission is held). */
  disabledReason?: string | null
}) {
  const { allowed, reason } = usePermission('write')
  const blocked = !allowed ? reason : (disabledReason ?? null)
  return (
    <Button
      variant={variant}
      icon={icon}
      disabled={blocked !== null}
      title={blocked ?? undefined}
      onClick={(event) => {
        event.stopPropagation()
        onClick()
      }}
    >
      {label}
    </Button>
  )
}

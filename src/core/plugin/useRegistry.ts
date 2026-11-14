import { useSyncExternalStore } from 'react'
import { registry } from './registry'

/**
 * Re-renders the caller whenever plugins register or their enabled /
 * available status changes. The shell uses it so the activity bar,
 * sidebar, panels and status bar follow the Extensions view live.
 */
export function useRegistryRevision(): number {
  return useSyncExternalStore(
    (onChange) => registry.subscribe(onChange),
    () => registry.getRevision(),
  )
}

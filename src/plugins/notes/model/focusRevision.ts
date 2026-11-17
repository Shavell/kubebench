import { useSyncExternalStore } from 'react'

/**
 * A counter that grows whenever the browser window regains focus or the
 * page becomes visible again. Note queries list it in their deps, so they
 * re-run when the user comes back — when another user's change may
 * matter. Notes are KubeBench data, not Kubernetes objects, so the live
 * update stream (design §7.10) cannot report their changes (§7.13).
 */
let revision = 0
const listeners = new Set<() => void>()

function bump(): void {
  if (document.visibilityState === 'hidden') return
  revision++
  listeners.forEach((listener) => listener())
}

function subscribe(listener: () => void): () => void {
  if (listeners.size === 0) {
    window.addEventListener('focus', bump)
    document.addEventListener('visibilitychange', bump)
  }
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
    if (listeners.size === 0) {
      window.removeEventListener('focus', bump)
      document.removeEventListener('visibilitychange', bump)
    }
  }
}

export function useFocusRevision(): number {
  return useSyncExternalStore(subscribe, () => revision)
}

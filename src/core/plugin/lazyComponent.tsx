import { createElement, lazy, Suspense, useState, type ComponentType, type ReactNode } from 'react'
import { LoadBoundary, ViewLoading } from './LazyViewParts'

/**
 * A component whose code is loaded on first render (a separate chunk), for
 * the tab pages of plugins: manifests stay small and the main bundle only
 * carries what the workbench needs at start.
 *
 *   const ReleaseListPage = lazyComponent(() => import('./pages/ReleaseListPage').then((m) => m.ReleaseListPage))
 *
 * A placeholder shows while the chunk loads; a failed load (server
 * unreachable, a stale page after an upgrade) shows the reason and a Retry
 * button instead of breaking the workbench.
 */
export function lazyComponent<P extends object>(load: () => Promise<ComponentType<P>>): (props: P) => ReactNode {
  const create = () => lazy(async () => ({ default: await load() }))
  // A rejected lazy() keeps rejecting, so a retry needs a fresh one (shared by every instance).
  let loaded = create()

  function LazyView(props: P) {
    const [attempt, setAttempt] = useState(0)
    const retry = () => {
      loaded = create()
      setAttempt((n) => n + 1)
    }
    return (
      <LoadBoundary key={attempt} onRetry={retry}>
        <Suspense fallback={<ViewLoading />}>{createElement(loaded, props)}</Suspense>
      </LoadBoundary>
    )
  }
  return LazyView
}

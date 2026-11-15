import type { LiveProvider } from '../providers'
import type { WatchHandlers, WatchStream } from '../streams'

/** Like the server: topics report `starting`, then `live` once "synced". */
const SYNC_LATENCY_MS = 150

/**
 * Live updates in mock mode. The mock dataset changes only through the
 * UI's own mutations, which refresh their views themselves, so the stream
 * reports every topic live and never a change.
 */
function watch(topics: readonly string[], handlers: WatchHandlers): WatchStream {
  // Handlers run asynchronously, as with a real stream.
  const opened = setTimeout(() => {
    for (const topic of topics) handlers.onStatus({ topic, mode: 'starting' })
  }, 0)
  const synced = setTimeout(() => {
    for (const topic of topics) handlers.onStatus({ topic, mode: 'live' })
  }, SYNC_LATENCY_MS)
  return {
    close: () => {
      clearTimeout(opened)
      clearTimeout(synced)
    },
  }
}

export function createMockLive(): LiveProvider {
  return { watch }
}

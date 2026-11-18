import { ApiError, apiRefusal, apiUrl } from '../apiClient'
import type { LiveProvider } from '../providers'
import type { WatchHandlers, WatchStream } from '../streams'
import type { WatchChange, WatchTopicStatus } from '../types'
import { streamReasonText } from './streamReason'

/**
 * Follows GET /api/watch with an EventSource. Like the log stream, the
 * browser's own reconnect is switched off by closing the source on the
 * first transport error: the live-update controller reconnects with its
 * own backoff. EventSource hides refused responses, so their reason is
 * fetched separately (apiRefusal).
 */
function watch(topics: readonly string[], handlers: WatchHandlers): WatchStream {
  const params = { topics: topics.join(',') }
  const source = new EventSource(apiUrl('/watch', params))
  let opened = false
  let finished = false // the end or a failure was reported, or the caller closed the stream
  let cancelled = false // the caller closed the stream
  const finish = () => {
    finished = true
    source.close()
  }

  source.addEventListener('status', (event) => {
    if (finished) return
    opened = true
    handlers.onStatus(JSON.parse(event.data) as WatchTopicStatus)
  })
  source.addEventListener('change', (event) => {
    if (!finished) handlers.onChange(JSON.parse(event.data) as WatchChange)
  })
  source.addEventListener('end', (event) => {
    if (finished) return
    finish()
    handlers.onEnd(streamReasonText(String(event.data)))
  })
  source.addEventListener('error', () => {
    if (finished) return
    finish()
    if (opened) {
      handlers.onFailure(new ApiError(0, 'network', 'the live-update stream was interrupted'))
      return
    }
    void apiRefusal('/watch', params).then((error) => {
      if (!cancelled) handlers.onFailure(error ?? new ApiError(0, 'network', 'the live-update stream could not be opened'))
    })
  })
  return {
    close: () => {
      cancelled = true
      finish()
    },
  }
}

export function createApiLive(): LiveProvider {
  return { watch }
}

import { ApiError, apiRefusal, apiUrl, type QueryParams } from '../apiClient'
import type { LogStream, LogStreamHandlers, LogStreamRequest } from '../streams'
import { streamReasonText } from './streamReason'

/**
 * Follows logs through GET /api/logs/stream with an EventSource. The
 * browser's own reconnect is switched off by closing the source on the
 * first transport error: callers reconnect with their own backoff (and
 * resume point). EventSource hides refused responses, so their reason is
 * fetched separately (apiRefusal).
 */
export function streamLogs(request: LogStreamRequest, handlers: LogStreamHandlers): LogStream {
  const params: QueryParams = {
    ns: request.namespace,
    pod: request.pod,
    selector: request.selector,
    container: request.container,
    tail: request.tail,
    previous: request.previous || undefined,
    timestamps: request.timestamps || undefined,
    sinceSeconds: request.sinceSeconds,
  }
  const source = new EventSource(apiUrl('/logs/stream', params))
  let opened = false
  let finished = false // the end or a failure was reported, or the caller closed the stream
  let cancelled = false // the caller closed the stream
  const finish = () => {
    finished = true
    source.close()
  }

  source.addEventListener('pods', (event) => {
    if (finished) return
    opened = true
    const { pods, total } = JSON.parse(event.data) as { pods: string[]; total: number }
    handlers.onOpen(pods, total)
  })
  source.addEventListener('message', (event) => {
    if (!finished) handlers.onLine(event.data)
  })
  source.addEventListener('end', () => {
    if (finished) return
    finish()
    handlers.onEnd()
  })
  // The server's "event: error" and the source's own failures share the
  // event name; only the former carry data.
  source.addEventListener('error', (event) => {
    if (finished) return
    if (event instanceof MessageEvent) {
      handlers.onError(streamReasonText(String(event.data)))
      return
    }
    finish()
    if (opened) {
      handlers.onFailure(new ApiError(0, 'network', 'the log stream was interrupted'))
      return
    }
    void apiRefusal('/logs/stream', params).then((error) => {
      if (!cancelled) handlers.onFailure(error ?? new ApiError(0, 'network', 'the log stream could not be opened'))
    })
  })
  return {
    close: () => {
      cancelled = true
      finish()
    },
  }
}

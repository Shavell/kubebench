import { create } from 'zustand'

/**
 * State of live updates (design §7.10), shown in the status bar:
 * - `connecting`: the stream is opening;
 * - `live`: views refresh when the cluster changes (`polled` lists the
 *   topics the server cannot watch, which are refreshed on a timer);
 * - `polling`: no topic is watched (the server refused the stream, or
 *   watches none of the topics, or live updates are off and auto-refresh
 *   is on); views refresh on a timer;
 * - `offline`: the server cannot be reached; reconnecting;
 * - `off`: live updates and auto-refresh are both off.
 */
export type LiveState = 'connecting' | 'live' | 'polling' | 'offline' | 'off'

/** A topic refreshed on a timer and why: the server's reason, or `overflow` (beyond the per-stream topic limit). */
export interface PolledTopic {
  topic: string
  reason: string
  overflow: boolean
}

export interface LiveStatus {
  state: LiveState
  /** Why the state is `polling` or `offline`, as the server or the browser said. */
  reason: string | null
  /** Topics of mounted views that are not watched live. */
  polled: readonly PolledTopic[]
  /** Topics watched live. */
  watched: readonly string[]
}

export const OFF_STATUS: LiveStatus = { state: 'off', reason: null, polled: [], watched: [] }

export const useLiveStore = create<LiveStatus>()(() => OFF_STATUS)

export function setLiveStatus(status: LiveStatus): void {
  useLiveStore.setState(status, true)
}
